// internal/cli/her.go
package cli

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/curses"
	"github.com/spiritpaper/cursed-git/internal/repo"
)

// curseEnv connects her to the real world. Tests replace it with a fake clock, a seeded
// random source and a pretend terminal.
var curseEnv = func(stderr io.Writer) curses.Env {
	return curses.Env{
		Now:  time.Now,
		Roll: rand.Float64,
		Pick: rand.Intn,
		// only a person at a terminal gets cursed: scripts, CI and pipes get plain git
		Interactive: isTerminal(os.Stdin) && os.Getenv("CI") == "",
		In:          bufio.NewReader(os.Stdin),
		Out:         stderr,
		Sleep:       time.Sleep,
	}
}

// Run executes one cg command and returns the process exit code. She gets a say before
// and after every command (if this repo has her and you're at a terminal).
// It never calls os.Exit itself, so tests can call it directly.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "init" {
		return dispatch(args, stdout, stderr)
	}
	var her *curses.Engine
	if gitDir, err := repo.Find("."); err == nil {
		her, _ = curses.Open(gitDir, curseEnv(stderr))
	}
	cmd := args[0]
	if cmd == "hook" {
		return hookCmd(her, args[1:], stderr)
	}
	if cmd != "forget" { // forgetting and then remembering in the same breath would undo it
		defer her.Touch() // she remembers what the repo looked like when she last saw you
	}
	rest, refused := her.Before(cmd, args[1:])
	if refused {
		return 1
	}
	code := dispatch(append([]string{cmd}, rest...), stdout, stderr)
	her.After(cmd, rest, code)
	return code
}

// hookStdin is what git feeds the hook (ref updates). Tests replace it.
var hookStdin io.Reader = os.Stdin

// hookCmd runs inside git: .git/hooks/<name> calls `cg hook <name> <args>`.
// She only speaks if git's stderr is your terminal; she never breaks git by accident.
func hookCmd(her *curses.Engine, args []string, stderr io.Writer) int {
	if her == nil || len(args) == 0 {
		return 0
	}
	speak := isTerminal(os.Stderr)
	switch args[0] {
	case "reference-transaction":
		state := ""
		if len(args) > 1 {
			state = args[1]
		}
		return her.HookRefTransaction(state, hookStdin, stderr, speak)
	case "pre-commit":
		return her.HookPreCommit(stderr)
	}
	return 0
}

// herCmd: cg mood | beg | graveyard | curse on [--preset p] | curse off | restraining-order
func herCmd(args []string, stdout, stderr io.Writer) int {
	gitDir, err := repo.Find(".")
	if err != nil {
		fmt.Fprintf(stderr, "cg %s: %v\n", args[0], err)
		return 1
	}
	env := curseEnv(stderr)

	if args[0] == "forget" {
		all := len(args) > 1 && args[1] == "--all"
		if curses.Forget(gitDir, all) {
			fmt.Fprintln(stdout, "Forgotten. (She'd never forget you. But the file is gone.)")
		} else {
			fmt.Fprintln(stdout, "She doesn't remember anything about this. Lucky you.")
		}
		return 0
	}

	if args[0] == "curse" {
		if len(args) < 2 || (args[1] != "on" && args[1] != "off") {
			fmt.Fprintln(stderr, "usage: cg curse on [--preset clingy|toxic|psycho] | cg curse off")
			return 1
		}
		if args[1] == "off" {
			args = []string{"restraining-order"}
		} else {
			preset := ""
			if len(args) == 4 && args[2] == "--preset" {
				preset = args[3]
			}
			her, err := curses.Enable(gitDir, preset, env)
			if err != nil {
				fmt.Fprintf(stderr, "cg curse: %v\n", err)
				return 1
			}
			her.Say("curse_on", nil)
			printHowToPlay(stderr, her)
			her.AnnounceNew()
			return 0
		}
	}

	her, err := curses.Open(gitDir, env)
	if err != nil {
		fmt.Fprintf(stderr, "cg %s: %v\n", args[0], err)
		return 1
	}
	if her == nil {
		fmt.Fprintln(stderr, "She isn't in this repo. `cg curse on` to invite her (you've been warned).")
		return 1
	}

	switch args[0] {
	case "breakup":
		return her.Breakup(len(args) > 1 && args[1] == "--dry-run", stdout)

	case "card":
		fmt.Fprintln(stdout, strings.Join(her.Card(env.Now()), "\n"))
		return 0

	case "achievements":
		got := her.Unlocked()
		fmt.Fprintf(stdout, "Achievements: %d/%d\n\n", len(got), len(curses.Achievements))
		for _, a := range curses.Achievements {
			if at, ok := got[a.ID]; ok {
				when := ""
				if at > 0 {
					when = "  (" + time.Unix(at, 0).Format("Jan 2") + ")"
				}
				fmt.Fprintf(stdout, "  🏆 %-18s %s%s\n", a.Name, a.Desc, when)
			} else {
				fmt.Fprintf(stdout, "  ·  %-18s %s\n", a.Name, a.Desc)
			}
		}
		return 0

	case "restraining-order":
		her.Off()
		fmt.Fprintln(stdout, "Restraining order filed. She's off in this repo. (`cg curse on` if you ever miss her.)")
		return 0

	case "beg":
		paths, err := her.Beg()
		if err != nil {
			fmt.Fprintf(stderr, "cg beg: %v\n", err)
			return 1
		}
		if len(paths) == 0 {
			fmt.Fprintln(stdout, "Beg for what? She hasn't taken anything. Yet.")
			return 0
		}
		for _, p := range paths {
			fmt.Fprintf(stdout, "restored %s\n", p)
		}
		her.Say("beg", map[string]string{"n": fmt.Sprint(len(paths))})
		return 0

	case "graveyard":
		st := her.State()
		if len(st.Graveyard) == 0 && len(st.Sacrificed) == 0 {
			fmt.Fprintln(stdout, "The graveyard is empty. For now.")
			return 0
		}
		for _, ref := range st.Graveyard {
			fmt.Fprintf(stdout, "%s\n", ref)
		}
		if len(st.Graveyard) > 0 {
			fmt.Fprintln(stdout, "\n`cg beg` brings back the newest one. Plain git works too: git checkout <ref> -- <file>")
		}
		if len(st.Sacrificed) > 0 {
			if len(st.Graveyard) > 0 {
				fmt.Fprintln(stdout)
			}
			fmt.Fprintln(stdout, "Sacrificed commits:")
			for _, ref := range st.Sacrificed {
				fmt.Fprintf(stdout, "%s\n", ref)
			}
			fmt.Fprintln(stdout, "\n`cg cherry-pick <ref>` replays one on top of your branch.")
		}
		return 0

	default: // mood
		printMood(stdout, her, env.Now())
		return 0
	}
}

func printMood(w io.Writer, her *curses.Engine, now time.Time) {
	st := her.State()
	tier := st.Tier(now)
	m := curses.Mood(tier)
	switch {
	case !st.On:
		m = curses.Unimpressed
	case st.Lovebombing(now):
		m = curses.InLove
	}
	printFace(w, m)
	if !st.On {
		fmt.Fprintln(w, "She's under a restraining order in this repo. `cg curse on` to undo it.")
		return
	}
	temper := fmt.Sprint(st.Temper(now))
	if st.Temper(now) > 0 {
		temper = "+" + temper
	}
	fmt.Fprintf(w, "Tier %d · %s    temper %s    preset %s (starts at %d, never above %d)\n",
		tier, curses.TierName(tier), temper, st.Preset, st.Base, st.Max)
	if her.DeathMarch() {
		fmt.Fprintln(w, "☠  DEATH MARCH: shutdowns and deletions are real.")
	}
	last := "no commits yet" // from the real history, not her memory: she never lies about your data
	if her.HasCommits() {
		last = "last commit " + now.Sub(time.Unix(her.HeadCommitAt(), 0)).Round(time.Minute).String() + " ago"
	}
	fmt.Fprintf(w, "You: toxicity %d/%d · %s · %s\n", st.Toxicity, curses.ToxicThreshold, plural(len(st.Grudges), "grudge"), last)
	if limit := curses.Deadline(tier); limit > 0 {
		fmt.Fprintf(w, "Commit deadline at this tier: %s\n", limit)
	}
	if n := len(st.Grudges); n > 0 {
		last := st.Grudges[max(0, n-3):]
		fmt.Fprintf(w, "She remembers: %s\n", strings.Join(last, "; "))
	}
	if n := len(st.Graveyard); n > 0 {
		fmt.Fprintf(w, "She has %d of your things. (`cg beg`)\n", n)
	}
	fmt.Fprintln(w, her.ClosureLine())
	if h := her.History(); h.Projects > 1 {
		fmt.Fprintf(w, "Across %d projects together: %d fights won, %d lost, %s\n", h.Projects, h.Won, h.Lost, plural(h.Breakups, "breakup"))
	}
	if st.BossWon+st.BossLost > 0 {
		fmt.Fprintf(w, "Boss fights: %d won, %d lost\n", st.BossWon, st.BossLost)
	}
	if bar := her.FightBar(now); bar != "" {
		fmt.Fprintln(w, bar)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// printHowToPlay is the card she hands you when you invite her in.
func printHowToPlay(w io.Writer, her *curses.Engine) {
	st := her.State()
	fmt.Fprintf(w, `
  How to play (preset %s: she starts at Tier %d, never goes above %d)
    calms her   good commit messages, committing often, winning boss fights
    angers her  lazy messages ("fix", "wip"), ignoring her deadlines, force-pushing,
                using plain git behind her back
    XP          every committed line, decent messages, won fights: leave her at Lv %d
                (cg breakup)
    check in    cg mood     safe word  cg restraining-order (stops her, no questions)
    lost files  cg beg / cg graveyard   (she saves everything before she takes it)

`, st.Preset, st.Base, st.Max, her.Verdict().Rules.BreakupLevel)
}
