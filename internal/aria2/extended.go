package aria2

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// The calls Aria2-Next adds to the upstream aria2.
//
// They were verified against aria2-next 3.0.9 by asking the engine for
// its own list of methods, and each one was called to see what it
// answers. The upstream aria2 does not have them, so a call against it
// fails with "method not found".

// Probe is what a download from a URL would receive.
//
// The engine makes the request, so it goes out with the options and the
// headers the download itself would carry.
type Probe struct {
	// Status is the HTTP status the server answered with. "0" means
	// the server was not reached at all.
	Status string `json:"status"`
	// FinalURL is the address the request ended at, after the
	// redirections.
	FinalURL string `json:"finalUrl"`
	// ContentType is the content type the server announced.
	ContentType string `json:"contentType"`
	// Resumable reports whether the server accepts a range request.
	Resumable string `json:"resumable"`
	// TotalLength is the size the server announced, as a string.
	TotalLength string `json:"totalLength"`
	// Filename is the name the server suggested, when it sent one.
	Filename string `json:"filename"`
	// Error is the explanation of a failed request.
	Error string `json:"error"`
}

// Peer is one peer of a BitTorrent download.
type Peer struct {
	// PeerID is the identifier the peer announces.
	PeerID string `json:"peerId"`
	// IP is the address of the peer.
	IP string `json:"ip"`
	// Port is the port the peer listens on.
	Port int `json:"port"`
	// DownloadSpeed is the speed of the peer, in bytes per second.
	DownloadSpeed int64 `json:"downloadSpeed"`
	// UploadSpeed is the speed towards the peer, in bytes per second.
	UploadSpeed int64 `json:"uploadSpeed"`
	// Bitfield is the pieces the peer has.
	Bitfield string `json:"bitfield"`
	// Choking reports whether the peer is choking us.
	Choking string `json:"choking"`
	// PeerChoking reports whether we are choking the peer.
	PeerChoking string `json:"peerChoking"`
}

// Tracker is one tracker of a BitTorrent download.
type Tracker struct {
	// ID is the identifier of the tracker.
	ID int `json:"id"`
	// AnnounceURL is the address the tracker is announced to.
	AnnounceURL string `json:"announceUrl"`
	// Status is the state of the tracker.
	Status string `json:"status"`
	// Tier is the tier the tracker is in.
	Tier int `json:"tier"`
}

// TorrentFile is one file of a torrent.
type TorrentFile struct {
	// Index is the one-based index of the file.
	Index int `json:"index"`
	// Path is the path of the file inside the torrent.
	Path string `json:"path"`
	// Length is the size of the file.
	Length int64 `json:"length"`
}

// TorrentInspection is what a torrent file holds.
//
// It is read without creating a download, so the add dialog can show
// the files before the user commits to them.
type TorrentInspection struct {
	// Name is the name of the torrent.
	Name string `json:"name"`
	// Mode is "single" or "multi": whether the torrent holds one file
	// or several.
	Mode string `json:"mode"`
	// InfoHashV1 is the info hash of the first generation.
	InfoHashV1 string `json:"infoHashV1"`
	// InfoHashV2 is the info hash of the second generation.
	InfoHashV2 string `json:"infoHashV2"`
	// TotalLength is the size of the whole torrent.
	TotalLength int64 `json:"totalLength"`
	// Files are the files of the torrent.
	Files []TorrentFile `json:"files"`
}

// PeerAddResult is what the engine reports when peers are added.
type PeerAddResult struct {
	// Added is the number of peers that were accepted.
	Added int `json:"added"`
	// Failed is the number of peers that were refused.
	Failed int `json:"failed"`
}

// Probe asks the engine what a download from a URL would receive.
//
// The options are the ones the download would carry, so the request
// goes out with the same headers and the same credentials.
func (c *Client) Probe(ctx context.Context, url string, options map[string]string) (Probe, error) {
	var probe Probe
	if err := c.call(ctx, methodProbe, []any{url, options}, &probe); err != nil {
		return Probe{}, err
	}
	return probe, nil
}

// InspectTorrent reads a torrent file without creating a download.
//
// The content is the base64 of the file, because the bindings carry
// JSON.
func (c *Client) InspectTorrent(ctx context.Context, contentBase64 string) (TorrentInspection, error) {
	var inspection TorrentInspection
	if err := c.call(ctx, methodInspectTorrent, []any{contentBase64}, &inspection); err != nil {
		return TorrentInspection{}, err
	}
	return inspection, nil
}

// Peers returns the peers of a BitTorrent download.
func (c *Client) Peers(ctx context.Context, gid string) ([]Peer, error) {
	var peers []Peer
	if err := c.call(ctx, methodGetPeers, []any{gid}, &peers); err != nil {
		return nil, err
	}
	return peers, nil
}

// BtTrackers returns the trackers of a BitTorrent download.
func (c *Client) BtTrackers(ctx context.Context, gid string) ([]Tracker, error) {
	var trackers []Tracker
	if err := c.call(ctx, methodGetBtTrackers, []any{gid}, &trackers); err != nil {
		return nil, err
	}
	return trackers, nil
}

// ReplaceBtTrackers replaces the trackers of a BitTorrent download.
//
// The trackers are the announce addresses, in the order they should be
// used.
func (c *Client) ReplaceBtTrackers(ctx context.Context, gid string, trackers []string) error {
	return c.call(ctx, methodReplaceBtTrackers, []any{gid, trackers}, nil)
}

// ReplaceBtWebSeeds replaces the web seeds of a BitTorrent download.
func (c *Client) ReplaceBtWebSeeds(ctx context.Context, gid string, seeds []string) error {
	return c.call(ctx, methodReplaceBtWebSeeds, []any{gid, seeds}, nil)
}

// AddBtPeers adds peers to a BitTorrent download.
func (c *Client) AddBtPeers(ctx context.Context, gid string, peers []string) (PeerAddResult, error) {
	var result PeerAddResult
	if err := c.call(ctx, methodAddBtPeers, []any{gid, peers}, &result); err != nil {
		return PeerAddResult{}, err
	}
	return result, nil
}

// ForceBtRecheck asks the engine to check the data of a torrent again.
func (c *Client) ForceBtRecheck(ctx context.Context, gid string) error {
	return c.call(ctx, methodForceBtRecheck, []any{gid}, nil)
}

// ChangeURI replaces the source of a download.
//
// The engine only accepts the change while the download is paused.
func (c *Client) ChangeURI(ctx context.Context, gid string, uris []string, position int) error {
	return c.call(ctx, methodChangeURI, []any{gid, uris, position}, nil)
}

// ResolveFilename works out the name the engine would give a download.
//
// The content disposition is the one the server sent, and it is empty
// when the server sent none.
func (c *Client) ResolveFilename(ctx context.Context, url string, contentDisposition []byte) (string, error) {
	var name string
	if err := c.call(ctx, methodResolveFilename, []any{url, contentDisposition}, &name); err != nil {
		return "", err
	}
	return name, nil
}

// Option returns the options of a download.
func (c *Client) Option(ctx context.Context, gid string) (map[string]string, error) {
	options := map[string]string{}
	if err := c.call(ctx, methodGetOption, []any{gid}, &options); err != nil {
		return nil, err
	}
	return options, nil
}

// Uris returns the sources of a download.
func (c *Client) Uris(ctx context.Context, gid string) ([]string, error) {
	var uris []string
	if err := c.call(ctx, methodGetUris, []any{gid}, &uris); err != nil {
		return nil, err
	}
	return uris, nil
}

// SessionInfo returns what the engine knows about its session file.
func (c *Client) SessionInfo(ctx context.Context) (map[string]string, error) {
	info := map[string]string{}
	if err := c.call(ctx, methodGetSessionInfo, nil, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// SaveSession writes the session file.
//
// The engine writes it on its own, so this only asks for it to happen
// now.
func (c *Client) SaveSession(ctx context.Context) error {
	return c.call(ctx, methodSaveSession, nil, nil)
}

// SetBtPeerBlocklist replaces the peer blocklist of the engine.
//
// The blocklist is the list of peers the engine refuses to talk to.
func (c *Client) SetBtPeerBlocklist(ctx context.Context, peers []string) error {
	return c.call(ctx, methodSetBtPeerBlocklist, []any{peers}, nil)
}

// BtSessionStatus returns the state of the BitTorrent session of the
// engine: the trackers it announces to and the peers it knows.
func (c *Client) BtSessionStatus(ctx context.Context) (map[string]any, error) {
	status := map[string]any{}
	if err := c.call(ctx, methodGetBtSessionStatus, nil, &status); err != nil {
		return nil, err
	}
	return status, nil
}

// ForceBtAnnounce asks the engine to announce the torrents to their
// trackers now, instead of waiting for the next interval.
func (c *Client) ForceBtAnnounce(ctx context.Context) error {
	return c.call(ctx, methodForceBtAnnounce, nil, nil)
}

// MoveDownload moves a download to another directory.
//
// The files on disk are moved as well.
func (c *Client) MoveDownload(ctx context.Context, gid, dir string) error {
	return c.call(ctx, methodMoveDownload, []any{gid, dir}, nil)
}

// EncodeTorrent encodes the bytes of a torrent file as base64.
//
// The bindings carry JSON, so a file has to travel as a string.
func EncodeTorrent(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeTorrent decodes the base64 of a torrent file.
//
// A data URL is accepted as well, because that is what a browser hands
// out when a file is read from an input.
func DecodeTorrent(encoded string) ([]byte, error) {
	cleaned := strings.TrimSpace(encoded)
	if cleaned == "" {
		return nil, nil
	}

	if index := strings.Index(cleaned, ","); strings.HasPrefix(cleaned, "data:") && index > 0 {
		cleaned = cleaned[index+1:]
	}

	data, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("aria2: the torrent file is not valid base64: %w", err)
	}
	return data, nil
}
