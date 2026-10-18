// internal/commands/Init.go
package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spiritpaper/cursed-git/internal/repo"
)

func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		// the path exists now return if it is dir or file
		return info.IsDir(), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		// the path did not exist
		return false, nil
	}
	// other errors like permision denied
	return false, err
}

// the folders real git creates inside .git on init
var pa = []string{"objects/info", "objects/pack", "refs/heads", "refs/tags", "hooks", "info"}

// the files real git creates inside .git on init, with the same content
var files = map[string]string{
	"HEAD":        "ref: refs/heads/main\n",
	"config":      "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n\tlogallrefupdates = true\n",
	"description": "Unnamed repository; edit this file 'description' to name the repository.\n",
}

// GitDir returns the absolute path of the .git folder for a target directory.
func GitDir(targetdir string) string {
	return repo.ExteractGitRepo(filepath.Join(targetdir, ".git"))
}

// Init creates the base structure for a git repo inside targetdir.
// "" or "." means the current folder. The target is created if it doesn't exist.
func Init(targetdir string) error {
	if targetdir == "" {
		targetdir = "."
	}

	// the path to git dir
	gitDir := filepath.Join(targetdir, ".git")

	isGit, err := dirExists(gitDir)
	if err != nil {
		return fmt.Errorf("WHAT you looked if there is already a repo and it failed!: %w", err)
	}
	if isGit {
		return fmt.Errorf("%s is already a repo. One relationship at a time", GitDir(targetdir))
	}

	// create the core git repo
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", gitDir, err)
	}

	if err := repo.CreateFolderStructure(gitDir, pa); err != nil {
		return err
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(gitDir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}
