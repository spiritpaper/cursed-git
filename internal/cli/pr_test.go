package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// teamRepo: your repo with her in it, one commit, pushed to a bare remote without an upstream
// (`git push origin main`, no -u): a remote is all she needs.
func teamRepo(t *testing.T, h *her, preset string) (dir, remote string) {
	t.Helper()
	remote = t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	dir = newRepo(t, preset)
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"forever\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	mustRun(t, "remote", "add", "origin", remote)
	mustRun(t, "push", "-q", "origin", "main")
	return dir, remote
}

// teammate: Sam clones the remote, commits files on a branch of their own and pushes it.
func teammate(t *testing.T, remote, branch string, files map[string]string) {
	t.Helper()
	other := t.TempDir()
	plainGit(t, other, "clone", "-q", remote, ".")
	plainGit(t, other, "switch", "-q", "-c", branch)
	write(t, other, files)
	plainGit(t, other, "add", ".")
	commit := exec.Command("git", "-C", other, "commit", "-q", "-m", "add the login page")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Sam Rivera", "GIT_AUTHOR_EMAIL=sam@team.dev") // env beats pinIdentity's
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if out, err := plainGit(t, other, "push", "-q", "origin", branch); err != nil {
		t.Fatalf("%v %s", err, out)
	}
}

func TestPullRequestFromATeammateMergesClean(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "psycho")
	teammate(t, remote, "login", map[string]string{"login.go": "package core\n"})

	h.luck = 0.01 // the 2% fires
	_, _, errs := run("ls-files")
	h.luck = lucky
	if !strings.Contains(errs, "Pull request #1 from Sam: origin/login → main") {
		t.Fatalf("no pull request:\n%s", errs)
	}
	if readFile(t, dir, "login.go") == "<missing>" {
		t.Error("the pull request wasn't merged")
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s%n%p"); !strings.Contains(got, "Merge pull request #1 from Sam (origin/login)") ||
		len(strings.Fields(strings.SplitN(got, "\n", 2)[1])) != 2 {
		t.Errorf("want a real merge commit named after the pull request, got %q", got)
	}
	if !strings.Contains(errs, "+25 XP") || strings.Contains(errs, "BOSS FIGHT") {
		t.Errorf("a clean merge pays and starts no fight:\n%s", errs)
	}
	if _, _, errs := run("status"); strings.Contains(errs, "behind my back") {
		t.Errorf("her own merge counted as betrayal: %q", errs)
	}
}

func TestPullRequestThatConflictsIsAFightWithItsAuthor(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "psycho")
	teammate(t, remote, "login", map[string]string{"core.go": "package core\n\nfunc Love() string { return \"never\" }\n"})
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"always\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "a small change of heart") // you didn't pull first

	h.luck = 0.01
	_, _, errs := run("ls-files")
	h.luck = lucky
	if !strings.Contains(errs, "BOSS FIGHT: Sam  ·  origin/login") {
		t.Fatalf("a conflicting pull request should be a fight against Sam:\n%s", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err != nil {
		t.Error("the fight should be a real git merge in progress")
	}
}

func TestTier2ClosesPullRequestsThatConflict(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "toxic")
	teammate(t, remote, "login", map[string]string{"core.go": "package core\n\nfunc Love() string { return \"never\" }\n"})
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"always\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "a small change of heart")
	head := gitIn(t, dir, "rev-parse", "HEAD")

	h.luck = 0.01
	_, _, errs := run("ls-files")
	h.luck = lucky
	if !strings.Contains(errs, "Pull request #1 from Sam") || !strings.Contains(errs, "cg merge origin/login") {
		t.Fatalf("she should open it and send you to sync:\n%s", errs)
	}
	if strings.Contains(errs, "BOSS FIGHT") || gitIn(t, dir, "rev-parse", "HEAD") != head {
		t.Errorf("at Tier 2 a conflicting pull request changes nothing:\n%s", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err == nil {
		t.Error("no merge should be in progress")
	}
}

// Uncommitted work in files the pull request doesn't touch: she merges, and your work stays.
func TestPullRequestLeavesYourOtherWorkAlone(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "psycho")
	teammate(t, remote, "login", map[string]string{"login.go": "package core\n"})
	write(t, dir, map[string]string{"core.go": "package core\n// not committed yet\n", "idea.txt": "new\n"})

	h.luck = 0.01
	_, _, errs := run("ls-files")
	h.luck = lucky
	if readFile(t, dir, "login.go") == "<missing>" {
		t.Fatalf("she should merge a pull request that doesn't touch your work:\n%s", errs)
	}
	if got := readFile(t, dir, "core.go"); got != "package core\n// not committed yet\n" {
		t.Errorf("your uncommitted work changed: %q", got)
	}
	if got := readFile(t, dir, "idea.txt"); got != "new\n" {
		t.Errorf("your new file changed: %q", got)
	}
	if strings.Contains(errs, "(+25 XP") && !strings.Contains(errs, "reset --keep ORIG_HEAD") {
		t.Errorf("the undo hint must keep uncommitted work (--keep, never --hard):\n%s", errs)
	}
}

// A pull request that touches a file you're still working on waits for your commit.
func TestPullRequestWaitsForYourCommit(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "psycho")
	teammate(t, remote, "login", map[string]string{"core.go": "package core\n\nfunc Love() string { return \"never\" }\n"})
	mine := "package core\n\nfunc Love() string { return \"always\" }\n"
	write(t, dir, map[string]string{"core.go": mine})

	h.luck = 0.01
	_, _, errs := run("ls-files")
	h.luck = lucky
	if !strings.Contains(errs, "core.go") || strings.Contains(errs, "BOSS FIGHT") {
		t.Fatalf("she should tell you it waits for core.go, and start nothing:\n%s", errs)
	}
	if got := readFile(t, dir, "core.go"); got != mine {
		t.Errorf("your uncommitted work changed: %q", got)
	}
	mustRun(t, "add", ".")
	_, _, errs = run("commit", "-m", "a small change of heart")
	if !strings.Contains(errs, "Pull request #1 from Sam") || !strings.Contains(errs, "BOSS FIGHT: Sam") {
		t.Errorf("right after your commit, the waiting pull request comes in (and conflicts):\n%s", errs)
	}
}

// At Tier 4 a lost fight aborts the merge: your work in other files must survive that too.
func TestLostPullRequestFightKeepsYourOtherWork(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "psycho")
	teammate(t, remote, "login", map[string]string{"core.go": "package core\n\nfunc Love() string { return \"never\" }\n"})
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"always\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "a small change of heart")
	mustRun(t, "reset", "--hard") // +1 temper: Tier 4
	write(t, dir, map[string]string{"notes.txt": "uncommitted notes\n"})

	h.luck = 0.01
	_, _, errs := run("ls-files")
	h.luck = lucky
	if !strings.Contains(errs, "BOSS FIGHT: Sam") {
		t.Fatalf("no fight:\n%s", errs)
	}
	h.wait(2 * time.Minute) // the fight clock runs out (90 s), long before the deadline (15 min)
	_, _, errs = run("status")
	if _, err := os.Stat(filepath.Join(dir, ".git", "MERGE_HEAD")); err == nil {
		t.Fatalf("at Tier 4 a lost fight aborts the merge:\n%s", errs)
	}
	if got := readFile(t, dir, "notes.txt"); got != "uncommitted notes\n" {
		t.Errorf("your other work didn't survive the abort: %q", got)
	}
}
