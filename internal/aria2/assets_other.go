//go:build !(linux && (amd64 || arm64)) && !(darwin && (amd64 || arm64)) && !(windows && amd64)

package aria2

import "embed"

// bundledAssets is empty on a platform BitMaher has no binary for.
//
// The application reports that no bundled binary is present and expects
// a system-installed aria2-next instead, rather than failing to build.
var bundledAssets embed.FS
