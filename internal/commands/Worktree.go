// internal/commands/Worktree.go
//
// Moving the working tree and index from one commit to another (switch, checkout, reset --hard)
// without ever losing uncommitted work. The rule, same as git's:
//
//	for every file that is DIFFERENT between the two commits:
//	  - if you have local changes to it (staged or not), refuse
//	  - if an untracked file is in the way, refuse
//	  - otherwise, replace it with the target version (or delete it)
//	files that are the SAME in both commits keep whatever local changes you have
//
// Nothing is touched until every file has been checked, so a refusal changes nothing.
package commands

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

// commitFiles lists every file in a commit's tree ("" = no commit = no files).
func commitFiles(gitDir, commit string) (map[string]object.TreeEntry, error) {
	if commit == "" {
		return map[string]object.TreeEntry{}, nil
	}
	c, err := ReadCommit(gitDir, commit)
	if err != nil {
		return nil, err
	}
	return FlattenTree(gitDir, c.Tree)
}

func parseMode(m string) uint32 {
	v, _ := strconv.ParseUint(m, 8, 32)
	return uint32(v)
}

// writeFile puts a blob on disk with the right mode (or as a symlink).
func writeFile(gitDir, workTree, path string, ent object.TreeEntry) error {
	_, content, err := object.Read(gitDir, ent.Hash)
	if err != nil {
		return err
	}
	full := filepath.Join(workTree, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	os.Remove(full)
	switch ent.Mode {
	case "120000":
		return os.Symlink(string(content), full)
	case "100755":
		return os.WriteFile(full, content, 0777) // like git: 0777/0666 and your umask decides
	default:
		return os.WriteFile(full, content, 0666)
	}
}

// removeFile deletes a file and any folders that only existed for it.
func removeFile(workTree, path string) error {
	full := filepath.Join(workTree, filepath.FromSlash(path))
	if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for dir := filepath.Dir(full); dir != workTree && strings.HasPrefix(dir, workTree); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
	return nil
}

// freshEntry builds an index entry for a file that was just written from ent.
func freshEntry(workTree, path string, ent object.TreeEntry) (index.Entry, error) {
	e, err := index.FromFile(filepath.Join(workTree, filepath.FromSlash(path)), path)
	if err != nil {
		return e, err
	}
	hex.Decode(e.Hash[:], []byte(ent.Hash))
	return e, nil
}

// treeEntry turns a tree entry into an index entry without touching the disk (stat data left
// empty, so status will hash the file once to check it).
func treeEntry(path string, ent object.TreeEntry) index.Entry {
	e := index.Entry{Path: path, Mode: parseMode(ent.Mode)}
	hex.Decode(e.Hash[:], []byte(ent.Hash))
	return e
}

// refresh fills in stat data for an entry if the file on disk really has that content.
func refresh(workTree string, e index.Entry) index.Entry {
	now, err := index.FromFile(filepath.Join(workTree, filepath.FromSlash(e.Path)), e.Path)
	if err != nil || now.Mode != e.Mode {
		return e
	}
	var content []byte
	full := filepath.Join(workTree, filepath.FromSlash(e.Path))
	if now.Mode == 0o120000 {
		t, err := os.Readlink(full)
		if err != nil {
			return e
		}
		content = []byte(t)
	} else if content, err = os.ReadFile(full); err != nil {
		return e
	}
	if object.Hash(object.Blob, content) != e.HashHex() {
		return e
	}
	now.Hash, now.Flags, now.Flags2 = e.Hash, e.Flags, e.Flags2
	return now
}

func sameEntry(a object.TreeEntry, e index.Entry) bool {
	return a.Hash == e.HashHex() && parseMode(a.Mode) == e.Mode
}

// ErrUnmerged: switching in the middle of a merge conflict would lose the conflict.
var ErrUnmerged = errors.New("you need to resolve your current index first")

// WouldOverwrite is git's "your local changes would be overwritten" refusal.
type WouldOverwrite struct {
	Verb      string // "checkout" or "merge"
	Changed   []string
	Untracked []string
}

func (w *WouldOverwrite) Error() string {
	var b strings.Builder
	if len(w.Changed) > 0 {
		fmt.Fprintf(&b, "Your local changes to the following files would be overwritten by %s:\n", w.Verb)
		for _, p := range w.Changed {
			fmt.Fprintf(&b, "\t%s\n", p)
		}
		fmt.Fprintf(&b, "Please commit your changes or stash them before you switch branches.\n")
	}
	if len(w.Untracked) > 0 {
		fmt.Fprintf(&b, "The following untracked working tree files would be overwritten by %s:\n", w.Verb)
		for _, p := range w.Untracked {
			fmt.Fprintf(&b, "\t%s\n", p)
		}
		fmt.Fprintf(&b, "Please move or remove them before you switch branches.\n")
	}
	b.WriteString("Aborting")
	return b.String()
}

// moveTo makes the index and working tree go from commit `from` to commit `to`, carrying
// local changes over, or refuses (changing nothing) if that would lose work.
func moveTo(gitDir, from, to string) error {
	workTree := filepath.Dir(gitDir)
	H, err := commitFiles(gitDir, from)
	if err != nil {
		return err
	}
	T, err := commitFiles(gitDir, to)
	if err != nil {
		return err
	}
	idx, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	I := map[string]index.Entry{}
	for _, e := range idx.Entries {
		if e.Stage() != 0 {
			return ErrUnmerged
		}
		I[e.Path] = e
	}

	paths := map[string]bool{}
	for p := range H {
		paths[p] = true
	}
	for p := range T {
		paths[p] = true
	}
	var writes, removes []string
	refuse := &WouldOverwrite{Verb: "checkout"}
	for p := range paths {
		h, inH := H[p]
		t, inT := T[p]
		if inH == inT && (!inH || (h.Hash == t.Hash && h.Mode == t.Mode)) {
			continue // the same in both commits: local changes stay
		}
		i, inI := I[p]
		diskClean := true
		if inI {
			kind, err := diskChange(gitDir, workTree, i, nil)
			diskClean = err == nil && kind == 0
		} else {
			_, err := os.Lstat(filepath.Join(workTree, filepath.FromSlash(p)))
			diskClean = errors.Is(err, fs.ErrNotExist)
		}
		indexClean := inI == inH && (!inI || sameEntry(h, i))
		switch {
		case indexClean && diskClean:
			if inT {
				writes = append(writes, p)
			} else {
				removes = append(removes, p)
			}
		case inI && inT && sameEntry(t, i) && diskClean:
			// you already staged exactly the target version: nothing to do
		case !inH && !inI && inT:
			// an untracked file is where the target wants one: git refuses, even if the content
			// happens to be identical (it's not tracked, so it's not git's to overwrite)
			refuse.Untracked = append(refuse.Untracked, p)
		default:
			refuse.Changed = append(refuse.Changed, p)
		}
	}
	// a new file needs its folders: an untracked FILE where a folder must go is in the way
	removing := map[string]bool{}
	for _, p := range removes {
		removing[p] = true
	}
	for _, p := range writes {
		for dir := filepath.ToSlash(filepath.Dir(p)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			info, err := os.Lstat(filepath.Join(workTree, filepath.FromSlash(dir)))
			if err == nil && !info.IsDir() && !removing[dir] {
				refuse.Untracked = append(refuse.Untracked, dir)
			}
		}
	}
	if len(refuse.Changed) > 0 || len(refuse.Untracked) > 0 {
		sort.Strings(refuse.Changed)
		sort.Strings(refuse.Untracked)
		return refuse
	}

	// checks passed: now change things. Removals first, so a file can become a folder.
	sort.Strings(removes)
	sort.Strings(writes)
	for _, p := range removes {
		if err := removeFile(workTree, p); err != nil {
			return err
		}
		delete(I, p)
	}
	for _, p := range writes {
		if err := writeFile(gitDir, workTree, p, T[p]); err != nil {
			return err
		}
		e, err := freshEntry(workTree, p, T[p])
		if err != nil {
			return err
		}
		I[p] = e
	}
	out := &index.Index{}
	for _, e := range I {
		out.Entries = append(out.Entries, e)
	}
	return out.Write(gitDir)
}

func sameContent(workTree, path string, ent object.TreeEntry) bool {
	data, err := os.ReadFile(filepath.Join(workTree, filepath.FromSlash(path)))
	return err == nil && object.Hash(object.Blob, data) == ent.Hash
}

// who is the reflog identity for ref moves that aren't commits.
func who(gitDir string) refs.Who {
	if _, c, err := Identity(gitDir); err == nil {
		return refs.Who(c.String())
	}
	return refs.Who(object.Now("cg", "cg@cursed.git").String())
}

// headName is how git names where HEAD is in reflog messages: the branch, or the hash.
func headName(gitDir string) (name, hash string) {
	hash, _ = refs.Resolve(gitDir, "HEAD")
	ref, _ := refs.Follow(gitDir, "HEAD")
	if b, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return b, hash
	}
	return hash, hash
}
