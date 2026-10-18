package cli

import (
	"strings"
	"testing"
	"time"
)

// tier4Deadline: psycho at Tier 4 (one temper), with uncommitted work past the 15-minute deadline.
func tier4Deadline(t *testing.T, h *her, dir string) string {
	t.Helper()
	commitFile(t, dir, "a.txt", "a\n", "a file v1")
	mustRun(t, "reset", "--hard") // +1 temper: Tier 4
	write(t, dir, map[string]string{"a.txt": "unsaved work\n"})
	h.wait(16 * time.Minute)
	_, _, errs := run("status") // Tier 4: no warnings, straight to the wipe
	return errs
}

func TestDeathMarchNeedsTheOath(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	gitIn(t, dir, "config", "cursed.deathMarch", "true")

	h.input = "no thanks\n"
	_, _, errs := run("status")
	if !strings.Contains(errs, "DEATH MARCH") || !strings.Contains(errs, "SHUT DOWN") || !strings.Contains(errs, "cg closure") {
		t.Fatalf("the warning should explain exactly what happens:\n%s", errs)
	}
	if mood := mustRun(t, "mood"); strings.Contains(mood, "☠") {
		t.Error("without the oath, nothing is real")
	}

	h.input = "I accept the death march\n"
	run("status")
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "☠  DEATH MARCH") {
		t.Errorf("after the oath, mood should say so:\n%s", mood)
	}

	gitIn(t, dir, "config", "--unset", "cursed.deathMarch")
	run("status")
	gitIn(t, dir, "config", "cursed.deathMarch", "true")
	h.input = ""
	if _, _, errs := run("status"); !strings.Contains(errs, "DEATH MARCH") {
		t.Error("turning it off and on again means accepting again")
	}
}

func TestShutdownIsFakeWithoutDeathMarch(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	before := machine.shutdowns
	errs := tier4Deadline(t, h, dir)
	if !strings.Contains(errs, "Goodbye.") {
		t.Errorf("she should still play the shutdown:\n%s", errs)
	}
	if machine.shutdowns != before {
		t.Error("without Death March the computer must never really shut down")
	}
}

func TestShutdownIsRealInDeathMarchOncePerDay(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	gitIn(t, dir, "config", "cursed.deathMarch", "true")
	h.input = "I accept the death march\n"
	run("status")
	h.input = ""
	before := machine.shutdowns
	tier4Deadline(t, h, dir)
	if machine.shutdowns != before+1 {
		t.Fatalf("Death March at Tier 4: one real shutdown, got %d", machine.shutdowns-before)
	}
	write(t, dir, map[string]string{"a.txt": "more unsaved work\n"})
	h.wait(16 * time.Minute)
	run("status")
	if machine.shutdowns != before+1 {
		t.Error("at most one real shutdown a day")
	}
}

func TestRestrainingOrderEndsTheDeathMarch(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	gitIn(t, dir, "config", "cursed.deathMarch", "true")
	h.input = "I accept the death march\n"
	run("status")
	mustRun(t, "restraining-order")
	mustRun(t, "curse", "on")
	h.input = ""
	if mood := mustRun(t, "mood"); strings.Contains(mood, "☠") {
		t.Error("after a restraining order, Death March needs a new oath")
	}
}
