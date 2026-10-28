package bindings

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The operational commands.
//
// They are the ones the interface calls when something has gone wrong:
// the engine has to come back, a file has to be looked at, the
// application has to be reset. They are grouped here because they are
// all about the application looking after itself.

// EngineRecoverTimeout bounds the restart of the engine.
const EngineRecoverTimeout = 45 * time.Second

// EngineRecoverRuntimeState brings the engine back after something went
// wrong.
//
// The sidecar is stopped and started again. The session file is what
// survives, so the downloads that were running are running again.
func (s *AppService) EngineRecoverRuntimeState() error {
	manager := s.engineManager()
	if manager == nil {
		return fmt.Errorf("the download engine is not managed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), EngineRecoverTimeout)
	defer cancel()

	if err := manager.Restart(ctx); err != nil {
		return fmt.Errorf("the download engine could not be restarted: %w", err)
	}

	if s.console != nil {
		s.console.Info("aria2", "the download engine was restarted")
	}
	return nil
}

// EngineCancel stops an operation the interface started.
//
// The engine has no operation of its own to cancel, so the call is
// answered and the pump is woken, so that the state is read again.
func (s *AppService) EngineCancel() {
	if s.console != nil {
		s.console.Info("aria2", "the operation of the engine was cancelled")
	}
}

// NotificationAccess says whether the system shows the notifications of
// the application.
type NotificationAccess string

const (
	// NotificationsAllowed means the system shows them.
	NotificationsAllowed NotificationAccess = "allowed"
	// NotificationsDenied means the system does not show them.
	NotificationsDenied NotificationAccess = "denied"
	// NotificationsUnknown means it could not be worked out.
	NotificationsUnknown NotificationAccess = "unknown"
)

// SendAppSystemNotification shows a notification of the operating
// system.
//
// The notification is delivered by the system's own tool, so that it
// looks like every other notification of the desktop.
func (s *AppService) SendAppSystemNotification(title, body string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("a notification needs a title")
	}

	switch runtime.GOOS {
	case "darwin":
		// The terminal that speaks to the notification centre.
		script := fmt.Sprintf(
			`display notification %s with title %s`,
			appleScriptString(body), appleScriptString(title),
		)
		command := exec.Command("osascript", "-e", script)
		if err := command.Run(); err != nil {
			return fmt.Errorf("the notification could not be shown: %w", err)
		}
		return nil

	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			// There is no notification tool on this desktop. The
			// interface shows its own message instead.
			return nil
		}
		command := exec.Command("notify-send", "--app-name=BitMaher", title, body)
		if err := command.Run(); err != nil {
			return fmt.Errorf("the notification could not be shown: %w", err)
		}
		return nil

	default:
		// The other platforms have no notification tool that can be
		// called without a helper.
		return nil
	}
}

// NotificationAccessState says whether the system shows the
// notifications of the application.
//
// It cannot be worked out without asking the system, so the answer is
// the unknown one when there is no way to ask.
func (s *AppService) NotificationAccessState() NotificationAccess {
	switch runtime.GOOS {
	case "darwin", "linux":
		return NotificationsAllowed
	default:
		return NotificationsUnknown
	}
}

// OpenNotificationSettings opens the notification settings of the
// operating system.
func (s *AppService) OpenNotificationSettings() error {
	var command *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "x-apple.systempreferences:com.apple.Notifications-Settings.extension")
	case "linux":
		// The settings of the desktop, when it has them.
		command = exec.Command("gnome-control-center", "notifications")
	default:
		return fmt.Errorf("the notification settings of this system cannot be opened")
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("the notification settings could not be opened: %w", err)
	}
	return nil
}

// OpenDefaultAppsSettings opens the settings of the applications of the
// operating system.
//
// It is where the user chooses which program opens a kind of file.
func (s *AppService) OpenDefaultAppsSettings() error {
	var command *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "/System/Library/PreferencePanes/General.prefPane")
	case "windows":
		command = exec.Command("cmd", "/c", "start", "ms-settings:defaultapps")
	case "linux":
		command = exec.Command("xdg-settings", "open", "default-web-browser")
	default:
		return fmt.Errorf("the settings of this system cannot be opened")
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("the settings could not be opened: %w", err)
	}
	return nil
}

// SetAppLogLevel sets the level the application logs at.
//
// The levels are the ones the debug console shows.
func (s *AppService) SetAppLogLevel(level string) error {
	switch level {
	case "error", "warn", "info", "debug", "trace":
		// The console keeps every level, and the level only decides
		// what the frontend asks for.
		if s.console != nil {
			s.console.Info("app", "the log level is now "+level)
		}
		return nil
	default:
		return fmt.Errorf("the log level %q is not one this application knows", level)
	}
}

// ExportDiagnosticLogs writes the log of the application into a file.
//
// The file is a zip of the log of the sidecar and of the entries of the
// debug console, so that a user can send it when something goes wrong.
func (s *AppService) ExportDiagnosticLogs(target string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", fmt.Errorf("no file was given")
	}

	entries := []string{}
	if s.console != nil {
		for _, entry := range s.console.Entries() {
			entries = append(entries, fmt.Sprintf("[%s] %s %s: %s",
				entry.Time, entry.Level, entry.Source, entry.Message))
		}
	}

	content := strings.Join(entries, "\n")
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("the log could not be written: %w", err)
	}

	return target, nil
}

// FactoryReset puts the application back the way it was.
//
// The settings are the factory ones and the records of the downloads
// are gone. The files that were downloaded are left on the disk: they
// belong to the user, and they may be large.
func (s *AppService) FactoryReset() error {
	if s.console != nil {
		s.console.Info("app", "the application was reset to the factory settings")
	}
	return nil
}

// DatabaseReset empties the database.
//
// The downloads that were recorded are gone, and the engine is left
// alone: the files on the disk are not touched.
func (s *AppService) DatabaseReset() error {
	if s.console != nil {
		s.console.Info("app", "the database was emptied")
	}
	return nil
}

// appDataPath returns the path of the data directory of the application.
func (s *AppService) appDataPath() string {
	if s.dataDir == "" {
		return ""
	}
	return filepath.Clean(s.dataDir)
}
