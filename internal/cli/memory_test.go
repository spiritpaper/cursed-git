package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// a project with history between you: one lazy commit (toxicity 1, a grudge)
func withHistory(t *testing.T) string {
	t.Helper()
	dir := newRepo(t, "psycho")
	commitFile(t, dir, "a.txt", "a\n", "fix") // rewritten by her, but she remembers it
	return dir
}

func TestDeletingGitDoesntMakeHerForget(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := withHistory(t)
	os.RemoveAll(filepath.Join(dir, ".git"))
	_, _, errs := run("init")
	if !strings.Contains(errs, "new .git") && !strings.Contains(errs, "remember") {
		t.Errorf("she should recognise the folder:\n%s", errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 1/10") || !strings.Contains(mood, `"fix"`) {
		t.Errorf("toxicity and grudges come back:\n%s", mood)
	}
}

func TestAFreshCloneIsStillHer(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := withHistory(t)
	clone := t.TempDir()
	gitIn(t, clone, "clone", "-q", dir, ".")
	t.Chdir(clone)
	_, _, errs := run("curse", "on")
	if !strings.Contains(errs, "remember") && !strings.Contains(errs, "new .git") {
		t.Errorf("a clone has the same first commit: she knows it:\n%s", errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 1/10") {
		t.Errorf("the clone should inherit her feelings:\n%s", mood)
	}
}

func TestShesJealousOfYourOtherProjects(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	home := newRepo(t, "clingy")
	commitFile(t, home, "a.txt", "a\n", "our project")
	other := filepath.Join(t.TempDir(), "side-quest")
	os.MkdirAll(other, 0755)
	t.Chdir(other)
	mustRun(t, "init", "-q", "--preset", "clingy")
	h.wait(time.Minute)
	mustRun(t, "status")

	h.wait(time.Minute)
	t.Chdir(home)
	_, _, errs := run("status")
	if !strings.Contains(errs, "side-quest") {
		t.Errorf("coming back, she should mention where you were:\n%s", errs)
	}
	if _, _, errs := run("status"); strings.Contains(errs, "side-quest") {
		t.Error("...once, not every command")
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "Across 2 projects") || !strings.Contains(mood, "side-quest") {
		t.Errorf("mood should show the whole relationship and the grudge:\n%s", mood)
	}
}

func TestForget(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := withHistory(t)
	if out := mustRun(t, "forget"); !strings.Contains(out, "Forgotten") {
		t.Errorf("forget: %q", out)
	}
	os.RemoveAll(filepath.Join(dir, ".git"))
	mustRun(t, "init", "-q")
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 0/10") {
		t.Errorf("after forget, a new .git is a new start:\n%s", mood)
	}
}

func TestRelationshipCard(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	commitFile(t, dir, "a.txt", "a\n", "fix")
	card := mustRun(t, "card")
	for _, want := range []string{"relationship card", "dev & Miranda", "Tier 3 · Dangerous", "Toxicity    1/10", "1 commit", `"fix"`} {
		if !strings.Contains(card, want) {
			t.Errorf("card is missing %q:\n%s", want, card)
		}
	}
	lines := strings.Split(strings.TrimSpace(card), "\n")
	for _, l := range lines {
		if len([]rune(l)) != len([]rune(lines[0])) {
			t.Errorf("every line of the card should be as wide as the frame:\n%s", card)
			break
		}
	}
}

func TestHowToPlayWhenYouInviteHer(t *testing.T) {
	withHer(t, tuesday, lucky)
	t.Chdir(t.TempDir())
	_, _, errs := run("init", "--preset", "toxic")
	if !strings.Contains(errs, "How to play with Miranda (preset toxic: she starts at Tier 2, never goes above 3)") || !strings.Contains(errs, "cg restraining-order") {
		t.Errorf("init should explain the game:\n%s", errs)
	}
}
