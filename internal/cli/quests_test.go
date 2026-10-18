package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// quest asks for the next quest and checks it's the one we expect.
func quest(t *testing.T, title string) {
	t.Helper()
	if out := mustRun(t, "quest"); !strings.Contains(out, title) {
		t.Fatalf("expected the quest %q:\n%s", title, out)
	}
}

// completes runs a command and checks it finished the quest.
func completes(t *testing.T, title string, args ...string) {
	t.Helper()
	_, _, errs := run(args...)
	if !strings.Contains(errs, "Quest complete: "+title) || !strings.Contains(errs, "What you learned:") {
		t.Fatalf("cg %v should complete %q:\n%s", args, title, errs)
	}
}

// The whole learning path, in order, the way a player would do it.
func TestEveryQuestInOrder(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy") // Tier 1: no fights, so the conflict quest is a plain conflict
	commitFile(t, dir, "song.txt", "la la la\n", "the first song")

	quest(t, "Branch Out")
	if _, _, errs := run("status"); strings.Contains(errs, "Quest complete") {
		t.Fatal("status doesn't complete anything")
	}
	completes(t, "Branch Out", "switch", "-c", "verse")

	quest(t, "Peek Inside")
	completes(t, "Peek Inside", "cat-file", "-p", "HEAD")

	quest(t, "Bring It Home")
	commitFile(t, dir, "verse.txt", "a verse\n", "a verse on its own branch")
	mustRun(t, "switch", "main")
	completes(t, "Bring It Home", "merge", "verse")

	quest(t, "Tag It")
	completes(t, "Tag It", "tag", "v0.1")

	quest(t, "Undo, Keep the Work")
	commitFile(t, dir, "oops.txt", "oops\n", "this one was a mistake")
	completes(t, "Undo, Keep the Work", "reset", "--soft", "HEAD~1")

	quest(t, "Second Thoughts")
	mustRun(t, "commit", "-m", "this one was fine after all")
	completes(t, "Second Thoughts", "commit", "--amend", "-m", "this one is fine, really")

	quest(t, "Pick a Cherry")
	mustRun(t, "switch", "verse")
	commitFile(t, dir, "chorus.txt", "a chorus\n", "a chorus worth stealing")
	chorus := strings.TrimSpace(gitIn(t, dir, "rev-parse", "HEAD"))
	mustRun(t, "switch", "main")
	completes(t, "Pick a Cherry", "cherry-pick", chorus)

	quest(t, "Hide It")
	write(t, dir, map[string]string{"song.txt": "la la la, unfinished\n"})
	mustRun(t, "stash")
	completes(t, "Hide It", "stash", "pop")
	mustRun(t, "restore", "song.txt")

	quest(t, "Bring It Back")
	os.Remove(filepath.Join(dir, "song.txt"))
	completes(t, "Bring It Back", "restore", "song.txt")

	quest(t, "Survive a Conflict")
	mustRun(t, "switch", "-c", "remix")
	commitFile(t, dir, "song.txt", "la la LA (remix)\n", "the remix version")
	mustRun(t, "switch", "main")
	commitFile(t, dir, "song.txt", "la la la (acoustic)\n", "the acoustic version")
	run("merge", "remix")
	write(t, dir, map[string]string{"song.txt": "la la la (acoustic remix)\n"})
	mustRun(t, "add", "song.txt")
	completes(t, "Survive a Conflict", "commit", "-m", "both versions, finally")

	quest(t, "Rewrite the Road")
	mustRun(t, "switch", "verse")
	commitFile(t, dir, "bridge.txt", "a bridge\n", "a bridge on the verse branch")
	_, _, errs := run("rebase", "main")
	if !strings.Contains(errs, "Quest complete: Rewrite the Road") {
		t.Fatalf("rebase should complete the last quest:\n%s", errs)
	}
	if !strings.Contains(errs, "That was the last one") || !strings.Contains(errs, "Achievement unlocked: Professor") {
		t.Errorf("finishing every quest is worth saying (and an achievement):\n%s", errs)
	}
	if out := mustRun(t, "quest"); !strings.Contains(out, "No quests left") {
		t.Errorf("after the last quest:\n%s", out)
	}
	if mood := mustRun(t, "achievements"); !strings.Contains(mood, "🏆 Scholar") {
		t.Errorf("5 quests should have unlocked Scholar:\n%s", mood)
	}
}

func TestSkippingAQuest(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	commitFile(t, dir, "a.txt", "a\n", "a file to start with")
	quest(t, "Branch Out")
	if out := mustRun(t, "quest", "skip"); !strings.Contains(out, "Peek Inside") {
		t.Errorf("skip should move on:\n%s", out)
	}
	if _, _, errs := run("switch", "-c", "late"); strings.Contains(errs, "Quest complete") {
		t.Error("a skipped quest pays nothing")
	}
}

func TestQuestsNeedThePlayer(t *testing.T) {
	dir := newRepo(t, "clingy") // nobody at a terminal: no quest checking, no XP
	commitFile(t, dir, "a.txt", "a\n", "a file to start with")
	mustRun(t, "quest")
	if _, _, errs := run("switch", "-c", "x"); strings.Contains(errs, "Quest") {
		t.Errorf("scripts don't play:\n%s", errs)
	}
}
