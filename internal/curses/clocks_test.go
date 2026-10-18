package curses

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func clocksWith(t *testing.T, cfg string) Clocks {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config"), []byte(cfg), 0644)
	return ClocksFor(dir)
}

func TestDefaultClocks(t *testing.T) {
	c := clocksWith(t, "")
	if c.Deadline(3) != 30*time.Minute || c.Deadline(4) != 15*time.Minute || c.Deadline(0) != 0 {
		t.Errorf("deadlines: %v %v %v", c.Deadline(3), c.Deadline(4), c.Deadline(0))
	}
	if c.Fight(3) != 3*time.Minute || c.Fight(2) != 0 || c.Decay(3) != 30*24*time.Hour {
		t.Errorf("fight %v / %v, decay %v", c.Fight(3), c.Fight(2), c.Decay(3))
	}
}

func TestSpeedMakesEverythingFaster(t *testing.T) {
	c := clocksWith(t, "[cursed]\n  speed = 2\n")
	if c.Deadline(3) != 15*time.Minute || c.Fight(4) != 45*time.Second || c.Every(6*time.Hour) != 3*time.Hour {
		t.Errorf("speed 2: deadline %v, fight %v, charm %v", c.Deadline(3), c.Fight(4), c.Every(6*time.Hour))
	}
	if c.Deadline(0) != 0 {
		t.Error("a clock that's off at a tier stays off")
	}
}

func TestOneClockByHand(t *testing.T) {
	c := clocksWith(t, "[cursed]\n  speed = 2\n  deadline = 20m\n  fightTime = 1m\n")
	if c.Deadline(3) != 20*time.Minute || c.Deadline(4) != 10*time.Minute || c.Deadline(2) != 40*time.Minute {
		t.Errorf("20m at Tier 3, proportional elsewhere, speed ignored: %v %v %v", c.Deadline(2), c.Deadline(3), c.Deadline(4))
	}
	if c.Fight(3) != time.Minute || c.Fight(4) != 30*time.Second {
		t.Errorf("fight: %v %v", c.Fight(3), c.Fight(4))
	}
}

func TestEventsRotFasterSoItCanHappen(t *testing.T) {
	c := clocksWith(t, "[cursed]\n  eventHours = 24\n")
	if c.Decay(3) != 8*time.Hour {
		t.Errorf("a 24h event: decay in a third of it, got %v", c.Decay(3))
	}
	if c := clocksWith(t, "[cursed]\n  eventHours = 24\n  decay = 2h\n"); c.Decay(3) != 2*time.Hour {
		t.Errorf("an explicit decay wins: %v", c.Decay(3))
	}
	for in, want := range map[string]time.Duration{"30d": 30 * 24 * time.Hour, "2d12h": 60 * time.Hour, "90m": 90 * time.Minute} {
		if d, ok := parseDuration(in); !ok || d != want {
			t.Errorf("parseDuration(%q) = %v %v", in, d, ok)
		}
	}
	if _, ok := parseDuration("soon"); ok {
		t.Error("nonsense isn't a duration")
	}
}
