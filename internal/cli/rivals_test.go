package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRivalry(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	// commits dated after she moved in (pinIdentity dates them a year earlier)
	when := fmt.Sprintf("%d +0000", tuesday.Unix()+3600)
	t.Setenv("GIT_AUTHOR_DATE", when)
	t.Setenv("GIT_COMMITTER_DATE", when)
	commitFile(t, dir, "mine.txt", "mine\n", "my first part of the project")

	// a teammate commits three times (same machine, their own identity)
	for i := 1; i <= 3; i++ {
		os.WriteFile(filepath.Join(dir, fmt.Sprintf("riya%d.txt", i)), []byte("riya's work\nmore of it\n"), 0644)
		plainGit(t, dir, "add", ".")
		cmd := exec.Command("git", "-C", dir, "-c", "user.name=Riya Rao", "-c", "user.email=riya@team.dev", "commit", "-q", "-m", fmt.Sprintf("riya's part %d", i))
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Riya Rao", "GIT_AUTHOR_EMAIL=riya@team.dev")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	run("status") // (she notices the plain git, that's another story)

	h.luck = 0.2 // the jab fires (25%), nothing else does at Tier 1
	write(t, dir, map[string]string{"mine2.txt": "more\n"})
	mustRun(t, "add", "mine2.txt")
	_, _, errs := run("commit", "-m", "my second part, catching up")
	h.luck = lucky
	if !strings.Contains(errs, "Riya made 3 commits") {
		t.Errorf("she should compare you to the teammate who's ahead:\n%s", errs)
	}
	out := mustRun(t, "rivals")
	if !strings.Contains(out, "♥ Riya Rao") || !strings.Contains(out, "dev (you)") || !strings.Contains(out, "git shortlog -sn") {
		t.Errorf("the scoreboard should put Riya first, mark you, and show the git way:\n%s", out)
	}

	for i := 3; i <= 6; i++ {
		write(t, dir, map[string]string{fmt.Sprintf("mine%d.txt", i): "a lot more work\nreally\nso much\n"})
		mustRun(t, "add", ".")
		_, _, errs = run("commit", "-m", fmt.Sprintf("my part number %d", i))
		if strings.Contains(errs, "favourite") {
			break
		}
	}
	if !strings.Contains(errs, "Achievement unlocked: Favourite") {
		t.Errorf("taking first place should unlock Favourite:\n%s", errs)
	}
	if out := mustRun(t, "rivals"); !strings.HasPrefix(strings.Split(out, "\n")[3], "♥ dev (you)") {
		t.Errorf("now you're her favourite:\n%s", out)
	}
}
