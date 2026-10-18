package cli

import (
	"strings"
	"testing"
	"time"
)

func TestLineXPPaysOnlyAfterTheCommitSticks(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	commitFile(t, dir, "poem.txt", "one\ntwo\nthree\n", "a poem in three lines")
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "10 XP (+3 pending)") || !strings.Contains(mood, "0/2000 lines") {
		t.Errorf("a decent message pays now, lines wait:\n%s", mood)
	}
	h.wait(25 * time.Hour)
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "13 XP") || strings.Contains(mood, "pending") || !strings.Contains(mood, "3/2000 lines") {
		t.Errorf("after a day the lines should be paid:\n%s", mood)
	}
}

func TestXPCantBeFarmedWithReverts(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	commitFile(t, dir, "base.txt", "base\n", "base file")
	h.wait(25 * time.Hour)
	mustRun(t, "status")
	before := xpOf(t)

	commitFile(t, dir, "big.txt", "1\n2\n3\n4\n5\n", "five lines!")
	h.wait(25 * time.Hour)
	mustRun(t, "status") // paid: +5
	gitIn(t, dir, "rm", "-q", "big.txt")
	mustRun(t, "commit", "-m", "take it back")
	commitFile(t, dir, "big.txt", "1\n2\n3\n4\n5\n", "five lines again")
	h.wait(25 * time.Hour)
	mustRun(t, "status")
	// three "decent" messages (10 each) and the 5 lines exactly once
	if got := xpOf(t) - before; got != 35 {
		t.Errorf("XP gained = %d, want 35 (re-committing the same content pays nothing)", got)
	}
}

func TestXPForWorkThatWasResetAwayIsCancelled(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	commitFile(t, dir, "a.txt", "a\n", "first")
	commitFile(t, dir, "b.txt", "b\nb\nb\n", "three b's")
	mustRun(t, "reset", "--hard", "HEAD~1")
	h.wait(25 * time.Hour)
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "1/2000 lines") {
		t.Errorf("only a.txt's line survived:\n%s", mood)
	}
}

// xpOf reads the XP number out of `cg mood`.
func xpOf(t *testing.T) int {
	t.Helper()
	mood := mustRun(t, "mood")
	i := strings.Index(mood, " XP")
	j := strings.LastIndex(mood[:i], " ")
	n := 0
	for _, c := range mood[j+1 : i] {
		n = n*10 + int(c-'0')
	}
	return n
}
