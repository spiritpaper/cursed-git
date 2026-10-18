// internal/curses/clocks.go
//
// How fast the game runs. The defaults suit a project that lives for weeks: 30 minutes to
// commit at Tier 3, 3 minutes per conflict block, 30 days for a file to rot. An event (a
// weekend jam, a one-day hackathon) wants them faster, so they're all in .git/config:
//
//	[cursed]
//	    speed = 2          ; every clock twice as fast (0.5: a slow, relaxed week)
//	    deadline = 20m     ; or set one clock: its Tier 3 value (other tiers keep their proportions)
//	    fightTime = 2m
//	    decay = 6h
//
// A clock you set yourself is used as it is; speed only changes the ones you didn't set.
// With eventHours set (the hackathon preset), decay that can actually happen before the end:
// a third of the event.
package curses

import (
	"strconv"
	"time"

	"github.com/spiritpaper/cursed-git/internal/config"
)

// Clocks are this repo's timings.
type Clocks struct {
	speed                  float64
	deadline, fight, decay time.Duration // the Tier 3 values you set (0 = default)
}

// the Tier 3 defaults the other tiers are proportional to
const (
	baseDeadline = 30 * time.Minute
	baseFight    = 3 * time.Minute
	baseDecay    = 30 * 24 * time.Hour
)

// ClocksFor reads the clocks for a repository.
func ClocksFor(gitDir string) Clocks {
	cfg := config.Load(gitDir)
	c := Clocks{speed: 1}
	if s, err := strconv.ParseFloat(cfg.Get("cursed.speed"), 64); err == nil && s > 0 {
		c.speed = s
	}
	dur := func(key string) time.Duration {
		if d, err := time.ParseDuration(cfg.Get("cursed." + key)); err == nil && d > 0 {
			return d
		}
		return 0
	}
	c.deadline, c.fight, c.decay = dur("deadline"), dur("fightTime"), dur("decay")
	if c.decay == 0 {
		if h := EventHours(gitDir); h > 0 {
			c.decay = time.Duration(h) * time.Hour / 3
		}
	}
	return c
}

// scale: a tier's default, made proportional to the Tier 3 value you set, or sped up.
func (c Clocks) scale(def, base, custom time.Duration) time.Duration {
	if def == 0 {
		return 0 // off at this tier, whatever the clocks say
	}
	if custom > 0 {
		return time.Duration(float64(def) * float64(custom) / float64(base))
	}
	return time.Duration(float64(def) / c.speed)
}

// Deadline: how long uncommitted changes may sit at this tier.
func (c Clocks) Deadline(tier int) time.Duration {
	return c.scale(Deadline(tier), baseDeadline, c.deadline)
}

// Fight: the boss fight clock per conflict block.
func (c Clocks) Fight(tier int) time.Duration { return c.scale(HunkTime(tier), baseFight, c.fight) }

// Decay: how long until a neglected file is fully rotten.
func (c Clocks) Decay(tier int) time.Duration {
	return c.scale(DecayPeriod(tier), baseDecay, c.decay)
}

// Every: her own rhythms (the charm, the hoover), sped up with everything else.
func (c Clocks) Every(d time.Duration) time.Duration { return time.Duration(float64(d) / c.speed) }

func (e *Engine) clocks() Clocks { return ClocksFor(e.gitDir) }
