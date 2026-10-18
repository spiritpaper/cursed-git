// internal/commands/Diff.go
package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spiritpaper/cursed-git/internal/diff"
	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// version is one side of a file in a diff: what it was, or what it is now.
type version struct {
	mode, hash string
	content    []byte
}

type side map[string]version

type DiffOptions struct {
	Cached bool     // index vs a commit (default HEAD) instead of files vs index
	Revs   []string // 0, 1 or 2 commits
}

// Diff prints a unified diff, like `git diff`:
//
//	cg diff              files on disk vs the index (what you haven't staged)
//	cg diff --cached     the index vs HEAD (what you're about to commit)
//	cg diff <commit>     files on disk vs that commit
//	cg diff <a> <b>      commit a vs commit b
func Diff(o DiffOptions, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	workTree := filepath.Dir(gitDir)
	idx, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	var old, new side
	switch {
	case len(o.Revs) == 2:
		if old, err = commitSide(gitDir, o.Revs[0]); err == nil {
			new, err = commitSide(gitDir, o.Revs[1])
		}
	case o.Cached:
		rev := "HEAD"
		if len(o.Revs) == 1 {
			rev = o.Revs[0]
		}
		if old, err = commitSide(gitDir, rev); err == nil {
			new, err = indexSide(gitDir, idx)
		}
	case len(o.Revs) == 1:
		if old, err = commitSide(gitDir, o.Revs[0]); err == nil {
			paths := map[string]bool{}
			for p := range old {
				paths[p] = true
			}
			for _, e := range idx.Entries {
				paths[e.Path] = true
			}
			new, err = diskSide(workTree, paths)
		}
	default:
		// files vs index: only look at files whose stat data says they changed
		if old, err = indexSide(gitDir, idx); err != nil {
			return err
		}
		changed := map[string]bool{}
		for _, e := range idx.Entries {
			if e.Stage() != 0 {
				fmt.Fprintf(stdout, "* Unmerged path %s\n", e.Path)
				delete(old, e.Path)
				continue
			}
			if kind, err := diskChange(gitDir, workTree, e, nil); err == nil && kind != 0 {
				changed[e.Path] = true
			} else {
				delete(old, e.Path)
			}
		}
		new, err = diskSide(workTree, changed)
	}
	if err != nil {
		return err
	}

	paths := map[string]bool{}
	for p := range old {
		paths[p] = true
	}
	for p := range new {
		paths[p] = true
	}
	var sorted []string
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	for _, p := range sorted {
		a, inA := old[p]
		b, inB := new[p]
		if inA && inB && a.hash == b.hash && a.mode == b.mode {
			continue
		}
		var pa, pb *version
		if inA {
			pa = &a
		}
		if inB {
			pb = &b
		}
		if pa != nil && pb != nil && (pa.mode == "120000") != (pb.mode == "120000") {
			// a file became a symlink (or back): git shows that as a deletion plus a new file
			if err := fileDiff(gitDir, stdout, p, pa, nil); err != nil {
				return err
			}
			pa = nil
		}
		if err := fileDiff(gitDir, stdout, p, pa, pb); err != nil {
			return err
		}
	}
	return nil
}

func commitSide(gitDir, rev string) (side, error) {
	out := side{}
	hash, err := RevParse(gitDir, rev)
	if err != nil {
		if rev == "HEAD" {
			return out, nil // no commits yet: everything is new
		}
		return nil, err
	}
	files, err := commitFiles(gitDir, hash)
	if err != nil {
		return nil, err
	}
	for p, e := range files {
		out[p] = version{mode: e.Mode, hash: e.Hash}
	}
	return out, nil
}

func indexSide(gitDir string, idx *index.Index) (side, error) {
	out := side{}
	for _, e := range idx.Entries {
		if e.Stage() == 0 {
			out[e.Path] = version{mode: fmt.Sprintf("%o", e.Mode), hash: e.HashHex()}
		}
	}
	return out, nil
}

// diskSide reads the given paths from the working tree (missing files are left out).
func diskSide(workTree string, paths map[string]bool) (side, error) {
	out := side{}
	for p := range paths {
		full := filepath.Join(workTree, filepath.FromSlash(p))
		e, err := index.FromFile(full, p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			continue // a folder where a file was: git shows it as deleted
		}
		var content []byte
		if e.Mode == 0o120000 {
			t, err := os.Readlink(full)
			if err != nil {
				return nil, err
			}
			content = []byte(t)
		} else if content, err = os.ReadFile(full); err != nil {
			return nil, err
		}
		out[p] = version{mode: fmt.Sprintf("%o", e.Mode), hash: object.Hash(object.Blob, content), content: content}
	}
	return out, nil
}

func (v *version) load(gitDir string) ([]byte, error) {
	if v.content != nil || v.hash == object.Hash(object.Blob, nil) {
		return v.content, nil
	}
	_, data, err := object.Read(gitDir, v.hash)
	v.content = data
	return data, err
}

// binary: git's test is a NUL byte in the first 8000 bytes.
func binary(b []byte) bool { return bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 }

func fileDiff(gitDir string, w io.Writer, path string, a, b *version) error {
	fmt.Fprintf(w, "diff --git a/%s b/%s\n", path, path)
	zero := "0000000"
	switch {
	case a == nil:
		fmt.Fprintf(w, "new file mode %s\n", b.mode)
	case b == nil:
		fmt.Fprintf(w, "deleted file mode %s\n", a.mode)
	case a.mode != b.mode:
		fmt.Fprintf(w, "old mode %s\nnew mode %s\n", a.mode, b.mode)
	}
	if a != nil && b != nil && a.hash == b.hash {
		return nil // only the mode changed
	}
	ah, bh := zero, zero
	var ac, bc []byte
	var err error
	if a != nil {
		ah = a.hash[:7]
		if ac, err = a.load(gitDir); err != nil {
			return err
		}
	}
	if b != nil {
		bh = b.hash[:7]
		if bc, err = b.load(gitDir); err != nil {
			return err
		}
	}
	if a != nil && b != nil && a.mode == b.mode {
		fmt.Fprintf(w, "index %s..%s %s\n", ah, bh, a.mode)
	} else {
		fmt.Fprintf(w, "index %s..%s\n", ah, bh)
	}
	from, to := "a/"+path, "b/"+path
	if a == nil {
		from = "/dev/null"
	}
	if b == nil {
		to = "/dev/null"
	}
	if binary(ac) || binary(bc) {
		fmt.Fprintf(w, "Binary files %s and %s differ\n", from, to)
		return nil
	}
	hunks := diff.Unified(diff.Split(ac), diff.Split(bc), 3)
	if hunks == "" {
		return nil // e.g. a new empty file
	}
	fmt.Fprintf(w, "--- %s\n+++ %s\n%s", from, to, hunks)
	return nil
}
