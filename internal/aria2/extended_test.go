package aria2

import (
	"context"
	"encoding/json"
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
