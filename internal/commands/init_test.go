package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitIn runs real git inside dir. Tests that need it are skipped when git isn't installed.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func TestInitCreatesWhatGitCreates(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range append(pa, "HEAD", "config", "description") {
		if _, err := os.Stat(filepath.Join(dir, ".git", p)); err != nil {
			t.Errorf("missing .git/%s: %v", p, err)
		}
	}
}

func TestRealGitAcceptsTheRepo(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "status")
	gitIn(t, dir, "fsck")
	if got := gitIn(t, dir, "symbolic-ref", "HEAD"); got != "refs/heads/main\n" {
		t.Errorf("HEAD points at %q, want refs/heads/main", got)
	}
}

func TestInitCreatesMissingTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "project")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "status")
}

func TestInitTwiceIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err == nil {
		t.Fatal("second init should fail, got nil")
	}
}

func TestInitReportsPermissionErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(parent, 0755) })
	if err := Init(filepath.Join(parent, "repo")); err == nil {
		t.Fatal("init in a read-only folder should fail, got nil")
	}
}
