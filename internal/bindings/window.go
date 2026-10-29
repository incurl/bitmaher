package bindings

import (
	"sync/atomic"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The close of the window.
//
// The interface asks the user what to do when the window is closed: the
// window goes away, or the application goes with it. The question is
// asked in the frontend, so the backend has to hold the close until the
// answer is known.
//
// Wails closes the window on its own when it is asked to. The hook
// below stands in front of that: it holds the close, and the frontend
// releases it when the user has chosen.

// EventWindowCloseRequest is emitted when the window is asked to close.
//
// The frontend listens for it, shows its question, and either closes
// the window or lets it go.
const EventWindowCloseRequest = "window:close-request"

// WindowCloser holds the close of the window until the frontend has
// released it.
//
// The held flag is atomic, because the hook and the release run on
// different goroutines: the hook runs on the thread of the window
// manager, and the release comes from the frontend's request.
type WindowCloser struct {
	// held says whether the close is still being held.
	held atomic.Bool
	// window is the window the closer holds.
	window *application.WebviewWindow
	// stop ends the interception.
	stop func()
}

// InterceptWindowClose holds the close of the window until the
// frontend has asked for it.
//
// The frontend listens for the close request and answers it. When it
// wants the window to go away, it calls the backend, which releases the
// close.
//
// The returned function stops the interception and releases a close
// that is still held. It is used when the application quits: the quit
// must not be held by a question that nobody is there to answer.
func InterceptWindowClose(app *application.App, window *application.WebviewWindow) *WindowCloser {
	if app == nil || window == nil {
		return nil
	}

	closer := &WindowCloser{window: window}
	closer.held.Store(true)

	closer.stop = window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !closer.held.Load() {
			// The close has been released: it goes on.
			return
		}

		// The close is held. The window stays open, and the frontend
		// is asked what to do.
		event.Cancel()
		app.Event.Emit(EventWindowCloseRequest, nil)
	})

	return closer
}

// Release lets a held close go on.
//
// The frontend calls it when the user has chosen to close the
// application. The window closes, and the application goes with it.
func (c *WindowCloser) Release() {
	if c == nil {
		return
	}
	c.held.Store(false)
	if c.window != nil {
		c.window.Close()
	}
}

// Stop ends the interception.
//
// A close that is still held is released first, so that the window is
// not left behind when the application quits.
func (c *WindowCloser) Stop() {
	if c == nil {
		return
	}
	c.held.Store(false)
	if c.stop != nil {
		c.stop()
	}
}

// SetWindowCloser tells the service how to release the close of the
// window.
//
// It is a function and not a method, because a method on the service
// would become a method of the frontend's API: the binding generator
// exposes every exported method of a bound service, and this one is
// not for the frontend.
func SetWindowCloser(service *AppService, closer *WindowCloser) {
	if service == nil {
		return
	}
	service.closer = closer
}

// ReleaseWindowClose lets the close of the window go on.
//
// The frontend calls it when the user has chosen to close the
// application. The window closes, and the application goes with it.
func (s *AppService) ReleaseWindowClose() {
	if s == nil {
		return
	}
	s.closer.Release()
}

// MinimiseWindowToTray hides the window and keeps the application
// running.
//
// The engine keeps transferring, and the window comes back when the
// user asks for it.
func (s *AppService) MinimiseWindowToTray() {
	if s == nil || s.window == nil {
		return
	}
	s.window.Hide()
}
