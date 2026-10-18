// internal/curses/xp.go
//
// Closure XP (docs/CURSES.md §5.1, §6.2): to be allowed to leave her, you have to do real work.
//
//	every line you add in a commit     +1   (paid later, see below)
//	a commit with a decent message     +10
//	a boss fight won                   +250, +1 for every 10 seconds left on the clock
//
// Line XP can't be farmed. It's paid per blob (a file's exact content, by hash) that she has
// never paid for, so commit -> revert -> commit again pays once: the re-committed file has the
// same hash. And it starts as *pending*: it only pays once the commit is still on a branch
// after xpMaturity (24h). Work that gets reset, amended or rebased away never pays.
//
// level = floor(sqrt(xp / 50)), so Lv 10 is 5000 XP.
package curses

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/config"
	"github.com/spiritpaper/cursed-git/internal/diff"
	"github.com/spiritpaper/cursed-git/internal/object"
)

// pending line XP for one commit.
type pending struct {
	Commit string
	Lines  int
	At     int64
	Blobs  []string
}

// Level is the closure level for an amount of XP.
func Level(xp int) int { return int(math.Sqrt(float64(xp) / 50)) }

// XPForLevel is the XP a level starts at.
func XPForLevel(lv int) int { return lv * lv * 50 }

// Rules are the endgame numbers, from [cursed] in .git/config (docs/CURSES.md §7).
type Rules struct {
	BreakupLevel, MinWins, MinLines, ToxicThreshold int
	Maturity                                        time.Duration
}

func rules(gitDir string) Rules {
	cfg := config.Load(gitDir)
	num := func(key string, def int) int {
		if n, err := strconv.Atoi(cfg.Get("cursed." + key)); err == nil && n >= 0 {
			return n
		}
		return def
	}
	r := Rules{num("breakupLevel", 10), num("minBossFightsWon", 3), num("minLinesCommitted", 2000),
		num("toxicThreshold", ToxicThreshold), 24 * time.Hour}
	if d, err := time.ParseDuration(cfg.Get("cursed.xpMaturity")); err == nil && d >= 0 {
		r.Maturity = d
	}
	return r
}

// payForCommit records line XP for a commit you just made: new content only, pending.
func (e *Engine) payForCommit(hash string) {
	c, err := commands.ReadCommit(e.gitDir, hash)
	if err != nil || len(c.Parents) > 1 {
		return // a merge commit's lines are mostly someone else's work
	}
	now, err := commands.FlattenTree(e.gitDir, c.Tree)
	if err != nil {
		return
	}
	before := map[string]object.TreeEntry{}
	if len(c.Parents) == 1 {
		if p, err := commands.ReadCommit(e.gitDir, c.Parents[0]); err == nil {
			before, _ = commands.FlattenTree(e.gitDir, p.Tree)
		}
	}
	lines := 0
	var blobs []string
	for path, ent := range now {
		old, had := before[path]
		if (had && old.Hash == ent.Hash) || e.st.PaidBlobs[ent.Hash] || ent.Mode == "120000" {
			continue
		}
		_, newData, err := object.Read(e.gitDir, ent.Hash)
		if err != nil || isBinary(newData) {
			continue
		}
		var oldData []byte
		if had {
			_, oldData, _ = object.Read(e.gitDir, old.Hash)
		}
		added := 0
		for _, op := range diff.Diff(diff.Split(oldData), diff.Split(newData)) {
			if op.Kind == '+' {
				added++
			}
		}
		if added > 0 {
			lines += added
			blobs = append(blobs, ent.Hash)
		}
	}
	if lines > 0 {
		e.log(Event{Type: "xp_pending", Text: hash, Delta: lines, Blobs: blobs})
	}
}

func isBinary(b []byte) bool { return strings.IndexByte(string(b[:min(len(b), 8000)]), 0) >= 0 }

// matureXP pays (or cancels) pending line XP whose waiting time is over.
func (e *Engine) matureXP(now time.Time) {
	wait := int64(rules(e.gitDir).Maturity.Seconds())
	for _, p := range e.st.Pending {
		if now.Unix()-p.At < wait {
			continue
		}
		out, err := e.git("branch", "--contains", p.Commit, "--format=%(refname)")
		if err == nil && strings.TrimSpace(out) != "" {
			e.log(Event{Type: "xp_paid", Text: p.Commit})
		} else {
			e.log(Event{Type: "xp_cancel", Text: p.Commit}) // reset, amended or rebased away
		}
	}
}

// ClosureLine is the progress line for `cg mood`.
func (e *Engine) ClosureLine() string {
	st, r := e.st, rules(e.gitDir)
	lv := Level(st.XP)
	full := min(10, lv*10/max(1, r.BreakupLevel))
	line := "Closure: Lv " + strconv.Itoa(lv) + " " + strings.Repeat("▓", full) + strings.Repeat("░", 10-full) +
		" " + strconv.Itoa(st.XP) + " XP"
	if p := st.PendingLines(); p > 0 {
		line += " (+" + strconv.Itoa(p) + " pending)"
	}
	line += " · " + strconv.Itoa(st.Lines) + "/" + strconv.Itoa(r.MinLines) + " lines · " +
		strconv.Itoa(st.BossWon) + "/" + strconv.Itoa(r.MinWins) + " fights won"
	if lv < r.BreakupLevel {
		line += " (need Lv " + strconv.Itoa(r.BreakupLevel) + " to leave her)"
	}
	return line
}

// PendingLines is line XP waiting to mature.
func (s *State) PendingLines() int {
	n := 0
	for _, p := range s.Pending {
		n += p.Lines
	}
	return n
}
