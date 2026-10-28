//go:build linux && amd64

package aria2

import "embed"

// bundledAssets holds the bundled binary of this build.
//
// Only the binary this build can run is embedded: a Linux amd64
// executable has no use for the aarch64 one, and carrying it would grow
// the download by 18 MB for nothing.
//
// The directive always matches at least one file, because the directory
// is kept in the repository by its .gitkeep, so a fresh clone builds.
//
//go:embed all:assets/linux-x86_64
var bundledAssets embed.FS
