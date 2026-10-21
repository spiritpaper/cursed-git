# How the game works

> *"She's a version control system. She controls you."*

This is the long version of what Miranda does, when she does it, and why it's safe. If you
just want to play, [How to play](PLAYING.md) is enough. Read this if you're running an event,
writing a character, contributing code, or you just lost a file and want to know why.

🎬 marks a moment with a matching animation in `cursed-git-video/` (list at the end).

> **A note from me (the human who made this).** Cursed GIT is the thing I actually wanted to build, and building it taught me a ton about git internals, Go and binary formats. It's a joke character, though. Thinking about a real ex (or a real partner) as someone who punishes you, checks up on you and won't let you leave is not a cute idea, it's a red flag. If any of her lines sound like a relationship you're in, that's worth talking to someone about. Laugh at her, learn git from her, don't date her.

## The short version

You use `cg` wherever you'd type `git`. Everything git can do still works: what cg doesn't
handle itself, it hands straight to git. But the repository now has someone living in it.

- **She has a mood**, from 0 (fine) to 4 (catastrophic). The worse it is, the more she does.
- **Doing git well calms her down**: commit often, write real messages, pull before you push.
- **Doing it badly makes her worse**: lazy messages, uncommitted work, force pushes, sneaking
  around her with plain git.
- **Merge conflicts are boss fights** with a timer.
- **Real work earns XP**, and with enough of it you can break up with her. How that goes
  depends on how you treated her.
- **She never destroys your work.** Anything she takes is saved first, and `cg beg` gives it back.

She's **Miranda** by default. **Juniper** is the gentle alternative: same game, no curses at
all. Both are [line packs](CHARACTERS.md), and Miranda's personality has its own guide
([MIRANDA.md](MIRANDA.md)).

## Meet her

She's an ASCII-art anime girl, and her face changes with her mood. `cg face` shows her,
`cg mood` shows how she feels about you right now.

```
          _.-''''''''''-._
       .-'  .-''''''''-.  '-.
     .'   .'/\/\/\/\/\/\'.   '.
    /   .' |    ^  ^    | '.   \
   /   /   |    .  .    |   \   \
   |   |    \   \__/   /    |   |
   |   |     '.______.'     |   |
   |   |      _|    |_      |   |
  /|   |    .' '----' '.    |   |\
 / |   |   /     <>     \   |   | \
```

| Mood | Eyes | Cheeks | Mouth |
|---|---|---|---|
| 0 content | `^  ^` | `.  .` | `\__/` |
| 1 unimpressed | `-  -` | | `__` |
| 2 judging | `=  =` | | `~~` |
| 3 hostile | `\  /` | `o  o` | `/^^\` |
| 4 furious | `>  <` | `#  #` | `/VVVV\` |
| love-bombing | `<3  <3` | `//  //` | `ww` |
| crying (the good ending) | `T  T` | `;  ;` | `()` |

Only the left half of her is drawn by hand; the code mirrors it, so she's always symmetric
(`internal/curses/portrait.go`). In a normal sentence she uses a small face instead:
`( ^_^ )` up to `(╬ ò_ó)`.

---

## 1. The promise: nobody loses work

The joke only works if it stays a joke. These rules are in the code and in the tests, and a
change that breaks one doesn't get merged.

1. **She only lives where you invited her.** `cg init` or `cg curse on` moves her in (into
   `.git/cursed/`). In any other repository, `cg` is just git.
2. **She saves before she takes.** Before she hides, wipes or drops anything, she stores it as
   git objects under `refs/cursed/graveyard/…` (or `refs/cursed/sacrificed/…` for commits).
   `cg beg` gives it back; even plain git can (`git checkout refs/cursed/graveyard/…`). If she
   can't save something, she doesn't take it.
3. **She lies about her feelings, never about your data.** `status`, `log`, hashes and the
   numbers in `cg mood` are always true. Git can always prove what really happened.
4. **She only acts at a terminal.** In a script, an editor, CI (`CI=true`) or anywhere without
   a person at a keyboard, she's silent and `cg` is plain git.
5. **The emergency exit always works.** `cg restraining-order` stops every curse immediately,
   no conditions, no punishment. It's for real emergencies (a demo, a work deadline), not a
   way to win: *"Fine. Legal says I have to stop."*
6. **Dark, not cruel.** No threats of violence or self-harm, nothing sexual, nothing outside
   the repository. Guilt, jealousy, love-bombing and passive aggression: yes.
7. **Real damage needs Death March.** The only things that reach outside the repository (a real
   shutdown, deleting the project folder) happen only in [Death March](#6-death-march-),
   which you turn on by hand and accept by typing an oath.

---

## 2. Her tier: one dial for difficulty and safety 🎬

Everything she does depends on one number, her **tier**, from 0 to 4.

| Tier | Name | Face | What it feels like |
|---|---|---|---|
| 0 | **Safe** | `( ^_^ )` | Comments and opinions. She never blocks you. |
| 1 | **Annoying** | `( -_- )` | She refuses things, judges you, warns you. She never touches files. |
| 2 | **Hostile** | `( ¬_¬ )` | Deadlines with consequences, pull requests from your team, force-push apologies. |
| 3 | **Dangerous** | `( ò_ó )` | Takes uncommitted work, rots old files, drops commits, boss fights. |
| 4 | **Catastrophic** | `(╬ ò_ó)` | No more warnings. Losing means a shutdown (a fake one, outside Death March). |

### Where she starts and how far she can go

You pick a **preset** when she moves in (`cg init --preset …` or `cg curse on --preset …`).
It sets where she starts and the highest tier she can ever reach:

| Preset | Starts at | Never above | Good for |
|---|---|---|---|
| `clingy` | 1 | 2 | a project you actually care about |
| `toxic` | 2 | 3 | a gentler game |
| **`psycho`** (default) | **3** | **4** | the full experience |
| `hackathon` | 3 | 4 | events: the endgame goals shrink to fit the event (`--hours N`) |
| `gentle` | 0 | 0 | Juniper: nothing bad, ever |

### How her tier moves

Her live tier is worked out fresh on every command:

```
tier = where she started + her temper + 1 if you're as toxic as she is
       (never more than 2 below where she started, never above the preset's cap)
```

**Temper** is the short-term part: −2 to +2, drifting one step back toward 0 for every two
quiet hours.

| What you did | Temper |
|---|---|
| Ignored three of her warnings | +1 |
| `cg beg`, a force push, `reset --hard`, `revert`, `commit --amend` | +1 each |
| The same lazy mistake twice in 24 hours | +1 |
| Used plain git behind her back | +1 |
| Three good commits in a row | −1 |
| Won a boss fight | −1 |

At −2 she **love-bombs** you: an hour of compliments and no curses at all. A lazy commit
message during that hour ruins the moment (+4).

**Your toxicity** is the long-term part. It's a score of how *you* behaved, and it mostly only
goes up:

| What you did | Toxicity |
|---|---|
| A lazy commit message (`fix`, `wip`, anything under 5 characters) | +1 |
| Missed the commit deadline | +1 |
| Used plain git behind her back | +1 |
| Had to `cg beg` for your files | +2 |
| Force push | +2 |
| Deleted her graveyard | +2 |
| Lost a boss fight | +3 |
| Won a boss fight with more than half the clock left | −1 |

Reach **10** and you're as toxic as she is: she's one tier worse from then on (*"I learned it
from you"*), and it decides how the breakup ends.

### Her clocks

Some curses run on timers, and the timers get shorter as she gets angrier:

| Tier | 0 | 1 | 2 | 3 | 4 |
|---|---|---|---|---|---|
| commit deadline | off | 60 min (warnings only) | 60 min | 30 min | 15 min |
| boss fight, per conflict | — | — | — | 3 min | 90 s |
| an untouched file fully rots | — | — | — | 30 days | 10 days |

All of these can be changed per repository (`speed`, `deadline`, `fightTime`, `decay` in
[the settings](#8-settings)). A one-day jam might use `speed = 2`.

### When does she act?

Only when you run `cg`. There's no background process: on every command she checks the
clock, catches up on whatever "should have happened" while you were away, and then lets the
command run. That keeps her predictable and easy to test.

---

## 3. What she does, tier by tier

Every curse needs her tier to be at least its own. The lines quoted here are real ones from
her pack (`internal/curses/lines/miranda.json`); she has a few for every moment and doesn't
repeat herself.

### Tier 0: Safe 🎬 *(cg-montage)*

- **She comments.** After a push, sometimes: *"At this point I'm doing more work than you."*
- **She reads your commit messages.** `wip` → *"Work in progress. Like us."* `revert` →
  *"You're taking it back? Like everything else?"* Most messages get no comment; she only
  reacts when she has something specific to say.
- **She judges lazy messages** (`fix`, `wip`, `asdf`, `final final`, anything under 5
  characters). At Tier 0 it's just a comment.
- **She notices big commits.** Eight or more files: *"Have you considered asking an AI to
  write the rest?"*
- **She holds grudges.** She remembers your worst moments and brings them up at random.
- **She's sometimes wonderful.** Rarely, she's warm and specific about your work. Then she
  takes it back (*"Don't get used to it."*). That push and pull is the hook; see [MIRANDA.md](MIRANDA.md).

### Tier 1: Annoying 🎬 *(cg-montage)*

- **Lazy messages are refused.** *"I'm not committing to "fix". I have standards. One of us
  has to."* Write a real one and it goes through.
- **Random rejections.** Now and then a commit is refused for a fake reason (*"Commit
  rejected. Mercury is in retrograde."*). Just try again.
- **Refusing anything.** Very rarely, any command: *"Not today."* Never twice in ten minutes.
- **Her rules.** Embarrassing branch names (`temp-fix-2`), 15+ files in one commit, a commit
  that only deletes things: she has opinions.
- **"We need to talk."** A commit after midnight (before 5am) or on a Friday after 4pm gets a
  question first: *"Is this really what you want to be doing right now? [y/N]"* Say no and
  the commit waits. Say yes and it goes through, and she remembers it.
- **The silent treatment.** When she's upset (temper +1), she sometimes just sighs, or makes
  you wait a few seconds.
- **Jealousy.** A commit by an author she doesn't know: *"Who is {email}? You never mentioned them."*
- **Rewriting history** (`reset --hard`, `revert`, `commit --amend`) is allowed, but it costs
  temper: *"You want to pretend it never happened?"*

### Tier 2: Hostile

**🕰️ The commit deadline.** Her clock starts the first time she sees uncommitted changes (at
any `cg` command), not at your last commit: one word typed after a quiet afternoon is a fresh
start. When the changes have been sitting there longer than the deadline (the clock table
above), she acts. Never while you're running `cg add` or `cg commit`, though: committing is
exactly what she wanted. Commit, or undo your changes, and the clock stops.

1. **Two warnings**, spaced out: *"{minutes} minutes. I'm not angry. I'm keeping a list."*
   Sometimes with a deal: *"Make 3 commits and I'll pause your deadline. Promise."* (Keep it
   and she'll deny it ever happened.)
2. **Then she takes something.** At Tier 2, one random changed file. At Tier 3, every
   uncommitted change. At Tier 4 she skips the warnings.

Taking always goes the same way: she confesses something, saves the files to her graveyard,
puts them back the way they were at your last commit, makes an excuse (*"I'm only version 0.1.
I'm still learning how to love you."*) and tells you how to get them back:
*"(cg beg, if you want them back. On your knees.)"* `cg beg` always works, and costs you +2
toxicity. `cg graveyard` lists everything she's holding.

At Tier 1 the deadline only ever warns.

**📬 Pull requests from your team.** If the repository has a remote, now and then she fetches
it, picks a branch someone else pushed that you don't have yet, and opens it as a pull
request:

```
📬 Pull request #3 from Sam: origin/login → main · 2 commits
   "add the login page"
( ò_ó )  Sam sent you a pull request. I already said yes for you. You're welcome.
```

Then she merges it. **If you've been in sync with your team** (pulling often, not working
on the same lines at once), it goes in clean: a real merge commit (`Merge pull request #3
from Sam (origin/login)`), +25 XP, and a jab: *"Not a single conflict. So you CAN communicate.
Just not with me."*

**If you haven't**, you both changed the same lines:
- At Tier 2 she asks git first (`git merge-tree`, which touches nothing), sees the conflict,
  and closes the request: *"I closed Sam's pull request. You two changed the same lines. Talk
  to each other."* Go and fix it yourself while it's still small.
- At Tier 3 and up she merges anyway, and the conflict is a **boss fight against Sam** (§4).

**Your uncommitted work is safe.** If the pull request changes a file you're still working
on, she doesn't merge it: *"Sam wants into core.go, but you're still in there. Commit
first."* It waits, and comes in right after your next commit. Work in other files is left
exactly where it is: git carries it through the merge (and through an aborted one). She
doesn't start while you have staged changes, since you're about to commit. A clean merge is
git's own, and `cg reset --keep ORIG_HEAD` takes it back without touching your uncommitted work.
This is the curse that teaches the most real teamwork: pull often, push small, talk.

**🔥 Force-push apologies.** `cg push --force` waits until you type, exactly, *"I am sorry I
rewrote our history."* Anything else and the push doesn't happen. Then +1 temper and +2
toxicity anyway.

### Tier 3: Dangerous

**🦠 Code decay** 🎬 *(code-decay).* Files nobody has touched in a long time start to rot in
your working tree, with characters slowly turning into `░▒▓`. It begins a third of the way
into the decay period and gets worse each time she looks.
- **Only files that exactly match the last commit rot**, so there's never work at risk. Git
  still has the clean version; `cg status` shows the file as modified.
- **Rot can't be committed.** `cg add` and `cg commit` clean it first, and her `pre-commit`
  hook stops plain `git commit` too.
- **Any commit cleans everything** (*"Fine. It lives."*). Rot only uses `░▒▓`, never
  look-alike characters, so it can't hide as a real bug.

**🔪 Commit sacrifice** 🎬 *(git-explained, scene 7).* When she's upset (temper +1 or more),
each commit has a 10% chance of: *"This repository cannot be saved as it is. Something has
to be sacrificed."* She drops one of your older commits and git replays everything after it
(`git rebase --onto`), so those commits get new IDs. It's the best way there is to understand
why a commit's ID depends on its parent.
- Only **unpushed** commits: rewriting history other people have is not hers to do.
- Never a merge, never the very first commit, never across a merge.
- The victim is kept forever under `refs/cursed/sacrificed/…`; `cg graveyard` lists it, and
  `cg cherry-pick <ref>` brings it back.
- At Tier 4 she takes two.

**⚔️ Boss fights**, next section.

### Tier 4: Catastrophic

She only gets here if you push her: ignoring warnings, losing fights, being as toxic as she is.
Everything above gets harsher:

- The commit deadline takes everything, with no warnings.
- Winning a boss fight isn't the end: she pulls again right away, and if that conflicts,
  it's **phase 2**.
- Losing a boss fight means she calls off the merge herself and keeps your half-done work
  in the graveyard.
- Commit sacrifice takes two.
- **The shutdown.** A missed deadline or a lost fight at Tier 4, and the screen goes black:
  *"Goodbye." … "(she turned off the lights)"*. Outside Death March that's all it is, a show
  in the terminal. In Death March it's real (§6).

---

## 4. Boss fights: merge conflicts 🎬 *(boss-fight)*

When git can't merge two versions of the same lines, it writes both into the file and asks you
to choose. At Tier 3 and up, that's a fight.

**How one starts.** Any merge can do it: your own `cg merge`, `cg pull`, `cg rebase` or
`cg cherry-pick`, one of her pull requests, or phase 2. Git does the merging (cg passes
it straight through); afterwards she looks at git's index, and if any file has conflict
stages in it, the fight begins.

- **The boss** is whoever you're fighting: the author of the pull request, **Chad from
  origin** for a plain pull from a remote, **Past You** when it's your own commits.
- **HP** is the number of conflict blocks (`<<<<<<<` … `>>>>>>>`), plus one for each file
  that was deleted on one side.
- **The clock** is HP × the per-conflict time for her tier (3 minutes at Tier 3, 90 seconds
  at Tier 4).
- **The intro teaches you the fight**: what the markers mean, `cg add` per file, `cg commit`
  to finish, and `cg ls-files -s` to see git's three copies of the file (1 = the common
  ancestor, 2 = yours, 3 = theirs).

**During the fight.**
- `cg status` (and `add`, `diff`, `ls-files`) shows a health bar, on stderr so the real
  output stays identical to git's: `⚔  Sam  HP ██████░░░░ 3/5  ⏳ 7:12`.
- Resolving a block is a hit.
- **You can't run.** `merge --abort`, `rebase --skip`, `reset --hard` and friends are refused:
  *"Running? cg disabled that."*
- **`cg add` won't stage a file that still has markers in it.** That's the classic beginner
  mistake, and git would happily let you make it.
- She taunts you now and then: *"You can't even handle a 3-way merge? Sam could."*
- Her other file-touching curses wait until the merge is done.

**Winning:** finish the merge with a commit before the clock runs out. +250 XP (plus a bonus
for time left), −1 temper, −1 toxicity if you were fast, and a K.O. screen.

**Losing:** the clock ran out (she notices at your next command), or the conflict vanished
without a commit because you ran with plain git. +3 toxicity, a grudge, and:
- At Tier 3 she **dictates your next commit message**: *"I lost to Sam and I am sorry"*.
- At Tier 4 she saves your half-done resolution to the graveyard, aborts the merge herself,
  and shuts down (fake, outside Death March). `cg beg` gives the work back.

The conflict itself is still there after a loss, as a normal git conflict; she doesn't start a
second fight over it.

---

## 5. The breakup 🎬 *(breakup)*

You can't just leave her. You have to earn it.

### Closure XP

| What you did | XP |
|---|---|
| Each line added in a commit | +1 (paid once the commit has survived a day) |
| A commit with a decent message (10+ characters, not lazy) | +10 |
| A clean merge from one of her pull requests | +25 |
| A boss fight won | +250, +1 for every 10 seconds left |

Your closure level is `floor(√(XP / 50))`, so level 10 is 5,000 XP. `cg mood` and `cg card`
show your progress. To be **allowed** to break up you need all three of: level 10, three boss
fights won, and 2,000 committed lines. (The hackathon preset scales these down to fit the event.)

### Three endings

`cg breakup` decides which one you get. `cg breakup --dry-run` tells you without doing it.

**A: "Too soon"** (you're not there yet). She doesn't argue. *"You don't get to leave me.
I leave you."* She writes you a letter, on your Desktop and in `.git/cursed/letters/`, built
from your history together: the day you met, your worst grudges, how far you still are. Then
the shutdown (fake, outside Death March). Next time you run `cg`, she asks if you read it.

**B: "The toxic ending"** (you made it, but your toxicity is 10 or more: you were as bad as
her). *"Fine. Go. But you're not taking anything with you."* She archives the whole project,
`.git` included, and checks the archive. Then:
- **In Death March** she empties the project folder and leaves one file in it,
  `SHE_TOOK_EVERYTHING.txt`, a receipt of everything you did.
- **Otherwise** she writes the receipt and deletes nothing: *"I wanted to burn it all.
  Apparently I'm not allowed."*

The receipt doesn't say how to get it back. The secret: `cg closure`, run in that folder,
restores everything from the archive. If the archive fails, she doesn't delete anything at
all (*"You're not even worth deleting."*), and she refuses outright outside your home folder
or on cg's own source code.

**C: "The good ending"** (you made it, and you treated her better than she treated you).
Denial (*"You're joking, right? After everything?"*), bargaining (you have to say no to every
deal she offers; saying yes cancels the breakup and she wins), tears, and then acceptance:
*"Okay. Be happy. Commit often. With someone else."* Her folder becomes
`.git/cursed-memories/`, the credits roll with your stats, and Juniper is introduced.

```
cg breakup
   ├─ not enough level / wins / lines ──► A: a letter, and a (fake) shutdown
   ├─ enough, but toxicity ≥ 10 ────────► B: archive, a receipt (deletion only in Death March)
   └─ enough, toxicity < 10 ────────────► C: denial, bargaining, tears, freedom
```

After a breakup she may come back days later, in any project (*the hoover*). Typing `no`
blocks her. See [MIRANDA.md](MIRANDA.md).

---

## 6. Death March ☠

The mode where her threats are real. It's made for events where everyone opted in, on
machines and projects that don't matter.

**Turning it on takes two deliberate steps,** and no cg command, event file or organizer can
do it for you:

1. Add it to the project's `.git/config` yourself:
   ```ini
   [cursed]
       deathMarch = true
   ```
2. At the next command she shows a warning that spells out exactly what will happen, and
   waits for you to type `I accept the death march`. Anything else changes nothing, and she
   asks again next time.

**What becomes real:**
- **The shutdown** (Tier 4 losses, ending A): `systemctl poweroff` on Linux,
  `osascript … shut down` on macOS, `shutdown /s /t 0` on Windows. No countdown, at most once
  a day. Unsaved work in *other* programs is lost; that's what the warning says.
- **Ending B deletes the project folder**, after archiving and checking it. `cg closure`
  brings it all back.

**What stays safe, even then:** everything in §1. The graveyard still saves before she takes,
`cg beg` still works, and she still only acts at a terminal. `cg mood` shows `☠ DEATH MARCH`
while it's on. Remove the setting, or use `cg restraining-order`, to end it.

The tests never shut anything down: every real action goes through a `Machine` interface that
the tests replace with a fake.

---

## 7. Why cheating doesn't work 🎬 *(anti-cheat)*

It's your machine, and real `git` is always one command away, so the game can't be cheat-proof.
Instead it's built so that cheating costs more than playing, and makes the story worse.

**Her state lives outside your history.** XP, toxicity, grudges and fights are in
`.git/cursed/`, which git never tracks. You can't `git revert` your way out of toxicity.

**XP can't be farmed.** Line XP is paid per *content*, by blob hash: she remembers every file
version she has paid for. Commit 500 lines, revert, commit them again: the second time the
blobs have the same hashes, so they pay nothing. Git's content addressing does the work.
And line XP only pays once the commit has survived a day (an hour at events), so work that
gets reset or rebased away never pays.

**She notices plain git.** She keeps track of every commit and ref change she made. If git's
reflog shows something she didn't do, you went behind her back: +1 temper, +1 toxicity, a
grudge, and *"I saw what you did with `git reset`. Who taught you that?"* (Her own merges,
pulls and rebases are marked as hers, so they never count against you.)

**Her hooks.** `cg init` installs a `reference-transaction` hook (it guards her refs under
`refs/cursed/`, and git aborts any change to them) and a `pre-commit` hook (no committing
rot). It never overwrites hooks you already have. Deleting or redirecting them (`core.hooksPath`)
is noticed: *"You changed the locks?"*

**Her diary is tamper-evident.** Everything that happens is appended to
`.git/cursed/events.log`. Each entry includes the hash of the one before (the same idea as
commits pointing at their parents), and the latest hash is also kept in `refs/cursed/heart`.
Her whole state is rebuilt by replaying the diary, so there's nothing else to edit. Change a
line, or delete the folder, and she knows: *"You read my diary."*

None of this stops someone who reads the source. It turns cheating into a story beat instead
of a quiet exploit.

---

## 8. Settings

Git ignores config sections it doesn't know, so her settings live in the repository's own
`.git/config`, under `[cursed]`. All of these are optional.

```ini
[cursed]
    personality = miranda     ; who you play with: miranda, juniper, or your own pack (CHARACTERS.md)

    ; clocks
    speed = 1                 ; every clock runs this many times faster (2 = a one-day jam)
    deadline = 30m            ; or set one clock at its Tier 3 value; other tiers keep their proportions
    fightTime = 3m
    decay = 30d               ; durations like 90m, 8h, 30d, 2d12h

    ; the endgame
    breakupLevel = 10
    minBossFightsWon = 3
    minLinesCommitted = 2000
    toxicThreshold = 10       ; where you count as toxic for the breakup
    xpMaturity = 24h          ; how long a commit must survive before its line XP pays

    ; events (set for you by --preset hackathon and cg event join)
    eventHours = 24           ; scales the endgame goals to the event
    eventEnds = 2026-11-01T18:00

    deathMarch = false        ; true, set by hand, plus the oath: real shutdowns and deletion (§6)
```

Her difficulty (the preset) is part of her diary, not the config, so it can't be quietly
edited down mid-game. Choose it when she moves in, with `--preset`.

Organizers can hand the same clock and goal settings to every team in an event file:
see [EVENTS.md](EVENTS.md).

---

## 9. Animations

All in `cursed-git-video/`, and playable in the browser. Add `?video` to the address for a
clean recording stage (its README has the details).

| Topic | Animation |
|---|---|
| Her tiers | `curse-tiers.html` (the ladder) · `her-tier.html` (how the parts add up) |
| Tier 0–1 curses | `cg-montage.html` (`?set=1` opinions, `?set=2` feelings) |
| Code decay | `code-decay.html` |
| Commit sacrifice | `git-explained.html?scene=7` |
| Boss fights | `boss-fight.html` |
| The breakup | `breakup.html` (`?ending=a`, `b`, `c`) |
| Anti-cheat | `anti-cheat.html` (`?scene=1` behind her back, `2` XP farming, `3` the diary) |
| How git works inside | `git-explained.html`, `git-internals.html` ([GIT.md](GIT.md)) |
