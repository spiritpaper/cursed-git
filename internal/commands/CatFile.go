// internal/commands/CatFile.go
package commands

import (
	"errors"
	"fmt"
	"io"

	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// ErrMissing is what `cat-file -e` returns for a missing object: exit 1, print nothing.
var ErrMissing = errors.New("object missing")

// CatFile prints an object.
//
//	-t  its type      -s  its size in bytes
//	-p  pretty-print (trees are formatted like `git ls-tree`, everything else is printed as is)
//	-e  just check it exists (exit code only)
//	<type>  print the raw content, if the object really is that type
func CatFile(mode, name string, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	t, content, err := object.Read(gitDir, name)
	if mode == "-e" {
		if errors.Is(err, object.ErrNotFound) {
			return ErrMissing
		}
		return err
	}
	if err != nil {
		return err
	}

	switch mode {
	case "-t":
		fmt.Fprintln(stdout, t)
	case "-s":
		fmt.Fprintln(stdout, len(content))
	case "-p":
		if t == object.Tree {
			entries, err := object.ParseTree(content)
			if err != nil {
				return fmt.Errorf("tree %s: %w", name, err)
			}
			fmt.Fprint(stdout, object.FormatTree(entries))
			return nil
		}
		stdout.Write(content)
	default:
		want, err := object.ParseType(mode)
		if err != nil {
			return fmt.Errorf("unknown option %q (use -t, -s, -p, -e or a type)", mode)
		}
		if want != t {
			return fmt.Errorf("%s is a %s, not a %s", name, t, want)
		}
		stdout.Write(content)
	}
	return nil
}
