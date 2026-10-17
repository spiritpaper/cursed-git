// internal/cli/branch.go
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"cursedgit.com/internal/commands"
	"cursedgit.com/internal/repo"
)

func fail(stderr io.Writer, cmd string, err error) int {
	var refuse *commands.WouldOverwrite
	if errors.As(err, &refuse) {
		fmt.Fprintf(stderr, "error: %v\n", err) // git's wording, line for line
		return 1
	}
	fmt.Fprintf(stderr, "cg %s: %v\n", cmd, err)
	return 1
}

// branchCmd: cg branch | cg branch <name> [<start>] | cg branch -d|-D <name>...
func branchCmd(args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0:
		if err := commands.ListBranches(stdout); err != nil {
			return fail(stderr, "branch", err)
		}
		return 0
	case (args[0] == "-d" || args[0] == "-D" || args[0] == "--delete") && len(args) > 1 && !hasOption(args[1:]):
		code := 0
		for _, name := range args[1:] {
			if err := commands.DeleteBranch(name, args[0] == "-D", stdout); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				code = 1
			}
		}
		return code
	case len(args) <= 2 && !hasOption(args):
		start := ""
		if len(args) == 2 {
			start = args[1]
		}
		if err := commands.CreateBranch(args[0], start); err != nil {
			return fail(stderr, "branch", err)
		}
		return 0
	}
	return toGit(append([]string{"branch"}, args...), stdout, stderr) // -a, -v, -m, --show-current, ...
}

// switchCmd: cg switch <branch> | cg switch -c <new> [<start>] | cg switch --detach <commit>
func switchCmd(args []string, stdout, stderr io.Writer) int {
	toG := func() int { return toGit(append([]string{"switch"}, args...), stdout, stderr) }
	var err error
	switch {
	case len(args) == 1 && !hasOption(args):
		err = commands.Switch(args[0], false, "", stdout, stderr)
	case len(args) >= 2 && len(args) <= 3 && (args[0] == "-c" || args[0] == "--create") && !hasOption(args[1:]):
		start := ""
		if len(args) == 3 {
			start = args[2]
		}
		err = commands.Switch(args[1], true, start, stdout, stderr)
	case len(args) == 2 && args[0] == "--detach" && !hasOption(args[1:]):
		err = commands.Detach(args[1], stdout, stderr)
	default:
		return toG()
	}
	if errors.Is(err, commands.ErrNoSuchBranch) {
		return toG() // maybe a remote branch: git knows how to set up tracking for it
	}
	if err != nil {
		return fail(stderr, "switch", err)
	}
	return 0
}

// checkoutCmd: the old all-in-one. cg checkout <branch> | -b <new> [<start>] | <commit> |
// [<commit>] -- <paths> | <paths>
func checkoutCmd(args []string, stdout, stderr io.Writer) int {
	toG := func() int { return toGit(append([]string{"checkout"}, args...), stdout, stderr) }
	if dash := indexOf(args, "--"); dash >= 0 {
		before, paths := args[:dash], args[dash+1:]
		if hasOption(before) || len(before) > 1 || len(paths) == 0 {
			return toG()
		}
		var err error
		if len(before) == 1 { // checkout <commit> -- <paths>: index and files from that commit
			err = commands.Restore(paths, true, true, before[0])
		} else { // checkout -- <paths>: files from the index
			err = commands.Restore(paths, false, true, "")
		}
		if err != nil {
			return fail(stderr, "checkout", err)
		}
		return 0
	}
	var err error
	switch {
	case len(args) >= 2 && len(args) <= 3 && args[0] == "-b" && !hasOption(args[1:]):
		start := ""
		if len(args) == 3 {
			start = args[2]
		}
		err = commands.Switch(args[1], true, start, stdout, stderr)
	case len(args) == 1 && !hasOption(args) && commands.IsBranch(args[0]):
		err = commands.Switch(args[0], false, "", stdout, stderr)
	case len(args) == 1 && !hasOption(args) && isRev(args[0]):
		err = commands.Detach(args[0], stdout, stderr)
	case len(args) >= 1 && !hasOption(args):
		err = commands.Restore(args, false, true, "") // checkout <file>: throw away its changes
	default:
		return toG()
	}
	if errors.Is(err, commands.ErrNoSuchBranch) {
		return toG()
	}
	if err != nil {
		return fail(stderr, "checkout", err)
	}
	return 0
}

func isRev(s string) bool {
	gitDir, err := repo.Find(".")
	if err != nil {
		return false
	}
	h, err := commands.RevParse(gitDir, s)
	if err != nil {
		return false
	}
	_, err = commands.ReadCommit(gitDir, h)
	return err == nil
}

func indexOf(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

// resetCmd: cg reset [--soft|--mixed|--hard] [-q] [<commit>] | cg reset [<commit>] [--] <paths>
func resetCmd(args []string, stdout, stderr io.Writer) int {
	mode, quiet := "", false
	var rest []string
	for _, a := range args {
		switch a {
		case "--soft", "--mixed", "--hard":
			mode = strings.TrimPrefix(a, "--")
		case "-q", "--quiet":
			quiet = true
		default:
			rest = append(rest, a)
		}
	}
	if quiet {
		stdout = io.Discard
	}
	if hasOption(rest) && indexOf(rest, "--") < 0 {
		return toGit(append([]string{"reset"}, args...), stdout, stderr)
	}
	var err error
	if dash := indexOf(rest, "--"); dash >= 0 || (len(rest) > 0 && !isRev(rest[0])) || len(rest) > 1 {
		// paths: unstage them
		rev, paths := "", rest
		if dash >= 0 {
			paths = rest[dash+1:]
			if dash == 1 {
				rev = rest[0]
			}
		} else if len(rest) > 1 && isRev(rest[0]) {
			rev, paths = rest[0], rest[1:]
		}
		if mode != "" && mode != "mixed" {
			fmt.Fprintf(stderr, "cg reset: cannot do a --%s reset with paths\n", mode)
			return 1
		}
		err = commands.ResetPaths(rev, paths, stdout)
	} else {
		rev := ""
		if len(rest) == 1 {
			rev = rest[0]
		}
		if mode == "" {
			mode = "mixed"
		}
		err = commands.Reset(mode, rev, stdout)
	}
	if err != nil {
		return fail(stderr, "reset", err)
	}
	return 0
}

// restoreCmd: cg restore [--staged] [--worktree] [--source=<commit>] <paths>
func restoreCmd(args []string, stdout, stderr io.Writer) int {
	staged, worktree, source := false, false, ""
	var paths []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--staged" || a == "-S":
			staged = true
		case a == "--worktree" || a == "-W":
			worktree = true
		case (a == "--source" || a == "-s") && i+1 < len(args):
			i++
			source = args[i]
		case strings.HasPrefix(a, "--source="):
			source = strings.TrimPrefix(a, "--source=")
		case a == "--":
			paths = append(paths, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-"):
			return toGit(append([]string{"restore"}, args...), stdout, stderr)
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "cg restore: you must specify path(s) to restore")
		return 1
	}
	if err := commands.Restore(paths, staged, worktree, source); err != nil {
		return fail(stderr, "restore", err)
	}
	return 0
}

// diffCmd: cg diff [--cached|--staged] [<commit> [<commit>]]
func diffCmd(args []string, stdout, stderr io.Writer) int {
	o := commands.DiffOptions{}
	for _, a := range args {
		switch {
		case a == "--cached" || a == "--staged":
			o.Cached = true
		case strings.HasPrefix(a, "-"):
			return toGit(append([]string{"diff"}, args...), stdout, stderr) // --stat, -w, --name-only, -- paths, ...
		default:
			o.Revs = append(o.Revs, a)
		}
	}
	if len(o.Revs) > 2 || (o.Cached && len(o.Revs) > 1) {
		return toGit(append([]string{"diff"}, args...), stdout, stderr)
	}
	for _, r := range o.Revs {
		if !isRev(r) {
			return toGit(append([]string{"diff"}, args...), stdout, stderr) // a path, A..B, ...
		}
	}
	if err := commands.Diff(o, stdout); err != nil {
		return fail(stderr, "diff", err)
	}
	return 0
}
