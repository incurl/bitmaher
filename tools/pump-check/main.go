// A live check of the pump.
//
// The pump is the single reader of the engine's state. It publishes the
// whole state as a snapshot, and the frontend waits for it.
//
// The check starts a real Aria2-Next sidecar, adds a download, starts
// the pump and waits for the snapshot that carries the download. It
// proves that the push model works against a real engine: the pump
// reads, the snapshot is published, and the state in it is the truth.
//
// The check needs an aria2-next binary. The bundled one is used, and
// the BITMAHER_ENGINE_BINARY environment variable overrides it.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
	bitmaherpb "github.com/incurl/bitmaher/internal/pocketbase"
	"github.com/incurl/bitmaher/internal/pump"
)

// collector records the snapshots the pump publishes.
type collector struct {
	mu        sync.Mutex
	snapshots []pump.Snapshot
	published chan struct{}
}

func newCollector() *collector {
	return &collector{published: make(chan struct{})}
}

func (c *collector) EmitSnapshot(snapshot pump.Snapshot) {
	c.mu.Lock()
	c.snapshots = append(c.snapshots, snapshot)
	c.mu.Unlock()

	select {
	case <-c.published:
	default:
		close(c.published)
	}
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.snapshots)
}

// last returns the newest snapshot.
func (c *collector) last() (pump.Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.snapshots) == 0 {
		return pump.Snapshot{}, false
	}
	return c.snapshots[len(c.snapshots)-1], true
}

func main() {
	dir, err := os.MkdirTemp("", "bitmaher-pump")
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

	// A file server, so that there is something to download.
	payload := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(payload, make([]byte, 256*1024), 0o600); err != nil {
		panic(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, payload)
	}))
	defer server.Close()

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

	// The pump reads the engine and publishes the snapshots.
	collector := newCollector()
	enginePump := pump.New(pump.NewEngine(service, manager), collector)
	enginePump.Start()
	defer enginePump.Stop()
	fmt.Println("4. The pump was started")

	// The snapshot that carries the download is the one that matters.
	deadline := time.Now().Add(20 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		snapshot, ok := collector.last()
		if ok {
			for _, task := range snapshot.Tasks {
				if task.GID == job.GID {
					found = true
					break
				}
			}
		}
		if !found {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if !found {
		fmt.Println("   FAIL: no snapshot carried the download")
		fmt.Printf("   the pump published %d snapshot(s)\n", collector.count())
		os.Exit(1)
	}
	fmt.Printf("5. A snapshot carried the download: %d snapshot(s) published\n", collector.count())

	// The snapshot carries the state of the engine as well.
	snapshot, _ := collector.last()
	if snapshot.Stat.DownloadSpeed < 0 {
		fmt.Println("   FAIL: the statistics of the snapshot are wrong")
		os.Exit(1)
	}
	fmt.Printf("6. The snapshot carried the statistics: %d active, %d waiting, %d stopped\n",
		snapshot.Stat.NumActive, snapshot.Stat.NumWaiting, snapshot.Stat.NumStopped)

	// The pump keeps publishing. The engine pushes nothing, so the
	// ticks are what keep the list fresh, and they must not stop.
	before := collector.count()
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if collector.count() > before+1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if collector.count() <= before+1 {
		fmt.Printf("   FAIL: the pump published %d snapshot(s) and stopped\n", collector.count())
		os.Exit(1)
	}
	fmt.Printf("7. The pump kept publishing: %d snapshot(s) in total\n", collector.count())

	// The download finished, and the snapshot says so.
	current, _ := collector.last()
	for _, task := range current.Tasks {
		if task.GID == job.GID && task.Status == "complete" {
			fmt.Println("8. The snapshot reports the download as complete")
			fmt.Println("\nPUMP: PASS")
			return
		}
	}

	fmt.Println("   FAIL: the snapshot does not report the download as complete")
	os.Exit(1)
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
