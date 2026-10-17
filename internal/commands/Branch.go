// internal/commands/Branch.go
package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"cursedgit.com/internal/refs"
	"cursedgit.com/internal/repo"
)

// ValidBranchName follows the important parts of `git check-ref-format`.
func ValidBranchName(name string) bool {
	if name == "" || name == "HEAD" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") ||
		strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") ||
		strings.Contains(name, "..") || strings.Contains(name, "//") || strings.Contains(name, "@{") ||
		strings.ContainsAny(name, " ~^:?*[\\\t\n") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// ListBranches prints branches like `git branch`: "* " for the current one.
func ListBranches(stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	current, _ := refs.Follow(gitDir, "HEAD")
	if current == "HEAD" {
		hash, _ := refs.Resolve(gitDir, "HEAD")
		fmt.Fprintf(stdout, "* (HEAD detached at %s)\n", hash[:7])
	}
	for _, b := range refs.Branches(gitDir) {
		mark := "  "
		if "refs/heads/"+b == current {
			mark = "* "
		}
		fmt.Fprintf(stdout, "%s%s\n", mark, b)
	}
	return nil
}

// CreateBranch makes refs/heads/<name> point at start (default HEAD), without switching to it.
func CreateBranch(name, start string) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	if !ValidBranchName(name) {
		return fmt.Errorf("'%s' is not a valid branch name", name)
	}
	if _, _, err := refs.Read(gitDir, "refs/heads/"+name); err == nil {
		return fmt.Errorf("a branch named '%s' already exists", name)
	}
	if start == "" {
		start = "HEAD"
	}
	hash, err := RevParse(gitDir, start)
	if err != nil {
		return fmt.Errorf("not a valid object name: '%s'", start)
	}
	if _, err := ReadCommit(gitDir, hash); err != nil {
		return fmt.Errorf("not a valid branch point: '%s'", start)
	}
	return refs.Update(gitDir, "refs/heads/"+name, hash, refs.Zero, who(gitDir), "branch: Created from "+start)
}

var ErrNotMerged = errors.New("not fully merged")

// DeleteBranch deletes a branch. Without force it refuses if the branch has commits that
// aren't in HEAD (you'd lose track of them). You can't delete the branch you're on.
func DeleteBranch(name string, force bool, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	ref := "refs/heads/" + name
	hash, _, err := refs.Read(gitDir, ref)
	if err != nil {
		return fmt.Errorf("branch '%s' not found", name)
	}
	if current, _ := refs.Follow(gitDir, "HEAD"); current == ref {
		return fmt.Errorf("cannot delete branch '%s' you're on: switch to another branch first", name)
	}
	if !force {
		head, err := refs.Resolve(gitDir, "HEAD")
		if err != nil || !isAncestor(gitDir, hash, head) {
			return fmt.Errorf("the branch '%s' is %w.\nIf you are sure you want to delete it, run 'cg branch -D %s'", name, ErrNotMerged, name)
		}
	}
	if err := refs.Delete(gitDir, ref, hash); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Deleted branch %s (was %s).\n", name, hash[:7])
	return nil
}

// isAncestor reports whether commit a is reachable from commit b.
func isAncestor(gitDir, a, b string) bool {
	seen := map[string]bool{}
	stack := []string{b}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if h == a {
			return true
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		if c, err := ReadCommit(gitDir, h); err == nil {
			stack = append(stack, c.Parents...)
		}
	}
	return false
}
