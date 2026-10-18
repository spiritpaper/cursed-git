// internal/curses/card.go
//
// `cg card`: your relationship on one screen, made for screenshots (and event leaderboards).
package curses

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const cardWidth = 50

// Card is the relationship card, line by line.
func (e *Engine) Card(now time.Time) []string {
	st := e.st
	tier := st.Tier(now)
	face := miniFaces[tier]
	status := fmt.Sprintf("Tier %d · %s", tier, TierName(tier))
	switch {
	case !st.On:
		face, status = "( -_- )", "restraining order"
	case st.Lovebombing(now):
		face = "( ♥_♥ )"
	}
	lv := Level(st.XP)
	need := rules(e.gitDir).BreakupLevel
	full := min(10, lv*10/max(1, need))
	days := 0
	if st.OnSince > 0 {
		days = int(now.Sub(time.Unix(st.OnSince, 0)).Hours() / 24)
	}
	rows := []string{
		"CURSED GIT · relationship card",
		fmt.Sprintf("%s & her · %s", e.firstName(), filepath.Base(e.workTree)),
		"",
		fmt.Sprintf("%-28s%s", status, face),
		fmt.Sprintf("Closure     Lv %-3d %s  %d XP", lv, strings.Repeat("▓", full)+strings.Repeat("░", 10-full), st.XP),
		fightsLine(st),
		fmt.Sprintf("Toxicity    %d/%d", st.Toxicity, rules(e.gitDir).ToxicThreshold),
		fmt.Sprintf("Together    %s · %s", plural(days, "day"), plural(st.Commits, "commit")),
	}
	if left, ok := e.EventLeft(now); ok {
		ev := "over"
		if left > 0 {
			ev = fmt.Sprintf("%dh %02dm left", int(left.Hours()), int(left.Minutes())%60)
		}
		rows = append(rows, "Event       "+ev)
	}
	got := e.Unlocked()
	ach := fmt.Sprintf("Achievements %d/%d", len(got), len(Achievements))
	var latest string
	var latestAt int64 = -1
	for _, a := range Achievements {
		if at, ok := got[a.ID]; ok && at >= latestAt {
			latest, latestAt = a.Name, at
		}
	}
	if latest != "" {
		ach += " · latest: " + latest
	}
	rows = append(rows, ach)
	if h := e.History(); h.Projects > 1 {
		rows = append(rows, fmt.Sprintf("Across      %d projects", h.Projects))
	}
	if n := len(st.Grudges); n > 0 {
		rows = append(rows, "", "She remembers: "+st.Grudges[n-1])
	}
	if e.DeathMarch() {
		rows = append(rows, "", "☠  DEATH MARCH")
	}
	out := []string{"┌" + strings.Repeat("─", cardWidth+2) + "┐"}
	for _, r := range rows {
		for _, line := range wrap(r, cardWidth) {
			out = append(out, "│ "+line+strings.Repeat(" ", cardWidth-utf8.RuneCountInString(line))+" │")
		}
	}
	return append(out, "└"+strings.Repeat("─", cardWidth+2)+"┘")
}

// wrap breaks a line at spaces so it fits the card.
func wrap(s string, width int) []string {
	if utf8.RuneCountInString(s) <= width {
		return []string{s}
	}
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		switch {
		case line == "":
			line = w
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) <= width:
			line += " " + w
		default:
			lines = append(lines, line)
			line = w
		}
	}
	for utf8.RuneCountInString(line) > width { // one very long word
		r := []rune(line)
		lines = append(lines, string(r[:width]))
		line = string(r[width:])
	}
	return append(lines, line)
}

// CardJSON is the card as data, for event leaderboards (tools/leaderboard.html).
type CardJSON struct {
	Format       int      `json:"cg_card"`
	Player       string   `json:"player"`
	Project      string   `json:"project"`
	Tier         int      `json:"tier"`
	Level        int      `json:"level"`
	XP           int      `json:"xp"`
	Lines        int      `json:"lines"`
	FightsWon    int      `json:"fights_won"`
	FightsLost   int      `json:"fights_lost"`
	Fastest      int      `json:"fastest_fight_seconds,omitempty"`
	Toxicity     int      `json:"toxicity"`
	Commits      int      `json:"commits"`
	Achievements []string `json:"achievements"`
	Ending       string   `json:"ending,omitempty"`
	DeathMarch   bool     `json:"death_march,omitempty"`
	EventHours   int      `json:"event_hours,omitempty"`
	At           int64    `json:"at"`
}

// JSON is the card for machines.
func (e *Engine) JSON(now time.Time) []byte {
	st := e.st
	c := CardJSON{Format: 1, Player: e.firstName(), Project: filepath.Base(e.workTree), Tier: st.Tier(now),
		Level: Level(st.XP), XP: st.XP, Lines: st.Lines, FightsWon: st.BossWon, FightsLost: st.BossLost,
		Fastest: st.FastestFight, Toxicity: st.Toxicity, Commits: st.Commits, Achievements: []string{},
		DeathMarch: e.DeathMarch(), EventHours: EventHours(e.gitDir), At: now.Unix()}
	got := e.Unlocked()
	for _, a := range Achievements {
		if _, ok := got[a.ID]; ok {
			c.Achievements = append(c.Achievements, a.Name)
		}
	}
	if m, ok := recall(e.gitDir); ok {
		c.Ending = m.Ending
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return data
}

func fightsLine(st *State) string {
	line := fmt.Sprintf("Boss fights %d won · %d lost", st.BossWon, st.BossLost)
	if st.FastestFight > 0 {
		line += fmt.Sprintf(" · best %s", clock(int64(st.FastestFight)))
	}
	return line
}
