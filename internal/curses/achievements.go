// internal/curses/achievements.go
//
// Achievements. They unlock while her diary is replayed, at the moment the event happened, so
// they're exactly as trustworthy as everything else she knows: faking one means editing the
// diary, which breaks its hash chain ("you read my diary"). Two of them look across projects
// (her memory file) instead.
//
// Some are proud moments, some are shame. Both go on the card.
package curses

import (
	"strings"
	"time"
)

// Achievement is one badge.
type Achievement struct {
	ID, Name, Desc string
}

// Achievements, in the order `cg achievements` lists them.
var Achievements = []Achievement{
	{"official", "It's Official", "your first commit with her"},
	{"first_blood", "First Blood", "win your first boss fight"},
	{"speedrun", "Speedrun", "win a boss fight in 30 seconds or less"},
	{"untouchable", "Untouchable", "win 3 boss fights in a row"},
	{"chad_slayer", "Chad Slayer", "beat the rival who came from a remote (a pull)"},
	{"therapy", "Therapy", "beat Past You (a conflict with your own branch)"},
	{"rebase_survivor", "Rebase Survivor", "win a boss fight in the middle of a rebase"},
	{"phase_two", "Phase 2", "win the Tier 4 rematch"},
	{"good_partner", "Good Partner", "9 good commit messages in a row"},
	{"sweet_talker", "Sweet Talker", "make her fall for you (love-bombing)"},
	{"halfway", "Halfway There", "reach closure level 5"},
	{"night_shift", "Night Shift", "commit between 1 and 5 in the morning"},
	{"on_the_record", "On the Record", "commit the apology she dictated"},
	{"lamb", "Sacrificial Lamb", "lose a commit to her sacrifice"},
	{"pinky_promise", "Pinky Promise", "do what she asked, and find out what her promises are worth"},
	{"grave_robber", "Grave Robber", "beg for your files back 3 times"},
	{"ghosted", "Ghosted", "ignore 3 of her deadline warnings"},
	{"caught", "Caught Red-Handed", "get caught using plain git behind her back"},
	{"toxic_twins", "Toxic Twins", "become as toxic as she is"},
	{"death_marcher", "Death Marcher", "swear the Death March oath"},
	{"favourite", "Favourite", "top her scoreboard over a teammate (cg rivals)"},
	{"bingo", "Bingo", "see every one of her red flags (🚩 11/11)"},
	{"scholar", "Scholar", "finish 5 quests (cg quest)"},
	{"professor", "Professor", "finish every quest"},
	{"serial_dater", "Serial Dater", "play with her in 5 different projects"},
	{"every_costume", "Seen Them All", "she came back after it was over, every way she could. You know them now."},
	{"heartbreaker", "Heartbreaker", "reach the good ending, in any project"},
}

// unlock records an achievement the first time it happens.
func (s *State) unlock(id string, at int64) {
	if _, ok := s.Unlocked[id]; !ok {
		s.Unlocked[id] = at
	}
}

// checkUnlocks runs during replay, after each event has been applied. apologyBefore is the
// dictated apology that was pending just before this event.
func (s *State) checkUnlocks(e Event, apologyBefore string) {
	switch e.Type {
	case "commit":
		s.unlock("official", e.T)
		if h := time.Unix(e.T, 0).Hour(); h >= 1 && h < 5 {
			s.unlock("night_shift", e.T)
		}
		if apologyBefore != "" {
			s.unlock("on_the_record", e.T)
		}
	case "fight_won":
		f := s.LastFight
		s.unlock("first_blood", e.T)
		if s.winStreak >= 3 {
			s.unlock("untouchable", e.T)
		}
		if f == nil {
			break
		}
		if f.Seconds-e.Delta <= 30 {
			s.unlock("speedrun", e.T)
		}
		if f.Remote || strings.HasPrefix(f.Boss, "Chad from ") {
			s.unlock("chad_slayer", e.T)
		}
		if f.Boss == "Past You" {
			s.unlock("therapy", e.T)
		}
		if f.Branch == "your rebase" {
			s.unlock("rebase_survivor", e.T)
		}
		if f.Phase == 2 {
			s.unlock("phase_two", e.T)
		}
	case "good":
		if s.GoodStreak >= 9 {
			s.unlock("good_partner", e.T)
		}
	case "lovebomb":
		s.unlock("sweet_talker", e.T)
	case "xp", "xp_paid":
		if Level(s.XP) >= 5 {
			s.unlock("halfway", e.T)
		}
	case "sacrifice":
		s.unlock("lamb", e.T)
	case "promise_broken":
		s.unlock("pinky_promise", e.T)
	case "beg":
		if s.begs >= 3 {
			s.unlock("grave_robber", e.T)
		}
	case "warning":
		if s.IgnoredWarns >= 3 {
			s.unlock("ghosted", e.T)
		}
	case "toxicity":
		if strings.Contains(e.Text, "behind my back") {
			s.unlock("caught", e.T)
		}
		if s.Toxicity >= ToxicThreshold {
			s.unlock("toxic_twins", e.T)
		}
	case "deathmarch_on":
		s.unlock("death_marcher", e.T)
	case "favourite":
		s.unlock("favourite", e.T)
	case "flag":
		if len(s.Flags) >= RedFlags {
			s.unlock("bingo", e.T)
		}
	case "quest_done":
		if s.questsFinished >= 5 {
			s.unlock("scholar", e.T)
		}
		if s.questsFinished >= len(Quests) {
			s.unlock("professor", e.T)
		}
	}
}

// Unlocked is every achievement you have, with when it happened (0 = known, time unknown).
func (e *Engine) Unlocked() map[string]int64 {
	out := map[string]int64{}
	for id, at := range e.st.Unlocked {
		out[id] = at
	}
	projects := 0
	for _, m := range loadMemories() {
		projects++
		if m.Ending == "C" {
			if _, ok := out["heartbreaker"]; !ok {
				out["heartbreaker"] = 0
			}
		}
	}
	if len(loadHoover().Seen) >= len(costumes) {
		if _, ok := out["every_costume"]; !ok {
			out["every_costume"] = 0
		}
	}
	if projects >= 5 {
		if _, ok := out["serial_dater"]; !ok {
			out["serial_dater"] = 0
		}
	}
	return out
}

// AnnounceNew announces what unlocked since this engine was opened (for commands that don't
// end with Touch, like init and curse on).
func (e *Engine) AnnounceNew() {
	if e.awake() {
		e.announce(e.achBefore)
		e.achBefore = e.Unlocked()
	}
}

// announce tells you about achievements unlocked since `before`.
func (e *Engine) announce(before map[string]int64) {
	now := e.Unlocked()
	for _, a := range Achievements {
		if _, had := before[a.ID]; had {
			continue
		}
		if _, has := now[a.ID]; has {
			e.env.Out.Write([]byte("🏆  Achievement unlocked: " + a.Name + " (" + a.Desc + ")\n"))
		}
	}
}
