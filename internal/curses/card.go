// internal/curses/card.go
//
// `cg card`: your relationship on one screen, made for screenshots (and event leaderboards).
package curses

import (
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
		fmt.Sprintf("Boss fights %d won · %d lost", st.BossWon, st.BossLost),
		fmt.Sprintf("Toxicity    %d/%d", st.Toxicity, rules(e.gitDir).ToxicThreshold),
		fmt.Sprintf("Together    %s · %s", plural(days, "day"), plural(st.Commits, "commit")),
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
