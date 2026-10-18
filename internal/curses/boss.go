// internal/curses/boss.go
//
// Boss fights (docs/CURSES.md §4). Merging is git's job: `cg merge`, `cg pull`, `cg rebase`
// and `cg cherry-pick` all go to real git. When git stops because both sides changed the same
// lines, it leaves the conflict in two places, and that's all she needs:
//
//	the index      stages 1/2/3 (base, ours, theirs) for every conflicted file
//	the files      <<<<<<< / ======= / >>>>>>> blocks around the two versions
//
// At Tier 3+ a conflict becomes a fight. The boss's HP is the number of conflict blocks, the
// clock is a few minutes per block, and every block you resolve is a hit. You win by
// committing before the clock runs out. You can't run (`--abort` is refused).
//
// Below Tier 3 a conflict is just a conflict, and she only explains what happened.
package curses

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/config"
	"github.com/spiritpaper/cursed-git/internal/index"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

// Fight is one boss fight, as written in her diary when it starts.
type Fight struct {
	Boss    string   `json:"boss"`
	Branch  string   `json:"branch"` // what was being merged in
	HP      int      `json:"hp"`     // conflict blocks at the start
	Files   []string `json:"files"`  // conflicted files at the start
	Head    string   `json:"head"`   // HEAD at the start: the fight is won by a new commit
	Seconds int      `json:"seconds"`
	Phase   int      `json:"phase,omitempty"`  // 2 = the rematch at Tier 4 (no phase 3)
	Remote  bool     `json:"remote,omitempty"` // he came from a remote (a pull)
	Start   int64    `json:"-"`                // the event's time
}

func (f *Fight) deadline() int64 { return f.Start + int64(f.Seconds) }

// HunkTime is the clock per conflict block, by live tier (0 = no fights at that tier).
func HunkTime(tier int) time.Duration {
	return [...]time.Duration{0, 0, 0, 3 * time.Minute, 90 * time.Second}[clamp(tier, 0, 4)]
}

// git commands that can leave conflicts behind
var conflictMakers = map[string]bool{"merge": true, "pull": true, "rebase": true, "cherry-pick": true,
	"revert": true, "stash": true, "checkout": true, "switch": true, "am": true}

// unmerged lists files with conflict stages in the index.
func (e *Engine) unmerged() map[string]bool {
	out := map[string]bool{}
	idx, err := index.Read(e.gitDir)
	if err != nil {
		return out
	}
	for _, en := range idx.Entries {
		if en.Stage() != 0 {
			out[en.Path] = true
		}
	}
	return out
}

func (e *Engine) exists(name string) bool {
	_, err := os.Stat(filepath.Join(e.gitDir, name))
	return err == nil
}

// inProgress: git is in the middle of something that a commit (or --continue) finishes.
func (e *Engine) inProgress() bool {
	if len(e.unmerged()) > 0 {
		return true
	}
	for _, f := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		if e.exists(f) {
			return true
		}
	}
	return false
}

var markerStart = []byte("<<<<<<< ")

// blocks counts conflict blocks in a file: lines starting with "<<<<<<< ".
func (e *Engine) blocks(path string) int {
	data, err := os.ReadFile(filepath.Join(e.workTree, filepath.FromSlash(path)))
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(line, markerStart) {
			n++
		}
	}
	return n
}

// hpLeft: every block still in a file is 1 HP. A file that's still conflicted in the index
// but has no blocks (one side deleted it) is 1 HP until you `cg add` or `cg rm` it.
func (e *Engine) hpLeft(f *Fight) int {
	staged := e.unmerged()
	paths := map[string]bool{}
	for _, p := range f.Files {
		paths[p] = true
	}
	for p := range staged {
		paths[p] = true
	}
	left := 0
	for p := range paths {
		n := e.blocks(p)
		if staged[p] {
			n = max(n, 1)
		}
		left += n
	}
	return left
}

var quoted = regexp.MustCompile(`'([^']+)'`)

// opponent names the boss: her rival (Chad, for Miranda) if it came from a remote, Past You if it's your own branch,
// otherwise whoever wrote the commit you're merging.
func (e *Engine) opponent() (boss, branch string) {
	branch = "their side"
	msg, _ := os.ReadFile(filepath.Join(e.gitDir, "MERGE_MSG"))
	first, _, _ := strings.Cut(string(msg), "\n")
	if m := quoted.FindStringSubmatch(first); m != nil {
		branch = m[1]
	}
	if e.exists("rebase-merge") || e.exists("rebase-apply") {
		branch = "your rebase"
	}
	if strings.Contains(first, "remote-tracking") {
		remote, _, _ := strings.Cut(branch, "/")
		return e.rival() + " from " + remote, branch
	}
	if _, url, ok := strings.Cut(first, "' of "); ok {
		// a pull: git names the URL, so find which remote that is
		remote := "upstream"
		for key, val := range config.Load(e.gitDir) {
			if name, ok := strings.CutPrefix(key, "remote."); ok && strings.HasSuffix(key, ".url") && val == strings.TrimSpace(url) {
				remote = strings.TrimSuffix(name, ".url")
			}
		}
		return e.rival() + " from " + remote, remote + "/" + branch
	}
	for _, f := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "REBASE_HEAD"} {
		data, err := os.ReadFile(filepath.Join(e.gitDir, f))
		if err != nil {
			continue
		}
		hash, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
		c, err := commands.ReadCommit(e.gitDir, hash)
		if err != nil {
			continue
		}
		if me, _, err := commands.Identity(e.gitDir); err == nil && c.Author.Email == me.Email {
			return "Past You", branch
		}
		if name := strings.Fields(c.Author.Name); len(name) > 0 {
			return name[0], branch
		}
	}
	return e.rival(), branch
}

func clock(secs int64) string {
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

// startFight turns the conflict git just left into a boss fight.
func (e *Engine) startFight(now time.Time, tier int) {
	staged := e.unmerged()
	f := &Fight{}
	for p := range staged {
		f.Files = append(f.Files, p)
	}
	sort.Strings(f.Files)
	f.Head, _ = refs.Resolve(e.gitDir, "HEAD")
	f.Boss, f.Branch = e.opponent()
	f.Remote = strings.HasPrefix(f.Boss, e.rival()+" from ")
	f.Start = now.Unix()
	f.HP = e.hpLeft(f)
	f.Seconds = f.HP * int(e.clocks().Fight(tier).Seconds())
	if e.phase2 {
		f.Phase = 2
	}
	e.log(Event{Type: "fight", Text: f.Boss, Fight: f})

	e.pause()
	e.say("fight_intro", map[string]string{"boss": f.Boss, "branch": f.Branch})
	out := e.env.Out
	fmt.Fprintf(out, "\n⚔  BOSS FIGHT: %s  ·  %s  ·  HP %d  ·  ⏳ %s\n", f.Boss, f.Branch, f.HP, clock(int64(f.Seconds)))
	fmt.Fprintf(out, "   git merged what it could. In %s it found lines that BOTH sides changed,\n", plural(len(f.Files), "file"))
	fmt.Fprintf(out, "   so it wrote both versions in and left the choice to you:\n\n")
	fmt.Fprintf(out, "       <<<<<<< HEAD        your version\n       =======\n       >>>>>>> %-11s theirs\n\n", f.Branch)
	fmt.Fprintf(out, "   Every block is 1 HP. Keep the right lines, delete the three marker lines, then\n")
	fmt.Fprintf(out, "   `cg add <file>`. `cg commit` finishes him. `cg status` shows his health.\n")
	for _, p := range f.Files {
		if e.blocks(p) == 0 {
			fmt.Fprintf(out, "   %s was deleted on one side: `cg add` it to keep it, `cg rm` it to let it go.\n", p)
		}
	}
	fmt.Fprintf(out, "   (Curious? `cg ls-files -s` shows git's three copies: 1 = base, 2 = yours, 3 = theirs.)\n\n")
}

// FightBar is the health bar: "⚔  Chad from origin  HP ██████░░░░ 3/5  ⏳ 7:12". "" if no fight.
func (e *Engine) FightBar(now time.Time) string {
	f := e.st.Fight
	if f == nil {
		return ""
	}
	left := e.hpLeft(f)
	total := max(f.HP, left, 1)
	full := (left*10 + total - 1) / total
	return fmt.Sprintf("⚔  %s  HP %s%s %d/%d  ⏳ %s", f.Boss, strings.Repeat("█", full), strings.Repeat("░", 10-full),
		left, total, clock(f.deadline()-now.Unix()))
}

// running away: anything that throws the conflict away instead of resolving it
func running(cmd string, args []string) bool {
	switch cmd {
	case "merge", "rebase", "cherry-pick", "revert", "am":
		return contains(args, "--abort") || contains(args, "--quit") || contains(args, "--skip")
	case "reset":
		return contains(args, "--hard") || contains(args, "--merge") || contains(args, "--keep")
	}
	return false
}

// markedFile finds a file you're about to `cg add` that still has conflict blocks in it.
func (e *Engine) markedFile(args []string) string {
	for _, p := range e.st.Fight.Files {
		if e.blocks(p) == 0 {
			continue
		}
		for _, a := range args {
			rel := filepath.ToSlash(filepath.Clean(a))
			if a == "-A" || a == "--all" || a == "-u" || rel == "." || rel == p || strings.HasPrefix(p, rel+"/") {
				return p
			}
		}
	}
	return ""
}

// fightBefore runs before every command while a fight is on. It settles fights that ended
// (or ran out of time) while she wasn't looking, and refuses running away.
func (e *Engine) fightBefore(cmd string, args []string, now time.Time, exempt bool) bool {
	f := e.st.Fight
	if f == nil {
		return false
	}
	if !e.inProgress() {
		e.fightOver(now)
		return false
	}
	if now.Unix() > f.deadline() {
		e.loseFight(now, "time")
		return false
	}
	if exempt {
		return false
	}
	if running(cmd, args) {
		e.say("fight_run", map[string]string{"boss": f.Boss})
		return true
	}
	if cmd == "add" {
		if p := e.markedFile(args); p != "" {
			e.say("fight_markers", map[string]string{"file": p, "n": fmt.Sprint(e.blocks(p))})
			return true
		}
		e.pre.hp = e.hpLeft(f)
	}
	if e.chance(0.25) {
		e.say("fight_taunt", map[string]string{"boss": f.Boss})
	}
	return false
}

// fightAfter: start a fight if git just left a conflict, or show how the current one is going.
func (e *Engine) fightAfter(cmd string, now time.Time) {
	f := e.st.Fight
	if f == nil {
		if len(e.unmerged()) == 0 {
			return
		}
		if last := e.st.LastFight; last != nil {
			if head, _ := refs.Resolve(e.gitDir, "HEAD"); head == last.Head {
				return // the fight over this conflict already ended: now it's just a conflict
			}
		}
		if tier := e.st.Tier(now); HunkTime(tier) > 0 {
			e.startFight(now, tier)
		} else if conflictMakers[cmd] {
			e.say("conflict_small", nil) // below Tier 3 a conflict is just a conflict
		}
		return
	}
	if !e.inProgress() {
		e.fightOver(now)
		return
	}
	if cmd == "add" && e.hpLeft(f) < e.pre.hp {
		e.say("fight_hit", map[string]string{"boss": f.Boss, "n": fmt.Sprint(e.pre.hp - e.hpLeft(f))})
	}
	switch cmd {
	case "status", "add", "rm", "diff", "ls-files":
		fmt.Fprintln(e.env.Out, e.FightBar(now))
	}
}

// fightOver: git isn't in the middle of anything any more. A new commit means you won;
// the same HEAD means the conflict was thrown away (with plain git: she couldn't stop it).
func (e *Engine) fightOver(now time.Time) {
	f := e.st.Fight
	head, _ := refs.Resolve(e.gitDir, "HEAD")
	if head == f.Head {
		e.loseFight(now, "ran")
		return
	}
	left := f.deadline() - now.Unix()
	tier := e.st.Tier(now) // the tier she fought at (winning calms her down)
	e.log(Event{Type: "fight_won", Text: f.Boss, Delta: int(left)})
	e.log(Event{Type: "xp", Delta: 250 + int(max(0, left))/10, Text: "beat " + f.Boss})
	e.log(Event{Type: "temper", Delta: -1, Text: "beat " + f.Boss})
	key := "fight_won"
	if left*2 > int64(f.Seconds) {
		e.log(Event{Type: "toxicity", Delta: -1, Text: "won a fight fast"})
		key = "fight_won_fast"
	}
	out := e.env.Out
	fmt.Fprintf(out, "\n   ╔═══════════════════════════════════════╗\n")
	fmt.Fprintf(out, "   ║  K.O.   %-30s║\n", f.Boss+" is down.")
	fmt.Fprintf(out, "   ║  %-37s║\n", fmt.Sprintf("%d HP in %s, %s left on the clock", f.HP, clock(now.Unix()-f.Start), clock(left)))
	fmt.Fprintf(out, "   ╚═══════════════════════════════════════╝\n")
	fmt.Fprintf(out, "   (Your merge commit has two parents now: `cg log --oneline -n 3`.)\n\n")
	e.say(key, map[string]string{"boss": f.Boss})
	e.charm(now, "you beat "+f.Boss)
	if tier >= 4 && f.Phase < 2 {
		// Catastrophic: you won, so she brings him back. Phase 2.
		e.pause()
		e.say("fight_phase2", map[string]string{"boss": f.Boss})
		e.phase2 = true
		e.pull(tier)
		e.phase2 = false
	}
}

// loseFight: ran out of time, or ran away. Tier 3: she dictates your next commit message.
// Tier 4: she takes your resolution work (into the graveyard) and aborts the merge herself.
func (e *Engine) loseFight(now time.Time, why string) {
	f := e.st.Fight
	tier := e.st.Tier(now)
	e.log(Event{Type: "fight_lost", Text: f.Boss + ": " + why})
	e.log(Event{Type: "toxicity", Delta: 3, Text: "lost to " + f.Boss})
	if g := "the day you lost to " + f.Boss; !contains(e.st.Grudges, g) {
		e.log(Event{Type: "grudge", Text: g})
	}
	if why == "ran" {
		e.say("fight_ran", map[string]string{"boss": f.Boss})
		return
	}
	if tier >= 4 && e.abort(f, now) {
		e.say("fight_lost_abort", map[string]string{"boss": f.Boss})
		e.say("graveyard_hint", nil)
		e.shutdown(now, "lost to "+f.Boss+" at Tier 4")
		return
	}
	apology := e.pick("fight_apology", map[string]string{"boss": f.Boss})
	e.log(Event{Type: "apology", Text: apology})
	e.say("fight_lost", map[string]string{"boss": f.Boss})
	fmt.Fprintf(e.env.Out, "         → your next commit message: %q\n", apology)
	e.excuse()
}

// abort saves whatever you did to the conflicted files, then lets git abort the merge.
func (e *Engine) abort(f *Fight, now time.Time) bool {
	var present []string
	for _, p := range f.Files {
		if _, err := os.Lstat(filepath.Join(e.workTree, filepath.FromSlash(p))); err == nil {
			present = append(present, p)
		}
	}
	if len(present) > 0 {
		ref, err := Bury(e.gitDir, e.workTree, present, "lost the fight against "+f.Boss, now)
		if err != nil {
			return false // if she can't save it, she doesn't take it
		}
		e.log(Event{Type: "graveyard", Text: ref})
	}
	what := "merge"
	switch {
	case e.exists("rebase-merge") || e.exists("rebase-apply"):
		what = "rebase"
	case e.exists("CHERRY_PICK_HEAD"):
		what = "cherry-pick"
	case e.exists("REVERT_HEAD"):
		what = "revert"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", e.workTree, what, "--abort")
	cmd.Env = append(os.Environ(), "CG_VIA=1")
	return cmd.Run() == nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
