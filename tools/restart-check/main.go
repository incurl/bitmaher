package main

// A check that a download survives a restart of the application.
//
// The engine keeps a session file with the gid of every download that
// was running. On the next start it picks them up again, and the
// records are brought in step with them.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
	pb "github.com/incurl/bitmaher/internal/pocketbase"
)

func main() {
	dir, _ := os.MkdirTemp("", "bitmaher-restart")
	defer os.RemoveAll(dir)

	svc := pb.NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		panic(err)
	}
	defer svc.Close()

	payload := filepath.Join(dir, "payload.bin")
	os.WriteFile(payload, make([]byte, 8*1024*1024), 0o600)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, payload)
	}))
	defer server.Close()

	secret, _ := aria2.GenerateSecret()
	dataDir := filepath.Join(dir, "aria2")

	// --- first run ---
	mgr := aria2.NewManager(aria2.WithBinary("/usr/bin/aria2-next"),
		aria2.WithDataDir(dataDir), aria2.WithSecret(secret))
	ctx := context.Background()
	if _, err := mgr.Start(ctx); err != nil {
		panic(err)
	}
	store := downloads.NewStore(svc.App())
	svc1 := downloads.NewService(mgr, store, nil)

	job, err := svc1.Add(ctx, downloads.AddRequest{
		Kind: "http", URI: server.URL + "/payload.bin", Dir: dir,
		Options: map[string]string{"max-download-limit": "50K"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("1. The download was added: gid %s\n", job.GID)

	// Let it get going, then stop the application.
	time.Sleep(2 * time.Second)
	svc1.Sync(ctx)
	if err := mgr.Stop(ctx); err != nil {
		panic(err)
	}
	fmt.Println("2. The application was closed (the sidecar stopped)")

	// --- second run ---
	mgr2 := aria2.NewManager(aria2.WithBinary("/usr/bin/aria2-next"),
		aria2.WithDataDir(dataDir), aria2.WithSecret(secret))
	if _, err := mgr2.Start(ctx); err != nil {
		panic(err)
	}
	fmt.Println("3. The application was started again")

	svc2 := downloads.NewService(mgr2, store, nil)
	if _, err := svc2.Sync(ctx); err != nil {
		panic(err)
	}
	stored, err := store.Get(job.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("4. The record after the restart: status=%s completed=%d gid=%s\n",
		stored.Status, stored.CompletedLength, stored.GID)

	active, _ := mgr2.Client().Active(ctx)
	waiting, _ := mgr2.Client().Waiting(ctx, 0, 50)
	fmt.Printf("   the engine knows %d active and %d waiting downloads\n", len(active), len(waiting))

	if stored.GID != job.GID {
		fmt.Println("   FAIL: the gid changed")
		os.Exit(1)
	}
	if stored.Status != "active" && stored.Status != "waiting" {
		fmt.Printf("   FAIL: the download is %q, it should have been picked up\n", stored.Status)
		os.Exit(1)
	}
	fmt.Println("   the download was picked up again with the same gid")
	mgr2.Stop(ctx)
	fmt.Println("\nRESTART RECOVERY: PASS")
}
