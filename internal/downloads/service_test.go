package downloads

import (
	"testing"

	"github.com/incurl/bitmaher/internal/aria2"
)

// TestJoinIndexesRendersTheEngineList verifies the rendering of the
// chosen file indexes.
//
// The engine wants them as a comma separated list of one-based
// indexes, in order.
func TestJoinIndexesRendersTheEngineList(t *testing.T) {
	cases := []struct {
		indexes []int
		want    string
	}{
		{nil, ""},
		{[]int{1}, "1"},
		{[]int{3, 1, 2}, "1,2,3"},
		{[]int{0, 2}, "2"},
		{[]int{2, 2}, "2,2"},
	}

	for _, c := range cases {
		if got := joinIndexes(c.indexes); got != c.want {
			t.Errorf("joinIndexes(%v) = %q, want %q", c.indexes, got, c.want)
		}
	}
}

// TestNormaliseKindFallsBackToHTTP verifies that an unknown kind
// becomes the one that needs no extra information.
func TestNormaliseKindFallsBackToHTTP(t *testing.T) {
	if normaliseKind(KindTorrent) != KindTorrent {
		t.Error("torrent should be kept")
	}
	if normaliseKind(KindMagnet) != KindMagnet {
		t.Error("magnet should be kept")
	}
	if normaliseKind("something else") != KindHTTP {
		t.Error("an unknown kind should become http")
	}
	if normaliseKind("") != KindHTTP {
		t.Error("an empty kind should become http")
	}
}

// TestCleanOptionsDropsTheEmptyValues verifies that the empty options
// are not sent to the engine.
//
// The engine refuses some of them, and an empty value is never what
// the user meant.
func TestCleanOptionsDropsTheEmptyValues(t *testing.T) {
	cleaned := cleanOptions(map[string]string{
		"dir":         "/downloads",
		"out":         "  ",
		"max-tries":   "5",
		"user-agent":  "",
		"split":       " 8 ",
		"empty-value": "",
	})

	if cleaned["dir"] != "/downloads" {
		t.Errorf("dir = %q", cleaned["dir"])
	}
	if cleaned["max-tries"] != "5" {
		t.Errorf("max-tries = %q", cleaned["max-tries"])
	}
	if cleaned["split"] != "8" {
		t.Errorf("split = %q, want it trimmed", cleaned["split"])
	}
	if _, found := cleaned["out"]; found {
		t.Error("an empty value should be dropped")
	}
	if _, found := cleaned["user-agent"]; found {
		t.Error("an empty value should be dropped")
	}
}

// TestDecodeOptionsReadsTheStoredMap verifies that the options are
// read back out of a JSON field.
//
// The field may hold a map or a JSON string, depending on how it was
// written.
func TestDecodeOptionsReadsTheStoredMap(t *testing.T) {
	fromMap := decodeOptions(map[string]any{"dir": "/downloads", "split": "8"})
	if fromMap["dir"] != "/downloads" || fromMap["split"] != "8" {
		t.Errorf("from a map: %v", fromMap)
	}

	fromString := decodeOptions(`{"dir":"/downloads","split":"8"}`)
	if fromString["dir"] != "/downloads" || fromString["split"] != "8" {
		t.Errorf("from a string: %v", fromString)
	}

	fromBytes := decodeOptions([]byte(`{"dir":"/downloads","split":"8"}`))
	if fromBytes["dir"] != "/downloads" {
		t.Errorf("from bytes: %v", fromBytes)
	}

	// A value that cannot be read leaves the options empty rather
	// than failing.
	if len(decodeOptions("not json")) != 0 {
		t.Error("rubbish should leave the options empty")
	}
	if len(decodeOptions(nil)) != 0 {
		t.Error("nothing should leave the options empty")
	}
}

// TestDecodeIntSliceReadsTheStoredList verifies that the chosen file
// indexes are read back.
func TestDecodeIntSliceReadsTheStoredList(t *testing.T) {
	values := decodeIntSlice([]any{1, 2, 3})
	if len(values) != 3 || values[0] != 1 || values[2] != 3 {
		t.Errorf("from a list: %v", values)
	}

	fromString := decodeIntSlice("[1,2,3]")
	if len(fromString) != 3 {
		t.Errorf("from a string: %v", fromString)
	}

	if len(decodeIntSlice(nil)) != 0 {
		t.Error("nothing should leave the list empty")
	}
}

// TestJobProgressAndState verifies the helpers of the job model.
func TestJobProgressAndState(t *testing.T) {
	job := Job{TotalLength: 100, CompletedLength: 25, Status: StatusActive}
	if job.Progress() != 0.25 {
		t.Errorf("progress = %v", job.Progress())
	}
	if !job.IsActive() {
		t.Error("the job should be active")
	}
	if job.IsFinished() {
		t.Error("the job should not be finished")
	}

	finished := Job{Status: StatusComplete}
	if !finished.IsFinished() {
		t.Error("the job should be finished")
	}
	if finished.IsActive() {
		t.Error("the job should not be active")
	}

	// A download of unknown size has no progress.
	unknown := Job{}
	if unknown.Progress() != 0 {
		t.Errorf("progress = %v", unknown.Progress())
	}
}

// TestToJobKeepsTheApplicationsOwnFields verifies that the fields the
// engine does not report are kept.
//
// The torrent file and the chosen files belong to the application, so
// they must survive a sync.
func TestToJobKeepsTheApplicationsOwnFields(t *testing.T) {
	engine := aria2.Download{
		GID:         "abc123",
		Status:      aria2.StatusActive,
		Title:       "movie.mkv",
		TotalLength: 1000,
	}

	// The store is empty, so the fields are simply not there.
	job := toJob(engine, nil)
	if job.Kind != KindHTTP {
		t.Errorf("kind = %q", job.Kind)
	}
	if job.SelectedFiles == nil {
		t.Error("the chosen files should be an empty list, not nil")
	}
	if job.Options == nil {
		t.Error("the options should be an empty map, not nil")
	}
}
