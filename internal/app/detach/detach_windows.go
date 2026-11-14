//go:build windows

package detach

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// start asks WMI to start the process (Win32_Process.Create), so that it is
// created by the WMI service rather than by this process: its parent is
// WmiPrvSE.exe, it is in no job object, and it doesn't inherit this process's
// environment or anything Steam hooked into it. Steam's "Stop" kills the
// game's processes however it tracks them (process tree, job, overlay), and a
// direct child of the wrapper was killed with it even outside the job.
//
// If WMI fails, the process is started directly, without a console, in its
// own process group and, if the job allows it, outside this process's job.
func start(exe string, args []string) error {
	err := startWMI(exe, args)
	if err == nil {
		return nil
	}
	log.Printf("starting through WMI failed (%v); starting it directly", err)
	const flags = windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP
	err = plain(exe, args, func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags | windows.CREATE_BREAKAWAY_FROM_JOB, HideWindow: true}
	})
	if err == nil {
		return nil
	}
	log.Printf("starting outside the job object failed (%v); starting it inside, where closing the job ends it", err)
	return plain(exe, args, func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: true}
	})
}

// startWMI starts exe through WMI, using PowerShell's Invoke-CimMethod.
func startWMI(exe string, args []string) error {
	parts := []string{syscall.EscapeArg(exe)}
	for _, a := range args {
		parts = append(parts, syscall.EscapeArg(a))
	}
	script := "$r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=" +
		psQuote(strings.Join(parts, " ")) +
		"}; if ($r.ReturnValue -ne 0) { exit [int]$r.ReturnValue }; [Console]::Out.Write($r.ProcessId)"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = withoutSteam(cmd.Environ())
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW, HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return fmt.Errorf("Win32_Process.Create: exit %d: %s", ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return err
	}
	if _, err := strconv.Atoi(strings.TrimSpace(string(out))); err != nil {
		return fmt.Errorf("Win32_Process.Create: unexpected output %q", out)
	}
	return nil
}

// psQuote quotes s as a PowerShell single-quoted string literal.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Where says whether this process runs inside a job object and which process
// started it, for the log: a detached process still in a job is ended when the
// job is closed.
func Where() string {
	var in int32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	job := "outside any job object"
	if r, _, err := proc.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&in))); r == 0 {
		job = "job object: " + err.Error()
	} else if in != 0 {
		job = "inside a job object"
	}
	return job + ", parent " + parentName()
}

// parentName is the executable name of this process's parent, or "?".
func parentName() string {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return "?"
	}
	defer func() { _ = windows.CloseHandle(snap) }()
	procs := map[uint32]windows.ProcessEntry32{}
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		procs[e.ProcessID] = e
	}
	self, ok := procs[windows.GetCurrentProcessId()]
	if !ok {
		return "?"
	}
	if p, ok := procs[self.ParentProcessID]; ok {
		return windows.UTF16ToString(p.ExeFile[:])
	}
	return "? (pid " + strconv.Itoa(int(self.ParentProcessID)) + ", gone)"
}
