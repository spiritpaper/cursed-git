// internal/curses/ledger.go
//
// The guilt ledger: every act of "care" is an invoice. She saved your files (after taking
// them), held your hand through merges (that she started), fixed your commit messages (that
// were fine), waited for you (you were at work). All counted from her diary, so the numbers
// are real; the debt is not.
//
// And the receipts: when she claims a grand sacrifice ("I gave you everything"), the card
// puts her claim next to what she actually contributed. Which is nothing. She never commits.
package curses

import (
	"fmt"
	"strings"
)

// Ledger is what she says you owe her.
type Ledger struct {
	Saves, Returns, Merges, Fixes, Hours int
}

func (s *State) Ledger() Ledger {
	return Ledger{Saves: s.burials, Returns: s.begs, Merges: s.fights, Fixes: s.lazies, Hours: s.waited}
}

func (l Ledger) empty() bool { return l == Ledger{} }

// String: "3 rescues, 2 merges, 5 rewrites, 41 hours of waiting".
func (l Ledger) String() string {
	var parts []string
	add := func(n int, one, many string) {
		if n == 1 {
			parts = append(parts, "1 "+one)
		} else if n > 1 {
			parts = append(parts, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(l.Saves, "time she kept your files safe", "times she kept your files safe")
	add(l.Returns, "time she gave them back", "times she gave them back")
	add(l.Merges, "merge she held your hand through", "merges she held your hand through")
	add(l.Fixes, "commit message she had to fix", "commit messages she had to fix")
	add(l.Hours, "hour of waiting", "hours of waiting")
	return strings.Join(parts, ", ")
}

// OwedLine is the ledger line for `cg mood` ("" if she keeps none, or you owe nothing yet).
func (e *Engine) OwedLine() string {
	l := e.st.Ledger()
	if e.Gentle() || l.empty() {
		return ""
	}
	return "You owe her: " + l.String()
}

// invoice: right after she gives your files back, the bill.
func (e *Engine) Invoice() {
	if e.Gentle() || !e.awake() {
		return
	}
	e.say("invoice", nil)
	if l := e.st.Ledger(); !l.empty() {
		fmt.Fprintf(e.env.Out, "         (the bill so far: %s)\n", l)
	}
}

// receipts is the card row next to her claims: what she really contributed.
func (e *Engine) receipts() string {
	if !e.st.Flags["love_costs_nothing"] {
		return ""
	}
	return "She claims everything. Receipts: 0 commits."
}
