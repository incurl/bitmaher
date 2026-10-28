//go:build darwin && arm64

package aria2

import "embed"

// bundledAssets holds the bundled binary of this build.
//
//go:embed all:assets/darwin-arm64
var bundledAssets embed.FS
