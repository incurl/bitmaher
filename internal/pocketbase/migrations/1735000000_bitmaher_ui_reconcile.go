package migrations

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// init registers the migration that reconciles the two records of the
// interface.
//
// The file name sorts after 1734000000_bitmaher_http_auth.go, so it
// runs last.
func init() {
	registerMigration("1735000000_bitmaher_ui_reconcile.go", func(app core.App) error {
		return reconcileInterfaceRecords(app)
	})
}

// The keys of the two records.
const (
	// settingAppConfig holds the whole configuration of the frontend.
	settingAppConfig = "app.config"
	// settingUI holds the copy of the interface preferences that the
	// backend keeps for itself.
	settingUI = "ui"
)

// reconcileInterfaceRecords makes the two records of the interface hold
// the same size and the same theme.
//
// The configuration is the record of truth, and the `ui` record is the
// copy the backend keeps for itself. The read path used to hand the
// frontend the size of the `ui` record over the one of the
// configuration, so a database that was written before the two were
// reconciled can hold two different sizes.
//
// The configuration wins: it is the one the frontend wrote, and the one
// the user sees. The copy is brought into line with it.
func reconcileInterfaceRecords(app core.App) error {
	config := findSetting(app, settingAppConfig)
	if config == nil {
		// The configuration was never written. The copy is all
		// there is, and it stays.
		return nil
	}

	values, err := rawObject(config.Get("value"))
	if err != nil {
		return fmt.Errorf("read the configuration: %w", err)
	}

	ui := findSetting(app, settingUI)
	if ui == nil {
		// There is no copy yet. It is written the next time the
		// frontend saves its preferences.
		return nil
	}

	copied, err := rawObject(ui.Get("value"))
	if err != nil {
		return fmt.Errorf("read the interface preferences: %w", err)
	}

	// The size of the interface: the configuration holds a factor,
	// and the copy holds a percentage.
	scale, hasScale := values["uiScale"].(float64)
	if !hasScale || scale <= 0 {
		return nil
	}
	zoom := int(scale*100 + 0.5)

	if copied["zoom"] == float64(zoom) {
		return nil
	}
	copied["zoom"] = float64(zoom)

	raw, err := json.Marshal(copied)
	if err != nil {
		return fmt.Errorf("encode the interface preferences: %w", err)
	}
	ui.Set("value", json.RawMessage(raw))

	if err := app.Save(ui); err != nil {
		return fmt.Errorf("save the interface preferences: %w", err)
	}
	return nil
}

// findSetting returns the record of a setting, or nil when there is
// none.
func findSetting(app core.App, key string) *core.Record {
	record, err := app.FindFirstRecordByFilter(
		"app_settings",
		"key = {:key}",
		map[string]any{"key": key},
	)
	if err != nil {
		return nil
	}
	return record
}

// rawObject reads a stored value as an object.
//
// The value is stored as it was given, and it comes back as the
// database has it: bytes, a string or a document. A value that is not
// an object is an empty one: the record is reconciled anyway, and the
// copy is written from the configuration.
func rawObject(value any) (map[string]any, error) {
	raw, err := rawJSON(value)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}, nil
	}

	values := map[string]any{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

// rawJSON reads a stored value as the bytes it was written as.
func rawJSON(value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	case json.RawMessage:
		return typed, nil
	default:
		return json.Marshal(value)
	}
}
