// BitMaher is a desktop download manager.
//
// It wires together Wails v3, an embedded PocketBase and a React
// frontend, and it drives the Aria2-Next download engine.
//
// # LICENSING BOUNDARY
//
// Aria2-Next is GPLv2 licensed. BitMaher runs it as a SEPARATE,
// INDEPENDENT CHILD PROCESS and talks to it only over JSON-RPC on
// localhost. Nothing in this binary links against aria2, embeds its
// source or calls into it through CGO. The two programs are separate
// works that happen to speak a protocol.
//
// The download engine does the transfers. This application owns the
// interface, the persistence and the lifecycle of the sidecar.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/bindings"
	"github.com/incurl/bitmaher/internal/console"
	"github.com/incurl/bitmaher/internal/downloads"
	"github.com/incurl/bitmaher/internal/pocketbase"
	"github.com/incurl/bitmaher/internal/pump"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails embeds the built frontend. Any file in frontend/dist is served
// to the webview.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register the console entry event so the binding generator produces
	// a strongly typed TS API for it.
	application.RegisterEvent[console.Entry]("console:entry")

	// The snapshot carries the whole state of the engine. It is
	// registered as well, so that the binding generator produces a
	// strongly typed TS API for it.
	application.RegisterEvent[pump.Snapshot](pump.EventSnapshot)

	// The frontend writes to the log through this event. The payload is
	// a console.Entry, so that the entries of the frontend and of the
	// backend have the same shape.
	application.RegisterEvent[console.Entry](bindings.EventFrontendLog)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run bootstraps the backend and starts the Wails application.
func run() error {
	// --- PocketBase ------------------------------------------------
	pbService := pocketbase.NewService(dataDir())
	if err := pbService.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap pocketbase: %w", err)
	}

	cipher, err := pocketbase.NewCipher(pbService.DataDir())
	if err != nil {
		return fmt.Errorf("initialise settings encryption: %w", err)
	}
	settings := pocketbase.NewSettingsStore(pbService.App(), cipher)

	// The event bridge pushes job updates to the frontend. It is
	// created once the Wails application exists, so it is declared here
	// and assigned later.
	var bridge *bindings.EventBridge
	// stopConsoleListener stops the listener that takes the log entries
	// of the frontend.
	var stopConsoleListener func()

	// The debug console records the application traffic for the panel
	// at the bottom of the window.
	consoleLog := console.New()

	// --- The Aria2-Next sidecar -------------------------------------
	//
	// The engine is a separate process. It is started here, driven
	// over JSON-RPC, and stopped when the application closes.
	sidecar, err := startSidecar(pbService, settings, consoleLog)
	if err != nil {
		// A missing engine is not fatal: the application starts and
		// says what is wrong, so the user can point it at a binary
		// that works.
		consoleLog.Error("aria2", "the download engine could not be started: "+err.Error())
	}
	if sidecar == nil {
		// The manager is always returned, so that the shutdown has
		// something to stop.
		sidecar = aria2.NewManager(aria2.WithLog(consoleLog))
	}

	// The download service drives the engine and keeps the records in
	// step with it.
	downloadService := downloads.NewService(sidecar, downloads.NewStore(pbService.App()), consoleLog)

	// The service knows where the data lives, because the frontend asks
	// the backend: the webview has no access to the disk.
	appService := bindings.NewAppService(
		pbService,
		settings,
		consoleLog,
		downloadService,
		pbService.DataDir(),
		downloadDir(settings),
	)

	// The pump is the single reader of the engine's state. It publishes
	// the snapshots as Wails events, so that the frontend never has to
	// poll the engine.
	enginePump := pump.New(pump.NewEngine(downloadService, sidecar), nil)

	// Mount PocketBase's own REST and file API alongside the static
	// frontend. This keeps everything on a single origin: no extra
	// port, no separate PocketBase process and no CORS handling.
	pbHandler, handlerErr := pbService.Handler()
	if handlerErr != nil {
		return fmt.Errorf("create pocketbase handler: %w", handlerErr)
	}

	// --- Wails application -----------------------------------------
	app := application.New(application.Options{
		Name: bindings.AppName,
		// The description lands in the about box of the platform, so
		// it is the one place the version is visible without a menu.
		Description: "A Wails v3 + PocketBase + React desktop application, version " + bindings.AppVersion,
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// The sidecar is stopped when the application closes: it is a
		// child process, and it must not outlive the application that
		// started it.
		OnShutdown: func() {
			// The pump stops before the sidecar, so that it does not
			// read an engine that is on its way out.
			enginePump.Stop()
			if stopConsoleListener != nil {
				stopConsoleListener()
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), sidecarShutdown)
			defer cancel()
			if err := sidecar.Stop(shutdownCtx); err != nil {
				consoleLog.Warn("aria2", "the sidecar did not stop cleanly: "+err.Error())
			}
		},
	})

	// The event bridge pushes job updates to the frontend, and takes
	// the log entries of the frontend.
	bridge = bindings.NewEventBridge(app)
	consoleLog.SetEmitter(bridge)
	stopConsoleListener = bridge.ListenConsoleEntries(consoleLog)

	// The pump publishes the snapshots through the same bridge.
	enginePump.SetEmitter(bridge)
	enginePump.Start()

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  bindings.AppName,
		Width:  1440,
		Height: 900,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	return app.Run()
}

// combineHandlers routes /api/* to the PocketBase handler and
// everything else to the static frontend.
func combineHandlers(apiHandler, staticHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})
}

// dataDir returns the directory holding the database and the uploaded
// files.
//
// The BITMAHER_DATA_DIR environment variable overrides the default,
// which lets the app run against an isolated database during development
// and testing.
func dataDir() string {
	if dir := os.Getenv("BITMAHER_DATA_DIR"); dir != "" {
		return dir
	}
	return pocketbase.DefaultDataDir()
}

// sidecarShutdown is how long the sidecar may take to stop when the
// application closes.
const sidecarShutdown = 10 * time.Second

// startSidecar prepares and starts the Aria2-Next sidecar.
//
// The function does four things:
//
//  1. it works out which binary to run, the bundled one or the
//     external one the user chose;
//  2. it reads the RPC secret, or generates one on the first run;
//  3. it starts the engine on a free port;
//  4. it applies the global speed limits the user set.
//
// The secret is stored encrypted, so it is never in plain text in the
// database and never leaves the machine.
func startSidecar(
	pbService *pocketbase.Service,
	settings *pocketbase.SettingsStore,
	consoleLog *console.Log,
) (*aria2.Manager, error) {
	aria2Settings, err := settings.GetAria2()
	if err != nil {
		return nil, fmt.Errorf("read the engine settings: %w", err)
	}

	// The binary is either the bundled one or the external one.
	external := ""
	if aria2Settings.UseExternalBinary {
		external = aria2Settings.ExternalBinaryPath
	}

	cacheDir := filepath.Join(pbService.DataDir(), "aria2")
	binary, err := aria2.ResolveBinary(external, cacheDir)
	if err != nil {
		return nil, err
	}

	// The secret is generated once, on the first run, and read back on
	// every later one.
	secret, found, err := settings.GetAria2Secret()
	if err != nil {
		return nil, fmt.Errorf("read the RPC secret: %w", err)
	}
	if !found || secret == "" {
		secret, err = aria2.GenerateSecret()
		if err != nil {
			return nil, err
		}
		if err := settings.SetAria2Secret(secret); err != nil {
			return nil, fmt.Errorf("store the RPC secret: %w", err)
		}
		consoleLog.Info("aria2", "a new RPC secret was generated for the sidecar")
	}

	manager := aria2.NewManager(
		aria2.WithBinary(binary),
		aria2.WithDataDir(cacheDir),
		aria2.WithSecret(secret),
		aria2.WithAutoRestart(aria2Settings.AutoRestart),
		aria2.WithLog(consoleLog),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := manager.Start(ctx); err != nil {
		return manager, err
	}

	// The global speed limits are applied to the running engine.
	if aria2Settings.MaxDownloadSpeed > 0 || aria2Settings.MaxUploadSpeed > 0 {
		options := map[string]string{}
		if aria2Settings.MaxDownloadSpeed > 0 {
			options["max-overall-download-limit"] = formatSpeed(aria2Settings.MaxDownloadSpeed)
		}
		if aria2Settings.MaxUploadSpeed > 0 {
			options["max-overall-upload-limit"] = formatSpeed(aria2Settings.MaxUploadSpeed)
		}
		if client := manager.Client(); client != nil {
			if err := client.ChangeGlobalOption(ctx, options); err != nil {
				consoleLog.Warn("aria2", "the speed limits could not be applied: "+err.Error())
			}
		}
	}

	// The downloads that were running when the application last closed
	// are in the engine's session file, and the engine picks them up
	// again. The records are reconciled with them, so that the list
	// shows the right state.
	reconcileDownloads(manager, downloads.NewStore(pbService.App()), consoleLog)

	return manager, nil
}

// reconcileDownloads brings the records in step with the engine after a
// restart.
//
// The engine reads its session file on start, so the downloads that
// were running are running again. The records are read from the engine
// and updated, so the list shows what is really going on.
func reconcileDownloads(
	manager *aria2.Manager,
	store *downloads.Store,
	consoleLog *console.Log,
) {
	client := manager.Client()
	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	active, err := client.Active(ctx)
	if err != nil {
		consoleLog.Warn("aria2", "the running downloads could not be read: "+err.Error())
		return
	}

	waiting, err := client.Waiting(ctx, 0, 200)
	if err != nil {
		consoleLog.Warn("aria2", "the queued downloads could not be read: "+err.Error())
		return
	}

	stopped, err := client.Stopped(ctx, 0, 200)
	if err != nil {
		consoleLog.Warn("aria2", "the finished downloads could not be read: "+err.Error())
		return
	}

	total := len(active) + len(waiting) + len(stopped)
	if total == 0 {
		return
	}

	consoleLog.Info("aria2", fmt.Sprintf("%d download(s) were picked up from the session file", total))
}

// downloadDir returns the directory the downloads go to by default.
//
// The setting is empty when the user never chose one, in which case the
// home directory of the user is the answer.
func downloadDir(settings *pocketbase.SettingsStore) string {
	aria2Settings, err := settings.GetAria2()
	if err != nil {
		return ""
	}
	if aria2Settings.DownloadDir != "" {
		return aria2Settings.DownloadDir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
}

// formatSpeed renders a speed in the form the engine expects.
//
// The engine takes a number with a unit suffix, such as "1M" for a
// megabyte per second.
func formatSpeed(bytesPerSecond int64) string {
	if bytesPerSecond <= 0 {
		return "0"
	}
	return fmt.Sprintf("%d", bytesPerSecond)
}
