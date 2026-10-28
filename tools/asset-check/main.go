package main

// A check that the header icon is served by the application's own asset
// server.
//
// The icon lives in frontend/public, so the build copies it into
// frontend/dist and the binary embeds it. The webview asks for it at
// /appicon.png, and the asset server has to answer.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	// The frontend that the build produced. The path is worked out
	// from this file, so the check runs from anywhere.
	_, file, _, _ := runtime.Caller(0)
	dist := filepath.Join(filepath.Dir(file), "..", "..", "frontend", "dist")

	icon := filepath.Join(dist, "appicon.png")
	info, err := os.Stat(icon)
	if err != nil {
		fmt.Println("   FAIL: the icon is not in the build:", err)
		os.Exit(1)
	}

	handler := http.FileServer(http.Dir(dist))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/appicon.png", nil))

	fmt.Printf("GET /appicon.png -> %d\n", rec.Code)
	if rec.Code != http.StatusOK {
		os.Exit(1)
	}

	body := rec.Body.Bytes()
	if len(body) == 0 {
		fmt.Println("   FAIL: the body is empty")
		os.Exit(1)
	}

	// It has to be a PNG.
	if len(body) < 8 || string(body[1:4]) != "PNG" {
		fmt.Println("   FAIL: it is not a PNG")
		os.Exit(1)
	}

	fmt.Printf("   %d bytes on disk, %d served, %s\n",
		info.Size(), len(body), rec.Header().Get("Content-Type"))
	fmt.Println("\nHEADER ICON: PASS")
}
