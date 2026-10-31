package downloads

import (
	"context"
	"errors"
	"testing"

	"github.com/incurl/bitmaher/internal/aria2"
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

// newTestService creates a downloads service backed by a throwaway
// database.
//
// The engine is not there: the service is used for the parts that do
// not need it, which is the history.
func newTestService(t *testing.T) *Service {
	t.Helper()

	pb := bitmaherpb.NewService(t.TempDir())
	if err := pb.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = pb.Close()
	})

	return NewService(aria2.NewManager(), NewStore(pb.App()), nil)
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

// TestTheHistoryEntryCanBeRemoved pins that a download that has come
// to rest can be deleted.
//
// A download that has come to rest has no record in the list any more:
// it lives in the history. The interface shows it in the list anyway,
// and deleting it has to work.
//
// The frontend only ever knows the engine's handle, so the handle is
// what the download is looked up by.
func TestTheHistoryEntryCanBeRemoved(t *testing.T) {
	store := newTestHistoryStore(t)

	_, err := store.AddRecord(HistoryRecord{
		GID:      "gone123",
		Name:     "payload.bin",
		URI:      "https://example.com/payload.bin",
		Dir:      "/downloads",
		Status:   HistoryStatusComplete,
		TaskType: "uri",
		AddedAt:  "2026-10-29T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("add the record: %v", err)
	}

	if err := store.RemoveRecord("gone123"); err != nil {
		t.Fatalf("remove the record: %v", err)
	}

	_, found, err := store.RecordByGID("gone123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if found {
		t.Error("the record is still there")
	}
}

// TestTheServiceRemovesADownloadThatHasComeToRest pins that a
// download in the history can be deleted through the service.
//
// The service removes a download from the engine and forgets it. A
// download that has come to rest is not in the engine any more, and it
// has no record in the list: it is in the history.
//
// The frontend only ever knows the engine's handle, and it passes that
// handle as the download's id. The service has to understand it.
func TestTheServiceRemovesADownloadThatHasComeToRest(t *testing.T) {
	service := newTestService(t)

	_, err := service.history.AddRecord(HistoryRecord{
		GID:      "rest123",
		Name:     "payload.bin",
		URI:      "https://example.com/payload.bin",
		Dir:      "/downloads",
		Status:   HistoryStatusComplete,
		TaskType: "uri",
		AddedAt:  "2026-10-29T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("add the record: %v", err)
	}

	if err := service.Remove(context.Background(), "rest123"); err != nil {
		t.Fatalf("remove the download: %v", err)
	}

	_, found, err := service.history.RecordByGID("rest123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if found {
		t.Error("the record is still there")
	}
}

// TestASeedingTorrentJoinsTheHistory verifies that a torrent that is
// sharing the content it already has is recorded as a completed
// download.
//
// The engine keeps a finished torrent running so that it seeds, and the
// status stays "active" for as long as it does. The flag is what says
// that the download itself is over, and the history is where a
// finished download is recorded.
func TestASeedingTorrentJoinsTheHistory(t *testing.T) {
	service := newTestService(t)

	service.recordHistory(Job{
		GID:             "seeding123",
		Kind:            "torrent",
		Status:          StatusActive,
		Seeder:          true,
		TorrentState:    "seeding",
		Title:           "Big Buck Bunny",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 524288,
	})

	record, found, err := service.history.RecordByGID("seeding123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if !found {
		t.Fatal("the seeding torrent is not in the history")
	}
	if record.Status != HistoryStatusComplete {
		t.Errorf("status = %q, want %q", record.Status, HistoryStatusComplete)
	}
	if record.Name != "Big Buck Bunny" {
		t.Errorf("name = %q", record.Name)
	}
}

// TestADownloadThatIsStillGoingJoinsNoHistory verifies that a download
// that is still fetching is not recorded as finished.
func TestADownloadThatIsStillGoingJoinsNoHistory(t *testing.T) {
	service := newTestService(t)

	service.recordHistory(Job{
		GID:    "busy123",
		Kind:   "http",
		Status: StatusActive,
		Title:  "payload.bin",
		Dir:    "/downloads",
	})

	_, found, err := service.history.RecordByGID("busy123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if found {
		t.Error("a download that is still going should not be in the history")
	}
}

// TestStoppingTheSharingDeactivatesTheDownload verifies that the record
// says that the download is over once the sharing has been stopped.
//
// The engine has no call for it, so the download is removed from the
// engine. The record went on saying "active" and "seeding", so the task
// went on sharing: the status is what deactivates it.
func TestStoppingTheSharingDeactivatesTheDownload(t *testing.T) {
	service := newTestService(t)

	// The record of a download that is sharing the content it has.
	job := Job{
		GID:             "sharing123",
		Kind:            "torrent",
		Status:          StatusActive,
		Seeder:          true,
		TorrentState:    "seeding",
		Title:           "Big Buck Bunny",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 524288,
	}
	if _, err := service.store.Save(job); err != nil {
		t.Fatalf("save the record: %v", err)
	}

	// The sharing is stopped. The engine is not running, so the removal
	// fails, and the record must not have changed: a download that is
	// still sharing is still sharing.
	if err := service.StopSharing(context.Background(), job.GID); err == nil {
		t.Fatal("the removal should have failed without an engine")
	}

	stored, err := service.store.GetByGID(job.GID)
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if stored.Status != StatusActive {
		t.Errorf("status = %q, want %q", stored.Status, StatusActive)
	}
	if !stored.Seeder {
		t.Error("the record should still say that the download is seeding")
	}
}

// TestADownloadWhoseSharingWasStoppedJoinsTheHistory verifies that a
// download whose sharing was stopped is recorded as the removed one it
// is.
//
// The record says that the download is over, and the history is where
// the user still finds it: the download is not lost, it has come to
// rest.
func TestADownloadWhoseSharingWasStoppedJoinsTheHistory(t *testing.T) {
	service := newTestService(t)

	service.recordHistory(Job{
		GID:             "stopped123",
		Kind:            "torrent",
		Status:          StatusRemoved,
		Seeder:          false,
		Title:           "Big Buck Bunny",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 524288,
	})

	record, found, err := service.history.RecordByGID("stopped123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if !found {
		t.Fatal("the download is not in the history")
	}
	if record.Status != HistoryStatusRemoved {
		t.Errorf("status = %q, want %q", record.Status, HistoryStatusRemoved)
	}
}

// TestPausingAPausedDownloadIsNotAFailure verifies that a download
// that is paused already can be paused again.
//
// The engine refuses to pause a paused download with "cannot be
// paused now", and the refusal was reported as the failure of the
// whole operation. The user asked for what the download is already in.
func TestPausingAPausedDownloadIsNotAFailure(t *testing.T) {
	service := newTestService(t)

	// The record of a download the user has paused.
	job := Job{
		GID:             "paused123",
		Kind:            "torrent",
		Status:          StatusPaused,
		Seeder:          true,
		TorrentState:    "paused",
		Title:           "Big Buck Bunny",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 524288,
	}
	if _, err := service.store.Save(job); err != nil {
		t.Fatalf("save the record: %v", err)
	}

	// The engine is not running, so the pause cannot be asked for at
	// all. The point of the check is that the refusal of the engine
	// ("cannot be paused now") is not what comes back: the download is
	// already paused, and there is nothing to ask for.
	err := service.Pause(context.Background(), job.GID)
	if err != nil && !errors.Is(err, aria2.ErrNotRunning) {
		t.Fatalf("pausing a paused download should not fail: %v", err)
	}

	// And the record is untouched.
	stored, err := service.store.GetByGID(job.GID)
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if stored.Status != StatusPaused {
		t.Errorf("status = %q, want %q", stored.Status, StatusPaused)
	}
}

// TestResumingARunningDownloadIsNotAFailure verifies the same for a
// download that is running already.
func TestResumingARunningDownloadIsNotAFailure(t *testing.T) {
	service := newTestService(t)

	job := Job{
		GID:         "running123",
		Kind:        "http",
		Status:      StatusActive,
		Title:       "payload.bin",
		Dir:         "/downloads",
		TotalLength: 1024,
	}
	if _, err := service.store.Save(job); err != nil {
		t.Fatalf("save the record: %v", err)
	}

	err := service.Resume(context.Background(), job.GID)
	if err != nil && !errors.Is(err, aria2.ErrNotRunning) {
		t.Fatalf("resuming a running download should not fail: %v", err)
	}
}

// TestADownloadThatWasPausedAfterItWasFinishedJoinsTheHistory
// verifies that a download that has all of its content joins the
// history, even when the engine says it is paused.
//
// The engine keeps the status "paused" for a download the user paused,
// and it says nothing about the content being on the disk. The
// download is over, and the history is where a finished download is
// recorded.
func TestADownloadThatWasPausedAfterItWasFinishedJoinsTheHistory(t *testing.T) {
	service := newTestService(t)

	service.recordHistory(Job{
		GID:             "paused123",
		Kind:            "torrent",
		Status:          StatusPaused,
		Title:           "Big Buck Bunny",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 524288,
	})

	record, found, err := service.history.RecordByGID("paused123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if !found {
		t.Fatal("the download is not in the history")
	}
	if record.Status != HistoryStatusComplete {
		t.Errorf("status = %q, want %q", record.Status, HistoryStatusComplete)
	}
}

// TestADownloadThatWasPausedHalfWayJoinsNoHistory verifies that a
// download that was paused before it was finished is not recorded as
// finished.
func TestADownloadThatWasPausedHalfWayJoinsNoHistory(t *testing.T) {
	service := newTestService(t)

	service.recordHistory(Job{
		GID:             "halfway123",
		Kind:            "http",
		Status:          StatusPaused,
		Title:           "payload.bin",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 1024,
	})

	_, found, err := service.history.RecordByGID("halfway123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if found {
		t.Error("a download that is not finished should not be in the history")
	}
}

// TestARecordWithNoMetadataKeepsItsMetadataEmpty verifies that a
// record whose metadata was never written does not carry the JSON null
// back to the interface.
//
// The column is a JSON one, and a value that was never written reads
// back as the literal null. Passing that on hands the frontend a
// string that parses to null, which is not an object: the record then
// fails to render, and every task of the list goes with it.
func TestARecordWithNoMetadataKeepsItsMetadataEmpty(t *testing.T) {
	service := newTestService(t)

	service.recordHistory(Job{
		GID:             "nometa123",
		Kind:            "magnet",
		Status:          StatusComplete,
		Title:           "Small Prophets",
		Dir:             "/downloads",
		TotalLength:     524288,
		CompletedLength: 524288,
	})

	record, found, err := service.history.RecordByGID("nometa123")
	if err != nil {
		t.Fatalf("read the record: %v", err)
	}
	if !found {
		t.Fatal("the download is not in the history")
	}
	if record.Meta != "" {
		t.Errorf("meta = %q, want it empty", record.Meta)
	}
}

// TestDecodeMetaReadsTheMetadataItUnderstands verifies that the
// decoder still reads the metadata it is given.
func TestDecodeMetaReadsTheMetadataItUnderstands(t *testing.T) {
	for _, written := range []string{
		`{"infoHash":"abc"}`,
		` {"completedLength":"42"} `,
	} {
		if decoded := decodeMeta([]byte(written)); decoded == "" {
			t.Errorf("the metadata %q should have been read", written)
		}
	}

	for _, empty := range []any{nil, "", []byte(""), []byte("null"), []byte(" null ")} {
		if decoded := decodeMeta(empty); decoded != "" {
			t.Errorf("the value %v should decode to nothing, got %q", empty, decoded)
		}
	}
}
