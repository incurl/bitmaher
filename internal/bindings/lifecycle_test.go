package bindings

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestAppExitEndsTheApplication pins the way the application ends.
//
// The frontend used to ask a command that no backend answered, so the
// application never ended. The bound method is the one that ends it.
func TestAppExitEndsTheApplication(t *testing.T) {
	var quit atomic.Bool
	service := &AppService{}
	SetQuitter(service, func() { quit.Store(true) })

	service.AppExit(0)

	waitFor(t, &quit)
	if ExitCodeOf(service) != 0 {
		t.Errorf("the exit code is %d, want 0", ExitCodeOf(service))
	}
}

// TestAppRelaunchAsksForARestart pins the restart.
//
// The restart is the backend's business: the engine is a child process,
// and a child process cannot survive the parent that starts it again.
func TestAppRelaunchAsksForARestart(t *testing.T) {
	var quit atomic.Bool
	service := &AppService{}
	SetQuitter(service, func() { quit.Store(true) })

	service.AppRelaunch()

	waitFor(t, &quit)
	if ExitCodeOf(service) != ExitCodeRelaunch {
		t.Errorf("the exit code is %d, want %d", ExitCodeOf(service), ExitCodeRelaunch)
	}
}

// TestTheQuitRunsOffTheCallingGoroutine pins that the quit does not
// wait for the caller.
//
// The quit tears the webview down, so the caller's request never gets
// an answer. A quit that ran on the caller's goroutine would block
// that answer forever.
func TestTheQuitRunsOffTheCallingGoroutine(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	service := &AppService{}
	SetQuitter(service, func() {
		close(started)
		<-release
	})

	service.AppExit(0)

	// The call has returned while the quit is still running.
	<-started
	close(release)
}

// TestAnApplicationWithoutAQuitterSurvivesTheRequest pins that the
// request is harmless when the application is not running yet.
//
// The service is created before the Wails application exists, and the
// quitter is installed afterwards.
func TestAnApplicationWithoutAQuitterSurvivesTheRequest(t *testing.T) {
	service := &AppService{}

	service.AppExit(0)
	service.AppRelaunch()

	if ExitCodeOf(service) != 0 {
		t.Errorf("the exit code is %d, want 0", ExitCodeOf(service))
	}
}

// waitFor waits for a flag to be set.
func waitFor(t *testing.T, flag *atomic.Bool) {
	t.Helper()

	for i := 0; i < 100; i++ {
		if flag.Load() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("the application was not asked to quit")
}
