package bindings

import (
	"fmt"
	"testing"

	"github.com/incurl/bitmaher/internal/pocketbase"
)

// TestTheInterfaceSizeIsStored verifies that the size of the interface
// survives the round trip through the settings.
//
// The frontend sends a factor, because that is what a scale is, and the
// backend stores a percentage, because that is what a person reads:
// "125%" means something, "1.25" does not.
func TestTheInterfaceSizeIsStored(t *testing.T) {
	updated, err := mergeUI(pocketbase.UI{Zoom: 100, Theme: "dark"}, map[string]any{
		"uiScale": 1.25,
	})
	if err != nil {
		t.Fatalf("merge the preferences: %v", err)
	}
	if updated.Zoom != 125 {
		t.Errorf("the zoom is %d, want 125", updated.Zoom)
	}
	if updated.Theme != "dark" {
		t.Errorf("the theme is %q, want dark", updated.Theme)
	}
}

// TestTheInterfaceSizeIsClamped verifies that a size outside the bounds
// is brought back into them.
func TestTheInterfaceSizeIsClamped(t *testing.T) {
	updated, err := mergeUI(pocketbase.UI{}, map[string]any{"uiScale": 9.0})
	if err != nil {
		t.Fatalf("merge the preferences: %v", err)
	}
	if updated.Zoom != maxZoom {
		t.Errorf("the zoom is %d, want %d", updated.Zoom, maxZoom)
	}

	updated, err = mergeUI(pocketbase.UI{}, map[string]any{"uiScale": 0.01})
	if err != nil {
		t.Fatalf("merge the preferences: %v", err)
	}
	if updated.Zoom != minZoom {
		t.Errorf("the zoom is %d, want %d", updated.Zoom, minZoom)
	}
}

// TestThePreferencesOfTheFrontendAreRead verifies that the store hands
// the frontend the size of the interface as a factor.
//
// The frontend's configuration is a factor, and the backend's setting is
// a percentage. The store is where the two meet.
func TestThePreferencesOfTheFrontendAreRead(t *testing.T) {
	service := newTestService(t)
	if err := service.settings.SetUI(pocketbase.UI{Zoom: 110, Theme: "dark"}); err != nil {
		t.Fatalf("store the preferences: %v", err)
	}
	if err := service.settings.SetAppConfig(map[string]any{"dir": "/downloads"}); err != nil {
		t.Fatalf("store the configuration: %v", err)
	}

	values, err := service.StoreRead("config.json")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}

	preferences, ok := values["preferences"].(map[string]any)
	if !ok {
		t.Fatalf("the store holds no preferences: %v", values)
	}

	if scale, ok := preferences["uiScale"].(float64); !ok || scale != 1.1 {
		t.Errorf("the size of the interface is %v, want 1.1", preferences["uiScale"])
	}

	// The configuration of the frontend comes back whole.
	if dir, ok := preferences["dir"].(string); !ok || dir != "/downloads" {
		t.Errorf("the download directory is %v, want /downloads", preferences["dir"])
	}
}

// sameValue compares two values that came out of a JSON round trip.
//
// A number that was an integer comes back as a float, because JSON has
// one kind of number. The comparison is therefore by value, not by
// type.
func sameValue(got, want any) bool {
	gotNumber, gotIsNumber := toFloat(got)
	wantNumber, wantIsNumber := toFloat(want)
	if gotIsNumber && wantIsNumber {
		return gotNumber == wantNumber
	}
	return fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
}

// toFloat returns the number a value holds, if it holds one.
func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}

// TestTheWholeConfigurationIsStored verifies that the configuration of
// the frontend survives a write and a read.
//
// The frontend keeps its whole configuration in one record, and the
// backend used to keep only the size of the interface and the theme.
// Every other field was dropped, so the settings of the user were lost
// on the next start.
func TestTheWholeConfigurationIsStored(t *testing.T) {
	service := newTestService(t)

	config := map[string]any{
		"uiScale":            1.25,
		"theme":              "dark",
		"dir":                "/home/user/Downloads",
		"maxDownloadLimit":   int64(1048576),
		"fileCategories":     []any{map[string]any{"name": "Documents", "dir": "/docs"}},
		"btMaxPeers":         50,
		"taskManualOrder":    map[string]any{"all": []any{"abc"}},
		"historyDirectories": []any{"/tmp"},
	}

	if err := service.StoreWrite("config.json", "preferences", config); err != nil {
		t.Fatalf("write the store: %v", err)
	}

	values, err := service.StoreRead("config.json")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}

	preferences, ok := values["preferences"].(map[string]any)
	if !ok {
		t.Fatalf("the store holds no preferences: %v", values)
	}

	// Every field has to come back, not only the ones the backend
	// understands.
	for key, want := range config {
		got, ok := preferences[key]
		if !ok {
			t.Errorf("the setting %q was lost", key)
			continue
		}
		if key == "uiScale" {
			// The size of the interface is stored as a
			// percentage, so it comes back as a factor.
			if scale, ok := got.(float64); !ok || scale != 1.25 {
				t.Errorf("the size of the interface is %v, want 1.25", got)
			}
			continue
		}
		if !sameValue(got, want) {
			t.Errorf("the setting %q is %v, want %v", key, got, want)
		}
	}

	// The backend's own record has to agree with the configuration.
	ui, err := service.settings.GetUI()
	if err != nil {
		t.Fatalf("get the interface preferences: %v", err)
	}
	if ui.Zoom != 125 {
		t.Errorf("the size of the interface is %d, want 125", ui.Zoom)
	}
	if ui.Theme != "dark" {
		t.Errorf("the theme is %q, want dark", ui.Theme)
	}
}
