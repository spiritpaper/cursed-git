// internal/diff/diff.go
//
// Line diffs, the way git makes them:
//
//  1. Myers' algorithm finds a shortest edit script (fewest lines deleted + added).
//  2. Compaction (git's xdl_change_compact, without the indent heuristic): a block of
//     changes can often be slid up or down without changing the result ("a\nb\na" vs "a":
//     delete lines 1-2 or 2-3?). Git slides every block down as far as it can, then back up
//     to line up with a change in the other file if there is one. Without this step, cg and
//     git would disagree about where a change is, even though both are correct.
//  3. Unified format: hunks with 3 lines of context, "@@ -a,b +c,d @@ <function line>".
package diff

import (
	"fmt"
	"strings"
)

// Split cuts content into lines, keeping each line's "\n" (so a missing final newline is a
// different line, like in git).
func Split(content []byte) []string {
	var out []string
	s := string(content)
	for s != "" {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

// myers marks which lines of a and b are changed, using the classic O(ND) algorithm:
// for each number of edits d, remember the furthest point reachable on each diagonal k = x-y,
// then walk the saved frontiers backwards to recover the path.
func myers(a, b []string) (delA, addB []bool) {
	n, m := len(a), len(b)
	delA, addB = make([]bool, n), make([]bool, m)
	max := n + m
	if max == 0 {
		return
	}
	v := make([]int, 2*max+2)
	var trace [][]int
	off := max + 1
	for d := 0; d <= max; d++ {
		trace = append(trace, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1] // step down: an insertion from b
			} else {
				x = v[off+k-1] + 1 // step right: a deletion from a
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1 // follow the "snake" of equal lines
			}
			v[off+k] = x
			if x >= n && y >= m {
				backtrack(trace, off, n, m, delA, addB)
				return
			}
		}
	}
	return
}

func backtrack(trace [][]int, off, x, y int, delA, addB []bool) {
	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[off+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x, y = x-1, y-1
		}
		if x == prevX {
			addB[prevY] = true
		} else {
			delA[prevX] = true
		}
		x, y = prevX, prevY
	}
}

// ---- compaction (git's xdl_change_compact, no indent heuristic) ----

type file struct {
	rec  []string
	rchg []bool
}

// a group is a run of changed lines [start, end) between unchanged lines (it may be empty).
// The k-th group of one file sits opposite the k-th group of the other file.
type group struct{ start, end int }

func (f *file) first() group {
	g := group{0, 0}
	for g.end < len(f.rec) && f.rchg[g.end] {
		g.end++
	}
	return g
}

func (f *file) next(g *group) bool {
	if g.end == len(f.rec) {
		return false
	}
	g.start = g.end + 1
	g.end = g.start
	for g.end < len(f.rec) && f.rchg[g.end] {
		g.end++
	}
	return true
}

func (f *file) previous(g *group) bool {
	if g.start == 0 {
		return false
	}
	g.end = g.start - 1
	g.start = g.end
	for g.start > 0 && f.rchg[g.start-1] {
		g.start--
	}
	return true
}

func (f *file) slideDown(g *group) bool {
	if g.end < len(f.rec) && f.rec[g.start] == f.rec[g.end] {
		f.rchg[g.start], f.rchg[g.end] = false, true
		g.start++
		g.end++
		for g.end < len(f.rec) && f.rchg[g.end] {
			g.end++
		}
		return true
	}
	return false
}

func (f *file) slideUp(g *group) bool {
	if g.start > 0 && f.rec[g.start-1] == f.rec[g.end-1] {
		g.start--
		g.end--
		f.rchg[g.start], f.rchg[g.end] = true, false
		for g.start > 0 && f.rchg[g.start-1] {
			g.start--
		}
		return true
	}
	return false
}

func compact(x, o *file) {
	g, go_ := x.first(), o.first()
	for {
		if g.end != g.start {
			var groupsize, earliestEnd int
			endMatchingOther := -1
			for {
				groupsize = g.end - g.start
				endMatchingOther = -1
				for x.slideUp(&g) {
					o.previous(&go_)
				}
				earliestEnd = g.end
				if go_.end > go_.start {
					endMatchingOther = g.end
				}
				for x.slideDown(&g) {
					o.next(&go_)
					if go_.end > go_.start {
						endMatchingOther = g.end
					}
				}
				if groupsize == g.end-g.start {
					break
				}
			}
			if g.end != earliestEnd && endMatchingOther != -1 {
				// line the (maybe merged) block up with the last change in the other file
				for go_.end == go_.start {
					x.slideUp(&g)
					o.previous(&go_)
				}
			}
		}
		if !x.next(&g) {
			break
		}
		o.next(&go_)
	}
}

// Op is one line of an edit script: ' ' kept, '-' deleted, '+' added.
type Op struct {
	Kind byte
	Line string
}

// Diff returns the edit script from a to b, git-compacted, deletions before additions.
func Diff(a, b []string) []Op {
	delA, addB := myers(a, b)
	fa, fb := &file{a, delA}, &file{b, addB}
	compact(fa, fb)
	compact(fb, fa)
	var ops []Op
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		switch {
		case i < len(a) && fa.rchg[i]:
			ops = append(ops, Op{'-', a[i]})
			i++
		case j < len(b) && fb.rchg[j]:
			ops = append(ops, Op{'+', b[j]})
			j++
		default:
			ops = append(ops, Op{' ', a[i]})
			i, j = i+1, j+1
		}
	}
	return ops
}

// funcLine is git's default "function name" for a hunk header: the nearest line above the
// hunk that starts with a letter, '_' or '$', cut to 80 bytes, trailing space trimmed.
func funcLine(a []string, before int) string {
	for i := before - 1; i >= 0; i-- {
		l := a[i]
		if l != "" && (isAlpha(l[0]) || l[0] == '_' || l[0] == '$') {
			if len(l) > 80 {
				l = l[:80]
			}
			return strings.TrimRight(l, " \t\r\n")
		}
	}
	return ""
}

func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func span(start, count int) string {
	if count == 0 {
		start--
	}
	if count == 1 {
		return fmt.Sprint(start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// Unified returns the hunks (everything from the first "@@") of a unified diff with
// `context` lines of context, or "" if a and b are equal.
func Unified(a, b []string, context int) string {
	ops := Diff(a, b)
	// positions of changes in the op list
	var changes []int
	for i, op := range ops {
		if op.Kind != ' ' {
			changes = append(changes, i)
		}
	}
	if len(changes) == 0 {
		return ""
	}
	var out strings.Builder
	for c := 0; c < len(changes); {
		// a hunk: changes closer than 2*context unchanged lines stay together
		first, last := changes[c], changes[c]
		for c+1 < len(changes) && changes[c+1]-last <= 2*context+1 {
			c++
			last = changes[c]
		}
		c++
		lo, hi := max(0, first-context), min(len(ops), last+context+1)
		// line numbers of the hunk start in a and b
		aStart, bStart := 1, 1
		for _, op := range ops[:lo] {
			if op.Kind != '+' {
				aStart++
			}
			if op.Kind != '-' {
				bStart++
			}
		}
		aCount, bCount := 0, 0
		for _, op := range ops[lo:hi] {
			if op.Kind != '+' {
				aCount++
			}
			if op.Kind != '-' {
				bCount++
			}
		}
		header := fmt.Sprintf("@@ -%s +%s @@", span(aStart, aCount), span(bStart, bCount))
		if fn := funcLine(a, aStart-1); fn != "" {
			header += " " + fn
		}
		out.WriteString(header + "\n")
		for _, op := range ops[lo:hi] {
			out.WriteByte(op.Kind)
			out.WriteString(op.Line)
			if !strings.HasSuffix(op.Line, "\n") {
				out.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return out.String()
}
