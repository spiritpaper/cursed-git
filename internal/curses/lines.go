// internal/curses/lines.go
//
// How she picks what to say: the approach Valve used for Left 4 Dead's dialogue (Elan Ruskin,
// GDC 2012). Every line can say *when* it applies; she plays the most specific line that
// matches what's going on, and plain lines are the fallback:
//
//	"lazy_reject": [
//	  "No.",                                                    <- always fits (specificity 0)
//	  {"text": "Work in progress. Like us.", "when": {"msg": ["wip"]}},          <- 1
//	  {"text": "At 3am? Really?", "when": {"msg": ["wip"], "hour": "1-5"}}       <- 2: wins at 3am
//	]
//
// Conditions: msg (words in the commit message), msg_emoji, tier ("3" or "3-4"),
// hour ("22-4" wraps past midnight), weekday, and min/max on facts (good_streak, toxicity,
// fights_won, fights_lost, commits, grudges, xp, level, temper).
//
// She also remembers what she said lately, per moment, so she doesn't repeat herself.
package curses

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Line is one thing she can say, and when.
type Line struct {
	Text string `json:"text"`
	When *When  `json:"when,omitempty"`
}

// When is the set of conditions a line needs. Every condition given must hold.
type When struct {
	Msg      []string       `json:"msg,omitempty"`
	MsgEmoji bool           `json:"msg_emoji,omitempty"`
	Tier     string         `json:"tier,omitempty"`
	Hour     string         `json:"hour,omitempty"`
	Weekday  []string       `json:"weekday,omitempty"`
	Min      map[string]int `json:"min,omitempty"`
	Max      map[string]int `json:"max,omitempty"`
}

// UnmarshalJSON accepts a plain string (the old format) or {"text": ..., "when": {...}}.
func (l *Line) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*l = Line{Text: s}
		return nil
	}
	type plain Line // no recursion into this method
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	if p.Text == "" {
		return errors.New("a line needs text")
	}
	*l = Line(p)
	return nil
}

func texts(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text
	}
	return out
}

// facts is what's true right now, for matching lines.
type facts struct {
	msg     string
	tier    int
	now     time.Time
	numbers map[string]int
}

func (e *Engine) facts(vars map[string]string) facts {
	now := e.env.Now()
	st := e.st
	msg := e.pre.msg
	if m, ok := vars["msg"]; ok {
		msg = m
	}
	return facts{msg: msg, tier: st.Tier(now), now: now, numbers: map[string]int{
		"good_streak": st.GoodStreak, "toxicity": st.Toxicity, "fights_won": st.BossWon,
		"fights_lost": st.BossLost, "commits": st.Commits, "grudges": len(st.Grudges),
		"xp": st.XP, "level": Level(st.XP), "temper": st.Temper(now),
	}}
}

// specificity is how many conditions a line has, or -1 if one of them doesn't hold.
func (w *When) specificity(f facts) int {
	if w == nil {
		return 0
	}
	n := 0
	if len(w.Msg) > 0 {
		if !hasWord(f.msg, w.Msg) {
			return -1
		}
		n++
	}
	if w.MsgEmoji {
		if !hasEmoji(f.msg) {
			return -1
		}
		n++
	}
	if w.Tier != "" {
		if !inRange(w.Tier, f.tier, false) {
			return -1
		}
		n++
	}
	if w.Hour != "" {
		if !inRange(w.Hour, f.now.Hour(), true) {
			return -1
		}
		n++
	}
	if len(w.Weekday) > 0 {
		ok := false
		for _, d := range w.Weekday {
			ok = ok || strings.EqualFold(d, f.now.Weekday().String())
		}
		if !ok {
			return -1
		}
		n++
	}
	for k, v := range w.Min {
		if f.numbers[k] < v {
			return -1
		}
		n++
	}
	for k, v := range w.Max {
		if f.numbers[k] > v {
			return -1
		}
		n++
	}
	return n
}

// hasWord: one of the words (or phrases) appears in the message as whole words, any case.
func hasWord(msg string, words []string) bool {
	m := " " + strings.Join(strings.FieldsFunc(strings.ToLower(msg), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}), " ") + " "
	for _, w := range words {
		w = " " + strings.Join(strings.Fields(strings.ToLower(w)), " ") + " "
		if strings.Contains(m, w) {
			return true
		}
	}
	return false
}

func hasEmoji(s string) bool {
	for _, r := range s {
		if r >= 0x1F300 && r <= 0x1FAFF || r >= 0x2600 && r <= 0x27BF {
			return true
		}
	}
	return false
}

// inRange: "3" or "3-4"; for hours, "22-4" wraps past midnight.
func inRange(spec string, v int, wraps bool) bool {
	lo, hi, isRange := strings.Cut(spec, "-")
	a, err1 := strconv.Atoi(strings.TrimSpace(lo))
	if err1 != nil {
		return false
	}
	if !isRange {
		return v == a
	}
	b, err2 := strconv.Atoi(strings.TrimSpace(hi))
	if err2 != nil {
		return false
	}
	if wraps && a > b {
		return v >= a || v <= b
	}
	return v >= a && v <= b
}

// choose picks the line for a moment: the most specific match she hasn't said lately.
// specificOnly: say nothing unless a line with conditions matches (reactions like
// "commit_reaction" shouldn't fire on every commit).
func (e *Engine) choose(key string, vars map[string]string, specificOnly bool) (string, bool) {
	opts := e.lines[key]
	if len(opts) == 0 {
		return "", false
	}
	f := e.facts(vars)
	byLevel := map[int][]string{}
	best := -1
	for _, l := range opts {
		s := l.When.specificity(f)
		if s < 0 {
			continue
		}
		byLevel[s] = append(byLevel[s], l.Text)
		best = max(best, s)
	}
	recent := e.recentLines(key)
	for lvl := best; lvl >= 0; lvl-- {
		cands := byLevel[lvl]
		if len(cands) == 0 {
			continue
		}
		if lvl == 0 && specificOnly {
			return "", false
		}
		fresh := cands[:0:0]
		for _, c := range cands {
			if !contains(recent, c) {
				fresh = append(fresh, c)
			}
		}
		if len(fresh) == 0 {
			if lvl > 0 && hasLower(byLevel, lvl) {
				continue // she just said the special one: fall back to something else
			}
			fresh = cands
		}
		pick := fresh[e.env.Pick(len(fresh))]
		e.rememberLine(key, pick, len(opts))
		return pick, true
	}
	return "", false
}

func hasLower(byLevel map[int][]string, lvl int) bool {
	for l := lvl - 1; l >= 0; l-- {
		if len(byLevel[l]) > 0 {
			return true
		}
	}
	return false
}

// ---- what she said lately (per moment), so she doesn't repeat herself ----

func (e *Engine) recentLines(key string) []string {
	s, _ := e.loadSeen()
	return s.Recent[key]
}

func (e *Engine) rememberLine(key, line string, options int) {
	s, _ := e.loadSeen()
	if s.Recent == nil {
		s.Recent = map[string][]string{}
	}
	keep := min(3, options-1) // remember up to 3, but never every option she has
	if keep <= 0 {
		return
	}
	r := append(s.Recent[key], line)
	if len(r) > keep {
		r = r[len(r)-keep:]
	}
	s.Recent[key] = r
	e.saveSeen(s)
}
