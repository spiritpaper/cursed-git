// internal/curses/decay.go
//
// Code decay (Tier 3+): files nobody has touched in a long time start to rot in the
// working tree. Characters turn into ░▒▓, more each time she looks, until any commit
// shows her you're still around and she restores them ("Fine. It lives.").
//
// Safety, because this one touches your files:
//   - only files that are clean (exactly what's committed) ever rot, so no work is at risk
//   - rot only uses ░▒▓: obvious, never compiles, never looks like real code
//     (no sneaky look-alike characters: that would plant real bugs)
//   - she keeps a map of what she rotted (decay.json) and the originals are in HEAD
//   - `cg add`/`cg commit` restore rot before staging, and the pre-commit hook stops
//     plain `git commit` from committing it
package curses

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/object"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

type rot struct {
	Orig   string `json:"orig"`   // blob hash of the clean file
	Rotted string `json:"rotted"` // blob hash of what she wrote
	Level  int    `json:"level"`  // 1–5
}

var rotGlyphs = []rune("░▒▓")

// DecayPeriod: how long until a neglected file is fully rotten, by tier. 0 = no decay.
func DecayPeriod(tier int) time.Duration {
	switch {
	case tier >= 4:
		return 10 * 24 * time.Hour
	case tier == 3:
		return 30 * 24 * time.Hour
	}
	return 0
}

func (e *Engine) decayPath() string { return filepath.Join(dirOf(e.gitDir), "decay.json") }

func (e *Engine) loadDecay() map[string]rot {
	m := map[string]rot{}
	if data, err := os.ReadFile(e.decayPath()); err == nil {
		json.Unmarshal(data, &m)
	}
	return m
}

func (e *Engine) saveDecay(m map[string]rot) {
	if len(m) == 0 {
		os.Remove(e.decayPath())
		return
	}
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		os.WriteFile(e.decayPath(), data, 0644)
	}
}

// rotten returns content with roughly level×12% of its visible characters replaced.
// Positions are chosen by hashing (path, position), so a higher level only adds rot.
func rotten(path string, content []byte, level int) []byte {
	var out bytes.Buffer
	i := 0
	for len(content) > 0 {
		r, size := utf8.DecodeRune(content)
		content = content[size:]
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			out.WriteRune(r) // keep the shape of the file: it should still look like code, just sick
			continue
		}
		h := fnv.New32a()
		fmt.Fprintf(h, "%s:%d", path, i)
		v := h.Sum32()
		if int(v%100) < level*12 {
			out.WriteRune(rotGlyphs[v/100%uint32(len(rotGlyphs))])
		} else {
			out.WriteRune(r)
		}
		i++
	}
	return out.Bytes()
}

func isText(b []byte) bool { return len(b) < 512*1024 && !bytes.ContainsRune(b, 0) && utf8.Valid(b) }

// lastChanged works out when each file in HEAD last changed, from the history (newest first).
func (e *Engine) lastChanged() map[string]int64 {
	out := map[string]int64{}
	head, err := refs.Resolve(e.gitDir, "HEAD")
	if err != nil {
		return out
	}
	h := head
	for i := 0; i < 300 && h != ""; i++ {
		c, err := commands.ReadCommit(e.gitDir, h)
		if err != nil {
			break
		}
		files, _ := commands.FlattenTree(e.gitDir, c.Tree)
		var before map[string]string
		if len(c.Parents) > 0 {
			if p, err := commands.ReadCommit(e.gitDir, c.Parents[0]); err == nil {
				pf, _ := commands.FlattenTree(e.gitDir, p.Tree)
				before = map[string]string{}
				for k, v := range pf {
					before[k] = v.Hash
				}
			}
		}
		for path, ent := range files {
			if _, done := out[path]; !done && (before == nil || before[path] != ent.Hash) {
				out[path] = c.Committer.When
			}
		}
		if len(c.Parents) == 0 {
			break
		}
		h = c.Parents[0]
	}
	return out
}

// decay runs (at most once an hour) and rots the three most neglected clean files a bit more.
func (e *Engine) decay(now time.Time, tier int, last seen) {
	period := e.clocks().Decay(tier)
	if period == 0 || now.Unix()-last.DecayAt < 3600 {
		return
	}
	last.DecayAt = now.Unix()
	e.saveSeen(last)

	idx, err := index.Read(e.gitDir)
	if err != nil {
		return
	}
	staged := map[string]string{}
	for _, en := range idx.Entries {
		if en.Stage() == 0 && en.Mode == 0o100644 {
			staged[en.Path] = en.HashHex()
		}
	}
	m := e.loadDecay()
	start := period / 3
	type cand struct {
		path  string
		level int
		days  int
	}
	var cands []cand
	for path, when := range e.lastChanged() {
		if strings.HasPrefix(filepath.Base(path), ".") || staged[path] == "" {
			continue // dotfiles (like .gitignore) change how git behaves: never rot those
		}
		age := now.Sub(time.Unix(when, 0))
		if age < start {
			continue
		}
		level := min(5, 1+int(5*(age-start)/(period-start)))
		cands = append(cands, cand{path, level, int(age.Hours() / 24)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].days != cands[j].days {
			return cands[i].days > cands[j].days
		}
		return cands[i].path < cands[j].path
	})

	var news []cand
	for _, c := range cands[:min(3, len(cands))] {
		full := filepath.Join(e.workTree, filepath.FromSlash(c.path))
		disk, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		cur := object.Hash(object.Blob, disk)
		r, rotting := m[c.path]
		switch {
		case !rotting && cur == staged[c.path]:
			r = rot{Orig: cur}
		case rotting && cur == r.Rotted && c.level > r.Level:
			// still exactly how she left it: she may make it worse
		default:
			continue // you've been working on it (or it's already this bad): hands off
		}
		_, orig, err := object.Read(e.gitDir, r.Orig)
		if err != nil || !isText(orig) {
			continue
		}
		sick := rotten(c.path, orig, c.level)
		if err := os.WriteFile(full, sick, 0666); err != nil {
			continue
		}
		r.Rotted, r.Level = object.Hash(object.Blob, sick), c.level
		m[c.path] = r
		news = append(news, c)
	}
	e.saveDecay(m)
	if len(news) > 0 {
		c := news[0]
		key := "decay_worse"
		if c.level == 1 {
			key = "decay_start"
		}
		e.say(key, map[string]string{"file": c.path, "days": fmt.Sprint(c.days)})
	}
}

// unrot restores every rotted file that's still exactly how she left it.
// It returns how many files came back.
func (e *Engine) unrot() int {
	m := e.loadDecay()
	n := 0
	for path, r := range m {
		full := filepath.Join(e.workTree, filepath.FromSlash(path))
		disk, err := os.ReadFile(full)
		if err == nil && object.Hash(object.Blob, disk) == r.Rotted {
			if _, orig, err := object.Read(e.gitDir, r.Orig); err == nil && os.WriteFile(full, orig, 0666) == nil {
				n++
			}
		}
		delete(m, path) // edited or restored: either way she lets go of it
	}
	e.saveDecay(m)
	return n
}

// stagedRot returns a staged file (from her decay map) whose staged content contains rot.
func (e *Engine) stagedRot() string {
	m := e.loadDecay()
	if len(m) == 0 {
		return ""
	}
	idx, err := index.Read(e.gitDir)
	if err != nil {
		return ""
	}
	for _, en := range idx.Entries {
		if _, ok := m[en.Path]; !ok {
			continue
		}
		if _, data, err := object.Read(e.gitDir, en.HashHex()); err == nil && bytes.ContainsAny(data, string(rotGlyphs)) {
			return en.Path
		}
	}
	return ""
}
