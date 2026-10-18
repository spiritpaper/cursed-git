// internal/commands/Reset.go
package commands

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/refs"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// Reset moves the current branch to rev:
//
//	soft:  only the branch moves; index and files stay (everything looks staged)
//	mixed: the index is reset too; your files stay (changes look unstaged)   [default]
//	hard:  index AND files are reset; uncommitted changes to tracked files are gone
//
// Untracked files are never touched.
func Reset(mode, rev string, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	workTree := filepath.Dir(gitDir)
	if rev == "" {
		rev = "HEAD"
	}
	_, oldHead := headName(gitDir)
	target, err := RevParse(gitDir, rev)
	if errors.Is(err, refs.ErrNotFound) || (err != nil && rev == "HEAD" && oldHead == "") {
		target = "" // reset on a repo with no commits: just empty the index
	} else if err != nil {
		return err
	}
	if target != "" {
		if _, err := ReadCommit(gitDir, target); err != nil {
			return err
		}
		if err := refs.Update(gitDir, "HEAD", target, "", who(gitDir), "reset: moving to "+rev); err != nil {
			return err
		}
	}
	if mode == "soft" {
		return nil
	}

	T, err := commitFiles(gitDir, target)
	if err != nil {
		return err
	}
	old, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	oldEntries := map[string]index.Entry{}
	for _, e := range old.Entries {
		if e.Stage() == 0 {
			oldEntries[e.Path] = e
		}
	}
	if mode == "hard" {
		// tracked files that the target doesn't have go away; the rest is written if different
		for p := range oldEntries {
			if _, keep := T[p]; !keep {
				removeFile(workTree, p)
			}
		}
		for p, ent := range T {
			if !sameContent(workTree, p, ent) || !modeMatches(workTree, p, ent) {
				if err := writeFile(gitDir, workTree, p, ent); err != nil {
					return err
				}
			}
		}
	}
	idx := &index.Index{}
	for p, ent := range T {
		if e, ok := oldEntries[p]; ok && sameEntry(ent, e) && mode != "hard" {
			idx.Entries = append(idx.Entries, e) // unchanged: keep its stat data
			continue
		}
		idx.Entries = append(idx.Entries, refresh(workTree, treeEntry(p, ent)))
	}
	if err := idx.Write(gitDir); err != nil {
		return err
	}
	if mode == "hard" {
		if target != "" {
			c, _ := ReadCommit(gitDir, target)
			fmt.Fprintf(stdout, "HEAD is now at %s %s\n", target[:7], subject(c.Message))
		}
		return nil
	}
	return printUnstaged(stdout)
}

func modeMatches(workTree, path string, ent object.TreeEntry) bool {
	e, err := index.FromFile(filepath.Join(workTree, filepath.FromSlash(path)), path)
	return err == nil && e.Mode == parseMode(ent.Mode)
}

// printUnstaged is git's "Unstaged changes after reset:" list.
func printUnstaged(stdout io.Writer) error {
	st, err := Status()
	if err != nil {
		return err
	}
	if len(st.Unstaged) == 0 {
		return nil
	}
	fmt.Fprintln(stdout, "Unstaged changes after reset:")
	for _, c := range st.Unstaged {
		k := c.Kind
		if k == 'T' {
			k = 'M'
		}
		fmt.Fprintf(stdout, "%c\t%s\n", k, c.Path)
	}
	return nil
}

// matchPaths turns command-line paths into a matcher on repo paths ("." and folders match
// everything under them).
func matchPaths(workTree string, paths []string) (func(string) bool, error) {
	var rels []string
	for _, p := range paths {
		r, err := repoPath(workTree, p)
		if err != nil {
			return nil, err
		}
		rels = append(rels, r)
	}
	return func(path string) bool {
		for _, r := range rels {
			if r == "" || path == r || strings.HasPrefix(path, r+"/") {
				return true
			}
		}
		return false
	}, nil
}

// ResetPaths unstages: the index entries for paths go back to how they are in rev (default HEAD).
func ResetPaths(rev string, paths []string, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	return setIndexFrom(gitDir, rev, paths, stdout, true)
}

func setIndexFrom(gitDir, rev string, paths []string, stdout io.Writer, report bool) error {
	workTree := filepath.Dir(gitDir)
	match, err := matchPaths(workTree, paths)
	if err != nil {
		return err
	}
	src := ""
	if rev == "" {
		rev = "HEAD"
	}
	if h, err := RevParse(gitDir, rev); err == nil {
		src = h
	} else if rev != "HEAD" {
		return err
	}
	T, err := commitFiles(gitDir, src)
	if err != nil {
		return err
	}
	idx, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	matched := false
	out := &index.Index{}
	for _, e := range idx.Entries {
		if match(e.Path) {
			matched = true
			continue // replaced below from the source tree
		}
		out.Entries = append(out.Entries, e)
	}
	old := map[string]index.Entry{}
	for _, e := range idx.Entries {
		old[e.Path] = e
	}
	for p, ent := range T {
		if !match(p) {
			continue
		}
		matched = true
		if e, ok := old[p]; ok && sameEntry(ent, e) && e.Stage() == 0 {
			out.Entries = append(out.Entries, e)
		} else {
			out.Entries = append(out.Entries, refresh(workTree, treeEntry(p, ent)))
		}
	}
	if !matched {
		return fmt.Errorf("pathspec '%s' did not match any file(s) known to git", strings.Join(paths, " "))
	}
	if err := out.Write(gitDir); err != nil {
		return err
	}
	if report {
		return printUnstaged(stdout)
	}
	return nil
}

// Restore puts files back:
//
//	worktree only (default): files <- index (throw away unstaged changes)
//	staged only:             index <- source (default HEAD)  = unstage
//	both:                    index and files <- source (default HEAD)
func Restore(paths []string, staged, worktree bool, source string) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	workTree := filepath.Dir(gitDir)
	if !staged && !worktree {
		worktree = true
	}
	if staged {
		if err := setIndexFrom(gitDir, source, paths, io.Discard, false); err != nil {
			return err
		}
	}
	if !worktree {
		return nil
	}
	match, err := matchPaths(workTree, paths)
	if err != nil {
		return err
	}
	files := map[string]object.TreeEntry{}
	if source != "" && !staged {
		h, err := RevParse(gitDir, source)
		if err != nil {
			return err
		}
		if files, err = commitFiles(gitDir, h); err != nil {
			return err
		}
	} else {
		idx, err := index.Read(gitDir)
		if err != nil {
			return err
		}
		for _, e := range idx.Entries {
			if e.Stage() == 0 {
				files[e.Path] = object.TreeEntry{Mode: fmt.Sprintf("%o", e.Mode), Name: e.Path, Hash: e.HashHex()}
			}
		}
	}
	var todo []string
	for p := range files {
		if match(p) {
			todo = append(todo, p)
		}
	}
	// restoring from a commit makes the files match it: tracked files it doesn't have are
	// removed (untracked files are never touched)
	if source != "" && !staged {
		idx, err := index.Read(gitDir)
		if err != nil {
			return err
		}
		for _, e := range idx.Entries {
			if _, inSource := files[e.Path]; !inSource && match(e.Path) {
				if err := removeFile(workTree, e.Path); err != nil {
					return err
				}
				todo = append(todo, "") // it matched something
			}
		}
	}
	if len(todo) == 0 {
		return fmt.Errorf("pathspec '%s' did not match any file(s) known to git", strings.Join(paths, " "))
	}
	sort.Strings(todo)
	for _, p := range todo {
		if p == "" {
			continue
		}
		if sameContent(workTree, p, files[p]) && modeMatches(workTree, p, files[p]) {
			continue
		}
		if err := writeFile(gitDir, workTree, p, files[p]); err != nil {
			return err
		}
	}
	return nil
}
