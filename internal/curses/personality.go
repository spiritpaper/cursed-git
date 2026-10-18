// internal/curses/personality.go
//
// Personality packs: who you're playing with. A pack is a JSON file of lines (see
// lines/miranda.json), plus a few keys about the character itself:
//
//	"_name":  ["Miranda"]   what she's called (the card says "you & Miranda")
//	"_rival": ["Chad"]      who shows up as the boss when a pull brings conflicts
//	"_mode":  ["toxic"]     toxic: the full game | gentle: no curses, someone who cherishes you
//
// cursed.personality picks one (default: miranda). Packs ship inside cg (lines/*.json), and you
// can add your own as $CURSED_GIT_HOME/personalities/<name>.json. A toxic pack falls back to
// Miranda's line for anything it doesn't say; a gentle pack never does (a missing line is
// silence, so a kind character can never accidentally say something cruel).
// docs/CHARACTERS.md is the guide to writing one.
package curses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/config"
)

const DefaultPersonality = "miranda"

// Personality is the pack this repo is set to.
func Personality(gitDir string) string {
	if p := strings.ToLower(strings.TrimSpace(config.Load(gitDir).Get("cursed.personality"))); p != "" {
		return p
	}
	return DefaultPersonality
}

// readPack finds a pack: yours first, then the ones that ship with cg.
func readPack(name string) (map[string][]Line, bool) {
	var data []byte
	var err error
	if dir := MemoryDir(); dir != "" {
		data, err = os.ReadFile(filepath.Join(dir, "personalities", name+".json"))
	}
	if data == nil || err != nil {
		if data, err = packs.ReadFile("lines/" + name + ".json"); err != nil {
			return nil, false
		}
	}
	lines := map[string][]Line{}
	if json.Unmarshal(data, &lines) != nil {
		return nil, false // a broken pack makes her mute, it never breaks git
	}
	return lines, true
}

// loadLines is the line pack for this repo.
func loadLines(gitDir string) map[string][]Line {
	base, _ := readPack(DefaultPersonality)
	if base == nil {
		base = map[string][]Line{}
	}
	name := Personality(gitDir)
	if name == DefaultPersonality {
		return base
	}
	pack, ok := readPack(name)
	if !ok {
		return base // unknown pack: Miranda it is
	}
	if first(pack["_mode"]) == "gentle" {
		return pack // never borrow a toxic line
	}
	for k, v := range pack {
		base[k] = v
	}
	return base
}

func first(list []Line) string {
	if len(list) == 0 {
		return ""
	}
	return list[0].Text
}

// Name is what the character is called.
func (e *Engine) Name() string {
	if n := first(e.lines["_name"]); n != "" {
		return n
	}
	return "her"
}

func (e *Engine) rival() string {
	if r := first(e.lines["_rival"]); r != "" {
		return r
	}
	return "Chad"
}

// Gentle: a character who cherishes you. No curses, no punishments.
func (e *Engine) Gentle() bool { return first(e.lines["_mode"]) == "gentle" }

// Characters lists the packs you can pick: the ones in cg and your own.
func Characters() []string {
	seen := map[string]bool{}
	if entries, err := packs.ReadDir("lines"); err == nil {
		for _, en := range entries {
			seen[strings.TrimSuffix(en.Name(), ".json")] = true
		}
	}
	if dir := MemoryDir(); dir != "" {
		if entries, err := os.ReadDir(filepath.Join(dir, "personalities")); err == nil {
			for _, en := range entries {
				if strings.HasSuffix(en.Name(), ".json") {
					seen[strings.TrimSuffix(en.Name(), ".json")] = true
				}
			}
		}
	}
	var out []string
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// Describe is a character's name and mode, for `cg partner`.
func Describe(name string) (display, mode string, ok bool) {
	pack, ok := readPack(name)
	if !ok {
		return "", "", false
	}
	display, mode = first(pack["_name"]), first(pack["_mode"])
	if display == "" {
		display = name
	}
	if mode == "" {
		mode = "toxic"
	}
	return display, mode, true
}
