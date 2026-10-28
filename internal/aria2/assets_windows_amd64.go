//go:build windows && amd64

package aria2

import "embed"

// bundledAssets holds the bundled binary of this build.
//
//go:embed all:assets/windows-x86_64
var bundledAssets embed.FS
