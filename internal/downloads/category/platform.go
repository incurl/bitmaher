package category

import (
	"os"
	"path/filepath"
	"runtime"
)

// The few things that differ between the platforms.
//
// The classification reads the disk and compares paths, and the rules
// of those differ a little: the separator of a path, the letter case
// of a path, and where the home directory of the user is.

// filepathToSlash returns a path with the separator of the platform.
//
// The classification compares paths and splits them into parts, so it
// works with one separator throughout.
func filepathToSlash(value string) string {
	return filepath.ToSlash(value)
}

// isWindows reports whether the application runs on Windows.
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// homeDirectory returns the home directory of the user.
//
// It is the directory a "~" in a path stands for.
func homeDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return home, nil
}
