package cli

import (
	"strings"
	"testing"
)

// angry: a repo where she's at Tier 3 with temper +1 (toxic preset: base 2, +1 temper),
// or at Tier 4 with the psycho preset. luck 0.09: the 10% sacrifice fires, but not the 8%
// random rejection of the commit itself.
func angry(t *testing.T, preset string) (dir string, h *her) {
	t.Helper()
	h = withHer(t, tuesday, lucky)
	dir = newRepo(t, preset)
	mustRun(t, "reset", "--hard") // "you want to pretend it never happened?" +1 temper
	return dir, h
}

func commitFile(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	write(t, dir, map[string]string{name: content})
	mustRun(t, "add", name)
	mustRun(t, "commit", "-m", msg)
}

func TestCommitSacrifice(t *testing.T) {
	dir, h := angry(t, "toxic")
	commitFile(t, dir, "one.txt", "1\n", "number 1")
	commitFile(t, dir, "two.txt", "2\n", "number 2")
	commitFile(t, dir, "three.txt", "3\n", "number 3")

	h.luck = 0.09
	write(t, dir, map[string]string{"four.txt": "4\n"})
	mustRun(t, "add", "four.txt")
	_, _, errs := run("commit", "-m", "number 4")
	h.luck = lucky
	if !strings.Contains(errs, "sacrificed") || !strings.Contains(errs, "number 3") || !strings.Contains(errs, "cg cherry-pick refs/cursed/sacrificed/") {
		t.Fatalf("she should sacrifice the newest older commit and explain how to get it back:\n%s", errs)
	}
	if got := gitIn(t, dir, "log", "--format=%s"); got != "number 4\nnumber 2\nnumber 1\n" {
		t.Errorf("history after the sacrifice:\n%s", got)
	}
	if readFile(t, dir, "three.txt") != "<missing>" {
		t.Error("three.txt came with the sacrificed commit and should be gone")
	}
	ref := strings.TrimSpace(gitIn(t, dir, "for-each-ref", "--format=%(refname)", "refs/cursed/sacrificed"))
	if got := gitIn(t, dir, "log", "-1", "--format=%s", ref); got != "number 3\n" {
		t.Errorf("%s should keep the victim, has %q", ref, got)
	}
	if out := mustRun(t, "graveyard"); !strings.Contains(out, ref) {
		t.Errorf("cg graveyard should list it:\n%s", out)
	}
	mustRun(t, "cherry-pick", ref) // and git brings it back
	if got := readFile(t, dir, "three.txt"); got != "3\n" {
		t.Errorf("cherry-pick should restore three.txt, got %q", got)
	}
	gitIn(t, dir, "fsck", "--strict")
}

func TestSacrificeNeverTouchesPushedCommits(t *testing.T) {
	dir, h := angry(t, "toxic")
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	commitFile(t, dir, "one.txt", "1\n", "number 1")
	commitFile(t, dir, "two.txt", "2\n", "number 2")
	mustRun(t, "remote", "add", "origin", remote)
	mustRun(t, "push", "-q", "-u", "origin", "main")

	h.luck = 0.09
	commitFile(t, dir, "three.txt", "3\n", "number 3") // the only unpushed commit is the new one
	h.luck = lucky
	if got := gitIn(t, dir, "log", "--format=%s"); got != "number 3\nnumber 2\nnumber 1\n" {
		t.Errorf("pushed history must stay as it is:\n%s", got)
	}
}

func TestSacrificeThatConflictsIsABossFight(t *testing.T) {
	dir, h := angry(t, "toxic")
	commitFile(t, dir, "song.txt", "a\n", "song a")
	commitFile(t, dir, "song.txt", "b\n", "song b") // the victim
	h.luck = 0.09
	write(t, dir, map[string]string{"song.txt": "c\n"}) // depends on b: replaying it onto a conflicts
	mustRun(t, "add", "song.txt")
	_, _, errs := run("commit", "-m", "song c")
	h.luck = lucky
	if !strings.Contains(errs, "BOSS FIGHT: Past You") || !strings.Contains(errs, "your rebase") {
		t.Fatalf("a conflicting replay should start a fight:\n%s", errs)
	}
	if code, _, _ := run("rebase", "--abort"); code == 0 {
		t.Error("running from the fight should be refused")
	}
	write(t, dir, map[string]string{"song.txt": "c\n"})
	mustRun(t, "add", "song.txt")
	t.Setenv("GIT_EDITOR", "true") // keep the replayed message as it is
	_, _, errs = run("rebase", "--continue")
	if !strings.Contains(errs, "K.O.") {
		// git finishes the rebase; the fight ends with the new commit
		t.Fatalf("finishing the rebase should win the fight:\n%s", errs)
	}
}

func TestTier4SacrificesTwo(t *testing.T) {
	dir, h := angry(t, "psycho") // base 3 + temper 1 = Tier 4
	for _, n := range []string{"1", "2", "3", "4"} {
		commitFile(t, dir, "f"+n+".txt", n+"\n", "number "+n)
	}
	h.luck = 0.09
	commitFile(t, dir, "f5.txt", "5\n", "number 5")
	h.luck = lucky
	if got := gitIn(t, dir, "log", "--format=%s"); got != "number 5\nnumber 2\nnumber 1\n" {
		t.Errorf("at Tier 4 she takes two commits:\n%s", got)
	}
}
