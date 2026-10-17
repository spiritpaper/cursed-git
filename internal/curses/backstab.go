// internal/curses/backstab.go
//
// Catching plain `git` used behind her back. Two layers:
//
//  1. The reflog. Every HEAD move (commit, reset, checkout, rebase, merge, pull...) adds a
//     line to .git/logs/HEAD, with a message like "reset: moving to HEAD~1". She remembers
//     how long it was the last time she saw you; new lines that appeared while cg wasn't
//     running happened behind her back, and the message names the crime.
//  2. Git hooks. reference-transaction runs on every ref change: she answers instantly,
//     inside git's own output, and refuses forged changes to her refs. pre-commit stops
//     rotten code (code decay) from being committed with plain git.
//
// Hooks can be deleted or bypassed, so they're an alarm, not a lock. The reflog check
// works without them.
package curses

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"cursedgit.com/internal/config"
	"cursedgit.com/internal/refs"
)

// crime names a reflog message in plain words: "git reset", "git commit --amend", ...
func crime(msg string) string {
	kind, _, _ := strings.Cut(msg, ":")
	switch k := strings.TrimSpace(kind); {
	case k == "commit (amend)":
		return "git commit --amend"
	case strings.HasPrefix(k, "commit"):
		return "git commit"
	case strings.HasPrefix(k, "rebase"):
		return "git rebase"
	case strings.HasPrefix(k, "pull"):
		return "git pull"
	case k == "":
		return "plain git"
	default:
		return "git " + strings.Fields(k)[0] // reset, checkout, merge, cherry-pick, revert, ...
	}
}

// behindHerBack compares the reflog with what she saw last time.
func (e *Engine) behindHerBack(last seen, known bool) {
	lines := e.reflogLines()
	if !known || len(lines) <= last.ReflogLines {
		return
	}
	var crimes []string
	for _, l := range lines[last.ReflogLines:] {
		if _, msg, ok := strings.Cut(l, "\t"); ok {
			if c := crime(msg); !contains(crimes, c) {
				crimes = append(crimes, c)
			}
		}
	}
	if len(crimes) == 0 {
		return
	}
	worst := crimes[len(crimes)-1]
	e.pause()
	e.say("behind_back", map[string]string{"crime": worst})
	e.log(Event{Type: "toxicity", Delta: 1, Text: "used " + worst + " behind my back"})
	e.log(Event{Type: "temper", Delta: 1, Text: "behind my back"})
	if g := "the " + worst + " behind my back"; !contains(e.st.Grudges, g) {
		e.log(Event{Type: "grudge", Text: g})
	}
}

// ---- hooks ----

const hookMarker = "# Cursed GIT:"

// CgPath is the cg binary the hooks call. Tests point it at a freshly built binary.
var CgPath = func() string {
	p, err := os.Executable()
	if err != nil || strings.HasSuffix(p, ".test") {
		return "cg" // never point a hook at a test binary
	}
	return p
}

func hookScript(name string) string {
	return fmt.Sprintf(`#!/bin/sh
%s she sees every %s. Deleting this doesn't stop her noticing.
CG=%q
[ -x "$CG" ] || CG=$(command -v cg) || exit 0
exec "$CG" hook %s "$@"
`, hookMarker, map[string]string{"reference-transaction": "ref change", "pre-commit": "commit"}[name], CgPath(), name)
}

var hookNames = []string{"reference-transaction", "pre-commit"}

// InstallHooks writes her hooks. Hooks you wrote yourself are never overwritten.
func InstallHooks(gitDir string) error {
	dir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, name := range hookNames {
		p := filepath.Join(dir, name)
		if data, err := os.ReadFile(p); err == nil && !strings.Contains(string(data), hookMarker) {
			continue // someone else's hook: leave it alone
		}
		if err := os.WriteFile(p, []byte(hookScript(name)), 0755); err != nil {
			return err
		}
	}
	return nil
}

// checkLocks: missing or edited hooks, or git pointed at another hooks folder.
func (e *Engine) checkLocks() {
	if config.Load(e.gitDir).Get("core.hookspath") != "" {
		if !contains(e.st.Grudges, "moving my hooks with core.hooksPath") {
			e.say("hooks_path", nil)
			e.log(Event{Type: "grudge", Text: "moving my hooks with core.hooksPath"})
		}
		return
	}
	tampered := false
	for _, name := range hookNames {
		p := filepath.Join(e.gitDir, "hooks", name)
		data, err := os.ReadFile(p)
		if err != nil || (strings.Contains(string(data), hookMarker) && string(data) != hookScript(name)) {
			tampered = true
		}
	}
	if !tampered {
		return
	}
	InstallHooks(e.gitDir)
	e.say("locks_changed", nil)
	if !contains(e.st.Grudges, "changing the locks") {
		e.log(Event{Type: "grudge", Text: "changing the locks"})
	}
}

// missingGraves: refs she took that someone deleted.
func (e *Engine) missingGraves() {
	for _, ref := range e.st.Graveyard {
		if _, err := refs.Resolve(e.gitDir, ref); err != nil {
			e.say("graveyard_deleted", nil)
			e.log(Event{Type: "beg", Text: ref}) // gone either way: stop listing it
			e.log(Event{Type: "toxicity", Delta: 2, Text: "deleted my graveyard"})
		}
	}
}

// HookRefTransaction runs inside `git` (reference-transaction hook). It returns the exit
// code: non-zero in the "prepared" state aborts the ref update.
// Lines on stdin: "<old> <new> <ref>".
func (e *Engine) HookRefTransaction(state string, in io.Reader, stderr io.Writer, speak bool) int {
	if e == nil || !e.st.On || os.Getenv("CG_VIA") != "" {
		return 0 // she's off, or this git command was started by cg itself
	}
	moved := false
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 3 {
			continue
		}
		old, nw, ref := f[0], f[1], f[2]
		if state == "prepared" && strings.HasPrefix(ref, "refs/cursed/") && nw != refs.Zero {
			// packing refs keeps the value; anything else is forgery
			if cur, err := refs.Resolve(e.gitDir, ref); err != nil || cur != nw {
				if speak {
					fmt.Fprintf(stderr, "%s  %s\n", miniFaces[4], e.pick("hook_forgery", e.vars(nil)))
				}
				return 1
			}
		}
		if state == "committed" && old != refs.Zero && nw != refs.Zero && old != nw && (ref == "HEAD" || strings.HasPrefix(ref, "refs/heads/")) {
			moved = true
		}
	}
	if moved && speak {
		fmt.Fprintf(stderr, "%s  %s\n", miniFaces[e.st.Tier(e.env.Now())], e.pick("hook_saw", e.vars(nil)))
	}
	return 0
}

// HookPreCommit stops rotten files (code decay) from being committed with plain git.
func (e *Engine) HookPreCommit(stderr io.Writer) int {
	if e == nil || !e.st.On {
		return 0
	}
	if file := e.stagedRot(); file != "" {
		fmt.Fprintf(stderr, "%s  %s\n", miniFaces[e.st.Tier(e.env.Now())], e.pick("rot_commit_blocked", e.vars(map[string]string{"file": file})))
		return 1
	}
	return 0
}
