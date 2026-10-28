package bindings

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/downloads"
)

// DownloadView is a download as the frontend sees it.
//
// It is the downloads.Job model with the fields the frontend needs
// spelled out, so that the generated TypeScript is useful.
type DownloadView struct {
	// ID is the record id.
	ID string `json:"id"`
	// GID is the engine's handle.
	GID string `json:"gid"`
	// Kind is "http", "torrent" or "magnet".
	Kind string `json:"kind"`
	// Status is "active", "waiting", "paused", "complete", "error" or
	// "removed".
	Status string `json:"status"`
	// Title is the display name.
	Title string `json:"title"`
	// URI is the source.
	URI string `json:"uri"`
	// Dir is the directory the download is written to.
	Dir string `json:"dir"`
	// TotalLength is the size in bytes.
	TotalLength int64 `json:"total_length"`
	// CompletedLength is how many bytes have been downloaded.
	CompletedLength int64 `json:"completed_length"`
	// UploadLength is how many bytes have been uploaded.
	UploadLength int64 `json:"upload_length"`
	// DownloadSpeed is the speed in bytes per second.
	DownloadSpeed int64 `json:"download_speed"`
	// UploadSpeed is the upload speed in bytes per second.
	UploadSpeed int64 `json:"upload_speed"`
	// Connections is the number of connections.
	Connections int `json:"connections"`
	// Seeders is the number of seeders of a BitTorrent download.
	Seeders int `json:"seeders"`
	// Peers is the number of peers of a BitTorrent download.
	Peers int `json:"peers"`
	// InfoHash is the BitTorrent info hash.
	InfoHash string `json:"info_hash"`
	// ErrorCode is the engine's error code.
	ErrorCode int `json:"error_code"`
	// ErrorMessage is the human readable failure explanation.
	ErrorMessage string `json:"error_message"`
	// SelectedFiles lists the indexes of the files chosen inside a
	// torrent.
	SelectedFiles []int `json:"selected_files"`
	// Options holds the engine options of the download.
	Options map[string]string `json:"options"`
	// Progress is the completion ratio, from 0 to 1.
	Progress float64 `json:"progress"`
	// Created and Updated are ISO-8601 timestamps.
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// toView converts a stored download into the frontend-facing view.
func toView(job downloads.Job) DownloadView {
	return DownloadView{
		ID:              job.ID,
		GID:             job.GID,
		Kind:            job.Kind,
		Status:          job.Status,
		Title:           job.Title,
		URI:             job.URI,
		Dir:             job.Dir,
		TotalLength:     job.TotalLength,
		CompletedLength: job.CompletedLength,
		UploadLength:    job.UploadLength,
		DownloadSpeed:   job.DownloadSpeed,
		UploadSpeed:     job.UploadSpeed,
		Connections:     job.Connections,
		Seeders:         job.Seeders,
		Peers:           job.Peers,
		InfoHash:        job.InfoHash,
		ErrorCode:       job.ErrorCode,
		ErrorMessage:    job.ErrorMessage,
		SelectedFiles:   job.SelectedFiles,
		Options:         job.Options,
		Progress:        job.Progress(),
		Created:         job.Created,
		Updated:         job.Updated,
	}
}

// toViews converts a list of stored downloads.
func toViews(jobs []downloads.Job) []DownloadView {
	views := make([]DownloadView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, toView(job))
	}
	return views
}

// AddDownloadRequest is a request to start a download.
//
// It is the frontend's view of the domain request: the fields are
// named the way the UI thinks of them, and the backend turns them
// into the engine's own options.
type AddDownloadRequest struct {
	// Kind is "http", "torrent" or "magnet".
	Kind string `json:"kind"`
	// URI is the URL or the magnet link.
	URI string `json:"uri"`
	// TorrentName is the name of the uploaded .torrent file.
	TorrentName string `json:"torrent_name"`
	// TorrentData is the uploaded .torrent file, base64 encoded.
	//
	// The file is sent as a base64 string because the Wails bindings
	// carry JSON. It is stored in a PocketBase file field, so the
	// built-in file route serves it.
	TorrentData string `json:"torrent_data"`
	// Dir is the directory the download is written to.
	Dir string `json:"dir"`
	// Out is the name of the file, for a single file download.
	Out string `json:"out"`
	// SelectedFiles lists the indexes of the files chosen inside a
	// torrent.
	SelectedFiles []int `json:"selected_files"`
	// Options holds the advanced engine options.
	Options map[string]string `json:"options"`
	// Paused starts the download paused.
	Paused bool `json:"paused"`
}

// FileView is one file of a download.
type FileView struct {
	// Index is the file's one-based index.
	Index int `json:"index"`
	// Path is the absolute path of the file.
	Path string `json:"path"`
	// Length is the size of the file in bytes.
	Length int64 `json:"length"`
	// CompletedLength is how many bytes of it have been downloaded.
	CompletedLength int64 `json:"completed_length"`
	// Selected reports whether the file is part of the download.
	Selected bool `json:"selected"`
}

// toFileViews converts the engine's files.
func toFileViews(files []aria2.File) []FileView {
	views := make([]FileView, 0, len(files))
	for _, file := range files {
		views = append(views, FileView{
			Index:           file.Index,
			Path:            file.Path,
			Length:          file.Length,
			CompletedLength: file.CompletedLength,
			Selected:        file.Selected,
		})
	}
	return views
}

// ListDownloads returns the recorded downloads, newest first.
//
// The frontend polls this call: the engine does not push its updates,
// so the application has to ask for them.
func (s *AppService) ListDownloads(limit int) ([]DownloadView, error) {
	jobs, err := s.downloads.List(limit)
	if err != nil {
		return nil, err
	}
	return toViews(jobs), nil
}

// SyncDownloads reads the state of the engine and updates the records.
//
// It returns the downloads whose state changed, so that the frontend
// only has to look at those.
func (s *AppService) SyncDownloads(ctx context.Context) ([]DownloadView, error) {
	jobs, err := s.downloads.Sync(ctx)
	if err != nil {
		return nil, err
	}
	return toViews(jobs), nil
}

// AddDownload starts a download.
//
// The uploaded .torrent file is stored in a PocketBase file field, so
// that it is served by the built-in file route and kept with the
// record.
func (s *AppService) AddDownload(ctx context.Context, request AddDownloadRequest) (DownloadView, error) {
	domain := downloads.AddRequest{
		Kind:          request.Kind,
		URI:           strings.TrimSpace(request.URI),
		TorrentName:   request.TorrentName,
		TorrentBase64: request.TorrentData,
		Dir:           strings.TrimSpace(request.Dir),
		Out:           strings.TrimSpace(request.Out),
		SelectedFiles: request.SelectedFiles,
		Options:       request.Options,
		Paused:        request.Paused,
	}

	// The default directory is used when the frontend did not choose
	// one.
	if domain.Dir == "" {
		settings, err := s.settings.GetAria2()
		if err != nil {
			return DownloadView{}, err
		}
		domain.Dir = settings.DownloadDir
	}

	job, err := s.downloads.Add(ctx, domain)
	if err != nil {
		return DownloadView{}, err
	}

	// The torrent file is kept with the record, so that the download
	// can be recognised after a restart.
	if request.TorrentData != "" {
		data, decodeErr := decodeBase64(request.TorrentData)
		if decodeErr != nil {
			s.console.Warn("downloads", "the torrent file could not be read: "+decodeErr.Error())
		} else if saveErr := s.downloads.Store().SaveTorrentFile(job.ID, request.TorrentName, data); saveErr != nil {
			// The download itself worked, so the file is a
			// detail rather than a failure.
			s.console.Warn("downloads", "the torrent file could not be stored: "+saveErr.Error())
		}
	}

	return toView(job), nil
}

// PauseDownload pauses a download.
func (s *AppService) PauseDownload(ctx context.Context, id string) error {
	return s.downloads.Pause(ctx, id)
}

// ResumeDownload resumes a paused download.
func (s *AppService) ResumeDownload(ctx context.Context, id string) error {
	return s.downloads.Resume(ctx, id)
}

// RemoveDownload removes a download from the engine and forgets it.
//
// The downloaded files are left on disk.
func (s *AppService) RemoveDownload(ctx context.Context, id string) error {
	return s.downloads.Remove(ctx, id)
}

// PurgeDownloads removes the finished downloads.
//
// The downloaded files are left on disk.
func (s *AppService) PurgeDownloads(ctx context.Context) error {
	return s.downloads.Purge(ctx)
}

// DownloadFiles returns the files of a download.
//
// It is what the add-task dialog shows when the user chooses the files
// of a torrent.
func (s *AppService) DownloadFiles(ctx context.Context, id string) ([]FileView, error) {
	files, err := s.downloads.Files(ctx, id)
	if err != nil {
		return nil, err
	}
	return toFileViews(files), nil
}

// SelectDownloadFiles chooses the files of a torrent.
//
// The engine only accepts the choice while the download is paused, so
// the backend pauses it, changes it and resumes it again.
func (s *AppService) SelectDownloadFiles(ctx context.Context, id string, indexes []int) error {
	return s.downloads.SelectFiles(ctx, id, indexes)
}

// DownloadStat returns the engine's global state.
//
// The frontend shows it in the header, so that the user sees the total
// speed without opening a download.
func (s *AppService) DownloadStat(ctx context.Context) (aria2.GlobalStat, error) {
	return s.downloads.Stat(ctx)
}

// ListPresets returns the saved download presets.
func (s *AppService) ListPresets() ([]downloads.Preset, error) {
	return s.downloads.Presets()
}

// SavePreset stores a download preset.
func (s *AppService) SavePreset(preset downloads.Preset) (downloads.Preset, error) {
	return s.downloads.SavePreset(preset)
}

// DeletePreset removes a download preset.
func (s *AppService) DeletePreset(id string) error {
	return s.downloads.DeletePreset(id)
}

// decodeBase64 decodes a base64 string.
//
// The frontend sends the uploaded file as a base64 string, because the
// Wails bindings carry JSON.
func decodeBase64(encoded string) ([]byte, error) {
	cleaned := strings.TrimSpace(encoded)
	if cleaned == "" {
		return nil, nil
	}

	// A data URL is accepted as well, because that is what a browser
	// hands out when a file is read from an input.
	if index := strings.Index(cleaned, ","); strings.HasPrefix(cleaned, "data:") && index > 0 {
		cleaned = cleaned[index+1:]
	}

	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("the uploaded file is not valid base64: %w", err)
	}
	return data, nil
}
