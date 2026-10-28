//go:build windows

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
// goes down with it.
func processGroupAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// signalProcessGroup asks the sidecar to stop.
//
// Windows has no SIGTERM, so the process is killed. The engine is
// asked to stop over RPC first, so the session file is written before
// this happens.
func signalProcessGroup(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Kill()
}

// killProcessGroup stops the sidecar at once.
//
// It is the same as the polite request on Windows, which has no
// gentler way to stop a process that does not share the console.
func killProcessGroup(process *os.Process) error {
	if process == nil {
		return nil
	}
	return process.Kill()
}

// exitCodeOf returns the exit code of a process that has ended.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	return exitErr.ExitCode()
}
