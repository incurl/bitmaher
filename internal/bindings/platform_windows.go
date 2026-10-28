//go:build windows

package bindings

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// freeSpaceOn returns the free space of the volume a path is on.
//
// The Windows API answers with the free space of the volume, in the
// clusters that are left.
func freeSpaceOn(path string) (int64, error) {
	var free uint64
	volume := filepath.VolumeName(path) + "\\"

	handle, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return 0, err
	}

	if err := windows.GetDiskFreeSpaceEx(handle, &free, nil, nil); err != nil {
		return 0, err
	}
	return int64(free), nil
}
