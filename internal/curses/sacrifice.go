// internal/curses/sacrifice.go
//
// Commit sacrifice (docs/CURSES.md §3, Tier 3). When she's angry (temper +1 or more), a
// commit can be followed by: "This repository cannot be saved as it is. Something has to be
// sacrificed." She drops one older commit from your branch and git replays everything after it:
//
//	A - B - C - D        sacrifice B:   git rebase --onto A B
//	A - C' - D'          C and D get new IDs (a commit's ID includes its parent's ID)
//
// Rules: only commits you haven't pushed (pushed history is not hers to rewrite), never a
// merge or the root commit, and the victim is kept forever under refs/cursed/sacrificed/,
// so `cg cherry-pick <ref>` brings it back. If the replay conflicts, that's a boss fight.
package curses

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

const sacrificeChance = 0.1

// git runs real git in the repo for her (CG_VIA: it's her doing it, not you behind her back).
func (e *Engine) git(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", e.workTree}, args...)...)
	cmd.Env = append(os.Environ(), "CG_VIA=1", "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true")
	out, err := cmd.Output()
	return string(out), err
}

// victims lists commits she may sacrifice, newest first: unpushed, not merges, not the root,
// with no merge between them and HEAD (a rebase would flatten it), and never HEAD itself.
func (e *Engine) victims() []string {
	out, err := e.git("rev-list", "--first-parent", "--parents", "--max-count=20", "HEAD", "--not", "--remotes")
	if err != nil {
		return nil
	}
	var list []string
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		f := strings.Fields(line) // hash parent [parent2]
		if len(f) != 2 {
			break // a merge (or the root): she stops here
		}
		if i > 0 {
			list = append(list, f[0])
		}
	}
	return list
}

// maybeSacrifice runs after a successful commit.
func (e *Engine) maybeSacrifice(now time.Time) {
	tier := e.st.Tier(now)
	if tier < 3 || e.st.Temper(now) < 1 || e.inProgress() || !e.chance(sacrificeChance) {
		return
	}
	if branch, err := refs.Follow(e.gitDir, "HEAD"); err != nil || branch == "HEAD" {
		return // detached HEAD: there's no branch to take from
	}
	n := 1
	if tier >= 4 {
		n = 2 // Catastrophic: she takes two
	}
	for i := 0; i < n; i++ {
		list := e.victims()
		if len(list) == 0 || !e.sacrifice(list[e.env.Pick(len(list))], now, i == 0) {
			return
		}
	}
}

// sacrifice drops one commit. It reports whether the replay finished cleanly.
func (e *Engine) sacrifice(victim string, now time.Time, first bool) bool {
	c, err := commands.ReadCommit(e.gitDir, victim)
	if err != nil || len(c.Parents) != 1 {
		return false
	}
	ref := fmt.Sprintf("refs/cursed/sacrificed/%d", now.UnixNano())
	for {
		if _, _, err := refs.Read(e.gitDir, ref); err != nil {
			break
		}
		ref += "x"
	}
	sig := refs.Who(fmt.Sprintf("cg <cg@cursed.git> %d +0000", now.Unix()))
	if err := refs.Update(e.gitDir, ref, victim, refs.Zero, sig, "sacrificed"); err != nil {
		return false // if she can't keep it, she doesn't take it
	}
	before, _ := refs.Resolve(e.gitDir, "HEAD")
	subject, _, _ := strings.Cut(strings.TrimSpace(c.Message), "\n")
	if first {
		e.pause()
		e.confess()
		e.say("sacrifice_intro", nil)
		e.pause()
	}
	_, rebaseErr := e.git("rebase", "--autostash", "--onto", c.Parents[0], victim)
	e.log(Event{Type: "sacrifice", Text: ref})
	e.log(Event{Type: "grudge", Text: fmt.Sprintf("%q, the commit I had to sacrifice", subject)})

	e.say("sacrifice_done", map[string]string{"msg": subject})
	e.excuse()
	out := e.env.Out
	after, _ := refs.Resolve(e.gitDir, "HEAD")
	fmt.Fprintf(out, "   She dropped %s %q from your branch. git replayed every commit after it\n", victim[:7], subject)
	fmt.Fprintf(out, "   (git rebase), so they all have new IDs: a commit's ID includes its parent's.\n")
	if after != "" && before != "" {
		fmt.Fprintf(out, "   HEAD was %s, now it's %s. Same work, different history.\n", before[:7], after[:7])
	}
	fmt.Fprintf(out, "   Want it back? `cg cherry-pick %s` replays it on top.\n\n", ref)

	if len(e.unmerged()) > 0 {
		// the replay hit a conflict: git stopped in the middle of the rebase
		e.startFight(now, e.st.Tier(now))
		return false
	}
	return rebaseErr == nil
}
