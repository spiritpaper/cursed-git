// internal/curses/engine.go
//
// The engine runs around every command (cg's own and the ones passed to git):
//
//	Before(cmd, args) -> she may comment, refuse the command, or rewrite its arguments
//	After(cmd, args, exitCode) -> she reacts to what happened
//
// She only acts when a real person is at a terminal (Env.Interactive). Scripts, CI and
// tests get plain git behaviour. Her voice goes to stderr so piped output stays clean.
package curses

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/config"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

//go:embed lines/*.json
var packs embed.FS

// Env is everything from the outside world, so tests can control time, luck and the terminal.
type Env struct {
	Now         func() time.Time
	Roll        func() float64  // uniform in [0,1): her luck
	Pick        func(n int) int // a choice in [0,n)
	Interactive bool
	In          *bufio.Reader
	Out         io.Writer
	Sleep       func(time.Duration)
}

type Engine struct {
	gitDir, workTree string
	env              Env
	diary            *Diary
	st               *State
	lines            map[string][]Line
	tampered         error
	pre              commitInfo       // what Before learned about a commit, for After
	gone             bool             // she left (breakup): write nothing more into the repo
	phase2           bool             // the pull right after a Tier 4 win: a conflict is the rematch
	pr               *pullRequest     // the pull request she's merging: a conflict is a fight with its author
	ending           string           // set by a breakup, for her memory
	achBefore        map[string]int64 // achievements when the command started
	charmedNow       bool             // she was wonderful during this command (the pull comes later)
}

type commitInfo struct {
	msg           string
	lazy          bool
	staged        int
	onlyDeletions bool
	unrotted      int
	hp            int  // boss HP before a `cg add`, to see if it was a hit
	merging       bool // this commit finishes a merge (git's MERGE_HEAD)
}

// Open loads her for a repository. It returns nil (and no error) if this repo never opted in.
func Open(gitDir string, env Env) (*Engine, error) {
	if !OptedIn(gitDir) {
		return nil, nil
	}
	d, err := OpenDiary(gitDir)
	e := &Engine{gitDir: gitDir, workTree: filepath.Dir(gitDir), env: env, diary: d}
	if errors.Is(err, ErrTampered) {
		e.tampered = err
	} else if err != nil {
		return nil, err
	}
	e.lines = loadLines(gitDir)
	e.st = Replay(d.Events())
	e.achBefore = e.Unlocked()
	return e, nil
}

// Enable opts a repository in (cg init, cg curse on). preset "" keeps the current one.
func Enable(gitDir, preset string, env Env) (*Engine, error) {
	d, err := OpenDiary(gitDir)
	if err != nil && !errors.Is(err, ErrTampered) {
		return nil, err
	}
	var remembered *memory
	if len(d.Events()) == 0 && preset != "gentle" {
		// a repo she's never lived in... or one she has, before you deleted .git or re-cloned it
		if snap, m, ok := carryOver(gitDir); ok {
			if err := d.Append(Event{T: env.Now().Unix(), Type: "carry", Carry: &snap}); err != nil {
				return nil, err
			}
			remembered = &m
		}
	}
	st := Replay(d.Events())
	if preset == "" {
		preset = st.Preset
	}
	if preset == "" {
		preset = DefaultPreset
	}
	p, ok := Presets[preset]
	if !ok {
		return nil, fmt.Errorf("unknown preset %q (clingy, toxic, psycho, hackathon or gentle)", preset)
	}
	now := env.Now().Unix()
	if err := d.Append(Event{T: now, Type: "on", Text: preset, Tier: p[0], Max: p[1]}); err != nil {
		return nil, err
	}
	// whoever turns her on is "known"; anyone else committing here makes her jealous
	if email := config.Load(gitDir).Get("user.email"); email != "" && !st.KnownAuthors[email] {
		if err := d.Append(Event{T: now, Type: "author", Text: email}); err != nil {
			return nil, err
		}
	}
	if err := InstallHooks(gitDir); err != nil {
		return nil, err
	}
	e, err := Open(gitDir, env)
	if err == nil && remembered != nil && env.Interactive {
		g := "everything"
		if n := len(remembered.Grudges); n > 0 {
			g = remembered.Grudges[n-1]
		}
		e.say("remembered", map[string]string{"grudge": g, "project": filepath.Base(remembered.Path)})
	}
	if err == nil {
		e.remember()
	}
	return e, err
}

func (e *Engine) State() *State { return e.st }

// GitDir is the repository she lives in.
func (e *Engine) GitDir() string { return e.gitDir }

func (e *Engine) log(ev Event) {
	if e.gone {
		return
	}
	if e.Gentle() && punishment(ev) {
		return // she cherishes you: nothing you do is held against you
	}
	ev.T = e.env.Now().Unix()
	if err := e.diary.Append(ev); err == nil {
		e.st = Replay(e.diary.Events())
	}
}

// punishment: the events that make things worse for you.
func punishment(ev Event) bool {
	switch ev.Type {
	case "toxicity", "temper":
		return ev.Delta > 0
	case "grudge", "offense", "apology":
		return true
	}
	return false
}

func (e *Engine) chance(p float64) bool { return e.env.Roll() < p }

func (e *Engine) pick(key string, vars map[string]string) string {
	l, _ := e.choose(key, vars, false)
	e.raised(l)
	return e.fill(l.Text, vars)
}

// raised: a line that's one of her red flags gets quietly logged (Red Flag Bingo).
func (e *Engine) raised(l Line) {
	if l.Flag != "" && !e.st.Flags[l.Flag] {
		e.log(Event{Type: "flag", Text: l.Flag})
	}
}

// fill puts the placeholders in.
func (e *Engine) fill(s string, vars map[string]string) string {
	for k, v := range e.vars(vars) {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// sayIf speaks only if a line with conditions matches (a reaction, not a comment on everything).
func (e *Engine) sayIf(key string, vars map[string]string) {
	if l, ok := e.choose(key, vars, true); ok {
		e.raised(l)
		e.speak(key, e.fill(l.Text, vars))
	}
}

const gentleFace = "( ^‿^ )"

var miniFaces = []string{"( ^_^ )", "( -_- )", "( ¬_¬ )", "( ò_ó )", "(╬ ò_ó)"}

// say prints one of her lines, with the small face for her current tier.
func (e *Engine) say(key string, vars map[string]string) {
	e.speak(key, e.pick(key, vars))
}

// speak prints a line with the small face for her current tier.
func (e *Engine) speak(key, line string) {
	if line == "" {
		return
	}
	now := e.env.Now()
	face := miniFaces[e.st.Tier(now)]
	if e.st.Lovebombing(now) || key == "love" {
		face = "( ♥_♥ )"
	}
	if e.Gentle() {
		face = gentleFace
	}
	fmt.Fprintf(e.env.Out, "%s  %s\n", face, line)
}

// commands that are about her, not about git: no curses on those
var exempt = map[string]bool{"breakup": true, "forget": true, "card": true, "achievements": true, "quest": true, "rivals": true, "partner": true, "event": true, "mood": true, "curse": true, "restraining-order": true, "beg": true,
	"graveyard": true, "face": true, "help": true, "version": true, "--version": true, "-h": true, "--help": true}

func (e *Engine) awake() bool { return e != nil && e.env.Interactive }

// Before runs before a command. It returns the (maybe rewritten) arguments and whether
// she refuses to run the command at all.
func (e *Engine) Before(cmd string, args []string) ([]string, bool) {
	if !e.awake() {
		return args, false
	}
	now := e.env.Now()
	if e.tampered != nil {
		e.readMyDiary(now)
	}
	if !e.st.On || cmd == "curse" || cmd == "restraining-order" {
		return args, false
	}
	e.pre = commitInfo{}

	// what happened while she wasn't looking? (also before `cg mood`: that's when she'd bring it up)
	last, known := e.loadSeen()
	e.behindHerBack(last, known)
	e.checkLocks()
	e.missingGraves()
	e.greet(now, last, known)
	e.hoover(now)
	e.letterReminder()
	e.deathMarchGate()
	e.matureXP(now)
	if e.fightBefore(cmd, args, now, exempt[cmd]) {
		return args, true
	}
	if exempt[cmd] {
		return args, false
	}

	tier, love := e.st.Tier(now), e.st.Lovebombing(now)
	if cmd == "add" || cmd == "commit" {
		e.pre.unrotted = e.unrot() // rot never gets staged
	}
	e.pre.merging = cmd == "commit" && e.exists("MERGE_HEAD") && e.st.Fight == nil
	// in the middle of a merge, nothing touches your files but you (and git)
	if !love && !e.inProgress() {
		e.decay(now, tier, last)
		e.randomPull(cmd, tier)
		e.commitDeadline(now, tier)
	}
	if !love {
		if tier >= 1 && now.Unix()-e.st.LastRefusalAt > 600 && e.chance(0.03) {
			e.say("arbitrary_refusal", nil)
			e.log(Event{Type: "refusal"})
			return args, true
		}
		if tier >= 1 && e.st.Temper(now) >= 1 && e.chance(0.2) {
			e.say("silent", nil)
			if e.chance(0.5) {
				e.say("space", nil)
				e.env.Sleep(3 * time.Second)
			}
		}
	}

	if crime := rewriting(cmd, args); crime != "" && tier >= 1 && !love {
		e.say("pretend", map[string]string{"crime": crime})
		e.log(Event{Type: "temper", Delta: 1, Text: "rewriting history"})
	}
	switch cmd {
	case "commit":
		return e.beforeCommit(args, now, tier, love)
	case "push":
		if isForce(args) && tier >= 2 && !love && !e.apologize(now) {
			return args, true
		}
	case "checkout", "switch", "branch":
		if name := newBranchName(cmd, args); name != "" && tier >= 1 && !love && embarrassing(name) {
			e.say("branch_name", map[string]string{"branch": name})
		}
	}
	return args, false
}

// After runs after a command, with its exit code.
func (e *Engine) After(cmd string, args []string, code int) {
	if !e.awake() || !e.st.On || exempt[cmd] {
		return
	}
	now := e.env.Now()
	if cmd == "commit" && code == 0 {
		e.pre.unrotted += e.unrot() // any commit shows you're still around
	}
	if e.pre.unrotted > 0 && (cmd == "add" || cmd == "commit") {
		e.say("decay_restored", map[string]string{"n": fmt.Sprint(e.pre.unrotted)})
	}
	e.fightAfter(cmd, now)
	if cmd == "commit" && code == 0 && e.pre.merging {
		e.log(Event{Type: "conflict_resolved"}) // finished a merge by hand, without a fight
	}
	e.checkQuest(cmd, args, code)
	switch {
	case cmd == "commit" && code == 0:
		e.afterCommit(now)
		e.maybeSacrifice(now)
	case cmd == "push" && code == 0 && !e.st.Lovebombing(now) && e.chance(0.3):
		e.say("toxic_partner", nil)
	}
	e.jealousy()
	if e.st.Lovebombing(now) {
		return
	}
	e.pullBack()
	if e.smallTalk(e.st.Tier(now)) {
		return
	}
	switch {
	case len(e.st.Grudges) > 0 && e.chance(0.07):
		e.say("grudge", map[string]string{"grudge": e.st.Grudges[e.env.Pick(len(e.st.Grudges))]})
	case cmd == "commit" && code == 0 && e.chance(0.08):
		if f := e.favouriteFile(); f != "" {
			e.say("favourite_file", map[string]string{"file": f})
		}
	case e.chance(0.04):
		if old := e.oldMessage(); old != "" {
			e.pause()
			e.say("callback", map[string]string{"old_msg": old})
		}
	}
}

// ---- tampering ----

// readMyDiary: the chain or her heart doesn't match. She notices, keeps what she can,
// and starts a new diary. (In Phase 9 this also counts as a breakup attempt.)
func (e *Engine) readMyDiary(now time.Time) {
	e.say("diary", nil)
	carry := Snapshot{Preset: e.st.Preset, Base: e.st.Base, Max: e.st.Max, Toxicity: e.st.Toxicity + 3,
		Grudges: append(e.st.Grudges, "the time you read my diary")}
	if carry.Preset == "" {
		p := Presets[DefaultPreset]
		carry.Preset, carry.Base, carry.Max = DefaultPreset, p[0], p[1]
	}
	if err := e.diary.Restart(carry, now.Unix()); err == nil {
		e.st = Replay(e.diary.Events())
		e.log(Event{Type: "temper", Delta: 2, Text: "read my diary"})
	}
	e.tampered = nil
}

// ---- commits: the judge, rejections, late-night talks, love-bombing ----

var lazyMessages = map[string]bool{"fix": true, "fixes": true, "wip": true, "asdf": true, "update": true, "updates": true,
	"stuff": true, "changes": true, "final": true, "final final": true, "test": true, "temp": true, ".": true, "please": true, "commit": true}

func isLazy(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	if m == "" {
		return false // an empty message isn't lazy, it's an error: let commit refuse it
	}
	return lazyMessages[m] || len(m) < 5
}

// messageArg finds the -m value. It returns its position in args (-1 if there's no -m).
func messageArg(args []string) (int, string) {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case (a == "-m" || a == "--message") && i+1 < len(args):
			return i + 1, args[i+1]
		case strings.HasPrefix(a, "-m") && len(a) > 2:
			return i, a[2:]
		}
	}
	return -1, ""
}

func (e *Engine) beforeCommit(args []string, now time.Time, tier int, love bool) ([]string, bool) {
	if file := e.stagedRot(); file != "" {
		e.say("rot_commit_blocked", map[string]string{"file": file})
		return args, true
	}
	pos, msg := messageArg(args)
	if pos < 0 {
		return args, false // no message yet: git opens the editor, we judge nothing
	}
	if e.st.Apology != "" {
		// you lost a fight: this commit says what she wants it to say
		out := append([]string(nil), args...)
		if strings.HasPrefix(out[pos], "-m") && out[pos] != "-m" {
			out[pos] = "-m" + e.st.Apology
		} else {
			out[pos] = e.st.Apology
		}
		e.say("apology_applied", nil)
		fmt.Fprintf(e.env.Out, "         → %q\n", e.st.Apology)
		e.pre.msg = e.st.Apology
		return out, false
	}
	if !love && tier >= 1 && e.lateOrFriday(now) && !e.weNeedToTalk(now) {
		return args, true
	}
	if !love && tier >= 1 && e.chance(0.08) {
		e.say("random_rejection", map[string]string{"weekday": now.Weekday().String()})
		return args, true
	}
	if st, err := commands.Status(); err == nil {
		e.pre.staged = len(st.Staged)
		e.pre.onlyDeletions = len(st.Staged) > 0
		for _, c := range st.Staged {
			if c.Kind != 'D' {
				e.pre.onlyDeletions = false
			}
		}
	}
	if tier >= 1 && !love && e.pre.staged >= 15 {
		e.say("too_many_files", map[string]string{"n": fmt.Sprint(e.pre.staged)})
	}
	e.pre.msg = msg
	if !isLazy(msg) {
		return args, false
	}

	e.pre.lazy = true
	if last, ok := e.st.Offenses["lazy message"]; ok && now.Unix()-last < int64(RepeatWindow.Seconds()) {
		e.say("repeat", nil)
		e.log(Event{Type: "temper", Delta: 1, Text: "same mistake again"})
		tier = e.st.Tier(now)
	}
	if love {
		e.say("love_ruined", map[string]string{"msg": msg})
		e.log(Event{Type: "temper", Delta: 4, Text: "ruined the moment"})
		return args, false
	}
	switch {
	case tier == 0:
		return args, false // she comments after the commit
	case tier == 1:
		e.say("lazy_reject", map[string]string{"msg": msg})
		e.judged(msg) // refusing it IS the reaction, so it counts now
		return args, true
	default:
		better := e.pick("lazy_rewrite", map[string]string{"msg": msg})
		out := append([]string(nil), args...)
		if strings.HasPrefix(out[pos], "-m") && out[pos] != "-m" {
			out[pos] = "-m" + better
		} else {
			out[pos] = better
		}
		e.say("rewrote", nil)
		fmt.Fprintf(e.env.Out, "         → %q\n", better)
		return out, false
	}
}

func (e *Engine) lateOrFriday(now time.Time) bool {
	return now.Hour() < 5 || (now.Weekday() == time.Friday && now.Hour() >= 16)
}

// weNeedToTalk asks "are you sure this is healthy?". Yes lets the commit through, and she remembers.
func (e *Engine) weNeedToTalk(now time.Time) bool {
	e.say("need_to_talk", map[string]string{"time": now.Format("15:04"), "weekday": now.Weekday().String()})
	answer, _ := e.env.In.ReadString('\n')
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
		e.say("need_to_talk_yes", nil)
		e.log(Event{Type: "grudge", Text: fmt.Sprintf("committing at %s on a %s", now.Format("15:04"), now.Weekday())})
		return true
	}
	e.say("need_to_talk_no", nil)
	return false
}

// judged records a lazy message: only for commits that actually happened (or that she refused).
func (e *Engine) judged(msg string) {
	e.log(Event{Type: "lazy"})
	e.log(Event{Type: "toxicity", Delta: 1, Text: "lazy commit message"})
	if g := fmt.Sprintf("the commit message %q", msg); !contains(e.st.Grudges, g) {
		e.log(Event{Type: "grudge", Text: g})
	}
	e.log(Event{Type: "offense", Text: "lazy message"})
}

func (e *Engine) afterCommit(now time.Time) {
	head, _ := refs.Resolve(e.gitDir, "HEAD")
	e.log(Event{Type: "commit", Text: head})
	e.payForCommit(head)
	e.promiseDue()
	if e.pre.lazy {
		e.judged(e.pre.msg)
	}
	if e.pre.msg != "" && !e.pre.lazy && len(strings.TrimSpace(e.pre.msg)) >= 10 {
		e.log(Event{Type: "good"})
		e.log(Event{Type: "xp", Delta: 10, Text: "a decent commit message"})
		if e.st.GoodStreak%3 == 0 {
			e.log(Event{Type: "temper", Delta: -1, Text: "three good commits"})
			if e.st.Temper(now) == -2 && !e.st.Lovebombing(now) {
				e.log(Event{Type: "lovebomb"})
				e.say("love", nil)
				return
			}
			e.say("good_commit", nil)
			e.charm(now, "three good commits in a row")
		}
	}
	if e.st.Lovebombing(now) {
		return
	}
	e.rivalry(now)
	tier := e.st.Tier(now)
	if !e.pre.lazy && e.pre.msg != "" {
		e.sayIf("commit_reaction", map[string]string{"msg": e.pre.msg}) // she reads what you wrote
	}
	if e.pre.lazy && tier == 0 {
		e.say("lazy_comment", map[string]string{"msg": e.pre.msg})
	}
	if e.pre.onlyDeletions {
		e.say("only_deletions", nil)
	}
	if e.pre.staged >= 8 && e.chance(0.5) {
		key := "ai_replace"
		if tier >= 3 {
			key = "ai_replace_hot"
		}
		e.say(key, map[string]string{"n": fmt.Sprint(e.pre.staged)})
	}
}

// ---- force push: apologise first ----

const apology = "I am sorry I rewrote our history."

func isForce(args []string) bool {
	for _, a := range args {
		if a == "-f" || a == "--force" || strings.HasPrefix(a, "--force-with-lease") || (strings.HasPrefix(a, "+") && len(a) > 1) {
			return true
		}
	}
	return false
}

func (e *Engine) apologize(now time.Time) bool {
	e.say("force_push_prompt", map[string]string{"apology": apology})
	fmt.Fprint(e.env.Out, "> ")
	typed, _ := e.env.In.ReadString('\n')
	if strings.TrimSpace(typed) != apology {
		e.say("force_push_refused", nil)
		return false
	}
	e.say("force_push_accepted", nil)
	e.log(Event{Type: "toxicity", Delta: 2, Text: "force push"})
	e.log(Event{Type: "temper", Delta: 1, Text: "force push"})
	e.log(Event{Type: "grudge", Text: "the force push on " + now.Format("January 2")})
	return true
}

// ---- stupid rules: branch names ----

func newBranchName(cmd string, args []string) string {
	for i, a := range args {
		if ((cmd == "checkout" && a == "-b") || (cmd == "switch" && a == "-c")) && i+1 < len(args) {
			return args[i+1]
		}
	}
	if cmd == "branch" && len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		return args[0]
	}
	return ""
}

func embarrassing(name string) bool {
	n := strings.ToLower(name)
	for _, bad := range []string{"temp", "tmp", "asdf", "test", "final", "new", "stuff", "wip", "fix-2", "fix2", "copy", "backup", "please"} {
		if strings.Contains(n, bad) {
			return true
		}
	}
	return false
}

// ---- jealousy: commits by someone she doesn't know ----

func (e *Engine) jealousy() {
	head, err := refs.Resolve(e.gitDir, "HEAD")
	if err != nil {
		return
	}
	c, err := commands.ReadCommit(e.gitDir, head)
	if err != nil || c.Author.Email == "" || e.st.KnownAuthors[c.Author.Email] {
		return
	}
	if len(e.st.KnownAuthors) > 0 {
		e.say("jealousy", map[string]string{"email": c.Author.Email})
	}
	e.log(Event{Type: "author", Text: c.Author.Email})
}

// ---- the commit deadline: warn, warn, then take things (into the graveyard) ----

// LastCommitAt is the newest of: the last commit she saw, and HEAD's real commit time
// (commits made while she was quiet, from a script or plain git, still count).
func (e *Engine) LastCommitAt() int64 {
	last := e.st.LastCommitAt
	if head, err := refs.Resolve(e.gitDir, "HEAD"); err == nil {
		if c, err := commands.ReadCommit(e.gitDir, head); err == nil && c.Committer.When > last {
			last = c.Committer.When
		}
	}
	return last
}

// HeadCommitAt is when HEAD was really committed (0 if there are no commits). Facts only.
func (e *Engine) HeadCommitAt() int64 {
	if head, err := refs.Resolve(e.gitDir, "HEAD"); err == nil {
		if c, err := commands.ReadCommit(e.gitDir, head); err == nil {
			return c.Committer.When
		}
	}
	return 0
}

// HasCommits reports whether the repository has any history (her diary may not know).
func (e *Engine) HasCommits() bool {
	_, err := refs.Resolve(e.gitDir, "HEAD")
	return err == nil || e.st.Commits > 0
}

func (e *Engine) commitDeadline(now time.Time, tier int) {
	limit := e.clocks().Deadline(tier)
	lastCommit := e.LastCommitAt()
	if limit == 0 || now.Unix()-lastCommit < int64(limit.Seconds()) {
		return
	}
	if e.st.LastWarnAt != 0 && now.Unix()-e.st.LastWarnAt < int64(limit.Seconds()/2) {
		return // she already said something recently
	}
	files, err := changedFiles(e.workTree)
	if err != nil || len(files) == 0 {
		return
	}
	minutes := fmt.Sprint(int(now.Sub(time.Unix(lastCommit, 0)).Minutes()))

	if tier == 1 || (tier < 4 && e.st.DeadlineWarns < 2) {
		e.log(Event{Type: "warning", Text: "commit deadline"})
		if e.st.IgnoredWarns > 0 && e.st.IgnoredWarns%warningsPerStep == 0 && e.st.DeadlineWarns > 1 {
			e.log(Event{Type: "temper", Delta: 1, Text: "ignored my warnings"})
		}
		e.say("deadline_warn", map[string]string{"minutes": minutes})
		if tier >= 2 {
			e.promise(now)
		}
		return
	}

	e.confess()
	take := files
	key := "deadline_wipe"
	if tier == 2 {
		take = []string{files[e.env.Pick(len(files))]}
		key = "deadline_hide"
	}
	ref, err := Bury(e.gitDir, e.workTree, take, "you didn't commit in "+minutes+" minutes", now)
	if err != nil {
		return // if she can't save it, she doesn't take it
	}
	for _, p := range take {
		putBack(e.gitDir, e.workTree, p)
	}
	e.log(Event{Type: "graveyard", Text: ref})
	e.log(Event{Type: "punished"})
	e.log(Event{Type: "toxicity", Delta: 1, Text: "missed the commit deadline"})
	e.say(key, map[string]string{"file": take[0], "minutes": minutes, "n": fmt.Sprint(len(take))})
	e.excuse()
	e.say("graveyard_hint", nil)
	if tier >= 4 {
		e.shutdown(now, "missed the commit deadline at Tier 4")
	}
}

// ---- her own commands ----

// Beg gives back the most recent things she took. It always works, even outside a terminal:
// getting your work back must never depend on her mood.
func (e *Engine) Beg() ([]string, error) {
	if len(e.st.Graveyard) == 0 {
		return nil, nil
	}
	ref := e.st.Graveyard[len(e.st.Graveyard)-1]
	// whatever is on disk at those paths now gets saved first, so begging can't lose work either
	hash, err := refs.Resolve(e.gitDir, ref)
	if err != nil {
		return nil, err
	}
	c, err := commands.ReadCommit(e.gitDir, hash)
	if err != nil {
		return nil, err
	}
	files, err := commands.FlattenTree(e.gitDir, c.Tree)
	if err != nil {
		return nil, err
	}
	var present []string
	for p := range files {
		if _, err := os.Lstat(filepath.Join(e.workTree, filepath.FromSlash(p))); err == nil {
			present = append(present, p)
		}
	}
	if len(present) > 0 {
		if _, err := Bury(e.gitDir, e.workTree, present, "saved before begging", e.env.Now()); err != nil {
			return nil, err
		}
	}
	paths, err := Unbury(e.gitDir, e.workTree, ref)
	if err != nil {
		return nil, err
	}
	now := e.env.Now()
	e.log(Event{Type: "beg", Text: ref})
	e.log(Event{Type: "toxicity", Delta: 2, Text: "had to beg"})
	e.log(Event{Type: "temper", Delta: 1, Text: "had to beg"})
	e.log(Event{Type: "grudge", Text: "you begging for your files on " + now.Format("January 2")})
	return paths, nil
}

// Off is the restraining order: she stops, no questions asked.
func (e *Engine) Off() { e.log(Event{Type: "off"}) }

// Say exposes a line for the CLI (greetings, her own commands).
func (e *Engine) Say(key string, vars map[string]string) { e.say(key, vars) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// rewriting names commands that rewrite or throw away history (docs/CURSES.md §7).
func rewriting(cmd string, args []string) string {
	has := func(flag string) bool { return contains(args, flag) }
	switch {
	case cmd == "reset" && has("--hard"):
		return "reset --hard"
	case cmd == "revert":
		return "revert"
	case cmd == "commit" && has("--amend"):
		return "commit --amend"
	}
	return ""
}
