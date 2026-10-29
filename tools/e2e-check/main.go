// A live end-to-end check of BitMaher.
//
// It exercises the whole pipeline without a GUI: the PocketBase schema
// is applied, the Aria2-Next sidecar is started as a real child
// process, a download is added over JSON-RPC, the state is read back
// and the record is persisted.
//
// The engine is a separate process, driven over a loopback socket. The
// check proves that the boundary works: the sidecar is started, driven
// and stopped, and the data ends up in the database.
//
// The check needs an aria2-next binary. The one on the PATH is used, and
// the bundled one is used when there is none.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
	bitmaherpb "github.com/incurl/bitmaher/internal/pocketbase"
)

func main() {
	dir, err := os.MkdirTemp("", "bitmaher-e2e")
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

	// The binary is the one on the PATH, or the bundled one.
	binary, err := findBinary()
	if err != nil {
		fmt.Println("   FAIL:", err)
		os.Exit(1)
	}
	fmt.Printf("   the engine binary is %s\n", binary)

	// The sidecar is started as a real child process.
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

	// The engine has to be aria2-next. The upstream aria2 is not what
	// this application is built against.
	if !manager.Version().IsAria2Next() {
		fmt.Println("   FAIL: the engine is not aria2-next")
		os.Exit(1)
	}

	// A file server, so that there is something to download.
	payload := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(payload, make([]byte, 512*1024), 0o600); err != nil {
		panic(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, payload)
	}))
	defer server.Close()

	// The service drives the engine and keeps the records.
	store := downloads.NewStore(svc.App())
	service := downloads.NewService(manager, store, nil)

	job, err := service.Add(ctx, downloads.AddRequest{
		Kind: "http",
		URI:  server.URL + "/payload.bin",
		Dir:  dir,
	})
	if err != nil {
		fmt.Println("   FAIL: the download could not be added:", err)
		os.Exit(1)
	}
	fmt.Printf("3. The download was added: %s (gid %s)\n", job.Title, job.GID)

	// The state is read back and the record is updated.
	deadline := time.Now().Add(15 * time.Second)
	var last downloads.Job
	for time.Now().Before(deadline) {
		if _, err := service.Sync(ctx); err != nil {
			fmt.Println("   FAIL: the sync failed:", err)
			os.Exit(1)
		}

		stored, err := store.Get(job.ID)
		if err != nil {
			fmt.Println("   FAIL: the record could not be read:", err)
			os.Exit(1)
		}
		last = stored

		if stored.Status == downloads.StatusComplete {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("4. The record was updated: status=%s size=%d\n",
		last.Status, last.TotalLength)
	if last.Status != downloads.StatusComplete {
		fmt.Println("   FAIL: the download did not finish:", last.Status, last.ErrorMessage)
		os.Exit(1)
	}
	if last.TotalLength != 512*1024 {
		fmt.Printf("   FAIL: the size is %d, want %d\n", last.TotalLength, 512*1024)
		os.Exit(1)
	}

	// The downloaded file has to be on disk, at the place that was
	// asked for.
	found := filepath.Join(dir, "payload.bin")
	if _, err := os.Stat(found); err != nil {
		fmt.Println("   FAIL: the file was not written:", err)
		os.Exit(1)
	}
	fmt.Println("   the file was written to the download folder")

	// The download that has come to rest is in the history, and the
	// moment it was added is recorded. The two are what hold the
	// order of the list stable across a restart.
	history := service.History()
	records, err := history.Records(downloads.HistoryStatusComplete)
	if err != nil {
		fmt.Println("   FAIL: the history could not be read:", err)
		os.Exit(1)
	}
	if len(records) != 1 {
		fmt.Printf("   FAIL: the history holds %d entries, want 1\n", len(records))
		os.Exit(1)
	}
	if records[0].GID != job.GID {
		fmt.Printf("   FAIL: the history holds %q, want %q\n", records[0].GID, job.GID)
		os.Exit(1)
	}
	if records[0].AddedAt == "" {
		fmt.Println("   FAIL: the history has no moment of its own")
		os.Exit(1)
	}
	fmt.Printf("5. The history holds the download: %s (added at %s)\n",
		records[0].Name, records[0].AddedAt)

	// The configuration of the frontend survives a write and a read.
	// It used to keep only the size of the interface and the theme,
	// so every other setting of the user was lost.
	cipher, err := bitmaherpb.NewCipher(svc.DataDir())
	if err != nil {
		fmt.Println("   FAIL: the cipher could not be created:", err)
		os.Exit(1)
	}
	settings := bitmaherpb.NewSettingsStore(svc.App(), cipher)
	configuration := map[string]any{
		"uiScale":          1.25,
		"theme":            "dark",
		"dir":              dir,
		"maxDownloadLimit": int64(1048576),
		"btMaxPeers":       50,
	}
	if err := settings.SetAppConfig(configuration); err != nil {
		fmt.Println("   FAIL: the configuration could not be stored:", err)
		os.Exit(1)
	}
	stored, wasStored, readErr := settings.GetAppConfig()
	if readErr != nil {
		fmt.Println("   FAIL: the configuration could not be read:", readErr)
		os.Exit(1)
	}
	if !wasStored {
		fmt.Println("   FAIL: the configuration was not found")
		os.Exit(1)
	}
	for _, key := range []string{"dir", "maxDownloadLimit", "btMaxPeers"} {
		if _, ok := stored[key]; !ok {
			fmt.Printf("   FAIL: the setting %q was lost\n", key)
			os.Exit(1)
		}
	}
	fmt.Println("6. The configuration of the frontend survived the round trip")

	// The engine's own statistics have to answer.
	stat, err := service.Stat(ctx)
	if err != nil {
		fmt.Println("   FAIL: the statistics could not be read:", err)
		os.Exit(1)
	}
	fmt.Printf("7. The engine answered: %d stopped, %d active\n", stat.NumStopped, stat.NumActive)

	// The sidecar is stopped, and it must not outlive the check.
	if err := manager.Stop(ctx); err != nil {
		fmt.Println("   FAIL: the sidecar did not stop:", err)
		os.Exit(1)
	}
	fmt.Println("8. The sidecar stopped")

	fmt.Println("\nEND TO END: PASS")
}

// findBinary works out which aria2-next binary to use.
//
// The one on the PATH is preferred, because that is what a developer
// has. The bundled one is the fallback.
func findBinary() (string, error) {
	if path, err := exec.LookPath("aria2-next"); err == nil {
		return path, nil
	}

	cache := filepath.Join(os.TempDir(), "bitmaher-e2e-binary")
	path, err := aria2.BundledBinaryPath(cache)
	if err != nil {
		return "", fmt.Errorf("no aria2-next binary was found: %w", err)
	}
	return path, nil
}
