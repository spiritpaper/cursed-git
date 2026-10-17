package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixed identity and clock, and no personal git config: the same commit hashes on every machine.
func pinIdentity(t *testing.T) {
	for k, v := range map[string]string{
		"GIT_AUTHOR_NAME": "dev", "GIT_AUTHOR_EMAIL": "dev@cursed.git", "GIT_AUTHOR_DATE": "1759500000 +0330",
		"GIT_COMMITTER_NAME": "dev", "GIT_COMMITTER_EMAIL": "dev@cursed.git", "GIT_COMMITTER_DATE": "1759500000 +0330",
		"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null",
	} {
		t.Setenv(k, v)
	}
}

// The demo repo from cursed-git-video/git-internals.html, built entirely by cg.
// Every hash here was produced by real git and shown in the video.
func TestCommitsMatchTheVideo(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"README.md": "# cursed-git\n", "src/cg.go": "package main\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	if got := strings.TrimSpace(mustRun(t, "write-tree")); got != "7031a0bc2b367f35dbeddd63f160a1ce7ab8b617" {
		t.Fatalf("write-tree = %s, want 7031a0b (root tree in the video)", got)
	}
	if got := mustRun(t, "commit", "-m", "first curse"); got != "[main (root-commit) fe16bf5] first curse\n" {
		t.Fatalf("first commit printed %q", got)
	}

	write(t, dir, map[string]string{"README.md": "# cursed-git\n\ncg has opinions.\n"})
	mustRun(t, "add", "README.md")
	if got := mustRun(t, "commit", "-m", "add warning"); got != "[main dd70c3f] add warning\n" {
		t.Fatalf("second commit printed %q", got)
	}

	// real git agrees with all of it
	if got := gitIn(t, dir, "log", "--format=%h %s"); got != "dd70c3f add warning\nfe16bf5 first curse\n" {
		t.Errorf("git log:\n%s", got)
	}
	gitIn(t, dir, "fsck", "--strict")
	if got := gitIn(t, dir, "status", "--porcelain"); got != "" {
		t.Errorf("git status after commit is not clean:\n%s", got)
	}
	if got := gitIn(t, dir, "reflog", "--format=%gs"); got != "commit: add warning\ncommit (initial): first curse\n" {
		t.Errorf("git reflog:\n%s", got)
	}
	for name, want := range map[string]string{"HEAD": "dd70c3f", "main": "dd70c3f", "refs/heads/main": "dd70c3f", "fe16bf5": "fe16bf5"} {
		if got := mustRun(t, "rev-parse", name); !strings.HasPrefix(got, want) {
			t.Errorf("rev-parse %s = %s", name, got)
		}
	}
}

func TestCommitTreeMatchesTheVideo(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"README.md": "# cursed-git\n", "src/cg.go": "package main\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	tree := strings.TrimSpace(mustRun(t, "write-tree"))
	if got := strings.TrimSpace(mustRun(t, "commit-tree", tree[:7], "-m", "first curse")); got != "fe16bf536cf7d3f2f1769fb62c58265dd13d4c74" {
		t.Fatalf("commit-tree = %s", got)
	}
	mustRun(t, "update-ref", "refs/heads/main", "fe16bf5")
	if got := gitIn(t, dir, "log", "--oneline"); got != "fe16bf5 first curse\n" {
		t.Errorf("git log: %q", got)
	}
	// update-ref with the wrong old value must refuse
	if code, _, errs := run("update-ref", "refs/heads/main", "fe16bf5", tree); code == 0 || !strings.Contains(errs, "expected") {
		t.Errorf("stale update-ref: exit %d %q", code, errs)
	}
	if code, _, _ := run("commit-tree", "fe16bf5", "-m", "x"); code == 0 {
		t.Error("commit-tree on a commit (not a tree) should fail")
	}
}

// The tricky tree order: "foo.txt" sorts before the folder "foo", because folders sort as "foo/".
func TestTreeOrderMatchesGit(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"foo.txt": "a\n", "foo/bar": "b\n", "foo-bar": "c\n", "foo0": "d\n", "Foo": "e\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	got := strings.TrimSpace(mustRun(t, "write-tree"))
	gitIn(t, dir, "read-tree", "--empty")
	gitIn(t, dir, "add", ".")
	if want := strings.TrimSpace(gitIn(t, dir, "write-tree")); got != want {
		t.Errorf("tree %s, git made %s\n%s", got, want, gitIn(t, dir, "ls-tree", "-r", want))
	}
}

func TestGitAndCgTakeTurns(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"a.txt": "1\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "by cg")

	write(t, dir, map[string]string{"a.txt": "2\n"})
	gitIn(t, dir, "commit", "-qam", "by git")
	gitIn(t, dir, "pack-refs", "--all") // now refs/heads/main only exists in packed-refs
	if _, err := os.Stat(filepath.Join(dir, ".git", "refs", "heads", "main")); err == nil {
		t.Fatal("pack-refs didn't pack")
	}

	write(t, dir, map[string]string{"a.txt": "3\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "by cg again")
	if got := gitIn(t, dir, "log", "--format=%s"); got != "by cg again\nby git\nby cg\n" {
		t.Errorf("history:\n%s", got)
	}
	gitIn(t, dir, "fsck", "--strict")
}

func TestCommitRefusals(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	if code, out, _ := run("commit", "-m", "empty"); code == 0 || !strings.Contains(out, "nothing to commit") {
		t.Errorf("empty repo: exit %d %q", code, out)
	}
	write(t, dir, map[string]string{"a.txt": "1\n"})
	mustRun(t, "add", ".")
	if code, _, errs := run("commit", "-m", "   "); code == 0 || !strings.Contains(errs, "empty commit message") {
		t.Errorf("blank message: exit %d %q", code, errs)
	}
	mustRun(t, "commit", "-m", "one")
	if code, out, _ := run("commit", "-m", "again"); code == 0 || !strings.Contains(out, "nothing to commit") {
		t.Errorf("unchanged: exit %d %q", code, out)
	}
	t.Setenv("GIT_AUTHOR_NAME", "")
	write(t, dir, map[string]string{"a.txt": "2\n"})
	mustRun(t, "add", ".")
	if code, _, errs := run("commit", "-m", "anon"); code == 0 || !strings.Contains(errs, "who are you") {
		t.Errorf("no identity: exit %d %q", code, errs)
	}
}

func TestMultipleMessagesBecomeParagraphs(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"a.txt": "1\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "subject", "-m", "body line")
	if got := gitIn(t, dir, "log", "-1", "--format=%B"); got != "subject\n\nbody line\n\n" {
		t.Errorf("message: %q", got)
	}
}

func TestDetachedHeadCommit(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	write(t, dir, map[string]string{"a.txt": "1\n"})
	mustRun(t, "init", "-q")
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "one")
	first := strings.TrimSpace(mustRun(t, "rev-parse", "HEAD"))
	os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte(first+"\n"), 0644) // detach
	write(t, dir, map[string]string{"a.txt": "2\n"})
	mustRun(t, "add", ".")
	if got := mustRun(t, "commit", "-m", "two"); !strings.HasPrefix(got, "[HEAD ") {
		t.Errorf("detached commit printed %q", got)
	}
	if main := strings.TrimSpace(mustRun(t, "rev-parse", "main")); main != first {
		t.Error("a detached commit moved main")
	}
	gitIn(t, dir, "fsck", "--strict")
}
