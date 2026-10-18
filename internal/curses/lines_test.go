package curses

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func testEngine(t *testing.T, pack string, now time.Time) *Engine {
	t.Helper()
	e := &Engine{gitDir: t.TempDir(), st: Replay(nil), env: Env{
		Now: func() time.Time { return now }, Roll: func() float64 { return 0.99 }, Pick: func(n int) int { return 0 },
		In: bufio.NewReader(strings.NewReader("")), Out: io.Discard, Sleep: func(time.Duration) {},
	}}
	if err := json.Unmarshal([]byte(pack), &e.lines); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(dirOf(e.gitDir), 0755) // where she keeps what she said lately
	return e
}

var tuesday = time.Date(2026, 10, 13, 14, 0, 0, 0, time.Local)

const pack = `{"react": [
  "plain",
  {"text": "wip", "when": {"msg": ["wip", "work in progress"]}},
  {"text": "wip at night", "when": {"msg": ["wip"], "hour": "22-4"}},
  {"text": "emoji", "when": {"msg_emoji": true}},
  {"text": "angry", "when": {"tier": "3-4", "min": {"toxicity": 5}}}
]}`

func TestTheMostSpecificLineWins(t *testing.T) {
	e := testEngine(t, pack, tuesday)
	for msg, want := range map[string]string{
		"wip: login page":          "wip",
		"still a Work In Progress": "wip",   // phrases, any case
		"wiped the cache":          "plain", // whole words only
		"ship it 🚀":                "emoji",
		"refactor":                 "plain",
	} {
		if got := e.pick("react", map[string]string{"msg": msg}); got != want {
			t.Errorf("%q -> %q, want %q", msg, got, want)
		}
		e.saveSeen(seen{}) // forget what she said, for the next case
	}
	night := testEngine(t, pack, time.Date(2026, 10, 13, 2, 0, 0, 0, time.Local))
	if got := night.pick("react", map[string]string{"msg": "wip"}); got != "wip at night" {
		t.Errorf("at 2am the two-condition line wins, got %q", got)
	}
}

func TestFactsAndTiers(t *testing.T) {
	e := testEngine(t, pack, tuesday)
	e.st.Base, e.st.Max, e.st.Toxicity = 3, 4, 6
	if got := e.pick("react", nil); got != "angry" {
		t.Errorf("Tier 3 with toxicity 6: got %q", got)
	}
	e.st.Toxicity = 2
	e.saveSeen(seen{})
	if got := e.pick("react", nil); got != "plain" {
		t.Errorf("toxicity 2 is below the min: got %q", got)
	}
}

func TestSheDoesntRepeatHerself(t *testing.T) {
	e := testEngine(t, `{"k": ["one", "two", "three"]}`, tuesday)
	var got []string
	for i := 0; i < 3; i++ {
		got = append(got, e.pick("k", nil))
	}
	if strings.Join(got, ",") != "one,two,three" {
		t.Errorf("three in a row should all be different: %v", got)
	}
	// a special line she just said gives way to the general ones
	e = testEngine(t, pack, tuesday)
	first := e.pick("react", map[string]string{"msg": "wip"})
	second := e.pick("react", map[string]string{"msg": "wip"})
	if first != "wip" || second != "plain" {
		t.Errorf("wip twice: %q then %q, want wip then plain", first, second)
	}
}

func TestSpecificOnly(t *testing.T) {
	e := testEngine(t, pack, tuesday)
	if _, ok := e.choose("react", map[string]string{"msg": "add a button"}, true); ok {
		t.Error("a reaction without a matching rule should stay silent")
	}
	if s, ok := e.choose("react", map[string]string{"msg": "wip button"}, true); !ok || s.Text != "wip" {
		t.Errorf("got %q %v", s.Text, ok)
	}
}

func TestOldPacksStillLoad(t *testing.T) {
	var lines map[string][]Line
	if err := json.Unmarshal([]byte(`{"a": ["x", "y"], "b": [{"text": "z", "when": {"tier": "2"}}]}`), &lines); err != nil {
		t.Fatal(err)
	}
	if len(lines["a"]) != 2 || lines["b"][0].When.Tier != "2" {
		t.Errorf("%+v", lines)
	}
	if err := json.Unmarshal([]byte(`{"a": [{"when": {"tier": "2"}}]}`), &lines); err == nil {
		t.Error("a line without text is an error (the pack goes mute, git keeps working)")
	}
}

func TestInRange(t *testing.T) {
	for _, c := range []struct {
		spec  string
		v     int
		wraps bool
		want  bool
	}{{"3", 3, false, true}, {"3-4", 4, false, true}, {"3-4", 2, false, false}, {"22-4", 23, true, true}, {"22-4", 3, true, true}, {"22-4", 12, true, false}, {"x", 1, false, false}} {
		if got := inRange(c.spec, c.v, c.wraps); got != c.want {
			t.Errorf("inRange(%q, %d) = %v", c.spec, c.v, got)
		}
	}
}

func TestBingo(t *testing.T) {
	var events []Event
	for i := 0; i < RedFlags; i++ {
		events = append(events, Event{Type: "flag", Text: string(rune('a' + i)), T: int64(i + 1)})
	}
	st := Replay(events[:RedFlags-1])
	if _, ok := st.Unlocked["bingo"]; ok {
		t.Error("ten isn't bingo")
	}
	if st = Replay(events); st.Unlocked["bingo"] == 0 {
		t.Error("eleven red flags should unlock Bingo")
	}
}

func TestReceiptsOnlyAfterAClaim(t *testing.T) {
	e := testEngine(t, `{"_name": ["Miranda"]}`, tuesday)
	e.workTree = e.gitDir
	if strings.Contains(strings.Join(e.Card(tuesday), "\n"), "receipts") {
		t.Error("no claim, no receipts")
	}
	e.st.Flags["love_costs_nothing"] = true
	if card := strings.Join(e.Card(tuesday), "\n"); !strings.Contains(card, "She claims everything. Receipts: 0 commits.") {
		t.Errorf("after a claim, the receipts:\n%s", card)
	}
}

// every red flag has at least one line in Miranda's pack: Bingo is reachable
func TestEveryRedFlagHasALine(t *testing.T) {
	pack, ok := readPack("miranda")
	if !ok {
		t.Fatal("no miranda pack")
	}
	flags := map[string]bool{}
	for _, lines := range pack {
		for _, l := range lines {
			if l.Flag != "" {
				flags[l.Flag] = true
			}
		}
	}
	flags["hoover"] = true // raised by the hoover itself (texts.go), not by a line
	if len(flags) != RedFlags {
		t.Errorf("%d red flags have lines, want %d: %v", len(flags), RedFlags, flags)
	}
}
