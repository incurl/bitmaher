package downloads

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// The collections the history lives in.
//
// The history is what the user has already downloaded. It outlives the
// download records, and it is what the interface shows in its own
// panel.
const (
	// CollectionHistory stores the downloads that have come to rest.
	CollectionHistory = "download_history"
	// CollectionBirths stores the moment every download was added.
	CollectionBirths = "task_birth"
)

// The terminal statuses of a history record.
//
// They are the ones of the download records, so that the history and
// the list speak the same vocabulary.
const (
	// HistoryStatusComplete means the download finished.
	HistoryStatusComplete = "complete"
	// HistoryStatusError means the download failed.
	HistoryStatusError = "error"
	// HistoryStatusRemoved means the download was removed.
	HistoryStatusRemoved = "removed"
)

// HistoryRecord is one entry of the history.
//
// It is the frontend's view of a download that has come to rest. The
// moment the download was added is kept, because it is what holds the
// order of the list stable across a restart.
type HistoryRecord struct {
	// ID is the PocketBase record id.
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
	// AddedAt is the moment the download was added. It never changes.
	AddedAt string `json:"added_at"`
	// CreatedAt is the moment the record was created.
	CreatedAt string `json:"created_at"`
	// CompletedAt is the moment the download finished.
	CompletedAt string `json:"completed_at"`
	// Meta is the metadata of the download: the info hash of a
	// torrent, the files it holds, and what it shared.
	Meta string `json:"meta"`
}

// HistoryPage is a page of the history.
//
// The frontend asks for the history one page at a time, because the
// history can be long.
type HistoryPage struct {
	// Records are the entries of the page.
	Records []HistoryRecord `json:"records"`
	// Total is the number of entries in the whole history.
	Total int `json:"total"`
}

// HistoryStore keeps the history in PocketBase.
//
// It is the only part of the application that writes to the history
// collections.
type HistoryStore struct {
	app core.App
}

// NewHistoryStore creates a history store.
func NewHistoryStore(app core.App) *HistoryStore {
	return &HistoryStore{app: app}
}

// AddRecord writes a history record.
//
// The record is created when it does not exist yet, and updated when
// it is. The engine's gid is what links the two.
func (s *HistoryStore) AddRecord(record HistoryRecord) (HistoryRecord, error) {
	gid := strings.TrimSpace(record.GID)
	if gid == "" {
		return HistoryRecord{}, fmt.Errorf("the history record needs the gid of a download")
	}

	col, err := s.app.FindCollectionByNameOrId(CollectionHistory)
	if err != nil {
		return HistoryRecord{}, fmt.Errorf("find the history collection: %w", err)
	}

	existing, err := s.findHistoryByGID(gid)
	if err != nil {
		return HistoryRecord{}, err
	}

	if existing == nil {
		existing = core.NewRecord(col)
		existing.Set("gid", gid)
	}

	existing.Set("name", record.Name)
	existing.Set("uri", record.URI)
	existing.Set("dir", record.Dir)
	existing.Set("total_length", record.TotalLength)
	existing.Set("status", record.Status)
	existing.Set("task_type", record.TaskType)

	// The moment the download was added never changes: it is what
	// holds the order of the list stable.
	if record.AddedAt != "" {
		existing.Set("added_at", record.AddedAt)
	}
	if record.CompletedAt != "" {
		existing.Set("completed_at", record.CompletedAt)
	}
	if record.Meta != "" {
		existing.Set("meta", record.Meta)
	}

	if err := s.app.Save(existing); err != nil {
		return HistoryRecord{}, fmt.Errorf("save the history record: %w", err)
	}

	return toHistoryRecord(existing), nil
}

// Records returns the history, newest first.
//
// An empty status returns every entry.
func (s *HistoryStore) Records(status string) ([]HistoryRecord, error) {
	filter := ""
	if status != "" {
		filter = "status = {:status}"
	}

	records, err := s.app.FindRecordsByFilter(CollectionHistory, filter, "-created", 0, 0, map[string]any{"status": status})
	if err != nil {
		return nil, fmt.Errorf("list the history: %w", err)
	}

	history := make([]HistoryRecord, 0, len(records))
	for _, record := range records {
		history = append(history, toHistoryRecord(record))
	}
	return history, nil
}

// Page returns one page of the history.
//
// The page is ordered by the moment the download was added, so that the
// order does not change when a download finishes.
func (s *HistoryStore) Page(page, pageSize int, sortField string, sortOrder string) (HistoryPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	sort := "-added_at"
	switch sortField {
	case "name", "total_length", "status", "added_at", "completed_at", "created_at":
		sort = sortField
	}
	// The order of the frontend is "ascend" and "descend". The
	// backend takes a sign.
	switch sortOrder {
	case "ascend":
		sort = "+" + strings.TrimPrefix(sort, "+")
	case "descend":
		sort = "-" + strings.TrimPrefix(sort, "-")
	}
	// The moment a download was added is not unique: two downloads
	// can be added in the same millisecond. The id breaks the tie, so
	// that the order of the page does not depend on the order the
	// database happens to return the records in.
	sort += ",-id"

	total, err := s.app.CountRecords(CollectionHistory)
	if err != nil {
		return HistoryPage{}, fmt.Errorf("count the history: %w", err)
	}

	offset := (page - 1) * pageSize
	records, err := s.app.FindRecordsByFilter(CollectionHistory, "", sort, pageSize, offset, nil)
	if err != nil {
		return HistoryPage{}, fmt.Errorf("read the history: %w", err)
	}

	history := make([]HistoryRecord, 0, len(records))
	for _, record := range records {
		history = append(history, toHistoryRecord(record))
	}

	return HistoryPage{Records: history, Total: int(total)}, nil
}

// RecordByGID returns one entry of the history.
func (s *HistoryStore) RecordByGID(gid string) (HistoryRecord, bool, error) {
	record, err := s.findHistoryByGID(gid)
	if err != nil {
		return HistoryRecord{}, false, err
	}
	if record == nil {
		return HistoryRecord{}, false, nil
	}
	return toHistoryRecord(record), true, nil
}

// RemoveRecord removes one entry of the history.
func (s *HistoryStore) RemoveRecord(gid string) error {
	record, err := s.findHistoryByGID(gid)
	if err != nil {
		return err
	}
	if record == nil {
		return nil
	}
	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("remove the history record: %w", err)
	}
	return nil
}

// ClearRecords removes the entries of the history.
//
// An empty status removes every entry.
func (s *HistoryStore) ClearRecords(status string) error {
	filter := ""
	if status != "" {
		filter = "status = {:status}"
	}

	records, err := s.app.FindRecordsByFilter(CollectionHistory, filter, "", 0, 0, map[string]any{"status": status})
	if err != nil {
		return fmt.Errorf("list the history: %w", err)
	}

	for _, record := range records {
		if err := s.app.Delete(record); err != nil {
			return fmt.Errorf("remove the history record: %w", err)
		}
	}
	return nil
}

// RemoveStaleRecords removes the entries of downloads that are gone.
//
// The gids are the ones the engine still knows about. An entry whose
// download is not among them is stale.
func (s *HistoryStore) RemoveStaleRecords(gids []string) error {
	live := make(map[string]bool, len(gids))
	for _, gid := range gids {
		live[gid] = true
	}

	records, err := s.app.FindRecordsByFilter(CollectionHistory, "", "", 0, 0, nil)
	if err != nil {
		return fmt.Errorf("list the history: %w", err)
	}

	for _, record := range records {
		if live[record.GetString("gid")] {
			continue
		}
		if err := s.app.Delete(record); err != nil {
			return fmt.Errorf("remove the stale history record: %w", err)
		}
	}
	return nil
}

// RecordBirth records the moment a download was added.
//
// The moment never changes: it is what holds the order of the list
// stable across a restart.
func (s *HistoryStore) RecordBirth(gid, addedAt string) error {
	gid = strings.TrimSpace(gid)
	if gid == "" {
		return fmt.Errorf("the birth of a download needs its gid")
	}
	if addedAt == "" {
		addedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}

	col, err := s.app.FindCollectionByNameOrId(CollectionBirths)
	if err != nil {
		return fmt.Errorf("find the births collection: %w", err)
	}

	record, err := s.findBirthByGID(gid)
	if err != nil {
		return err
	}
	if record == nil {
		record = core.NewRecord(col)
		record.Set("gid", gid)
		record.Set("added_at", addedAt)
	}
	// The moment of a download that is already known is never
	// changed: it is what holds the order of the list stable.

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("save the birth: %w", err)
	}
	return nil
}

// Birth returns the moment a download was added.
//
// The second return value reports whether the moment was known. A
// download that was never seen has no moment.
func (s *HistoryStore) Birth(gid string) (string, bool, error) {
	record, err := s.findBirthByGID(gid)
	if err != nil {
		return "", false, err
	}
	if record == nil {
		return "", false, nil
	}
	return record.GetString("added_at"), true, nil
}

// Births returns the moment every download was added.
func (s *HistoryStore) Births() ([][2]string, error) {
	records, err := s.app.FindRecordsByFilter(CollectionBirths, "", "added_at", 0, 0, nil)
	if err != nil {
		return nil, fmt.Errorf("list the births: %w", err)
	}

	births := make([][2]string, 0, len(records))
	for _, record := range records {
		births = append(births, [2]string{record.GetString("gid"), record.GetString("added_at")})
	}
	return births, nil
}

// RemoveBirths removes the moments of the given downloads.
func (s *HistoryStore) RemoveBirths(gids []string) error {
	for _, gid := range gids {
		record, err := s.findBirthByGID(gid)
		if err != nil {
			return err
		}
		if record == nil {
			continue
		}
		if err := s.app.Delete(record); err != nil {
			return fmt.Errorf("remove the birth: %w", err)
		}
	}
	return nil
}

// findHistoryByGID looks a history entry up by the engine's handle.
func (s *HistoryStore) findHistoryByGID(gid string) (*core.Record, error) {
	record, err := s.app.FindFirstRecordByFilter(
		CollectionHistory,
		"gid = {:gid}",
		map[string]any{"gid": gid},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// The download is not in the history.
			return nil, nil
		}
		return nil, fmt.Errorf("look the history entry %q up: %w", gid, err)
	}
	return record, nil
}

// findBirthByGID looks a birth up by the engine's handle.
func (s *HistoryStore) findBirthByGID(gid string) (*core.Record, error) {
	record, err := s.app.FindFirstRecordByFilter(
		CollectionBirths,
		"gid = {:gid}",
		map[string]any{"gid": gid},
	)
	if err != nil {
		return nil, nil
	}
	return record, nil
}

// toHistoryRecord converts a PocketBase record into a history entry.
func toHistoryRecord(record *core.Record) HistoryRecord {
	return HistoryRecord{
		ID:          record.Id,
		GID:         record.GetString("gid"),
		Name:        record.GetString("name"),
		URI:         record.GetString("uri"),
		Dir:         record.GetString("dir"),
		TotalLength: int64(record.GetInt("total_length")),
		Status:      record.GetString("status"),
		TaskType:    record.GetString("task_type"),
		AddedAt:     record.GetString("added_at"),
		CreatedAt:   record.GetString("created"),
		CompletedAt: record.GetString("completed_at"),
		Meta:        decodeMeta(record.Get("meta")),
	}
}

// decodeMeta reads the metadata out of a JSON field.
//
// The field may hold a map or a JSON string, depending on how it was
// written, so both are accepted. A value that cannot be read leaves the
// metadata empty rather than failing the whole record.
func decodeMeta(raw any) string {
	if raw == nil {
		return ""
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
			return ""
		}
		encoded = raw
	}

	if len(encoded) == 0 {
		return ""
	}

	// A JSON field that was never written holds the literal null. That
	// is not metadata, and passing it on hands the frontend a string
	// that parses to null, which is not an object: the record then
	// fails to render, and every task of the list goes with it.
	if trimmed := strings.TrimSpace(string(encoded)); trimmed == "null" {
		return ""
	}

	return string(encoded)
}
