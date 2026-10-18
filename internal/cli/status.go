// internal/cli/status.go
package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"cursedgit.com/internal/commands"
	"cursedgit.com/internal/repo"
)

// statusCmd: cg status [-s|--short] [--porcelain] [-b|--branch]
// Any other option goes to git.
func statusCmd(args []string, stdout, stderr io.Writer) int {
	short, porcelain, branch := false, false, false
	for _, a := range args {
		switch a {
		case "-s", "--short":
			short = true
		case "--porcelain", "--porcelain=v1":
			porcelain = true
		case "-b", "--branch":
			branch = true
		case "-sb", "-bs":
			short, branch = true, true
		default:
			return toGit(append([]string{"status"}, args...), stdout, stderr)
		}
	}
	r, err := commands.Status()
	if err != nil {
		fmt.Fprintf(stderr, "cg status: %v\n", err)
		return 1
	}
	if short || porcelain {
		rel := func(p string) string { return p } // porcelain: always from the repo root
		if !porcelain {
			rel = relativeToHere
		}
		printShort(stdout, r, branch, rel)
		return 0
	}
	printLong(stdout, r)
	return 0
}

// relativeToHere turns a repo path into a path from the current folder, like git prints it.
func relativeToHere(p string) string {
	gitDir, err := repo.Find(".")
	if err != nil {
		return p
	}
	here, err := filepath.Abs(".")
	if err != nil {
		return p
	}
	r, err := filepath.Rel(here, filepath.Join(filepath.Dir(gitDir), filepath.FromSlash(p)))
	if err != nil {
		return p
	}
	r = filepath.ToSlash(r)
	if strings.HasSuffix(p, "/") && !strings.HasSuffix(r, "/") {
		r += "/"
	}
	return r
}

func printShort(w io.Writer, r *commands.StatusReport, branch bool, rel func(string) string) {
	if branch {
		switch {
		case r.Unborn:
			fmt.Fprintf(w, "## No commits yet on %s\n", r.Branch)
		case r.Branch == "":
			fmt.Fprintln(w, "## HEAD (no branch)")
		case r.Upstream == nil:
			fmt.Fprintf(w, "## %s\n", r.Branch)
		case r.Upstream.Gone:
			fmt.Fprintf(w, "## %s...%s [gone]\n", r.Branch, r.Upstream.Name)
		default:
			var parts []string
			if r.Upstream.Ahead > 0 {
				parts = append(parts, fmt.Sprintf("ahead %d", r.Upstream.Ahead))
			}
			if r.Upstream.Behind > 0 {
				parts = append(parts, fmt.Sprintf("behind %d", r.Upstream.Behind))
			}
			extra := ""
			if len(parts) > 0 {
				extra = " [" + strings.Join(parts, ", ") + "]"
			}
			fmt.Fprintf(w, "## %s...%s%s\n", r.Branch, r.Upstream.Name, extra)
		}
	}
	// one line per path: X = staged change, Y = unstaged change
	type xy struct{ x, y byte }
	lines := map[string]*xy{}
	var order []string
	get := func(p string) *xy {
		if lines[p] == nil {
			lines[p] = &xy{' ', ' '}
			order = append(order, p)
		}
		return lines[p]
	}
	for _, c := range r.Staged {
		get(c.Path).x = c.Kind
	}
	for _, c := range r.Unstaged {
		get(c.Path).y = c.Kind
	}
	for _, p := range r.Unmerged {
		l := get(p)
		l.x, l.y = r.Conflict[p][0], r.Conflict[p][1]
	}
	sort.Strings(order)
	for _, p := range order {
		fmt.Fprintf(w, "%c%c %s\n", lines[p].x, lines[p].y, rel(p))
	}
	for _, p := range r.Untracked {
		fmt.Fprintf(w, "?? %s\n", rel(p))
	}
}

var labels = map[byte]string{'A': "new file:   ", 'M': "modified:   ", 'D': "deleted:    ", 'T': "typechange: "}

var conflictLabels = map[string]string{
	"UU": "both modified:   ", "AA": "both added:      ", "UD": "deleted by them: ", "DU": "deleted by us:   ",
	"AU": "added by us:     ", "UA": "added by them:   ", "DD": "both deleted:    ",
}

// printLong copies git's long format, with "cg" in the hints. They all work: whatever cg
// doesn't do itself (restore, rm, push, pull) goes to git.
func printLong(w io.Writer, r *commands.StatusReport) {
	if r.Branch != "" {
		fmt.Fprintf(w, "On branch %s\n", r.Branch)
	} else {
		fmt.Fprintf(w, "HEAD detached at %s\n", r.Detached)
	}
	if u := r.Upstream; u != nil {
		plural := func(n int) string {
			if n == 1 {
				return "commit"
			}
			return "commits"
		}
		switch {
		case u.Gone:
			fmt.Fprintf(w, "Your branch is based on '%s', but the upstream is gone.\n  (use \"cg branch --unset-upstream\" to fixup)\n", u.Name)
		case u.Ahead == 0 && u.Behind == 0:
			fmt.Fprintf(w, "Your branch is up to date with '%s'.\n", u.Name)
		case u.Behind == 0:
			fmt.Fprintf(w, "Your branch is ahead of '%s' by %d %s.\n  (use \"cg push\" to publish your local commits)\n", u.Name, u.Ahead, plural(u.Ahead))
		case u.Ahead == 0:
			fmt.Fprintf(w, "Your branch is behind '%s' by %d %s, and can be fast-forwarded.\n  (use \"cg pull\" to update your local branch)\n", u.Name, u.Behind, plural(u.Behind))
		default:
			fmt.Fprintf(w, "Your branch and '%s' have diverged,\nand have %d and %d different commits each, respectively.\n  (use \"cg pull\" if you want to integrate the remote branch with yours)\n", u.Name, u.Ahead, u.Behind)
		}
	}
	// git separates sections with a blank line, except right after a bare "On branch x" line
	bare := r.Upstream == nil
	gap := func() {
		if !bare {
			fmt.Fprintln(w)
		}
		bare = false
	}
	if r.Unborn {
		fmt.Fprint(w, "\nNo commits yet\n") // always has a blank line above it
		bare = false
	}
	switch {
	case len(r.Unmerged) > 0:
		fmt.Fprint(w, "You have unmerged paths.\n  (fix conflicts and run \"cg commit\")\n  (use \"cg merge --abort\" to abort the merge)\n")
		bare = false
	case r.Merging:
		fmt.Fprint(w, "All conflicts fixed but you are still merging.\n  (use \"cg commit\" to conclude merge)\n")
		bare = false
	}

	if len(r.Staged) > 0 {
		gap()
		switch {
		case r.Merging:
			fmt.Fprint(w, "Changes to be committed:\n") // git drops the hint in the middle of a merge
		case r.Unborn:
			fmt.Fprint(w, "Changes to be committed:\n  (use \"cg rm --cached <file>...\" to unstage)\n")
		default:
			fmt.Fprint(w, "Changes to be committed:\n  (use \"cg restore --staged <file>...\" to unstage)\n")
		}
		for _, c := range r.Staged {
			fmt.Fprintf(w, "\t%s%s\n", labels[c.Kind], relativeToHere(c.Path))
		}
	}
	if len(r.Unmerged) > 0 {
		gap()
		hint := `(use "cg add <file>..." to mark resolution)`
		for _, p := range r.Unmerged {
			if code := r.Conflict[p]; code != "UU" && code != "AA" {
				hint = `(use "cg add/rm <file>..." as appropriate to mark resolution)`
			}
		}
		fmt.Fprintf(w, "Unmerged paths:\n  %s\n", hint)
		for _, p := range r.Unmerged {
			fmt.Fprintf(w, "\t%s%s\n", conflictLabels[r.Conflict[p]], relativeToHere(p))
		}
	}
	if len(r.Unstaged) > 0 {
		add := `(use "cg add <file>..." to update what will be committed)`
		for _, c := range r.Unstaged {
			if c.Kind == 'D' {
				add = `(use "cg add/rm <file>..." to update what will be committed)`
			}
		}
		gap()
		fmt.Fprintf(w, "Changes not staged for commit:\n  %s\n  (use \"cg restore <file>...\" to discard changes in working directory)\n", add)
		for _, c := range r.Unstaged {
			fmt.Fprintf(w, "\t%s%s\n", labels[c.Kind], relativeToHere(c.Path))
		}
	}
	if len(r.Untracked) > 0 {
		gap()
		fmt.Fprint(w, "Untracked files:\n  (use \"cg add <file>...\" to include in what will be committed)\n")
		for _, p := range r.Untracked {
			fmt.Fprintf(w, "\t%s\n", relativeToHere(p))
		}
	}

	switch {
	case len(r.Staged) > 0 || len(r.Unmerged) > 0:
		fmt.Fprintln(w)
	case r.Merging:
		gap()
		fmt.Fprint(w, "nothing to commit, working tree clean\n")
	case len(r.Unstaged) > 0:
		gap()
		fmt.Fprint(w, "no changes added to commit (use \"cg add\" and/or \"cg commit -a\")\n")
	case len(r.Untracked) > 0:
		gap()
		fmt.Fprint(w, "nothing added to commit but untracked files present (use \"cg add\" to track)\n")
	case r.Unborn:
		gap()
		fmt.Fprint(w, "nothing to commit (create/copy files and use \"cg add\" to track)\n")
	default:
		gap()
		fmt.Fprint(w, "nothing to commit, working tree clean\n")
	}
}
