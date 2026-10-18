// internal/curses/quests.go
//
// Quests: a learning path through git, as gameplay. `cg quest` gives you the next one. She
// checks it against the real repository after every command (the reflog, refs, what you ran,
// her diary), so there's no honour system. Finishing one pays XP and tells you what git
// actually did, in one sentence: that's the lesson.
package curses

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spiritpaper/cursed-git/internal/refs"
)

// Quest is one step of the path.
type Quest struct {
	ID, Title, Task, Hint, Lesson string
	XP                            int
	done                          func(q questCtx) bool
}

// QuestStart is what the repository looked like when a quest began (in her diary).
type QuestStart struct {
	ID       string `json:"id"`
	Reflog   int    `json:"reflog"`
	Branches int    `json:"branches"`
	Tags     int    `json:"tags"`
}

// questCtx is what a quest's check can look at.
type questCtx struct {
	start          QuestStart
	reflog         []string // HEAD reflog messages since the quest began
	branches, tags int
	cmd            string
	args           []string
	ok             bool    // the command succeeded
	events         []Event // her diary since the quest began
}

func (q questCtx) reflogHas(prefix string) bool {
	for _, m := range q.reflog {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

func (q questCtx) ran(cmd string, flags ...string) bool {
	if !q.ok || q.cmd != cmd {
		return false
	}
	for _, f := range flags {
		if !contains(q.args, f) {
			return false
		}
	}
	return true
}

func (q questCtx) happened(event string) bool {
	for _, e := range q.events {
		if e.Type == event {
			return true
		}
	}
	return false
}

// Quests, in the order she hands them out.
var Quests = []Quest{
	{ID: "branch", Title: "Branch Out", XP: 50,
		Task:   "Make a new branch.",
		Hint:   "cg switch -c <name>   (or cg branch <name>)",
		Lesson: "A branch is a tiny file in .git/refs/heads that holds one commit hash. That's all. Making one is free.",
		done:   func(q questCtx) bool { return q.branches > q.start.Branches }},
	{ID: "peek", Title: "Peek Inside", XP: 50,
		Task:   "Read a commit object with your own eyes.",
		Hint:   "cg cat-file -p HEAD",
		Lesson: "A commit is a small text file: a tree (the snapshot), parent hashes, author, message. Its hash is the hash of that text.",
		done:   func(q questCtx) bool { return q.ran("cat-file", "-p") }},
	{ID: "merge", Title: "Bring It Home", XP: 100,
		Task:   "Merge a branch into the one you're on.",
		Hint:   "commit something on another branch, cg switch back, then cg merge <branch>",
		Lesson: "If your branch hadn't moved, git just slid it forward (fast-forward). Otherwise it made a commit with two parents.",
		done:   func(q questCtx) bool { return q.reflogHas("merge ") || q.reflogHas("commit (merge)") }},
	{ID: "tag", Title: "Tag It", XP: 50,
		Task:   "Put a tag on a commit, like a release.",
		Hint:   "cg tag v0.1",
		Lesson: "A tag is a name for a commit that never moves, unlike a branch, which follows your new commits.",
		done:   func(q questCtx) bool { return q.tags > q.start.Tags }},
	{ID: "soft", Title: "Undo, Keep the Work", XP: 100,
		Task:   "Undo your last commit but keep its changes staged.",
		Hint:   "cg reset --soft HEAD~1",
		Lesson: "--soft only moves the branch back. The index and your files stay, so the work looks staged again.",
		done:   func(q questCtx) bool { return q.ran("reset", "--soft") }},
	{ID: "amend", Title: "Second Thoughts", XP: 75,
		Task:   "Fix your last commit (message or content) without making a new one on top.",
		Hint:   "cg commit --amend -m \"a better message\"   (she'll hate it. Do it anyway.)",
		Lesson: "Amend doesn't edit a commit, nothing in git can. It makes a new one and moves the branch to it. The old one is still in the reflog.",
		done:   func(q questCtx) bool { return q.reflogHas("commit (amend)") }},
	{ID: "cherry", Title: "Pick a Cherry", XP: 100,
		Task:   "Copy one commit from another branch onto yours.",
		Hint:   "cg cherry-pick <commit>",
		Lesson: "Cherry-pick replays the change a commit made, as a new commit with a new hash. Same change, different history.",
		done:   func(q questCtx) bool { return q.reflogHas("cherry-pick") }},
	{ID: "stash", Title: "Hide It", XP: 75,
		Task:   "Put unfinished work away, then bring it back.",
		Hint:   "change a file, cg stash, then cg stash pop",
		Lesson: "A stash is a commit nobody points at except refs/stash. pop applies it and drops it.",
		done:   func(q questCtx) bool { return q.ran("stash", "pop") || q.ran("stash", "apply") }},
	{ID: "restore", Title: "Bring It Back", XP: 50,
		Task:   "Delete or mangle a tracked file, then get it back from git.",
		Hint:   "rm <file>, then cg restore <file>",
		Lesson: "Git kept the file as a blob in the last commit (and in the index). restore just writes that blob back out.",
		done:   func(q questCtx) bool { return q.ran("restore") }},
	{ID: "conflict", Title: "Survive a Conflict", XP: 150,
		Task:   "Make two branches change the same line, merge them, and resolve it. No --abort.",
		Hint:   "same line, different text on two branches, then cg merge. Fix the file, cg add, cg commit.",
		Lesson: "During a conflict the index holds three versions of the file (stages 1, 2, 3). cg add replaces them with your one.",
		done:   func(q questCtx) bool { return q.happened("fight_won") || q.happened("conflict_resolved") }},
	{ID: "rebase", Title: "Rewrite the Road", XP: 150,
		Task:   "Rebase a branch onto another.",
		Hint:   "on your feature branch: cg rebase main",
		Lesson: "Rebase replays your commits on top of another branch, one by one. New parents, so new hashes: that's why you never rebase what you already pushed.",
		done:   func(q questCtx) bool { return q.reflogHas("rebase (finish)") }},
}

func questByID(id string) (Quest, bool) {
	for _, q := range Quests {
		if q.ID == id {
			return q, true
		}
	}
	return Quest{}, false
}

// countTags counts tags, loose and packed.
func (e *Engine) countTags() int {
	n := 0
	filepath.WalkDir(filepath.Join(e.gitDir, "refs", "tags"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	if data, err := os.ReadFile(filepath.Join(e.gitDir, "packed-refs")); err == nil {
		n += strings.Count(string(data), " refs/tags/")
	}
	return n
}

// NextQuest is the first quest you haven't finished (or skipped), nil when you've done them all.
func (e *Engine) NextQuest() *Quest {
	for _, q := range Quests {
		if _, done := e.st.QuestsDone[q.ID]; !done {
			q := q
			return &q
		}
	}
	return nil
}

// ActiveQuest is the quest you're on, if any.
func (e *Engine) ActiveQuest() *Quest {
	if e.st.Quest == nil {
		return nil
	}
	q, ok := questByID(e.st.Quest.ID)
	if !ok {
		return nil
	}
	return &q
}

// StartQuest begins the next quest (cg quest).
func (e *Engine) StartQuest() *Quest {
	q := e.NextQuest()
	if q == nil {
		return nil
	}
	start := QuestStart{ID: q.ID, Reflog: len(e.reflogLines()), Branches: len(refs.Branches(e.gitDir)), Tags: e.countTags()}
	e.log(Event{Type: "quest", Quest: &start})
	return q
}

// SkipQuest gives up on the current quest (it counts as done, for no XP).
func (e *Engine) SkipQuest() {
	if q := e.ActiveQuest(); q != nil {
		e.log(Event{Type: "quest_skipped", Text: q.ID})
	}
}

// checkQuest runs after every command: did that finish the quest you're on?
func (e *Engine) checkQuest(cmd string, args []string, code int) {
	s := e.st.Quest
	q := e.ActiveQuest()
	if s == nil || q == nil {
		return
	}
	ctx := questCtx{start: *s, branches: len(refs.Branches(e.gitDir)), tags: e.countTags(), cmd: cmd, args: args, ok: code == 0}
	lines := e.reflogLines()
	for _, l := range lines[min(s.Reflog, len(lines)):] {
		if _, msg, ok := strings.Cut(l, "\t"); ok {
			ctx.reflog = append(ctx.reflog, msg)
		}
	}
	started := false
	for _, ev := range e.diary.Events() {
		if ev.Type == "quest" && ev.Quest != nil && ev.Quest.ID == q.ID {
			started, ctx.events = true, nil
			continue
		}
		if started {
			ctx.events = append(ctx.events, ev)
		}
	}
	if !q.done(ctx) {
		return
	}
	e.log(Event{Type: "quest_done", Text: q.ID})
	e.log(Event{Type: "xp", Delta: q.XP, Text: "quest: " + q.Title})
	out := e.env.Out
	fmt.Fprintf(out, "\n📜  Quest complete: %s (+%d XP)\n    What you learned: %s\n", q.Title, q.XP, q.Lesson)
	e.say("quest_done", map[string]string{"quest": q.Title})
	e.charm(e.env.Now(), "you learned "+q.Title)
	if next := e.NextQuest(); next != nil {
		fmt.Fprintf(out, "    Next: `cg quest` for %q.\n\n", next.Title)
	} else {
		fmt.Fprintln(out, "    That was the last one. You know more git than most people who use it every day.")
		fmt.Fprintln(out)
	}
}
