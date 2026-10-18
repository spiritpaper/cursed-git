// internal/curses/rivals.go
//
// Rivalry: everyone who commits to this repository is on her scoreboard, and she plays them
// against each other. The numbers come from git's own history (since she moved in), never
// from her imagination: `git shortlog -sn` and `git log --author` would tell you the same.
package curses

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
)

// Rival is one person on her scoreboard.
type Rival struct {
	Name, Email          string
	Commits, Lines, Lazy int
	Score                int
	You                  bool
}

// Rivals ranks everyone who committed since she moved in, her favourite first.
func (e *Engine) Rivals() []Rival {
	since := "@" + strconv.FormatInt(e.st.OnSince, 10)
	out, err := e.git("log", "--no-merges", "--since="+since, "--format=%x00%ae%x09%an%x09%s", "--numstat")
	if err != nil {
		return nil
	}
	me := ""
	if a, _, err := commands.Identity(e.gitDir); err == nil {
		me = a.Email
	}
	by := map[string]*Rival{}
	for _, chunk := range strings.Split(out, "\x00")[1:] {
		lines := strings.Split(chunk, "\n")
		f := strings.SplitN(lines[0], "\t", 3)
		if len(f) < 3 {
			continue
		}
		r := by[f[0]]
		if r == nil {
			r = &Rival{Email: f[0], Name: f[1], You: f[0] == me}
			by[f[0]] = r
		}
		r.Commits++
		if isLazy(f[2]) {
			r.Lazy++
		}
		for _, l := range lines[1:] {
			if added, _, ok := strings.Cut(l, "\t"); ok {
				if n, err := strconv.Atoi(added); err == nil {
					r.Lines += n
				}
			}
		}
	}
	var list []Rival
	for _, r := range by {
		r.Score = r.Commits*10 + r.Lines - r.Lazy*25
		list = append(list, *r)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Score != list[j].Score {
			return list[i].Score > list[j].Score
		}
		return list[i].Email < list[j].Email
	})
	return list
}

// rivalry runs after your commit: a jab if someone's ahead of you, a crown if you just got ahead.
func (e *Engine) rivalry(now time.Time) {
	list := e.Rivals()
	if len(list) < 2 {
		return
	}
	if list[0].You {
		if !e.st.Favourite {
			e.log(Event{Type: "favourite"})
			e.say("rival_favourite", nil)
		}
		return
	}
	if e.st.Favourite {
		e.log(Event{Type: "dethroned"})
	}
	var me *Rival
	for i := range list {
		if list[i].You {
			me = &list[i]
		}
	}
	if me != nil && e.chance(0.25) {
		top := list[0]
		e.say("rival_jab", map[string]string{"rival": firstWord(top.Name), "theirs": fmt.Sprint(top.Commits), "mine": fmt.Sprint(me.Commits)})
	}
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return s
}
