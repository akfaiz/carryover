//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// attachConsole lets the GUI-subsystem binary print to the terminal it was
// started from (for the CLI subcommands).
func attachConsole() {
	if len(os.Args) < 2 {
		return
	}
	const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if r, _, _ := kernel32.NewProc("AttachConsole").Call(uintptr(attachParentProcess)); r == 0 {
		return
	}
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 {
		os.Stdout = os.NewFile(uintptr(h), "stdout")
	}
	if h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil && h != 0 {
		os.Stderr = os.NewFile(uintptr(h), "stderr")
	}
}

// showFatal shows a startup error in a message box when there is no console
// to print it to (the GUI build started from a shortcut or by Steam).
// Otherwise stderr already has it.
func showFatal(msg string) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	if hwnd, _, _ := kernel32.NewProc("GetConsoleWindow").Call(); hwnd != 0 {
		return
	}
	text, err := windows.UTF16PtrFromString(msg)
	if err != nil {
		return
	}
	title, _ := windows.UTF16PtrFromString("Carryover")
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONERROR)
}
