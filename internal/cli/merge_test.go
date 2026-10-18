package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoPair builds a repo with git (in theirs), using the helpers it's given, then copies it to
// mine and makes mine the current directory. Same idea as twins, for any history.
type builder struct {
	t   *testing.T
	dir string
}

func (b builder) git(args ...string) { b.t.Helper(); gitIn(b.t, b.dir, args...) }
func (b builder) file(name, content string) {
	p := filepath.Join(b.dir, filepath.FromSlash(name))
	os.MkdirAll(filepath.Dir(p), 0755)
	os.Remove(p)
	os.WriteFile(p, []byte(content), 0644)
}
func (b builder) commit(msg string) {
	b.t.Helper()
	b.git("add", "-A")
	b.git("commit", "-q", "-m", msg)
}

func repoPair(t *testing.T, build func(b builder)) (mine, theirs string) {
	t.Helper()
	pinIdentity(t)
	theirs = t.TempDir()
	b := builder{t, theirs}
	b.git("init", "-q", "-b", "main")
	build(b)
	mine = t.TempDir()
	if out, err := exec.Command("cp", "-a", theirs+"/.", mine).CombinedOutput(); err != nil {
		t.Fatalf("cp: %v %s", err, out)
	}
	t.Chdir(mine)
	return mine, theirs
}

// mergeState is what a merge leaves in .git besides refs and the index.
func mergeState(dir string) string {
	var b strings.Builder
	for _, f := range []string{"MERGE_HEAD", "MERGE_MSG", "MERGE_MODE", "ORIG_HEAD"} {
		data, err := os.ReadFile(filepath.Join(dir, ".git", f))
		if err != nil {
			b.WriteString(f + ": none\n")
			continue
		}
		b.WriteString(f + ": " + string(data) + "\n")
	}
	return b.String()
}

func sameMergeState(t *testing.T, mine, theirs string) {
	t.Helper()
	if a, b := mergeState(mine), mergeState(theirs); a != b {
		t.Errorf("merge state differs\n--- cg:\n%s--- git:\n%s", a, b)
	}
}

// diverged: main and feature both changed things since they split.
//
//	clean: f (different lines), a new file each side, a deletion, a chmod, a file -> symlink
//	conflicts (when withConflicts): content, add/add, modify/delete both ways, mode + content
func diverged(withConflicts bool) func(b builder) {
	return func(b builder) {
		b.file("f", "1\n2\n3\n4\n5\n6\n7\n8\n9\n")
		b.file("old", "going away\n")
		b.file("run.sh", "#!/bin/sh\n")
		b.file("ln", "becomes a link\n")
		b.file("dir/keep", "kept\n")
		if withConflicts {
			b.file("md", "modified there, deleted here\n")
			b.file("dm", "deleted there, modified here\n")
			b.file("mm", "both\nchange\nme\n")
		}
		b.commit("base")
		b.git("checkout", "-q", "-b", "feature")
		b.file("f", "1\n2 feature\n3\n4\n5\n6\n7\n8\n9\n")
		b.file("feature.txt", "only on feature\n")
		os.Remove(filepath.Join(b.dir, "old"))
		os.Chmod(filepath.Join(b.dir, "run.sh"), 0755)
		os.Remove(filepath.Join(b.dir, "ln"))
		os.Symlink("f", filepath.Join(b.dir, "ln"))
		b.file("dir/deep/new", "deep\n")
		if withConflicts {
			b.file("md", "theirs\n")
			os.Remove(filepath.Join(b.dir, "dm"))
			b.file("aa", "added by feature\n")
			b.file("mm", "both\nchanged by feature\nme\n")
		}
		b.commit("feature work")
		b.git("checkout", "-q", "main")
		b.file("f", "1\n2\n3\n4\n5\n6\n7\n8 main\n9\n")
		b.file("main.txt", "only on main\n")
		if withConflicts {
			os.Remove(filepath.Join(b.dir, "md"))
			b.file("dm", "ours\n")
			b.file("aa", "added by main\n")
			b.file("mm", "both\nchanged by main\nme\n")
			os.Chmod(filepath.Join(b.dir, "mm"), 0755)
		}
		b.commit("main work")
	}
}

// Merging is git's job: `cg merge` passes straight through. What cg must get right is
// everything after a conflict: status, refusing to commit too early, and the merge commit.
func TestFinishingAGitMergeWithCgMatchesGit(t *testing.T) {
	mine, theirs := repoPair(t, diverged(true))
	both(t, mine, theirs, "merge", "feature") // git does it in both repos; exit 1, conflicts
	for _, args := range [][]string{{}, {"-s"}} {
		_, cg, _ := run(append([]string{"status"}, args...)...)
		if want := asCg(gitStatus(t, theirs, args...)); cg != want {
			t.Errorf("status %v during a conflict\n--- cg:\n%s--- git:\n%s", args, cg, want)
		}
	}
	both(t, mine, theirs, "commit", "-m", "too early") // refused: unmerged files, "U\tpath" lines

	for _, dir := range []string{mine, theirs} {
		write(t, dir, map[string]string{"aa": "both, really\n", "mm": "both\nchanged by both\nme\n"})
	}
	both(t, mine, theirs, "add", "aa", "mm", "md", "dm")
	_, cg, _ := run("status")
	if want := asCg(gitStatus(t, theirs)); cg != want {
		t.Errorf("status after resolving\n--- cg:\n%s--- git:\n%s", cg, want)
	}
	both(t, mine, theirs, "commit", "-m", "fought it") // same two-parent commit, "commit (merge)" reflog
	sameMergeState(t, mine, theirs)
	gitIn(t, mine, "fsck", "--strict")
	if got := gitIn(t, mine, "log", "-1", "--format=%P"); len(strings.Fields(got)) != 2 {
		t.Errorf("the merge commit should have two parents, has: %s", got)
	}
}

// A merge with no conflicts but where you changed nothing: git still lets you commit it.
func TestMergeCommitWithNoChangesIsAllowed(t *testing.T) {
	mine, theirs := repoPair(t, diverged(false))
	both(t, mine, theirs, "merge", "--no-commit", "--no-ff", "feature")
	both(t, mine, theirs, "commit", "-m", "merged, nothing else")
	sameMergeState(t, mine, theirs)
}
