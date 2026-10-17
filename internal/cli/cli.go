// internal/cli/cli.go
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"cursedgit.com/internal/commands"
	"cursedgit.com/internal/curses"
	"cursedgit.com/internal/object"
)

const usage = `cg: Cursed GIT. Like git, but she has feelings.

usage: cg <command> [args]

commands:
  init [-q] [dir]  create an empty repository (default: current folder)
  add [-f] <path>...  stage files and folders (respects .gitignore; -f adds ignored files)
  ls-files [-s]    list staged files (-s with mode, hash and stage)
  commit -m <msg>  commit what's staged
  face [mood]  look at her (0–4, love, crying)

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

func printFace(w io.Writer, m curses.Mood) {
	fmt.Fprintln(w, strings.Join(curses.Portrait(m), "\n"))
	fmt.Fprintf(w, "%*s\n\n", 18+len(m.String())/2, m.String())
}

// Run executes one cg command and returns the process exit code.
// It never calls os.Exit itself, so tests can call it directly.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printFace(stderr, curses.Unimpressed)
		fmt.Fprint(stderr, usage)
		return 1
	}

	switch args[0] {
	case "init":
		// cg init        -> current folder
		// cg init <dir>  -> that folder (created if missing)
		target, quiet := ".", false
		for _, a := range args[1:] {
			switch {
			case a == "-q" || a == "--quiet":
				quiet = true
			case strings.HasPrefix(a, "-"):
				// without this, `cg init -q` used to create a folder called "-q"
				fmt.Fprintf(stderr, "cg init: unknown option %q\n", a)
				return 1
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
				fmt.Fprintf(stderr, "cg hash-object: unknown option %q\n", a)
				return 1
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
		if len(args) != 3 {
			fmt.Fprintln(stderr, "usage: cg cat-file (-t|-s|-p|-e|<type>) <object>")
			return 1
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
				fmt.Fprintf(stderr, "cg add: unknown option %q\n", a)
				return 1
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
				fmt.Fprintf(stderr, "cg ls-files: unknown option %q\n", a)
				return 1
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
		fmt.Fprintf(stderr, "cg %s: not implemented yet\n", args[0])
		return 1

	default:
		fmt.Fprintf(stderr, "cg: unknown command %q. See 'cg help'.\n", args[0])
		return 1
	}
}
