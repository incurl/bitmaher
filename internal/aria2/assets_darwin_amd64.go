//go:build darwin && amd64

package aria2

import "embed"

// bundledAssets holds the bundled binary of this build.
//
//go:embed all:assets/darwin-x86_64
var bundledAssets embed.FS
