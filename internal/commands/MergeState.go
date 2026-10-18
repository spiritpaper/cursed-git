// internal/commands/MergeState.go
//
// Merging itself is real git's job (`cg merge` and `cg pull` pass straight through). But when
// git stops on a conflict, you finish with cg, so cg has to know a merge is in progress:
//
//	.git/MERGE_HEAD   the commit being merged in: the next commit gets it as a second parent
//	.git/MERGE_MSG    the message git prepared ("Merge branch 'feature'")
//	index stages 1-3  base / ours / theirs for every file still in conflict
package commands

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// MergeHead returns the commit being merged in, or "" if no merge is in progress.
func MergeHead(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "MERGE_HEAD"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
}

// clearMerge forgets a finished merge, like git does after the merge commit.
func clearMerge(gitDir string) {
	for _, f := range []string{"MERGE_HEAD", "MERGE_MSG", "MERGE_MODE", "AUTO_MERGE"} {
		os.Remove(filepath.Join(gitDir, f))
	}
}

// Unmerged reports whether the index has conflict stages.
func Unmerged() bool {
	gitDir, err := repo.Find(".")
	if err != nil {
		return false
	}
	idx, err := index.Read(gitDir)
	if err != nil {
		return false
	}
	for _, e := range idx.Entries {
		if e.Stage() != 0 {
			return true
		}
	}
	return false
}
