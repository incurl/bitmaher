package main

// A check that the sidecar does not outlive the application, including
// after a watchdog restart.
//
// The sidecar is a child process under the GPL boundary. If it survived
// the application, it would keep transferring data with nobody driving
// it, and the boundary would look like a leak.

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
	dir, _ := os.MkdirTemp("", "bitmaher-leak")
	defer os.RemoveAll(dir)

	mgr := aria2.NewManager(
		aria2.WithBinary("/usr/bin/aria2-next"),
		aria2.WithDataDir(filepath.Join(dir, "aria2")),
		aria2.WithAutoRestart(true),
	)
	ctx := context.Background()

	if _, err := mgr.Start(ctx); err != nil {
		panic(err)
	}
	first := mgr.PID()
	fmt.Printf("1. Started, pid %d\n", first)

	// Force a watchdog restart.
	if err := syscall.Kill(-first, syscall.SIGKILL); err != nil {
		panic(err)
	}
	fmt.Println("2. Killed it, so the watchdog restarts it")

	var second int
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)
		if mgr.Ready() && mgr.PID() != first && mgr.PID() != 0 {
			second = mgr.PID()
			break
		}
	}
	if second == 0 {
		fmt.Println("   FAIL: the watchdog did not restart it")
		os.Exit(1)
	}
	fmt.Printf("3. Restarted, pid %d\n", second)

	// The application closes.
	if err := mgr.Stop(ctx); err != nil {
		fmt.Println("   FAIL: stop:", err)
		os.Exit(1)
	}
	fmt.Println("4. Stop returned")

	// Nothing may be left behind.
	time.Sleep(1 * time.Second)
	for _, pid := range []int{first, second} {
		if alive(pid) {
			fmt.Printf("   FAIL: the process %d outlived the application\n", pid)
			syscall.Kill(pid, syscall.SIGKILL)
			os.Exit(1)
		}
	}
	fmt.Println("5. No sidecar outlived the application")
	fmt.Println("\nNO LEAK: PASS")
}

// alive reports whether a process is still running.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}
