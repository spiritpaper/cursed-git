// internal/commands/Add.go
package commands

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cursedgit.com/internal/ignore"
	"cursedgit.com/internal/index"
	"cursedgit.com/internal/object"
	"cursedgit.com/internal/repo"
)

// IgnoredError lists explicitly named paths that .gitignore excludes (git refuses those without -f).
type IgnoredError struct{ Paths []string }

func (e *IgnoredError) Error() string {
	return "The following paths are ignored by one of your .gitignore files:\n" + strings.Join(e.Paths, "\n") +
		"\nhint: Use -f if you really want to add them."
}

// Add stages files: new and changed files are hashed into blobs and put in the index,
// and files that were deleted from disk are removed from the index.
// Folders are walked recursively, skipping .git and anything .gitignore excludes.
func Add(paths []string, force bool) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	workTree := filepath.Dir(gitDir)
	idx, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	ign := ignore.New(workTree)

	var ignored []string
	for _, p := range paths {
		rel, err := repoPath(workTree, p)
		if err != nil {
			return err
		}
		info, err := os.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// deleted from disk: stage the deletion (for a folder, everything that was under it)
			if !removeUnder(idx, rel, func(string) bool { return true }) {
				return fmt.Errorf("pathspec '%s' did not match any files", p)
			}
		case err != nil:
			return err
		case info.IsDir():
			seen := map[string]bool{}
			if err := addDir(gitDir, workTree, rel, idx, ign, force, seen); err != nil {
				return err
			}
			// anything in the index under this folder that we didn't see on disk was deleted
			removeUnder(idx, rel, func(path string) bool { return !seen[path] })
		default:
			if !force && ign.Ignored(rel, false) {
				ignored = append(ignored, rel)
				continue
			}
			if err := addFile(gitDir, workTree, rel, idx); err != nil {
				return err
			}
		}
	}
	if err := idx.Write(gitDir); err != nil {
		return err
	}
	if len(ignored) > 0 {
		return &IgnoredError{Paths: ignored}
	}
	return nil
}

// repoPath turns a command-line path into a slash-separated path relative to the repo root.
func repoPath(workTree, p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(workTree, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("'%s' is outside repository at '%s'", p, workTree)
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel), nil
}

func addDir(gitDir, workTree, rel string, idx *index.Index, ign *ignore.Matcher, force bool, seen map[string]bool) error {
	root := filepath.Join(workTree, filepath.FromSlash(rel))
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		r, _ := filepath.Rel(workTree, p)
		r = filepath.ToSlash(r)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if r != "." && r != rel && !force && ign.Ignored(r, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 {
			return nil // sockets, devices, ...: git skips them too
		}
		if !force && ign.Ignored(r, false) {
			return nil
		}
		seen[r] = true
		return addFile(gitDir, workTree, r, idx)
	})
}

func addFile(gitDir, workTree, rel string, idx *index.Index) error {
	full := filepath.Join(workTree, filepath.FromSlash(rel))
	e, err := index.FromFile(full, rel)
	if err != nil {
		return err
	}
	var content []byte
	if e.Mode == 0o120000 {
		target, err := os.Readlink(full) // a symlink's blob is the path it points to
		if err != nil {
			return err
		}
		content = []byte(target)
	} else if content, err = os.ReadFile(full); err != nil {
		return err
	}
	hash, err := object.Write(gitDir, object.Blob, content)
	if err != nil {
		return err
	}
	if _, err := hex.Decode(e.Hash[:], []byte(hash)); err != nil {
		return err
	}
	idx.Set(e)
	return nil
}

// removeUnder drops index entries at rel or inside folder rel ("" = whole repo) when drop(path) is true.
func removeUnder(idx *index.Index, rel string, drop func(string) bool) bool {
	removed := false
	kept := idx.Entries[:0]
	for _, e := range idx.Entries {
		inside := rel == "" || e.Path == rel || strings.HasPrefix(e.Path, rel+"/")
		if inside && drop(e.Path) {
			removed = true
			continue
		}
		kept = append(kept, e)
	}
	idx.Entries = kept
	return removed
}
