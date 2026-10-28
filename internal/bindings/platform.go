package bindings

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/incurl/bitmaher/internal/pocketbase"
)

// The platform commands answer the questions the frontend cannot answer
// itself: where the data lives, what the application is called, and the
// few operations that need the operating system.
//
// The frontend used to ask these questions through the Tauri plugins.
// They are the same questions, asked through Wails.

// Paths are the directories the application knows about.
type Paths struct {
	// AppData is the directory holding the database and the uploads.
	AppData string `json:"appData"`
	// AppLog is the directory holding the log of the sidecar.
	AppLog string `json:"appLog"`
	// Download is the directory the downloads go to by default.
	Download string `json:"download"`
	// Home is the home directory of the user.
	Home string `json:"home"`
	// Temp is the temporary directory of the system.
	Temp string `json:"temp"`
}

// PlatformPaths returns the directories the application knows about.
func (s *AppService) PlatformPaths() Paths {
	paths := Paths{
		AppData:  s.dataDir,
		Download: s.downloadDir,
	}

	if home, err := os.UserHomeDir(); err == nil {
		paths.Home = home
	}
	if s.dataDir != "" {
		paths.AppLog = filepath.Join(s.dataDir, "aria2")
	}
	if temp := os.TempDir(); temp != "" {
		paths.Temp = temp
	}

	return paths
}

// AppVersion returns the version of the application.
func (s *AppService) AppVersion() string {
	return AppVersion
}

// AppName returns the name of the application.
func (s *AppService) AppName() string {
	return AppName
}

// PathExists answers whether a path exists.
func (s *AppService) PathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// ReadLocalFile reads a file as text.
//
// The frontend reads a magnet link that was pasted as a file, and the
// contents of a log. The webview has no access to the disk, so the
// backend reads it.
func (s *AppService) ReadLocalFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("no path was given")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("the file could not be read: %w", err)
	}
	return string(data), nil
}

// WriteLocalFile writes a file.
func (s *AppService) WriteLocalFile(path, contents string) error {
	if path == "" {
		return fmt.Errorf("no path was given")
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("the file could not be written: %w", err)
	}
	return nil
}

// ShowInFolder shows a path in the file manager of the operating system.
func (s *AppService) ShowInFolder(path string) error {
	if path == "" {
		return fmt.Errorf("no path was given")
	}

	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "-R", path)
	case "windows":
		command = exec.Command("explorer", "/select,", path)
	default:
		command = exec.Command("xdg-open", filepath.Dir(path))
	}

	if err := command.Run(); err != nil {
		return fmt.Errorf("the folder could not be opened: %w", err)
	}
	return nil
}

// OpenExternalURL opens a URL in the default browser.
func (s *AppService) OpenExternalURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("only http and https URLs are opened")
	}

	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}

	if err := command.Start(); err != nil {
		return fmt.Errorf("the URL could not be opened: %w", err)
	}
	return nil
}

// WriteLog records an entry in the debug console.
//
// The frontend logs through this, so that the entries of the interface
// and the entries of the backend end up in the same place.
func (s *AppService) WriteLog(level, message string) {
	if s.console == nil {
		return
	}

	switch level {
	case "error":
		s.console.Error("frontend", message)
	case "warn":
		s.console.Warn("frontend", message)
	default:
		s.console.Info("frontend", message)
	}
}

// UnsupportedCommand reports a command the backend does not have.
//
// The ported frontend asks for the features of Rayburst that BitMaher
// does not have. The call is answered rather than ignored, so that the
// gap is visible in the debug console.
func (s *AppService) UnsupportedCommand(command string) {
	if s.console == nil {
		return
	}
	s.console.Warn("frontend", "the command is not supported by this backend: "+command)
}

// The key the preferences are stored under.
//
// The ported frontend keeps its whole configuration in one value of a
// key-value store, under this name.
const storePreferencesKey = "preferences"

// StoreRead returns the values of a store.
//
// The preferences of the ported frontend live in a JSON file. BitMaher
// keeps them in PocketBase, so the store is a view onto the settings
// record: the interface preferences are the ones the backend owns, and
// the rest of the configuration is returned as it was given.
func (s *AppService) StoreRead(name string) (map[string]any, error) {
	ui, err := s.settings.GetUI()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		storePreferencesKey: map[string]any{
			"uiScale": float64(ui.Zoom) / 100,
			"theme":   ui.Theme,
		},
	}, nil
}

// StoreReadValue reads one value of a store.
func (s *AppService) StoreReadValue(name, key string) (any, error) {
	values, err := s.StoreRead(name)
	if err != nil {
		return nil, err
	}
	return values[key], nil
}

// StoreWrite writes one value of a store.
//
// The interface preferences are the ones the backend owns: the size of
// the interface and the colour scheme. The rest of the configuration is
// accepted and dropped, because the backend has no place for it yet.
func (s *AppService) StoreWrite(name, key string, value any) error {
	if key != storePreferencesKey {
		return nil
	}

	values, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("the value of the preferences key is not a record")
	}

	settings, err := s.settings.GetUI()
	if err != nil {
		return err
	}

	updated, err := mergeUI(settings, values)
	if err != nil {
		return err
	}

	if err := s.settings.SetUI(updated); err != nil {
		return err
	}

	// The size of the interface is the zoom of the window, so the
	// change is applied at once. The user sees the interface grow or
	// shrink while the slider is still moving.
	s.applyZoom(updated.Zoom)

	return nil
}

// RefreshRuntimeConfig applies the stored preferences to the window.
//
// The frontend calls it after it has changed a preference, so that the
// change is visible at once. The size of the interface is the only
// preference the backend applies to the window.
func (s *AppService) RefreshRuntimeConfig() error {
	if s == nil {
		return nil
	}

	ui, err := s.settings.GetUI()
	if err != nil {
		return err
	}

	s.applyZoom(ui.Zoom)
	return nil
}

// applyZoom sets the zoom of the window.
//
// The zoom is the magnification of the page. The interface is laid out
// in `rem`, so the whole interface follows it, text included.
func (s *AppService) applyZoom(percent int) {
	if s == nil || s.window == nil || percent <= 0 {
		return
	}

	s.window.SetZoom(float64(percent) / 100)
}

// StoreDelete removes one value of a store.
func (s *AppService) StoreDelete(name, key string) error {
	return nil
}

// StoreClear removes every value of a store.
func (s *AppService) StoreClear(name string) error {
	return nil
}

// StoreKeys returns the keys of a store.
func (s *AppService) StoreKeys(name string) ([]string, error) {
	return []string{storePreferencesKey}, nil
}

// mergeUI folds the values of the frontend into the stored preferences.
//
// The frontend sends the whole record, so only the keys it knows about
// are taken. The rest are kept as they are.
//
// The size of the interface is a factor in the frontend and a
// percentage in the backend, because the setting is the one a person
// reads: "110%" means something, "1.1" does not.
func mergeUI(current pocketbase.UI, values map[string]any) (pocketbase.UI, error) {
	updated := current

	if scale, ok := values["uiScale"].(float64); ok {
		zoom := int(math.Round(scale * 100))
		if zoom < minZoom {
			zoom = minZoom
		}
		if zoom > maxZoom {
			zoom = maxZoom
		}
		updated.Zoom = zoom
	}
	if theme, ok := values["theme"].(string); ok {
		updated.Theme = theme
	}

	return updated, nil
}

// The bounds of the size of the interface, as a percentage.
//
// They are the bounds the settings declare, so that the backend and the
// interface agree on what is possible.
const (
	minZoom = 50
	maxZoom = 200
)

// EngineState is the state of the download engine.
//
// The frontend shows it in the about panel and in the header, so that
// the user can see which engine is running.
type EngineState struct {
	// Running tells whether the engine is up.
	Running bool `json:"running"`
	// Version is the version of the engine.
	Version string `json:"version"`
	// Port is the port the engine listens on.
	Port int `json:"port"`
}

// EngineState returns the state of the download engine.
func (s *AppService) EngineState() EngineState {
	state := EngineState{}
	if s.downloads == nil {
		return state
	}

	// The manager knows whether the engine is up. It is reached
	// through the service, which keeps the manager to itself.
	if manager := s.downloads.Manager(); manager != nil {
		state.Running = manager.Ready()
		state.Port = manager.Port()
		if version := manager.Version(); version.Version != "" {
			state.Version = version.Product + " " + version.Version
		}
	}

	return state
}

// DiskFreeSpace returns the free space of the disk a path is on.
//
// The frontend shows it next to the size of a download, so that the
// user can tell whether there is room for it.
func (s *AppService) DiskFreeSpace(path string) (int64, error) {
	if path == "" {
		path = s.downloadDir
	}
	if path == "" {
		return 0, fmt.Errorf("no path was given")
	}

	free, err := freeSpace(path)
	if err != nil {
		return 0, err
	}
	return free, nil
}

// Shutdown ends the application.
//
// The frontend asks for it when the user chooses to close the
// application and the downloads are still running.
func (s *AppService) Shutdown() {
	if s.console != nil {
		s.console.Info("app", "the application was asked to shut down")
	}
}

// freeSpace returns the free space of the disk a path is on.
//
// The portable way to ask is to stat the file system and read the
// blocks that are left. The unix and the windows spellings are
// different, so each one is handled on its own.
func freeSpace(path string) (int64, error) {
	// The path may be a file. The space is that of the file system it
	// is on, so the directory is asked for.
	directory := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		directory = filepath.Dir(path)
	}

	return freeSpaceOn(directory)
}
