// internal/cli/art.go
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spiritpaper/cursed-git/internal/curses"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

const artUsage = `usage: cg art                      list the pictures
       cg art <name>               show one
       cg art add <name> [<file>]  keep a picture (from a file, or paste it and press Ctrl-D)
       cg art rm <name>            throw one of yours away
`

// artCmd: the gallery. Works without a repository, like closure.
func artCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0:
		for _, p := range curses.Gallery() {
			mark := ""
			if p.Yours {
				mark = "  (yours)"
			}
			fmt.Fprintf(stdout, "%s%s\n", p.Name, mark)
		}
		fmt.Fprintln(stdout, "\n`cg art <name>` shows one. miranda is her portrait (she shows it at `cg init`);\n"+
			"miranda-furious, miranda-love, ... are her moods (`cg mood`, `cg face <mood>`).")
		return 0

	case args[0] == "add" && (len(args) == 2 || len(args) == 3):
		in := stdin
		if len(args) == 3 && args[2] != "-" {
			f, err := os.Open(args[2])
			if err != nil {
				fmt.Fprintf(stderr, "cg art: %v\n", err)
				return 1
			}
			defer f.Close()
			in = f
		} else if f, ok := stdin.(*os.File); ok && isTerminal(f) {
			fmt.Fprintln(stderr, "Paste it, then press Ctrl-D on a new line:")
		}
		data, err := io.ReadAll(io.LimitReader(in, curses.MaxArt+1))
		if err == nil {
			err = curses.AddArt(args[1], string(data))
		}
		if err != nil {
			fmt.Fprintf(stderr, "cg art: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Kept. `cg art %s` to look at it.\n", args[1])
		return 0

	case args[0] == "rm" && len(args) == 2:
		if err := curses.RemoveArt(args[1]); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(stderr, "cg art: you have no picture called %q (the ones in cg can't be removed)\n", args[1])
			} else {
				fmt.Fprintf(stderr, "cg art: %v\n", err)
			}
			return 1
		}
		fmt.Fprintf(stdout, "Gone.\n")
		return 0

	case len(args) == 1 && args[0] != "add" && args[0] != "rm":
		art, ok := curses.Art(args[0])
		if !ok {
			fmt.Fprintf(stderr, "cg art: no picture called %q (`cg art` lists them)\n", args[0])
			return 1
		}
		fmt.Fprintln(stdout, art)
		return 0
	}
	fmt.Fprint(stderr, artUsage)
	return 1
}

// printMoodFace shows her art for a mood (miranda-furious) if someone drew it, else the
// little ASCII face. The help screen keeps the little one.
func printMoodFace(w io.Writer, m curses.Mood) {
	name := curses.DefaultPersonality
	if gitDir, err := repo.Find("."); err == nil {
		name = curses.Personality(gitDir)
	}
	art, ok := curses.MoodArt(name, m)
	if !ok {
		printFace(w, m)
		return
	}
	fmt.Fprintln(w, art)
	width := utf8.RuneCountInString(strings.SplitN(art, "\n", 2)[0])
	fmt.Fprintf(w, "%*s\n\n", width/2+len(m.String())/2, m.String())
}
