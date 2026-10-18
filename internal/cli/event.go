// internal/cli/event.go
package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/config"
	"github.com/spiritpaper/cursed-git/internal/curses"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// what an event file may set (docs/EVENTS.md). Death March is NOT here, on purpose: that
// one only ever happens by the player's own hand.
var eventKeys = []string{
	"event.name", "event.submit", "event.contact",
	"cursed.preset", "cursed.personality", "cursed.eventhours", "cursed.eventends",
	"cursed.breakuplevel", "cursed.minbossfightswon", "cursed.minlinescommitted",
	"cursed.toxicthreshold", "cursed.xpmaturity",
	"cursed.speed", "cursed.deadline", "cursed.fighttime", "cursed.decay",
}

// eventCmd: cg event (what event am I in?) | cg event join <file or URL>
func eventCmd(args []string, stdout, stderr io.Writer) int {
	gitDir, err := repo.Find(".")
	if err != nil {
		return fail(stderr, "event", err)
	}
	if len(args) == 2 && args[0] == "join" {
		return eventJoin(gitDir, args[1], stdout, stderr)
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: cg event | cg event join <file or URL>")
		return 1
	}
	cfg := config.Load(gitDir)
	name := cfg.Get("event.name")
	if name == "" && curses.EventHours(gitDir) == 0 {
		fmt.Fprintln(stdout, "This repo isn't part of an event. Your organizer gives you a file: cg event join <file or URL>")
		return 0
	}
	printEvent(gitDir, stdout)
	return 0
}

func printEvent(gitDir string, w io.Writer) {
	cfg := config.Load(gitDir)
	name := cfg.Get("event.name")
	if name == "" {
		name = "an event"
	}
	fmt.Fprintf(w, "📣  %s\n", name)
	env := curseEnv(io.Discard)
	her, _ := curses.Open(gitDir, env)
	if her != nil {
		if left, ok := her.EventLeft(env.Now()); ok {
			fmt.Fprintf(w, "    ⏳ %s\n", eventLeft(left))
		}
		r := her.Verdict().Rules
		fmt.Fprintf(w, "    to earn your closure: level %d, %d boss fights won, %d lines committed\n", r.BreakupLevel, r.MinWins, r.MinLines)
		fmt.Fprintf(w, "    playing with: %s (%s)\n", her.Name(), her.State().Preset)
		c, tier := curses.ClocksFor(gitDir), her.State().Tier(env.Now())
		if d := c.Deadline(tier); d > 0 {
			fmt.Fprintf(w, "    clocks: commit within %s, %s per conflict block", short(d), short(c.Fight(max(tier, 3))))
			if dec := c.Decay(max(tier, 3)); dec > 0 {
				fmt.Fprintf(w, ", untouched files rot in %s", short(dec))
			}
			fmt.Fprintln(w)
		}
	}
	if s := cfg.Get("event.submit"); s != "" {
		fmt.Fprintf(w, "    hand in your card: cg card --json > team.json, then %s\n", s)
	}
	if c := cfg.Get("event.contact"); c != "" {
		fmt.Fprintf(w, "    stuck? %s\n", c)
	}
}

// eventJoin applies an organizer's event file to this repo and invites the character it names.
func eventJoin(gitDir, src string, stdout, stderr io.Writer) int {
	text, err := readEventFile(src)
	if err != nil {
		return fail(stderr, "event join", err)
	}
	ev := config.Parse(text)
	if ev.Get("cursed.deathmarch") != "" {
		fmt.Fprintln(stderr, "cg event join: this event file tries to turn on Death March. It can't: only you can, by hand, in .git/config. Skipped.")
	}
	var keys []string
	for k := range ev {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	set := 0
	for _, k := range keys {
		if !contains(eventKeys, k) {
			continue
		}
		if code := toGit([]string{"--git-dir", gitDir, "config", k, ev[k]}, stdout, stderr); code != 0 {
			return code
		}
		set++
	}
	if set == 0 {
		fmt.Fprintln(stderr, "cg event join: nothing in that file is an event setting (see docs/EVENTS.md)")
		return 1
	}
	preset := ev.Get("cursed.preset")
	if preset == "" {
		preset = "hackathon"
	}
	if code := gentleSetup(gitDir, preset, stdout, stderr); code != 0 {
		return code
	}
	her, err := curses.Enable(gitDir, preset, curseEnv(stderr))
	if err != nil {
		return fail(stderr, "event join", err)
	}
	fmt.Fprintln(stdout, "Joined.")
	printEvent(gitDir, stdout)
	her.Say("curse_on", nil)
	printHowToPlay(stderr, her)
	her.AnnounceNew()
	return 0
}

// readEventFile reads a local file, or downloads one (small, with a timeout).
func readEventFile(src string) (string, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		data, err := os.ReadFile(src)
		return string(data), err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", src, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return string(data), err
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// short prints a duration the way people say it: 30m, 1h30m, 8h, 2d.
func short(d time.Duration) string {
	switch {
	case d >= 48*time.Hour && d%(24*time.Hour) == 0:
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d >= time.Hour:
		return fmt.Sprintf("%dh%dm", d/time.Hour, (d%time.Hour)/time.Minute)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return d.Round(time.Second).String()
}
