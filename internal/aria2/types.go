// Package aria2 owns everything that touches the Aria2-Next sidecar.
//
// # LICENSING BOUNDARY
//
// Aria2-Next is GPLv2 licensed. BitMaher runs it as a SEPARATE,
// INDEPENDENT CHILD PROCESS and talks to it only over JSON-RPC on
// localhost. Nothing in this package links against aria2, embeds its
// source, imports its headers or calls into it through CGO. The only
// coupling is the JSON-RPC protocol spoken over a loopback socket,
// which is an interface between two independent programs, not a link
// between two parts of one program.
//
// If that boundary is ever broken -- by linking aria2, vendoring its
// sources, or shipping its code inside this repository -- the licensing
// of the whole application changes. Keep the boundary.
//
// The package is split in three:
//
//   - types.go    the normalised download model and the RPC structs.
//   - rpcclient.go the typed JSON-RPC client.
//   - manager.go  the sidecar lifecycle: process, port, watchdog, logs.
package aria2

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Status is the normalised status of a download.
//
// Aria2 reports six states. They are mapped onto the five the
// application shows, so that the frontend never has to know about the
// engine's vocabulary.
type Status string

const (
	// StatusActive means the download is running right now.
	StatusActive Status = "active"
	// StatusWaiting means the download is queued, waiting for a slot.
	StatusWaiting Status = "waiting"
	// StatusPaused means the user paused the download.
	StatusPaused Status = "paused"
	// StatusComplete means the download finished successfully.
	StatusComplete Status = "complete"
	// StatusError means the download failed.
	StatusError Status = "error"
	// StatusRemoved means the download was removed from the engine.
	StatusRemoved Status = "removed"
)

// Terminal reports whether the status is a resting state.
//
// A download in a terminal state is not making progress and will not
// change on its own.
func (s Status) Terminal() bool {
	switch s {
	case StatusComplete, StatusError, StatusRemoved:
		return true
	default:
		return false
	}
}

// String returns the status as a plain string.
func (s Status) String() string { return string(s) }

// NormalizeStatus maps an aria2 status string onto a normalised status.
//
// The mapping is deliberately total: an unknown value becomes
// StatusWaiting rather than an error, because the engine may grow a
// state and an unknown one must not take the whole application down.
//
// The mapping was verified against aria2-next 2.8.2 and 3.0.9.
func NormalizeStatus(raw string) Status {
	switch raw {
	case "active":
		return StatusActive
	case "waiting":
		return StatusWaiting
	case "paused":
		return StatusPaused
	case "complete":
		return StatusComplete
	case "error":
		return StatusError
	case "removed":
		return StatusRemoved
	default:
		return StatusWaiting
	}
}

// Kind is the kind of a download.
type Kind string

const (
	// KindHTTP is a plain HTTP or HTTPS download.
	KindHTTP Kind = "http"
	// KindTorrent is a BitTorrent download, added from a .torrent file.
	KindTorrent Kind = "torrent"
	// KindMagnet is a BitTorrent download, added from a magnet link.
	KindMagnet Kind = "magnet"
	// KindMetalink is a Metalink download.
	KindMetalink Kind = "metalink"
)

// NormalizeKind maps the kind recorded by the application onto a kind.
//
// An unknown kind becomes KindHTTP, because that is the only kind that
// needs no extra information to be restarted.
func NormalizeKind(raw string) Kind {
	switch raw {
	case string(KindTorrent):
		return KindTorrent
	case string(KindMagnet):
		return KindMagnet
	case string(KindMetalink):
		return KindMetalink
	default:
		return KindHTTP
	}
}

// Download is the normalised view of a download.
//
// It is what the application persists and what the frontend sees. The
// engine's own vocabulary does not leak into it: the numbers are typed
// as integers and the status is one of the normalised values.
type Download struct {
	// GID is the engine's handle for the download. It is what every
	// RPC call refers to.
	GID string `json:"gid"`
	// Kind is the kind of the download.
	Kind Kind `json:"kind"`
	// Status is the normalised status.
	Status Status `json:"status"`
	// Title is the display name of the download: the file name, or
	// the torrent name.
	Title string `json:"title"`
	// URI is the source: the http URL, the magnet link or the
	// "magnet:" form of a torrent's info hash.
	URI string `json:"uri"`
	// Dir is the directory the download is written to.
	Dir string `json:"dir"`
	// TotalLength is the size of the download in bytes.
	TotalLength int64 `json:"total_length"`
	// CompletedLength is how many bytes have been downloaded.
	CompletedLength int64 `json:"completed_length"`
	// UploadLength is how many bytes have been uploaded.
	UploadLength int64 `json:"upload_length"`
	// DownloadSpeed is the current download speed in bytes per second.
	DownloadSpeed int64 `json:"download_speed"`
	// UploadSpeed is the current upload speed in bytes per second.
	UploadSpeed int64 `json:"upload_speed"`
	// Connections is the number of connections the download has.
	Connections int `json:"connections"`
	// Seeders is the number of seeders a BitTorrent download sees.
	Seeders int `json:"seeders"`
	// Peers is the number of peers a BitTorrent download sees.
	Peers int `json:"peers"`
	// InfoHash is the BitTorrent info hash, for a torrent or a magnet.
	InfoHash string `json:"info_hash"`
	// ErrorCode is the engine's error code, for a failed download.
	ErrorCode int `json:"error_code"`
	// ErrorMessage is the engine's error explanation.
	ErrorMessage string `json:"error_message"`
	// Files lists the files of the download.
	Files []File `json:"files"`
}

// Progress returns the completion ratio of the download, from 0 to 1.
//
// A download of unknown size has no progress, so it reports 0.
func (d Download) Progress() float64 {
	if d.TotalLength <= 0 {
		return 0
	}
	done := float64(d.CompletedLength) / float64(d.TotalLength)
	if done > 1 {
		return 1
	}
	if done < 0 {
		return 0
	}
	return done
}

// File is one file of a download.
type File struct {
	// Index is the file's one-based index inside the download.
	Index int `json:"index"`
	// Path is the absolute path of the file on disk.
	Path string `json:"path"`
	// Length is the size of the file in bytes.
	Length int64 `json:"length"`
	// CompletedLength is how many bytes of it have been downloaded.
	CompletedLength int64 `json:"completed_length"`
	// Selected reports whether the file is part of the download.
	//
	// A torrent has more files than the user chose, and the ones that
	// were not chosen are not downloaded.
	Selected bool `json:"selected"`
}

// ---------------------------------------------------------------------------
// The raw JSON-RPC payloads.
//
// Aria2 speaks in strings: every number it reports is a string, and
// every option it takes is a string. These structs mirror the wire
// format exactly, and the conversion into the normalised model happens
// in status.go.
// ---------------------------------------------------------------------------

// rpcRequest is a JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

// rpcResponse is a JSON-RPC 2.0 response.
//
// Aria2 reports an error with HTTP 400 and a body that still follows
// the JSON-RPC shape, so the body is parsed in both cases.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// rpcError is the error member of a JSON-RPC response.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("aria2 error %d: %s", e.Code, e.Message)
}

// rawStatus is the status of a download as the engine reports it.
//
// Every number is a string in the wire format.
type rawStatus struct {
	GID             string     `json:"gid"`
	Status          string     `json:"status"`
	TotalLength     string     `json:"totalLength"`
	CompletedLength string     `json:"completedLength"`
	UploadLength    string     `json:"uploadLength"`
	DownloadSpeed   string     `json:"downloadSpeed"`
	UploadSpeed     string     `json:"uploadSpeed"`
	Connections     string     `json:"connections"`
	ErrorCode       string     `json:"errorCode"`
	ErrorMessage    string     `json:"errorMessage"`
	Dir             string     `json:"dir"`
	InfoHash        string     `json:"infoHash"`
	NumSeeders      string     `json:"numSeeders"`
	NumPeers        string     `json:"numPeers"`
	Files           []rawFile  `json:"files"`
	URIs            []rawURI   `json:"uris"`
	BitTorrent      *rawBittor `json:"bittorrent"`
}

// rawFile is one file as the engine reports it.
type rawFile struct {
	Index           string   `json:"index"`
	Path            string   `json:"path"`
	Length          string   `json:"length"`
	CompletedLength string   `json:"completedLength"`
	Selected        string   `json:"selected"`
	URIs            []rawURI `json:"uris"`
}

// rawURI is one source of a file.
type rawURI struct {
	URI    string `json:"uri"`
	Status string `json:"status"`
}

// rawBittor is the BitTorrent section of a status.
type rawBittor struct {
	InfoHashV1 string   `json:"infoHashV1"`
	MagnetLink string   `json:"magnetLink"`
	Info       *rawInfo `json:"info"`
}

// rawInfo is the torrent metadata.
type rawInfo struct {
	Name string `json:"name"`
}

// rawGlobalStat is the global statistics of the engine.
type rawGlobalStat struct {
	DownloadSpeed   string `json:"downloadSpeed"`
	UploadSpeed     string `json:"uploadSpeed"`
	NumActive       string `json:"numActive"`
	NumWaiting      string `json:"numWaiting"`
	NumStopped      string `json:"numStopped"`
	NumStoppedTotal string `json:"numStoppedTotal"`
}

// rawVersion is the version of the engine.
//
// The product field is what identifies it as aria2-next rather than as
// the upstream aria2.
type rawVersion struct {
	Version    string   `json:"version"`
	RPCVersion string   `json:"rpcVersion"`
	Product    string   `json:"product"`
	Enabled    []string `json:"enabledFeatures"`
}

// rawFileEntry is one entry of the getFiles result.
//
// It is the same shape as rawFile, but the engine returns it under a
// slightly different set of keys in some versions, so it is decoded
// separately.
type rawFileEntry struct {
	Index           string `json:"index"`
	Path            string `json:"path"`
	Length          string `json:"length"`
	CompletedLength string `json:"completedLength"`
	Selected        string `json:"selected"`
}

// parseInt64 reads an aria2 number.
//
// Aria2 sends its numbers as strings. An empty or malformed value is
// zero rather than an error: a missing number means "not known", and
// the caller decides what to do with that.
func parseInt64(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// parseBool reads an aria2 boolean.
//
// Aria2 sends its booleans as the strings "true" and "false".
func parseBool(raw string) bool { return raw == "true" }
