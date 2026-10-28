package main

// A check of the torrent flow: adding a .torrent, reading the files
// inside it, choosing some of them, and downloading only those.
//
// The engine only accepts a file selection while the download is
// paused, so the service pauses it, applies the choice and resumes it.
// This check pins that dance, because it is the part of the interface
// that is easiest to break.

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
	bitmaherpb "github.com/incurl/bitmaher/internal/pocketbase"
)

func main() {
	dir, _ := os.MkdirTemp("", "bitmaher-torrent")
	defer os.RemoveAll(dir)

	svc := bitmaherpb.NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		panic(err)
	}
	defer svc.Close()

	torrent, sizes := buildTorrent()
	fmt.Printf("1. A torrent with %d files was built\n", len(sizes))

	binary := findBinary()
	manager := aria2.NewManager(
		aria2.WithBinary(binary),
		aria2.WithDataDir(filepath.Join(dir, "aria2")),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := manager.Start(ctx); err != nil {
		panic(err)
	}
	defer manager.Stop(context.Background())

	store := downloads.NewStore(svc.App())
	service := downloads.NewService(manager, store, nil)

	// The torrent is added paused, with the second file chosen.
	job, err := service.Add(ctx, downloads.AddRequest{
		Kind:          downloads.KindTorrent,
		TorrentBase64: base64.StdEncoding.EncodeToString(torrent),
		TorrentName:   "sample.torrent",
		Dir:           dir,
		SelectedFiles: []int{2},
		Paused:        true,
	})
	if err != nil {
		fmt.Println("   FAIL: the torrent could not be added:", err)
		os.Exit(1)
	}
	fmt.Printf("2. It was added paused, gid %s\n", job.GID)

	// The files of the torrent have to be readable.
	files, err := service.Files(ctx, job.ID)
	if err != nil {
		fmt.Println("   FAIL: the files could not be read:", err)
		os.Exit(1)
	}
	if len(files) != len(sizes) {
		fmt.Printf("   FAIL: %d files, want %d\n", len(files), len(sizes))
		os.Exit(1)
	}
	fmt.Printf("3. The engine reports %d files\n", len(files))
	for _, file := range files {
		fmt.Printf("     %d. %s (%d bytes, selected=%v)\n",
			file.Index, filepath.Base(file.Path), file.Length, file.Selected)
	}

	// Only the chosen file may be selected.
	for _, file := range files {
		want := file.Index == 2
		if file.Selected != want {
			fmt.Printf("   FAIL: file %d is selected=%v, want %v\n", file.Index, file.Selected, want)
			os.Exit(1)
		}
	}
	fmt.Println("4. Only the chosen file is part of the download")

	// The selection is changed, the way the dialog does it.
	if err := service.SelectFiles(ctx, job.ID, []int{1, 3}); err != nil {
		fmt.Println("   FAIL: the selection could not be changed:", err)
		os.Exit(1)
	}
	files, err = service.Files(ctx, job.ID)
	if err != nil {
		panic(err)
	}
	selected := []int{}
	for _, file := range files {
		if file.Selected {
			selected = append(selected, file.Index)
		}
	}
	sort.Ints(selected)
	if len(selected) != 2 || selected[0] != 1 || selected[1] != 3 {
		fmt.Printf("   FAIL: the selection is %v, want [1 3]\n", selected)
		os.Exit(1)
	}
	fmt.Println("5. The selection was changed to 1 and 3")

	// The record kept the choice, which is what the drawer shows.
	stored, err := store.Get(job.ID)
	if err != nil {
		panic(err)
	}
	if len(stored.SelectedFiles) != 2 {
		fmt.Printf("   FAIL: the record holds %v\n", stored.SelectedFiles)
		os.Exit(1)
	}
	fmt.Printf("6. The record holds the choice: %v\n", stored.SelectedFiles)

	fmt.Println("\nTORRENT SELECTION: PASS")
}

// buildTorrent builds a small multi-file torrent.
//
// The pieces are computed over the concatenated stream, which is what
// the format requires.
func buildTorrent() ([]byte, []int) {
	const pieceLength = 16384

	contents := [][]byte{
		bytes.Repeat([]byte("A"), 20000),
		bytes.Repeat([]byte("B"), 30000),
		bytes.Repeat([]byte("C"), 15000),
	}
	names := []string{"first.bin", "second.bin", "third.bin"}

	blob := bytes.Join(contents, nil)
	pieces := []byte{}
	for offset := 0; offset < len(blob); offset += pieceLength {
		end := offset + pieceLength
		if end > len(blob) {
			end = len(blob)
		}
		sum := sha1.Sum(blob[offset:end])
		pieces = append(pieces, sum[:]...)
	}

	files := []any{}
	sizes := []int{}
	for index, content := range contents {
		files = append(files, map[string]any{
			"length": len(content),
			"path":   []any{names[index]},
		})
		sizes = append(sizes, len(content))
	}

	info := map[string]any{
		"name":         "bitmaher-sample",
		"piece length": pieceLength,
		"pieces":       pieces,
		"files":        files,
	}
	torrent := bencode(map[string]any{
		"announce": "http://tracker.example.com/announce",
		"info":     info,
	})
	return torrent, sizes
}

// bencode encodes a value the way a torrent file is laid out.
func bencode(value any) []byte {
	var buffer bytes.Buffer
	encodeValue(&buffer, value)
	return buffer.Bytes()
}

// encodeValue writes one bencoded value.
func encodeValue(buffer *bytes.Buffer, value any) {
	switch typed := value.(type) {
	case string:
		fmt.Fprintf(buffer, "%d:%s", len(typed), typed)
	case int:
		fmt.Fprintf(buffer, "i%de", typed)
	case []byte:
		fmt.Fprintf(buffer, "%d:%s", len(typed), typed)
	case []any:
		buffer.WriteByte('l')
		for _, item := range typed {
			encodeValue(buffer, item)
		}
		buffer.WriteByte('e')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		buffer.WriteByte('d')
		for _, key := range keys {
			encodeValue(buffer, key)
			encodeValue(buffer, typed[key])
		}
		buffer.WriteByte('e')
	}
}

// findBinary works out which aria2-next binary to use.
func findBinary() string {
	if path, err := exec.LookPath("aria2-next"); err == nil {
		return path
	}

	cache := filepath.Join(os.TempDir(), "bitmaher-torrent-binary")
	path, err := aria2.BundledBinaryPath(cache)
	if err != nil {
		fmt.Println("   FAIL: no aria2-next binary was found:", err)
		os.Exit(1)
	}
	return path
}
