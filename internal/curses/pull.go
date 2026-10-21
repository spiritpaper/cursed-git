// internal/curses/pull.go
//
// Random pull ("bringing other guys home"): every now and then, at Tier 2+, she merges
// something from your remotes without asking. Usually that's a pull request from one of your
// teammates' branches (pr.go). After a Tier 4 win she pulls your own upstream right away, and
// a conflict there is phase 2 (pull, below).
package curses

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/config"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

// network and history commands: she doesn't pull in the middle of those
var noPullDuring = map[string]bool{"pull": true, "push": true, "fetch": true, "clone": true, "merge": true,
	"rebase": true, "commit": true, "reset": true, "checkout": true, "switch": true, "cherry-pick": true, "revert": true, "stash": true}

func (e *Engine) randomPull(cmd string, tier int) {
	if tier < 2 || noPullDuring[cmd] || !e.chance(0.02) {
		return
	}
	e.pullRequest(tier)
}

// pull brings upstream home without asking (her random pull, and boss fight phase 2).
func (e *Engine) pull(tier int) {
	branch, err := refs.Follow(e.gitDir, "HEAD")
	if err != nil || branch == "HEAD" {
		return
	}
	name := branch[len("refs/heads/"):]
	remote := config.Load(e.gitDir).Get("branch." + name + ".remote")
	if remote == "" {
		return
	}
	before, _ := refs.Resolve(e.gitDir, "HEAD")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	how := []string{"--ff-only"}
	if tier >= 3 {
		how = []string{"--no-rebase", "--no-edit"} // a real merge, conflicts and all
	}
	pull := exec.CommandContext(ctx, "git", append([]string{"-C", e.workTree, "pull", "-q"}, how...)...)
	pull.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "CG_VIA=1") // never ask for passwords, never trigger her own hook
	err = pull.Run()
	if len(e.unmerged()) > 0 {
		e.say("fight_pull", map[string]string{"upstream": remote + "/" + name})
		e.startFight(e.env.Now(), tier)
		return
	}
	if err != nil {
		return // offline, diverged, dirty: she tried, nothing happened, nobody needs to know
	}
	after, _ := refs.Resolve(e.gitDir, "HEAD")
	if after == before {
		return
	}
	n := 0
	for h := after; h != before && h != "" && n < 1000; n++ {
		c, err := commands.ReadCommit(e.gitDir, h)
		if err != nil || len(c.Parents) == 0 {
			break
		}
		h = c.Parents[0]
	}
	upstream := remote + "/" + name
	e.say("random_pull", map[string]string{"upstream": upstream, "n": strconv.Itoa(n)})
	e.log(Event{Type: "grudge", Text: "the day " + upstream + " came over"})
}
