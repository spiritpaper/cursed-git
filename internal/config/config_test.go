package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	home := t.TempDir()
	global := filepath.Join(home, "gitconfig")
	os.WriteFile(global, []byte(`# my settings
[User]
	Name = "Sara Spirit"   ; trailing comment
	email = sara@example.com # another
[remote "origin"]
	url = git@github.com:me/cg.git
[core]
	bare
`), 0644)
	gitDir := t.TempDir()
	os.WriteFile(filepath.Join(gitDir, "config"), []byte("[user]\n\temail = work@example.com\n"), 0644)
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	c := Load(gitDir)
	for key, want := range map[string]string{
		"user.name":         "Sara Spirit",      // quotes and comment stripped, section case-insensitive
		"USER.NAME":         "Sara Spirit",      // keys are case-insensitive
		"user.email":        "work@example.com", // the repo's config wins over the global one
		"remote.origin.url": "git@github.com:me/cg.git",
		"core.bare":         "true", // a bare key means true
	} {
		if got := c.Get(key); got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
}
