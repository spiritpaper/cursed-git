// internal/index/stat.go
package index

import (
	"fmt"
	"io/fs"
	"os"
)

// FromFile builds an index entry for a file on disk. The hash must be filled in by the caller.
// Directories and other special files are rejected: the index only holds files and symlinks.
func FromFile(path, repoPath string) (Entry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{Path: repoPath}
	switch m := info.Mode(); {
	case m.IsRegular():
		e.Mode = 0o100644
		if m.Perm()&0o100 != 0 { // git only looks at the owner's execute bit
			e.Mode = 0o100755
		}
	case m&fs.ModeSymlink != 0:
		e.Mode = 0o120000
	default:
		return Entry{}, fmt.Errorf("%s: not a regular file or symlink", path)
	}
	e.Size = uint32(info.Size())
	fillStat(&e, info)
	return e, nil
}
