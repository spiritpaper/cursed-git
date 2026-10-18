package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A pack of your own: its name, its rival and its lines; Miranda's for anything it doesn't say.
func TestCustomToxicPack(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := os.Getenv("CURSED_GIT_HOME")
	os.MkdirAll(filepath.Join(dir, "personalities"), 0755)
	os.WriteFile(filepath.Join(dir, "personalities", "rex.json"), []byte(`{
  "_name": ["Rex"], "_rival": ["Stacy"], "_mode": ["toxic"],
  "rewrote": ["I fixed your message, babe."]
}`), 0644)
	repo := newRepo(t, "psycho")
	gitIn(t, repo, "config", "cursed.personality", "rex")
	write(t, repo, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", "a.txt")
	_, _, errs := run("commit", "-m", "fix")
	if !strings.Contains(errs, "I fixed your message, babe.") {
		t.Errorf("his own line should be used:\n%s", errs)
	}
	if !strings.Contains(errs, "(he said") && !strings.Contains(errs, "→") {
		t.Errorf("lines he doesn't have come from Miranda:\n%s", errs)
	}
	if card := mustRun(t, "card"); !strings.Contains(card, "dev & Rex") {
		t.Errorf("the card should use his name:\n%s", card)
	}
}

func TestUnknownPackIsMiranda(t *testing.T) {
	withHer(t, tuesday, lucky)
	repo := newRepo(t, "psycho")
	gitIn(t, repo, "config", "cursed.personality", "nobody")
	if card := mustRun(t, "card"); !strings.Contains(card, "dev & Miranda") {
		t.Errorf("an unknown pack falls back to Miranda:\n%s", card)
	}
}
