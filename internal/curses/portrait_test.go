package curses

import (
	"testing"
	"unicode/utf8"
)

func TestEveryMoodLinesUp(t *testing.T) {
	want := utf8.RuneCountInString(Portrait(Content)[0])
	for m := Content; m <= Crying; m++ {
		for i, line := range Portrait(m) {
			if n := utf8.RuneCountInString(line); n != want {
				t.Errorf("%v row %d is %d wide, want %d: %q", m, i, n, want, line)
			}
		}
	}
}

func TestFaceSlotsAreSixWide(t *testing.T) {
	for m, f := range faces {
		for i, part := range f {
			if n := utf8.RuneCountInString(part); n != 6 {
				t.Errorf("%v part %d is %d wide: %q", m, i, n, part)
			}
		}
	}
}

func TestHalvesAreMirrorImages(t *testing.T) {
	// the frame (everything but the face rows) must be perfectly symmetric
	for i, line := range Portrait(Content) {
		if rows[i][1] == "EYES" || rows[i][1] == "CHEEK" || rows[i][1] == "MOUTH" {
			continue
		}
		if mirror(line) != line {
			t.Errorf("row %d is not symmetric: %q", i, line)
		}
	}
}

func TestParseMood(t *testing.T) {
	for in, want := range map[string]Mood{"0": Content, "4": Furious, "furious": Furious, "love": InLove, "judging": Judging, "crying": Crying} {
		if got, ok := ParseMood(in); !ok || got != want {
			t.Errorf("ParseMood(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	if _, ok := ParseMood("happy-ish"); ok {
		t.Error("unknown mood should not parse")
	}
}
