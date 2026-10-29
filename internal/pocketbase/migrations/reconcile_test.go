package migrations

import (
	"encoding/json"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// TestTheTwoRecordsOfTheInterfaceAreReconciled verifies that the copy
// the backend keeps is brought into line with the configuration.
//
// The read path used to hand the frontend the size of the copy over the
// one of the configuration, so a database that was written before the
// two were reconciled can hold two different sizes. The configuration
// wins: it is the one the user sees.
func TestTheTwoRecordsOfTheInterfaceAreReconciled(t *testing.T) {
	app := newTestApp(t)

	// The configuration says 150, and the copy says 90.
	writeSetting(t, app, settingAppConfig, map[string]any{
		"uiScale": 1.5,
		"theme":   "dark",
	})
	writeSetting(t, app, settingUI, map[string]any{
		"zoom":  90,
		"theme": "light",
	})

	if err := reconcileInterfaceRecords(app); err != nil {
		t.Fatalf("reconcile the records: %v", err)
	}

	// The copy now holds the size of the configuration.
	copied := readSetting(t, app, settingUI)
	if copied["zoom"] != 150.0 {
		t.Errorf("the zoom of the copy is %v, want 150", copied["zoom"])
	}

	// The configuration is untouched: it is the record of truth.
	config := readSetting(t, app, settingAppConfig)
	if config["uiScale"] != 1.5 {
		t.Errorf("the size of the interface is %v, want 1.5", config["uiScale"])
	}
	if config["theme"] != "dark" {
		t.Errorf("the theme is %v, want dark", config["theme"])
	}
}

// TestTheRecordsThatAgreeAreLeftAlone verifies that a database that is
// already in order is not written to.
func TestTheRecordsThatAgreeAreLeftAlone(t *testing.T) {
	app := newTestApp(t)

	writeSetting(t, app, settingAppConfig, map[string]any{"uiScale": 1.25})
	writeSetting(t, app, settingUI, map[string]any{"zoom": 125})

	if err := reconcileInterfaceRecords(app); err != nil {
		t.Fatalf("reconcile the records: %v", err)
	}

	copied := readSetting(t, app, settingUI)
	if copied["zoom"] != 125.0 {
		t.Errorf("the zoom of the copy is %v, want 125", copied["zoom"])
	}
}

// TestAConfigurationThatIsNotThereIsHarmless verifies that the
// reconciliation does not fail on a database that has no configuration.
func TestAConfigurationThatIsNotThereIsHarmless(t *testing.T) {
	app := newTestApp(t)

	writeSetting(t, app, settingUI, map[string]any{"zoom": 90})

	if err := reconcileInterfaceRecords(app); err != nil {
		t.Fatalf("reconcile the records: %v", err)
	}
}

// newTestApp creates a PocketBase application with the BitMaher schema.
//
// The migrations package cannot import the pocketbase package: that one
// imports this one, to register the migrations. The application is
// therefore built here, and the migrations are applied by hand.
func newTestApp(t *testing.T) core.App {
	t.Helper()

	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: t.TempDir(),
	})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = app.ResetBootstrapState()
	})

	if err := app.RunAppMigrations(); err != nil {
		t.Fatalf("run the migrations: %v", err)
	}
	return app
}

// writeSetting writes a value of the app_settings collection.
func writeSetting(t *testing.T, app core.App, key string, value map[string]any) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode the setting: %v", err)
	}

	record := findSetting(app, key)
	if record == nil {
		collection, err := app.FindCollectionByNameOrId("app_settings")
		if err != nil {
			t.Fatalf("find the collection: %v", err)
		}
		record = core.NewRecord(collection)
		record.Set("key", key)
		record.Set("encrypted", false)
	}
	record.Set("value", json.RawMessage(raw))

	if err := app.Save(record); err != nil {
		t.Fatalf("save the setting: %v", err)
	}
}

// readSetting reads a value of the app_settings collection.
func readSetting(t *testing.T, app core.App, key string) map[string]any {
	t.Helper()

	record := findSetting(app, key)
	if record == nil {
		t.Fatalf("the setting %q is not there", key)
	}

	values, err := rawObject(record.Get("value"))
	if err != nil {
		t.Fatalf("read the setting: %v", err)
	}
	return values
}
