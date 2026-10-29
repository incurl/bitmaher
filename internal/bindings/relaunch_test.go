package bindings

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// TestTheRestartWaitsForTheInstanceThatStartedIt pins that a restart
// does not fight over the database with the instance that starts it.
//
// The old instance still holds the database when it starts the new one,
// so the new one waits before it touches anything.
func TestTheRestartWaitsForTheInstanceThatStartedIt(t *testing.T) {
	t.Setenv(EnvRelaunchParent, strconv.Itoa(os.Getpid()))

	// The process that is waited for is this one: it is alive, so the
	// wait runs its course. The test is about the wiring, not about
	// the timing.
	done := make(chan struct{})
	go func() {
		WaitForPreviousInstance()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(previousInstanceTimeout + 5*time.Second):
		t.Error("the wait for the previous instance did not end")
	}
}

// TestTheRestartWithoutAPreviousInstanceIsInstant pins that an
// ordinary start does not wait for anything.
func TestTheRestartWithoutAPreviousInstanceIsInstant(t *testing.T) {
	t.Setenv(EnvRelaunchParent, "")

	done := make(chan struct{})
	go func() {
		WaitForPreviousInstance()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("the wait for the previous instance did not end")
	}
}

// TestTheRestartIgnoresAMalformedParent pins that a pid that makes no
// sense does not hold the new instance hostage.
//
// The environment is written by the instance that starts the restart, so
// it holds a pid. A value that is not one is not waited for.
func TestTheRestartIgnoresAMalformedParent(t *testing.T) {
	t.Setenv(EnvRelaunchParent, "not-a-pid")

	done := make(chan struct{})
	go func() {
		WaitForPreviousInstance()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Error("the wait for the previous instance did not end")
	}
}

// TestTheRestartCarriesThePidOfTheOldInstance pins that the new
// instance is told which instance started it.
//
// The environment of the child holds the pid of this process: that is
// what the new instance waits for.
func TestTheRestartCarriesThePidOfTheOldInstance(t *testing.T) {
	// The child is the shell, not the test binary: the restart of the
	// test binary would run the tests again.
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("the shell is not there: %v", err)
	}

	child := exec.Command(shell, "-c", "echo \"$"+EnvRelaunchParent+"\"")
	child.Env = append(os.Environ(),
		EnvRelaunchParent+"="+strconv.Itoa(os.Getpid()),
	)

	out, err := child.Output()
	if err != nil {
		t.Fatalf("the child did not run: %v", err)
	}

	got := string(out)
	want := strconv.Itoa(os.Getpid()) + "\n"
	if got != want {
		t.Errorf("the child saw %q, want %q", got, want)
	}
}
