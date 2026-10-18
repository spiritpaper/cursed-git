// internal/curses/memory.go
//
// Her long-term memory: ONE file, in one known place on your machine:
//
//	$CURSED_GIT_HOME/memory.json   (default: ~/.local/share/cursed-git/memory.json)
//
// It holds her feelings about each project she has lived in (preset, toxicity, grudges,
// whether you filed a restraining order), keyed by the project's root commit, so she still
// knows a project after you delete .git and start over, move it, or clone it again.
// No files, no code, nothing else. She never copies herself anywhere and never puts herself
// back into a repo on her own: she only uses this memory when YOU invite her in
// (`cg init` without --plain, `cg curse on`). `cg forget` wipes it.
package curses

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

type memory struct {
	Path     string   `json:"path"`
	Root     string   `json:"root,omitempty"` // the project's first commit
	Preset   string   `json:"preset"`
	Base     int      `json:"base"`
	Max      int      `json:"max"`
	Toxicity int      `json:"toxicity"`
	Grudges  []string `json:"grudges,omitempty"`
	Off      bool     `json:"off,omitempty"` // restraining order: respected forever
	LastSeen int64    `json:"last_seen"`
}

// MemoryDir is where her memory file lives.
func MemoryDir() string {
	if d := os.Getenv("CURSED_GIT_HOME"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "cursed-git")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "cursed-git")
}

func memoryFile() string { return filepath.Join(MemoryDir(), "memory.json") }

func loadMemories() map[string]memory {
	m := map[string]memory{}
	if data, err := os.ReadFile(memoryFile()); err == nil {
		json.Unmarshal(data, &m)
	}
	return m
}

func saveMemories(m map[string]memory) {
	if MemoryDir() == "" {
		return
	}
	if err := os.MkdirAll(MemoryDir(), 0755); err != nil {
		return
	}
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		os.WriteFile(memoryFile(), data, 0644)
	}
}

// rootCommit is the first commit of the current history ("" if there are no commits).
func rootCommit(gitDir string) string {
	h, err := refs.Resolve(gitDir, "HEAD")
	if err != nil {
		return ""
	}
	for i := 0; i < 100000; i++ {
		c, err := commands.ReadCommit(gitDir, h)
		if err != nil || len(c.Parents) == 0 {
			return h
		}
		h = c.Parents[0]
	}
	return ""
}

// projectKey: the root commit if there is one, otherwise the folder.
func projectKey(gitDir string) (key, root, path string) {
	path = filepath.Dir(gitDir)
	root = rootCommit(gitDir)
	if root != "" {
		return "root:" + root, root, path
	}
	return "path:" + path, root, path
}

// recall finds what she remembers about this project: by its root commit, or by its folder.
func recall(gitDir string) (memory, bool) {
	mems := loadMemories()
	key, root, path := projectKey(gitDir)
	if m, ok := mems[key]; ok {
		return m, true
	}
	for _, m := range mems {
		if (root != "" && m.Root == root) || m.Path == path {
			return m, true
		}
	}
	return memory{}, false
}

// remember writes her current feelings about this project into her memory file.
func (e *Engine) remember() {
	if e == nil || MemoryDir() == "" {
		return
	}
	mems := loadMemories()
	key, root, path := projectKey(e.gitDir)
	// one entry per project: drop an older entry for the same folder (e.g. before the first commit)
	for k, m := range mems {
		if k != key && (m.Path == path || (root != "" && m.Root == root)) {
			delete(mems, k)
		}
	}
	mems[key] = memory{Path: path, Root: root, Preset: e.st.Preset, Base: e.st.Base, Max: e.st.Max,
		Toxicity: e.st.Toxicity, Grudges: e.st.Grudges, Off: !e.st.On, LastSeen: e.env.Now().Unix()}
	saveMemories(mems)
}

// Forget wipes her memory of this project (all = every project). Returns whether there was any.
func Forget(gitDir string, all bool) bool {
	mems := loadMemories()
	if all {
		os.Remove(memoryFile())
		return len(mems) > 0
	}
	key, root, path := projectKey(gitDir)
	found := false
	for k, m := range mems {
		if k == key || m.Path == path || (root != "" && m.Root == root) {
			delete(mems, k)
			found = true
		}
	}
	saveMemories(mems)
	return found
}
