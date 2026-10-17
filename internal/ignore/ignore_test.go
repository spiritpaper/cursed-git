package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func matcher(t *testing.T, files map[string]string) *Matcher {
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(content), 0644)
	}
	return New(root)
}

func TestRules(t *testing.T) {
	m := matcher(t, map[string]string{
		".gitignore": `# comment
/cg
node_modules/
*.mp4
!keep.mp4
build/**
**/logs
docs/*.tmp
\#hash
`,
		"sub/.gitignore": "secret.txt\n/only-here\n",
	})
	cases := []struct {
		path string
		dir  bool
		want bool
		why  string
	}{
		{"cg", false, true, "anchored to root"},
		{"cmd/cg", false, false, "/cg only matches at the root"},
		{"node_modules", true, true, "dir-only pattern"},
		{"node_modules", false, false, "a FILE called node_modules isn't matched by node_modules/"},
		{"web/node_modules/x/y.js", false, true, "inside an ignored folder at any depth"},
		{"video.mp4", false, true, "glob by name"},
		{"a/b/video.mp4", false, true, "glob by name at any depth"},
		{"keep.mp4", false, false, "negation"},
		{"build/out/app", false, true, "trailing /**"},
		{"build", true, false, "build/** matches inside build, not build itself"},
		{"logs", true, true, "**/ at the top"},
		{"a/b/logs", true, true, "**/ at any depth"},
		{"docs/x.tmp", false, true, "slash in the middle anchors"},
		{"docs/deeper/x.tmp", false, false, "* does not cross /"},
		{"#hash", false, true, "escaped #"},
		{"sub/secret.txt", false, true, "nested .gitignore"},
		{"secret.txt", false, false, "nested rules only apply below their folder"},
		{"sub/only-here", false, true, "anchored to the nested folder"},
		{"sub/x/only-here", false, false, "anchored means not deeper"},
		{"src/main.go", false, false, "normal file"},
	}
	for _, c := range cases {
		if got := m.Ignored(c.path, c.dir); got != c.want {
			t.Errorf("Ignored(%q, dir=%v) = %v, want %v (%s)", c.path, c.dir, got, c.want, c.why)
		}
	}
}

func TestInfoExclude(t *testing.T) {
	m := matcher(t, map[string]string{".git/info/exclude": "*.local\n"})
	if !m.Ignored("settings.local", false) {
		t.Error(".git/info/exclude was not used")
	}
}
