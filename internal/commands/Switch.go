// internal/commands/Switch.go
package commands

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/refs"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// ErrNoSuchBranch lets the CLI hand unusual cases (like a branch that only exists on a
// remote, which git sets up for tracking) over to git.
var ErrNoSuchBranch = errors.New("no such branch")

// Switch moves HEAD to a branch (creating it first if create is set), updating the index
// and working tree on the way. Messages go to stderr like git's.
func Switch(name string, create bool, start string, stdout, stderr io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	ref := "refs/heads/" + name
	fromName, fromHash := headName(gitDir)
	var target string
	if create {
		if !ValidBranchName(name) {
			return fmt.Errorf("'%s' is not a valid branch name", name)
		}
		if _, _, err := refs.Read(gitDir, ref); err == nil {
			return fmt.Errorf("a branch named '%s' already exists", name)
		}
		if start == "" {
			start = "HEAD"
		}
		if target, err = RevParse(gitDir, start); err != nil && !(start == "HEAD" && fromHash == "") {
			return fmt.Errorf("invalid reference: %s", start)
		}
	} else {
		if target, _, err = refs.Read(gitDir, ref); err != nil {
			return fmt.Errorf("%s: %w", name, ErrNoSuchBranch)
		}
		if current, _ := refs.Follow(gitDir, "HEAD"); current == ref {
			// git still writes a reflog line ("moving from main to main") and lists local changes
			refs.LogHEAD(gitDir, fromHash, fromHash, who(gitDir), fmt.Sprintf("checkout: moving from %s to %s", fromName, name))
			showLocalChanges(stdout)
			fmt.Fprintf(stderr, "Already on '%s'\n", name)
			return nil
		}
	}

	if target != "" && target != fromHash {
		if err := moveTo(gitDir, fromHash, target); err != nil {
			return err
		}
	}
	if create && target != "" {
		if err := refs.Update(gitDir, ref, target, refs.Zero, who(gitDir), "branch: Created from "+start); err != nil {
			return err
		}
	}
	if err := refs.SetSymbolic(gitDir, "HEAD", ref); err != nil {
		return err
	}
	if target != "" {
		refs.LogHEAD(gitDir, fromHash, target, who(gitDir), fmt.Sprintf("checkout: moving from %s to %s", fromName, name))
	}
	showLocalChanges(stdout)
	if create {
		fmt.Fprintf(stderr, "Switched to a new branch '%s'\n", name)
	} else {
		fmt.Fprintf(stderr, "Switched to branch '%s'\n", name)
	}
	return nil
}

// Detach moves HEAD to a commit directly ("detached HEAD").
func Detach(rev string, stdout, stderr io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	target, err := RevParse(gitDir, rev)
	if err != nil {
		return err
	}
	c, err := ReadCommit(gitDir, target)
	if err != nil {
		return err
	}
	fromName, fromHash := headName(gitDir)
	if target != fromHash {
		if err := moveTo(gitDir, fromHash, target); err != nil {
			return err
		}
	}
	if err := refs.SetDetached(gitDir, target); err != nil {
		return err
	}
	refs.LogHEAD(gitDir, fromHash, target, who(gitDir), fmt.Sprintf("checkout: moving from %s to %s", fromName, rev))
	showLocalChanges(stdout)
	fmt.Fprintf(stderr, "Note: you're on commit '%s' now, not on a branch (\"detached HEAD\").\n", rev)
	fmt.Fprintf(stderr, "HEAD is now at %s %s\n", target[:7], subject(c.Message))
	return nil
}

// showLocalChanges lists changes that came along, like git: "M\tfile".
func showLocalChanges(stdout io.Writer) {
	st, err := Status()
	if err != nil {
		return
	}
	kinds := map[string]byte{}
	for _, c := range st.Staged {
		kinds[c.Path] = c.Kind
	}
	for _, c := range st.Unstaged {
		if _, ok := kinds[c.Path]; !ok || c.Kind == 'D' {
			kinds[c.Path] = c.Kind
		}
	}
	var paths []string
	for p := range kinds {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		k := kinds[p]
		if k == 'T' {
			k = 'M'
		}
		fmt.Fprintf(stdout, "%c\t%s\n", k, p)
	}
}

// IsBranch reports whether name is a local branch.
func IsBranch(name string) bool {
	gitDir, err := repo.Find(".")
	if err != nil || strings.HasPrefix(name, "-") {
		return false
	}
	_, _, err = refs.Read(gitDir, "refs/heads/"+name)
	return err == nil
}
