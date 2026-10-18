package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAchievementsUnlockAndAnnounceOnce(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", "a.txt")
	_, _, errs := run("commit", "-m", "the very first one")
	if !strings.Contains(errs, "🏆  Achievement unlocked: It's Official") {
		t.Errorf("the first commit should unlock It's Official:\n%s", errs)
	}
	commitFile(t, dir, "b.txt", "b\n", "the second one")
	if _, _, errs := run("status"); strings.Contains(errs, "Achievement") {
		t.Errorf("an achievement is announced once:\n%s", errs)
	}
	out := mustRun(t, "achievements")
	if !strings.Contains(out, "Achievements: 1/") || !strings.Contains(out, "🏆 It's Official") || !strings.Contains(out, "·  First Blood") {
		t.Errorf("the list should show unlocked and locked ones:\n%s", out)
	}
	if card := mustRun(t, "card"); !strings.Contains(card, "latest: It's Official") {
		t.Errorf("the card should show the latest:\n%s", card)
	}
}

func TestBossFightAchievements(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := arena(t, h, "psycho")
	run("merge", "feature")
	h.wait(20 * time.Second) // quick: Speedrun
	resolve(t, dir)
	mustRun(t, "add", "song.txt", "notes.txt")
	_, _, errs := run("commit", "-m", "settled the song")
	for _, want := range []string{"First Blood", "Speedrun", "Therapy"} {
		if !strings.Contains(errs, "Achievement unlocked: "+want) {
			t.Errorf("winning fast against Past You should unlock %s:\n%s", want, errs)
		}
	}
}

func TestShameAchievements(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	commitFile(t, dir, "a.txt", "a\n", "a file v1")
	plainGit(t, dir, "commit", "--allow-empty", "-q", "-m", "sneaky")
	_, _, errs := run("status")
	if !strings.Contains(errs, "Caught Red-Handed") {
		t.Errorf("plain git behind her back is an achievement too (a shameful one):\n%s", errs)
	}
}

func TestSerialDaterAcrossProjects(t *testing.T) {
	withHer(t, tuesday, lucky)
	base := t.TempDir()
	var errs string
	for _, name := range []string{"one", "two", "three", "four", "five"} {
		dir := filepath.Join(base, name)
		os.MkdirAll(dir, 0755)
		t.Chdir(dir)
		_, _, errs = run("init", "--preset", "clingy")
	}
	if !strings.Contains(errs, "Serial Dater") {
		t.Errorf("the fifth project should unlock Serial Dater:\n%s", errs)
	}
}
