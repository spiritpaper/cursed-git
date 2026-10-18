// internal/commands/LsFiles.go
package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// LsFiles prints the index. Like git, from a subfolder it only shows files under that
// folder, with paths relative to it. stage adds "<mode> <hash> <stage>\t" in front.
func LsFiles(stage bool, stdout io.Writer) error {
	gitDir, err := repo.Find(".")
	if err != nil {
		return err
	}
	idx, err := index.Read(gitDir)
	if err != nil {
		return err
	}
	here, err := repoPath(filepath.Dir(gitDir), ".")
	if err != nil {
		return err
	}
	for _, e := range idx.Entries {
		name := e.Path
		if here != "" {
			if !strings.HasPrefix(name, here+"/") {
				continue
			}
			name = strings.TrimPrefix(name, here+"/")
		}
		if stage {
			fmt.Fprintf(stdout, "%06o %s %d\t%s\n", e.Mode, e.HashHex(), e.Stage(), name)
		} else {
			fmt.Fprintln(stdout, name)
		}
	}
	return nil
}
