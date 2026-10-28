//go:build !windows

package bindings

import "golang.org/x/sys/unix"

// freeSpaceOn returns the free space of the file system a path is on.
//
// The statfs system call answers with the block counts of the file
// system. The free space is what is left of the blocks that are not
// reserved.
func freeSpaceOn(path string) (int64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}

	// The blocks that are free are the ones the super user may still
	// use, so the reserved ones are left out.
	blockSize := int64(stat.Bsize)
	return int64(stat.Bavail) * blockSize, nil
}
