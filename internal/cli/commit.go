// internal/cli/commit.go
package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/refs"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// commitCmd: cg commit -m <msg> [-m <paragraph>]...
func commitCmd(args []string, stdout, stderr io.Writer) int {
	var msgs []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case (a == "-m" || a == "--message") && i+1 < len(args):
			i++
			msgs = append(msgs, args[i])
		case strings.HasPrefix(a, "-m") && len(a) > 2:
			msgs = append(msgs, a[2:])
		default:
			return toGit(append([]string{"commit"}, args...), stdout, stderr) // -a, --amend, ...
		}
	}
	if len(msgs) == 0 {
		return toGit(append([]string{"commit"}, args...), stdout, stderr) // git opens your editor
	}
	res, err := commands.Commit(msgs)
	if errors.Is(err, commands.ErrUnresolved) {
		fmt.Fprintf(stderr, "error: %v\nfatal: Exiting because of an unresolved conflict.\n", err)
		if st, err := commands.Status(); err == nil {
			for _, p := range st.Unmerged {
				fmt.Fprintf(stdout, "U\t%s\n", p) // like git: the files still in conflict
			}
		}
		return 128
	}
	if errors.Is(err, commands.ErrNothingToCommit) {
		fmt.Fprintln(stdout, err) // like git: on stdout, exit 1
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "cg commit: %v\n", err)
		return 1
	}
	root := ""
	if res.Root {
		root = " (root-commit)"
	}
	fmt.Fprintf(stdout, "[%s%s %s] %s\n", res.Branch, root, res.Hash[:7], res.Subject)
	return 0
}

// writeTreeCmd: cg write-tree
func writeTreeCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: cg write-tree")
		return 1
	}
	gitDir, err := repo.Find(".")
	if err == nil {
		var hash string
		if hash, err = commands.WriteTree(gitDir); err == nil {
			fmt.Fprintln(stdout, hash)
			return 0
		}
	}
	fmt.Fprintf(stderr, "cg write-tree: %v\n", err)
	return 1
}

// commitTreeCmd: cg commit-tree <tree> [-p <parent>]... -m <msg>
func commitTreeCmd(args []string, stdout, stderr io.Writer) int {
	var tree string
	var parents, msgs []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-p" && i+1 < len(args):
			i++
			parents = append(parents, args[i])
		case a == "-m" && i+1 < len(args):
			i++
			msgs = append(msgs, args[i])
		case strings.HasPrefix(a, "-") || tree != "":
			fmt.Fprintln(stderr, "usage: cg commit-tree <tree> [-p <parent>]... -m <message>")
			return 1
		default:
			tree = a
		}
	}
	if tree == "" || len(msgs) == 0 {
		fmt.Fprintln(stderr, "usage: cg commit-tree <tree> [-p <parent>]... -m <message>")
		return 1
	}
	fail := func(err error) int { fmt.Fprintf(stderr, "cg commit-tree: %v\n", err); return 1 }
	gitDir, err := repo.Find(".")
	if err != nil {
		return fail(err)
	}
	c := object.CommitData{Message: strings.Join(msgs, "\n\n") + "\n"}
	if c.Tree, err = mustBe(gitDir, tree, object.Tree); err != nil {
		return fail(err)
	}
	for _, p := range parents {
		h, err := mustBe(gitDir, p, object.Commit)
		if err != nil {
			return fail(err)
		}
		c.Parents = append(c.Parents, h)
	}
	if c.Author, c.Committer, err = commands.Identity(gitDir); err != nil {
		return fail(err)
	}
	hash, err := object.Write(gitDir, object.Commit, object.EncodeCommit(c))
	if err != nil {
		return fail(err)
	}
	fmt.Fprintln(stdout, hash)
	return 0
}

// mustBe resolves a name and checks the object has the expected type.
func mustBe(gitDir, name string, want object.Type) (string, error) {
	hash, err := commands.RevParse(gitDir, name)
	if err != nil {
		return "", err
	}
	t, _, err := object.Read(gitDir, hash)
	if err != nil {
		return "", err
	}
	if t != want {
		return "", fmt.Errorf("%s is a %s, not a %s", name, t, want)
	}
	return hash, nil
}

// updateRefCmd: cg update-ref <ref> <new> [<old>]
func updateRefCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || len(args) > 3 || hasOption(args) {
		return toGit(append([]string{"update-ref"}, args...), stdout, stderr) // -d, -m, --stdin, ...
	}
	fail := func(err error) int { fmt.Fprintf(stderr, "cg update-ref: %v\n", err); return 1 }
	gitDir, err := repo.Find(".")
	if err != nil {
		return fail(err)
	}
	newHash, err := commands.RevParse(gitDir, args[1])
	if err != nil {
		return fail(err)
	}
	old := ""
	if len(args) == 3 {
		if old = args[2]; old != refs.Zero {
			if old, err = commands.RevParse(gitDir, old); err != nil {
				return fail(err)
			}
		}
	}
	_, committer, err := commands.Identity(gitDir)
	if err != nil {
		return fail(err)
	}
	if err := refs.Update(gitDir, args[0], newHash, old, refs.Who(committer.String()), "update-ref"); err != nil {
		return fail(err)
	}
	return 0
}

// symbolicRefCmd: cg symbolic-ref HEAD [<ref>]
func symbolicRefCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || len(args) > 2 || hasOption(args) {
		return toGit(append([]string{"symbolic-ref"}, args...), stdout, stderr) // --short, -q, ...
	}
	gitDir, err := repo.Find(".")
	if err != nil {
		fmt.Fprintf(stderr, "cg symbolic-ref: %v\n", err)
		return 1
	}
	if len(args) == 2 {
		if err := refs.SetSymbolic(gitDir, args[0], args[1]); err != nil {
			fmt.Fprintf(stderr, "cg symbolic-ref: %v\n", err)
			return 1
		}
		return 0
	}
	v, sym, err := refs.Read(gitDir, args[0])
	if err != nil || !sym {
		fmt.Fprintf(stderr, "cg symbolic-ref: %s is not a symbolic ref\n", args[0])
		return 1
	}
	fmt.Fprintln(stdout, v)
	return 0
}

// revParseCmd: cg rev-parse <name>...
func revParseCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || hasOption(args) {
		return toGit(append([]string{"rev-parse"}, args...), stdout, stderr) // --abbrev-ref, --show-toplevel, ...
	}
	gitDir, err := repo.Find(".")
	if err != nil {
		fmt.Fprintf(stderr, "cg rev-parse: %v\n", err)
		return 1
	}
	for _, name := range args {
		hash, err := commands.RevParse(gitDir, name)
		if err != nil {
			fmt.Fprintf(stderr, "cg rev-parse: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, hash)
	}
	return 0
}

// logCmd: cg log [--oneline] [-n <N> | -<N> | --max-count=<N>] [<rev>...]
func logCmd(args []string, stdout, stderr io.Writer) int {
	o := commands.LogOptions{Max: -1}
	for i := 0; i < len(args); i++ {
		a := args[i]
		num := ""
		switch {
		case a == "--oneline":
			o.Oneline = true
			continue
		case a == "-n" && i+1 < len(args):
			i++
			num = args[i]
		case strings.HasPrefix(a, "--max-count="):
			num = strings.TrimPrefix(a, "--max-count=")
		case len(a) > 1 && a[0] == '-' && strings.Trim(a[1:], "0123456789") == "":
			num = a[1:]
		case strings.HasPrefix(a, "-"):
			return toGit(append([]string{"log"}, args...), stdout, stderr) // --graph, -p, --stat, ...
		default:
			o.Starts = append(o.Starts, a)
			continue
		}
		n, err := strconv.Atoi(num)
		if err != nil || n < 0 {
			fmt.Fprintf(stderr, "cg log: %q is not a number\n", num)
			return 1
		}
		o.Max = n
	}
	if err := commands.Log(o, stdout); err != nil {
		fmt.Fprintf(stderr, "cg log: %v\n", err)
		return 1
	}
	return 0
}

func hasOption(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return true
		}
	}
	return false
}
