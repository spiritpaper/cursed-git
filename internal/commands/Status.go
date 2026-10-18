// internal/commands/Status.go
package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cursedgit.com/internal/config"
	"cursedgit.com/internal/ignore"
	"cursedgit.com/internal/index"
	"cursedgit.com/internal/object"
	"cursedgit.com/internal/refs"
	"cursedgit.com/internal/repo"
)

// Change is one changed path. Kind is a git status letter: A added, M modified, D deleted, T type changed.
type Change struct {
	Kind byte
	Path string
}

type Upstream struct {
	Name          string // "origin/main"
	Gone          bool   // configured, but the remote branch doesn't exist (any more)
	Ahead, Behind int
}

// StatusReport compares three snapshots: the HEAD commit, the index, and the files on disk.
type StatusReport struct {
	Branch    string // "" when HEAD is detached
	Detached  string // short hash when detached
	Unborn    bool   // no commits yet
	Upstream  *Upstream
	Staged    []Change          // HEAD vs index ("Changes to be committed")
	Unstaged  []Change          // index vs disk ("Changes not staged for commit")
	Unmerged  []string          // paths with merge conflicts (index stages 1-3)
	Conflict  map[string]string // path -> git's two-letter code: UU both modified, AA both added, UD deleted by them, ...
	Merging   bool              // a merge is in progress (MERGE_HEAD exists)
	Untracked []string          // not in the index and not ignored; whole folders end in "/"
}

func Status() (*StatusReport, error) {
	gitDir, err := repo.Find(".")
	if err != nil {
		return nil, err
	}
	workTree := filepath.Dir(gitDir)
	r := &StatusReport{}

	// where are we?
	head, err := refs.Follow(gitDir, "HEAD")
	if err != nil {
		return nil, err
	}
	headHash, err := refs.Resolve(gitDir, "HEAD")
	switch {
	case errors.Is(err, refs.ErrNotFound):
		r.Unborn = true
	case err != nil:
		return nil, err
	}
	if head == "HEAD" {
		r.Detached = headHash[:7]
	} else {
		r.Branch = strings.TrimPrefix(head, "refs/heads/")
	}

	// 1. HEAD vs index
	headFiles := map[string]object.TreeEntry{}
	if !r.Unborn {
		c, err := ReadCommit(gitDir, headHash)
		if err != nil {
			return nil, err
		}
		if err := flattenTree(gitDir, c.Tree, "", headFiles); err != nil {
			return nil, err
		}
		if r.Branch != "" {
			r.Upstream = upstream(gitDir, r.Branch, headHash)
		}
	}
	idx, err := index.Read(gitDir)
	if err != nil {
		return nil, err
	}
	staged := map[string]index.Entry{}
	tracked := map[string]bool{}
	stages := map[string]int{} // bit per stage present
	r.Merging = MergeHead(gitDir) != ""
	for _, e := range idx.Entries {
		tracked[e.Path] = true
		if e.Stage() != 0 {
			if len(r.Unmerged) == 0 || r.Unmerged[len(r.Unmerged)-1] != e.Path {
				r.Unmerged = append(r.Unmerged, e.Path)
			}
			stages[e.Path] |= 1 << e.Stage()
			continue
		}
		staged[e.Path] = e
		h, inHead := headFiles[e.Path]
		switch {
		case !inHead:
			r.Staged = append(r.Staged, Change{'A', e.Path})
		case h.Hash != e.HashHex() || h.Mode != modeString(e.Mode):
			r.Staged = append(r.Staged, Change{changeKind(h.Mode, modeString(e.Mode)), e.Path})
		}
	}
	for p := range headFiles {
		if _, ok := staged[p]; !ok && !contains(r.Unmerged, p) {
			r.Staged = append(r.Staged, Change{'D', p})
		}
	}

	r.Conflict = map[string]string{}
	for p, bits := range stages {
		// which of base (1), ours (2), theirs (3) exist says what kind of conflict it is
		r.Conflict[p] = map[int]string{
			1<<1 | 1<<2 | 1<<3: "UU", 1<<2 | 1<<3: "AA", 1<<1 | 1<<2: "UD", 1<<1 | 1<<3: "DU",
			1 << 2: "AU", 1 << 3: "UA", 1 << 1: "DD",
		}[bits]
	}

	// 2. index vs disk
	indexInfo, _ := os.Stat(filepath.Join(gitDir, "index"))
	for _, e := range idx.Entries {
		if e.Stage() != 0 {
			continue
		}
		kind, err := diskChange(gitDir, workTree, e, indexInfo)
		if err != nil {
			return nil, err
		}
		if kind != 0 {
			r.Unstaged = append(r.Unstaged, Change{kind, e.Path})
		}
	}

	// 3. untracked
	if r.Untracked, err = untracked(workTree, tracked); err != nil {
		return nil, err
	}

	sortChanges(r.Staged)
	sortChanges(r.Unstaged)
	return r, nil
}

func modeString(m uint32) string { return fmt.Sprintf("%o", m) } // 0o40000 -> "40000", like in trees

// a file that became a symlink (or back) is a "typechange", not a modification
func changeKind(oldMode, newMode string) byte {
	if (oldMode == "120000") != (newMode == "120000") {
		return 'T'
	}
	return 'M'
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sortChanges(c []Change) { sort.Slice(c, func(i, j int) bool { return c[i].Path < c[j].Path }) }

// flattenTree lists every file in a tree, recursively, as "dir/sub/file" -> entry.
func flattenTree(gitDir, hash, prefix string, out map[string]object.TreeEntry) error {
	_, data, err := object.Read(gitDir, hash)
	if err != nil {
		return err
	}
	entries, err := object.ParseTree(data)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Mode == "40000" {
			if err := flattenTree(gitDir, e.Hash, prefix+e.Name+"/", out); err != nil {
				return err
			}
			continue
		}
		out[prefix+e.Name] = e
	}
	return nil
}

// diskChange compares an index entry with the file on disk. The fast path: if size, mtime,
// inode and mode match what the index remembers, the file didn't change. If the file was
// modified in the same instant the index was written ("racily clean") the stat data can't be
// trusted, so we hash the content like git does.
func diskChange(gitDir, workTree string, e index.Entry, indexInfo os.FileInfo) (byte, error) {
	full := filepath.Join(workTree, filepath.FromSlash(e.Path))
	now, err := index.FromFile(full, e.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return 'D', nil
	}
	if err != nil {
		return 'M', nil // a folder or something odd where a file used to be
	}
	if now.Mode != e.Mode {
		return changeKind(modeString(e.Mode), modeString(now.Mode)), nil
	}
	sameStat := now.Size == e.Size && now.MtimeSec == e.MtimeSec && now.MtimeNsec == e.MtimeNsec &&
		now.Ino == e.Ino && now.CtimeSec == e.CtimeSec && now.CtimeNsec == e.CtimeNsec
	racy := indexInfo == nil || int64(e.MtimeSec) >= indexInfo.ModTime().Unix()
	if sameStat && !racy {
		return 0, nil
	}
	if now.Size != e.Size {
		return 'M', nil // different size, no need to hash
	}
	var content []byte
	if now.Mode == 0o120000 {
		target, err := os.Readlink(full)
		if err != nil {
			return 0, err
		}
		content = []byte(target)
	} else if content, err = os.ReadFile(full); err != nil {
		return 0, err
	}
	if object.Hash(object.Blob, content) != e.HashHex() {
		return 'M', nil
	}
	return 0, nil
}

// untracked walks the working tree for files that aren't in the index and aren't ignored.
// A folder with no tracked files at all is shown once, as "folder/", like git does.
func untracked(workTree string, tracked map[string]bool) ([]string, error) {
	trackedDirs := map[string]bool{}
	for p := range tracked {
		for d := filepath.ToSlash(filepath.Dir(p)); d != "."; d = filepath.ToSlash(filepath.Dir(d)) {
			trackedDirs[d] = true
		}
	}
	ign := ignore.New(workTree)
	var out []string
	err := filepath.WalkDir(workTree, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(workTree, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || ign.Ignored(rel, true) {
				return filepath.SkipDir
			}
			if !trackedDirs[rel] {
				if hasVisibleFile(workTree, rel, ign) {
					out = append(out, rel+"/")
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !tracked[rel] && !ign.Ignored(rel, false) && (d.Type().IsRegular() || d.Type()&fs.ModeSymlink != 0) {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// hasVisibleFile reports whether a folder contains at least one file that isn't ignored.
// An untracked folder with only ignored files in it (like a build output folder) isn't shown.
func hasVisibleFile(workTree, rel string, ign *ignore.Matcher) bool {
	found := errors.New("found")
	err := filepath.WalkDir(filepath.Join(workTree, filepath.FromSlash(rel)), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		r, _ := filepath.Rel(workTree, p)
		r = filepath.ToSlash(r)
		if d.IsDir() {
			if r != rel && (d.Name() == ".git" || ign.Ignored(r, true)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !ign.Ignored(r, false) {
			return found
		}
		return nil
	})
	return err == found
}

// upstream reads branch.<name>.remote / .merge from the config and counts how far apart
// the branch and its remote-tracking branch are.
func upstream(gitDir, branch, local string) *Upstream {
	cfg := config.Load(gitDir)
	remote, merge := cfg.Get("branch."+branch+".remote"), cfg.Get("branch."+branch+".merge")
	if remote == "" || merge == "" {
		return nil
	}
	ref, name := merge, strings.TrimPrefix(merge, "refs/heads/")
	if remote != "." {
		ref = "refs/remotes/" + remote + "/" + name
		name = remote + "/" + name
	}
	u := &Upstream{Name: name}
	theirs, err := refs.Resolve(gitDir, ref)
	if err != nil {
		u.Gone = true
		return u
	}
	mine, theirSet := ancestors(gitDir, local), ancestors(gitDir, theirs)
	for h := range mine {
		if !theirSet[h] {
			u.Ahead++
		}
	}
	for h := range theirSet {
		if !mine[h] {
			u.Behind++
		}
	}
	return u
}

func ancestors(gitDir, start string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[h] {
			continue
		}
		seen[h] = true
		if c, err := ReadCommit(gitDir, h); err == nil {
			stack = append(stack, c.Parents...)
		}
	}
	return seen
}

// FlattenTree lists every file in a tree, recursively, as "dir/sub/file" -> entry.
func FlattenTree(gitDir, hash string) (map[string]object.TreeEntry, error) {
	out := map[string]object.TreeEntry{}
	return out, flattenTree(gitDir, hash, "", out)
}
