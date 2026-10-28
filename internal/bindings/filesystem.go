package bindings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The file system commands.
//
// The frontend asks the backend about the disk, because the webview has
// no access to it. The commands are the ones Rayburst's backend offers,
// so that the ported components work unchanged.

// FileDeletionMode says how a file is removed.
//
// "trash" moves it to the trash of the operating system, where the
// user can take it back, and "permanent" removes it for good.
type FileDeletionMode string

const (
	// DeletionTrash moves the file to the trash.
	DeletionTrash FileDeletionMode = "trash"
	// DeletionPermanent removes the file for good.
	DeletionPermanent FileDeletionMode = "permanent"
)

// PathIsDirectory answers whether a path is a directory.
//
// A path that does not exist is not a directory.
func (s *AppService) PathIsDirectory(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// DeletePath removes a file or a directory.
//
// It answers whether the path was there to begin with: a path that was
// already gone is not a failure, because the outcome the caller wanted
// has been reached.
func (s *AppService) DeletePath(path string, mode FileDeletionMode) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("the path could not be read: %w", err)
	}

	if mode == DeletionTrash {
		if err := moveToTrash(path); err != nil {
			return false, err
		}
		return true, nil
	}

	// The permanent removal of a directory takes everything in it.
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return false, fmt.Errorf("the folder could not be removed: %w", err)
		}
		return true, nil
	}

	if err := os.Remove(path); err != nil {
		return false, fmt.Errorf("the file could not be removed: %w", err)
	}
	return true, nil
}

// DirectoryExists answers whether a directory is there.
//
// It is the check the file dialog does before it offers a folder.
func (s *AppService) DirectoryExists(path string) bool {
	return s.PathIsDirectory(path)
}

// ListDirectoryFiles lists the files of a directory.
//
// The names are the entries of the directory, without the path. The
// folders come first, so that the list reads the way a file dialog
// shows it.
func (s *AppService) ListDirectoryFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("the folder could not be read: %w", err)
	}

	folders := []string{}
	files := []string{}

	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		} else {
			files = append(files, entry.Name())
		}
	}

	return append(folders, files...), nil
}

// RenameCompletedFile renames the file of a finished download.
//
// The download must be finished and hold a single file, and the file
// must still be on the disk. The record of the download is updated to
// the new name.
func (s *AppService) RenameCompletedFile(gid, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("enter a filename")
	}
	if err := validateFilename(name); err != nil {
		return err
	}

	job, err := s.downloads.Get(gid)
	if err != nil {
		return err
	}

	if job.Status != "complete" {
		return fmt.Errorf("finish the download before renaming its file")
	}

	files, err := s.downloads.Files(context.Background(), gid)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return fmt.Errorf("only a completed single-file download can be renamed")
	}

	source := files[0].Path
	if !filepath.IsAbs(source) {
		return fmt.Errorf("only a completed regular file can be renamed")
	}

	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("the file could not be read: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("only a completed regular file can be renamed")
	}

	target := filepath.Join(filepath.Dir(source), name)
	if target == source {
		return nil
	}

	if err := os.Rename(source, target); err != nil {
		return fmt.Errorf("the file could not be renamed: %w", err)
	}

	// The record holds the new name, so that the list shows it.
	if err := s.downloads.Rename(gid, name, target); err != nil {
		return err
	}

	return nil
}

// validateFilename answers whether a name may be used for a file.
//
// The name may not hold a separator, and it may not be one of the names
// the operating system keeps for itself.
func validateFilename(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("enter a filename")
	}

	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("a filename may not hold a path separator")
	}

	if name == "." || name == ".." {
		return fmt.Errorf("a filename may not be %q", name)
	}

	// The names the operating system keeps for itself.
	switch strings.ToUpper(name) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("%q is a name the system keeps for itself", name)
	}

	return nil
}
