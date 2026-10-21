package curses

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)

func at(d time.Duration) int64 { return t0.Add(d).Unix() }

// The formula from docs/CURSES.md §2, chapter by chapter (the same story as her-tier.html).
func TestTierFormula(t *testing.T) {
	on := Event{T: at(0), Type: "on", Text: "psycho", Tier: 3, Max: 4}
	cases := []struct {
		name   string
		events []Event
		want   int
	}{
		{"default: psycho starts at 3", []Event{on}, 3},
		{"temper +1 reaches tier 4", []Event{on, {T: at(0), Type: "temper", Delta: 1}}, 4},
		{"temper can't go past the cap", []Event{on, {T: at(0), Type: "temper", Delta: 5}}, 4},
		{"calming down stops at the floor (base-2)", []Event{on, {T: at(0), Type: "temper", Delta: -9}}, 1},
		{"your toxicity makes her one tier worse", []Event{on, {T: at(0), Type: "temper", Delta: -2}, {T: at(0), Type: "toxicity", Delta: 10}}, 2},
		{"clingy: cap 2 even when furious", []Event{{T: at(0), Type: "on", Text: "clingy", Tier: 1, Max: 2}, {T: at(0), Type: "temper", Delta: 2}}, 2},
	}
	for _, c := range cases {
		if got := Replay(c.events).Tier(t0); got != c.want {
			t.Errorf("%s: tier %d, want %d", c.name, got, c.want)
		}
	}
}

func TestTemperCoolsDown(t *testing.T) {
	s := Replay([]Event{{T: at(0), Type: "on", Text: "psycho", Tier: 3, Max: 4}, {T: at(0), Type: "temper", Delta: 2}})
	for _, c := range []struct {
		after time.Duration
		want  int
	}{{0, 2}, {time.Hour, 2}, {2 * time.Hour, 1}, {4 * time.Hour, 0}, {48 * time.Hour, 0}} {
		if got := s.Temper(t0.Add(c.after)); got != c.want {
			t.Errorf("after %v: temper %d, want %d", c.after, got, c.want)
		}
	}
}

func TestDiaryChain(t *testing.T) {
	gitDir := filepath.Join(t.TempDir(), ".git")
	os.MkdirAll(filepath.Join(gitDir, "objects"), 0755)
	os.MkdirAll(filepath.Join(gitDir, "refs"), 0755)
	d, err := OpenDiary(gitDir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if err := d.Append(Event{T: at(time.Duration(i) * time.Minute), Type: "grudge", Text: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := OpenDiary(gitDir); err != nil {
		t.Fatalf("an untouched diary must verify: %v", err)
	}

	// cut the last line off: the chain still holds, but her heart remembers more
	log := filepath.Join(gitDir, "cursed", "events.log")
	data, _ := os.ReadFile(log)
	lines := splitLines(data)
	os.WriteFile(log, []byte(lines[0]+"\n"+lines[1]+"\n"), 0644)
	if _, err := OpenDiary(gitDir); !errors.Is(err, ErrTampered) {
		t.Errorf("a removed last line wasn't noticed: %v", err)
	}
	// remove a line from the middle: the chain breaks
	os.WriteFile(log, []byte(lines[0]+"\n"+lines[2]+"\n"), 0644)
	if _, err := OpenDiary(gitDir); !errors.Is(err, ErrTampered) {
		t.Errorf("a removed middle line wasn't noticed: %v", err)
	}
}

func splitLines(b []byte) []string {
	var out []string
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, string(b[start:i]))
			start = i + 1
		}
	}
	return out
}
