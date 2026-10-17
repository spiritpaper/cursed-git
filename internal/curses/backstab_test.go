package curses

import (
	"strings"
	"testing"
)

func TestCrimeNames(t *testing.T) {
	for msg, want := range map[string]string{
		"reset: moving to HEAD~1":                          "git reset",
		"commit (amend): fix typo":                         "git commit --amend",
		"commit: add warning":                              "git commit",
		"commit (initial): first":                          "git commit",
		"rebase (finish): returning to refs/heads/x":       "git rebase",
		"checkout: moving from main to x":                  "git checkout",
		"pull: Fast-forward":                               "git pull",
		"merge feature: Merge made by the 'ort' strategy.": "git merge",
	} {
		if got := crime(msg); got != want {
			t.Errorf("crime(%q) = %q, want %q", msg, got, want)
		}
	}
}

// Rot only ever grows: every character rotten at level 2 is still rotten at level 4,
// and whitespace (the shape of the file) never rots.
func TestRotOnlyGrows(t *testing.T) {
	src := []byte("package core\n\nfunc Love() string {\n\treturn \"forever\"\n}\n")
	lo, hi := []rune(string(rotten("core.go", src, 2))), []rune(string(rotten("core.go", src, 4)))
	orig := []rune(string(src))
	for i := range orig {
		if strings.ContainsRune("░▒▓", lo[i]) && !strings.ContainsRune("░▒▓", hi[i]) {
			t.Fatalf("char %d healed as rot got worse", i)
		}
		if strings.ContainsRune(" \t\n", orig[i]) && lo[i] != orig[i] {
			t.Fatalf("whitespace at %d rotted", i)
		}
	}
	if string(rotten("core.go", src, 0)) != string(src) {
		t.Error("level 0 should change nothing")
	}
}
