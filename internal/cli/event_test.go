package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const hackIni = `# Cursed Hack: the organizer's event file
[event]
    name = Cursed Hack 2026
    submit = "the #leaderboard channel"
    contact = ask Sam at the front desk
[cursed]
    preset = hackathon
    eventHours = 24
    eventEnds = %s
    minLinesCommitted = 300
    deathMarch = true
`

func TestJoiningAnEventFromAFile(t *testing.T) {
	withHer(t, tuesday, lucky)
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustRun(t, "init", "-q", "--plain")
	ends := tuesday.Add(6 * time.Hour).Format("2006-01-02 15:04")
	file := filepath.Join(t.TempDir(), "hack.ini")
	os.WriteFile(file, []byte(fmt.Sprintf(hackIni, ends)), 0644)

	code, out, errs := run("event", "join", file)
	if code != 0 {
		t.Fatalf("join: exit %d\n%s%s", code, out, errs)
	}
	if !strings.Contains(out, "Cursed Hack 2026") || !strings.Contains(out, "⏳ 6h 00m left") || !strings.Contains(out, "300 lines") {
		t.Errorf("joining should show the event, the shared clock and the goals:\n%s", out)
	}
	if !strings.Contains(errs, "can't: only you can") {
		t.Errorf("an event file must not turn on Death March:\n%s", errs)
	}
	if got, err := plainGit(t, dir, "config", "--get", "cursed.deathMarch"); err == nil {
		t.Errorf("deathMarch got set to %q", got)
	}
	if !strings.Contains(errs, "How to play with Miranda (preset hackathon") {
		t.Errorf("she should move in with the event's preset:\n%s", errs)
	}
	out = mustRun(t, "event")
	if !strings.Contains(out, "the #leaderboard channel") || !strings.Contains(out, "ask Sam") {
		t.Errorf("cg event says where to hand in and who to ask:\n%s", out)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "Event: 6h 00m left") {
		t.Errorf("the shared end time drives the clock:\n%s", mood)
	}
}

func TestJoiningAnEventFromAURL(t *testing.T) {
	withHer(t, tuesday, lucky)
	pinIdentity(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "[event]\n  name = Gentle Jam\n[cursed]\n  preset = gentle\n  eventHours = 8\n")
	}))
	defer srv.Close()
	t.Chdir(t.TempDir())
	mustRun(t, "init", "-q", "--plain")
	_, out, errs := run("event", "join", srv.URL+"/jam.ini")
	if !strings.Contains(out, "Gentle Jam") || !strings.Contains(errs, "I'm Juniper") {
		t.Errorf("a gentle event brings Juniper:\n%s%s", out, errs)
	}
}

func TestNotInAnEvent(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	if out := mustRun(t, "event"); !strings.Contains(out, "isn't part of an event") {
		t.Errorf("%s", out)
	}
	file := filepath.Join(t.TempDir(), "junk.ini")
	os.WriteFile(file, []byte("[user]\n  name = mallory\n"), 0644)
	if code, _, errs := run("event", "join", file); code == 0 || !strings.Contains(errs, "nothing in that file") {
		t.Errorf("only event settings are applied: exit %d %q", code, errs)
	}
}

// A one-day weekend jam: everything faster.
func TestWeekendJamClocks(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustRun(t, "init", "-q", "--plain")
	file := filepath.Join(t.TempDir(), "jam.ini")
	os.WriteFile(file, []byte("[event]\n  name = Weekend Jam\n[cursed]\n  preset = psycho\n  eventHours = 8\n  speed = 3\n  fightTime = 1m\n"), 0644)
	_, out, _ := run("event", "join", file)
	if !strings.Contains(out, "clocks: commit within 10m, 1m per conflict block, untouched files rot in 2h40m") {
		t.Errorf("cg event should show the jam's clocks:\n%s", out)
	}
	commitFile(t, dir, "a.txt", "a\n", "a file to start with")
	write(t, dir, map[string]string{"a.txt": "unsaved\n"})
	run("status")            // she sees it: her deadline clock starts now
	h.wait(11 * time.Minute) // 30m / speed 3 = 10m
	if _, _, errs := run("status"); !strings.Contains(errs, "minutes") {
		t.Errorf("speed 3: the deadline warning comes after 10 minutes:\n%s", errs)
	}
}
