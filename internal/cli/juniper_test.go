package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func toxicityOf(t *testing.T) int {
	t.Helper()
	var c struct {
		Toxicity int `json:"toxicity"`
	}
	json.Unmarshal([]byte(mustRun(t, "card", "--json")), &c)
	return c.Toxicity
}

func juniperRepo(t *testing.T) (string, *her) {
	t.Helper()
	h := withHer(t, tuesday, lucky)
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	_, _, errs := run("init", "--preset", "gentle")
	if !strings.Contains(errs, "I'm Juniper") || !strings.Contains(errs, "no curses, no punishments") {
		t.Fatalf("the gentle preset should bring Juniper:\n%s", errs)
	}
	return dir, h
}

func TestJuniperNeverPunishes(t *testing.T) {
	dir, h := juniperRepo(t)
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", "a.txt")
	_, _, errs := run("commit", "-m", "fix")
	if strings.Contains(errs, "I fixed your commit message") {
		t.Errorf("Juniper never rewrites your message:\n%s", errs)
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s"); got != "fix\n" {
		t.Errorf("message changed to %q", got)
	}
	plainGit(t, dir, "commit", "--allow-empty", "-q", "-m", "plain git")
	if _, _, errs := run("status"); !strings.Contains(errs, "I just like being included") {
		t.Errorf("plain git gets a kind word:\n%s", errs)
	}
	if n := toxicityOf(t); n != 0 {
		t.Errorf("toxicity = %d, Juniper never holds anything against you", n)
	}

	write(t, dir, map[string]string{"a.txt": "unsaved work, hours of it\n"})
	h.wait(3 * time.Hour)
	run("status")
	if readFile(t, dir, "a.txt") != "unsaved work, hours of it\n" {
		t.Error("Juniper never takes anything")
	}
	if card := mustRun(t, "card"); !strings.Contains(card, "dev & Juniper") || !strings.Contains(card, "cherishing you") {
		t.Errorf("card:\n%s", card)
	}
}

func TestJuniperConflictIsNotAFight(t *testing.T) {
	dir, _ := juniperRepo(t)
	commitFile(t, dir, "song.txt", "a\n", "a song to start with")
	mustRun(t, "switch", "-c", "other")
	commitFile(t, dir, "song.txt", "b\n", "the other version of it")
	mustRun(t, "switch", "main")
	commitFile(t, dir, "song.txt", "c\n", "my version of the song")
	_, _, errs := run("merge", "other")
	if strings.Contains(errs, "BOSS FIGHT") || !strings.Contains(errs, "You've got this") {
		t.Errorf("a conflict with Juniper is help, not a fight:\n%s", errs)
	}
	if code, _, _ := run("merge", "--abort"); code != 0 {
		t.Error("and --abort just works")
	}
}

func TestYouCantSwapMirandaOut(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	if code, _, errs := run("partner", "juniper"); code == 0 || !strings.Contains(errs, "Break up with me") {
		t.Errorf("Miranda refuses to be replaced: exit %d\n%s", code, errs)
	}
	out := mustRun(t, "partner")
	if !strings.Contains(out, "♥ Miranda") || !strings.Contains(out, "Juniper    gentle") {
		t.Errorf("cg partner lists who's here and who else there is:\n%s", out)
	}
	mustRun(t, "restraining-order")
	if _, _, errs := run("partner", "juniper"); !strings.Contains(errs, "I'm Juniper") {
		t.Errorf("after the restraining order, Juniper can move in:\n%s", errs)
	}
	if out := mustRun(t, "partner"); !strings.Contains(out, "♥ Juniper") {
		t.Errorf("now she's the one:\n%s", out)
	}
	if _, _, errs := run("partner", "miranda"); !strings.Contains(errs, "Miranda") && !strings.Contains(errs, "Tier 3") {
		t.Errorf("you can always go back to the drama:\n%s", errs)
	}
}

func TestAfterTheGoodEndingJuniperIsWaiting(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := eligible(t, h, false)
	h.input = "yes\nyes\nyes\nno\nno\nno\n"
	_, _, errs := run("breakup")
	if !strings.Contains(errs, "cg partner juniper") {
		t.Fatalf("the good ending should introduce Juniper:\n%s", errs)
	}
	_, _, errs = run("partner", "juniper")
	if !strings.Contains(errs, "I'm Juniper") {
		t.Fatalf("Juniper moves in:\n%s", errs)
	}
	if n := toxicityOf(t); n != 0 {
		t.Errorf("she doesn't inherit Miranda's grudges: toxicity %d", n)
	}
	h.input = ""
	_, _, errs = run("breakup")
	if !strings.Contains(errs, "cheering for you") && !strings.Contains(errs, "glad I got to see you grow") {
		t.Errorf("leaving Juniper is a kind goodbye:\n%s", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "cursed")); err == nil {
		t.Error("and then she's gone, gently")
	}
}
