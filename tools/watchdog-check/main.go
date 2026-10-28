package main

// A check that the watchdog starts the sidecar again when it crashes.
//
// The sidecar is a child process, so it can die on its own: a signal
// from the user, a crash in the engine, an out-of-memory kill. The
// watchdog notices and starts it again.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
)

func main() {
	dir, _ := os.MkdirTemp("", "bitmaher-watchdog")
	defer os.RemoveAll(dir)

	secret, _ := aria2.GenerateSecret()
	mgr := aria2.NewManager(
		aria2.WithBinary("/usr/bin/aria2-next"),
		aria2.WithDataDir(filepath.Join(dir, "aria2")),
		aria2.WithSecret(secret),
		aria2.WithAutoRestart(true),
	)

	ctx := context.Background()
	if _, err := mgr.Start(ctx); err != nil {
		panic(err)
	}
	firstPort := mgr.Port()
	fmt.Printf("1. The sidecar started on port %d (pid %d)\n", firstPort, mgr.PID())

	// The sidecar is killed, as if it had crashed.
	if err := syscall.Kill(-mgr.PID(), syscall.SIGKILL); err != nil {
		fmt.Println("   the kill failed:", err)
		os.Exit(1)
	}
	fmt.Println("2. The sidecar was killed")

	// The watchdog has a moment to notice and start it again.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if mgr.Ready() && mgr.Port() != firstPort {
			fmt.Printf("3. The watchdog started it again on port %d\n", mgr.Port())
			if err := mgr.Stop(ctx); err != nil {
				fmt.Println("   FAIL: the sidecar did not stop:", err)
				os.Exit(1)
			}
			fmt.Println("4. The sidecar stopped")
			fmt.Println("\nWATCHDOG: PASS")
			return
		}
	}

	fmt.Println("   FAIL: the watchdog did not start the sidecar again")
	os.Exit(1)
}
