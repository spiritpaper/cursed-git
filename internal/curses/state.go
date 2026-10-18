// internal/curses/state.go
package curses

import (
	"time"
)

// Presets: where she starts (base) and how far she may ever go (max). See docs/CURSES.md §2.3.
var Presets = map[string][2]int{
	"clingy": {1, 2},
	"toxic":  {2, 3},
	"psycho": {3, 4},
	// psycho, at an event: goals scale to cursed.eventHours (set by `--preset hackathon`)
	"hackathon": {3, 4},
}

const (
	DefaultPreset   = "psycho"
	ToxicThreshold  = 10
	TemperCooldown  = 2 * time.Hour
	LovebombLasts   = time.Hour
	RepeatWindow    = 24 * time.Hour
	warningsPerStep = 3 // ignored warnings per +1 temper
)

var tierNames = []string{"Safe", "Annoying", "Hostile", "Dangerous", "Catastrophic"}

func TierName(t int) string { return tierNames[clamp(t, 0, 4)] }

// State is what she knows, rebuilt from the diary every time.
type State struct {
	On                 bool
	Preset             string
	Base, Max          int
	temper             int
	lastTemperAt       int64
	Toxicity           int
	Grudges            []string
	GoodStreak         int
	LastCommitAt       int64 // also reset after a deadline punishment
	Commits            int   // commits she has seen
	DeadlineWarns      int   // warnings since the last commit
	LastWarnAt         int64
	IgnoredWarns       int
	KnownAuthors       map[string]bool
	Offenses           map[string]int64 // offense -> when it last happened
	LastRefusalAt      int64
	Graveyard          []string // refs she took that you haven't begged back yet
	Sacrificed         []string // refs/cursed/sacrificed/...: commits she dropped (cherry-pick brings one back)
	LovebombAt         int64
	OnSince            int64
	Fight              *Fight // the boss fight going on right now, if any
	LastFight          *Fight // the last one that ended
	BossWon            int
	BossLost           int
	Apology            string // a commit message she dictated after you lost (used by your next commit)
	XP                 int
	Lines              int             // committed lines that have been paid
	Pending            []pending       // line XP waiting to mature
	PaidBlobs          map[string]bool // content she has paid for (or is about to)
	DeathMarchAccepted bool
	LastShutdownAt     int64
	BreakupAttempts    int
	LetterPending      string           // ending A's letter, until she's mentioned it
	Unlocked           map[string]int64 // achievement -> when (achievements.go)
	winStreak, begs    int
	Quest              *QuestStart      // the quest you're on
	QuestsDone         map[string]int64 // finished or skipped
	questsFinished     int              // finished for real (not skipped)
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

// Replay rebuilds her state from the diary.
func Replay(events []Event) *State {
	s := &State{KnownAuthors: map[string]bool{}, Offenses: map[string]int64{}, PaidBlobs: map[string]bool{},
		Unlocked: map[string]int64{}, QuestsDone: map[string]int64{}}
	for _, e := range events {
		apologyBefore := s.Apology
		switch e.Type {
		case "on":
			s.On = true
			if e.Text != "" {
				s.Preset, s.Base, s.Max = e.Text, e.Tier, e.Max
			}
			if s.OnSince == 0 {
				s.OnSince = e.T
			}
			if s.LastCommitAt == 0 {
				s.LastCommitAt = e.T
			}
		case "carry":
			c := e.Carry
			s.On, s.Preset, s.Base, s.Max, s.Toxicity, s.Grudges = true, c.Preset, c.Base, c.Max, c.Toxicity, c.Grudges
			s.OnSince, s.LastCommitAt = e.T, e.T
		case "off":
			s.On = false
			s.DeathMarchAccepted = false // a restraining order ends the death march too
		case "temper":
			s.temper = clamp(s.temperAt(e.T)+e.Delta, -2, 2)
			s.lastTemperAt = e.T
		case "toxicity":
			s.Toxicity = max(0, s.Toxicity+e.Delta)
		case "grudge":
			s.Grudges = append(s.Grudges, e.Text)
		case "commit":
			s.LastCommitAt, s.DeadlineWarns, s.LastWarnAt = e.T, 0, 0
			s.Commits++
			s.Apology = ""
		case "fight":
			if e.Fight != nil {
				f := *e.Fight
				f.Start = e.T
				s.Fight = &f
			}
		case "fight_won":
			s.LastFight, s.Fight = s.Fight, nil
			s.BossWon++
			s.winStreak++
		case "fight_lost":
			s.LastFight, s.Fight = s.Fight, nil
			s.BossLost++
			s.winStreak = 0
		case "apology":
			s.Apology = e.Text
		case "xp":
			s.XP = max(0, s.XP+e.Delta)
		case "deathmarch_on":
			s.DeathMarchAccepted = true
		case "quest":
			s.Quest = e.Quest
		case "quest_done", "quest_skipped":
			s.QuestsDone[e.Text] = e.T
			if s.Quest != nil && s.Quest.ID == e.Text {
				s.Quest = nil
			}
			if e.Type == "quest_done" {
				s.questsFinished++
			}
		case "deathmarch_off":
			s.DeathMarchAccepted = false
		case "shutdown":
			s.LastShutdownAt = e.T
		case "breakup_attempt":
			s.BreakupAttempts++
			s.LetterPending = e.Text
		case "letter_seen":
			s.LetterPending = ""
		case "xp_pending":
			s.Pending = append(s.Pending, pending{Commit: e.Text, Lines: e.Delta, At: e.T, Blobs: e.Blobs})
			for _, b := range e.Blobs {
				s.PaidBlobs[b] = true
			}
		case "xp_paid", "xp_cancel":
			for i, p := range s.Pending {
				if p.Commit != e.Text {
					continue
				}
				s.Pending = append(s.Pending[:i], s.Pending[i+1:]...)
				if e.Type == "xp_paid" {
					s.XP += p.Lines
					s.Lines += p.Lines
				} else {
					for _, b := range p.Blobs {
						delete(s.PaidBlobs, b) // never paid: if it comes back and sticks, it pays then
					}
				}
				break
			}
		case "good":
			s.GoodStreak++
		case "lazy":
			s.GoodStreak = 0
		case "offense":
			s.Offenses[e.Text] = e.T
		case "warning":
			s.DeadlineWarns++
			s.LastWarnAt = e.T
			if s.DeadlineWarns > 1 {
				s.IgnoredWarns++
			}
		case "punished":
			s.LastCommitAt, s.DeadlineWarns, s.LastWarnAt = e.T, 0, 0
		case "graveyard":
			s.Graveyard = append(s.Graveyard, e.Text)
		case "sacrifice":
			s.Sacrificed = append(s.Sacrificed, e.Text)
		case "beg":
			s.begs++
			for i, ref := range s.Graveyard {
				if ref == e.Text {
					s.Graveyard = append(s.Graveyard[:i], s.Graveyard[i+1:]...)
					break
				}
			}
		case "author":
			s.KnownAuthors[e.Text] = true
		case "refusal":
			s.LastRefusalAt = e.T
		case "lovebomb":
			s.LovebombAt = e.T
		}
		s.checkUnlocks(e, apologyBefore)
	}
	return s
}

// temperAt applies the cooldown: one step toward 0 for every TemperCooldown without news.
func (s *State) temperAt(now int64) int {
	t := s.temper
	if s.lastTemperAt == 0 || t == 0 {
		return t
	}
	steps := int((now - s.lastTemperAt) / int64(TemperCooldown.Seconds()))
	for ; steps > 0 && t != 0; steps-- {
		if t > 0 {
			t--
		} else {
			t++
		}
	}
	return t
}

func (s *State) Temper(now time.Time) int { return s.temperAt(now.Unix()) }

// Grudge is +1 once YOUR toxicity reaches the threshold: "I learned it from you."
func (s *State) Grudge() int {
	if s.Toxicity >= ToxicThreshold {
		return 1
	}
	return 0
}

// Floor: she calms down two tiers below where she started, never further.
func (s *State) Floor() int { return max(0, s.Base-2) }

// Tier is her live tier: clamp(base + temper + grudge, floor, max).
func (s *State) Tier(now time.Time) int {
	return clamp(s.Base+s.Temper(now)+s.Grudge(), s.Floor(), s.Max)
}

// Lovebombing: right after temper hits -2 she's too nice for an hour. No curses.
func (s *State) Lovebombing(now time.Time) bool {
	return s.LovebombAt > 0 && now.Unix()-s.LovebombAt < int64(LovebombLasts.Seconds()) && s.Temper(now) == -2
}

// Deadline is how long uncommitted changes may sit before she acts, by live tier.
// 0 means the commit deadline curse is off at that tier.
func Deadline(tier int) time.Duration {
	return [...]time.Duration{0, 60 * time.Minute, 60 * time.Minute, 30 * time.Minute, 15 * time.Minute}[clamp(tier, 0, 4)]
}
