package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gitStatus runs git status with rename detection off (cg doesn't detect renames yet).
func gitStatus(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return gitIn(t, dir, append([]string{"-c", "status.renames=false", "status"}, args...)...)
}

// asCg rewrites git's hints ("use "git add ...") the way cg prints them.
func asCg(s string) string { return strings.ReplaceAll(s, `"git `, `"cg `) }

// messy builds a repo in every state status knows about.
func messy(t *testing.T) string {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	old := time.Now().Add(-time.Hour)
	write(t, dir, map[string]string{
		".gitignore":    "*.log\nbuild/\n",
		"keep.txt":      "unchanged\n",
		"edit.txt":      "v1\n",
		"both.txt":      "v1\n",
		"gone.txt":      "bye\n",
		"staged-rm.txt": "bye\n",
		"run.sh":        "#!/bin/sh\n",
		"link-me.txt":   "i will become a symlink\n",
		"src/main.go":   "package main\n",
	})
	filepath.Walk(dir, func(p string, _ os.FileInfo, _ error) error { os.Chtimes(p, old, old); return nil })
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "base")

	write(t, dir, map[string]string{
		"edit.txt":             "v2, not staged\n",
		"both.txt":             "v2 staged\n",
		"new-staged.txt":       "hello\n",
		"src/util.go":          "package main\n",
		"untracked.txt":        "?\n",
		"newdir/a.txt":         "a\n",
		"newdir/deep/b.txt":    "b\n",
		"debug.log":            "ignored\n",
		"onlyignored/x.log":    "ignored\n",
		"build/out.bin":        "ignored\n",
		"src/new-untracked.go": "package main\n",
	})
	mustRun(t, "add", "both.txt", "new-staged.txt", "src/util.go")
	write(t, dir, map[string]string{"both.txt": "v3 changed again after staging\n"})
	os.Remove("gone.txt")
	os.Remove("staged-rm.txt")
	mustRun(t, "add", "staged-rm.txt")
	os.Chmod("run.sh", 0755)
	os.Remove("link-me.txt")
	os.Symlink("keep.txt", "link-me.txt")
	return dir
}

func TestStatusPorcelainMatchesGit(t *testing.T) {
	dir := messy(t)
	got := mustRun(t, "status", "--porcelain")
	if want := gitStatus(t, dir, "--porcelain"); got != want {
		t.Errorf("--porcelain\n--- cg:\n%s--- git:\n%s", got, want)
	}
	got = mustRun(t, "status", "-sb")
	if want := gitStatus(t, dir, "-sb"); got != want {
		t.Errorf("-sb\n--- cg:\n%s--- git:\n%s", got, want)
	}
}

func TestStatusLongMatchesGit(t *testing.T) {
	dir := messy(t)
	got := mustRun(t, "status")
	if want := asCg(gitStatus(t, dir)); got != want {
		t.Errorf("long format\n--- cg:\n%s--- git:\n%s", got, want)
	}
}

func TestStatusFromSubfolderMatchesGit(t *testing.T) {
	dir := messy(t)
	t.Chdir(filepath.Join(dir, "src"))
	if got, want := mustRun(t, "status", "-s"), gitStatus(t, dir+"/src", "-s"); got != want {
		t.Errorf("-s from src/\n--- cg:\n%s--- git:\n%s", got, want)
	}
	if got, want := mustRun(t, "status"), asCg(gitStatus(t, dir+"/src")); got != want {
		t.Errorf("long from src/\n--- cg:\n%s--- git:\n%s", got, want)
	}
}

func TestStatusSimpleStatesMatchGit(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	check := func(name string) {
		t.Helper()
		if got, want := mustRun(t, "status"), asCg(gitStatus(t, dir)); got != want {
			t.Errorf("%s\n--- cg:\n%s--- git:\n%s", name, got, want)
		}
		if got, want := mustRun(t, "status", "-sb"), gitStatus(t, dir, "-sb"); got != want {
			t.Errorf("%s -sb\n--- cg:\n%s--- git:\n%s", name, got, want)
		}
	}
	check("empty repo")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	check("untracked only, no commits")
	mustRun(t, "add", ".")
	check("staged, no commits")
	mustRun(t, "commit", "-m", "one")
	check("clean")
	write(t, dir, map[string]string{"a.txt": "changed\n"})
	check("modified, nothing staged")
}

// A file rewritten with the same size in the same second as the index: the stat data
// looks unchanged, so status has to hash it.
func TestRacilyCleanFileIsDetected(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"a.txt": "aaaa\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	info, _ := os.Stat("a.txt")
	write(t, dir, map[string]string{"a.txt": "bbbb\n"})
	os.Chtimes("a.txt", info.ModTime(), info.ModTime()) // same size, same mtime
	if got := mustRun(t, "status", "--porcelain"); got != "AM a.txt\n" {
		t.Errorf("got %q, want the change to be noticed", got)
	}
}

func TestStatusWithUpstreamMatchesGit(t *testing.T) {
	pinIdentity(t)
	remote, dir := t.TempDir(), t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	t.Chdir(dir)
	write(t, dir, map[string]string{"a.txt": "1\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "one")
	// push, pull, branch: none of those are cg's, so they go through git
	mustRun(t, "remote", "add", "origin", remote)
	mustRun(t, "push", "-q", "-u", "origin", "main")
	check := func(name string) {
		t.Helper()
		if got, want := mustRun(t, "status"), asCg(gitStatus(t, dir)); got != want {
			t.Errorf("%s\n--- cg:\n%s--- git:\n%s", name, got, want)
		}
		if got, want := mustRun(t, "status", "-sb"), gitStatus(t, dir, "-sb"); got != want {
			t.Errorf("%s -sb\n--- cg:\n%s--- git:\n%s", name, got, want)
		}
	}
	check("up to date")
	for i := 2; i <= 3; i++ {
		write(t, dir, map[string]string{"a.txt": strings.Repeat("x", i) + "\n"})
		mustRun(t, "add", ".")
		mustRun(t, "commit", "-m", "more")
	}
	check("ahead 2")
	mustRun(t, "push", "-q")
	mustRun(t, "reset", "-q", "--hard", "HEAD~1")
	check("behind 1")
	write(t, dir, map[string]string{"b.txt": "b\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "diverge")
	check("diverged")
}

func TestPassthrough(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"a.txt": "1\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "one")

	// commands cg doesn't have
	if got := mustRun(t, "rev-list", "--count", "HEAD"); got != "1\n" {
		t.Errorf("rev-list: %q", got)
	}
	if got := mustRun(t, "branch", "--show-current"); got != "main\n" {
		t.Errorf("branch: %q", got)
	}
	// options cg doesn't have: the whole command goes to git
	write(t, dir, map[string]string{"a.txt": "2\n"})
	mustRun(t, "commit", "-q", "-am", "via git")
	if got := mustRun(t, "log", "--format=%s"); got != "via git\none\n" {
		t.Errorf("log --format: %q", got)
	}
	if got := mustRun(t, "rev-parse", "--abbrev-ref", "HEAD"); got != "main\n" {
		t.Errorf("rev-parse --abbrev-ref: %q", got)
	}
	// exit codes come back unchanged
	code, _, errs := run("checkout", "does-not-exist")
	if want := exitCode(exec.Command("git", "checkout", "does-not-exist")); code != want || errs == "" {
		t.Errorf("checkout of a missing branch: exit %d (git: %d), stderr %q", code, want, errs)
	}
	if code, _, _ := run("frobnicate"); code == 0 {
		t.Error("unknown command should fail like git does")
	}
	// no git installed
	old := gitBinary
	gitBinary = "definitely-not-git"
	defer func() { gitBinary = old }()
	if code, _, errs := run("push"); code != 127 || !strings.Contains(errs, "git isn't installed") {
		t.Errorf("without git: exit %d %q", code, errs)
	}
}

func exitCode(cmd *exec.Cmd) int {
	if err := cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return e.ExitCode()
		}
		return -1
	}
	return 0
}
