package diff

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitHunks runs `git diff --no-index` on two files and returns everything from the first "@@".
func gitHunks(t *testing.T, a, b string, heuristic bool) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	pa, pb := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(pa, []byte(a), 0644)
	os.WriteFile(pb, []byte(b), 0644)
	cmd := exec.Command("git", "-c", fmt.Sprintf("diff.indentHeuristic=%v", heuristic), "diff", "--no-index", "--no-color", pa, pb)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, _ := cmd.Output() // exit 1 just means "different"
	s := string(out)
	if i := strings.Index(s, "@@"); i >= 0 {
		return s[i:]
	}
	return ""
}

var cases = []struct{ name, a, b string }{
	{"one line changed", "hello world\n", "hello\n"},
	{"added at end", "a\nb\n", "a\nb\nc\n"},
	{"added at start", "b\nc\n", "a\nb\nc\n"},
	{"deleted middle", "a\nb\nc\nd\ne\n", "a\nb\nd\ne\n"},
	{"to empty", "a\nb\n", ""},
	{"from empty", "", "x\ny\n"},
	{"no final newline", "a\nb", "a\nc"},
	{"newline added at end", "a\nb", "a\nb\n"},
	{"slide ambiguity", "a\nb\na\n", "a\n"},
	{"repeated block", "x\na\nb\na\nb\ny\n", "x\na\nb\ny\n"},
	{"two hunks far apart", "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15\n", "1\nTWO\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\nFOURTEEN\n15\n"},
	{"two changes merge into one hunk", "1\n2\n3\n4\n5\n6\n7\n8\n9\n", "1\nX\n3\n4\n5\n6\n7\nY\n9\n"},
	{"function context", "package main\n\nfunc main() {\n\ta := 1\n\tb := 2\n\tc := 3\n\td := 4\n\te := 5\n}\n", "package main\n\nfunc main() {\n\ta := 1\n\tb := 2\n\tc := 3\n\td := 40\n\te := 5\n}\n"},
	{"blank line runs", "a\n\n\nb\n", "a\n\nb\n"},
	{"braces", "if x {\n\tdo()\n}\n\nif y {\n\tdo()\n}\n", "if x {\n\tdo()\n}\n\nif z {\n\tdo()\n}\n\nif y {\n\tdo()\n}\n"},
}

func TestHunksMatchGit(t *testing.T) {
	for _, c := range cases {
		got := Unified(Split([]byte(c.a)), Split([]byte(c.b)), 3)
		if want := gitHunks(t, c.a, c.b, false); got != want {
			t.Errorf("%s\n--- cg:\n%s--- git (no indent heuristic):\n%s", c.name, got, want)
		}
	}
}

// Random edits: the patch must be correct (git can apply it and gets b exactly), and in the
// vast majority of cases identical to git's own.
func TestRandomDiffsApplyAndMostlyMatch(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	words := []string{"a\n", "b\n", "c\n", "}\n", "\n", "func x() {\n", "\treturn\n"}
	same := 0
	const n = 150
	for i := 0; i < n; i++ {
		var a, b strings.Builder
		for j := r.Intn(25); j > 0; j-- {
			a.WriteString(words[r.Intn(len(words))])
		}
		lines := Split([]byte(a.String()))
		for _, l := range lines { // b = a with random edits
			switch r.Intn(6) {
			case 0: // drop
			case 1:
				b.WriteString(words[r.Intn(len(words))])
				b.WriteString(l)
			default:
				b.WriteString(l)
			}
		}
		got := Unified(lines, Split([]byte(b.String())), 3)
		if got == gitHunks(t, a.String(), b.String(), false) {
			same++
		}
		if !applies(t, a.String(), b.String(), got) {
			t.Fatalf("case %d: our patch doesn't turn a into b\na=%q\nb=%q\npatch:\n%s", i, a.String(), b.String(), got)
		}
	}
	if same < n*95/100 {
		t.Errorf("only %d/%d random diffs identical to git's", same, n)
	}
	t.Logf("%d/%d random diffs byte-identical to git", same, n)
}

func applies(t *testing.T, a, b, hunks string) bool {
	if hunks == "" {
		return a == b
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f"), []byte(a), 0644)
	patch := "--- a/f\n+++ b/f\n" + hunks
	os.WriteFile(filepath.Join(dir, "p"), []byte(patch), 0644)
	cmd := exec.Command("git", "apply", "--unidiff-zero", "p")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("git apply: %v %s", err, out)
		return false
	}
	got, _ := os.ReadFile(filepath.Join(dir, "f"))
	return string(got) == b
}
