package bindings

import (
	"github.com/incurl/bitmaher/internal/console"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventConsoleEntry is emitted whenever a debug console entry is added.
// The payload is a console.Entry.
const EventConsoleEntry = "console:entry"

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
