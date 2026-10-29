package bindings

// The lifecycle of the application: the way it ends, and the way it
// starts again.
//
// The frontend asks for the end through a bound method, because the
// webview cannot end the process on its own. The bound method calls the
// quit function the bootstrap installed, which is the Wails
// application's own quit.

// SetQuitter tells the service how to end the application.
//
// It is a function and not a method, because a method on the service
// would become a method of the frontend's API: the binding generator
// exposes every exported method of a bound service, and this one is
// not for the frontend.
//
// The quitter runs on its own goroutine. The quit tears the webview
// down, so the caller's request never gets an answer: the promise of
// the frontend is left hanging, and the application goes away with it.
func SetQuitter(service *AppService, quit func()) {
	if service == nil {
		return
	}
	service.quit = quit
}

// AppExit ends the application.
//
// The frontend asks for it when the user chooses to quit. The exit code
// is the one the process ends with; the desktop platforms only know a
// failure from a success, so the code is mostly bookkeeping.
//
// The call does not wait for the application to end: the webview is
// gone by the time it could answer.
func (s *AppService) AppExit(code int) {
	if s == nil {
		return
	}
	if s.console != nil {
		s.console.Info("app", "the application was asked to quit")
	}
	s.requestQuit(code)
}

// AppRelaunch restarts the application.
//
// The backend restarts the application, because it owns the sidecar and
// has to bring it up again: the engine is a child process, and a child
// process cannot survive the parent that starts it again.
//
// The new instance is started by the old one as it goes down, so that
// the two do not fight over the data directory and the port of the
// engine.
func (s *AppService) AppRelaunch() {
	if s == nil {
		return
	}
	if s.console != nil {
		s.console.Info("app", "the application was asked to restart")
	}
	s.requestQuit(ExitCodeRelaunch)
}

// ExitCodeRelaunch is the exit code of a restart.
//
// The application reads it at start-up, and starts itself again. It is
// a code of its own, so that a restart can tell itself apart from an
// ordinary exit.
const ExitCodeRelaunch = 42

// requestQuit asks the application to end.
//
// The exit code is recorded first, so that the shutdown knows how the
// application has to end.
func (s *AppService) requestQuit(code int) {
	if s == nil || s.quit == nil {
		return
	}

	s.exitCode = code
	go s.quit()
}

// ExitCodeOf returns the exit code the service was asked to end with.
//
// It is a function and not a method, because a method on the service
// would become a method of the frontend's API: the binding generator
// exposes every exported method of a bound service, and this one is
// not for the frontend.
//
// The shutdown reads it when the application ends, to tell a restart
// apart from an ordinary exit.
func ExitCodeOf(service *AppService) int {
	if service == nil {
		return 0
	}
	return service.exitCode
}
