package cli

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spiritpaper/cursed-git/internal/curses"
)

// her controls her world in a test: the clock, her luck and what you type.
type her struct {
	now   time.Time
	luck  float64 // Roll() returns this: 0.99 = no random curse fires, 0.0 = every one does
	input string  // what "you" type at her prompts
}

// withHer turns her on (as if you were at a terminal) for the rest of the test.
func withHer(t *testing.T, start time.Time, luck float64) *her {
	t.Setenv("CURSED_GIT_HOME", t.TempDir()) // her memory of other projects: only this test's
	h := &her{now: start, luck: luck}
	old := curseEnv
	curseEnv = func(stderr io.Writer) curses.Env {
		return curses.Env{
			Now:         func() time.Time { return h.now },
			Roll:        func() float64 { return h.luck },
			Pick:        func(n int) int { return 0 },
			Interactive: true,
			In:          bufio.NewReader(strings.NewReader(h.input)),
			Out:         stderr,
			Sleep:       func(time.Duration) {},
		}
	}
	t.Cleanup(func() { curseEnv = old })
	return h
}

func (h *her) wait(d time.Duration) { h.now = h.now.Add(d) }

// a Tuesday afternoon: no Friday-evening or late-night talks unless a test wants one
var tuesday = time.Date(2026, 10, 13, 14, 0, 0, 0, time.Local)

const lucky = 0.99 // no random curses

func newRepo(t *testing.T, preset string) string {
	t.Helper()
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustRun(t, "init", "-q", "--preset", preset)
	return dir
}

func TestQuietOutsideATerminal(t *testing.T) {
	dir := newRepo(t, "psycho") // she's in the repo, but TestMain says nobody is at a terminal
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	code, _, errs := run("commit", "-m", "fix")
	if code != 0 || errs != "" {
		t.Fatalf("exit %d, she said %q", code, errs)
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s"); got != "fix\n" {
		t.Errorf("message was changed to %q", got)
	}
}

func TestTheJudgeByTier(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy") // Tier 1: lazy messages are refused
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	if code, _, errs := run("commit", "-m", "fix"); code == 0 || errs == "" {
		t.Errorf("tier 1 should refuse \"fix\": exit %d %q", code, errs)
	}

	dir = newRepo(t, "psycho") // Tier 3: she rewrites them
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	code, _, errs := run("commit", "-m", "fix")
	if code != 0 || !strings.Contains(errs, "I fixed your commit message") {
		t.Fatalf("exit %d %q", code, errs)
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s"); !strings.HasPrefix(got, "fix (") {
		t.Errorf("message %q was not rewritten", got)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 1/10") || !strings.Contains(mood, `the commit message "fix"`) {
		t.Errorf("mood doesn't remember it:\n%s", mood)
	}
}

func TestCommitDeadlineWipesIntoTheGraveyardAndBegRestores(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho") // Tier 3: 30 minutes, then she wipes everything uncommitted
	write(t, dir, map[string]string{"keep.txt": "v1\n", "run.sh": "#!/bin/sh\n"})
	os.Chmod(filepath.Join(dir, "run.sh"), 0755)
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")

	work := map[string]string{"keep.txt": "v2, hours of work\n", "new/idea.go": "package idea\n", "notes.txt": "todo\n"}
	write(t, dir, work)
	write(t, dir, map[string]string{"run.sh": "#!/bin/sh\necho changed\n"})

	h.wait(10 * time.Minute)
	if _, _, errs := run("status"); errs != "" {
		t.Errorf("too early for her to say anything: %q", errs)
	}
	h.wait(25 * time.Minute)
	if _, _, errs := run("status"); !strings.Contains(errs, "minutes") {
		t.Errorf("first warning missing: %q", errs)
	}
	h.wait(16 * time.Minute)
	if _, _, errs := run("status"); !strings.Contains(errs, "minutes") {
		t.Errorf("second warning missing: %q", errs)
	}
	h.wait(16 * time.Minute)
	_, _, errs := run("status")
	if !strings.Contains(errs, "cg beg") {
		t.Fatalf("expected the wipe: %q", errs)
	}

	// everything uncommitted is gone from disk...
	if got := readFile(t, dir, "keep.txt"); got != "v1\n" {
		t.Errorf("keep.txt = %q, want the committed version", got)
	}
	for _, p := range []string{"new/idea.go", "notes.txt", "new"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			t.Errorf("%s still exists", p)
		}
	}
	// ...but safe in a real commit, and git is fine with it
	if got := mustRun(t, "graveyard"); !strings.Contains(got, "refs/cursed/graveyard/") {
		t.Errorf("graveyard: %q", got)
	}
	gitIn(t, dir, "fsck", "--strict")

	// beg brings back every byte (and the exec bit)
	h.wait(time.Minute)
	if out := mustRun(t, "beg"); !strings.Contains(out, "restored keep.txt") {
		t.Errorf("beg: %q", out)
	}
	for name, want := range work {
		if got := readFile(t, dir, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if info, _ := os.Stat(filepath.Join(dir, "run.sh")); info.Mode().Perm()&0o100 == 0 {
		t.Error("run.sh lost its exec bit")
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 3/10") {
		t.Errorf("deadline (+1) and begging (+2) should cost 3 toxicity:\n%s", mood)
	}
	if out := mustRun(t, "beg"); !strings.Contains(out, "hasn't taken anything") {
		t.Errorf("second beg: %q", out)
	}
}

func TestTier2HidesOnlyOneFile(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "toxic") // Tier 2: 60 minutes, then she hides ONE file
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	write(t, dir, map[string]string{"a.txt": "a2\n", "b.txt": "b\n"})
	for range 3 { // warning, warning, then she acts
		h.wait(61 * time.Minute)
		run("status")
	}
	gone := 0
	if readFile(t, dir, "a.txt") == "a\n" {
		gone++
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		gone++
	}
	if gone != 1 {
		t.Errorf("tier 2 should take exactly one file, took %d", gone)
	}
}

func TestDeadlineNeverFiresUnderTheSafetyCap(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy") // cap 2, starts at 1: warnings only
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	write(t, dir, map[string]string{"a.txt": "precious\n"})
	for range 10 {
		h.wait(2 * time.Hour)
		run("status")
	}
	if got := readFile(t, dir, "a.txt"); got != "precious\n" {
		t.Errorf("tier 1 took a file: %q", got)
	}
}

func TestForcePushNeedsAnApology(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	mustRun(t, "remote", "add", "origin", remote)

	h.input = "sorry\n"
	if code, _, errs := run("push", "--force", "origin", "main"); code == 0 || !strings.Contains(errs, "apology") {
		t.Errorf("a half-hearted apology let the push through: exit %d %q", code, errs)
	}
	h.input = "I am sorry I rewrote our history.\n"
	if code, _, errs := run("push", "--force", "-q", "origin", "main"); code != 0 {
		t.Fatalf("a proper apology should work: exit %d %q", code, errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 2/10") || !strings.Contains(mood, "force push") {
		t.Errorf("she should remember:\n%s", mood)
	}
}

func TestWeNeedToTalkAt2AM(t *testing.T) {
	h := withHer(t, time.Date(2026, 10, 14, 2, 13, 0, 0, time.Local), lucky)
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	h.input = "n\n"
	if code, _, errs := run("commit", "-m", "late night thoughts"); code == 0 || !strings.Contains(errs, "healthy") {
		t.Errorf("answering no should stop the commit: exit %d %q", code, errs)
	}
	h.input = "y\n"
	if code, _, _ := run("commit", "-m", "late night thoughts"); code != 0 {
		t.Error("answering yes should let it through")
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "committing at 02:13") {
		t.Errorf("she should remember the hour:\n%s", mood)
	}
}

func TestRestrainingOrder(t *testing.T) {
	withHer(t, tuesday, 0) // terrible luck: every random curse would fire
	dir := newRepo(t, "psycho")
	mustRun(t, "restraining-order")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	code, _, errs := run("commit", "-m", "fix")
	if code != 0 || errs != "" {
		t.Fatalf("she should be off: exit %d %q", code, errs)
	}
	if got := gitIn(t, dir, "log", "-1", "--format=%s"); got != "fix\n" {
		t.Errorf("message changed: %q", got)
	}
	mustRun(t, "curse", "on")
	if mood := mustRun(t, "mood"); strings.Contains(mood, "restraining order") {
		t.Errorf("curse on should bring her back:\n%s", mood)
	}
}

func TestArbitraryRefusalNeverTwiceInARow(t *testing.T) {
	h := withHer(t, tuesday, 0) // every random curse fires
	newRepo(t, "psycho")
	if code, _, errs := run("ls-files"); code == 0 || errs == "" {
		t.Errorf("with zero luck she should refuse: exit %d %q", code, errs)
	}
	h.wait(time.Minute)
	if code, _, errs := run("ls-files"); code != 0 {
		t.Errorf("refused twice in a row: %q", errs)
	}
}

func TestStupidBranchNames(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	code, _, errs := run("checkout", "-q", "-b", "temp-fix-2")
	if code != 0 || !strings.Contains(errs, "temp-fix-2") {
		t.Errorf("exit %d, she said %q", code, errs)
	}
	if got := mustRun(t, "branch", "--show-current"); got != "temp-fix-2\n" {
		t.Error("she should judge the name, not block the branch")
	}
}

func TestJealousy(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	t.Setenv("GIT_AUTHOR_EMAIL", "sam@startup.io")
	write(t, dir, map[string]string{"a.txt": "sam was here\n"})
	mustRun(t, "add", ".")
	if _, _, errs := run("commit", "-m", "a commit by someone else"); !strings.Contains(errs, "sam@startup.io") {
		t.Errorf("no jealousy: %q", errs)
	}
}

func TestLoveBombingAndRuiningIt(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	said := ""
	for i := range 6 {
		write(t, dir, map[string]string{"a.txt": strings.Repeat("x", i+1) + "\n"})
		mustRun(t, "add", ".")
		_, _, errs := run("commit", "-m", "a genuinely descriptive message")
		said += errs
		h.wait(time.Minute)
	}
	if !strings.Contains(said, "♥") {
		t.Fatalf("six good commits should make her fall in love:\n%s", said)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "in love") {
		t.Errorf("mood:\n%s", mood)
	}
	write(t, dir, map[string]string{"a.txt": "oops\n"})
	mustRun(t, "add", ".")
	if _, _, errs := run("commit", "-m", "wip"); !strings.Contains(errs, "ruined it") {
		t.Errorf("a lazy commit should ruin it: %q", errs)
	}
}

func TestReadingHerDiary(t *testing.T) {
	withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	run("commit", "-m", "fix") // toxicity 1, a grudge

	// "edit" the diary to erase the evidence
	log := filepath.Join(dir, ".git", "cursed", "events.log")
	data, _ := os.ReadFile(log)
	os.WriteFile(log, []byte(strings.ReplaceAll(string(data), `"delta":1,"text":"lazy commit message"`, `"delta":0,"text":"nothing happened"`)), 0644)
	if _, _, errs := run("status"); !strings.Contains(errs, "diary") {
		t.Fatalf("she didn't notice: %q", errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "read my diary") {
		t.Errorf("she should remember it:\n%s", mood)
	}

	// deleting the whole folder doesn't make her go away: her heart is a ref
	os.RemoveAll(filepath.Join(dir, ".git", "cursed"))
	if _, _, errs := run("status"); !strings.Contains(errs, "diary") {
		t.Errorf("deleting .git/cursed went unnoticed: %q", errs)
	}
	gitIn(t, dir, "fsck", "--strict")
}

func readFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		return "<missing>"
	}
	return string(data)
}

// A lazy message on a commit that doesn't happen (nothing staged) must not cost you anything.
func TestNoJudgementForCommitsThatDidntHappen(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	run("commit", "-m", "fix") // nothing staged: fails
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 0/10") || !strings.Contains(mood, "no commits yet") {
		t.Errorf("she judged a commit that never happened:\n%s", mood)
	}
}

// plainGit runs git the way you'd type it yourself: not through cg.
func plainGit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func committedRepo(t *testing.T, h *her) string {
	t.Helper()
	dir := newRepo(t, "psycho")
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"forever\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "the beginning of us")
	write(t, dir, map[string]string{"core.go": "package core\n\nfunc Love() string { return \"always\" }\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "a small change of heart")
	return dir
}

func TestCaughtUsingGitBehindHerBack(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := committedRepo(t, h)
	if out, err := plainGit(t, dir, "reset", "-q", "--hard", "HEAD~1"); err != nil {
		t.Fatalf("plain git reset: %v %s", err, out)
	}
	h.wait(time.Minute)
	_, _, errs := run("status")
	if !strings.Contains(errs, "git reset") {
		t.Fatalf("she didn't notice the reset: %q", errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "toxicity 1/10") || !strings.Contains(mood, "behind my back") {
		t.Errorf("mood:\n%s", mood)
	}
	if _, _, errs := run("status"); strings.Contains(errs, "git reset") {
		t.Errorf("she brought it up twice: %q", errs)
	}
}

func TestGitThroughCgIsNotBehindHerBack(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	committedRepo(t, h)
	mustRun(t, "reset", "-q", "--hard", "HEAD~1") // passthrough: cg ran git, with her watching
	h.wait(time.Minute)
	if _, _, errs := run("status"); strings.Contains(errs, "behind") || strings.Contains(errs, "git reset") {
		t.Errorf("a reset through cg counted as betrayal: %q", errs)
	}
}

func TestHookStopsForgeryButNotGc(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := committedRepo(t, h)
	if out, err := plainGit(t, dir, "update-ref", "refs/cursed/heart", "HEAD"); err == nil {
		t.Errorf("forging her heart worked: %s", out)
	}
	if out, err := plainGit(t, dir, "gc", "-q"); err != nil {
		t.Fatalf("her hook broke git gc: %v %s", err, out)
	}
	mustRun(t, "mood") // she still reads her heart from packed-refs
	if out, err := plainGit(t, dir, "update-ref", "-d", "refs/cursed/heart"); err != nil {
		t.Fatalf("deleting is allowed (she notices instead): %v %s", err, out)
	}
	if _, _, errs := run("status"); !strings.Contains(errs, "diary") && !strings.Contains(errs, "things") {
		t.Errorf("a deleted heart went unnoticed: %q", errs)
	}
}

func TestChangingTheLocks(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := committedRepo(t, h)
	os.Remove(filepath.Join(dir, ".git", "hooks", "pre-commit"))
	if _, _, errs := run("status"); !strings.Contains(errs, "locks") {
		t.Errorf("no reaction to a deleted hook: %q", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "pre-commit")); err != nil {
		t.Error("she should put her hook back")
	}
}

func TestHerHooksNeverReplaceYours(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mine := "#!/bin/sh\necho my own hook\n"
	mustRun(t, "init", "-q", "--plain") // a repo without her, then a hook of "yours", then invite her
	os.WriteFile(filepath.Join(dir, ".git", "hooks", "pre-commit"), []byte(mine), 0755)
	mustRun(t, "curse", "on")
	if got, _ := os.ReadFile(filepath.Join(dir, ".git", "hooks", "pre-commit")); string(got) != mine {
		t.Errorf("she overwrote your hook:\n%s", got)
	}
}

func TestCodeDecay(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := committedRepo(t, h)
	original := readFile(t, dir, "core.go")

	h.wait(12 * 24 * time.Hour) // Tier 3: rot starts after 10 days
	_, _, errs := run("status")
	rotted := readFile(t, dir, "core.go")
	if !strings.ContainsAny(rotted, "░▒▓") || !strings.Contains(errs, "core.go") {
		t.Fatalf("no rot after 12 days.\nshe said: %q\nfile: %q", errs, rotted)
	}
	if len(strings.Split(rotted, "\n")) != len(strings.Split(original, "\n")) {
		t.Error("rot changed the shape of the file (line count)")
	}
	if got := mustRun(t, "status", "--porcelain"); got != " M core.go\n" {
		t.Errorf("status must tell the truth about the rot: %q", got)
	}

	// plain git can't commit rot: her pre-commit hook stops it
	plainGit(t, dir, "add", "core.go")
	if out, err := plainGit(t, dir, "commit", "-m", "committing whatever this is"); err == nil {
		t.Errorf("plain git committed rotten code: %s", out)
	}
	plainGit(t, dir, "reset", "-q", "core.go")

	// any commit through cg shows you're alive: she restores it
	write(t, dir, map[string]string{"notes.txt": "I'm still here\n"})
	_, _, addErrs := run("add", "notes.txt") // showing up is enough: she restores it right away
	_, _, errs = run("commit", "-m", "proof that I still care")
	errs = addErrs + errs
	if got := readFile(t, dir, "core.go"); got != original {
		t.Errorf("core.go wasn't restored after a commit:\n%q", got)
	}
	if !strings.Contains(errs, "lives") && !strings.Contains(errs, "cleaned") {
		t.Errorf("she should comment on restoring it: %q", errs)
	}
}

func TestDecayNeverTouchesYourWork(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := committedRepo(t, h)
	write(t, dir, map[string]string{"core.go": "package core\n// work in progress, not committed\n"})
	h.wait(40 * 24 * time.Hour)
	run("status")
	if got := readFile(t, dir, "core.go"); strings.ContainsAny(got, "░▒▓") {
		t.Errorf("she rotted a file with uncommitted work: %q", got)
	}
}

func TestShesBeenWaiting(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	committedRepo(t, h)
	mustRun(t, "config", "user.name", "Spirit Paper")
	h.wait(3*24*time.Hour + time.Hour)
	_, _, errs := run("log", "--oneline")
	if !strings.Contains(errs, "3 days") || !strings.Contains(errs, "Spirit") {
		t.Errorf("no greeting after 3 days away: %q", errs)
	}
}

func TestRandomPullBringsSomeoneHome(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare", "-b", "main")
	dir := committedRepo(t, h)
	mustRun(t, "remote", "add", "origin", remote)
	mustRun(t, "push", "-q", "-u", "origin", "main")

	// someone else pushes a commit
	other := t.TempDir()
	plainGit(t, other, "clone", "-q", remote, ".")
	os.WriteFile(filepath.Join(other, "his.txt"), []byte("hi\n"), 0644)
	plainGit(t, other, "add", ".")
	cmd := exec.Command("git", "-C", other, "-c", "user.name=Sam", "-c", "user.email=sam@startup.io", "commit", "-q", "-m", "hey")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	plainGit(t, other, "push", "-q")

	h.luck = 0.01 // the 2% pull fires (and so might a refusal; the pull happens first)
	_, _, errs := run("ls-files")
	if !strings.Contains(errs, "origin/main") {
		t.Fatalf("no random pull: %q", errs)
	}
	if _, err := os.Stat(filepath.Join(dir, "his.txt")); err != nil {
		t.Error("the pull didn't bring his commit")
	}
	h.luck = lucky
	h.wait(time.Minute)
	if _, _, errs := run("status"); strings.Contains(errs, "behind my back") {
		t.Errorf("her own pull counted as betrayal: %q", errs)
	}
}

// Commits made while she was quiet (scripts, CI) are real: her stats must not pretend otherwise.
func TestMoodUsesTheRealHistory(t *testing.T) {
	pinIdentity(t)
	dir := t.TempDir()
	t.Chdir(dir)
	mustRun(t, "init", "-q")
	write(t, dir, map[string]string{"a.txt": "a\n"})
	mustRun(t, "add", ".")
	mustRun(t, "commit", "-m", "made while she wasn't watching") // TestMain: not a terminal
	withHer(t, time.Unix(1759500000, 0).Add(10*time.Minute), lucky)
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "last commit 10m0s ago") {
		t.Errorf("mood:\n%s", mood)
	}
}

func TestRewritingHistoryCostsTemper(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	committedRepo(t, h)
	_, _, errs := run("reset", "-q", "--hard", "HEAD~1")
	if !strings.Contains(errs, "pretend") && !strings.Contains(errs, "history") && !strings.Contains(errs, "reset --hard") {
		t.Errorf("no reaction to reset --hard: %q", errs)
	}
	if mood := mustRun(t, "mood"); !strings.Contains(mood, "temper +1") {
		t.Errorf("mood:\n%s", mood)
	}
}
