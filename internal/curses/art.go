// internal/curses/art.go
//
// Art: text pictures (ASCII, braille, anything that's just characters). Some ship with cg
// (art/*.txt); you can add your own as $CURSED_GIT_HOME/art/<name>.txt, or with
// `cg art add <name> <file>`. Yours wins if both exist. Art named after a character
// (miranda.txt) is that character's portrait: she shows it when she arrives (`cg init`).
// Art named after a character and a mood (miranda-furious.txt) is that mood's face: `cg mood`
// and `cg face <mood>` show it instead of the little ASCII face.
package curses

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed art/*.txt
var shippedArt embed.FS

// MaxArt is the biggest picture she'll keep (a 200x100 braille picture is about 60 KB).
const MaxArt = 64 << 10

var artName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

func artDir() string {
	if d := MemoryDir(); d != "" {
		return filepath.Join(d, "art")
	}
	return ""
}

// Art finds a picture by name: yours first, then the ones that ship with cg.
func Art(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !artName.MatchString(name) {
		return "", false
	}
	if dir := artDir(); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, name+".txt")); err == nil {
			return cleanArt(string(data)), true
		}
	}
	if data, err := shippedArt.ReadFile("art/" + name + ".txt"); err == nil {
		return cleanArt(string(data)), true
	}
	return "", false
}

// MoodArt is a character's picture for a mood (miranda-furious), if someone drew one.
func MoodArt(character string, m Mood) (string, bool) {
	if m.Slug() == "" {
		return "", false
	}
	return Art(character + "-" + m.Slug())
}

// Introduce shows her portrait, the first time she's in a repo.
func (e *Engine) Introduce() {
	if !e.awake() {
		return
	}
	if art, ok := Art(Personality(e.gitDir)); ok {
		fmt.Fprintln(e.env.Out, art)
	}
}

// ArtPiece is one entry of the gallery.
type ArtPiece struct {
	Name  string
	Yours bool // in your art folder (and removable)
}

// Gallery lists every picture: the ones in cg and yours.
func Gallery() []ArtPiece {
	seen := map[string]bool{}
	if entries, err := shippedArt.ReadDir("art"); err == nil {
		for _, en := range entries {
			seen[strings.TrimSuffix(en.Name(), ".txt")] = false
		}
	}
	if dir := artDir(); dir != "" {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, en := range entries {
				if n := strings.TrimSuffix(en.Name(), ".txt"); n != en.Name() && artName.MatchString(n) {
					seen[n] = true
				}
			}
		}
	}
	out := make([]ArtPiece, 0, len(seen))
	for n, yours := range seen {
		out = append(out, ArtPiece{n, yours})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AddArt saves a picture to your art folder.
func AddArt(name, text string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !artName.MatchString(name) {
		return errors.New("a name is lowercase letters, digits, - and _ (up to 40)")
	}
	if len(text) > MaxArt {
		return errors.New("that's too big (64 KB at most)")
	}
	text = cleanArt(text)
	if text == "" {
		return errors.New("there's no picture in that")
	}
	dir := artDir()
	if dir == "" {
		return errors.New("no home folder to keep it in (set CURSED_GIT_HOME)")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".txt"), []byte(text+"\n"), 0644)
}

// RemoveArt deletes one of your pictures. The ones in cg can't be removed (only covered up).
func RemoveArt(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !artName.MatchString(name) || artDir() == "" {
		return os.ErrNotExist
	}
	return os.Remove(filepath.Join(artDir(), name+".txt"))
}

// cleanArt keeps the picture and nothing that could talk to the terminal: no escape codes,
// no carriage returns. Trailing spaces and blank lines at the ends go too.
func cleanArt(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= ' ' && r != 0x7f && !(r >= 0x80 && r < 0xa0)) {
			return r
		}
		return -1
	}, s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
