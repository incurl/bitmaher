package bindings

import (
	"testing"
)

// TestTheServiceWithoutAWindowSurvivesTheRelease pins that the release
// is harmless when the application runs without a window.
//
// The service is created before the window exists, and the window is
// given to it afterwards.
func TestTheServiceWithoutAWindowSurvivesTheRelease(t *testing.T) {
	service := &AppService{}

	service.ReleaseWindowClose()
	service.MinimiseWindowToTray()
}

// TestTheCloserWithoutAWindowIsHarmless pins that the interception
// needs a window.
//
// There is nothing to hold without one.
func TestTheCloserWithoutAWindowIsHarmless(t *testing.T) {
	if closer := InterceptWindowClose(nil, nil); closer != nil {
		t.Error("the closer was created without a window")
	}
}

// TestTheCloserReleasesTheCloseItHolds pins that a held close can be
// released.
//
// The release is what lets the window go away once the user has been
// asked what to do with it.
func TestTheCloserReleasesTheCloseItHolds(t *testing.T) {
	closer := &WindowCloser{}

	closer.held.Store(true)
	if !closer.held.Load() {
		t.Error("the close was not held")
	}

	closer.Release()
	if closer.held.Load() {
		t.Error("the close was not released")
	}
}

// TestTheCloserStopsHoldingTheClose pins that the shutdown releases a
// close that is still held.
//
// The quit must not be held by a question that nobody is there to
// answer.
func TestTheCloserStopsHoldingTheClose(t *testing.T) {
	closer := &WindowCloser{}
	closer.held.Store(true)

	closer.Stop()

	if closer.held.Load() {
		t.Error("the close was still held after the stop")
	}
}

// TestANilCloserIsHarmless pins that the release of a closer that was
// never built does not crash.
func TestANilCloserIsHarmless(t *testing.T) {
	var closer *WindowCloser

	closer.Release()
	closer.Stop()
}
