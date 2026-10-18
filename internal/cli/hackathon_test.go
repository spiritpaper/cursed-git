package cli

import (
	"strings"
	"testing"
	"time"
)

func TestHackathonPresetScalesTheGame(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	t.Chdir(t.TempDir())
	_, _, errs := run("init", "--preset", "hackathon")
	if !strings.Contains(errs, "leave her at Lv 6") {
		t.Errorf("a 24h event: the how-to-play card should say Lv 6:\n%s", errs)
	}
	if got := strings.TrimSpace(gitIn(t, ".", "config", "cursed.eventHours")); got != "24" {
		t.Errorf("eventHours = %q, want 24", got)
	}
	if out := mustRun(t, "breakup", "--dry-run"); !strings.Contains(out, "level 0/6 · fights won 0/2 · lines 0/1200") {
		t.Errorf("goals scaled to 24h:\n%s", out)
	}
	commitFile(t, ".", "app.go", "package app\n\nfunc main() {}\n", "the hackathon project begins")
	h.wait(61 * time.Minute) // at an event, line XP pays after an hour, not a day
	mood := mustRun(t, "mood")
	if !strings.Contains(mood, "3/1200 lines") {
		t.Errorf("line XP should be paid after an hour:\n%s", mood)
	}
	if !strings.Contains(mood, "Event: 22h 59m left") {
		t.Errorf("mood should show the event clock:\n%s", mood)
	}
	if card := mustRun(t, "card"); !strings.Contains(card, "Event       22h 59m left") {
		t.Errorf("so should the card:\n%s", card)
	}
}

func TestShortEventAndOrganizerOverrides(t *testing.T) {
	withHer(t, tuesday, lucky)
	t.Chdir(t.TempDir())
	mustRun(t, "init", "-q", "--preset", "hackathon", "--hours", "8")
	if out := mustRun(t, "breakup", "--dry-run"); !strings.Contains(out, "level 0/3 · fights won 0/1 · lines 0/400") {
		t.Errorf("an 8h event:\n%s", out)
	}
	gitIn(t, ".", "config", "cursed.minLinesCommitted", "100") // the organizer's word is final
	if out := mustRun(t, "breakup", "--dry-run"); !strings.Contains(out, "lines 0/100") {
		t.Errorf("explicit settings win over the event defaults:\n%s", out)
	}
	if code, _, errs := run("curse", "on", "--hours", "soon"); code == 0 || !strings.Contains(errs, "number of hours") {
		t.Errorf("--hours must be a number: exit %d %q", code, errs)
	}
}
