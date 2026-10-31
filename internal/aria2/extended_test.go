package aria2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// engineStub answers one method with one result.
func engineStub(t *testing.T, method string, result any) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode the request: %v", err)
			return
		}

		if request.Method != method {
			t.Errorf("the method is %q, want %q", request.Method, method)
		}

		// The token is always the first parameter.
		if len(request.Params) == 0 || request.Params[0] != "token:secret" {
			t.Errorf("the request carries no token: %v", request.Params)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result":  result,
		}); err != nil {
			t.Errorf("encode the reply: %v", err)
		}
	}))
}

// TestProbeReadsTheAnswerOfTheEngine verifies that the probe reads the
// fields the engine answers with.
func TestProbeReadsTheAnswerOfTheEngine(t *testing.T) {
	server := engineStub(t, methodProbe, map[string]any{
		"status":      "200",
		"finalUrl":    "http://example.com/file.zip",
		"contentType": "application/zip",
		"resumable":   "true",
		"totalLength": "1024",
		"filename":    "file.zip",
	})
	defer server.Close()

	client := NewClient(server.URL, "secret")

	probe, err := client.Probe(context.Background(), "http://example.com/file.zip", nil)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	if probe.Status != "200" {
		t.Errorf("the status is %q, want 200", probe.Status)
	}
	if probe.TotalLength != "1024" {
		t.Errorf("the length is %q, want 1024", probe.TotalLength)
	}
	if probe.ContentType != "application/zip" {
		t.Errorf("the content type is %q, want application/zip", probe.ContentType)
	}
	if probe.Filename != "file.zip" {
		t.Errorf("the filename is %q, want file.zip", probe.Filename)
	}
}

// TestThePeersAreRead verifies that the peers of a torrent are read.
func TestThePeersAreRead(t *testing.T) {
	server := engineStub(t, methodGetPeers, []map[string]any{
		{"peerId": "abc", "ip": "10.0.0.1", "port": 6881, "downloadSpeed": 100},
	})
	defer server.Close()

	client := NewClient(server.URL, "secret")

	peers, err := client.Peers(context.Background(), "gid")
	if err != nil {
		t.Fatalf("peers: %v", err)
	}

	if len(peers) != 1 {
		t.Fatalf("the engine answered %d peers, want 1", len(peers))
	}
	if peers[0].IP != "10.0.0.1" {
		t.Errorf("the address is %q, want 10.0.0.1", peers[0].IP)
	}
	if peers[0].Port != 6881 {
		t.Errorf("the port is %d, want 6881", peers[0].Port)
	}
}

// TestTheTorrentIsInspected verifies that a torrent is read without
// creating a download.
func TestTheTorrentIsInspected(t *testing.T) {
	server := engineStub(t, methodInspectTorrent, map[string]any{
		"name":        "Ubuntu",
		"mode":        "single",
		"infoHashV1":  "abc",
		"totalLength": 2048,
		"files": []map[string]any{
			{"index": 1, "path": "ubuntu.iso", "length": 2048},
		},
	})
	defer server.Close()

	client := NewClient(server.URL, "secret")

	inspection, err := client.InspectTorrent(context.Background(), "dGVzdA==")
	if err != nil {
		t.Fatalf("inspect the torrent: %v", err)
	}

	if inspection.Name != "Ubuntu" {
		t.Errorf("the name is %q, want Ubuntu", inspection.Name)
	}
	if inspection.TotalLength != 2048 {
		t.Errorf("the length is %d, want 2048", inspection.TotalLength)
	}
	if len(inspection.Files) != 1 || inspection.Files[0].Path != "ubuntu.iso" {
		t.Errorf("the files are %v", inspection.Files)
	}
}

// TestTheTorrentIsDecoded verifies that a torrent file travels as base64.
func TestTheTorrentIsDecoded(t *testing.T) {
	data, err := DecodeTorrent("dGVzdA==")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(data) != "test" {
		t.Errorf("the content is %q, want test", data)
	}

	// A data URL is accepted, because that is what a browser hands out.
	data, err = DecodeTorrent("data:application/x-bittorrent;base64,dGVzdA==")
	if err != nil {
		t.Fatalf("decode the data URL: %v", err)
	}
	if string(data) != "test" {
		t.Errorf("the content is %q, want test", data)
	}

	// An empty value is no torrent, and not a failure.
	data, err = DecodeTorrent("")
	if err != nil {
		t.Fatalf("decode the empty value: %v", err)
	}
	if data != nil {
		t.Errorf("the content is %v, want nothing", data)
	}
}

// TestRemovingADownloadAsksTheEngineTwice verifies the calls that end
// a download.
//
// The engine has no call for stopping the sharing of a torrent: the
// download is removed instead, and the engine's own record of it goes as
// well, so that it does not come back on the next sync. Both calls
// carry the handle of the download.
func TestRemovingADownloadAsksTheEngineTwice(t *testing.T) {
	var methods []string
	var gids []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode the request: %v", err)
			return
		}

		methods = append(methods, request.Method)
		if len(request.Params) > 1 {
			if gid, ok := request.Params[1].(string); ok {
				gids = append(gids, gid)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result":  "OK",
		}); err != nil {
			t.Errorf("encode the reply: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret")

	if err := client.Remove(context.Background(), "gid-1"); err != nil {
		t.Fatalf("remove the download: %v", err)
	}
	if err := client.RemoveResult(context.Background(), "gid-1"); err != nil {
		t.Fatalf("remove the engine's record: %v", err)
	}

	want := []string{methodRemove, methodRemoveResult}
	if len(methods) != len(want) {
		t.Fatalf("the engine was asked %v, want %v", methods, want)
	}
	for i, method := range want {
		if methods[i] != method {
			t.Errorf("the call %d is %q, want %q", i, methods[i], method)
		}
	}
	for i, gid := range gids {
		if gid != "gid-1" {
			t.Errorf("the call %d carries %q, want %q", i, gid, "gid-1")
		}
	}
}

// TestTheEngineKeepsADownloadThatIsPausedAndRemovedAtOnce verifies
// what the engine does with a download that is removed while it is
// still pausing.
//
// The engine answers the pause at once and pauses the download a
// moment later. A download that is removed in between is not removed:
// the engine answers the removal with a success and keeps the download
// in its queue. The caller has to wait for the pause to take effect
// before it removes the download.
//
// This is the behaviour that made "stop seeding" a no-op: the engine
// kept the torrent, and it went on sharing it.
func TestTheEngineKeepsADownloadThatIsPausedAndRemovedAtOnce(t *testing.T) {
	// The engine answers the removal with a success, and answers the
	// question about the download with "paused": the removal did not
	// take, although it said it did.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode the request: %v", err)
			return
		}

		result := any("OK")
		if request.Method == methodTellStatus {
			result = map[string]any{
				"gid":        "gid-1",
				"status":     "paused",
				"files":      []any{},
				"bittorrent": map[string]any{},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result":  result,
		}); err != nil {
			t.Errorf("encode the reply: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret")
	ctx := context.Background()

	// The removal says it worked.
	if err := client.Remove(ctx, "gid-1"); err != nil {
		t.Fatalf("remove the download: %v", err)
	}

	// And the engine still holds the download, paused.
	status, err := client.Status(ctx, "gid-1")
	if err != nil {
		t.Fatalf("read the status: %v", err)
	}
	if status.Status != StatusPaused {
		t.Errorf("status = %q, want %q: the removal did not take", status.Status, StatusPaused)
	}
}

// TestTheRefusalsOfTheEngineAreRecognised verifies the two refusals
// that are not failures.
//
// The engine refuses to pause a download that is not running, and to
// resume one that is not paused. Both of them are races: the download
// may have come to rest between the moment the caller read its state
// and the moment it asked for the change. The refusal carries no
// information the caller did not already have.
func TestTheRefusalsOfTheEngineAreRecognised(t *testing.T) {
	notPausable := errors.New("aria2 error 1: GID 0123456789abcdef cannot be paused now")
	if !IsNotPausableNow(notPausable) {
		t.Error("the refusal of the pause should be recognised")
	}
	if IsNotResumableNow(notPausable) {
		t.Error("the refusal of the pause is not a refusal of the resume")
	}

	notResumable := errors.New("aria2 error 1: GID 0123456789abcdef cannot be unpaused now")
	if !IsNotResumableNow(notResumable) {
		t.Error("the refusal of the resume should be recognised")
	}
	if IsNotPausableNow(notResumable) {
		t.Error("the refusal of the resume is not a refusal of the pause")
	}

	other := errors.New("aria2 error 1: something else went wrong")
	if IsNotPausableNow(other) || IsNotResumableNow(other) {
		t.Error("another failure should not be recognised as a refusal")
	}

	if IsNotPausableNow(nil) || IsNotResumableNow(nil) {
		t.Error("no failure is not a refusal")
	}
}
