package main

// A check that the bundled aria2-next binary is found, copied to the
// cache and started.
//
// The bundled binary is embedded in the application, so it is copied
// into the cache directory before it is run: the directory the
// application was installed in is not always writable.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
)

func main() {
	cache := filepath.Join(os.TempDir(), "bitmaher-bundled-check")
	os.RemoveAll(cache)

	if !aria2.HasBundledBinary() {
		fmt.Println("   FAIL: no bundled binary is present")
		fmt.Println("   run: wails3 task common:fetch:aria2")
		os.Exit(1)
	}
	fmt.Printf("1. The bundled binary is present (version %s)\n", aria2.BundledVersion())

	path, err := aria2.BundledBinaryPath(cache)
	if err != nil {
		fmt.Println("   FAIL:", err)
		os.Exit(1)
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Println("   FAIL: the copy is missing:", err)
		os.Exit(1)
	}
	fmt.Printf("2. It was copied to %s (%d bytes, mode %v)\n", path, info.Size(), info.Mode())

	// The copy has to be executable.
	if info.Mode()&0o111 == 0 {
		fmt.Println("   FAIL: the copy is not executable")
		os.Exit(1)
	}

	// The copy has to be the one that runs.
	mgr := aria2.NewManager(
		aria2.WithBinary(path),
		aria2.WithDataDir(filepath.Join(cache, "data")),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := mgr.Start(ctx); err != nil {
		fmt.Println("   FAIL: the bundled binary did not start:", err)
		os.Exit(1)
	}
	fmt.Printf("3. The bundled binary started: %s %s on port %d\n",
		mgr.Version().Product, mgr.Version().Version, mgr.Port())

	if !mgr.Version().IsAria2Next() {
		fmt.Println("   FAIL: the engine is not aria2-next")
		os.Exit(1)
	}

	if err := mgr.Stop(ctx); err != nil {
		fmt.Println("   FAIL: the sidecar did not stop:", err)
		os.Exit(1)
	}
	fmt.Println("4. The sidecar stopped")
	fmt.Println("\nBUNDLED BINARY: PASS")
}
