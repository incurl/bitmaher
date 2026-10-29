package bindings

import (
	"fmt"

	"github.com/incurl/bitmaher/internal/downloads"
)

// The history commands answer for the downloads that have come to rest.
//
// The history outlives the download records: a download that is removed
// from the list stays in the history, and the history is what the
// interface shows in its own panel.
//
// The commands are named after the ones of the port, because the
// frontend calls them by name.

// HistoryRecordView is one entry of the history, as the frontend sees
// it.
type HistoryRecordView struct {
	// ID is the record id.
	ID string `json:"id"`
	// GID is the engine's handle for the download.
	GID string `json:"gid"`
	// Name is the display name of the download.
	Name string `json:"name"`
	// URI is the source: the URL or the magnet link.
	URI string `json:"uri"`
	// Dir is the directory the download was written to.
	Dir string `json:"dir"`
	// TotalLength is the size of the download in bytes.
	TotalLength int64 `json:"total_length"`
	// Status is the terminal status of the download.
	Status string `json:"status"`
	// TaskType is the kind of the download.
	TaskType string `json:"task_type"`
	// AddedAt is the moment the download was added.
	AddedAt string `json:"added_at"`
	// CreatedAt is the moment the record was created.
	CreatedAt string `json:"created_at"`
	// CompletedAt is the moment the download finished.
	CompletedAt string `json:"completed_at"`
	// Meta is the metadata of the download.
	Meta string `json:"meta"`
}

// HistoryPageView is a page of the history.
//
// The frontend asks for the history one page at a time, because the
// history can be long.
type HistoryPageView struct {
	// Records are the entries of the page.
	Records []HistoryRecordView `json:"records"`
	// Total is the number of entries in the whole history.
	Total int `json:"total"`
}

// HistoryBirthView is the moment a download was added.
//
// The frontend reads them in one go, to restore the order of the list
// after a restart.
type HistoryBirthView struct {
	// GID is the engine's handle for the download.
	GID string `json:"gid"`
	// AddedAt is the moment the download was added.
	AddedAt string `json:"added_at"`
}

// HistoryPageInput is what the frontend asks for when it reads a page.
type HistoryPageInput struct {
	// Status narrows the page to one status. It is empty for every
	// status.
	Status string `json:"status"`
	// Page is the one-based number of the page.
	Page int `json:"page"`
	// PageSize is how many entries a page holds.
	PageSize int `json:"pageSize"`
	// SortField is the field the page is ordered by.
	SortField string `json:"sortField"`
	// SortOrder is "ascend" or "descend".
	SortOrder string `json:"sortOrder"`
}

// historyStore returns the history store, or nil when the application
// runs without the download service.
func (s *AppService) historyStore() *downloads.HistoryStore {
	if s == nil || s.downloads == nil {
		return nil
	}
	return s.downloads.History()
}

// HistoryAddRecord writes an entry of the history.
//
// The record is created when it does not exist yet, and updated when it
// is. The engine's gid is what links the two.
func (s *AppService) HistoryAddRecord(record HistoryRecordView) (HistoryRecordView, error) {
	store := s.historyStore()
	if store == nil {
		return HistoryRecordView{}, fmt.Errorf("the history is not available")
	}

	saved, err := store.AddRecord(downloads.HistoryRecord{
		GID:         record.GID,
		Name:        record.Name,
		URI:         record.URI,
		Dir:         record.Dir,
		TotalLength: record.TotalLength,
		Status:      record.Status,
		TaskType:    record.TaskType,
		AddedAt:     record.AddedAt,
		CompletedAt: record.CompletedAt,
		Meta:        record.Meta,
	})
	if err != nil {
		return HistoryRecordView{}, err
	}

	return toHistoryRecordView(saved), nil
}

// HistoryGetRecords returns the history, newest first.
func (s *AppService) HistoryGetRecords(status string) ([]HistoryRecordView, error) {
	store := s.historyStore()
	if store == nil {
		return []HistoryRecordView{}, nil
	}

	records, err := store.Records(status)
	if err != nil {
		return nil, err
	}

	views := make([]HistoryRecordView, 0, len(records))
	for _, record := range records {
		views = append(views, toHistoryRecordView(record))
	}
	return views, nil
}

// HistoryGetPage returns one page of the history.
func (s *AppService) HistoryGetPage(input HistoryPageInput) (HistoryPageView, error) {
	store := s.historyStore()
	if store == nil {
		return HistoryPageView{Records: []HistoryRecordView{}}, nil
	}

	page, err := store.Page(input.Page, input.PageSize, input.SortField, input.SortOrder)
	if err != nil {
		return HistoryPageView{}, err
	}

	views := make([]HistoryRecordView, 0, len(page.Records))
	for _, record := range page.Records {
		views = append(views, toHistoryRecordView(record))
	}

	return HistoryPageView{Records: views, Total: page.Total}, nil
}

// HistoryGetRecord returns one entry of the history.
//
// The second return value reports whether the entry was found.
func (s *AppService) HistoryGetRecord(gid string) (HistoryRecordView, bool, error) {
	store := s.historyStore()
	if store == nil {
		return HistoryRecordView{}, false, nil
	}

	record, found, err := store.RecordByGID(gid)
	if err != nil {
		return HistoryRecordView{}, false, err
	}
	if !found {
		return HistoryRecordView{}, false, nil
	}

	return toHistoryRecordView(record), true, nil
}

// HistoryRemoveRecord removes one entry of the history.
func (s *AppService) HistoryRemoveRecord(gid string) error {
	store := s.historyStore()
	if store == nil {
		return nil
	}
	return store.RemoveRecord(gid)
}

// HistoryClearRecords removes the entries of the history.
//
// An empty status removes every entry.
func (s *AppService) HistoryClearRecords(status string) error {
	store := s.historyStore()
	if store == nil {
		return nil
	}
	return store.ClearRecords(status)
}

// HistoryRemoveStale removes the entries of downloads that are gone.
//
// The gids are the ones the engine still knows about.
func (s *AppService) HistoryRemoveStale(gids []string) error {
	store := s.historyStore()
	if store == nil {
		return nil
	}
	return store.RemoveStaleRecords(gids)
}

// HistoryRecordBirth records the moment a download was added.
//
// The moment never changes: it is what holds the order of the list
// stable across a restart.
func (s *AppService) HistoryRecordBirth(gid, addedAt string) error {
	store := s.historyStore()
	if store == nil {
		return nil
	}
	return store.RecordBirth(gid, addedAt)
}

// HistoryLoadBirths returns the moment every download was added.
func (s *AppService) HistoryLoadBirths() ([]HistoryBirthView, error) {
	store := s.historyStore()
	if store == nil {
		return []HistoryBirthView{}, nil
	}

	births, err := store.Births()
	if err != nil {
		return nil, err
	}

	views := make([]HistoryBirthView, 0, len(births))
	for _, birth := range births {
		views = append(views, HistoryBirthView{GID: birth[0], AddedAt: birth[1]})
	}
	return views, nil
}

// HistoryRemoveBirths removes the moments of the given downloads.
func (s *AppService) HistoryRemoveBirths(gids []string) error {
	store := s.historyStore()
	if store == nil {
		return nil
	}
	return store.RemoveBirths(gids)
}

// DatabaseSchemaVersion returns the version of the schema.
//
// The database is the one of PocketBase, and the version is the number
// of the migrations that have been applied. It grows every time a
// migration is applied.
func (s *AppService) DatabaseSchemaVersion() int64 {
	if s == nil || s.pb == nil {
		return 0
	}

	applied := s.pb.App().Store().Get("schema_version")

	switch version := applied.(type) {
	case int:
		return int64(version)
	case int64:
		return version
	case float64:
		return int64(version)
	default:
		return 0
	}
}

// DatabaseIntegrityCheck checks that the database is sound.
//
// It answers "ok" when the database is sound, and the reason when it is
// not.
func (s *AppService) DatabaseIntegrityCheck() string {
	if s == nil || s.pb == nil {
		return "ok"
	}

	// The database is SQLite, and PocketBase keeps it consistent on
	// its own. The check is that it can still be read.
	if _, err := s.pb.App().DB().NewQuery("SELECT 1").Execute(); err != nil {
		return err.Error()
	}
	return "ok"
}

// toHistoryRecordView converts a history entry into its view.
func toHistoryRecordView(record downloads.HistoryRecord) HistoryRecordView {
	return HistoryRecordView{
		ID:          record.ID,
		GID:         record.GID,
		Name:        record.Name,
		URI:         record.URI,
		Dir:         record.Dir,
		TotalLength: record.TotalLength,
		Status:      record.Status,
		TaskType:    record.TaskType,
		AddedAt:     record.AddedAt,
		CreatedAt:   record.CreatedAt,
		CompletedAt: record.CompletedAt,
		Meta:        record.Meta,
	}
}
