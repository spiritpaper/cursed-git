package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The demo repo from cursed-git-video/git-internals.html, built by real git with a fixed
// author and date so every hash is known in advance.
func demoRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# cursed-git\n"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "cg.go"), []byte("package main\n"), 0644)
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"commit", "-q", "-m", "first curse"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=dev", "GIT_AUTHOR_EMAIL=dev@cursed.git", "GIT_AUTHOR_DATE=1759500000 +0330",
			"GIT_COMMITTER_NAME=dev", "GIT_COMMITTER_EMAIL=dev@cursed.git", "GIT_COMMITTER_DATE=1759500000 +0330",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

func TestCatFileMatchesGit(t *testing.T) {
	dir := demoRepo(t)
	t.Chdir(filepath.Join(dir, "src")) // from a subfolder: cg has to find .git by walking up

	objects := map[string]string{
		"fe16bf536cf7d3f2f1769fb62c58265dd13d4c74": "commit",
		"7031a0bc2b367f35dbeddd63f160a1ce7ab8b617": "tree",
		"164b7e98663f9012bdc063cb9d97cca2c731fd86": "tree",
		"acca208c2431912c8ea918ac5bd7aad06c471ccc": "blob",
	}
	for hash, typ := range objects {
		for _, mode := range []string{"-t", "-s", "-p"} {
			code, out, errs := run("cat-file", mode, hash[:7])
			if code != 0 {
				t.Fatalf("cg cat-file %s %s: exit %d: %s", mode, hash[:7], code, errs)
			}
			if want := gitOut(t, dir, "cat-file", mode, hash); out != want {
				t.Errorf("cat-file %s %s (%s):\n cg: %q\ngit: %q", mode, hash[:7], typ, out, want)
			}
		}
	}
	if code, _, _ := run("cat-file", "-e", "acca208"); code != 0 {
		t.Error("-e on an existing object should exit 0")
	}
	if code, out, errs := run("cat-file", "-e", "deadbeef"); code != 1 || out+errs != "" {
		t.Errorf("-e on a missing object: exit %d, output %q", code, out+errs)
	}
	if code, out, _ := run("cat-file", "blob", "acca208"); code != 0 || out != "# cursed-git\n" {
		t.Errorf("cat-file blob: exit %d, %q", code, out)
	}
	if code, _, _ := run("cat-file", "tree", "acca208"); code == 0 {
		t.Error("asking for a tree when it's a blob should fail")
	}
}

func TestHashObjectWritesWhatGitReads(t *testing.T) {
	dir := demoRepo(t)
	t.Chdir(dir)
	os.WriteFile("new.txt", []byte("cg was here\n"), 0644)

	code, out, errs := run("hash-object", "-w", "new.txt")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	hash := strings.TrimSpace(out)
	if want := strings.TrimSpace(gitOut(t, dir, "hash-object", "new.txt")); hash != want {
		t.Fatalf("hash %s, git says %s", hash, want)
	}
	if got := gitOut(t, dir, "cat-file", "-p", hash); got != "cg was here\n" {
		t.Errorf("git read back %q", got)
	}
	gitOut(t, dir, "fsck", "--strict")
}

func TestHashObjectWithoutRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	os.WriteFile("f", []byte("hello\n"), 0644)
	if code, out, _ := run("hash-object", "f"); code != 0 || out != "ce013625030ba8dba906f756967f9e9ca394464a\n" {
		t.Errorf("exit %d, %q", code, out)
	}
	if code, _, errs := run("hash-object", "-w", "f"); code == 0 || !strings.Contains(errs, "not a git repository") {
		t.Errorf("-w outside a repo: exit %d, %q", code, errs)
	}
	if code, _, _ := run("hash-object", "missing.txt"); code == 0 {
		t.Error("missing file should fail")
	}
}
