package aria2

import (
	"path/filepath"
	"strings"
)

// toDownload converts the engine's status into the normalised model.
//
// This is the only place where the engine's vocabulary is translated.
// Everything downstream -- the persistence, the frontend, the tests --
// sees the normalised model.
//
// The conversion was written against the payloads of aria2-next 2.8.2
// and 3.0.9. A field the engine does not report is left at its zero
// value rather than guessed at.
func toDownload(raw rawStatus) Download {
	download := Download{
		GID:             raw.GID,
		Status:          NormalizeStatus(raw.Status),
		Dir:             raw.Dir,
		TotalLength:     parseInt64(raw.TotalLength),
		CompletedLength: parseInt64(raw.CompletedLength),
		UploadLength:    parseInt64(raw.UploadLength),
		DownloadSpeed:   parseInt64(raw.DownloadSpeed),
		UploadSpeed:     parseInt64(raw.UploadSpeed),
		Connections:     int(parseInt64(raw.Connections)),
		ErrorCode:       int(parseInt64(raw.ErrorCode)),
		ErrorMessage:    raw.ErrorMessage,
		InfoHash:        raw.InfoHash,
		Seeders:         int(parseInt64(raw.NumSeeders)),
		Peers:           int(parseInt64(raw.NumPeers)),
		Files:           []File{},
	}

	// A torrent reports its info hash inside the BitTorrent section,
	// and the plain status may not carry one.
	if download.InfoHash == "" && raw.BitTorrent != nil {
		download.InfoHash = raw.BitTorrent.InfoHashV1
	}

	// The engine keeps a finished torrent running so that it seeds,
	// and the status stays "active" for as long as it does. The two
	// facts below are what say that the download itself is over.
	download.Seeder = parseBool(raw.Seeder)
	if raw.BitTorrent != nil {
		download.TorrentState = raw.BitTorrent.State
	}

	// The files of the status are the ones the engine is downloading.
	for _, file := range raw.Files {
		download.Files = append(download.Files, toFile(file))
	}

	download.Title = downloadTitle(raw, download)
	download.URI = downloadURI(raw, download)

	return download
}

// toFile converts one file of the engine's status.
func toFile(raw rawFile) File {
	return File{
		Index:           int(parseInt64(raw.Index)),
		Path:            raw.Path,
		Length:          parseInt64(raw.Length),
		CompletedLength: parseInt64(raw.CompletedLength),
		Selected:        parseBool(raw.Selected),
		URIs:            toURIs(raw.URIs),
	}
}

// toURIs converts the sources of a file.
//
// The engine reports an empty list for a torrent's file, because the
// file is read from the swarm rather than from an address.
func toURIs(raw []rawURI) []URI {
	uris := make([]URI, 0, len(raw))
	for _, entry := range raw {
		uris = append(uris, URI{URI: entry.URI, Status: entry.Status})
	}
	return uris
}

// downloadTitle derives the display name of a download.
//
// The name comes from the torrent metadata when there is one, because
// that is the human name of the content. Otherwise it is the name of
// the file the engine is writing.
func downloadTitle(raw rawStatus, download Download) string {
	if raw.BitTorrent != nil && raw.BitTorrent.Info != nil && raw.BitTorrent.Info.Name != "" {
		return raw.BitTorrent.Info.Name
	}

	// The name of the file, without its directory.
	for _, file := range raw.Files {
		if file.Path != "" {
			return filepath.Base(file.Path)
		}
	}

	// A magnet link that has not fetched its metadata yet has no name
	// and no file: the info hash is the only thing that identifies it.
	if download.InfoHash != "" {
		return download.InfoHash
	}

	return download.GID
}

// downloadURI derives the source of a download.
//
// It is the magnet link when the engine has one, and the first URI of
// the first file otherwise.
func downloadURI(raw rawStatus, download Download) string {
	if raw.BitTorrent != nil && raw.BitTorrent.MagnetLink != "" {
		return raw.BitTorrent.MagnetLink
	}

	for _, file := range raw.Files {
		for _, uri := range file.URIs {
			if uri.URI != "" {
				return uri.URI
			}
		}
	}

	for _, uri := range raw.URIs {
		if uri.URI != "" {
			return uri.URI
		}
	}

	return ""
}

// toFiles converts the result of the getFiles call.
//
// The engine returns the same shape as the files of a status, but it
// is a separate call, so it is decoded separately.
func toFiles(raw []rawFileEntry) []File {
	files := make([]File, 0, len(raw))
	for _, entry := range raw {
		files = append(files, File{
			Index:           int(parseInt64(entry.Index)),
			Path:            entry.Path,
			Length:          parseInt64(entry.Length),
			CompletedLength: parseInt64(entry.CompletedLength),
			Selected:        parseBool(entry.Selected),
			URIs:            toURIs(entry.URIs),
		})
	}
	return files
}

// GlobalStat is the engine's global state.
//
// The frontend shows it in the header, so that the user sees the total
// speed without opening a download.
type GlobalStat struct {
	// DownloadSpeed is the total download speed in bytes per second.
	DownloadSpeed int64 `json:"download_speed"`
	// UploadSpeed is the total upload speed in bytes per second.
	UploadSpeed int64 `json:"upload_speed"`
	// NumActive is the number of running downloads.
	NumActive int `json:"num_active"`
	// NumWaiting is the number of queued downloads.
	NumWaiting int `json:"num_waiting"`
	// NumStopped is the number of stopped downloads.
	NumStopped int `json:"num_stopped"`
}

// toGlobalStat converts the engine's statistics.
func toGlobalStat(raw rawGlobalStat) GlobalStat {
	return GlobalStat{
		DownloadSpeed: parseInt64(raw.DownloadSpeed),
		UploadSpeed:   parseInt64(raw.UploadSpeed),
		NumActive:     int(parseInt64(raw.NumActive)),
		NumWaiting:    int(parseInt64(raw.NumWaiting)),
		NumStopped:    int(parseInt64(raw.NumStopped)),
	}
}

// Version is the identity of the engine.
//
// The manager uses it to verify that the binary it started really is
// aria2-next before it marks the sidecar as ready.
type Version struct {
	// Product is the engine's product name, "aria2-next".
	Product string `json:"product"`
	// Version is the engine's version.
	Version string `json:"version"`
	// RPCVersion is the version of the JSON-RPC protocol.
	RPCVersion string `json:"rpc_version"`
}

// IsAria2Next reports whether the engine identifies itself as
// aria2-next.
//
// The upstream aria2 reports no product at all, so a binary that
// identifies itself as something else is rejected: BitMaher is built
// against the aria2-next RPC surface, and the upstream aria2 does not
// have it.
func (v Version) IsAria2Next() bool {
	return strings.EqualFold(v.Product, "aria2-next")
}
