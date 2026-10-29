//go:build windows

package bindings

import (
	"os"
	"syscall"
)

// detachedProcessAttributes puts the restarted application in a process
// group of its own.
//
// The group is what keeps the new instance from being killed by the
// shutdown of the old one: the shutdown signals the group it is in, and
// the new one is not in it.
func detachedProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// processExists answers whether a process is still running.
//
// Windows has no signal 0, so the handle of the process is asked: an
// open handle means that the process is there.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}

	// The exit code of a process that is running is still the constant
	// one the system gives.
	const stillActive = 259
	return code == stillActive
}
