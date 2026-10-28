//go:build !windows

package bindings

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// moveToTrash moves a file to the trash of the operating system.
//
// The desktop of a Linux system has no trash of its own that every
// application agrees on, so the freedesktop.org specification is
// followed: the file is moved into the "Trash" folder of the home
// directory, or of the disk it is on.
//
// The user can take the file back from there, which is the whole point
// of the trash: a download that was removed by mistake is not lost.
func moveToTrash(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("the path could not be read: %w", err)
	}

	trash, err := trashFolder(absolute)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(trash, 0o700); err != nil {
		return fmt.Errorf("the trash could not be created: %w", err)
	}

	// The trash holds a file of every name, so the name is made unique.
	target := uniquePath(trash, filepath.Base(absolute))

	if err := os.Rename(absolute, target); err != nil {
		return fmt.Errorf("the file could not be moved to the trash: %w", err)
	}

	// The information file tells the desktop when the file was removed
	// and where it came from.
	info := fmt.Sprintf("[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		absolute, time.Now().Format("2006-01-02T15:04:05"))

	if err := os.WriteFile(target+".trashinfo", []byte(info), 0o600); err != nil {
		// The file is in the trash, which is what the caller wanted.
		// The information file is a convenience for the desktop.
		return nil
	}

	return nil
}

// trashFolder returns the trash folder of the disk a path is on.
//
// The specification says that a trash lives on the disk of the file it
// holds, so that moving a file into it does not copy it across disks.
// The home directory holds the trash of the disk it is on, and every
// other disk holds a trash of its own at its root.
func trashFolder(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("the home directory is not available: %w", err)
	}

	// The home directory holds the trash of the disk it is on.
	if sameDevice(path, home) {
		return filepath.Join(home, ".local", "share", "Trash", "files"), nil
	}

	// The trash of another disk lives at the root of that disk.
	mount, err := mountPoint(path)
	if err != nil {
		return "", err
	}

	return filepath.Join(mount, ".Trash-"+strconv.Itoa(os.Getuid()), "files"), nil
}

// sameDevice reports whether two paths are on the same disk.
//
// A move between two disks is a copy, and the specification says that
// the trash holds a move.
func sameDevice(left, right string) bool {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false
	}
	return leftInfo.Sys().(*syscall.Stat_t).Dev == rightInfo.Sys().(*syscall.Stat_t).Dev
}

// mountPoint returns the directory a path is mounted at.
//
// The mount point is the directory whose own parent is on another disk:
// that is where the file system of the path begins.
func mountPoint(path string) (string, error) {
	directory := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		directory = filepath.Dir(path)
	}

	for {
		parent := filepath.Dir(directory)
		if parent == directory {
			// The root is the mount point of the root disk.
			return string(filepath.Separator), nil
		}

		if !sameDevice(directory, parent) {
			return directory, nil
		}

		directory = parent
	}
}

// uniquePath returns a path in a folder that nothing holds yet.
//
// The trash of a file that was downloaded twice holds both, so the
// name is made unique with a number.
func uniquePath(folder, name string) string {
	candidate := filepath.Join(folder, name)
	if _, err := os.Lstat(candidate); os.IsNotExist(err) {
		return candidate
	}

	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)

	for i := 1; i < 1000; i++ {
		candidate = filepath.Join(folder, fmt.Sprintf("%s.%d%s", stem, i, extension))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}

	return filepath.Join(folder, fmt.Sprintf("%s.%d%s", stem, time.Now().UnixNano(), extension))
}
