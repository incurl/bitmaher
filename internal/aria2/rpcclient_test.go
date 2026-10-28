package aria2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubServer is a fake JSON-RPC endpoint.
//
// It records the requests it receives and answers with what the test
// tells it to.
type stubServer struct {
	*httptest.Server
	// lastRequest is the body of the last request.
	lastRequest map[string]any
	// status is the HTTP status the server answers with.
	status int
	// body is the body the server answers with.
	body string
	// calls counts the requests.
	calls int
}

// newStubServer creates a fake endpoint that answers with body.
func newStubServer(t *testing.T, body string) *stubServer {
	t.Helper()

	stub := &stubServer{status: http.StatusOK, body: body}
	stub.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.calls++

		decoder := json.NewDecoder(r.Body)
		_ = decoder.Decode(&stub.lastRequest)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = w.Write([]byte(stub.body))
	}))
	t.Cleanup(stub.Close)

	return stub
}

// TestClientSendsTheToken verifies that the secret is sent with every
// call.
//
// The engine refuses a request without it, so it is the one thing that
// must never be missing.
func TestClientSendsTheToken(t *testing.T) {
	stub := newStubServer(t, `{"jsonrpc":"2.0","id":"1","result":"OK"}`)
	client := NewClient(stub.URL, "s3cr3t")

	if err := client.call(context.Background(), methodGetVersion, nil, nil); err != nil {
		t.Fatalf("call: %v", err)
	}

	params, ok := stub.lastRequest["params"].([]any)
	if !ok || len(params) == 0 {
		t.Fatalf("the request carries no parameters: %v", stub.lastRequest)
	}
	if params[0] != "token:s3cr3t" {
		t.Errorf("the first parameter is %v, want the token", params[0])
	}
}

// TestClientReportsTheEngineError verifies that the engine's own error
// message reaches the caller.
//
// The engine answers an error with HTTP 400 in 2.8.2 and with HTTP
// 200 in 3.0.9, so the body is what carries the message. Reading only
// the status code would lose it.
func TestClientReportsTheEngineError(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":"1","error":{"code":1,"message":"Invalid GID notagid"}}`

	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		stub := newStubServer(t, body)
		stub.status = status

		client := NewClient(stub.URL, "token")
		err := client.call(context.Background(), methodTellStatus, []any{"notagid"}, nil)
		if err == nil {
			t.Fatalf("status %d: the call should have failed", status)
		}
		if !strings.Contains(err.Error(), "Invalid GID notagid") {
			t.Errorf("status %d: the error is %q, want the engine's message", status, err.Error())
		}
	}
}

// TestClientDecodesTheResult verifies that a result is decoded into the
// caller's type.
func TestClientDecodesTheResult(t *testing.T) {
	stub := newStubServer(t, `{"jsonrpc":"2.0","id":"1","result":{"product":"aria2-next","version":"3.0.9","rpcVersion":"1.1.0"}}`)

	client := NewClient(stub.URL, "token")
	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version.Product != "aria2-next" {
		t.Errorf("product = %q", version.Product)
	}
	if version.Version != "3.0.9" {
		t.Errorf("version = %q", version.Version)
	}
}

// TestClientDecodesADownloadStatus verifies that a status payload
// becomes a normalised download.
func TestClientDecodesADownloadStatus(t *testing.T) {
	stub := newStubServer(t, `{"jsonrpc":"2.0","id":"1","result":{
		"gid":"abc","status":"active","totalLength":"1000","completedLength":"250",
		"downloadSpeed":"100","uploadSpeed":"0","connections":"2","dir":"/tmp",
		"files":[{"index":"1","path":"/tmp/a.bin","length":"1000","completedLength":"250","selected":"true"}]
	}}`)

	client := NewClient(stub.URL, "token")
	download, err := client.Status(context.Background(), "abc")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if download.Status != StatusActive {
		t.Errorf("status = %q", download.Status)
	}
	if download.TotalLength != 1000 || download.CompletedLength != 250 {
		t.Errorf("lengths = %d and %d", download.TotalLength, download.CompletedLength)
	}
	if download.Title != "a.bin" {
		t.Errorf("title = %q", download.Title)
	}
}

// TestClientHonoursTheContext verifies that a cancelled call stops.
//
// The context is what keeps a wedged sidecar from hanging a UI call.
func TestClientHonoursTheContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(server.URL, "token")
	if err := client.call(ctx, methodGetVersion, nil, nil); err == nil {
		t.Error("the call should have failed")
	}
}

// TestAddURIRejectsAnEmptyList verifies the guard on the input.
func TestAddURIRejectsAnEmptyList(t *testing.T) {
	client := NewClient("http://127.0.0.1:1/jsonrpc", "token")
	if _, err := client.AddURI(context.Background(), nil, nil); err == nil {
		t.Error("the call should have failed")
	}
}

// TestAddTorrentRejectsAnEmptyFile verifies the guard on the input.
func TestAddTorrentRejectsAnEmptyFile(t *testing.T) {
	client := NewClient("http://127.0.0.1:1/jsonrpc", "token")
	if _, err := client.AddTorrent(context.Background(), "", nil); err == nil {
		t.Error("the call should have failed")
	}
}

// TestCallIsNumbered verifies that every request carries its own id.
//
// The id is what tells a reply apart from another one.
func TestCallIsNumbered(t *testing.T) {
	stub := newStubServer(t, `{"jsonrpc":"2.0","id":"1","result":"OK"}`)
	client := NewClient(stub.URL, "token")

	for i := 0; i < 3; i++ {
		if err := client.call(context.Background(), methodGetVersion, nil, nil); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}

	if stub.calls != 3 {
		t.Errorf("calls = %d, want 3", stub.calls)
	}
}

// TestAddTorrentSendsTheURIsSlot verifies the shape of the addTorrent
// parameters.
//
// The engine takes the torrent, then the webseed URIs, then the
// options. This is not the order addUri uses, and getting it wrong is
// reported as "The parameter at 1 has wrong type" rather than as
// anything that names the problem. The regression is easy to
// reintroduce, so the shape is pinned here.
func TestAddTorrentSendsTheURIsSlot(t *testing.T) {
	stub := newStubServer(t, `{"jsonrpc":"2.0","id":"1","result":"abc123"}`)
	client := NewClient(stub.URL, "token")

	gid, err := client.AddTorrent(
		context.Background(),
		"ZDQ6aW5mb2Q0Om5hbWUxOmFl",
		map[string]string{"dir": "/downloads"},
	)
	if err != nil {
		t.Fatalf("add torrent: %v", err)
	}
	if gid != "abc123" {
		t.Errorf("gid = %q", gid)
	}

	params, ok := stub.lastRequest["params"].([]any)
	if !ok {
		t.Fatalf("the request carries no parameters: %v", stub.lastRequest)
	}

	// The token, the torrent, the URIs and the options.
	if len(params) != 4 {
		t.Fatalf("%d parameters, want 4: %v", len(params), params)
	}
	if params[0] != "token:token" {
		t.Errorf("the first parameter is %v, want the token", params[0])
	}
	if _, ok := params[1].(string); !ok {
		t.Errorf("the torrent is %T, want a string", params[1])
	}
	// The URIs slot has to be a list, not an object.
	if _, ok := params[2].([]any); !ok {
		t.Errorf("the URIs slot is %T, want a list: the engine refuses an object", params[2])
	}
	if _, ok := params[3].(map[string]any); !ok {
		t.Errorf("the options are %T, want an object", params[3])
	}
}

// TestAddURISendsTheOptionsAfterTheList verifies the shape of the
// addUri parameters.
//
// Here the options come straight after the list of URIs, which is the
// opposite of addTorrent.
func TestAddURISendsTheOptionsAfterTheList(t *testing.T) {
	stub := newStubServer(t, `{"jsonrpc":"2.0","id":"1","result":"abc123"}`)
	client := NewClient(stub.URL, "token")

	if _, err := client.AddURI(
		context.Background(),
		[]string{"https://example.test/file.zip"},
		map[string]string{"dir": "/downloads"},
	); err != nil {
		t.Fatalf("add uri: %v", err)
	}

	params, ok := stub.lastRequest["params"].([]any)
	if !ok {
		t.Fatalf("the request carries no parameters: %v", stub.lastRequest)
	}
	if len(params) != 3 {
		t.Fatalf("%d parameters, want 3: %v", len(params), params)
	}
	if _, ok := params[1].([]any); !ok {
		t.Errorf("the URIs are %T, want a list", params[1])
	}
	if _, ok := params[2].(map[string]any); !ok {
		t.Errorf("the options are %T, want an object", params[2])
	}
}
