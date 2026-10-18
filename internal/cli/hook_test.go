package cli

import (
	"strings"
	"testing"
	"time"
)

// Push, then pull: after a real win she's wonderful, and on a later command she takes it back.
func TestPushAndPull(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "clingy")
	commitFile(t, dir, "a.txt", "a\n", "the first good message")
	commitFile(t, dir, "b.txt", "b\n", "the second good message")
	h.luck = 0.3 // the charm (50%) and the pull (60%) fire; nothing else at Tier 1 does
	write(t, dir, map[string]string{"c.txt": "c\n"})
	mustRun(t, "add", "c.txt")
	_, _, errs := run("commit", "-m", "the third good message")
	if !strings.Contains(errs, "You know you can do anything you want") {
		t.Fatalf("three good commits in a row: the charm:\n%s", errs)
	}
	if strings.Contains(errs, "Don't get used to it") {
		t.Error("the pull never comes in the same breath")
	}
	_, _, errs = run("status")
	if !strings.Contains(errs, "Don't get used to it") {
		t.Errorf("a later command: the pull:\n%s", errs)
	}
	for _, f := range []string{"d", "e", "f"} {
		commitFile(t, dir, f+".txt", f+"\n", "another good message, "+f)
	}
	if _, _, errs := run("status"); strings.Contains(errs, "anything you want") {
		t.Error("the charm is rare: at most once every few hours")
	}
	h.luck = lucky
}

func TestHerPromisesAreWorthNothing(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "toxic") // Tier 2: an hour's deadline
	commitFile(t, dir, "a.txt", "a\n", "a file to start with")
	write(t, dir, map[string]string{"a.txt": "unsaved\n"})
	h.wait(61 * time.Minute)
	h.luck = 0.2 // she offers a deal (30%)
	_, _, errs := run("status")
	h.luck = lucky
	if !strings.Contains(errs, "Make 3 commits and I'll pause your deadline") {
		t.Fatalf("with a warning, sometimes a deal:\n%s", errs)
	}
	var out string
	for i, f := range []string{"x", "y", "z"} {
		write(t, dir, map[string]string{f + ".txt": f + "\n"})
		mustRun(t, "add", ".")
		_, _, out = run("commit", "-m", "doing what she asked, part "+string(rune('1'+i)))
	}
	if !strings.Contains(out, "I said I'd *think* about it") || !strings.Contains(out, "Achievement unlocked: Pinky Promise") {
		t.Errorf("the third commit: she breaks it (and that's an achievement):\n%s", out)
	}
}

func TestTheLeashBeforeTheDoor(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	_, _, errs := run("breakup")
	if !strings.Contains(errs, "commits nobody points to") {
		t.Errorf("heading for the door, the wound comes first:\n%s", errs)
	}
	if code, out, _ := run("restraining-order"); code != 0 || !strings.Contains(out, "Restraining order filed") {
		t.Errorf("but the restraining order always just works: exit %d %q", code, out)
	}
}

func TestHerVersionOfWhatHappened(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	commitFile(t, dir, "a.txt", "a\n", "a file to start with")
	write(t, dir, map[string]string{"a.txt": "work she's about to take\n"})
	h.wait(31 * time.Minute)
	run("status")
	h.wait(16 * time.Minute)
	run("status")
	h.wait(16 * time.Minute)
	run("status") // the wipe
	h.wait(9 * time.Hour)
	h.luck = 0.3
	_, _, errs := run("status")
	h.luck = lucky
	if !strings.Contains(errs, "testing whether you'd fight for us") || !strings.Contains(errs, "cg reflog") {
		t.Errorf("after time apart, her version, and where the truth is:\n%s", errs)
	}
}

func TestTheLastLineIsYours(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	eligible(t, h, false)
	h.input = "yes\nyes\nyes\nno\nno\nno\n"
	_, _, errs := run("breakup")
	accept := strings.Index(errs, "Be happy")
	yours := strings.Index(errs, "you      No need. I hope you succeed.")
	if accept < 0 || yours < accept {
		t.Errorf("the good ending closes with your line, after hers:\n%s", errs)
	}
}

func TestRedFlagBingoCountsButNeverNames(t *testing.T) {
	withHer(t, tuesday, lucky)
	newRepo(t, "psycho")
	_, _, errs := run("breakup") // the leash
	for _, name := range []string{"leash", "flag", "tactic", "manipulat"} {
		if strings.Contains(strings.ToLower(errs), name) {
			t.Errorf("she must never name what she's doing (%q):\n%s", name, errs)
		}
	}
	card := mustRun(t, "card")
	if !strings.Contains(card, "Red flags  🚩 1/11") {
		t.Errorf("the card shows the count:\n%s", card)
	}
	if json := mustRun(t, "card", "--json"); !strings.Contains(json, `"red_flags": 1`) {
		t.Errorf("and so does the JSON:\n%s", json)
	}
	run("breakup") // the same red flag again doesn't count twice
	if card := mustRun(t, "card"); !strings.Contains(card, "🚩 1/11") {
		t.Errorf("each red flag counts once:\n%s", card)
	}
}

func TestTextsFromHerWhileYouWereGone(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	dir := newRepo(t, "psycho")
	commitFile(t, dir, "a.txt", "a\n", "a file to start with")
	h.wait(10 * time.Hour)
	h.luck = 0.3 // texts (60%) and the typing (50%)
	_, _, errs := run("status")
	h.luck = lucky
	if !strings.Contains(errs, "3 messages from Miranda while you were gone") || !strings.Contains(errs, "│ 15:00  you're not committing.") {
		t.Errorf("after 10 hours away: a stack of timestamped texts:\n%s", errs)
	}
	if !strings.Contains(errs, "Miranda is typing…") || !strings.Contains(errs, "Miranda is deleting…") {
		t.Errorf("...and what was she going to say?\n%s", errs)
	}
}

func TestSheComesBackInCostumes(t *testing.T) {
	h := withHer(t, tuesday, lucky)
	eligible(t, h, false)
	h.input = "yes\nyes\nyes\nno\nno\nno\n"
	run("breakup") // the good ending
	_, _, errs := run("partner", "juniper")
	if strings.Contains(errs, "where you left her") {
		t.Error("not right away: she waits a few days")
	}
	h.input = "no\n"
	var seen []string
	for i := 0; i < 5; i++ {
		h.wait(4 * 24 * time.Hour)
		_, _, errs = run("status")
		if strings.Contains(errs, "where you left her") {
			seen = append(seen, errs)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("four costumes, one at a time, then never again: saw %d\n%s", len(seen), strings.Join(seen, "\n---\n"))
	}
	if !strings.Contains(seen[0], "I've been journaling") || !strings.Contains(seen[3], "I still check your repos") {
		t.Errorf("the costumes come in order:\n%s\n...\n%s", seen[0], seen[3])
	}
	if !strings.Contains(seen[0], "Blocked.") || !strings.Contains(seen[0], "You don't owe her a reply") {
		t.Errorf("typing no blocks her, and Juniper is there:\n%s", seen[0])
	}
	if out := mustRun(t, "achievements"); !strings.Contains(out, "🏆 Seen Them All") {
		t.Errorf("all four costumes: an achievement:\n%s", out)
	}
}
