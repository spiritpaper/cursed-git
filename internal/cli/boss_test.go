package cli

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// arena: a repo with her in it (Tier 3 by default) and a branch that conflicts with main in
// two places in one file, plus a file feature deleted and main changed. Merging it is a fight.
func arena(t *testing.T, h *her, preset string) string {
	t.Helper()
	dir := newRepo(t, preset)
	write(t, dir, map[string]string{
		"song.txt":  "verse one\nchorus\nverse two\nbridge\nverse three\nmore\nlines\nhere\nend\n",
		"notes.txt": "notes\n",
	})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "song v1")
	mustRun(t, "switch", "-c", "feature")
	write(t, dir, map[string]string{"song.txt": "verse one\nCHORUS (his)\nverse two\nbridge\nverse three\nmore\nlines\nhere\nTHE END (his)\n"})
	mustRun(t, "add", "song.txt")
	gitIn(t, dir, "rm", "-q", "notes.txt")
	mustRun(t, "commit", "-m", "his song")
	mustRun(t, "switch", "main")
	write(t, dir, map[string]string{
		"song.txt":  "verse one\nchorus (mine)\nverse two\nbridge\nverse three\nmore\nlines\nhere\nthe end (mine)\n",
		"notes.txt": "notes, longer now\n",
	})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "my song")
	return dir
}

func resolve(t *testing.T, dir string) {
	t.Helper()
	write(t, dir, map[string]string{"song.txt": "verse one\nchorus (ours)\nverse two\nbridge\nverse three\nmore\nlines\nhere\nthe end\n"})
}

func TestBossFightWin(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := arena(t, h, "psycho")

	code, _, errs := run("merge", "feature") // git merges; she turns the conflict into a fight
	if code == 0 || !strings.Contains(errs, "BOSS FIGHT: Past You") || !strings.Contains(errs, "HP 3") {
		t.Fatalf("expected a fight against Past You with 3 HP (2 blocks + a deleted file), exit %d:\n%s", code, errs)
	}
	if !strings.Contains(errs, "9:00") || !strings.Contains(errs, "cg add <file>") || !strings.Contains(errs, "notes.txt was deleted on one side") {
		t.Errorf("the intro should give the clock (3 × 3:00) and explain how to win:\n%s", errs)
	}

	// running away is refused, and the conflict is still there
	if code, _, errs := run("merge", "--abort"); code == 0 || !strings.Contains(errs, "run") {
		t.Errorf("--abort should be refused: exit %d %q", code, errs)
	}
	if _, err := os.Stat(".git/MERGE_HEAD"); err != nil {
		t.Fatal("the merge was aborted anyway")
	}
	// staging the markers is refused: that's the classic beginner mistake
	if code, _, errs := run("add", "song.txt"); code == 0 || !strings.Contains(errs, "<<<<<<<") {
		t.Errorf("adding a file with markers should be refused: exit %d %q", code, errs)
	}

	h.wait(time.Minute)
	if _, _, errs := run("status"); !strings.Contains(errs, "HP ██████████ 3/3") || !strings.Contains(errs, "8:00") {
		t.Errorf("status should show his health and the clock:\n%s", errs)
	}
	resolve(t, dir)
	_, _, errs = run("add", "song.txt")
	if !strings.Contains(errs, "1/3") {
		t.Errorf("resolving song.txt is 2 hits:\n%s", errs)
	}
	mustRun(t, "add", "notes.txt")
	_, _, errs = run("commit", "-m", "we finally agree on the song")
	if !strings.Contains(errs, "K.O.") || !strings.Contains(errs, "Past You is down") {
		t.Fatalf("committing should win the fight:\n%s", errs)
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%P"); len(strings.Fields(got)) != 2 {
		t.Errorf("the winning commit should be a merge commit: %q", got)
	}
	mood := mustRun(t, "mood")
	if !strings.Contains(mood, "Boss fights: 1 won, 0 lost") || strings.Contains(mood, "⚔") {
		t.Errorf("mood should remember the win and show no fight:\n%s", mood)
	}
	if !strings.Contains(mood, "temper -1") {
		t.Errorf("winning should calm her down:\n%s", mood)
	}
}

func TestBossFightLostOnTimeDictatesTheCommit(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := arena(t, h, "psycho")
	run("merge", "feature")
	h.wait(10 * time.Minute) // 3 HP × 3 min: too slow
	_, _, errs := run("status")
	if !strings.Contains(errs, "your next commit message") {
		t.Fatalf("time's up: she should dictate the next commit message:\n%s", errs)
	}
	// the conflict is still yours to finish, as a normal conflict
	resolve(t, dir)
	mustRun(t, "add", "song.txt", "notes.txt")
	_, _, errs = run("commit", "-m", "whatever, it's done")
	if strings.Contains(errs, "K.O.") {
		t.Errorf("you can't win a fight you already lost:\n%s", errs)
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s"); !strings.Contains(got, "Past You") {
		t.Errorf("the commit should carry her dictated message, got %q", got)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "0 won, 1 lost") || !strings.Contains(mood, "lost to Past You") {
		t.Errorf("mood should remember the loss:\n%s", mood)
	}
}

func TestBossFightLostAtTier4AbortsButKeepsYourWork(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := arena(t, h, "psycho")
	run("reset", "--hard") // rewriting history: +1 temper, Tier 4
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "Tier 4") {
		t.Fatalf("setup: she should be at Tier 4:\n%s", mood)
	}
	_, _, errs := run("merge", "feature")
	if !strings.Contains(errs, "4:30") { // 3 HP × 90 s
		t.Errorf("Tier 4 clock should be 90 s per block:\n%s", errs)
	}
	write(t, dir, map[string]string{"song.txt": "half-resolved, my precious work\n"})
	h.wait(5 * time.Minute)
	_, _, errs = run("status")
	if !strings.Contains(errs, "cg beg") {
		t.Fatalf("at Tier 4 losing aborts the merge (into the graveyard):\n%s", errs)
	}
	if _, err := os.Stat(".git/MERGE_HEAD"); err == nil {
		t.Error("the merge should be gone")
	}
	if got := readFile(t, dir, "song.txt"); strings.Contains(got, "precious") {
		t.Errorf("song.txt should be back to main's version, got %q", got)
	}
	mustRun(t, "beg") // and nothing is ever really lost
	if got := readFile(t, dir, "song.txt"); got != "half-resolved, my precious work\n" {
		t.Errorf("beg should bring the resolution work back, got %q", got)
	}
}

func TestRunningAwayWithPlainGitIsALoss(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := arena(t, h, "psycho")
	run("merge", "feature")
	plainGit(t, dir, "merge", "--abort")
	_, _, errs := run("status")
	if !strings.Contains(errs, "You ran") && !strings.Contains(errs, "vanished") {
		t.Errorf("she should notice you ran:\n%s", errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "1 lost") {
		t.Errorf("running counts as a loss:\n%s", mood)
	}
}

func TestNoFightBelowTier3(t *testing.T) {
	withHer(t, tuesday, lucky)
	arena(t, nil, "toxic") // Tier 2
	_, _, errs := run("merge", "feature")
	if strings.Contains(errs, "BOSS FIGHT") || !strings.Contains(errs, "<<<<<<<") {
		t.Errorf("below Tier 3 a conflict is just a conflict (with a hint):\n%s", errs)
	}
	if code, _, _ := run("merge", "--abort"); code != 0 {
		t.Error("without a fight, --abort works like in git")
	}
}

func TestNoFightOutsideATerminal(t *testing.T) {
	arena(t, nil, "psycho") // TestMain: nobody at a terminal
	_, _, errs := run("merge", "feature")
	if strings.Contains(errs, "BOSS") {
		t.Errorf("scripts get plain git:\n%s", errs)
	}
	if code, _, _ := run("merge", "--abort"); code != 0 {
		t.Error("--abort must work for scripts")
	}
}

// At Tier 3 her random pull is a real merge: if upstream changed your lines, Chad is here.
func TestRandomPullStartsAFightWithChad(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"forever\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "us v1")
	mustRun(t, "remote", "add", "origin", remote)
	mustRun(t, "push", "-q", "-u", "origin", "main")

	other := t.TempDir() // someone else changes the same line and pushes
	plainGit(t, other, "clone", "-q", remote, ".")
	os.WriteFile(other+"/core.go", []byte("package core\n\nfunc Love() string { return \"never\" }\n"), 0644)
	plainGit(t, other, "add", ".")
	chad := exec.Command("git", "-C", other, "commit", "-q", "-m", "mine now")
	chad.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Chad", "GIT_AUTHOR_EMAIL=chad@gym.io") // env beats pinIdentity's
	if out, err := chad.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	plainGit(t, other, "push", "-q")

	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"always\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "us v2")

	h.luck = 0.01 // the pull fires
	_, _, errs := run("ls-files")
	h.luck = lucky
	if !strings.Contains(errs, "BOSS FIGHT: Chad  ·  origin/main") {
		t.Fatalf("a conflicting random pull should start a fight with Chad:\n%s", errs)
	}
	if _, err := os.Stat(dir + "/.git/MERGE_HEAD"); err != nil {
		t.Error("the pull should have left a real git merge in progress")
	}
}

// Tier 4: winning a fight makes her pull right away. If upstream conflicts: phase 2, against Chad.
func TestTier4WinStartsPhase2(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	dir := arena(t, h, "psycho")
	mustRun(t, "remote", "add", "origin", remote)
	mustRun(t, "push", "-q", "-u", "origin", "main")
	other := t.TempDir() // upstream changes the same line of notes.txt as we will
	plainGit(t, other, "clone", "-q", remote, ".")
	os.WriteFile(other+"/notes.txt", []byte("chad's notes\n"), 0644)
	plainGit(t, other, "add", ".")
	if out, err := exec.Command("git", "-C", other, "-c", "user.name=Chad", "-c", "user.email=chad@gym.io", "commit", "-q", "-m", "mine").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	plainGit(t, other, "push", "-q")
	mustRun(t, "reset", "--hard") // +1 temper: Tier 4

	run("merge", "feature")
	resolve(t, dir)
	write(t, dir, map[string]string{"notes.txt": "my notes, final\n"})
	mustRun(t, "add", "song.txt", "notes.txt")
	_, _, errs := run("commit", "-m", "won round one")
	if !strings.Contains(errs, "K.O.") || !strings.Contains(errs, "Phase 2") || !strings.Contains(errs, "BOSS FIGHT: Chad from origin") {
		t.Fatalf("a Tier 4 win should bring phase 2:\n%s", errs)
	}
	write(t, dir, map[string]string{"notes.txt": "our notes\n"})
	mustRun(t, "add", "notes.txt")
	_, _, errs = run("commit", "-m", "won round two")
	if !strings.Contains(errs, "K.O.") || strings.Contains(errs, "BOSS FIGHT") {
		t.Errorf("winning phase 2 ends it (no phase 3):\n%s", errs)
	}
	if !strings.Contains(errs, "Achievement unlocked: Phase 2") {
		t.Errorf("winning the rematch is an achievement:\n%s", errs)
	}
}
