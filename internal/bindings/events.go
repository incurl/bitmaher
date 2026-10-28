package bindings

import (
	"github.com/incurl/bitmaher/internal/console"
	"github.com/incurl/bitmaher/internal/pump"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventConsoleEntry is emitted whenever a debug console entry is added.
// The payload is a console.Entry.
const EventConsoleEntry = "console:entry"

// EventFrontendLog is emitted by the frontend when it writes to the log.
//
// The payload is a console.Entry, so that the entry of the frontend
// lands in the same console as the entries of the backend. The name is
// its own, because the frontend sends entries and the backend sends
// them back.
const EventFrontendLog = "frontend:log"

// EventBridge pushes the debug console entries to the frontend as
// Wails events.
type EventBridge struct {
	app *application.App
}

// NewEventBridge creates an event bridge.
func NewEventBridge(app *application.App) *EventBridge {
	return &EventBridge{app: app}
}

// EmitConsoleEntry implements console.Emitter.
func (b *EventBridge) EmitConsoleEntry(entry console.Entry) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventConsoleEntry, entry)
}

// LogEntry is a log entry that the frontend sends.
//
// The frontend keeps its own log, and writes the entries to the
// backend so that they land in the same debug console as the entries
// of the backend itself.
type LogEntry struct {
	// Level is the severity of the entry.
	Level string `json:"level"`
	// Message is the human readable text.
	Message string `json:"message"`
}

// ListenConsoleEntries records the log entries of the frontend.
//
// It is called once the Wails application exists. The returned function
// stops the listener.
func (b *EventBridge) ListenConsoleEntries(log *console.Log) func() {
	if b == nil || b.app == nil || log == nil {
		return func() {}
	}

	return b.app.Event.On(EventFrontendLog, func(event *application.CustomEvent) {
		entry, ok := event.Data.(console.Entry)
		if !ok || entry.Message == "" {
			return
		}

		switch entry.Level {
		case console.LevelError:
			log.Error(entry.Source, entry.Message)
		case console.LevelWarn:
			log.Warn(entry.Source, entry.Message)
		default:
			log.Info(entry.Source, entry.Message)
		}
	})
}

// EmitSnapshot implements pump.Emitter.
//
// The snapshot carries the whole state of the engine: the global
// statistics and every download. The frontend replaces its list with it,
// so it is the one source of truth for the task list.
func (b *EventBridge) EmitSnapshot(snapshot pump.Snapshot) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(pump.EventSnapshot, snapshot)
}
