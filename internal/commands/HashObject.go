// internal/commands/HashObject.go
package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

type HashObjectOptions struct {
	Type  object.Type // blob unless -t says otherwise
	Write bool        // -w: also store the object in .git/objects
	Stdin io.Reader   // --stdin: hash what comes in on stdin (printed first, like git)
	Paths []string
}

// HashObject prints the object id of each input, one per line.
// Without -w it works anywhere; with -w it needs a repository.
func HashObject(o HashObjectOptions, stdout io.Writer) error {
	if o.Type == "" {
		o.Type = object.Blob
	}
	gitDir := ""
	if o.Write {
		var err error
		if gitDir, err = repo.Find("."); err != nil {
			return err
		}
	}

	one := func(data []byte) error {
		var hash string
		if o.Write {
			var err error
			if hash, err = object.Write(gitDir, o.Type, data); err != nil {
				return err
			}
		} else {
			hash = object.Hash(o.Type, data)
		}
		fmt.Fprintln(stdout, hash)
		return nil
	}

	if o.Stdin != nil {
		data, err := io.ReadAll(o.Stdin)
		if err != nil {
			return err
		}
		if err := one(data); err != nil {
			return err
		}
	}
	for _, p := range o.Paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("could not open '%s' for reading: %w", p, err)
		}
		if err := one(data); err != nil {
			return err
		}
	}
	return nil
}
