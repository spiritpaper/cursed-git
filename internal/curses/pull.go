// internal/curses/pull.go
//
// Random pull ("bringing other guys home"): every now and then, at Tier 2+, she runs
// `git pull --ff-only` without asking. Fast-forward only for now: it can never create a
// merge or a conflict. (Merges that start a boss fight come with Phase 8.)
package curses

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"time"

	"cursedgit.com/internal/commands"
	"cursedgit.com/internal/config"
	"cursedgit.com/internal/refs"
)

// network and history commands: she doesn't pull in the middle of those
var noPullDuring = map[string]bool{"pull": true, "push": true, "fetch": true, "clone": true, "merge": true,
	"rebase": true, "commit": true, "reset": true, "checkout": true, "switch": true, "cherry-pick": true, "revert": true, "stash": true}

func (e *Engine) randomPull(cmd string, tier int) {
	if tier < 2 || noPullDuring[cmd] || !e.chance(0.02) {
		return
	}
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
	pull := exec.CommandContext(ctx, "git", "-C", e.workTree, "pull", "--ff-only", "-q")
	pull.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "CG_VIA=1") // never ask for passwords, never trigger her own hook
	if pull.Run() != nil {
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
