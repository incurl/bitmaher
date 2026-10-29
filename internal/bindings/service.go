// Package bindings exposes the BitMaher backend to the frontend as
// Wails services.
//
// These are the primary communication channel between the React UI and
// Go. The frontend never talks to PocketBase or to a remote API
// directly.
package bindings

import (
	"github.com/incurl/bitmaher/internal/console"
	"github.com/incurl/bitmaher/internal/downloads"
	"github.com/incurl/bitmaher/internal/pocketbase"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// AppService is the main Wails service. It is bound to the frontend and
// exposes every operation the UI needs.
type AppService struct {
	pb       *pocketbase.Service
	settings *pocketbase.SettingsStore
	console  *console.Log
	// httpAuth keeps the saved HTTP credentials. The password is
	// encrypted at rest, and only the backend ever sees it.
	httpAuth *pocketbase.HTTPAuthStore
	// downloads drives the Aria2-Next sidecar and keeps the download
	// records.
	downloads *downloads.Service
	// dataDir is the directory holding the database and the uploads.
	// The frontend asks for it, because the webview has no access to
	// the disk.
	dataDir string
	// downloadDir is the directory the downloads go to by default.
	downloadDir string
	// window is the window the application runs in. It is what the size
	// of the interface is applied to. It is set once the Wails
	// application exists.
	window *application.WebviewWindow
	// quit ends the application. It is the Wails application's own
	// quit, installed by the bootstrap once the application exists.
	quit func()
	// exitCode is the code the application is asked to end with. It
	// tells a restart apart from an ordinary exit.
	exitCode int
	// closer holds the close of the window until the frontend has
	// released it.
	closer *WindowCloser
}

// SetWindow tells the service which window the application runs in.
//
// The window is created by the bootstrap, and the service needs it to
// apply the size of the interface when the user changes it.
//
// It is a function and not a method, because a method on the service
// would become a method of the frontend's API: the binding generator
// exposes every exported method of a bound service, and this one is
// not for the frontend.
func SetWindow(service *AppService, window *application.WebviewWindow) {
	if service == nil {
		return
	}
	service.window = window
}

// NewAppService creates the application service.
//
// The download service is optional: the application runs without it,
// and the download calls then report that the engine is not running.
func NewAppService(
	pb *pocketbase.Service,
	settings *pocketbase.SettingsStore,
	log *console.Log,
	downloadService *downloads.Service,
	httpAuth *pocketbase.HTTPAuthStore,
	dataDir string,
	downloadDir string,
) *AppService {
	return &AppService{
		pb:          pb,
		settings:    settings,
		console:     log,
		downloads:   downloadService,
		httpAuth:    httpAuth,
		dataDir:     dataDir,
		downloadDir: downloadDir,
	}
}

// The service deliberately exposes no accessors for the objects it
// owns. Every exported method on a bound service becomes a method in the
// generated TypeScript, so a JobManager() or a Settings() accessor
// advertises an API the frontend cannot use: those types hold unexported
// fields and serialise to an empty object. They were also unused.
//
// If a method is needed, bind it explicitly here.

// ListConsoleEntries returns the recorded debug console entries,
// oldest first.
func (s *AppService) ListConsoleEntries() []console.Entry {
	if s.console == nil {
		return nil
	}
	return s.console.Entries()
}

// ClearConsoleEntries discards the recorded debug console entries.
func (s *AppService) ClearConsoleEntries() {
	if s.console == nil {
		return
	}
	s.console.Clear()
}

// SettingsView is the settings as the frontend sees them.
//
// The API key is never included: the frontend only learns whether one
// is configured. The same is true of the RPC secret of the sidecar.
type SettingsView struct {
	// HasAPIKey reports whether an API key is stored.
	HasAPIKey bool `json:"has_api_key"`
	// BaseURL is the configured remote API host.
	BaseURL string `json:"base_url"`
	// UI holds the interface preferences.
	UI pocketbase.UI `json:"ui"`
	// Aria2 holds the sidecar settings.
	Aria2 pocketbase.Aria2 `json:"aria2"`
}

// GetSettings returns the application settings.
func (s *AppService) GetSettings() (SettingsView, error) {
	view := SettingsView{}

	if _, found, err := s.settings.GetAPIKey(); err != nil {
		return SettingsView{}, err
	} else {
		view.HasAPIKey = found
	}

	baseURL, err := s.settings.GetBaseURL()
	if err != nil {
		return SettingsView{}, err
	}
	view.BaseURL = baseURL

	ui, err := s.settings.GetUI()
	if err != nil {
		return SettingsView{}, err
	}
	view.UI = ui

	aria2Settings, err := s.settings.GetAria2()
	if err != nil {
		return SettingsView{}, err
	}
	view.Aria2 = aria2Settings

	return view, nil
}

// SaveAria2 stores the sidecar settings.
//
// The RPC secret is not part of the settings: it is generated once and
// stored encrypted, and the frontend never sees it.
func (s *AppService) SaveAria2(settings pocketbase.Aria2) error {
	return s.settings.SetAria2(settings)
}

// SaveCredentials stores the API credentials.
//
// An empty apiKey keeps the stored key, so the frontend never has to
// read back a secret it cannot see.
func (s *AppService) SaveCredentials(apiKey, baseURL string) error {
	if apiKey != "" {
		if err := s.settings.SetAPIKey(apiKey); err != nil {
			return err
		}
	}
	if baseURL != "" {
		if err := s.settings.SetBaseURL(baseURL); err != nil {
			return err
		}
	}
	return nil
}

// SaveUI stores the interface preferences, including the zoom level.
func (s *AppService) SaveUI(ui pocketbase.UI) error {
	return s.settings.SetUI(ui)
}

// AppName is the name of the application.
const AppName = "BitMaher"

// AppVersion is the version of the application.
const AppVersion = "0.1.0"
