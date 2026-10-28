package bindings

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
)

// The preview commands.
//
// The add dialog shows what a download would be before the user
// commits to it: the size and the name. The engine makes the request,
// so it goes out with the options and the headers the download itself
// would carry.

// PreviewTimeout bounds one preview.
//
// The engine answers in milliseconds on a fast link and in seconds on
// a slow one, and the user is waiting.
const PreviewTimeout = 25 * time.Second

// PreviewRequest is a request to look ahead at a download.
type PreviewRequest struct {
	// ID names the preview, so that it can be cancelled.
	ID string `json:"id"`
	// URL is the address the download would come from.
	URL string `json:"url"`
	// Options are the engine options the download would carry.
	Options map[string]string `json:"options"`
}

// Preview is what a download from a URL would receive.
type Preview struct {
	// Size is the number of bytes the download would write. It is
	// empty for a page and for a stream playlist, which the engine
	// does not save as they come.
	Size *int64 `json:"size"`
	// Filename is the name the engine would give the file.
	Filename string `json:"filename"`
}

// PreviewDownload looks ahead at a download.
//
// The engine makes the request, so it carries the headers and the
// credentials the download would carry.
func (s *AppService) PreviewDownload(request PreviewRequest) (Preview, error) {
	if request.URL == "" {
		return Preview{}, fmt.Errorf("a download preview takes an address")
	}
	if !strings.HasPrefix(request.URL, "http://") &&
		!strings.HasPrefix(request.URL, "https://") &&
		!strings.HasPrefix(request.URL, "thunder:") {
		return Preview{}, fmt.Errorf("a download preview takes an HTTP or a Thunder address")
	}

	client := s.engineClient()
	if client == nil {
		return Preview{}, aria2.ErrNotRunning
	}

	ctx, cancel := context.WithTimeout(context.Background(), PreviewTimeout)
	defer cancel()

	probe, err := client.Probe(ctx, request.URL, request.Options)
	if err != nil {
		return Preview{}, err
	}

	// The engine reports the status as a string, and "0" means that
	// the server was not reached at all.
	if !strings.HasPrefix(probe.Status, "2") {
		if probe.Status == "0" {
			if probe.Error != "" {
				return Preview{}, fmt.Errorf("no response: %s", probe.Error)
			}
			return Preview{}, fmt.Errorf("no response")
		}
		return Preview{}, fmt.Errorf("the server answered HTTP %s", probe.Status)
	}

	filename, err := s.resolveFilename(ctx, client, request.URL, probe.Filename)
	if err != nil {
		return Preview{}, err
	}

	return Preview{
		Size:     previewedSize(probe),
		Filename: filename,
	}, nil
}

// CancelDownloadPreview stops a preview that is running.
//
// The preview is a request to the engine, and the engine has no way of
// being told to stop one. The answer is that the result is dropped,
// which is what the caller wants.
func (s *AppService) CancelDownloadPreview(id string) {
	if s.console != nil {
		s.console.Info("frontend", "the download preview "+id+" was cancelled")
	}
}

// resolveFilename works out the name the engine would give a file.
//
// The server may suggest a name in its content disposition, and the
// engine decides what to do with it.
func (s *AppService) resolveFilename(
	ctx context.Context,
	client *aria2.Client,
	url string,
	suggested string,
) (string, error) {
	var disposition []byte
	if suggested != "" {
		disposition = []byte("attachment; filename*=UTF-8''" + urlEncode(suggested))
	}

	name, err := client.ResolveFilename(ctx, url, disposition)
	if err != nil {
		return "", err
	}
	return name, nil
}

// previewedSize returns the size a successful probe promises.
//
// A page and a stream playlist are not saved as they come, so they
// have no size.
func previewedSize(probe aria2.Probe) *int64 {
	essence := probe.ContentType
	if index := strings.Index(essence, ";"); index >= 0 {
		essence = essence[:index]
	}
	essence = strings.ToLower(strings.TrimSpace(essence))

	if essence == "text/html" || essence == "application/xhtml+xml" ||
		strings.HasSuffix(essence, "mpegurl") || essence == "application/dash+xml" {
		return nil
	}

	if probe.TotalLength == "" {
		return nil
	}

	size, err := parseInt64(probe.TotalLength)
	if err != nil {
		return nil
	}
	return &size
}

// TaskFileState is the state of the file of a download on the disk.
type TaskFileState string

const (
	// FileAvailable means the file is there.
	FileAvailable TaskFileState = "available"
	// FileMissing means the file is not there.
	FileMissing TaskFileState = "missing"
	// FileInaccessible means the file is there, but it cannot be read.
	FileInaccessible TaskFileState = "inaccessible"
	// FileUnknown means the state could not be worked out.
	FileUnknown TaskFileState = "unknown"
)

// TaskFileStates returns the state of the files of the given downloads.
//
// The interface shows a mark next to a download whose file has gone,
// and it asks the backend because the webview cannot look at the disk.
func (s *AppService) TaskFileStates(gids []string) map[string]TaskFileState {
	states := map[string]TaskFileState{}

	for _, gid := range gids {
		states[gid] = FileUnknown

		files, err := s.downloads.Files(context.Background(), gid)
		if err != nil || len(files) == 0 {
			continue
		}

		// The state of the download is the state of its files: a
		// download is available when its files are.
		state := FileAvailable
		for _, file := range files {
			fileState := fileState(file.Path)
			if fileState == FileMissing {
				state = FileMissing
				break
			}
			if fileState == FileInaccessible {
				state = FileInaccessible
			}
		}

		states[gid] = state
	}

	return states
}

// fileState works out the state of a path on the disk.
func fileState(path string) TaskFileState {
	if path == "" {
		return FileUnknown
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileMissing
		}
		// The file is there, but it cannot be read.
		return FileInaccessible
	}

	if !info.Mode().IsRegular() {
		return FileAvailable
	}

	// The file is opened, so that a file the user may not read is
	// told apart from one that is gone.
	file, err := os.Open(path)
	if err != nil {
		return FileInaccessible
	}
	defer file.Close()

	return FileAvailable
}
