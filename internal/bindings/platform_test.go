package bindings

import (
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
}
