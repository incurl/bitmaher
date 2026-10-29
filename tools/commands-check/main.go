// A live check of the commands the port added.
//
// The check runs against a real Aria2-Next sidecar: the preview asks
// the engine what a download would be, the categories classify a file,
// the file system removes what the interface asks it to remove, and the
// engine comes back after a restart.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/bindings"
	"github.com/incurl/bitmaher/internal/console"
	"github.com/incurl/bitmaher/internal/downloads"
	"github.com/incurl/bitmaher/internal/downloads/category"
	bitmaherpb "github.com/incurl/bitmaher/internal/pocketbase"
)

func main() {
	dir, err := os.MkdirTemp("", "bitmaher-commands")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	svc := bitmaherpb.NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		panic(err)
	}
	defer svc.Close()
	fmt.Println("1. PocketBase bootstrapped, schema applied")

	binary := engineBinary()
	fmt.Printf("   the engine binary is %s\n", binary)

	secret, err := aria2.GenerateSecret()
	if err != nil {
		panic(err)
	}

	manager := aria2.NewManager(
		aria2.WithBinary(binary),
		aria2.WithDataDir(filepath.Join(dir, "aria2")),
		aria2.WithSecret(secret),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := manager.Start(ctx); err != nil {
		fmt.Println("   FAIL: the sidecar did not start:", err)
		os.Exit(1)
	}
	fmt.Printf("2. The sidecar started on port %d (%s %s)\n",
		manager.Port(), manager.Version().Product, manager.Version().Version)

	cipher, err := bitmaherpb.NewCipher(svc.DataDir())
	if err != nil {
		panic(err)
	}

	service := bindings.NewAppService(
		svc,
		bitmaherpb.NewSettingsStore(svc.App(), cipher),
		console.New(),
		downloads.NewService(manager, downloads.NewStore(svc.App()), nil),
		bitmaherpb.NewHTTPAuthStore(svc.App(), cipher),
		dir,
		dir,
	)

	// --- the preview ----------------------------------------------------
	payload := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(payload, make([]byte, 128*1024), 0o600); err != nil {
		panic(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="payload.bin"`)
		http.ServeFile(w, r, payload)
	}))
	defer server.Close()

	preview, err := service.PreviewDownload(bindings.PreviewRequest{
		ID:      "preview-1",
		URL:     server.URL + "/payload.bin",
		Options: map[string]string{},
	})
	if err != nil {
		fmt.Println("   FAIL: the preview failed:", err)
		os.Exit(1)
	}
	fmt.Printf("3. The preview answered: %s, %v bytes\n", preview.Filename, preview.Size)

	if preview.Filename != "payload.bin" {
		fmt.Printf("   FAIL: the name is %q, want payload.bin\n", preview.Filename)
		os.Exit(1)
	}
	if preview.Size == nil || *preview.Size != 128*1024 {
		fmt.Printf("   FAIL: the size is %v, want %d\n", preview.Size, 128*1024)
		os.Exit(1)
	}

	// --- the categories ---------------------------------------------------
	categories := []category.Category{{
		Label:          "Documents",
		Directory:      "Documents",
		DirectoryMode:  category.ModeRelative,
		Extensions:     []string{"pdf"},
		URLPatternMode: category.ModeWildcard,
	}}

	resolved, err := service.ResolveFileCategory(
		[]category.Candidate{{Path: "manual.pdf"}},
		categories,
		dir,
	)
	if err != nil {
		fmt.Println("   FAIL: the category was not resolved:", err)
		os.Exit(1)
	}
	if resolved == nil {
		fmt.Println("   FAIL: no category was resolved")
		os.Exit(1)
	}
	fmt.Printf("4. The category was resolved: %s -> %s\n",
		resolved.Label, resolved.Directory)

	// A file that no category wants is not classified.
	resolved, err = service.ResolveFileCategory(
		[]category.Candidate{{Path: "movie.mkv"}},
		categories,
		dir,
	)
	if err != nil {
		fmt.Println("   FAIL: the category was not resolved:", err)
		os.Exit(1)
	}
	if resolved != nil {
		fmt.Println("   FAIL: a file that no category wants was classified")
		os.Exit(1)
	}
	fmt.Println("5. A file that no category wants is not classified")

	// The categories are checked before they are saved.
	if err := service.ValidateFileCategories(categories, dir); err != nil {
		fmt.Println("   FAIL: the categories were refused:", err)
		os.Exit(1)
	}
	fmt.Println("6. The categories are valid")

	// --- the file system ----------------------------------------------------
	trashFile := filepath.Join(dir, "doomed.bin")
	if err := os.WriteFile(trashFile, []byte("x"), 0o600); err != nil {
		panic(err)
	}

	removed, err := service.DeletePath(trashFile, bindings.DeletionTrash)
	if err != nil {
		fmt.Println("   FAIL: the file was not moved to the trash:", err)
		os.Exit(1)
	}
	if !removed {
		fmt.Println("   FAIL: the file was not removed")
		os.Exit(1)
	}
	if _, err := os.Stat(trashFile); !os.IsNotExist(err) {
		fmt.Println("   FAIL: the file is still there")
		os.Exit(1)
	}
	fmt.Println("7. The file was moved to the trash")

	// A path that was never there is not a failure.
	removed, err = service.DeletePath(filepath.Join(dir, "never.bin"), bindings.DeletionPermanent)
	if err != nil {
		fmt.Println("   FAIL: a missing path was a failure:", err)
		os.Exit(1)
	}
	if removed {
		fmt.Println("   FAIL: a missing path was reported as removed")
		os.Exit(1)
	}
	fmt.Println("8. A path that was never there is not a failure")

	// --- the engine ---------------------------------------------------------
	if err := service.EngineRecoverRuntimeState(); err != nil {
		fmt.Println("   FAIL: the engine did not come back:", err)
		os.Exit(1)
	}
	if !manager.Ready() {
		fmt.Println("   FAIL: the engine is not ready")
		os.Exit(1)
	}
	fmt.Println("9. The engine came back after a restart")

	fmt.Println("\nCOMMANDS: PASS")
}

// engineBinary returns the engine the check runs against.
//
// The bundled binary is the one the application ships, so the check
// tests what the user gets. The BITMAHER_ENGINE_BINARY environment
// variable overrides it, for a run against another build.
func engineBinary() string {
	if path := os.Getenv("BITMAHER_ENGINE_BINARY"); path != "" {
		return path
	}

	path, err := aria2.BundledBinaryPath(filepath.Join(os.TempDir(), "bitmaher-check-engine"))
	if err != nil {
		return "/usr/bin/aria2-next"
	}
	return path
}
