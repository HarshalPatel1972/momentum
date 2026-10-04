//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// detach makes the daemon outlive the IDE's MCP process (and its console/job).
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess,
		HideWindow:    true,
	}
}

// attachParentConsole makes CLI output visible when the GUI-subsystem exe is
// run from cmd/PowerShell. If output is already redirected, it does nothing.
func attachParentConsole() {
	if _, err := os.Stdout.Stat(); err == nil {
		return
	}
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := attach.Call(uintptr(^uint32(0))); r == 0 { // ATTACH_PARENT_PROCESS
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = out, out
	}
}
