//go:build !windows

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
//
// The session is started as well, so that the new instance survives the
// terminal the old one was started from.
func detachedProcessAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Setsid: true}
}

// processExists answers whether a process is still running.
//
// The signal 0 is the one that asks without sending anything: the
// error says that there is no such process.
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}
