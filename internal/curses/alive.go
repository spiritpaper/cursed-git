// internal/curses/alive.go
//
// What makes her feel alive is attention: she knows how long you were gone, what time it
// is, your name, the file you can't stop touching, and what you wrote months ago.
// All of it comes from the repository and her diary. She never looks outside the repo.
package curses

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/config"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

// seen is her short-term memory between commands. It's a cache, not evidence: deleting it
// just makes her start counting again (the diary is what she can't forget).
type seen struct {
	At          int64               `json:"at"`           // last time any cg command ran
	ReflogLines int                 `json:"reflog_lines"` // length of .git/logs/HEAD at that moment
	Head        string              `json:"head"`
	DecayAt     int64               `json:"decay_at"` // last time she checked for rot
	TalkedAt    int64               `json:"talked_at"`
	Recent      map[string][]string `json:"recent,omitempty"` // her last lines per moment (lines.go)
}

func (e *Engine) seenPath() string { return filepath.Join(dirOf(e.gitDir), "seen.json") }

func (e *Engine) loadSeen() (seen, bool) {
	var s seen
	data, err := os.ReadFile(e.seenPath())
	if err != nil || json.Unmarshal(data, &s) != nil {
		return seen{}, false
	}
	return s, s.At != 0 // only a finished command counts as "she saw it" (not just a line she said)
}

func (e *Engine) saveSeen(s seen) {
	if data, err := json.Marshal(s); err == nil {
		os.WriteFile(e.seenPath(), data, 0644)
	}
}

func (e *Engine) reflogLines() []string {
	data, err := os.ReadFile(filepath.Join(e.gitDir, "logs", "HEAD"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// Touch is called at the end of every cg command: she remembers when she last saw you and
// what the repository looked like, so next time she can tell what happened without her.
func (e *Engine) Touch() {
	if e == nil || e.gone {
		return
	}
	s, _ := e.loadSeen()
	s.At = e.env.Now().Unix()
	s.ReflogLines = len(e.reflogLines())
	s.Head, _ = refs.Resolve(e.gitDir, "HEAD")
	e.saveSeen(s)
	e.remember()
	if e.awake() {
		if d, err := OpenDiary(e.gitDir); err == nil {
			e.st = Replay(d.Events()) // fresh: her own commands write with another engine
		}
		e.announce(e.achBefore)
	}
}

// vars fills the placeholders every line can use.
func (e *Engine) vars(extra map[string]string) map[string]string {
	now := e.env.Now()
	v := map[string]string{
		"name":    e.firstName(),
		"hour":    now.Format("15:04"),
		"weekday": now.Weekday().String(),
	}
	for k, val := range extra {
		v[k] = val
	}
	return v
}

func (e *Engine) firstName() string {
	name := strings.TrimSpace(config.Load(e.gitDir).Get("user.name"))
	if a, _, err := commands.Identity(e.gitDir); err == nil {
		name = strings.TrimSpace(a.Name) // the same name git puts on your commits
	}
	if name == "" {
		return "you"
	}
	return strings.Fields(name)[0]
}

func humanGap(d time.Duration) string {
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	case d >= 24*time.Hour:
		return "a whole day"
	case d >= 2*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	default:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
}

// greet: the first thing she says when you come back after a while.
func (e *Engine) greet(now time.Time, last seen, known bool) {
	if !known || last.At == 0 {
		return
	}
	gap := now.Sub(time.Unix(last.At, 0))
	key := ""
	switch {
	case gap >= 7*24*time.Hour:
		key = "absence_ages"
	case gap >= 48*time.Hour:
		key = "absence_long"
	case gap >= 8*time.Hour:
		key = "absence_short"
	}
	if key != "" {
		e.pause()
		e.say(key, map[string]string{"gone": humanGap(gap)})
		e.rewrite()
	}
	// were you somewhere else while she waited here?
	if other := e.elsewhere(last.At); other != "" && e.st.On {
		e.say("other_project", map[string]string{"project": other})
		if g := "the time you went off with " + other; !contains(e.st.Grudges, g) {
			e.log(Event{Type: "grudge", Text: g})
		}
		return
	}
	if key != "" {
		return
	}
	// late at night she gets... personal. Not every time.
	if (now.Hour() >= 1 && now.Hour() < 5) && now.Unix()-last.TalkedAt > 3600 && e.chance(0.5) {
		e.say("night_owl", nil)
		s, _ := e.loadSeen() // fresh: saying a line just updated it
		s.TalkedAt = now.Unix()
		e.saveSeen(s)
	}
}

// oldMessage picks one of your commit messages from a while back, for callbacks.
func (e *Engine) oldMessage() string {
	head, err := refs.Resolve(e.gitDir, "HEAD")
	if err != nil {
		return ""
	}
	var msgs []string
	for h, i := head, 0; h != "" && i < 60; i++ {
		c, err := commands.ReadCommit(e.gitDir, h)
		if err != nil {
			break
		}
		if i >= 3 { // not the last few: it has to feel like a memory
			if subject, _, _ := strings.Cut(strings.TrimSpace(c.Message), "\n"); subject != "" && !strings.HasPrefix(subject, "graveyard:") {
				msgs = append(msgs, subject)
			}
		}
		if len(c.Parents) == 0 {
			break
		}
		h = c.Parents[0]
	}
	if len(msgs) == 0 {
		return ""
	}
	return msgs[e.env.Pick(len(msgs))]
}

// favouriteFile is the file changed in the most of your recent commits.
func (e *Engine) favouriteFile() string {
	head, err := refs.Resolve(e.gitDir, "HEAD")
	if err != nil {
		return ""
	}
	count := map[string]int{}
	h := head
	for i := 0; i < 20 && h != ""; i++ {
		c, err := commands.ReadCommit(e.gitDir, h)
		if err != nil || len(c.Parents) == 0 {
			break
		}
		p, err := commands.ReadCommit(e.gitDir, c.Parents[0])
		if err != nil {
			break
		}
		now, _ := commands.FlattenTree(e.gitDir, c.Tree)
		before, _ := commands.FlattenTree(e.gitDir, p.Tree)
		for path, ent := range now {
			if before[path].Hash != ent.Hash {
				count[path]++
			}
		}
		h = c.Parents[0]
	}
	best, n := "", 1 // it only counts as a favourite if it's in at least 2 commits
	var names []string
	for p := range count {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		if count[p] > n {
			best, n = p, count[p]
		}
	}
	return best
}

// pause: a beat before the line that hurts. Only at a real terminal.
func (e *Engine) pause() { e.env.Sleep(700 * time.Millisecond) }
