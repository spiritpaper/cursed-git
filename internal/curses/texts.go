// internal/curses/texts.go
//
// Texts from her: when you come back after a while, she doesn't just say hi. You find the
// messages she sent while you were gone, timestamped, getting worse. (And sometimes she's
// typing... and then she's deleting. Borrowed from Emily Is Away: what was she going to say?)
//
// And the hoover: after the relationship is over (the toxic ending, or the good one), she
// comes back. Days later, in whatever project you're in, in a new costume each time. Typing
// `no` blocks her. Each costume comes once; seeing all of them is how you know you're free.
package curses

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// textsFrom shows the stack of messages she sent while you were away. Returns false if her
// pack has no texts (then the old one-line greeting is used).
func (e *Engine) textsFrom(away, now time.Time) bool {
	var msgs []string
	for _, key := range []string{"texts_1", "texts_2", "texts_3"} {
		if line := e.pick(key, nil); line != "" {
			msgs = append(msgs, line)
		}
	}
	if len(msgs) == 0 {
		return false
	}
	gap := now.Sub(away)
	stamp := "15:04"
	if gap >= 24*time.Hour {
		stamp = "Mon 15:04"
	}
	at := []float64{0.1, 0.5, 0.9}
	out := e.env.Out
	e.pause()
	fmt.Fprintf(out, "  ┌ %d messages from %s while you were gone\n", len(msgs), e.Name())
	for i, m := range msgs {
		when := away.Add(time.Duration(float64(gap) * at[i]))
		fmt.Fprintf(out, "  │ %s  %s\n", when.Format(stamp), m)
	}
	fmt.Fprintln(out, "  └")
	if !e.Gentle() && e.chance(0.5) {
		e.env.Sleep(900 * time.Millisecond)
		fmt.Fprintf(out, "    %s is typing…\n", e.Name())
		e.env.Sleep(1500 * time.Millisecond)
		fmt.Fprintf(out, "    %s is deleting…\n", e.Name())
	}
	return true
}

// ---- the hoover ----

// the costumes she comes back in, in order (Miranda's pack: hoover_<costume>)
var costumes = []string{"therapy", "blame", "tired", "declaration"}

// how long after the last one before she tries again
const hooverEvery = 3 * 24 * time.Hour

type hooverState struct {
	LastAt  int64    `json:"last_at"`
	Seen    []string `json:"seen"`    // costumes she's shown up in
	Blocked []string `json:"blocked"` // the ones you said no to
}

func hooverFile() string { return filepath.Join(MemoryDir(), "hoover.json") }

func loadHoover() hooverState {
	var h hooverState
	if data, err := os.ReadFile(hooverFile()); err == nil {
		json.Unmarshal(data, &h)
	}
	return h
}

func saveHoover(h hooverState) {
	if MemoryDir() == "" || os.MkdirAll(MemoryDir(), 0755) != nil {
		return
	}
	if data, err := json.MarshalIndent(h, "", "  "); err == nil {
		os.WriteFile(hooverFile(), data, 0644)
	}
}

// overWith: the project where the relationship ended (toxic or good ending), if any.
func overWith() string {
	for _, m := range loadMemories() {
		if m.Ending == "B" || m.Ending == "C" {
			return filepath.Base(m.Path)
		}
	}
	return ""
}

// hoover runs before commands: if it's over with Miranda, sometimes she's back.
func (e *Engine) hoover(now time.Time) {
	where := overWith()
	if where == "" || MemoryDir() == "" {
		return
	}
	h := loadHoover()
	if len(h.Seen) >= len(costumes) || (h.LastAt != 0 && now.Unix()-h.LastAt < int64(e.clocks().Every(hooverEvery).Seconds())) {
		return
	}
	if h.LastAt == 0 {
		// the first time, she waits a few days after the ending. Start counting now.
		h.LastAt = now.Unix()
		saveHoover(h)
		return
	}
	miranda, ok := readPack(DefaultPersonality)
	if !ok {
		return
	}
	costume := costumes[len(h.Seen)]
	out := e.env.Out
	e.pause()
	fmt.Fprintf(out, "  ┌ %s (from %s, where you left her)\n", first(miranda["_name"]), where)
	for _, l := range miranda["hoover_"+costume] {
		e.env.Sleep(700 * time.Millisecond)
		fmt.Fprintf(out, "  │ %s\n", e.fill(l.Text, nil))
	}
	fmt.Fprintln(out, "  └")
	fmt.Fprint(out, "  Reply? (no blocks her) > ")
	answer, _ := e.env.In.ReadString('\n')
	h.Seen = append(h.Seen, costume)
	h.LastAt = now.Unix()
	if strings.EqualFold(strings.TrimSpace(answer), "no") {
		h.Blocked = append(h.Blocked, costume)
		fmt.Fprintln(out, "  Blocked.")
	} else {
		fmt.Fprintln(out, "  (She saw that you read it.)")
	}
	saveHoover(h)
	if !e.st.Flags["hoover"] {
		e.log(Event{Type: "flag", Text: "hoover"})
	}
	if e.Gentle() {
		e.say("after_hoover", nil) // the partner who's actually here
	}
}
