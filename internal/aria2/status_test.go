package aria2

import (
	"testing"
)

// TestNormalizeStatusMapsTheEngineStates verifies that every state the
// engine reports is mapped onto a normalised one.
//
// The engine's states were taken from aria2-next 2.8.2 and 3.0.9.
func TestNormalizeStatusMapsTheEngineStates(t *testing.T) {
	cases := map[string]Status{
		"active":   StatusActive,
		"waiting":  StatusWaiting,
		"paused":   StatusPaused,
		"complete": StatusComplete,
		"error":    StatusError,
		"removed":  StatusRemoved,
	}

	for raw, want := range cases {
		if got := NormalizeStatus(raw); got != want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", raw, got, want)
		}
	}
}

// TestNormalizeStatusIsTotal verifies that an unknown state does not
// take the application down.
//
// The engine may grow a state, and an unknown one must not be fatal.
func TestNormalizeStatusIsTotal(t *testing.T) {
	for _, raw := range []string{"", "queued", "seeding", "REMOVED", "Active"} {
		got := NormalizeStatus(raw)
		if got == "" {
			t.Errorf("NormalizeStatus(%q) returned an empty status", raw)
		}
	}
}

// TestStatusTerminalReportsTheRestingStates verifies which states are
// resting ones.
func TestStatusTerminalReportsTheRestingStates(t *testing.T) {
	terminal := []Status{StatusComplete, StatusError, StatusRemoved}
	for _, status := range terminal {
		if !status.Terminal() {
			t.Errorf("%q should be terminal", status)
		}
	}

	running := []Status{StatusActive, StatusWaiting, StatusPaused}
	for _, status := range running {
		if status.Terminal() {
			t.Errorf("%q should not be terminal", status)
		}
	}
}

// TestToDownloadReadsTheEnginePayload verifies the conversion of a
// status payload.
//
// The payload is the one aria2-next returns, with its numbers as
// strings.
func TestToDownloadReadsTheEnginePayload(t *testing.T) {
	raw := rawStatus{
		GID:             "abc123",
		Status:          "active",
		TotalLength:     "1048576",
		CompletedLength: "524288",
		UploadLength:    "1024",
		DownloadSpeed:   "65536",
		UploadSpeed:     "128",
		Connections:     "8",
		ErrorCode:       "0",
		ErrorMessage:    "",
		Dir:             "/downloads",
		InfoHash:        "0123456789abcdef",
		NumSeeders:      "3",
		NumPeers:        "5",
		Files: []rawFile{
			{
				Index:           "1",
				Path:            "/downloads/movie.mkv",
				Length:          "1048576",
				CompletedLength: "524288",
				Selected:        "true",
			},
		},
	}

	download := toDownload(raw)

	if download.GID != "abc123" {
		t.Errorf("gid = %q", download.GID)
	}
	if download.Status != StatusActive {
		t.Errorf("status = %q", download.Status)
	}
	if download.TotalLength != 1048576 {
		t.Errorf("total length = %d", download.TotalLength)
	}
	if download.CompletedLength != 524288 {
		t.Errorf("completed length = %d", download.CompletedLength)
	}
	if download.DownloadSpeed != 65536 {
		t.Errorf("download speed = %d", download.DownloadSpeed)
	}
	if download.Connections != 8 {
		t.Errorf("connections = %d", download.Connections)
	}
	if download.Seeders != 3 || download.Peers != 5 {
		t.Errorf("seeders = %d, peers = %d", download.Seeders, download.Peers)
	}
	if download.Title != "movie.mkv" {
		t.Errorf("title = %q, want the file name", download.Title)
	}
	if download.Dir != "/downloads" {
		t.Errorf("dir = %q", download.Dir)
	}
	if len(download.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(download.Files))
	}
	if !download.Files[0].Selected {
		t.Error("the file should be selected")
	}
	if download.Files[0].Index != 1 {
		t.Errorf("file index = %d", download.Files[0].Index)
	}
}

// TestToDownloadReadsTheSeederFlag verifies the flag that says that a
// torrent is sharing the content it already has.
//
// The engine keeps a finished torrent running so that it seeds, and the
// status stays "active" for as long as it does. The flag is the only
// thing that says that the download itself is over, and it was dropped:
// the download stayed in the progress list for ever.
func TestToDownloadReadsTheSeederFlag(t *testing.T) {
	raw := rawStatus{
		GID:             "abc123",
		Status:          "active",
		Seeder:          "true",
		TotalLength:     "1048576",
		CompletedLength: "1048576",
		BitTorrent: &rawBittor{
			InfoHashV1: "deadbeef",
			Info:       &rawInfo{Name: "Big Buck Bunny"},
			State:      "seeding",
		},
	}

	download := toDownload(raw)

	if !download.Seeder {
		t.Error("the download should be reported as seeding")
	}
	if download.TorrentState != "seeding" {
		t.Errorf("torrent state = %q, want seeding", download.TorrentState)
	}
	// The status is the engine's truth: the download is still running,
	// and it is sharing the content it has.
	if download.Status != StatusActive {
		t.Errorf("status = %q, want active", download.Status)
	}
}

// TestToDownloadOfAFinishedTorrentIsNotSeeding verifies that a torrent
// that has stopped seeding is not reported as seeding.
//
// The engine reports "stopped" for a torrent whose seed time is over.
func TestToDownloadOfAFinishedTorrentIsNotSeeding(t *testing.T) {
	raw := rawStatus{
		GID:             "abc123",
		Status:          "complete",
		Seeder:          "false",
		CompletedLength: "1048576",
		TotalLength:     "1048576",
		BitTorrent:      &rawBittor{State: "stopped"},
	}

	download := toDownload(raw)

	if download.Seeder {
		t.Error("a stopped torrent should not be reported as seeding")
	}
	if download.Status != StatusComplete {
		t.Errorf("status = %q, want complete", download.Status)
	}
}

// TestToDownloadUsesTheTorrentName verifies that the torrent's name is
// preferred over the file name.
//
// The name of the content is what the user recognises.
func TestToDownloadUsesTheTorrentName(t *testing.T) {
	raw := rawStatus{
		GID:    "abc123",
		Status: "active",
		BitTorrent: &rawBittor{
			InfoHashV1: "deadbeef",
			Info:       &rawInfo{Name: "Big Buck Bunny"},
		},
		Files: []rawFile{
			{Index: "1", Path: "/downloads/bbb/bbb.mp4", Selected: "true"},
		},
	}

	download := toDownload(raw)

	if download.Title != "Big Buck Bunny" {
		t.Errorf("title = %q, want the torrent name", download.Title)
	}
	if download.InfoHash != "deadbeef" {
		t.Errorf("info hash = %q", download.InfoHash)
	}
}

// TestToDownloadFallsBackToTheInfoHash verifies that a magnet link
// without metadata still has a name.
func TestToDownloadFallsBackToTheInfoHash(t *testing.T) {
	raw := rawStatus{
		GID:        "abc123",
		Status:     "waiting",
		InfoHash:   "0123456789abcdef",
		BitTorrent: &rawBittor{InfoHashV1: "0123456789abcdef"},
	}

	download := toDownload(raw)

	if download.Title != "0123456789abcdef" {
		t.Errorf("title = %q, want the info hash", download.Title)
	}
}

// TestToFilesReadsThePayloadOfGetFiles verifies the conversion of the
// files of a download.
//
// The panel of a download shows them, so the sizes, the progress and
// the sources have to survive the conversion.
func TestToFilesReadsThePayloadOfGetFiles(t *testing.T) {
	raw := []rawFileEntry{
		{
			Index:           "1",
			Path:            "/downloads/movie.mkv",
			Length:          "1500000000",
			CompletedLength: "750000000",
			Selected:        "true",
			URIs: []rawURI{
				{URI: "http://example.com/movie.mkv", Status: "used"},
				{URI: "http://mirror.example.com/movie.mkv", Status: "waiting"},
			},
		},
	}

	files := toFiles(raw)

	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	file := files[0]
	if file.Index != 1 {
		t.Errorf("index = %d", file.Index)
	}
	if file.Length != 1500000000 {
		t.Errorf("length = %d", file.Length)
	}
	if file.CompletedLength != 750000000 {
		t.Errorf("completed length = %d", file.CompletedLength)
	}
	if !file.Selected {
		t.Error("the file should be selected")
	}
	if len(file.URIs) != 2 {
		t.Fatalf("uris = %d, want 2", len(file.URIs))
	}
	if file.URIs[0].Status != "used" {
		t.Errorf("the first source should be the one in use, got %q", file.URIs[0].Status)
	}
	if file.URIs[1].URI != "http://mirror.example.com/movie.mkv" {
		t.Errorf("the mirror = %q", file.URIs[1].URI)
	}
}

// TestToFilesOfATorrentHaveNoSources verifies the shape a torrent's
// file has.
//
// It is read from the swarm, so the engine reports no address for it,
// and the panel must not invent one.
func TestToFilesOfATorrentHaveNoSources(t *testing.T) {
	files := toFiles([]rawFileEntry{{Index: "1", Path: "/downloads/bbb/bbb.mp4", Selected: "true"}})

	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
	if len(files[0].URIs) != 0 {
		t.Errorf("uris = %d, want none", len(files[0].URIs))
	}
}

// TestProgressIsClamped verifies the completion ratio.
func TestProgressIsClamped(t *testing.T) {
	cases := []struct {
		completed int64
		total     int64
		want      float64
	}{
		{0, 100, 0},
		{50, 100, 0.5},
		{100, 100, 1},
		{150, 100, 1},
		{10, 0, 0},
		{-5, 100, 0},
	}

	for _, c := range cases {
		download := Download{CompletedLength: c.completed, TotalLength: c.total}
		if got := download.Progress(); got != c.want {
			t.Errorf("Progress(%d, %d) = %v, want %v", c.completed, c.total, got, c.want)
		}
	}
}

// TestParseInt64ToleratesMissingValues verifies that a number the
// engine did not send is zero rather than an error.
func TestParseInt64ToleratesMissingValues(t *testing.T) {
	if got := parseInt64(""); got != 0 {
		t.Errorf("parseInt64(\"\") = %d, want 0", got)
	}
	if got := parseInt64("not a number"); got != 0 {
		t.Errorf("parseInt64 of rubbish = %d, want 0", got)
	}
	if got := parseInt64("42"); got != 42 {
		t.Errorf("parseInt64(\"42\") = %d", got)
	}
}

// TestParseBoolReadsTheEngineBooleans verifies that the engine's
// string booleans are read.
func TestParseBoolReadsTheEngineBooleans(t *testing.T) {
	if !parseBool("true") {
		t.Error("\"true\" should be true")
	}
	if parseBool("false") {
		t.Error("\"false\" should be false")
	}
	if parseBool("") {
		t.Error("an empty value should be false")
	}
}

// TestIsAria2Next verifies the check of the engine's identity.
func TestIsAria2Next(t *testing.T) {
	if !(Version{Product: "aria2-next"}).IsAria2Next() {
		t.Error("aria2-next should be recognised")
	}
	if !(Version{Product: "ARIA2-NEXT"}).IsAria2Next() {
		t.Error("the check should ignore the case")
	}
	if (Version{Product: ""}).IsAria2Next() {
		t.Error("the upstream aria2 must not be accepted")
	}
	if (Version{Product: "aria2"}).IsAria2Next() {
		t.Error("aria2 must not be accepted")
	}
}

// TestNormalizeKindFallsBackToHTTP verifies that an unknown kind
// becomes the one that needs no extra information.
func TestNormalizeKindFallsBackToHTTP(t *testing.T) {
	if NormalizeKind("torrent") != KindTorrent {
		t.Error("torrent should be kept")
	}
	if NormalizeKind("magnet") != KindMagnet {
		t.Error("magnet should be kept")
	}
	if NormalizeKind("something else") != KindHTTP {
		t.Error("an unknown kind should become http")
	}
}
