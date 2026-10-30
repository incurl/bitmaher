package bindings

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/incurl/bitmaher/internal/pocketbase"
)

// TestShowInFolderNeedsAPath verifies that a path is asked for.
//
// The command that answers nothing would be read as a success.
func TestShowInFolderNeedsAPath(t *testing.T) {
	service := newTestService(t)

	if err := service.ShowInFolder(""); err == nil {
		t.Error("an empty path should be refused")
	}
}

// TestShowInFolderRefusesAPathThatIsNotThere verifies that a path that
// does not exist is refused.
//
// The frontend asks for the folder of a file that may have been
// deleted, and the answer has to say so.
func TestShowInFolderRefusesAPathThatIsNotThere(t *testing.T) {
	service := newTestService(t)

	missing := filepath.Join(t.TempDir(), "not-there")
	if err := service.ShowInFolder(missing); err == nil {
		t.Error("a path that is not there should be refused")
	}
}

// TestOpenPathNeedsAPath verifies that a path is asked for.
func TestOpenPathNeedsAPath(t *testing.T) {
	service := newTestService(t)

	if err := service.OpenPath(""); err == nil {
		t.Error("an empty path should be refused")
	}
}

// TestOpenPathRefusesAPathThatIsNotThere verifies that a path that does
// not exist is refused.
func TestOpenPathRefusesAPathThatIsNotThere(t *testing.T) {
	service := newTestService(t)

	missing := filepath.Join(t.TempDir(), "not-there")
	if err := service.OpenPath(missing); err == nil {
		t.Error("a path that is not there should be refused")
	}
}

// TestRevealingAFolderOpensTheFolderItself verifies the two shapes of
// the reveal.
//
// A torrent's content lives in a folder of its own, and the folder is
// what the user asked to see. Revealing it must not open the folder
// that holds it: that would be the wrong folder.
func TestRevealingAFolderOpensTheFolderItself(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "Big Buck Bunny")

	program, args := revealCommand(folder, true)

	if runtime.GOOS == "darwin" {
		if program != "open" {
			t.Errorf("program = %q, want open", program)
		}
		if len(args) != 1 || args[0] != folder {
			t.Errorf("args = %v, want the folder itself", args)
		}
		return
	}

	if runtime.GOOS == "windows" {
		if program != "explorer" {
			t.Errorf("program = %q, want explorer", program)
		}
		if len(args) != 1 || args[0] != folder {
			t.Errorf("args = %v, want the folder itself", args)
		}
		return
	}

	if program != "xdg-open" {
		t.Errorf("program = %q, want xdg-open", program)
	}
	if len(args) != 1 || args[0] != folder {
		t.Errorf("args = %v, want the folder itself", args)
	}
}

// TestRevealingAFileOpensTheFolderThatHoldsIt verifies that a file is
// revealed through the folder that holds it.
//
// The folder of the file is what the file manager shows, and the file
// is marked in it where the operating system allows it.
func TestRevealingAFileOpensTheFolderThatHoldsIt(t *testing.T) {
	file := filepath.Join(t.TempDir(), "movie.mkv")

	program, args := revealCommand(file, false)

	switch runtime.GOOS {
	case "darwin":
		if program != "open" {
			t.Errorf("program = %q, want open", program)
		}
		if len(args) != 2 || args[0] != "-R" || args[1] != file {
			t.Errorf("args = %v, want the file marked", args)
		}
	case "windows":
		if program != "explorer" {
			t.Errorf("program = %q, want explorer", program)
		}
		if len(args) != 2 || args[1] != file {
			t.Errorf("args = %v, want the file selected", args)
		}
	default:
		if program != "xdg-open" {
			t.Errorf("program = %q, want xdg-open", program)
		}
		if len(args) != 1 || args[0] != filepath.Dir(file) {
			t.Errorf("args = %v, want the folder that holds the file", args)
		}
	}
}

// TestOpeningAPathUsesTheProgramOfTheSystem verifies that opening a
// path and revealing it are two different things.
//
// Opening a file must start the program that reads it, and not the file
// manager: the two commands were the same one, and the file was never
// opened.
func TestOpeningAPathUsesTheProgramOfTheSystem(t *testing.T) {
	file := filepath.Join(t.TempDir(), "movie.mkv")

	program, args := openCommand(file)

	switch runtime.GOOS {
	case "darwin":
		if program != "open" {
			t.Errorf("program = %q, want open", program)
		}
	case "windows":
		if program != "rundll32" {
			t.Errorf("program = %q, want rundll32", program)
		}
	default:
		if program != "xdg-open" {
			t.Errorf("program = %q, want xdg-open", program)
		}
	}

	if len(args) == 0 || args[len(args)-1] != file {
		t.Errorf("args = %v, want the path itself", args)
	}

	// The reveal of a file opens the folder that holds it on the
	// desktop, so the two commands cannot be the same one there.
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		revealProgram, revealArgs := revealCommand(file, false)
		if revealProgram == program && revealArgs[0] == args[0] {
			t.Error("opening a path and revealing it should not be the same command")
		}
	}
}

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
	if err := service.StoreWrite("config.json", "preferences", map[string]any{
		"uiScale": 1.1,
		"theme":   "dark",
		"dir":     "/downloads",
	}); err != nil {
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

// TestTheNeutralAnswersAreFalsy pins the answers of the features
// BitMaher does not have.
//
// The runtime answers a method that returns nothing with an empty
// object, and an empty object is **true** in JavaScript. The frontend
// would then read the answer of a question about a feature BitMaher
// does not have as a yes, and act on it: the window hid itself at
// start-up, because the question about the autostart was answered with
// a yes.
//
// This is the mistake that made the window flash and disappear.
func TestTheNeutralAnswersAreFalsy(t *testing.T) {
	service := &AppService{}

	if service.NeutralBool() {
		t.Error("the neutral boolean is true")
	}
	if service.UnsupportedCommand("some_command") {
		t.Error("the answer of an unsupported command is true")
	}

	// The value answer has to be a typed nil, and not the nil of an
	// interface: the runtime turns the nil of an interface into an
	// empty object, and an empty object is true in JavaScript.
	if value := service.NeutralValue(); value != nil {
		t.Errorf("the neutral value is %v, want nothing", value)
	}
}

// TestTheTwoRecordsOfTheInterfaceCannotDisagree verifies that the
// configuration and the `ui` record hold the same size of the
// interface.
//
// The configuration is the record of truth, and the `ui` record is the
// copy the backend keeps for itself. A copy that is read over the
// original would be able to disagree with it, and the size of the
// interface would then depend on which of them the answer came from.
func TestTheTwoRecordsOfTheInterfaceCannotDisagree(t *testing.T) {
	service := newTestService(t)

	if err := service.StoreWrite("config.json", "preferences", map[string]any{
		"uiScale": 1.25,
		"theme":   "light",
		"locale":  "de",
	}); err != nil {
		t.Fatalf("write the store: %v", err)
	}

	// The copy holds the size of the interface.
	ui, err := service.settings.GetUI()
	if err != nil {
		t.Fatalf("read the interface preferences: %v", err)
	}
	if ui.Zoom != 125 {
		t.Errorf("the zoom of the copy is %d, want 125", ui.Zoom)
	}
	if ui.Theme != "light" {
		t.Errorf("the theme of the copy is %q, want light", ui.Theme)
	}

	// The original holds it too, and the store hands it back as it is.
	values, err := service.StoreRead("config.json")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}
	preferences, ok := values["preferences"].(map[string]any)
	if !ok {
		t.Fatalf("the store holds no preferences: %v", values)
	}
	if preferences["uiScale"] != 1.25 {
		t.Errorf("the size of the interface is %v, want 1.25", preferences["uiScale"])
	}
	if preferences["theme"] != "light" {
		t.Errorf("the theme is %v, want light", preferences["theme"])
	}
	if preferences["locale"] != "de" {
		t.Errorf("the locale is %v, want de", preferences["locale"])
	}
}

// TestTheSizeOfTheInterfaceIsReadFromTheConfiguration verifies that
// the store hands the frontend the size it stored.
//
// The backend keeps a copy of the size for itself, and the copy is
// never read over the configuration.
func TestTheSizeOfTheInterfaceIsReadFromTheConfiguration(t *testing.T) {
	service := newTestService(t)

	if err := service.StoreWrite("config.json", "preferences", map[string]any{
		"uiScale": 1.5,
	}); err != nil {
		t.Fatalf("write the store: %v", err)
	}

	values, err := service.StoreRead("config.json")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}
	preferences := values["preferences"].(map[string]any)
	if preferences["uiScale"] != 1.5 {
		t.Errorf("the size of the interface is %v, want 1.5", preferences["uiScale"])
	}
}

// TestTheThemeThatFollowsTheSystemIsKept verifies that the theme
// "auto" does not empty the copy.
//
// The frontend decides what "auto" looks like, and the copy keeps the
// last concrete theme, so that the backend has one to apply before the
// interface is loaded.
func TestTheThemeThatFollowsTheSystemIsKept(t *testing.T) {
	service := newTestService(t)

	if err := service.StoreWrite("config.json", "preferences", map[string]any{
		"uiScale": 1.0,
		"theme":   "dark",
	}); err != nil {
		t.Fatalf("write the store: %v", err)
	}
	if err := service.StoreWrite("config.json", "preferences", map[string]any{
		"uiScale": 1.0,
		"theme":   "auto",
	}); err != nil {
		t.Fatalf("write the store: %v", err)
	}

	ui, err := service.settings.GetUI()
	if err != nil {
		t.Fatalf("read the interface preferences: %v", err)
	}
	if ui.Theme != "dark" {
		t.Errorf("the theme of the copy is %q, want the last concrete one", ui.Theme)
	}

	// The configuration holds what the frontend stored.
	values, err := service.StoreRead("config.json")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}
	preferences := values["preferences"].(map[string]any)
	if preferences["theme"] != "auto" {
		t.Errorf("the theme is %v, want auto", preferences["theme"])
	}
}

// TestTheInterfacePreferencesAreStoredTogether verifies that the
// preferences that the backend owns are written to both records.
//
// The `ui` record is written on its own by the backend, and it would
// be able to leave the configuration behind.
func TestTheInterfacePreferencesAreStoredTogether(t *testing.T) {
	service := newTestService(t)

	if err := service.SaveUI(pocketbase.UI{Zoom: 150, Theme: "light"}); err != nil {
		t.Fatalf("save the interface preferences: %v", err)
	}

	ui, err := service.settings.GetUI()
	if err != nil {
		t.Fatalf("read the interface preferences: %v", err)
	}
	if ui.Zoom != 150 {
		t.Errorf("the zoom is %d, want 150", ui.Zoom)
	}

	values, err := service.StoreRead("config.json")
	if err != nil {
		t.Fatalf("read the store: %v", err)
	}
	preferences, ok := values["preferences"].(map[string]any)
	if !ok {
		t.Fatalf("the store holds no preferences: %v", values)
	}
	if preferences["uiScale"] != 1.5 {
		t.Errorf("the size of the interface is %v, want 1.5", preferences["uiScale"])
	}
	if preferences["theme"] != "light" {
		t.Errorf("the theme is %v, want light", preferences["theme"])
	}
}
