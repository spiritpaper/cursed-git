package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// twins makes two identical repos with a history built by git:
//
//	main:    base -> main2         (edits a.txt, deletes old.txt, adds dir/deep/new.txt, chmod +x run.sh)
//	feature: base -> feat1         (edits shared.txt, adds feature.txt, a.txt -> symlink)
//
// mine is where cg runs (and the current directory); theirs is where git runs.
func twins(t *testing.T) (mine, theirs string) {
	t.Helper()
	pinIdentity(t)
	theirs = t.TempDir()
	g := func(args ...string) { t.Helper(); gitIn(t, theirs, args...) }
	file := func(name, content string, mode os.FileMode) {
		p := filepath.Join(theirs, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0755)
		os.Remove(p)
		os.WriteFile(p, []byte(content), mode)
		os.Chmod(p, mode)
	}
	g("init", "-q", "-b", "main")
	file("a.txt", "a\n", 0644)
	file("shared.txt", "same on both branches\n", 0644)
	file("old.txt", "going away\n", 0644)
	file("run.sh", "#!/bin/sh\n", 0644)
	g("add", ".")
	g("commit", "-q", "-m", "base")
	g("branch", "feature")
	file("a.txt", "a on main\n", 0644)
	os.Remove(filepath.Join(theirs, "old.txt"))
	file("dir/deep/new.txt", "new on main\n", 0644)
	os.Chmod(filepath.Join(theirs, "run.sh"), 0755)
	g("add", "-A")
	g("commit", "-q", "-m", "main2")
	g("checkout", "-q", "feature")
	file("feature.txt", "only on feature\n", 0644)
	os.Remove(filepath.Join(theirs, "a.txt"))
	os.Symlink("shared.txt", filepath.Join(theirs, "a.txt"))
	g("add", "-A")
	g("commit", "-q", "-m", "feat1")
	g("checkout", "-q", "main")

	mine = t.TempDir()
	if out, err := exec.Command("cp", "-a", theirs+"/.", mine).CombinedOutput(); err != nil {
		t.Fatalf("cp: %v %s", err, out)
	}
	t.Chdir(mine)
	return mine, theirs
}

// snapshot is everything a person could notice: files (content + mode), index, HEAD, branches, reflog.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var files []string
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(dir, p)
		info, _ := os.Lstat(p)
		switch {
		case d.IsDir():
			files = append(files, rel+"/")
		case info.Mode()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			files = append(files, fmt.Sprintf("%s -> %s", rel, target))
		default:
			data, _ := os.ReadFile(p)
			files = append(files, fmt.Sprintf("%s %o %q", rel, info.Mode().Perm(), data))
		}
		return nil
	})
	sort.Strings(files)
	head, _ := exec.Command("git", "-C", dir, "rev-parse", "--symbolic-full-name", "HEAD").Output()
	hash, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	return strings.Join(files, "\n") +
		"\n--- index\n" + gitIn(t, dir, "ls-files", "-s") +
		"--- HEAD " + string(head) + string(hash) +
		"--- branches\n" + gitIn(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads") +
		"--- reflog\n" + gitIn(t, dir, "reflog", "--format=%H %gs") +
		"--- status\n" + gitIn(t, dir, "-c", "status.renames=false", "status", "--porcelain")
}

// both runs an operation with cg in mine and with git in theirs, and checks the outcome
// (exit code, output, every file, index, HEAD, branches, reflog) is the same.
func both(t *testing.T, mine, theirs string, args ...string) {
	t.Helper()
	code, out, errs := run(args...)
	cmd := exec.Command("git", append([]string{"-C", theirs}, args...)...)
	var gout, gerr strings.Builder
	cmd.Stdout, cmd.Stderr = &gout, &gerr
	gcode := exitCode(cmd)
	if (code == 0) != (gcode == 0) {
		t.Fatalf("%v: cg exit %d (%s), git exit %d (%s)", args, code, errs, gcode, gerr.String())
	}
	if out != gout.String() {
		t.Errorf("%v stdout\n--- cg:\n%s--- git:\n%s", args, out, gout.String())
	}
	if a, b := snapshot(t, mine), snapshot(t, theirs); a != b {
		t.Errorf("%v left the repos different\n--- cg:\n%s\n--- git:\n%s", args, a, b)
	}
}

func TestSwitchingBranchesMatchesGit(t *testing.T) {
	mine, theirs := twins(t)
	both(t, mine, theirs, "switch", "feature") // files appear, disappear, become symlinks, folders vanish
	both(t, mine, theirs, "switch", "main")
	both(t, mine, theirs, "checkout", "feature")
	both(t, mine, theirs, "checkout", "-b", "topic")
	both(t, mine, theirs, "switch", "-c", "topic2", "main")
	both(t, mine, theirs, "checkout", "feature~1") // detached
	both(t, mine, theirs, "switch", "main")
	both(t, mine, theirs, "switch", "main") // already on main
}

func TestLocalChangesTravelOrBlockLikeGit(t *testing.T) {
	mine, theirs := twins(t)
	edit := func(name, content string) {
		for _, d := range []string{mine, theirs} {
			os.WriteFile(filepath.Join(d, name), []byte(content), 0644)
		}
	}
	// shared.txt is the same on both branches: the change comes along
	edit("shared.txt", "my local edit\n")
	both(t, mine, theirs, "switch", "feature")
	both(t, mine, theirs, "switch", "main")
	// a.txt differs between branches: switching must refuse and change nothing
	edit("a.txt", "precious local work\n")
	both(t, mine, theirs, "switch", "feature")
	code, _, errs := run("switch", "feature")
	if code == 0 || !strings.Contains(errs, "Your local changes to the following files would be overwritten") {
		t.Errorf("refusal message: %q", errs)
	}
	both(t, mine, theirs, "checkout", "--", "a.txt") // throw the edit away
	// an untracked file where the other branch has a file
	edit("feature.txt", "untracked and in the way\n")
	both(t, mine, theirs, "switch", "feature")
	// identical content doesn't matter: an untracked file in the way is always a refusal
	edit("feature.txt", "only on feature\n")
	both(t, mine, theirs, "switch", "feature")
}

func TestResetMatchesGit(t *testing.T) {
	for _, mode := range []string{"--soft", "--mixed", "--hard"} {
		t.Run(mode, func(t *testing.T) {
			mine, theirs := twins(t)
			for _, d := range []string{mine, theirs} {
				os.WriteFile(filepath.Join(d, "a.txt"), []byte("uncommitted\n"), 0644)
				os.WriteFile(filepath.Join(d, "untracked.txt"), []byte("never touched\n"), 0644)
				gitIn(t, d, "add", "a.txt")
			}
			both(t, mine, theirs, "reset", mode, "HEAD~1")
		})
	}
	mine, theirs := twins(t)
	for _, d := range []string{mine, theirs} {
		os.WriteFile(filepath.Join(d, "a.txt"), []byte("staged\n"), 0644)
		gitIn(t, d, "add", "a.txt")
	}
	both(t, mine, theirs, "reset", "a.txt")        // unstage one file
	both(t, mine, theirs, "reset", "-q", "--hard") // and throw it away
}

func TestRestoreMatchesGit(t *testing.T) {
	mine, theirs := twins(t)
	for _, d := range []string{mine, theirs} {
		os.WriteFile(filepath.Join(d, "a.txt"), []byte("staged\n"), 0644)
		gitIn(t, d, "add", "a.txt")
		os.WriteFile(filepath.Join(d, "a.txt"), []byte("then edited\n"), 0644)
		os.Remove(filepath.Join(d, "shared.txt"))
	}
	both(t, mine, theirs, "restore", "shared.txt")            // deleted file comes back
	both(t, mine, theirs, "restore", "a.txt")                 // files <- index
	both(t, mine, theirs, "restore", "--staged", "a.txt")     // index <- HEAD
	both(t, mine, theirs, "restore", "--source=feature", ".") // files <- another commit
	both(t, mine, theirs, "checkout", "main~1", "--", "a.txt")
}

func TestBranchCommandsMatchGit(t *testing.T) {
	mine, theirs := twins(t)
	both(t, mine, theirs, "branch")
	both(t, mine, theirs, "branch", "topic")
	both(t, mine, theirs, "branch", "old", "main~1")
	both(t, mine, theirs, "branch")
	both(t, mine, theirs, "branch", "topic") // already exists
	both(t, mine, theirs, "branch", "-d", "old")
	both(t, mine, theirs, "branch", "-d", "feature") // not merged into main: refused
	both(t, mine, theirs, "branch", "-D", "feature")
	both(t, mine, theirs, "branch", "-d", "main") // the current branch: refused
	both(t, mine, theirs, "branch", "bad name")   // invalid
	gitIn(t, mine, "pack-refs", "--all")
	gitIn(t, theirs, "pack-refs", "--all")
	both(t, mine, theirs, "branch", "-d", "topic") // a packed branch
}

func TestDiffMatchesGit(t *testing.T) {
	mine, theirs := twins(t)
	for _, d := range []string{mine, theirs} {
		os.WriteFile(filepath.Join(d, "a.txt"), []byte("a on main\nplus a line\n"), 0644) // modified
		os.Remove(filepath.Join(d, "shared.txt"))                                         // deleted
		os.Chmod(filepath.Join(d, "run.sh"), 0644)                                        // mode only
		os.WriteFile(filepath.Join(d, "bin.dat"), []byte("x\x00y"), 0644)
		os.WriteFile(filepath.Join(d, "empty.txt"), nil, 0644)
		os.WriteFile(filepath.Join(d, "staged.txt"), []byte("no newline at the end"), 0644)
		gitIn(t, d, "add", "bin.dat", "empty.txt", "staged.txt")
		gitIn(t, d, "config", "diff.indentHeuristic", "false")
	}
	both(t, mine, theirs, "diff")
	both(t, mine, theirs, "diff", "--cached")
	both(t, mine, theirs, "diff", "HEAD~1")
	both(t, mine, theirs, "diff", "main", "feature") // symlink, new and deleted files
	both(t, mine, theirs, "diff", "feature", "main")
}
