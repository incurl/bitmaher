package aria2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Client is the typed JSON-RPC client of the Aria2-Next sidecar.
//
// # LICENSING BOUNDARY
//
// The client speaks the JSON-RPC protocol over a loopback HTTP
// connection. That is the whole of the coupling between BitMaher and
// Aria2-Next: two independent programs, one a child process, talking
// over a socket. No aria2 code is linked in, imported or embedded.
//
// The client is safe for concurrent use. Every call carries the
// sidecar's secret token, which is how the engine authenticates the
// caller: a request without a valid token is refused with
// "Unauthorized".
type Client struct {
	// endpoint is the full URL of the JSON-RPC endpoint.
	endpoint string
	// token is the shared secret the engine requires.
	token string
	// http is the client used for the calls.
	http *http.Client
	// sequence numbers the requests, so that a reply can be matched
	// to its request.
	sequence atomic.Int64
}

// The methods of the engine's RPC surface.
//
// The names are the ones the engine actually implements. Several of
// them differ from the names the upstream aria2 documentation uses,
// which was verified against aria2-next 2.8.2 and 3.0.9:
//
//   - aria2.getFiles, not aria2.tellFiles.
//   - aria2.changeGlobalOption, not aria2.setGlobalOption.
//   - aria2.tellWaiting and aria2.tellStopped take an offset and a
//     count, while aria2.tellActive takes neither.
const (
	methodAddURI             = "aria2.addUri"
	methodAddTorrent         = "aria2.addTorrent"
	methodAddMetalink        = "aria2.addMetalink"
	methodForcePause         = "aria2.forcePause"
	methodPause              = "aria2.pause"
	methodForceUnpause       = "aria2.forceUnpause"
	methodUnpause            = "aria2.unpause"
	methodRemove             = "aria2.remove"
	methodForceRemove        = "aria2.forceRemove"
	methodTellStatus         = "aria2.tellStatus"
	methodTellActive         = "aria2.tellActive"
	methodTellWaiting        = "aria2.tellWaiting"
	methodTellStopped        = "aria2.tellStopped"
	methodGetFiles           = "aria2.getFiles"
	methodChangeOption       = "aria2.changeOption"
	methodChangeGlobalOption = "aria2.changeGlobalOption"
	methodGetGlobalStat      = "aria2.getGlobalStat"
	methodGetGlobalOption    = "aria2.getGlobalOption"
	methodGetVersion         = "aria2.getVersion"
	methodPurgeResult        = "aria2.purgeDownloadResult"
	methodRemoveResult       = "aria2.removeDownloadResult"
	methodShutdown           = "aria2.shutdown"
	methodForceShutdown      = "aria2.forceShutdown"
)

// DefaultTimeout is how long a single RPC call may take.
//
// The engine answers in milliseconds on a loopback socket. The bound
// is there so that a wedged sidecar cannot hang a UI call forever.
const DefaultTimeout = 10 * time.Second

// NewClient creates a client for the sidecar at endpoint.
//
// The token is the secret the sidecar was started with. It is never
// logged and never returned to the frontend.
func NewClient(endpoint, token string) *Client {
	return &Client{
		endpoint: endpoint,
		token:    token,
		http: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// Endpoint returns the URL of the JSON-RPC endpoint.
func (c *Client) Endpoint() string { return c.endpoint }

// call performs one JSON-RPC call and decodes the result into out.
//
// The engine reports an error with HTTP 400 and a body that still
// follows the JSON-RPC shape, so the body is parsed first and the
// status code only decides whether there is a body at all. Treating a
// 400 as a transport failure would hide the engine's own message,
// which is the part that tells the user what went wrong.
func (c *Client) call(ctx context.Context, method string, params []any, out any) error {
	if c == nil {
		return fmt.Errorf("aria2: the client is not configured")
	}

	if params == nil {
		params = []any{}
	}
	// The secret token is always the first parameter.
	params = append([]any{"token:" + c.token}, params...)

	id := strconv.FormatInt(c.sequence.Add(1), 10)
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("aria2: encode the %s request: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("aria2: build the %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("aria2: the %s call failed: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("aria2: read the %s response: %w", method, err)
	}

	var decoded rpcResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return fmt.Errorf("aria2: the %s response is not JSON: %w", method, err)
	}

	if decoded.Error != nil {
		return decoded.Error
	}

	if out == nil {
		return nil
	}
	if len(decoded.Result) == 0 {
		return fmt.Errorf("aria2: the %s response carries no result", method)
	}
	if err := json.Unmarshal(decoded.Result, out); err != nil {
		return fmt.Errorf("aria2: decode the %s result: %w", method, err)
	}
	return nil
}

// Version returns the identity of the engine.
//
// It is also the call the manager uses to verify that the binary it
// started is really aria2-next.
func (c *Client) Version(ctx context.Context) (Version, error) {
	var raw rawVersion
	if err := c.call(ctx, methodGetVersion, nil, &raw); err != nil {
		return Version{}, err
	}
	return Version{
		Product:    raw.Product,
		Version:    raw.Version,
		RPCVersion: raw.RPCVersion,
	}, nil
}

// AddURI adds a download from a list of URIs.
//
// The options are the engine's own options, such as "dir" and "out".
// The names are the ones the engine understands.
func (c *Client) AddURI(ctx context.Context, uris []string, options map[string]string) (string, error) {
	if len(uris) == 0 {
		return "", fmt.Errorf("aria2: a download needs at least one URI")
	}
	params := []any{uris}
	if len(options) > 0 {
		params = append(params, options)
	}
	var gid string
	if err := c.call(ctx, methodAddURI, params, &gid); err != nil {
		return "", err
	}
	return gid, nil
}

// AddTorrent adds a BitTorrent download from a .torrent file.
//
// The torrent is passed base64 encoded, as the RPC surface requires.
// The select option holds the one-based indices of the files the user
// chose; an empty selection downloads every file.
//
// The files of a torrent can only be chosen this way, and only while
// the download is paused: the engine refuses to change the selection
// of a running download.
func (c *Client) AddTorrent(ctx context.Context, torrentBase64 string, options map[string]string) (string, error) {
	if torrentBase64 == "" {
		return "", fmt.Errorf("aria2: the torrent file is empty")
	}
	params := []any{torrentBase64}
	if len(options) > 0 {
		params = append(params, options)
	}
	var gid string
	if err := c.call(ctx, methodAddTorrent, params, &gid); err != nil {
		return "", err
	}
	return gid, nil
}

// AddMetalink adds a download from a metalink document.
func (c *Client) AddMetalink(ctx context.Context, metalinkBase64 string, options map[string]string) (string, error) {
	if metalinkBase64 == "" {
		return "", fmt.Errorf("aria2: the metalink document is empty")
	}
	params := []any{metalinkBase64}
	if len(options) > 0 {
		params = append(params, options)
	}
	var gid string
	if err := c.call(ctx, methodAddMetalink, params, &gid); err != nil {
		return "", err
	}
	return gid, nil
}

// Status returns the state of one download.
func (c *Client) Status(ctx context.Context, gid string) (Download, error) {
	var raw rawStatus
	if err := c.call(ctx, methodTellStatus, []any{gid}, &raw); err != nil {
		return Download{}, err
	}
	return toDownload(raw), nil
}

// Active returns the running downloads.
func (c *Client) Active(ctx context.Context) ([]Download, error) {
	return c.list(ctx, methodTellActive, []any{})
}

// Waiting returns the queued downloads.
//
// The engine wants an offset and a count for this call.
func (c *Client) Waiting(ctx context.Context, offset, count int) ([]Download, error) {
	return c.list(ctx, methodTellWaiting, []any{offset, count})
}

// Stopped returns the finished downloads.
//
// The engine wants an offset and a count for this call.
func (c *Client) Stopped(ctx context.Context, offset, count int) ([]Download, error) {
	return c.list(ctx, methodTellStopped, []any{offset, count})
}

// list performs one of the listing calls and converts the result.
func (c *Client) list(ctx context.Context, method string, params []any) ([]Download, error) {
	var raw []rawStatus
	if err := c.call(ctx, method, params, &raw); err != nil {
		return nil, err
	}
	downloads := make([]Download, 0, len(raw))
	for _, item := range raw {
		downloads = append(downloads, toDownload(item))
	}
	return downloads, nil
}

// Files returns the files of a download.
//
// It is the call that answers "which files are in this torrent", and
// it is what the file selection of the add-task dialog is built on.
func (c *Client) Files(ctx context.Context, gid string) ([]File, error) {
	var raw []rawFileEntry
	if err := c.call(ctx, methodGetFiles, []any{gid}, &raw); err != nil {
		return nil, err
	}
	return toFiles(raw), nil
}

// Pause pauses a download.
func (c *Client) Pause(ctx context.Context, gid string) error {
	return c.call(ctx, methodPause, []any{gid}, nil)
}

// Resume resumes a paused download.
func (c *Client) Resume(ctx context.Context, gid string) error {
	return c.call(ctx, methodUnpause, []any{gid}, nil)
}

// Remove removes a download.
//
// The engine refuses to remove a running download, so the caller has
// to pause it first.
func (c *Client) Remove(ctx context.Context, gid string) error {
	return c.call(ctx, methodRemove, []any{gid}, nil)
}

// ForceRemove removes a download even when the engine is still working
// on it.
//
// It is the last resort, used when a download has to go and the engine
// would not let it go politely.
func (c *Client) ForceRemove(ctx context.Context, gid string) error {
	return c.call(ctx, methodForceRemove, []any{gid}, nil)
}

// ChangeOption changes the options of a running download.
//
// The engine only accepts the options that may change while a
// download runs, such as the speed limits.
func (c *Client) ChangeOption(ctx context.Context, gid string, options map[string]string) error {
	if len(options) == 0 {
		return nil
	}
	return c.call(ctx, methodChangeOption, []any{gid, options}, nil)
}

// ChangeGlobalOption changes the engine's global options.
//
// The speed limits live here: "max-overall-download-limit" and
// "max-overall-upload-limit".
func (c *Client) ChangeGlobalOption(ctx context.Context, options map[string]string) error {
	if len(options) == 0 {
		return nil
	}
	return c.call(ctx, methodChangeGlobalOption, []any{options}, nil)
}

// GlobalStat returns the engine's global state.
func (c *Client) GlobalStat(ctx context.Context) (GlobalStat, error) {
	var raw rawGlobalStat
	if err := c.call(ctx, methodGetGlobalStat, nil, &raw); err != nil {
		return GlobalStat{}, err
	}
	return toGlobalStat(raw), nil
}

// GlobalOption returns the engine's global options.
func (c *Client) GlobalOption(ctx context.Context) (map[string]string, error) {
	options := map[string]string{}
	if err := c.call(ctx, methodGetGlobalOption, nil, &options); err != nil {
		return nil, err
	}
	return options, nil
}

// PurgeResults discards the records of the finished downloads.
//
// It does not touch the downloaded files, only the engine's history of
// them.
func (c *Client) PurgeResults(ctx context.Context) error {
	return c.call(ctx, methodPurgeResult, nil, nil)
}

// RemoveResult discards the record of one finished download.
func (c *Client) RemoveResult(ctx context.Context, gid string) error {
	return c.call(ctx, methodRemoveResult, []any{gid}, nil)
}

// Shutdown asks the engine to stop.
//
// The engine finishes what it is doing and then exits, so it is only
// used when the application is closing.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.call(ctx, methodShutdown, nil, nil)
}

// ForceShutdown stops the engine at once.
func (c *Client) ForceShutdown(ctx context.Context) error {
	return c.call(ctx, methodForceShutdown, nil, nil)
}
