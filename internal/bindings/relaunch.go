package bindings

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// The restart of the application.
//
// The backend restarts the application, because it owns the sidecar and
// has to bring it up again: the engine is a child process, and a child
// process cannot survive the parent that starts it again.
//
// The new instance is started by the old one, on the way down. It is
// put in a process group of its own, so that the shutdown of the old
// one does not take it with it, and it waits for the old one to be
// gone before it touches the database.

// Relaunch starts the application again.
//
// The path of the executable is the one of the running process, and the
// arguments are the ones it was started with, so that a restart looks
// like the original start.
//
// The new instance is started, and this process returns at once: the
// caller ends the application, and the new one takes its place.
func Relaunch() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(),
		EnvRelaunchParent+"="+strconv.Itoa(os.Getpid()),
	)
	cmd.SysProcAttr = detachedProcessAttributes()

	if err := cmd.Start(); err != nil {
		return err
	}

	// The child is not waited for: it outlives this process on
	// purpose. Releasing it here keeps a zombie out of the table.
	go func() { _ = cmd.Wait() }()
	return nil
}

// The environment of a restart.
const (
	// EnvRelaunchParent holds the pid of the instance that started
	// this one.
	EnvRelaunchParent = "BITMAHER_RELAUNCH_PARENT"
)

// WaitForPreviousInstance waits for the instance that started this one
// to be gone.
//
// The old instance still holds the database when it starts the new one,
// and the two would fight over it. The new one waits here, before it
// touches anything, so that the database belongs to one process at a
// time.
//
// It is a no-op when this process was not started by a restart.
func WaitForPreviousInstance() {
	raw := os.Getenv(EnvRelaunchParent)
	if raw == "" {
		return
	}

	pid, err := strconv.Atoi(raw)
	if err != nil || pid <= 0 {
		return
	}

	waitForProcess(pid, previousInstanceTimeout)
}

// previousInstanceTimeout is how long the new instance waits for the
// old one.
//
// The old instance ends as soon as its shutdown is done, so the wait
// is short. The limit is there so that a process that does not go away
// cannot hold the new one hostage.
const previousInstanceTimeout = 15 * time.Second

// waitForProcess waits for a process to be gone.
//
// The process is looked for by its pid. A pid that does not exist
// belongs to a process that is already gone, and the wait is over.
func waitForProcess(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}
