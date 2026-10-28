//go:build !windows

package aria2

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// processGroupAttributes puts the sidecar in its own process group.
//
// The engine is then stopped as a group, so that any child it started
// goes down with it. On Unix a process group is signalled with a
// negative pid.
func processGroupAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// signalProcessGroup asks the sidecar to stop.
//
// SIGTERM is the polite request: the engine finishes what it is doing
// and exits.
func signalProcessGroup(process *os.Process) error {
	if process == nil {
		return nil
	}
	// A negative pid addresses the whole process group.
	return syscall.Kill(-process.Pid, syscall.SIGTERM)
}

// killProcessGroup stops the sidecar at once.
//
// SIGKILL cannot be caught, so it is the last resort.
func killProcessGroup(process *os.Process) error {
	if process == nil {
		return nil
	}
	return syscall.Kill(-process.Pid, syscall.SIGKILL)
}

// exitCodeOf returns the exit code of a process that has ended.
//
// A process that was signalled has no exit code of its own, so the
// signal is reported instead: it is what tells the user whether the
// sidecar was stopped or crashed.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}

	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			return 128 + int(status.Signal())
		}
		return status.ExitStatus()
	}
	return exitErr.ExitCode()
}
