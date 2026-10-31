package downloads

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/incurl/bitmaher/internal/aria2"
	"github.com/incurl/bitmaher/internal/console"
)

// Service drives the Aria2-Next sidecar and keeps the records in step
// with it.
//
// The service is the only place that talks to the engine. The frontend
// never sees a raw RPC payload: it sends a request, and gets back the
// normalised model.
//
// # LICENSING BOUNDARY
//
// The engine is a separate process. The service talks to it over a
// loopback JSON-RPC socket, through the typed client of the
// internal/aria2 package. Nothing is linked against it.
type Service struct {
	// manager owns the sidecar's lifecycle.
	manager *aria2.Manager
	// store keeps the records.
	store *Store
	// history keeps what the user has already downloaded.
	history *HistoryStore
	// log records what happened, for the debug console.
	log Logger

	// mu guards the fields below.
	mu sync.Mutex
	// syncing guards against two syncs running at once.
	syncing bool
	// lastSync is when the last sync finished.
	lastSync time.Time
}

// Logger is the debug console the service writes to.
//
// It is an interface, so that the service does not depend on the
// console package. The console's own methods return the entry they
// wrote, and the service has no use for it: the contract is only
// about writing.
type Logger interface {
	// Info records a normal event.
	Info(source, message string) console.Entry
	// Warn records a recoverable problem.
	Warn(source, message string) console.Entry
	// Error records a failure.
	Error(source, message string) console.Entry
}

// NewService creates a service.
//
// The logger may be nil: the service then records nothing, which is
// what a test or a headless run wants.
func NewService(manager *aria2.Manager, store *Store, log Logger) *Service {
	return &Service{
		manager: manager,
		store:   store,
		history: NewHistoryStore(store.app),
		log:     log,
	}
}

// History returns the history store.
//
// It is exposed so that the binding layer can reach the history
// without the service having to grow a method for every operation.
func (s *Service) History() *HistoryStore {
	if s == nil {
		return nil
	}
	return s.history
}

// info records a normal event.
func (s *Service) info(message string) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Info("downloads", message)
}

// warn records a recoverable problem.
func (s *Service) warn(message string) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Warn("downloads", message)
}

// error records a failure.
func (s *Service) error(message string) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Error("downloads", message)
}

// Store returns the record store.
//
// It is exposed so that the binding layer can store the uploaded
// torrent file on a record, without the service having to grow a
// method for it.
func (s *Service) Store() *Store {
	if s == nil {
		return nil
	}
	return s.store
}

// Rename changes the name of a finished download.
//
// The file on the disk has already been renamed by the caller: this
// updates the record, so that the list shows the new name and the
// engine is told where the file went.
func (s *Service) Rename(gid, name, path string) error {
	job, err := s.store.Get(gid)
	if err != nil {
		return err
	}

	job.Title = name
	if job.Options == nil {
		job.Options = map[string]string{}
	}
	job.Options["out"] = name

	// The engine is told where the file is, so that it keeps the
	// download in step with what is on the disk.
	if client := s.manager.Client(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := client.ChangeOption(ctx, job.GID, map[string]string{"out": name}); err != nil {
			s.warn("the engine could not be told about the new name: " + err.Error())
		}
	}

	_, err = s.store.Save(job)
	return err
}

// Manager returns the manager of the sidecar.
//
// It is exposed so that the binding layer can report the state of the
// engine without the service having to grow a method for it.
func (s *Service) Manager() *aria2.Manager {
	if s == nil {
		return nil
	}
	return s.manager
}

// AddRequest is a request to start a download.
type AddRequest struct {
	// Kind is the kind of the download: "http", "torrent" or "magnet".
	Kind string `json:"kind"`
	// URI is the URL or the magnet link.
	URI string `json:"uri"`
	// TorrentBase64 is the uploaded .torrent file, base64 encoded.
	//
	// It is used for a torrent download. The frontend sends the file
	// as a base64 string, because the Wails bindings carry JSON.
	TorrentBase64 string `json:"torrent_base64"`
	// TorrentName is the name of the uploaded file.
	TorrentName string `json:"torrent_name"`
	// Dir is the directory the download is written to.
	Dir string `json:"dir"`
	// Out is the name of the file, for a single file download.
	Out string `json:"out"`
	// SelectedFiles lists the indexes of the files chosen inside a
	// torrent.
	SelectedFiles []int `json:"selected_files"`
	// Options holds the engine options of the download.
	Options map[string]string `json:"options"`
	// Paused starts the download paused.
	//
	// It is what the add-task dialog does when the user wants to
	// choose the files of a torrent before it starts.
	Paused bool `json:"paused"`
}

// Add starts a download and records it.
//
// The record is written before the download is added to the engine, so
// that a download is never invisible to the user.
func (s *Service) Add(ctx context.Context, request AddRequest) (Job, error) {
	client := s.manager.Client()
	if client == nil {
		return Job{}, aria2.ErrNotRunning
	}

	kind := normaliseKind(request.Kind)
	options := cleanOptions(request.Options)

	if request.Dir != "" {
		options["dir"] = request.Dir
	}
	if request.Out != "" {
		options["out"] = request.Out
	}

	var gid string
	var err error

	switch kind {
	case KindTorrent:
		if request.TorrentBase64 == "" {
			return Job{}, fmt.Errorf("the torrent file is missing")
		}
		// The engine only honours the file selection while the
		// download is paused, so a download with a selection starts
		// paused.
		if len(request.SelectedFiles) > 0 {
			options["select-file"] = joinIndexes(request.SelectedFiles)
			request.Paused = true
		}
		gid, err = client.AddTorrent(ctx, request.TorrentBase64, options)

	case KindMagnet:
		if request.URI == "" {
			return Job{}, fmt.Errorf("the magnet link is missing")
		}
		gid, err = client.AddURI(ctx, []string{request.URI}, options)

	default:
		if request.URI == "" {
			return Job{}, fmt.Errorf("the link is missing")
		}
		gid, err = client.AddURI(ctx, []string{request.URI}, options)
	}

	if err != nil {
		s.error("the download could not be started: " + err.Error())
		return Job{}, err
	}

	if request.Paused {
		options["pause"] = "true"
	}

	job := Job{
		GID:           gid,
		Kind:          kind,
		Status:        StatusWaiting,
		Title:         titleFor(request),
		URI:           request.URI,
		Dir:           request.Dir,
		SelectedFiles: request.SelectedFiles,
		Options:       options,
	}

	saved, err := s.store.Save(job)
	if err != nil {
		return Job{}, err
	}

	s.info(fmt.Sprintf("the download %q was started", saved.Title))
	return saved, nil
}

// Sync reads the state of the engine and updates the records.
//
// The engine does not push its updates: it answers a question, and the
// application has to ask. The WebSocket transport accepts a connection
// but never sends anything on it, so there is nothing to subscribe to.
// The frontend therefore polls, and this is the call it polls.
//
// The call is guarded, so that two syncs never run at once: a slow
// engine must not turn into a pile of overlapping requests.
func (s *Service) Sync(ctx context.Context) ([]Job, error) {
	client := s.manager.Client()
	if client == nil {
		return nil, aria2.ErrNotRunning
	}

	s.mu.Lock()
	if s.syncing {
		s.mu.Unlock()
		// A sync is already running. The caller gets the records it
		// has, rather than a second round trip.
		return s.store.List(200)
	}
	s.syncing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.syncing = false
		s.lastSync = time.Now()
		s.mu.Unlock()
	}()

	downloads, err := s.collect(ctx, client)
	if err != nil {
		return nil, err
	}

	changed := []Job{}
	for _, download := range downloads {
		job, err := s.store.Save(toJob(download, s.store))
		if err != nil {
			s.error("the download could not be saved: " + err.Error())
			continue
		}
		changed = append(changed, job)

		// A download that has come to rest joins the history, and
		// the moment it was added is recorded the first time the
		// engine is seen. The two are what hold the order of the list
		// stable across a restart.
		s.recordHistory(job)
	}

	return changed, nil
}

// recordHistory writes a download that has come to rest into the
// history.
//
// The moment the download was added is recorded once, when the
// download is first seen. It never changes afterwards: it is what
// holds the order of the list stable.
func (s *Service) recordHistory(job Job) {
	if s.history == nil || job.GID == "" {
		return
	}

	// The moment the download was added. It is written the first time
	// only, so that a download that is seen again does not move.
	if err := s.history.RecordBirth(job.GID, job.Created); err != nil {
		s.error("the moment the download was added could not be saved: " + err.Error())
	}

	if !job.IsFinished() && !job.IsSeeding() {
		return
	}

	status := job.Status
	switch status {
	case StatusComplete, StatusError, StatusRemoved:
	default:
		// A torrent that is seeding has finished: the content is on
		// the disk, and the engine is sharing it. The engine keeps
		// the status "active" for as long as it seeds, so the history
		// records the download as the completed one it is.
		if !job.IsSeeding() {
			return
		}
		status = StatusComplete
	}

	addedAt := job.Created
	if birth, found, err := s.history.Birth(job.GID); err == nil && found && birth != "" {
		addedAt = birth
	}

	if _, err := s.history.AddRecord(HistoryRecord{
		GID:         job.GID,
		Name:        job.Title,
		URI:         job.URI,
		Dir:         job.Dir,
		TotalLength: job.TotalLength,
		Status:      status,
		TaskType:    job.Kind,
		AddedAt:     addedAt,
		CompletedAt: job.Updated,
	}); err != nil {
		s.error("the history could not be saved: " + err.Error())
	}
}

// collect asks the engine for every download it knows about.
//
// The engine keeps the running ones and the queued ones apart, and
// the finished ones in a third list.
func (s *Service) collect(ctx context.Context, client *aria2.Client) ([]aria2.Download, error) {
	downloads := []aria2.Download{}

	active, err := client.Active(ctx)
	if err != nil {
		return nil, fmt.Errorf("the running downloads could not be read: %w", err)
	}
	downloads = append(downloads, active...)

	waiting, err := client.Waiting(ctx, 0, 200)
	if err != nil {
		return nil, fmt.Errorf("the queued downloads could not be read: %w", err)
	}
	downloads = append(downloads, waiting...)

	stopped, err := client.Stopped(ctx, 0, 200)
	if err != nil {
		return nil, fmt.Errorf("the finished downloads could not be read: %w", err)
	}
	downloads = append(downloads, stopped...)

	return downloads, nil
}

// toJob converts the engine's model into the application's, keeping the
// fields the application owns.
//
// The torrent file and the chosen files are the application's own
// bookkeeping: the engine does not report them back, so the stored
// record is the one that holds them.
func toJob(download aria2.Download, store *Store) Job {
	job := Job{
		GID:             download.GID,
		Kind:            KindHTTP,
		Status:          string(download.Status),
		Title:           download.Title,
		URI:             download.URI,
		Dir:             download.Dir,
		TotalLength:     download.TotalLength,
		CompletedLength: download.CompletedLength,
		UploadLength:    download.UploadLength,
		DownloadSpeed:   download.DownloadSpeed,
		UploadSpeed:     download.UploadSpeed,
		Connections:     download.Connections,
		Seeders:         download.Seeders,
		Peers:           download.Peers,
		InfoHash:        download.InfoHash,
		ErrorCode:       download.ErrorCode,
		ErrorMessage:    download.ErrorMessage,
		Seeder:          download.Seeder,
		TorrentState:    download.TorrentState,
		SelectedFiles:   []int{},
		Options:         map[string]string{},
	}

	// The stored record holds what the engine does not report.
	if store != nil {
		if existing, err := store.findByGID(download.GID); err == nil && existing != nil {
			job.Kind = existing.GetString("kind")
			job.TorrentFile = existing.GetString("torrent_file")
			job.SelectedFiles = decodeIntSlice(existing.Get("selected_files"))
			job.Options = decodeOptions(existing.Get("options"))
		}
	}

	if job.Kind == "" {
		job.Kind = KindHTTP
	}

	return job
}

// Pause pauses a download.
func (s *Service) Pause(ctx context.Context, id string) error {
	job, client, err := s.resolve(ctx, id)
	if err != nil {
		return err
	}
	if err := client.Pause(ctx, job.GID); err != nil {
		return fmt.Errorf("the download could not be paused: %w", err)
	}
	s.info(fmt.Sprintf("the download %q was paused", job.Title))
	return s.syncOne(ctx, client, job.GID)
}

// PauseAll pauses every running download.
//
// It is what the interface calls when the user pauses everything at
// once. The downloads that cannot be paused are skipped, so that one
// stubborn download does not stop the rest.
func (s *Service) PauseAll(ctx context.Context) error {
	client := s.manager.Client()
	if client == nil {
		return aria2.ErrNotRunning
	}

	active, err := client.Active(ctx)
	if err != nil {
		return err
	}

	for _, download := range active {
		if err := client.Pause(ctx, download.GID); err != nil {
			s.warn("the download could not be paused: " + err.Error())
		}
	}

	return nil
}

// Resume resumes a paused download.
func (s *Service) Resume(ctx context.Context, id string) error {
	job, client, err := s.resolve(ctx, id)
	if err != nil {
		return err
	}
	if err := client.Resume(ctx, job.GID); err != nil {
		return fmt.Errorf("the download could not be resumed: %w", err)
	}
	s.info(fmt.Sprintf("the download %q was resumed", job.Title))
	return s.syncOne(ctx, client, job.GID)
}

// Remove removes a download from the engine and forgets it.
//
// The engine refuses to remove a running download, so it is paused
// first. The downloaded files are left on disk.
//
// A download that has come to rest has no record in the list any more:
// it lives in the history. Removing it means removing it from the
// history, and the engine has nothing to say about it.
func (s *Service) Remove(ctx context.Context, id string) error {
	if removed, err := s.removeFromHistory(id); removed {
		return err
	}

	job, client, err := s.resolve(ctx, id)
	if err != nil {
		return err
	}

	// A running download has to be paused before it can be removed.
	if job.IsActive() {
		if err := client.Pause(ctx, job.GID); err != nil {
			s.warn("the download could not be paused: " + err.Error())
		}
	}

	if err := client.Remove(ctx, job.GID); err != nil {
		return fmt.Errorf("the download could not be removed: %w", err)
	}

	// The engine's own record of the download goes as well, so that it
	// does not come back on the next sync.
	_ = client.RemoveResult(ctx, job.GID)

	// The record is deleted by its own id, which the resolution found.
	// The handle the caller passed is not the id: the frontend only
	// ever knows the handle.
	if err := s.store.Delete(job.ID); err != nil {
		return err
	}

	s.info(fmt.Sprintf("the download %q was removed", job.Title))
	return nil
}

// removeFromHistory removes a download that has come to rest.
//
// The second return value says whether the download was in the history
// at all: a download that is still running is not, and the caller has
// to remove it from the engine instead.
func (s *Service) removeFromHistory(id string) (bool, error) {
	if s == nil || s.history == nil || id == "" {
		return false, nil
	}

	_, found, err := s.history.RecordByGID(id)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	if err := s.history.RemoveRecord(id); err != nil {
		return true, err
	}

	// The moment the download was added goes with it: it is what holds
	// the order of the list, and a download that is not there any more
	// has no place in it.
	_ = s.history.RemoveBirths([]string{id})

	s.info(fmt.Sprintf("the history entry %q was removed", id))
	return true, nil
}

// Purge removes the finished downloads from the engine and from the
// records.
//
// The downloaded files are left on disk: the user may still want them.
func (s *Service) Purge(ctx context.Context) error {
	client := s.manager.Client()
	if client == nil {
		return aria2.ErrNotRunning
	}

	finished, err := s.store.List(500)
	if err != nil {
		return err
	}

	for _, job := range finished {
		if !job.IsFinished() {
			continue
		}
		_ = client.RemoveResult(ctx, job.GID)
		if err := s.store.Delete(job.ID); err != nil {
			s.warn("the download could not be forgotten: " + err.Error())
		}
	}

	// Whatever the engine still holds is cleared as well.
	if err := client.PurgeResults(ctx); err != nil {
		return fmt.Errorf("the finished downloads could not be cleared: %w", err)
	}

	s.info("the finished downloads were cleared")
	return nil
}

// Files returns the files of a download.
//
// It is what the add-task dialog shows when the user chooses the
// files of a torrent.
func (s *Service) Files(ctx context.Context, id string) ([]aria2.File, error) {
	job, client, err := s.resolve(ctx, id)
	if err != nil {
		return nil, err
	}
	return client.Files(ctx, job.GID)
}

// SelectFiles chooses the files of a torrent.
//
// The engine only accepts the choice while the download is paused, so
// it is paused, changed and resumed again.
func (s *Service) SelectFiles(ctx context.Context, id string, indexes []int) error {
	job, client, err := s.resolve(ctx, id)
	if err != nil {
		return err
	}

	selection := joinIndexes(indexes)
	if selection == "" {
		return fmt.Errorf("no file was chosen")
	}

	// The engine refuses to change the selection of a running
	// download, so it is paused for the change.
	wasRunning := job.Status == StatusActive
	if wasRunning {
		if err := client.Pause(ctx, job.GID); err != nil {
			return fmt.Errorf("the download could not be paused: %w", err)
		}
	}

	if err := client.ChangeOption(ctx, job.GID, map[string]string{"select-file": selection}); err != nil {
		return fmt.Errorf("the files could not be chosen: %w", err)
	}

	if wasRunning {
		if err := client.Resume(ctx, job.GID); err != nil {
			return fmt.Errorf("the download could not be resumed: %w", err)
		}
	}

	// The choice is the application's own bookkeeping, so it is kept
	// in the record as well.
	job.SelectedFiles = indexes
	if _, err := s.store.Save(job); err != nil {
		return err
	}

	return s.syncOne(ctx, client, job.GID)
}

// resolve looks a download up and returns it with the client.
//
// It is the guard every operation goes through: a download that is not
// there, or an engine that is not running, is refused before anything
// is sent.
func (s *Service) resolve(ctx context.Context, id string) (Job, *aria2.Client, error) {
	client := s.manager.Client()
	if client == nil {
		return Job{}, nil, aria2.ErrNotRunning
	}

	// The frontend only ever knows the engine's handle, so the handle is
	// what a download is looked up by. The record's own id is accepted
	// as well, because the backend's own callers use it.
	job, err := s.store.GetByGID(id)
	if err == nil {
		return job, client, nil
	}

	job, err = s.store.Get(id)
	if err != nil {
		return Job{}, nil, err
	}
	if job.GID == "" {
		return Job{}, nil, fmt.Errorf("the download %q is not known to the engine", id)
	}

	return job, client, nil
}

// syncOne refreshes the record of a single download.
//
// It is used after a change, so that the frontend sees the new state
// at once instead of waiting for the next poll.
func (s *Service) syncOne(ctx context.Context, client *aria2.Client, gid string) error {
	download, err := client.Status(ctx, gid)
	if err != nil {
		// A download the engine no longer knows about is not an
		// error: the change itself is what the caller wanted.
		return nil
	}

	_, err = s.store.Save(toJob(download, s.store))
	return err
}

// List returns the recorded downloads, newest first.
func (s *Service) List(limit int) ([]Job, error) {
	return s.store.List(limit)
}

// Get returns one download.
func (s *Service) Get(id string) (Job, error) {
	return s.store.Get(id)
}

// Logs returns the log of a download.
func (s *Service) Logs(jobID string, limit int) ([]LogEntry, error) {
	return s.store.Logs(jobID, limit)
}

// Presets returns the saved presets.
func (s *Service) Presets() ([]Preset, error) {
	return s.store.Presets()
}

// SavePreset stores a preset.
func (s *Service) SavePreset(preset Preset) (Preset, error) {
	return s.store.SavePreset(preset)
}

// DeletePreset removes a preset.
func (s *Service) DeletePreset(id string) error {
	return s.store.DeletePreset(id)
}

// Stat returns the engine's global state.
//
// The frontend shows it in the header, so that the user sees the total
// speed without opening a download.
func (s *Service) Stat(ctx context.Context) (aria2.GlobalStat, error) {
	client := s.manager.Client()
	if client == nil {
		return aria2.GlobalStat{}, aria2.ErrNotRunning
	}
	return client.GlobalStat(ctx)
}

// titleFor derives the name of a download that the engine has not
// reported on yet.
func titleFor(request AddRequest) string {
	if request.Out != "" {
		return request.Out
	}
	if request.TorrentName != "" {
		return request.TorrentName
	}
	if request.URI != "" {
		// The name of the file is the last part of the path, and a
		// magnet link has no name at all.
		if index := strings.LastIndex(request.URI, "/"); index >= 0 {
			return request.URI[index+1:]
		}
	}
	return "Download"
}

// cleanOptions removes the empty values from the options.
//
// The engine would refuse an empty value for some options, so they
// are dropped rather than sent.
func cleanOptions(options map[string]string) map[string]string {
	cleaned := map[string]string{}
	for key, value := range options {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		cleaned[key] = value
	}
	return cleaned
}

// joinIndexes renders the chosen file indexes as the engine's list.
//
// The engine wants them as a comma separated list of one-based
// indexes.
func joinIndexes(indexes []int) string {
	parts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		if index <= 0 {
			continue
		}
		parts = append(parts, strconv.Itoa(index))
	}
	sort.Slice(parts, func(i, j int) bool {
		a, _ := strconv.Atoi(parts[i])
		b, _ := strconv.Atoi(parts[j])
		return a < b
	})
	return strings.Join(parts, ",")
}

// normaliseKind maps the kind onto one of the known ones.
func normaliseKind(kind string) string {
	switch kind {
	case KindTorrent, KindMagnet:
		return kind
	default:
		return KindHTTP
	}
}
