// internal/curses/pr.go
//
// Random pull requests ("your bros want in"): every now and then, at Tier 2+, she fetches
// your remotes, picks a branch somebody pushed that you don't have yet, and merges it into
// yours without asking. If you've been pulling and talking to your team, it goes in clean.
// If you haven't, you both changed the same lines, and that's a boss fight against them:
//
//	Tier 2   she only merges what goes in clean (git merge-tree checks first, touching
//	         nothing); a conflicting one she closes, and tells you to go and sync
//	Tier 3+  she merges anyway: a conflict is a boss fight, named after whoever wrote it
//
// Your uncommitted work is safe: if the pull request touches a file you're still working on,
// it waits, and comes in right after your next commit. Work in other files is left alone
// (git carries it through the merge, and through `merge --abort`). Staged changes mean
// you're about to commit, so she doesn't start then. A clean merge is git's own, and
// `cg reset --keep ORIG_HEAD` undoes it without touching your uncommitted work.
package curses

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

// pullRequest is the branch she's merging, for the fight that may follow.
type pullRequest struct {
	n              int
	branch, author string // origin/feature, Sam
	boss           string // Sam, or Past You
}

// pullRequest: fetch, pick a branch that's ahead of you (prefer, if it still is), merge it.
func (e *Engine) pullRequest(tier int, prefer string) {
	branch, err := refs.Follow(e.gitDir, "HEAD")
	if err != nil || !strings.HasPrefix(branch, "refs/heads/") {
		return // a detached HEAD has nowhere to merge into
	}
	st, err := commands.Status()
	if err != nil || len(st.Staged) > 0 {
		return
	}
	remotes, _ := e.git("remote")
	if strings.TrimSpace(remotes) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fetch := exec.CommandContext(ctx, "git", "-C", e.workTree, "fetch", "-q", "--all")
	fetch.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "CG_VIA=1") // never ask for passwords
	fetch.Run()                                                           // offline: what we fetched last time still counts

	pr := e.pickRequest(prefer)
	if pr == nil {
		if prefer != "" {
			e.log(Event{Type: "pr_waiting"}) // they merged it themselves: nothing waits any more
		}
		return // you're up to date with everyone. She has nothing on you.
	}
	ahead, _ := e.git("rev-list", "--count", "HEAD.."+pr.branch)
	subject, _ := e.git("log", "-1", "--format=%s", pr.branch)
	vars := map[string]string{"author": pr.author, "branch": pr.branch, "n": strings.TrimSpace(ahead)}

	out := e.env.Out
	e.pause()
	local := strings.TrimPrefix(branch, "refs/heads/")
	fmt.Fprintf(out, "\n📬 Pull request #%d from %s: %s → %s · %s\n", pr.n, pr.author, pr.branch, local,
		plural(atoi(ahead), "commit"))
	fmt.Fprintf(out, "   %q\n", strings.TrimSpace(subject))
	if file := e.inTheWay(st, pr.branch); file != "" {
		vars["file"] = file
		e.say("pr_wait", vars)
		e.log(Event{Type: "pr_waiting", Text: pr.branch})
		return
	}
	e.say("pr_open", vars)
	e.log(Event{Type: "pull_request", Text: pr.branch})

	if tier < 3 && !e.mergesClean(pr.branch) {
		e.say("pr_closed", vars)
		fmt.Fprintf(out, "   (Pull it yourself and fix the conflict now: `cg pull`, or `cg merge %s`. It only gets worse.)\n", pr.branch)
		return
	}
	before, _ := refs.Resolve(e.gitDir, "HEAD")
	msg := fmt.Sprintf("Merge pull request #%d from %s (%s)", pr.n, pr.author, pr.branch)
	how := "--no-ff" // a pull request is a merge commit, like on GitHub...
	if strings.HasSuffix(pr.branch, "/"+local) {
		how = "--ff" // ...except your own branch from upstream: that's just a pull
	}
	_, err = e.git("merge", how, "--no-edit", "-q", "-m", msg, pr.branch)
	if len(e.unmerged()) > 0 {
		e.say("fight_pr", vars)
		e.pr = pr
		e.startFight(e.env.Now(), tier)
		e.pr = nil
		return
	}
	if after, _ := refs.Resolve(e.gitDir, "HEAD"); err != nil || after == before {
		return // git said no (an untracked file in the way): nothing changed
	}
	e.say("pr_clean", vars)
	e.log(Event{Type: "xp", Delta: 25, Text: "in sync with " + pr.author})
	fmt.Fprintf(out, "   (+25 XP for being in sync. `cg log --graph --oneline` shows the merge; `cg reset --keep ORIG_HEAD` takes it back.)\n")
	return
}

// pickRequest: a remote branch with commits you don't have. Your teammates' come first.
func (e *Engine) pickRequest(prefer string) *pullRequest {
	list, err := e.git("for-each-ref", "--format=%(refname)", "refs/remotes/")
	if err != nil {
		return nil
	}
	me, _, _ := commands.Identity(e.gitDir)
	var theirs, mine []*pullRequest
	for _, ref := range strings.Fields(list) {
		name := strings.TrimPrefix(ref, "refs/remotes/")
		if strings.HasSuffix(name, "/HEAD") {
			continue
		}
		if n, _ := e.git("rev-list", "--count", "HEAD.."+ref); atoi(n) == 0 {
			continue
		}
		who, _ := e.git("log", "-1", "--format=%an%x00%ae", ref)
		author, email, _ := strings.Cut(strings.TrimSpace(who), "\x00")
		pr := &pullRequest{branch: name, author: author, boss: author, n: e.st.PullRequests + 1}
		if f := strings.Fields(author); len(f) > 0 {
			pr.author, pr.boss = f[0], f[0]
		}
		if name == prefer {
			return pr
		}
		if email == me.Email {
			pr.boss = "Past You"
			mine = append(mine, pr)
		} else {
			theirs = append(theirs, pr)
		}
		if len(theirs)+len(mine) >= 30 {
			break
		}
	}
	pool := theirs
	if len(pool) == 0 {
		pool = mine
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[e.env.Pick(len(pool))]
}

// inTheWay: a file you haven't committed that the pull request changes ("" if none).
func (e *Engine) inTheWay(st *commands.StatusReport, branch string) string {
	theirs, err := e.git("diff", "--name-only", "HEAD..."+branch) // since where you split
	if err != nil {
		return ""
	}
	yours := map[string]bool{}
	for _, c := range st.Unstaged {
		yours[c.Path] = true
	}
	if files, err := changedFiles(e.workTree); err == nil {
		for _, f := range files {
			yours[f] = true
		}
	}
	for _, f := range strings.Split(strings.TrimSpace(theirs), "\n") {
		if yours[f] {
			return f
		}
	}
	return ""
}

// waitingRequest: a pull request that waited for your commit comes in right after it.
func (e *Engine) waitingRequest(now time.Time) {
	if e.st.PRWaiting == "" || e.inProgress() || e.st.Lovebombing(now) {
		return
	}
	if tier := e.st.Tier(now); tier >= 2 {
		e.pullRequest(tier, e.st.PRWaiting)
	}
}

// mergesClean asks git whether a merge would conflict, without touching anything (git 2.38+).
// If git can't tell, the answer is no: at Tier 2 she doesn't gamble.
func (e *Engine) mergesClean(branch string) bool {
	_, err := e.git("merge-tree", "--write-tree", "--quiet", "HEAD", branch)
	return err == nil // exit 1: it conflicts
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
