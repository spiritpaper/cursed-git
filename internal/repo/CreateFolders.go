// internal/repo/CreateFolders.go
package repo

import (
	"fmt"
	"os"
	"path/filepath"
)

// creat folder sructure peoperly creates the folders and subfolders for this
func CreateFolderStructure(basePath string, paths []string) error {
	for _, subdir := range paths {
		fullpath := filepath.Join(basePath, subdir)
		err := os.MkdirAll(fullpath, 0755)
		if err != nil {
			return fmt.Errorf("create directory %q: %w", fullpath, err)
		}
	}

	return nil
}
