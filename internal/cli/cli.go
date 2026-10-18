// internal/cli/cli.go
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/curses"
	"github.com/spiritpaper/cursed-git/internal/object"
)

const usage = `cg: Cursed GIT. Like git, but she has feelings.

usage: cg <command> [args]

commands:
  init [-q] [--preset clingy|toxic|psycho] [--plain] [dir]
                   create a repository (with her in it, unless --plain)
  add [-f] <path>...  stage files and folders (respects .gitignore; -f adds ignored files)
  ls-files [-s]    list staged files (-s with mode, hash and stage)
  commit -m <msg>  commit what's staged
  status [-s] [-b] [--porcelain]  what changed
  diff [--cached] [<commit> [<commit>]]  what changed, line by line
  branch [-d|-D] [<name> [<start>]]      list, create or delete branches
  switch [-c] <branch> | --detach <commit>
  checkout [-b] <branch>|<commit>|<file>  the old all-in-one
  reset [--soft|--mixed|--hard] [<commit>] | reset <file>
  restore [--staged] [--source=<commit>] <file>
  log [--oneline] [-n <N>] [<rev>...]   show history (HEAD~2, main^ and friends work)
  mood             how she feels about you right now
  card             your relationship on one screen (screenshot it)
  beg              ask for the files she took back
  graveyard        list what she took
  curse on [--preset p] | off
                   invite her back / ask her to leave
  breakup          try to leave her (you have to earn it: see cg mood)
  restraining-order  emergency off switch, no questions asked
  forget [--all]   wipe her memory of this project (or of every project)
  face [mood]  look at her (0–4, love, crying)

everything else (push, pull, merge, fetch, clone, stash, rebase, ...)
and any option cg doesn't know is handed to real git, unchanged.

plumbing:
  hash-object [-w] [-t type] [--stdin] <file>...   object id of a file (-w stores it)
  cat-file (-t|-s|-p|-e|<type>) <object>          read an object back
  write-tree                                      staged files -> tree objects
  commit-tree <tree> [-p <parent>]... -m <msg>    make a commit object
  update-ref <ref> <new> [<old>]                  move a branch
  symbolic-ref <name> [<ref>]                     read or set HEAD
  rev-parse <name>...                             name -> object id
  help         show this message
`

// version is stamped by `go install ...@v1.2.3`; a local build says "dev".
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

func printFace(w io.Writer, m curses.Mood) {
	fmt.Fprintln(w, strings.Join(curses.Portrait(m), "\n"))
	fmt.Fprintf(w, "%*s\n\n", 18+len(m.String())/2, m.String())
}

// dispatch runs one command (she has already had her say in Run).
func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printFace(stderr, curses.Unimpressed)
		fmt.Fprint(stderr, usage)
		return 1
	}

	switch args[0] {
	case "init":
		// cg init        -> current folder
		// cg init <dir>  -> that folder (created if missing)
		target, quiet, plain, preset := ".", false, false, ""
		for i := 1; i < len(args); i++ {
			switch a := args[i]; {
			case a == "-q" || a == "--quiet":
				quiet = true
			case a == "--plain":
				plain = true // a normal repo: she stays out of it
			case a == "--preset" && i+1 < len(args):
				i++
				preset = args[i]
			case strings.HasPrefix(a, "-"):
				// without this, `cg init -q` used to create a folder called "-q"
				return toGit(args, stdout, stderr) // --bare, -b <branch>, ...
			default:
				target = a
			}
		}
		if err := commands.Init(target); err != nil {
			fmt.Fprintf(stderr, "cg init: %v\n", err)
			return 1
		}
		if !quiet {
			fmt.Fprintf(stdout, "Initialized empty Cursed GIT repository in %s\n", commands.GitDir(target))
		}
		if !plain {
			her, err := curses.Enable(commands.GitDir(target), preset, curseEnv(stderr))
			if err != nil {
				fmt.Fprintf(stderr, "cg init: %v\n", err)
				return 1
			}
			if !quiet {
				her.Say("curse_on", nil)
				printHowToPlay(stderr, her)
			}
		}
		return 0

	case "help", "-h", "--help":
		printFace(stdout, curses.Content)
		fmt.Fprint(stdout, usage)
		return 0

	case "face":
		m := curses.Content
		if len(args) > 1 {
			var ok bool
			if m, ok = curses.ParseMood(args[1]); !ok {
				fmt.Fprintf(stderr, "cg face: she doesn't do %q. Try 0-4, love or crying.\n", args[1])
				return 1
			}
		}
		printFace(stdout, m)
		return 0

	case "hash-object":
		o := commands.HashObjectOptions{}
		for i := 1; i < len(args); i++ {
			switch a := args[i]; {
			case a == "-w":
				o.Write = true
			case a == "--stdin":
				o.Stdin = os.Stdin
			case a == "-t" && i+1 < len(args):
				i++
				t, err := object.ParseType(args[i])
				if err != nil {
					fmt.Fprintf(stderr, "cg hash-object: %v\n", err)
					return 1
				}
				o.Type = t
			case a == "--":
				o.Paths = append(o.Paths, args[i+1:]...)
				i = len(args)
			case strings.HasPrefix(a, "-"):
				return toGit(args, stdout, stderr)
			default:
				o.Paths = append(o.Paths, a)
			}
		}
		if o.Stdin == nil && len(o.Paths) == 0 {
			fmt.Fprintln(stderr, "usage: cg hash-object [-w] [-t type] [--stdin] <file>...")
			return 1
		}
		if err := commands.HashObject(o, stdout); err != nil {
			fmt.Fprintf(stderr, "cg hash-object: %v\n", err)
			return 1
		}
		return 0

	case "cat-file":
		if len(args) != 3 || (strings.HasPrefix(args[1], "-") && !strings.Contains("-t -s -p -e", args[1])) {
			return toGit(args, stdout, stderr) // --batch, --batch-check, ...
		}
		if err := commands.CatFile(args[1], args[2], stdout); err != nil {
			if errors.Is(err, commands.ErrMissing) {
				return 1
			}
			fmt.Fprintf(stderr, "cg cat-file: %v\n", err)
			return 1
		}
		return 0

	case "add":
		force, paths := false, []string{}
		for _, a := range args[1:] {
			switch {
			case a == "-f" || a == "--force":
				force = true
			case a == "--":
			case strings.HasPrefix(a, "-"):
				return toGit(args, stdout, stderr) // -p, -A, -u, ...
			default:
				paths = append(paths, a)
			}
		}
		if len(paths) == 0 {
			fmt.Fprintln(stderr, "Nothing specified, nothing added.\nhint: Maybe you wanted to say 'cg add .'?")
			return 0
		}
		if err := commands.Add(paths, force); err != nil {
			fmt.Fprintf(stderr, "cg add: %v\n", err)
			return 1
		}
		return 0

	case "ls-files":
		stage := false
		for _, a := range args[1:] {
			if a != "-s" && a != "--stage" {
				return toGit(args, stdout, stderr)
			}
			stage = true
		}
		if err := commands.LsFiles(stage, stdout); err != nil {
			fmt.Fprintf(stderr, "cg ls-files: %v\n", err)
			return 1
		}
		return 0

	case "commit":
		return commitCmd(args[1:], stdout, stderr)
	case "log":
		return logCmd(args[1:], stdout, stderr)
	case "write-tree":
		return writeTreeCmd(args[1:], stdout, stderr)
	case "commit-tree":
		return commitTreeCmd(args[1:], stdout, stderr)
	case "update-ref":
		return updateRefCmd(args[1:], stdout, stderr)
	case "symbolic-ref":
		return symbolicRefCmd(args[1:], stdout, stderr)
	case "rev-parse":
		return revParseCmd(args[1:], stdout, stderr)

	case "status":
		return statusCmd(args[1:], stdout, stderr)
	case "branch":
		return branchCmd(args[1:], stdout, stderr)
	case "switch":
		return switchCmd(args[1:], stdout, stderr)
	case "checkout":
		return checkoutCmd(args[1:], stdout, stderr)
	case "reset":
		return resetCmd(args[1:], stdout, stderr)
	case "restore":
		return restoreCmd(args[1:], stdout, stderr)
	case "diff":
		return diffCmd(args[1:], stdout, stderr)

	case "mood", "beg", "graveyard", "curse", "restraining-order", "breakup", "forget", "card":
		return herCmd(args, stdout, stderr)
	case "closure":
		// works without a repository: after a toxic breakup there may be nothing left
		if err := curses.Closure(".", stdout); err != nil {
			fmt.Fprintf(stderr, "cg closure: %v\n", err)
			return 1
		}
		return 0

	case "version", "--version":
		fmt.Fprintf(stdout, "cg version %s (Cursed GIT; everything she doesn't do herself goes to git)\n", version())
		return 0

	default:
		// push, pull, fetch, clone, remote, branch, checkout, stash, rebase, ...: real git does it
		return toGit(args, stdout, stderr)
	}
}
