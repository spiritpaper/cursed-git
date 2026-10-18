// internal/curses/graveyard.go
//
// Ground rule #2: nothing is truly deleted. Before she hides or wipes anything, the files
// are saved as a real commit (tree + blobs) pointed at by refs/cursed/graveyard/<time>.
// Refs keep objects alive, so git never garbage-collects them, and `cg beg` (or plain
// `git checkout refs/cursed/graveyard/<time> -- <file>`) brings them back.
package curses

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cursedgit.com/internal/commands"
	"cursedgit.com/internal/ignore"
	"cursedgit.com/internal/index"
	"cursedgit.com/internal/object"
	"cursedgit.com/internal/refs"
)

// changedFiles lists what she could take: modified tracked files and untracked files
// (untracked folders expanded, ignored files skipped). Staged content is never touched.
func changedFiles(workTree string) ([]string, error) {
	st, err := commands.Status()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, c := range st.Unstaged {
		if c.Kind != 'D' {
			out = append(out, c.Path)
		}
	}
	ign := ignore.New(workTree)
	for _, u := range st.Untracked {
		if !strings.HasSuffix(u, "/") {
			out = append(out, u)
			continue
		}
		root := filepath.Join(workTree, filepath.FromSlash(strings.TrimSuffix(u, "/")))
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(workTree, p)
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == ".git" || ign.Ignored(rel, true) {
					return filepath.SkipDir
				}
				return nil
			}
			if !ign.Ignored(rel, false) {
				out = append(out, rel)
			}
			return nil
		})
	}
	sort.Strings(out)
	return out, nil
}

// Bury saves files (as they are on disk now) into a graveyard commit and returns its ref.
func Bury(gitDir, workTree string, paths []string, why string, now time.Time) (string, error) {
	var entries []index.Entry
	for _, p := range paths {
		full := filepath.Join(workTree, filepath.FromSlash(p))
		e, err := index.FromFile(full, p)
		if err != nil {
			return "", err
		}
		var content []byte
		if e.Mode == 0o120000 {
			target, err := os.Readlink(full)
			if err != nil {
				return "", err
			}
			content = []byte(target)
		} else if content, err = os.ReadFile(full); err != nil {
			return "", err
		}
		hash, err := object.Write(gitDir, object.Blob, content)
		if err != nil {
			return "", err
		}
		hex.Decode(e.Hash[:], []byte(hash))
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	tree, err := commands.WriteTreeFrom(gitDir, entries)
	if err != nil {
		return "", err
	}
	sig := object.Signature{Name: "cg", Email: "cg@cursed.git", When: now.Unix(), TZ: "+0000"}
	c := object.CommitData{Tree: tree, Author: sig, Committer: sig,
		Message: fmt.Sprintf("graveyard: %s\n\n%s\n", why, strings.Join(paths, "\n"))}
	if head, err := refs.Resolve(gitDir, "HEAD"); err == nil {
		c.Parents = []string{head}
	}
	hash, err := object.Write(gitDir, object.Commit, object.EncodeCommit(c))
	if err != nil {
		return "", err
	}
	stamp := now.UnixNano()
	for { // two burials in the same instant (a frozen test clock, a fast machine) get different graves
		if _, _, err := refs.Read(gitDir, fmt.Sprintf("refs/cursed/graveyard/%d", stamp)); err != nil {
			break
		}
		stamp++
	}
	ref := fmt.Sprintf("refs/cursed/graveyard/%d", stamp)
	return ref, refs.Update(gitDir, ref, hash, refs.Zero, refs.Who(sig.String()), "graveyard: "+why)
}

// putBack makes a file look like it does in the index again (or removes it if it's untracked).
func putBack(gitDir, workTree, path string) error {
	idx, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	full := filepath.Join(workTree, filepath.FromSlash(path))
	for _, e := range idx.Entries {
		if e.Path == path && e.Stage() == 0 {
			_, content, err := object.Read(gitDir, e.HashHex())
			if err != nil {
				return err
			}
			return writeFile(full, e.Mode, content)
		}
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// tidy up folders that only existed for this file
	for dir := filepath.Dir(full); dir != workTree && strings.HasPrefix(dir, workTree); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

func writeFile(full string, mode uint32, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	os.Remove(full)
	if mode == 0o120000 {
		return os.Symlink(string(content), full)
	}
	perm := os.FileMode(0666) // like git: your umask decides the final permissions
	if mode == 0o100755 {
		perm = 0777
	}
	return os.WriteFile(full, content, perm)
}

// Unbury writes a graveyard's files back to disk and returns their paths.
func Unbury(gitDir, workTree, ref string) ([]string, error) {
	hash, err := refs.Resolve(gitDir, ref)
	if err != nil {
		return nil, err
	}
	c, err := commands.ReadCommit(gitDir, hash)
	if err != nil {
		return nil, err
	}
	files, err := commands.FlattenTree(gitDir, c.Tree)
	if err != nil {
		return nil, err
	}
	var paths []string
	for p, e := range files {
		_, content, err := object.Read(gitDir, e.Hash)
		if err != nil {
			return nil, err
		}
		var mode uint32 = 0o100644
		fmt.Sscanf(e.Mode, "%o", &mode)
		if err := writeFile(filepath.Join(workTree, filepath.FromSlash(p)), mode, content); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}
