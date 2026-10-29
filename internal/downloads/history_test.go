package downloads

import (
	"testing"

	bitmaherpb "github.com/incurl/bitmaher/internal/pocketbase"
)

// newTestHistoryStore creates a history store backed by a throwaway
// database.
func newTestHistoryStore(t *testing.T) *HistoryStore {
	t.Helper()

	service := bitmaherpb.NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	return NewHistoryStore(service.App())
}

// TestTheHistorySurvivesAWriteAndARead verifies that an entry of the
// history comes back as it was written.
func TestTheHistorySurvivesAWriteAndARead(t *testing.T) {
	store := newTestHistoryStore(t)

	written, err := store.AddRecord(HistoryRecord{
		GID:         "abc123",
		Name:        "payload.bin",
		URI:         "https://example.com/payload.bin",
		Dir:         "/downloads",
		TotalLength: 524288,
		Status:      HistoryStatusComplete,
		TaskType:    "uri",
		AddedAt:     "2026-10-29T10:00:00Z",
		CompletedAt: "2026-10-29T10:05:00Z",
		Meta:        `{"infoHash":"deadbeef"}`,
	})
	if err != nil {
		t.Fatalf("add the record: %v", err)
	}

	read, found, err := store.RecordByGID("abc123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if !found {
		t.Fatal("the record was not found")
	}

	if read.Name != written.Name {
		t.Errorf("the name is %q, want %q", read.Name, written.Name)
	}
	if read.TotalLength != 524288 {
		t.Errorf("the size is %d, want 524288", read.TotalLength)
	}
	if read.Status != HistoryStatusComplete {
		t.Errorf("the status is %q, want %q", read.Status, HistoryStatusComplete)
	}
	if read.AddedAt != "2026-10-29T10:00:00Z" {
		t.Errorf("the moment it was added is %q, want the one that was written", read.AddedAt)
	}
	if read.CompletedAt != "2026-10-29T10:05:00Z" {
		t.Errorf("the moment it finished is %q, want the one that was written", read.CompletedAt)
	}
}

// TestTheHistoryIsWrittenOnce verifies that writing the same download
// again updates the entry instead of adding a second one.
func TestTheHistoryIsWrittenOnce(t *testing.T) {
	store := newTestHistoryStore(t)

	if _, err := store.AddRecord(HistoryRecord{GID: "abc", Name: "first"}); err != nil {
		t.Fatalf("add the record: %v", err)
	}
	if _, err := store.AddRecord(HistoryRecord{GID: "abc", Name: "second"}); err != nil {
		t.Fatalf("update the record: %v", err)
	}

	records, err := store.Records("")
	if err != nil {
		t.Fatalf("read the history: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("the history holds %d entries, want 1", len(records))
	}
	if records[0].Name != "second" {
		t.Errorf("the name is %q, want second", records[0].Name)
	}
}

// TestTheHistoryIsFilteredByStatus verifies that the status narrows the
// history down.
func TestTheHistoryIsFilteredByStatus(t *testing.T) {
	store := newTestHistoryStore(t)

	if _, err := store.AddRecord(HistoryRecord{GID: "a", Status: HistoryStatusComplete}); err != nil {
		t.Fatalf("add the record: %v", err)
	}
	if _, err := store.AddRecord(HistoryRecord{GID: "b", Status: HistoryStatusError}); err != nil {
		t.Fatalf("add the record: %v", err)
	}

	records, err := store.Records(HistoryStatusError)
	if err != nil {
		t.Fatalf("read the history: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("the history holds %d entries, want 1", len(records))
	}
	if records[0].GID != "b" {
		t.Errorf("the entry is %q, want b", records[0].GID)
	}
}

// TestTheHistoryIsPaged verifies that the page holds the entries it
// should, and that the total is the size of the whole history.
func TestTheHistoryIsPaged(t *testing.T) {
	store := newTestHistoryStore(t)

	for _, gid := range []string{"a", "b", "c"} {
		if _, err := store.AddRecord(HistoryRecord{GID: gid, AddedAt: "2026-10-29T10:00:00Z"}); err != nil {
			t.Fatalf("add the record: %v", err)
		}
	}

	page, err := store.Page(1, 2, "", "")
	if err != nil {
		t.Fatalf("read the page: %v", err)
	}
	if len(page.Records) != 2 {
		t.Errorf("the page holds %d entries, want 2", len(page.Records))
	}
	if page.Total != 3 {
		t.Errorf("the total is %d, want 3", page.Total)
	}

	second, err := store.Page(2, 2, "", "")
	if err != nil {
		t.Fatalf("read the page: %v", err)
	}
	if len(second.Records) != 1 {
		t.Errorf("the second page holds %d entries, want 1", len(second.Records))
	}
}

// TestTheMomentADownloadWasAddedIsKept verifies that the moment a
// download was added never changes.
//
// It is what holds the order of the list stable across a restart.
func TestTheMomentADownloadWasAddedIsKept(t *testing.T) {
	store := newTestHistoryStore(t)

	if err := store.RecordBirth("abc", "2026-10-29T10:00:00Z"); err != nil {
		t.Fatalf("record the birth: %v", err)
	}
	// The same download, recorded again with another moment.
	if err := store.RecordBirth("abc", "2026-10-30T10:00:00Z"); err != nil {
		t.Fatalf("record the birth again: %v", err)
	}

	births, err := store.Births()
	if err != nil {
		t.Fatalf("read the births: %v", err)
	}
	if len(births) != 1 {
		t.Fatalf("the births hold %d entries, want 1", len(births))
	}
	if births[0][1] != "2026-10-29T10:00:00Z" {
		t.Errorf("the moment is %q, want the first one", births[0][1])
	}
}

// TestTheStaleEntriesAreRemoved verifies that the entries of downloads
// that are gone are removed.
func TestTheStaleEntriesAreRemoved(t *testing.T) {
	store := newTestHistoryStore(t)

	for _, gid := range []string{"a", "b"} {
		if _, err := store.AddRecord(HistoryRecord{GID: gid}); err != nil {
			t.Fatalf("add the record: %v", err)
		}
	}

	if err := store.RemoveStaleRecords([]string{"a"}); err != nil {
		t.Fatalf("remove the stale records: %v", err)
	}

	records, err := store.Records("")
	if err != nil {
		t.Fatalf("read the history: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("the history holds %d entries, want 1", len(records))
	}
	if records[0].GID != "a" {
		t.Errorf("the entry is %q, want a", records[0].GID)
	}
}
