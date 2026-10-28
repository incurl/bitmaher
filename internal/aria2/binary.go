package aria2

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// The bundled aria2-next binaries.
//
// # LICENSING BOUNDARY
//
// These files are PREBUILT BINARIES of Aria2-Next, which is GPLv2.
// They are shipped as opaque executables, exactly as the licence
// requires for a work that is distributed in binary form: the source
// is not vendored into this repository, the binaries are not linked
// against the Go code, and the application reaches them only by
// starting them as a separate process.
//
// The binaries are large (around 18 MB each), so they are kept out of
// the Git repository. The task that puts them in place is
// "common:fetch:aria2", and the embed directive below is satisfied by
// the placeholder files that ship with the repository.
//
// The version of the bundled binaries is pinned in aria2Version, so
// that the application can tell the user what it is running.
const aria2Version = "3.0.9"

// bundledAssets holds the bundled binary of the host platform.
//
// The declaration lives in a file per platform, so that a build carries
// only the binaries it can run. See assets_linux.go and the others.

// assetDir is the directory of the bundled binaries inside the embed
// filesystem.
//
// It is keyed by the operating system and the architecture, so that
// the right binary is picked for the machine the application runs on.
func assetDir() string {
	dir := runtime.GOOS
	if runtime.GOARCH == "amd64" {
		// The 64 bit build is called x86_64 by the upstream
		// releases.
		dir += "-x86_64"
	} else {
		dir += "-" + runtime.GOARCH
	}
	return "assets/" + dir
}

// assetName returns the file name of the bundled binary.
//
// Windows needs the .exe suffix, and nothing else does.
func assetName() string {
	if runtime.GOOS == "windows" {
		return "aria2-next.exe"
	}
	return "aria2-next"
}

// BundledVersion returns the version of the bundled aria2-next.
func BundledVersion() string { return aria2Version }

// HasBundledBinary reports whether a bundled binary is present.
//
// It is false in a checkout that has not fetched the binaries yet, so
// that the application can say so instead of failing to start.
func HasBundledBinary() bool {
	_, err := bundledAssets.ReadFile(assetDir() + "/" + assetName())
	return err == nil
}

// BundledBinaryPath copies the bundled binary into the cache directory
// and returns the path of the copy.
//
// The copy exists because the directory the application was installed
// in is not always writable: on Windows the binary is under
// "Program Files", and on macOS it is inside the application bundle.
// The cache directory belongs to the user, so the copy can be given
// the executable bit and run from there.
//
// The copy is only made when it is missing or when the bundled binary
// is newer, so the start-up cost is paid once.
func BundledBinaryPath(cacheDir string) (string, error) {
	name := assetName()

	source, err := bundledAssets.ReadFile(assetDir() + "/" + name)
	if err != nil {
		return "", fmt.Errorf(
			"aria2: the bundled binary for %s is missing: %w "+
				"(run the task that fetches it, or choose an external binary in the settings)",
			runtime.GOOS+"-"+runtime.GOARCH, err)
	}

	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", fmt.Errorf("aria2: create the binary cache: %w", err)
	}

	target := filepath.Join(cacheDir, name)

	// The copy is redone when it is missing or when it differs from
	// the bundled one.
	if existing, err := os.ReadFile(target); err == nil && len(existing) == len(source) {
		return target, nil
	}

	if err := os.WriteFile(target, source, 0o700); err != nil {
		return "", fmt.Errorf("aria2: write the bundled binary: %w", err)
	}

	return target, nil
}

// ResolveBinary works out which binary to run.
//
// The choice is the user's: the bundled one, or a system-installed
// aria2-next. The external path wins when it is set, because it is the
// more specific instruction.
//
// The external binary is checked before it is used, so that a wrong
// path is reported as a clear message instead of a failed start.
func ResolveBinary(externalPath, cacheDir string) (string, error) {
	if externalPath != "" {
		info, err := os.Stat(externalPath)
		if err != nil {
			return "", fmt.Errorf("aria2: the external binary %q cannot be used: %w", externalPath, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("aria2: the external binary %q is a directory", externalPath)
		}
		if info.Mode()&0o111 == 0 {
			return "", fmt.Errorf("aria2: the external binary %q is not executable", externalPath)
		}
		return externalPath, nil
	}

	return BundledBinaryPath(cacheDir)
}

// bundledAssetNames lists the bundled assets, for the tests.
func bundledAssetNames() []string {
	var names []string
	_ = fs.WalkDir(bundledAssets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	return names
}
