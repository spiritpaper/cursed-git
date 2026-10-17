package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// write creates files (and their folders) under dir.
func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errs := run(args...)
	if code != 0 {
		t.Fatalf("cg %v: exit %d\n%s", args, code, errs)
	}
	return out
}

// A tree shaped like this project: code, docs, and the things .gitignore must keep out.
var projectLike = map[string]string{
	".gitignore":                      "/cg\nnode_modules/\ncheck-output/\ndist/\n*.mp4\n/cursed-git-video/\n",
	"main.go":                         "package main\n",
	"go.mod":                          "module cursedgit.com\n",
	"internal/cli/cli.go":             "package cli\n",
	"internal/object/object.go":       "package object\n",
	"docs/GIT.md":                     "# git\n",
	"cg":                              "a binary\n",
	"cursed-git-video/breakup.html":   "<html>\n",
	"cursed-git-video/renders/x.mp4":  "video\n",
	"tools/node_modules/pkg/index.js": "module.exports = 1\n",
	"notes/talk.mp4":                  "video\n",
	"internal/cli/testdata/cg":        "not the root binary, so it stays\n",
}

func TestAddDotStagesWhatGitStages(t *testing.T) {
	mine, theirs := t.TempDir(), t.TempDir()
	write(t, mine, projectLike)
	write(t, theirs, projectLike)
	gitIn(t, theirs, "init", "-q")
	gitIn(t, theirs, "add", ".")

	t.Chdir(mine)
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	if got, want := mustRun(t, "ls-files", "-s"), gitIn(t, theirs, "ls-files", "-s"); got != want {
		t.Errorf("cg add . staged\n%s\ngit add . staged\n%s", got, want)
	}
	// git agrees with our index: same entries, and the stat data is right (no file looks modified)
	if got, want := gitIn(t, mine, "ls-files", "-s"), gitIn(t, theirs, "ls-files", "-s"); got != want {
		t.Errorf("git reads our index as\n%s\nwant\n%s", got, want)
	}
	gitIn(t, mine, "diff-files", "--quiet")
}

func TestAddUpdatesAndStagesDeletions(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"a.txt": "one\n", "dir/b.txt": "b\n", "dir/c.txt": "c\n"})
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")

	write(t, dir, map[string]string{"a.txt": "two\n"})
	os.Remove("dir/b.txt")
	mustRun(t, "add", "a.txt", "dir")
	if got := mustRun(t, "ls-files"); got != "a.txt\ndir/c.txt\n" {
		t.Errorf("after update + delete: %q", got)
	}
	if got := strings.Fields(mustRun(t, "ls-files", "-s"))[1]; got != strings.TrimSpace(gitIn(t, dir, "hash-object", "a.txt")) {
		t.Error("a.txt was not re-hashed")
	}

	os.Remove("a.txt")
	mustRun(t, "add", "a.txt") // naming a deleted file stages its deletion
	if got := mustRun(t, "ls-files"); got != "dir/c.txt\n" {
		t.Errorf("after deleting a.txt: %q", got)
	}
	if code, _, errs := run("add", "never-existed.txt"); code == 0 || !strings.Contains(errs, "did not match") {
		t.Errorf("unknown path: exit %d %q", code, errs)
	}
}

func TestAddIgnoredFileNeedsForce(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{".gitignore": "*.log\n", "debug.log": "x\n"})
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	code, _, errs := run("add", "debug.log")
	if code == 0 || !strings.Contains(errs, "ignored") {
		t.Errorf("exit %d, %q", code, errs)
	}
	mustRun(t, "add", "-f", "debug.log")
	if got := mustRun(t, "ls-files"); got != "debug.log\n" {
		t.Errorf("-f did not add it: %q", got)
	}
}

func TestModesMatchGit(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"run.sh": "#!/bin/sh\n", "plain.txt": "x\n"})
	os.Chmod(filepath.Join(dir, "run.sh"), 0755)
	os.Symlink("plain.txt", filepath.Join(dir, "link"))
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	got := mustRun(t, "ls-files", "-s")
	gitIn(t, dir, "rm", "-q", "--cached", "-r", ".")
	gitIn(t, dir, "add", ".")
	if want := gitIn(t, dir, "ls-files", "-s"); got != want {
		t.Errorf("cg:\n%s\ngit:\n%s", got, want)
	}
}

func TestAddAndLsFilesFromSubfolder(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"top.txt": "t\n", "sub/a.txt": "a\n", "sub/deeper/b.txt": "b\n"})
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	t.Chdir(filepath.Join(dir, "sub"))
	mustRun(t, "add", ".") // "." here means sub/, not the whole repo
	mustRun(t, "add", "../top.txt")
	if got := mustRun(t, "ls-files"); got != "a.txt\ndeeper/b.txt\n" {
		t.Errorf("from sub/: %q", got)
	}
	t.Chdir(dir)
	got := strings.Split(strings.TrimSpace(mustRun(t, "ls-files")), "\n")
	if !sort.StringsAreSorted(got) || len(got) != 3 {
		t.Errorf("from the root: %q", got)
	}
	if code, _, errs := run("add", "/etc/hostname"); code == 0 || !strings.Contains(errs, "outside repository") {
		t.Errorf("outside path: exit %d %q", code, errs)
	}
}
