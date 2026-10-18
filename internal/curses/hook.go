// internal/curses/hook.go
//
// The hook: why people stay. Not constant cruelty, but warmth that comes rarely and
// unpredictably, followed by withdrawal (push and pull: yes, like git). Plus the small tactics
// of docs/MIRANDA.md, each in its moment. The game never names them; the docs do.
package curses

import (
	"fmt"
	"strconv"
	"time"
)

// at most one charm per this long: rare is what makes it work
const charmEvery = 6 * time.Hour

// charm: after a real win, sometimes, she's wonderful. (Gentle characters are wonderful
// all the time and don't need this.)
func (e *Engine) charm(now time.Time, why string) {
	if e.Gentle() || e.st.Lovebombing(now) || now.Unix()-e.st.CharmAt < int64(charmEvery.Seconds()) || !e.chance(0.5) {
		return
	}
	e.pause()
	e.say("charm", map[string]string{"why": why})
	e.log(Event{Type: "charm"})
	e.charmedNow = true
}

// pullBack: on a later command, the warmth goes away again.
func (e *Engine) pullBack() {
	if !e.st.PullPending || e.charmedNow || e.Gentle() || !e.chance(0.6) {
		return
	}
	e.say("pull_back", nil)
	e.log(Event{Type: "pull"})
}

// excuse: right after she's done something bad, the child's voice.
func (e *Engine) excuse() {
	if e.chance(0.5) {
		e.say("excuse", nil)
	}
}

// confess: an honest little sin, so you won't look for the big one.
func (e *Engine) confess() {
	if e.chance(0.5) {
		e.say("confession", nil)
	}
}

// promise: she offers a deal for your commits. She never keeps it.
func (e *Engine) promise(now time.Time) {
	if e.st.Promise != nil || e.Gentle() || !e.chance(0.3) {
		return
	}
	e.say("promise_offer", map[string]string{"n": "3"})
	e.log(Event{Type: "promise", Delta: 3, Text: strconv.Itoa(e.st.Commits)})
}

// promiseDue: you did what she asked. She remembers it differently.
func (e *Engine) promiseDue() {
	p := e.st.Promise
	if p == nil || e.st.Commits-p.Base < p.Need {
		return
	}
	e.pause()
	e.say("promise_broken", nil)
	e.log(Event{Type: "promise_broken"})
}

// rewrite: after time apart, her version of what happened. The reflog has the real one.
func (e *Engine) rewrite() {
	if e.Gentle() || (len(e.st.Graveyard) == 0 && e.st.BossLost == 0 && len(e.st.Sacrificed) == 0) || !e.chance(0.5) {
		return
	}
	e.say("rewrite", nil)
	fmt.Fprintln(e.env.Out, "         (git remembers what really happened: `cg reflog`)")
}

// smallTalk: the rare, unprompted ones. Returns whether she said something.
func (e *Engine) smallTalk(tier int) bool {
	switch {
	case tier >= 1 && e.chance(0.03):
		e.say("confession", nil)
	case tier >= 1 && e.chance(0.03):
		e.say("triangle", nil)
	case tier >= 2 && e.chance(0.02):
		e.say("void", nil)
	case tier >= 1 && e.chance(0.02):
		e.say("claim", nil)
	default:
		return false
	}
	return true
}
