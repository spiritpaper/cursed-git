package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eligible: a repo where the breakup rules are met (tiny thresholds through .git/config, the
// way an event can tune a short game). toxic makes it ending B instead of C.
func eligible(t *testing.T, h *her, toxic bool) string {
	t.Helper()
	dir := newRepo(t, "psycho")
	t.Setenv("HOME", filepath.Dir(dir)) // she only deletes inside your home folder
	for k, v := range map[string]string{"breakupLevel": "0", "minBossFightsWon": "0", "minLinesCommitted": "0", "toxicThreshold": "1"} {
		gitIn(t, dir, "config", "cursed."+k, v)
	}
	commitFile(t, dir, "story.txt", "once upon a time\n", "how we met, in a file")
	if toxic {
		commitFile(t, dir, "x.txt", "x\n", "wip") // a lazy message: +1 toxicity
	}
	return dir
}

func TestBreakupDryRun(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	out := mustRun(t, "breakup", "--dry-run")
	if !strings.Contains(out, "Ending A, too soon") || !strings.Contains(out, "level 0/10") {
		t.Errorf("dry run:\n%s", out)
	}
}

func TestBreakupNeedsATerminal(t *testing.T) {
	newRepo(t, "psycho") // TestMain: nobody at a terminal
	if code, _, errs := run("breakup"); code == 0 || !strings.Contains(errs, "terminal") {
		t.Errorf("exit %d %q", code, errs)
	}
}

func TestEndingATooSoon(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	os.MkdirAll(machine.desktop, 0755)
	before := machine.shutdowns
	_, _, errs := run("breakup")
	if !strings.Contains(errs, "I leave you") || !strings.Contains(errs, "Goodbye.") {
		t.Errorf("ending A:\n%s", errs)
	}
	letter := readFile(t, machine.desktop, "READ_THIS_BEFORE_YOU_TRY_AGAIN.txt")
	if !strings.Contains(letter, "You still need: level 10") {
		t.Errorf("the letter should say what you're missing:\n%s", letter)
	}
	copies, _ := filepath.Glob(filepath.Join(dir, ".git", "cursed", "letters", "*.txt"))
	if len(copies) != 1 {
		t.Errorf("a copy of the letter belongs in .git/cursed/letters, found %v", copies)
	}
	if machine.shutdowns != before {
		t.Error("no Death March: the shutdown is only an animation")
	}
	if _, _, errs := run("status"); !strings.Contains(errs, "READ_THIS_BEFORE_YOU_TRY_AGAIN.txt") {
		t.Errorf("the next command should open with the letter:\n%s", errs)
	}
	if _, _, errs := run("status"); strings.Contains(errs, "READ_THIS") {
		t.Error("...but only once")
	}
}

func TestEndingBWithoutDeathMarchDeletesNothing(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := eligible(t, h, true)
	_, _, errs := run("breakup")
	if !strings.Contains(errs, "not allowed") {
		t.Errorf("ending B without Death March:\n%s", errs)
	}
	if readFile(t, dir, "story.txt") != "once upon a time\n" {
		t.Error("nothing may be deleted without Death March")
	}
	if receipt := readFile(t, dir, "SHE_TOOK_EVERYTHING.txt"); !strings.Contains(receipt, "lazy commit message") {
		t.Errorf("the receipt should list the offences:\n%s", receipt)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "restraining order") {
		t.Errorf("she's gone after the breakup:\n%s", mood)
	}
}

func TestEndingBInDeathMarchTakesEverythingAndClosureBringsItBack(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := eligible(t, h, true)
	gitIn(t, dir, "config", "cursed.deathMarch", "true")
	h.input = "I accept the death march\n"
	run("status")
	h.input = ""

	run("breakup")
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "SHE_TOOK_EVERYTHING.txt" {
		t.Fatalf("Death March ending B leaves only the receipt, found %v", entries)
	}
	if receipt := readFile(t, dir, "SHE_TOOK_EVERYTHING.txt"); strings.Contains(receipt, "exes") || strings.Contains(receipt, ".tar.gz") {
		t.Errorf("the receipt must not say where the archive is:\n%s", receipt)
	}

	out := mustRun(t, "closure")
	if !strings.Contains(out, "Restored") {
		t.Errorf("closure:\n%s", out)
	}
	if readFile(t, dir, "story.txt") != "once upon a time\n" || readFile(t, dir, "SHE_TOOK_EVERYTHING.txt") != "<missing>" {
		t.Error("closure should bring every file back and remove the receipt")
	}
	gitIn(t, dir, "fsck", "--strict")
	if got := gitIn(t, dir, "log", "--format=%s"); !strings.Contains(got, "how we met") {
		t.Errorf("history should be back too:\n%s", got)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "restraining order") {
		t.Errorf("after closure she stays gone:\n%s", mood)
	}
}

func TestEndingBRefusesOutsideHome(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := eligible(t, h, true)
	t.Setenv("HOME", t.TempDir()) // the repo is not inside it
	gitIn(t, dir, "config", "cursed.deathMarch", "true")
	h.input = "I accept the death march\n"
	run("status")
	h.input = ""
	run("breakup")
	if readFile(t, dir, "story.txt") != "once upon a time\n" {
		t.Error("she must never delete outside your home folder")
	}
}

func TestEndingCSheLetsYouGo(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := eligible(t, h, false)
	h.input = "yes\nyes\nyes\nno\nno\nno\n"
	_, _, errs := run("breakup")
	if !strings.Contains(errs, "how we met, in a file") || !strings.Contains(errs, "Commit often") {
		t.Errorf("ending C should plead with your history and end with the credits:\n%s", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "cursed")); err == nil {
		t.Error(".git/cursed should be gone")
	}
	if !strings.Contains(readFile(t, dir, ".git/cursed-memories/letter.txt"), "- her") {
		t.Error("she leaves a letter in .git/cursed-memories")
	}
	if readFile(t, dir, ".git/hooks/pre-commit") != "<missing>" {
		t.Error("her hooks should be gone")
	}
	trophies, _ := filepath.Glob(filepath.Join(os.Getenv("CURSED_GIT_HOME"), "trophies", "*"))
	if len(trophies) == 0 {
		t.Error("a good ending earns a trophy")
	}
	if _, _, errs := run("commit", "--allow-empty", "-m", "wip"); errs != "" {
		t.Errorf("after the good ending it's just git:\n%s", errs)
	}
	if _, _, mood := run("mood"); !strings.Contains(mood, "isn't in this repo") {
		t.Errorf("she's gone for good:\n%s", mood)
	}
}

func TestEndingCTakingTheDealCancelsIt(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := eligible(t, h, false)
	h.input = "yes\nyes\nyes\nyes\n"
	run("breakup")
	if _, err := os.Stat(filepath.Join(dir, ".git", "cursed")); err != nil {
		t.Error("taking her deal means staying")
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "♥") && !strings.Contains(mood, "temper -2") {
		t.Errorf("she won: love-bombing:\n%s", mood)
	}
}
