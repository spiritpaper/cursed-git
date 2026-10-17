package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// history builds, with real git, a repo with everything awkward for `log`:
//
//	main:  c1 - c2 ------- M (merge, two parents) - c5
//	                \     /
//	feature:         c3 - c4
//
// with multi-paragraph messages, blank lines inside messages and different timezones.
func history(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	date := 1759500000
	g := func(args ...string) {
		t.Helper()
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git not installed")
		}
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		tz := []string{"+0330", "-0800", "+0000", "+0545"}[date%4]
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=dev", "GIT_AUTHOR_EMAIL=dev@cursed.git", "GIT_COMMITTER_NAME=dev", "GIT_COMMITTER_EMAIL=dev@cursed.git",
			fmt.Sprintf("GIT_AUTHOR_DATE=%d %s", date, tz), fmt.Sprintf("GIT_COMMITTER_DATE=%d %s", date, tz))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		date += 3601
	}
	file := func(name, content string) { os.WriteFile(filepath.Join(dir, name), []byte(content), 0644) }

	g("init", "-q", "-b", "main")
	file("a.txt", "1\n")
	g("add", ".")
	g("commit", "-q", "-m", "first curse")
	file("a.txt", "2\n")
	g("commit", "-qam", "add warning", "-m", "Second paragraph.\nWith two lines.", "-m", "Third.")
	g("checkout", "-q", "-b", "feature")
	file("b.txt", "b\n")
	g("add", ".")
	g("commit", "-q", "-m", "feature: start")
	file("b.txt", "bb\n")
	g("commit", "-qam", "feature: finish")
	g("checkout", "-q", "main")
	file("c.txt", "c\n")
	g("add", ".")
	g("commit", "-q", "-m", "meanwhile on main")
	g("merge", "-q", "--no-ff", "feature", "-m", "Merge branch 'feature'")
	file("a.txt", "3\n")
	g("commit", "-qam", "after the merge")
	return dir
}

func TestLogMatchesGit(t *testing.T) {
	dir := history(t)
	t.Chdir(dir)
	cases := [][]string{
		{},
		{"--oneline"},
		{"-n", "2"},
		{"-3", "--oneline"},
		{"--max-count=1"},
		{"feature"},
		{"HEAD~2"},
		{"--oneline", "feature", "HEAD~1"},
	}
	for _, args := range cases {
		got := mustRun(t, append([]string{"log"}, args...)...)
		want := gitIn(t, dir, append([]string{"log", "--no-decorate"}, args...)...)
		if got != want {
			t.Errorf("cg log %v differs from git\n--- cg:\n%s\n--- git:\n%s", args, got, want)
		}
	}
}

func TestRevisionsMatchGit(t *testing.T) {
	dir := history(t)
	t.Chdir(dir)
	for _, rev := range []string{"HEAD", "@", "main", "feature", "HEAD~", "HEAD~1", "HEAD^", "HEAD~2",
		"HEAD^1", "HEAD~1^2", "HEAD~1^2~1", "HEAD^^", "HEAD~1^1~2", "main^0", "feature~1", "@~3"} {
		got := mustRun(t, "rev-parse", rev)
		if want := gitIn(t, dir, "rev-parse", rev); got != want {
			t.Errorf("rev-parse %s = %s, git says %s", rev, got, want)
		}
	}
	for _, bad := range []string{"HEAD~99", "HEAD^3", "nope", "feature^2"} {
		if code, _, errs := run("rev-parse", bad); code == 0 || !strings.Contains(errs, "unknown revision") {
			t.Errorf("rev-parse %s: exit %d %q", bad, code, errs)
		}
	}
}

// After `git gc` every object lives in a packfile: cg reads them through git.
func TestReadsPackedObjects(t *testing.T) {
	dir := history(t)
	t.Chdir(dir)
	gitIn(t, dir, "gc", "-q")
	loose, _ := filepath.Glob(filepath.Join(dir, ".git", "objects", "??", "*"))
	if len(loose) != 0 {
		t.Fatalf("gc left %d loose objects", len(loose))
	}
	if got, want := mustRun(t, "log", "--oneline"), gitIn(t, dir, "log", "--oneline", "--no-decorate"); got != want {
		t.Errorf("log after gc:\n%s\nwant:\n%s", got, want)
	}
	head := strings.TrimSpace(gitIn(t, dir, "rev-parse", "HEAD"))
	if got := mustRun(t, "cat-file", "-p", head[:7]); got != gitIn(t, dir, "cat-file", "-p", head) {
		t.Error("cat-file -p on a packed commit differs")
	}
	if got := mustRun(t, "rev-parse", "HEAD~1^2"); got != gitIn(t, dir, "rev-parse", "HEAD~1^2") {
		t.Error("rev-parse through packed commits differs")
	}

	// and cg can keep committing on top of a fully packed history
	pinIdentity(t)
	write(t, dir, map[string]string{"d.txt": "d\n"})
	mustRun(t, "add", "d.txt")
	mustRun(t, "commit", "-m", "cg on top of packs")
	gitIn(t, dir, "fsck", "--strict")
}

func TestLogOnEmptyRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	mustRun(t, "init", "-q")
	if code, _, errs := run("log"); code == 0 || !strings.Contains(errs, "does not have any commits yet") {
		t.Errorf("exit %d %q", code, errs)
	}
}
