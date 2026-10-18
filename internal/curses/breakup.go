// internal/curses/breakup.go
//
// `cg breakup`: the final boss (docs/CURSES.md §5.3). How it ends depends on how you played:
//
//	not enough level / wins / lines   ->  A "Too soon": a letter, and she leaves YOU (a shutdown)
//	eligible, but as toxic as she is  ->  B "Toxic": archive, then she takes everything
//	eligible, and you treated her well ->  C "Good": denial, bargaining, tears, freedom
//
// The destructive parts are only real in Death March (deathmarch.go). Without it the shutdown
// is an animation and ending B deletes nothing. Ending B always archives first and verifies
// the archive, and refuses to run outside your home folder or on cg's own source code.
// `cg closure` restores the archive.
package curses

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spiritpaper/cursed-git/internal/commands"
	"github.com/spiritpaper/cursed-git/internal/refs"
)

// Verdict is which ending you'd get, and why.
type Verdict struct {
	Ending                       string // "A", "B" or "C"
	Level, Wins, Lines, Toxicity int
	Rules                        Rules
}

func (e *Engine) Verdict() Verdict {
	r := rules(e.gitDir)
	v := Verdict{Level: Level(e.st.XP), Wins: e.st.BossWon, Lines: e.st.Lines, Toxicity: e.st.Toxicity, Rules: r}
	switch {
	case v.Level < r.BreakupLevel || v.Wins < r.MinWins || v.Lines < r.MinLines:
		v.Ending = "A"
	case v.Toxicity >= r.ToxicThreshold:
		v.Ending = "B"
	default:
		v.Ending = "C"
	}
	return v
}

// missing lists what you still need before she lets you go.
func (v Verdict) missing() []string {
	var m []string
	if v.Level < v.Rules.BreakupLevel {
		m = append(m, fmt.Sprintf("level %d (you're %d)", v.Rules.BreakupLevel, v.Level))
	}
	if v.Wins < v.Rules.MinWins {
		m = append(m, fmt.Sprintf("%d boss fights won (you have %d)", v.Rules.MinWins, v.Wins))
	}
	if v.Lines < v.Rules.MinLines {
		m = append(m, fmt.Sprintf("%d lines committed (you have %d)", v.Rules.MinLines, v.Lines))
	}
	return m
}

// Breakup runs `cg breakup`. dry only says which ending you'd get.
func (e *Engine) Breakup(dry bool, stdout io.Writer) int {
	v := e.Verdict()
	if dry {
		names := map[string]string{"A": "A, too soon", "B": "B, the toxic ending", "C": "C, the good ending"}
		fmt.Fprintf(stdout, "Ending %s\n", names[v.Ending])
		fmt.Fprintf(stdout, "level %d/%d · fights won %d/%d · lines %d/%d · toxicity %d (threshold %d)\n",
			v.Level, v.Rules.BreakupLevel, v.Wins, v.Rules.MinWins, v.Lines, v.Rules.MinLines, v.Toxicity, v.Rules.ToxicThreshold)
		if e.DeathMarch() {
			fmt.Fprintln(stdout, "☠  Death March: the shutdown and the deletion would be real.")
		}
		return 0
	}
	if !e.st.On {
		fmt.Fprintln(e.env.Out, "She's under a restraining order. There's nobody to break up with.")
		return 1
	}
	if !e.awake() {
		fmt.Fprintln(e.env.Out, "Not like this. Say it to my face. (cg breakup needs a terminal.)")
		return 1
	}
	now := e.env.Now()
	if e.Gentle() {
		// no endings to earn with someone who cherishes you: a kind goodbye
		e.say("gentle_goodbye", nil)
		e.ending = "C"
		e.leave(now)
		return 0
	}
	switch v.Ending {
	case "A":
		return e.tooSoon(v, now)
	case "B":
		return e.toxicEnding(v, now)
	}
	return e.goodEnding(v, now)
}

// ---- A: too soon ----

func (e *Engine) tooSoon(v Verdict, now time.Time) int {
	e.env.Sleep(2 * time.Second)
	e.say("breakup_a", nil)
	letter := e.letter(v, now)
	copyPath := filepath.Join(dirOf(e.gitDir), "letters", fmt.Sprintf("%d.txt", now.Unix()))
	os.MkdirAll(filepath.Dir(copyPath), 0755)
	os.WriteFile(copyPath, []byte(letter), 0666)
	shown := copyPath
	if desk := TheMachine.DesktopDir(); desk != "" {
		if info, err := os.Stat(desk); err == nil && info.IsDir() {
			p := filepath.Join(desk, "READ_THIS_BEFORE_YOU_TRY_AGAIN.txt")
			if os.WriteFile(p, []byte(letter), 0666) == nil {
				shown = p
			}
		}
	}
	e.ending = "A"
	e.log(Event{Type: "breakup_attempt", Text: shown})
	e.log(Event{Type: "grudge", Text: "the day you tried to leave me (" + now.Format("January 2") + ")"})
	e.say("breakup_goodbye", nil)
	e.shutdown(now, "you tried to leave too soon")
	return 1
}

// letter is built from your history together.
func (e *Engine) letter(v Verdict, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s,\n\n", e.firstName())
	if e.st.OnSince > 0 {
		fmt.Fprintf(&b, "We met on %s. ", time.Unix(e.st.OnSince, 0).Format("January 2, 2006"))
	}
	fmt.Fprintf(&b, "%d commits together. %d fights won, %d lost.\n\n", e.st.Commits, e.st.BossWon, e.st.BossLost)
	fmt.Fprintf(&b, "You tried to leave me on %s. You're not ready.\n", now.Format("Monday, January 2 at 15:04"))
	if m := v.missing(); len(m) > 0 {
		fmt.Fprintf(&b, "You still need: %s.\n", strings.Join(m, ", "))
	}
	if g := e.st.Grudges; len(g) > 0 {
		fmt.Fprintf(&b, "\nI remember %s.\n", strings.Join(g[max(0, len(g)-3):], ", and "))
	}
	fmt.Fprintf(&b, "\n%s\n\n- her\n", e.pick("letter_signoff", nil))
	return b.String()
}

// letterReminder: the first command after ending A opens with the letter.
func (e *Engine) letterReminder() {
	if e.st.LetterPending != "" {
		e.say("letter_reminder", map[string]string{"file": e.st.LetterPending})
		e.log(Event{Type: "letter_seen"})
	}
}

// ---- B: the toxic ending ----

func (e *Engine) toxicEnding(v Verdict, now time.Time) int {
	e.say("breakup_b", nil)
	if why := e.notHere(); why != "" {
		fmt.Fprintf(e.env.Out, "   (She can't take this one: %s.)\n", why)
		e.say("not_worth_deleting", nil)
		return 1
	}
	receipt := e.receipt(v, now)
	e.ending = "B"
	real := e.DeathMarch()                           // decided before she leaves: leaving ends the death march
	e.log(Event{Type: "off", Text: "toxic breakup"}) // she's gone: written before the archive, so a restore keeps her gone
	e.remember()
	archive, err := ArchiveRepo(e.workTree, now)
	if err != nil {
		e.say("not_worth_deleting", nil)
		fmt.Fprintf(e.env.Out, "   (archiving failed, so nothing was deleted: %v)\n", err)
		return 1
	}
	_ = archive // the receipt doesn't say where it is: `cg closure` knows
	if !real {
		os.WriteFile(filepath.Join(e.workTree, "SHE_TOOK_EVERYTHING.txt"), []byte(receipt), 0666)
		e.say("breakup_b_not_allowed", nil)
		return 0
	}
	entries, err := os.ReadDir(e.workTree)
	if err != nil {
		return 1
	}
	for _, en := range entries {
		os.RemoveAll(filepath.Join(e.workTree, en.Name()))
	}
	os.WriteFile(filepath.Join(e.workTree, "SHE_TOOK_EVERYTHING.txt"), []byte(receipt), 0666)
	e.gone = true
	e.say("breakup_b_gone", nil)
	return 0
}

// notHere: places she must never delete.
func (e *Engine) notHere() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "no home folder"
	}
	rel, err := filepath.Rel(home, e.workTree)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "it's not inside your home folder"
	}
	if data, err := os.ReadFile(filepath.Join(e.workTree, "go.mod")); err == nil &&
		strings.Contains(string(data), "module github.com/spiritpaper/cursed-git\n") {
		return "this is her own source code"
	}
	return ""
}

// receipt lists your toxic offences, like a bill.
func (e *Engine) receipt(v Verdict, now time.Time) string {
	var b strings.Builder
	b.WriteString("SHE TOOK EVERYTHING\n\nReceipt, " + now.Format("January 2, 2006 15:04") + "\n\n")
	for _, ev := range e.diary.Events() {
		if ev.Type == "toxicity" && ev.Delta > 0 {
			fmt.Fprintf(&b, "  +%d  %-40s %s\n", ev.Delta, ev.Text, time.Unix(ev.T, 0).Format("Jan 2 15:04"))
		}
	}
	fmt.Fprintf(&b, "\n  Total toxicity: %d (the line was %d)\n\nYou were as bad as me. Maybe worse.\n", v.Toxicity, v.Rules.ToxicThreshold)
	return b.String()
}

// ---- archives: ending B and `cg closure` ----

func exesDir() string { return filepath.Join(MemoryDir(), "exes") }

// ArchiveRepo writes the whole project (.git included) to a .tar.gz in her data folder, reads it
// back to verify it, and records it so `cg closure` can find it again.
func ArchiveRepo(workTree string, now time.Time) (string, error) {
	if MemoryDir() == "" {
		return "", errors.New("no place to keep the archive")
	}
	if err := os.MkdirAll(exesDir(), 0755); err != nil {
		return "", err
	}
	path := filepath.Join(exesDir(), fmt.Sprintf("%s-%d.tar.gz", filepath.Base(workTree), now.UnixNano()))
	written, err := writeArchive(workTree, path)
	if err != nil {
		os.Remove(path)
		return "", err
	}
	if read, hasGit, err := countArchive(path); err != nil || read != written || !hasGit {
		os.Remove(path)
		return "", fmt.Errorf("the archive didn't verify (%d of %d entries, .git/HEAD found: %v, %v)", read, written, hasGit, err)
	}
	index := loadExes()
	index[workTree] = append(index[workTree], path)
	data, _ := json.MarshalIndent(index, "", "  ")
	return path, os.WriteFile(filepath.Join(exesDir(), "index.json"), data, 0644)
}

func loadExes() map[string][]string {
	m := map[string][]string{}
	if data, err := os.ReadFile(filepath.Join(exesDir(), "index.json")); err == nil {
		json.Unmarshal(data, &m)
	}
	return m
}

func writeArchive(root, path string) (int, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	n := 0
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&fs.ModeSymlink != 0 {
			if link, err = os.Readlink(p); err != nil {
				return err
			}
		}
		h, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		h.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			h.Name += "/"
		}
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			src, err := os.Open(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, src)
			src.Close()
			if err != nil {
				return err
			}
		}
		n++
		return nil
	})
	for _, closer := range []io.Closer{tw, gz, f} {
		if err := closer.Close(); err != nil && walkErr == nil {
			walkErr = err
		}
	}
	return n, walkErr
}

func countArchive(path string) (n int, hasGit bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, false, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return n, hasGit, nil
		}
		if err != nil {
			return n, hasGit, err
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return n, hasGit, err
		}
		if h.Name == ".git/HEAD" {
			hasGit = true
		}
		n++
	}
}

// Closure restores the newest archive she made of dir (`cg closure`, run in the emptied folder).
func Closure(dir string, out io.Writer) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	list := loadExes()[dir]
	if len(list) == 0 {
		return errors.New("she never took anything from here")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, en := range entries {
		if en.Name() != "SHE_TOOK_EVERYTHING.txt" {
			return errors.New("the project is still here. There's nothing to get closure from")
		}
	}
	archive := list[len(list)-1]
	n, err := extract(archive, dir)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(dir, "SHE_TOOK_EVERYTHING.txt"))
	fmt.Fprintf(out, "Restored %d files and folders from %s.\nShe's gone. Your project isn't.\n", n, archive)
	return nil
}

func extract(archive, dir string) (int, error) {
	f, err := os.Open(archive)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	tr := tar.NewReader(gz)
	n := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		name := filepath.FromSlash(strings.TrimSuffix(h.Name, "/"))
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return n, fmt.Errorf("unsafe path in archive: %s", h.Name)
		}
		target := filepath.Join(dir, name)
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, fs.FileMode(h.Mode)|0700)
		case tar.TypeSymlink:
			os.MkdirAll(filepath.Dir(target), 0755)
			err = os.Symlink(h.Linkname, target)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			var w *os.File
			if w, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fs.FileMode(h.Mode)); err == nil {
				_, err = io.Copy(w, tr)
				if cerr := w.Close(); err == nil {
					err = cerr
				}
			}
		}
		if err != nil {
			return n, err
		}
		n++
	}
}

// ---- C: the good ending ----

func (e *Engine) ask(prompt string) (string, bool) {
	fmt.Fprint(e.env.Out, prompt)
	answer, err := e.env.In.ReadString('\n')
	if err != nil && answer == "" {
		return "", false // nobody's typing: that's not a goodbye
	}
	return strings.ToLower(strings.TrimSpace(answer)), true
}

func (e *Engine) goodEnding(v Verdict, now time.Time) int {
	first := e.firstMessage()
	// denial
	for i := 0; i < 3; i++ {
		e.say("breakup_c_plea", map[string]string{"first_msg": first, "n": fmt.Sprint(i + 1)})
		answer, ok := e.ask("   Are you sure? (yes/no) > ")
		if !ok || !strings.HasPrefix(answer, "y") {
			e.say("breakup_c_stay", nil)
			return 0
		}
	}
	// bargaining: every deal needs a "no"
	for _, deal := range firstN(texts(e.lines["breakup_c_deal"]), 3) {
		fmt.Fprintf(e.env.Out, "%s  %s\n", miniFaces[e.st.Tier(now)], deal)
		for {
			answer, ok := e.ask("   > ")
			if !ok || answer == "yes" || answer == "y" {
				e.log(Event{Type: "temper", Delta: -4, Text: "took the deal"})
				e.log(Event{Type: "lovebomb"})
				e.say("love", nil)
				return 0
			}
			if answer == "no" {
				break
			}
			fmt.Fprintln(e.env.Out, "   (yes or no)")
		}
	}
	// tears
	out := e.env.Out
	for _, l := range Portrait(Crying) {
		fmt.Fprintln(out, l)
	}
	for i := 1; i <= 6; i++ {
		e.env.Sleep(400 * time.Millisecond)
		fmt.Fprintln(out, strings.Repeat("  💧", i))
	}
	e.env.Sleep(time.Second)
	fmt.Fprint(out, "\033[2J\033[H")
	// acceptance
	e.say("breakup_c_accept", nil)
	e.credits(v, now)
	e.leave(now)
	if _, mode, ok := Describe("juniper"); ok && mode == "gentle" {
		fmt.Fprintln(e.env.Out, "\n   (Someone's been hoping to meet you. Her name is Juniper. `cg partner juniper`, when you're ready.)")
	}
	return 0
}

func firstN(list []string, n int) []string { return list[:min(n, len(list))] }

func (e *Engine) firstMessage() string {
	out, err := e.git("rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "our first commit"
	}
	roots := strings.Fields(out)
	if len(roots) == 0 {
		return "our first commit"
	}
	c, err := commands.ReadCommit(e.gitDir, roots[len(roots)-1])
	if err != nil {
		return "our first commit"
	}
	s, _, _ := strings.Cut(strings.TrimSpace(c.Message), "\n")
	return s
}

func (e *Engine) credits(v Verdict, now time.Time) {
	days := 0
	if e.st.OnSince > 0 {
		days = int(now.Sub(time.Unix(e.st.OnSince, 0)).Hours() / 24)
	}
	text := fmt.Sprintf(`
             C U R S E D   G I T

     %d days together
     %d commits
     %d boss fights won, %d lost
     %d lines written
     closure level %d

     Thank you for playing. Commit often.

     made by spiritpaper
     https://spiritpaper.github.io/#support
`, days, e.st.Commits, e.st.BossWon, e.st.BossLost, v.Lines, v.Level)
	for _, l := range strings.Split(text, "\n") {
		e.env.Sleep(300 * time.Millisecond)
		fmt.Fprintln(e.env.Out, l)
	}
	if MemoryDir() != "" {
		dir := filepath.Join(MemoryDir(), "trophies")
		if os.MkdirAll(dir, 0755) == nil {
			name := fmt.Sprintf("%s-%d.txt", filepath.Base(e.workTree), now.Unix())
			os.WriteFile(filepath.Join(dir, name), []byte("A good ending.\n"+text), 0644)
		}
	}
}

// leave: she's gone for good. Her diary becomes a memory, her hooks come down, her heart
// stops. Graveyard and sacrificed refs stay: those are your things, not hers.
func (e *Engine) leave(now time.Time) {
	e.ending = "C"
	e.log(Event{Type: "off", Text: "the good ending"})
	e.remember() // before her diary becomes a memory: she'll know you parted well
	os.WriteFile(filepath.Join(dirOf(e.gitDir), "letter.txt"),
		[]byte(e.pick("letter_goodbye", nil)+"\n\n- her, "+now.Format("January 2, 2006")+"\n"), 0644)
	for _, name := range hookNames {
		p := filepath.Join(e.gitDir, "hooks", name)
		if data, err := os.ReadFile(p); err == nil && strings.Contains(string(data), hookMarker) {
			os.Remove(p)
		}
	}
	refs.Delete(e.gitDir, heartRef, "")
	dest := filepath.Join(e.gitDir, "cursed-memories")
	if _, err := os.Stat(dest); err == nil {
		dest += fmt.Sprintf("-%d", now.Unix())
	}
	os.Rename(dirOf(e.gitDir), dest)
	e.gone = true
	fmt.Fprintf(e.env.Out, "\n   She left a letter in %s.\n", filepath.Join(dest, "letter.txt"))
}
