package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestNoPullRequestOnTopOfUncommittedWork(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir, remote := teamRepo(t, h, "psycho")
	teammate(t, remote, "login", map[string]string{"login.go": "package core\n"})
	write(t, dir, map[string]string{"core.go": "package core\n// not committed yet\n"})

	h.luck = 0.01
	_, _, errs := run("ls-files")
	h.luck = lucky
	if strings.Contains(errs, "Pull request") || readFile(t, dir, "login.go") != "<missing>" {
		t.Errorf("she merged on top of uncommitted work:\n%s", errs)
	}
	if got := readFile(t, dir, "core.go"); got != "package core\n// not committed yet\n" {
		t.Errorf("your work changed: %q", got)
	}
}
