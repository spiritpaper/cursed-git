// internal/repo/Find.go
package repo

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNotRepo means no .git folder was found in this folder or any parent.
var ErrNotRepo = errors.New("not a git repository (or any of the parent directories): .git")

// Find walks up from start until it finds a folder containing .git,
// the same way real git does when you run it from a subfolder.
// It returns the absolute path of the .git folder.
func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return gitDir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached the root of the filesystem
			return "", ErrNotRepo
		}
		dir = parent
	}
}
