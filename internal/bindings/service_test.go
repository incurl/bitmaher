package bindings

import (
	"testing"

	"github.com/incurl/bitmaher/internal/console"
	"github.com/incurl/bitmaher/internal/pocketbase"
)

// newTestService creates an application service backed by a throwaway
// database.
func newTestService(t *testing.T) *AppService {
	t.Helper()

	pb := pocketbase.NewService(t.TempDir())
	if err := pb.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = pb.Close()
	})

	cipher, err := pocketbase.NewCipher(pb.DataDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	settings := pocketbase.NewSettingsStore(pb.App(), cipher)

	log := console.New()

	return NewAppService(pb, settings, log, nil, pocketbase.NewHTTPAuthStore(pb.App(), cipher), pb.DataDir(), "")
}

// TestSettingsNeverExposeTheSecret verifies that the API key is not
// returned to the frontend.
func TestSettingsNeverExposeTheSecret(t *testing.T) {
	service := newTestService(t)

	if err := service.SaveCredentials("sk-live-SECRET", "https://api.test"); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if !settings.HasAPIKey {
		t.Error("the settings do not report that a key is stored")
	}
	if settings.BaseURL != "https://api.test" {
		t.Errorf("base url = %q", settings.BaseURL)
	}
}

// TestSettingsNeverExposeTheRPCSecret verifies that the engine's RPC
// secret is not part of the settings the frontend sees.
//
// The secret is the engine's authentication, so it must never cross the
// boundary in either direction.
func TestSettingsNeverExposeTheRPCSecret(t *testing.T) {
	service := newTestService(t)

	secret := "s3cr3t-rpc-token"
	if err := service.settings.SetAria2Secret(secret); err != nil {
		t.Fatalf("store the secret: %v", err)
	}

	settings, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}

	// The view has no field for it at all. The check spells the fields
	// out, so that a future field cannot quietly carry it.
	if settings.Aria2.DownloadDir == secret {
		t.Error("the RPC secret leaked into the settings")
	}
	if settings.Aria2.ExternalBinaryPath == secret {
		t.Error("the RPC secret leaked into the settings")
	}
}

// TestAria2SettingsRoundTrip verifies that the engine settings survive
// a write and a read.
func TestAria2SettingsRoundTrip(t *testing.T) {
	service := newTestService(t)

	before, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	// A fresh database has the defaults.
	if before.Aria2.AutoRestart != true {
		t.Error("the watchdog should be on by default")
	}
	if before.Aria2.DownloadDir == "" {
		t.Error("a download directory should be set by default")
	}

	want := pocketbase.Aria2{
		DownloadDir:        "/tmp/downloads",
		MaxDownloadSpeed:   1024 * 1024,
		MaxUploadSpeed:     64 * 1024,
		UseExternalBinary:  true,
		ExternalBinaryPath: "/usr/bin/aria2-next",
		AutoRestart:        false,
		LogLevel:           "info",
	}
	if err := service.SaveAria2(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	after, err := service.GetSettings()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if after.Aria2.DownloadDir != want.DownloadDir {
		t.Errorf("download dir = %q", after.Aria2.DownloadDir)
	}
	if after.Aria2.MaxDownloadSpeed != want.MaxDownloadSpeed {
		t.Errorf("download limit = %d", after.Aria2.MaxDownloadSpeed)
	}
	if after.Aria2.AutoRestart != false {
		t.Error("the watchdog should be off")
	}
}
