// Package launcher starts games and waits for them to exit.
package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Spec describes how to start a game.
type Spec struct {
	Exe     string
	Args    []string
	WorkDir string
	Env     map[string]string
	// WaitProcess, if set, is a process name (e.g. "Game-Win64-Shipping.exe")
	// to wait for after Exe exits — for games whose launcher exits immediately.
	WaitProcess string
}

// Result of a game run.
type Result struct {
	ExitCode int
	Duration time.Duration
}

// Run starts the game and blocks until it (and WaitProcess, if set) exits.
func Run(ctx context.Context, spec Spec, onStarted func(pid int)) (Result, error) {
	if spec.Exe == "" {
		return Result{}, errors.New("no executable configured for this device")
	}
	cmd := exec.Command(spec.Exe, spec.Args...)
	cmd.Dir = spec.WorkDir
	if cmd.Dir == "" && filepath.IsAbs(spec.Exe) {
		cmd.Dir = filepath.Dir(spec.Exe)
	}
	cmd.Env = os.Environ()
	for k, v := range spec.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	configureCmd(cmd)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start %s: %w", spec.Exe, err)
	}
	if onStarted != nil {
		onStarted(cmd.Process.Pid)
	}
	err := cmd.Wait()
	res := Result{ExitCode: 0}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		res.ExitCode = exitErr.ExitCode()
		err = nil
	}
	if err != nil {
		return res, err
	}
	if spec.WaitProcess != "" {
		waitForProcess(ctx, spec.WaitProcess, 90*time.Second)
	}
	res.Duration = time.Since(start)
	return res, nil
}

// waitForProcess waits up to appear for a process named name to show up, then
// until no such process is running.
func waitForProcess(ctx context.Context, name string, appear time.Duration) {
	deadline := time.Now().Add(appear)
	seen := false
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		running := processRunning(name)
		if running {
			seen = true
		}
		if seen && !running {
			return
		}
		if !seen && time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func processRunning(name string) bool {
	procs, err := process.Processes()
	if err != nil {
		return false
	}
	for _, p := range procs {
		n, err := p.Name()
		if err == nil && nameMatches(n, name) {
			return true
		}
	}
	return false
}

// linuxCommLen is the longest process name Linux reports (TASK_COMM_LEN - 1).
const linuxCommLen = 15

// nameMatches compares a process name with the wanted one, ignoring case. The
// Linux kernel cuts names to 15 characters, and under Wine/Proton the full
// name can't be recovered from the command line (it is a Windows path), so a
// cut name matches when it is a prefix of the wanted one.
func nameMatches(got, want string) bool {
	if strings.EqualFold(got, want) {
		return true
	}
	if len(got) == linuxCommLen && len(want) > linuxCommLen {
		return strings.EqualFold(got, want[:linuxCommLen])
	}
	return false
}

// SplitArgs splits a command line into arguments, honouring double and single
// quotes and backslash escapes of quotes.
func SplitArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg := false
	var quote rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else if r == '\\' && quote == '"' && i+1 < len(rs) && (rs[i+1] == '"' || rs[i+1] == '\\') {
				i++
				cur.WriteRune(rs[i])
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inArg = true
		case r == ' ' || r == '\t' || r == '\n':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote in arguments")
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
