// internal/repo/RepoPath.go
package repo

import "path/filepath"

// can be used later to resolve absolute path
func ExteractGitRepo(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}
