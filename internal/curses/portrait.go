// internal/curses/portrait.go
package curses

import (
	"fmt"
	"strings"
)

// Her portrait. Only the LEFT half of each row is drawn by hand; the right half is the
// mirror image, so she always comes out symmetric. The middle 6 characters of the
// eyes / cheeks / mouth rows change with her mood.
var rows = [][2]string{
	{`          _.-''`, `''''''`},
	{`       .-'  .-'`, `''''''`},
	{`     .'   .'/\/`, `\/\/\/`},
	{`    /   .' |   `, "EYES"},
	{`   /   /   |   `, "CHEEK"},
	{`   |   |    \  `, "MOUTH"},
	{`   |   |     '.`, `______`},
	{`   |   |      _`, `|    |`},
	{`  /|   |    .' `, `'----'`},
	{` / |   |   /   `, `  <>  `},
}

// Mood is one of her expressions. 0–4 match the curse tiers (see docs/CURSES.md §2).
type Mood int

const (
	Content Mood = iota
	Unimpressed
	Judging
	Hostile
	Furious
	InLove
	Crying
)

var moodNames = []string{"content", "unimpressed", "judging you", "hostile", "furious", "in love (for now)", "crying"}

func (m Mood) String() string {
	if m < 0 || int(m) >= len(moodNames) {
		return fmt.Sprintf("mood(%d)", int(m))
	}
	return moodNames[m]
}

// eyes, cheeks, mouth: each exactly 6 characters wide
var faces = map[Mood][3]string{
	Content:     {` ^  ^ `, ` .  . `, ` \__/ `},
	Unimpressed: {` -  - `, `      `, `  __  `},
	Judging:     {` =  = `, `      `, `  ~~  `},
	Hostile:     {` \  / `, ` o  o `, ` /^^\ `},
	Furious:     {` >  < `, ` #  # `, `/VVVV\`},
	InLove:      {`<3  <3`, `//  //`, `  ww  `},
	Crying:      {` T  T `, ` ;  ; `, `  ()  `},
}

// ParseMood accepts a tier number ("0"–"4") or a name ("furious", "love", "crying").
func ParseMood(s string) (Mood, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	for m, name := range moodNames {
		if s == fmt.Sprint(m) || s == name || strings.HasPrefix(name, s+" ") || (s == "love" && Mood(m) == InLove) {
			return Mood(m), true
		}
	}
	return Content, false
}

var swap = strings.NewReplacer(`/`, `\`, `\`, `/`, `(`, `)`, `)`, `(`, `<`, `>`, `>`, `<`)

func mirror(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return swap.Replace(string(r))
}

// Portrait returns her ASCII portrait for a mood, one string per line.
func Portrait(m Mood) []string {
	f, ok := faces[m]
	if !ok {
		f = faces[Content]
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		left, mid := r[0], r[1]
		switch mid {
		case "EYES":
			mid = f[0]
		case "CHEEK":
			mid = f[1]
		case "MOUTH":
			mid = f[2]
		}
		out = append(out, left+mid+mirror(left))
	}
	return out
}
