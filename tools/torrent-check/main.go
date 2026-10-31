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

	// A torrent that has finished keeps running, so that it shares the
	// content it has. The engine reports it as "active" with the flag
	// that says it is seeding, and the application has to record it as
	// the completed download it is.
	if err := checkSeedingTorrent(ctx, service, dir); err != nil {
		fmt.Println("   FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("7. A seeding torrent is recorded as the completed download it is")

	fmt.Println("\nTORRENT SELECTION: PASS")
}

// checkSeedingTorrent verifies that a torrent that is sharing the content
// it already has is recorded as a completed download.
//
// The engine keeps a finished torrent running so that it seeds, and the
// status stays "active" for as long as it does. The flag that says it is
// seeding is the only thing that says that the download itself is over,
// and it was dropped: the download stayed in the progress list for ever
// and the completed list was empty.
func checkSeedingTorrent(ctx context.Context, service *downloads.Service, dir string) error {
	// The content is already there, so the engine verifies the pieces
	// and goes straight to seeding.
	content := bytes.Repeat([]byte("S"), 32768)
	pieceLength := 16384
	pieces := []byte{}
	for offset := 0; offset < len(content); offset += pieceLength {
		end := offset + pieceLength
		if end > len(content) {
			end = len(content)
		}
		sum := sha1.Sum(content[offset:end])
		pieces = append(pieces, sum[:]...)
	}
	torrent := bencode(map[string]any{
		"info": map[string]any{
			"name":         "seeding.bin",
			"piece length": pieceLength,
			"pieces":       pieces,
			"length":       len(content),
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "seeding.bin"), content, 0o600); err != nil {
		return fmt.Errorf("write the content: %w", err)
	}

	job, err := service.Add(ctx, downloads.AddRequest{
		Kind:          downloads.KindTorrent,
		TorrentBase64: base64.StdEncoding.EncodeToString(torrent),
		TorrentName:   "seeding.torrent",
		Dir:           dir,
	})
	if err != nil {
		return fmt.Errorf("add the torrent: %w", err)
	}

	// The engine needs a moment to verify the pieces and to start
	// seeding.
	seeding := false
	for i := 0; i < 40; i++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := service.Sync(ctx); err != nil {
			return fmt.Errorf("sync: %w", err)
		}
		stored, err := service.Get(job.ID)
		if err != nil {
			return fmt.Errorf("read the record: %w", err)
		}
		if stored.Seeder {
			seeding = true
			break
		}
	}
	if !seeding {
		return fmt.Errorf("the torrent never started seeding")
	}

	// The download is in the history, as the completed one it is.
	records, err := service.History().Records(downloads.HistoryStatusComplete)
	if err != nil {
		return fmt.Errorf("read the history: %w", err)
	}
	for _, record := range records {
		if record.GID == job.GID {
			return nil
		}
	}
	return fmt.Errorf("the seeding torrent is not in the history")
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
