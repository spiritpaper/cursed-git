// internal/commands/Commit.go
package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"cursedgit.com/internal/config"
	"cursedgit.com/internal/index"
	"cursedgit.com/internal/object"
	"cursedgit.com/internal/refs"
	"cursedgit.com/internal/repo"
)

var (
	ErrNothingToCommit = errors.New("nothing to commit (the staged files are the same as the last commit)")
	ErrEmptyMessage    = errors.New("aborting commit due to empty commit message")
)

// WriteTree turns the index into tree objects, bottom-up (a folder's tree needs its
// subfolders' hashes first), and returns the hash of the root tree.
func WriteTree(gitDir string) (string, error) {
	idx, err := index.Read(gitDir)
	if err != nil {
		return "", err
	}
	for _, e := range idx.Entries {
		if e.Stage() != 0 {
			return "", fmt.Errorf("%s has an unresolved merge conflict; fix it and `cg add` it first", e.Path)
		}
	}
	return writeTreeLevel(gitDir, idx.Entries, "")
}

// writeTreeLevel writes the tree for folder prefix ("" = root) from the sorted index entries under it.
func writeTreeLevel(gitDir string, entries []index.Entry, prefix string) (string, error) {
	var tree []object.TreeEntry
	for i := 0; i < len(entries); {
		rest := strings.TrimPrefix(entries[i].Path, prefix)
		name, _, isDir := strings.Cut(rest, "/")
		if !isDir {
			tree = append(tree, object.TreeEntry{Mode: fmt.Sprintf("%o", entries[i].Mode), Name: name, Hash: entries[i].HashHex()})
			i++
			continue
		}
		// all entries in this subfolder are next to each other, because the index is sorted
		sub := prefix + name + "/"
		j := i
		for j < len(entries) && strings.HasPrefix(entries[j].Path, sub) {
			j++
		}
		hash, err := writeTreeLevel(gitDir, entries[i:j], sub)
		if err != nil {
			return "", err
		}
		tree = append(tree, object.TreeEntry{Mode: "40000", Name: name, Hash: hash})
		i = j
	}
	data, err := object.EncodeTree(tree)
	if err != nil {
		return "", err
	}
	return object.Write(gitDir, object.Tree, data)
}

// Identity works out who is committing, the way git does: GIT_AUTHOR_* / GIT_COMMITTER_*
// environment variables first, then user.name and user.email from the config.
func Identity(gitDir string) (author, committer object.Signature, err error) {
	cfg := config.Load(gitDir)
	sig := func(kind string) (object.Signature, error) {
		name := firstSet(os.Getenv("GIT_"+kind+"_NAME"), cfg.Get("user.name"))
		email := firstSet(os.Getenv("GIT_"+kind+"_EMAIL"), cfg.Get("user.email"))
		if name == "" || email == "" {
			return object.Signature{}, errors.New("who are you? She needs a name before you commit to her.\n\n" +
				"  cg config isn't a thing yet, so run:\n    git config --global user.name \"Your Name\"\n    git config --global user.email you@example.com")
		}
		s := object.Now(name, email)
		if d := os.Getenv("GIT_" + kind + "_DATE"); d != "" {
			if s.When, s.TZ, err = object.ParseDate(d); err != nil {
				return object.Signature{}, err
			}
		}
		return s, nil
	}
	if author, err = sig("AUTHOR"); err != nil {
		return
	}
	committer, err = sig("COMMITTER")
	return
}

func firstSet(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// CommitResult says what happened, for the "[main (root-commit) abc1234] message" line.
type CommitResult struct {
	Hash, Branch string
	Root         bool
	Subject      string
}

// Commit writes the index as a tree, makes a commit on top of HEAD and moves the branch.
func Commit(messages []string) (CommitResult, error) {
	gitDir, err := repo.Find(".")
	if err != nil {
		return CommitResult{}, err
	}
	msg := cleanMessage(messages)
	if msg == "" {
		return CommitResult{}, ErrEmptyMessage
	}
	tree, err := WriteTree(gitDir)
	if err != nil {
		return CommitResult{}, err
	}
	var parents []string
	parent, err := refs.Resolve(gitDir, "HEAD")
	switch {
	case err == nil:
		_, data, err := object.Read(gitDir, parent)
		if err != nil {
			return CommitResult{}, fmt.Errorf("reading the current commit: %w", err)
		}
		prev, err := object.ParseCommit(data)
		if err != nil {
			return CommitResult{}, err
		}
		if prev.Tree == tree {
			return CommitResult{}, ErrNothingToCommit
		}
		parents = []string{parent}
	case errors.Is(err, refs.ErrNotFound):
		parent = refs.Zero // first commit on this branch
		if tree == object.Hash(object.Tree, nil) {
			return CommitResult{}, ErrNothingToCommit // nothing staged at all
		}
	default:
		return CommitResult{}, err
	}

	author, committer, err := Identity(gitDir)
	if err != nil {
		return CommitResult{}, err
	}
	hash, err := object.Write(gitDir, object.Commit, object.EncodeCommit(object.CommitData{
		Tree: tree, Parents: parents, Author: author, Committer: committer, Message: msg,
	}))
	if err != nil {
		return CommitResult{}, err
	}

	subject, _, _ := strings.Cut(msg, "\n")
	kind := "commit"
	if len(parents) == 0 {
		kind = "commit (initial)"
	}
	who := refs.Who(fmt.Sprintf("%s <%s> %d %s", committer.Name, committer.Email, committer.When, committer.TZ))
	if err := refs.Update(gitDir, "HEAD", hash, parent, who, kind+": "+subject); err != nil {
		return CommitResult{}, err
	}
	branch, _ := refs.Follow(gitDir, "HEAD")
	return CommitResult{Hash: hash, Branch: strings.TrimPrefix(branch, "refs/heads/"), Root: len(parents) == 0, Subject: subject}, nil
}

// cleanMessage joins -m paragraphs with a blank line (like git) and ends with one newline.
func cleanMessage(parts []string) string {
	var kept []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n\n") + "\n"
}

// WriteTreeFrom writes trees for a list of entries (sorted by path) and returns the root tree.
func WriteTreeFrom(gitDir string, entries []index.Entry) (string, error) {
	return writeTreeLevel(gitDir, entries, "")
}
