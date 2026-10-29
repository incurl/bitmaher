// Package downloads owns the BitMaher download domain: the records the
// application keeps about a download, and the service that drives the
// Aria2-Next sidecar and keeps the records in step with it.
//
// The package owns the domain rules. The engine owns the transfers,
// and PocketBase owns the storage.
//
// # LICENSING BOUNDARY
//
// Nothing in this package links against Aria2-Next. The engine is
// reached through the typed JSON-RPC client of the internal/aria2
// package, over a loopback socket, as a separate process.
package downloads

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// The collections this package owns.
const (
	// CollectionJobs stores the download records.
	CollectionJobs = "download_jobs"
	// CollectionLogs stores the log lines of a download.
	CollectionLogs = "job_logs"
	// CollectionPresets stores the saved BitTorrent presets.
	CollectionPresets = "bt_presets"
)

// The status values stored in download_jobs.status.
//
// They are the normalised ones of the engine, so that the frontend
// never has to know the engine's vocabulary.
const (
	// StatusActive means the download is running.
	StatusActive = "active"
	// StatusWaiting means the download is queued.
	StatusWaiting = "waiting"
	// StatusPaused means the user paused the download.
	StatusPaused = "paused"
	// StatusComplete means the download finished.
	StatusComplete = "complete"
	// StatusError means the download failed.
	StatusError = "error"
	// StatusRemoved means the download was removed.
	StatusRemoved = "removed"
)

// The kinds stored in download_jobs.kind.
const (
	// KindHTTP is a plain HTTP or HTTPS download.
	KindHTTP = "http"
	// KindTorrent is a BitTorrent download from a .torrent file.
	KindTorrent = "torrent"
	// KindMagnet is a BitTorrent download from a magnet link.
	KindMagnet = "magnet"
)

// Job is the frontend-facing view of a download.
//
// It is the normalised model of the engine, with the application's own
// fields added: the record id, the timestamps and the error the
// application recorded.
type Job struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// GID is the engine's handle for the download.
	GID string `json:"gid"`
	// Kind is the kind of the download.
	Kind string `json:"kind"`
	// Status is the normalised status.
	Status string `json:"status"`
	// Title is the display name.
	Title string `json:"title"`
	// URI is the source: the URL or the magnet link.
	URI string `json:"uri"`
	// Dir is the directory the download is written to.
	Dir string `json:"dir"`
	// TotalLength is the size of the download in bytes.
	TotalLength int64 `json:"total_length"`
	// CompletedLength is how many bytes have been downloaded.
	CompletedLength int64 `json:"completed_length"`
	// UploadLength is how many bytes have been uploaded.
	UploadLength int64 `json:"upload_length"`
	// DownloadSpeed is the current download speed in bytes per second.
	DownloadSpeed int64 `json:"download_speed"`
	// UploadSpeed is the current upload speed in bytes per second.
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
	// TorrentFile is the PocketBase file id of the uploaded .torrent.
	//
	// It is empty for a download that has no torrent file.
	TorrentFile string `json:"torrent_file"`
	// SelectedFiles lists the indexes of the files the user chose
	// inside a torrent.
	SelectedFiles []int `json:"selected_files"`
	// Options holds the engine options the download was started with.
	Options map[string]string `json:"options"`
	// Created and Updated are ISO-8601 timestamps.
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// Progress returns the completion ratio of the download, from 0 to 1.
func (j Job) Progress() float64 {
	if j.TotalLength <= 0 {
		return 0
	}
	done := float64(j.CompletedLength) / float64(j.TotalLength)
	if done > 1 {
		return 1
	}
	if done < 0 {
		return 0
	}
	return done
}

// IsActive reports whether the download is doing something.
func (j Job) IsActive() bool {
	return j.Status == StatusActive || j.Status == StatusWaiting || j.Status == StatusPaused
}

// IsFinished reports whether the download has come to rest.
func (j Job) IsFinished() bool {
	return j.Status == StatusComplete || j.Status == StatusError || j.Status == StatusRemoved
}

// Store keeps the download records in PocketBase.
//
// It is the only part of the application that writes to the
// download_jobs collection.
type Store struct {
	app core.App
}

// NewStore creates a store.
func NewStore(app core.App) *Store {
	return &Store{app: app}
}

// Save writes a download record.
//
// The record is created when it does not exist yet, and updated when
// it does. The engine's gid is what links the two: it is the handle
// the engine knows the download by.
func (s *Store) Save(job Job) (Job, error) {
	col, err := s.app.FindCollectionByNameOrId(CollectionJobs)
	if err != nil {
		return Job{}, fmt.Errorf("find the downloads collection: %w", err)
	}

	record, err := s.findByGID(job.GID)
	if err != nil {
		return Job{}, err
	}

	if record == nil {
		record = core.NewRecord(col)
		record.Set("gid", job.GID)
	}

	record.Set("kind", job.Kind)
	record.Set("status", job.Status)
	record.Set("title", job.Title)
	record.Set("uri", job.URI)
	record.Set("dir", job.Dir)
	record.Set("total_length", job.TotalLength)
	record.Set("completed_length", job.CompletedLength)
	record.Set("upload_length", job.UploadLength)
	record.Set("download_speed", job.DownloadSpeed)
	record.Set("upload_speed", job.UploadSpeed)
	record.Set("connections", job.Connections)
	record.Set("seeders", job.Seeders)
	record.Set("peers", job.Peers)
	record.Set("info_hash", job.InfoHash)
	record.Set("error_code", job.ErrorCode)
	record.Set("error_message", job.ErrorMessage)
	if job.TorrentFile != "" {
		record.Set("torrent_file", job.TorrentFile)
	}
	if len(job.SelectedFiles) > 0 {
		record.Set("selected_files", job.SelectedFiles)
	}
	if len(job.Options) > 0 {
		record.Set("options", job.Options)
	}

	if err := s.app.Save(record); err != nil {
		return Job{}, fmt.Errorf("save the download: %w", err)
	}

	return recordToJob(record), nil
}

// findByGID looks a download up by the engine's handle.
//
// A download that is not there is not an error: the record may be
// about to be created. Any other failure is reported, because a broken
// lookup is not the same as a missing download.
func (s *Store) findByGID(gid string) (*core.Record, error) {
	if gid == "" {
		return nil, nil
	}

	record, err := s.app.FindFirstRecordByFilter(
		CollectionJobs,
		"gid = {:gid}",
		map[string]any{"gid": gid},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// The download is not there yet.
			return nil, nil
		}
		return nil, fmt.Errorf("look the download %q up: %w", gid, err)
	}
	return record, nil
}

// GetByGID returns one download by the engine's handle.
//
// The frontend only ever knows the handle, so it is what a download is
// looked up by. A download that is not there is an error, and not an
// empty one: the caller asked for a download it believes exists.
func (s *Store) GetByGID(gid string) (Job, error) {
	record, err := s.findByGID(gid)
	if err != nil {
		return Job{}, fmt.Errorf("find the download %q: %w", gid, err)
	}
	if record == nil {
		return Job{}, fmt.Errorf("find the download %q: it is not known", gid)
	}
	return recordToJob(record), nil
}

// Get returns one download by its record id.
func (s *Store) Get(id string) (Job, error) {
	record, err := s.app.FindRecordById(CollectionJobs, id)
	if err != nil {
		return Job{}, fmt.Errorf("find the download %q: %w", id, err)
	}
	return recordToJob(record), nil
}

// List returns the downloads, newest first.
//
// The limit is capped, so that a caller cannot ask for the whole
// table by accident.
func (s *Store) List(limit int) ([]Job, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	records, err := s.app.FindRecordsByFilter(
		CollectionJobs,
		"",
		"-created",
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("list the downloads: %w", err)
	}

	jobs := make([]Job, 0, len(records))
	for _, record := range records {
		jobs = append(jobs, recordToJob(record))
	}
	return jobs, nil
}

// Delete removes a download record.
//
// The downloaded files are left alone: the engine owns them, and the
// user may still want them.
func (s *Store) Delete(id string) error {
	record, err := s.app.FindRecordById(CollectionJobs, id)
	if err != nil {
		return fmt.Errorf("find the download %q: %w", id, err)
	}
	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("delete the download %q: %w", id, err)
	}
	return nil
}

// DeleteByGID removes the record of a download the engine no longer
// knows about.
//
// It is used when a download is removed from the engine: the record
// would otherwise stay behind as a ghost.
func (s *Store) DeleteByGID(gid string) error {
	record, err := s.findByGID(gid)
	if err != nil {
		return err
	}
	if record == nil {
		return nil
	}
	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("delete the download of %q: %w", gid, err)
	}
	return nil
}

// SaveTorrentFile stores the uploaded .torrent file on a download.
//
// The file goes into the record's own PocketBase file field, so it is
// served by the built-in file route and inherits the collection's
// access rules. No custom file storage is built.
//
// The file name is stored alongside the file, because PocketBase keeps
// the name of an upload in the field itself.
func (s *Store) SaveTorrentFile(jobID, filename string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("the torrent file is empty")
	}

	record, err := s.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		return fmt.Errorf("find the download %q: %w", jobID, err)
	}

	if filename == "" {
		filename = "download.torrent"
	}

	file, err := filesystem.NewFileFromBytes(data, filepath.Base(filename))
	if err != nil {
		return fmt.Errorf("create the torrent file: %w", err)
	}

	record.Set("torrent_file", file)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("save the torrent file: %w", err)
	}
	return nil
}

// TorrentFile returns the uploaded .torrent file of a download.
//
// It is used to start the download again after a restart, when the
// engine needs the torrent that the session file does not hold.
func (s *Store) TorrentFile(jobID string) ([]byte, string, error) {
	record, err := s.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		return nil, "", fmt.Errorf("find the download %q: %w", jobID, err)
	}

	filename := record.GetString("torrent_file")
	if filename == "" {
		return nil, "", fmt.Errorf("the download %q has no torrent file", jobID)
	}

	fsys, err := s.app.NewFilesystem()
	if err != nil {
		return nil, "", fmt.Errorf("create the filesystem: %w", err)
	}
	defer func() { _ = fsys.Close() }()

	reader, err := fsys.GetReader(record.BaseFilesPath() + "/" + filename)
	if err != nil {
		return nil, "", fmt.Errorf("open the torrent file %q: %w", filename, err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("read the torrent file %q: %w", filename, err)
	}

	return data, filename, nil
}

// LogEntry is one line of a download's log.
type LogEntry struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// JobID is the download the line belongs to.
	JobID string `json:"job_id"`
	// Level is the severity of the line.
	Level string `json:"level"`
	// Message is the human readable text.
	Message string `json:"message"`
	// Detail holds the verbatim payload, when there is one.
	Detail string `json:"detail"`
	// Time is when the line was recorded.
	Time string `json:"time"`
}

// AppendLog records a line of a download's log.
//
// The log is what the user looks at when a download misbehaves, so the
// engine's own words are kept next to the application's.
func (s *Store) AppendLog(jobID, level, message, detail string) error {
	col, err := s.app.FindCollectionByNameOrId(CollectionLogs)
	if err != nil {
		return fmt.Errorf("find the log collection: %w", err)
	}

	record := core.NewRecord(col)
	if jobID != "" {
		record.Set("job", jobID)
	}
	record.Set("level", level)
	record.Set("message", message)
	if detail != "" {
		record.Set("detail", detail)
	}

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("save the log entry: %w", err)
	}
	return nil
}

// Logs returns the log of a download, newest first.
func (s *Store) Logs(jobID string, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	filter := ""
	params := dbx.Params{}
	if jobID != "" {
		filter = "job = {:job}"
		params["job"] = jobID
	}

	records, err := s.app.FindRecordsByFilter(
		CollectionLogs,
		filter,
		"-created",
		limit,
		0,
		params,
	)
	if err != nil {
		return nil, fmt.Errorf("list the log entries: %w", err)
	}

	entries := make([]LogEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, LogEntry{
			ID:      record.Id,
			JobID:   record.GetString("job"),
			Level:   record.GetString("level"),
			Message: record.GetString("message"),
			Detail:  record.GetString("detail"),
			Time:    record.GetString("created"),
		})
	}
	return entries, nil
}

// recordToJob converts a PocketBase record into the frontend-facing job.
func recordToJob(record *core.Record) Job {
	job := Job{
		ID:              record.Id,
		GID:             record.GetString("gid"),
		Kind:            record.GetString("kind"),
		Status:          record.GetString("status"),
		Title:           record.GetString("title"),
		URI:             record.GetString("uri"),
		Dir:             record.GetString("dir"),
		TotalLength:     int64(record.GetInt("total_length")),
		CompletedLength: int64(record.GetInt("completed_length")),
		UploadLength:    int64(record.GetInt("upload_length")),
		DownloadSpeed:   int64(record.GetInt("download_speed")),
		UploadSpeed:     int64(record.GetInt("upload_speed")),
		Connections:     record.GetInt("connections"),
		Seeders:         record.GetInt("seeders"),
		Peers:           record.GetInt("peers"),
		InfoHash:        record.GetString("info_hash"),
		ErrorCode:       record.GetInt("error_code"),
		ErrorMessage:    record.GetString("error_message"),
		TorrentFile:     record.GetString("torrent_file"),
		SelectedFiles:   []int{},
		Options:         map[string]string{},
		Created:         record.GetString("created"),
		Updated:         record.GetString("updated"),
	}

	for _, value := range decodeIntSlice(record.Get("selected_files")) {
		job.SelectedFiles = append(job.SelectedFiles, value)
	}

	job.Options = decodeOptions(record.Get("options"))

	return job
}

// decodeIntSlice reads a list of numbers out of a JSON field.
//
// The field may hold a list or a JSON string, depending on how it was
// written, so both are accepted. A value that cannot be read leaves the
// list empty rather than failing the whole record.
func decodeIntSlice(raw any) []int {
	values := []int{}
	if raw == nil {
		return values
	}

	var encoded []byte
	switch typed := raw.(type) {
	case []byte:
		encoded = typed
	case string:
		encoded = []byte(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return values
		}
		encoded = raw
	}

	if len(encoded) == 0 {
		return values
	}

	_ = json.Unmarshal(encoded, &values)
	return values
}

// decodeOptions reads the engine options out of a JSON field.
// The field may hold a map or a JSON string, depending on how it was
// written, so both are accepted. A value that cannot be read leaves the
// options empty rather than failing the whole record.
func decodeOptions(raw any) map[string]string {
	options := map[string]string{}
	if raw == nil {
		return options
	}

	var encoded []byte
	switch typed := raw.(type) {
	case []byte:
		encoded = typed
	case string:
		encoded = []byte(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return options
		}
		encoded = raw
	}

	if len(encoded) == 0 {
		return options
	}

	_ = json.Unmarshal(encoded, &options)
	if options == nil {
		options = map[string]string{}
	}
	return options
}

// Preset is a saved set of download options.
//
// It is the "download the same way again" of the add-task dialog.
type Preset struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// Name is the name of the preset.
	Name string `json:"name"`
	// Kind is the kind of download the preset is for.
	Kind string `json:"kind"`
	// Options holds the engine options of the preset.
	Options map[string]string `json:"options"`
	// Created and Updated are ISO-8601 timestamps.
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// SavePreset stores a preset.
//
// An empty name is refused: a preset without a name cannot be picked
// from the list.
func (s *Store) SavePreset(preset Preset) (Preset, error) {
	name := strings.TrimSpace(preset.Name)
	if name == "" {
		return Preset{}, fmt.Errorf("the preset needs a name")
	}

	col, err := s.app.FindCollectionByNameOrId(CollectionPresets)
	if err != nil {
		return Preset{}, fmt.Errorf("find the presets collection: %w", err)
	}

	record, err := s.app.FindFirstRecordByFilter(
		CollectionPresets,
		"name = {:name}",
		map[string]any{"name": name},
	)
	if err != nil {
		record = core.NewRecord(col)
	}

	record.Set("name", name)
	record.Set("kind", preset.Kind)
	if len(preset.Options) > 0 {
		record.Set("options", preset.Options)
	}

	if err := s.app.Save(record); err != nil {
		return Preset{}, fmt.Errorf("save the preset: %w", err)
	}

	return toPreset(record), nil
}

// Presets returns the saved presets.
func (s *Store) Presets() ([]Preset, error) {
	records, err := s.app.FindRecordsByFilter(CollectionPresets, "", "name", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("list the presets: %w", err)
	}

	presets := make([]Preset, 0, len(records))
	for _, record := range records {
		presets = append(presets, toPreset(record))
	}
	return presets, nil
}

// DeletePreset removes a preset.
func (s *Store) DeletePreset(id string) error {
	record, err := s.app.FindRecordById(CollectionPresets, id)
	if err != nil {
		return fmt.Errorf("find the preset %q: %w", id, err)
	}
	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("delete the preset %q: %w", id, err)
	}
	return nil
}

// toPreset converts a PocketBase record into a preset.
func toPreset(record *core.Record) Preset {
	return Preset{
		ID:      record.Id,
		Name:    record.GetString("name"),
		Kind:    record.GetString("kind"),
		Options: decodeOptions(record.Get("options")),
		Created: record.GetString("created"),
		Updated: record.GetString("updated"),
	}
}
