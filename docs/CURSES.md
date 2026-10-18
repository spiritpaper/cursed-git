# Cursed GIT — The Curse Part

> *"She's a version control system. She controls you."*

`cg` (short for **C**ursed **G**IT) is your toxic ex. It works like git, *mostly*, but it has moods, it gets jealous, it remembers everything, and if you ignore it, it will make you pay. Merge conflicts are **boss fights**, and leaving her is the final boss.

🎬 = a matching animation in `cursed-git-video/` (see §8).

> **A note from me (the human who made this).** Cursed GIT is the thing I actually wanted to build, and building it taught me a ton about git internals, Go and binary formats. It's a joke character, though. Thinking about a real ex (or a real partner) as someone who punishes you, checks up on you and won't let you leave is not a cute idea, it's a red flag. If any of her lines sound like a relationship you're in, that's worth talking to someone about. Laugh at her, learn git from her, don't date her.

## Status

Built so far (Phases 6 and 6b): the diary and heart, the tier engine, the graveyard and `cg beg`, `cg mood`, `cg curse on/off`, `cg restraining-order`, every Tier 0–2 curse in §3, the Tier 3/4 commit-deadline wipe, **code decay**, **random pull** (fast-forward at Tier 2, a real merge at Tier 3+), **boss fights** (§4), **commit sacrifice**, and the anti-cheat: reflog-based "behind her back" detection, her hooks (`reference-transaction`: she answers inside git's output and blocks forged changes to her refs; `pre-commit`: no committing rot), "you changed the locks", and the deleted-graveyard check. She also *remembers*: absence greetings, your name, your old commit messages, your favourite file, late-night talks. Also done: her memory across projects (`~/.local/share/cursed-git/memory.json`: a new `.git` or a fresh clone gets your toxicity and grudges back, she's jealous of your other projects, `cg forget` wipes it), XP and closure levels, 21 achievements (`cg achievements`, unlocked by replaying her diary, so they can't be faked without breaking it), `cg breakup` with all three endings, Death March (§5.5) and boss fight phase 2. Her lines live in `internal/curses/lines/default.json`.

## 0. Meet her

She's an ASCII-art anime girl: long hair, zigzag bangs, a ribbon, and a face that changes with her tier. `cg face [mood]` shows her, and `cg help` greets you with her best side.

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

| Tier / mood | Eyes | Cheeks | Mouth |
|---|---|---|---|
| 0 content | `^  ^` | `.  .` | `\__/` |
| 1 unimpressed | `-  -` | | `__` |
| 2 judging | `=  =` | | `~~` |
| 3 hostile | `\  /` | `o  o` | `/^^\` |
| 4 furious | `>  <` | `#  #` | `/VVVV\` |
| love-bombing | `<3  <3` | `//  //` | `ww` |
| crying (breakup C) | `T  T` | `;  ;` | `()` |

How she's drawn (`internal/curses/portrait.go`): only the left half of each row is written by hand, and the right half is mirrored in code, so she's always symmetric. The three face rows have a 6-character slot that the mood fills in. Tests check that every mood lines up. She's v1: redraw her any time, the tests will tell you if you broke the alignment.

The animations still use the small kaomoji faces (`( ^_^ )`, `(╬ ò_ó)`). Swapping them for her portrait is on the list for the launch phase.

---

## 1. Ground rules (read before cursing anything)

The joke only works if nobody actually loses their work. Every destructive curse must be **recoverable**, and the recovery can be part of the lore ("she hid your stuff, she didn't throw it out… this time").

1. **Opt-in per repo.** Curses only activate if `.git/cursed/` exists, which `cg init` or `cg curse on` creates. `cg` used on a normal repo is just git.
2. **Nothing is truly deleted.** Before a curse wipes, rots or sacrifices anything, it snapshots it as git objects plus a ref: `refs/cursed/graveyard/<timestamp>`. `cg beg` (or `cg apologize`) restores it. Real git can recover it too, with `git checkout refs/cursed/graveyard/...`. This includes the "she deletes your repo" ending (§5): the repo is archived before it disappears.
3. **Never touch `.git/` internals destructively.** Curses act on the working tree, on branch history through normal commits and refs, and on refs under `refs/cursed/`. They never touch objects that already exist.
4. **The tier cap is the safety protocol** (§2). `cursed.maxTier` is the most dangerous tier she may ever reach. Set it to `1` on a repo you care about, and nothing worse than insults can happen.
5. **Real damage needs Death March.** The real shutdown and the real deletion in ending B only happen in **Death March** mode (§5.5): `cursed.deathMarch = true`, set by hand in `.git/config`, plus a warning you accept by typing the oath. Otherwise she fakes the shutdown and deletes nothing.
6. **Emergency exit.** `cg restraining-order` disables every curse immediately, with no conditions and no punishment. It is for real emergencies, like a deadline at work or a demo, and it is **not** a game ending. It should be boring on purpose (*"Fine. Legal says I have to stop."*).
7. **Never curse on CI.** If `CI=true` or stdin isn't a terminal, stay quiet.
8. **She lies about her feelings, never about your data.** She can guilt-trip, twist your words and rewrite history *emotionally*, but `status`, `log`, `mood` statistics and every hash are always the truth, and everything she takes is recoverable. A version control tool you can't trust stops being funny.
9. **Dark, not cruel.** No threats of self-harm or violence, nothing sexual, and she never looks or reaches outside the repository (her "network" is `git pull` on your own remote). Manipulation, guilt, jealousy, love-bombing: yes. Anything that would make a real person feel unsafe: no.

These rules also make good YouTube content: show that "she deleted everything", then show the rescue.

---

## 2. Her toxicity tier (difficulty + safety in one dial) 🎬

There is one number that decides how dangerous she is right now: **her tier, 0–4**. The tier is her face, her mood, and the list of curses she's allowed to use. The *difficulty* is where she starts, and the *safety protocol* is how high she can ever go.

| Tier | Name | Face | What she does | Examples |
|---|---|---|---|---|
| 0 | **Safe** | `( ^_^ )` | Mostly personality. Never blocks you. | toxic partner · commit message judge · AI will replace you · grudges |
| 1 | **Annoying** | `( -_- )` | Warnings, insults, rejected commands. Never touches files. | random rejection · arbitrary refusal · stupid rules · silent treatment · jealousy |
| 2 | **Hostile** | `( ¬_¬ )` | Deadlines, forced problems. Hides things (recoverably). | commit deadline · random pull · force-push apology |
| 3 | **Dangerous** | `( ò_ó )` | Decay, sacrifice, forced conflicts. | code decay · commit sacrifice · merge conflict boss fights |
| 4 | **Catastrophic** | `(╬ ò_ó)` | Only if you truly deserve it. | instant wipes · boss fight phase 2 · full-force breakup endings |

### 2.1 How her live tier is calculated

```
live tier = clamp( base + temper + grudge, floor, maxTier )

base     = cursed.tier            (difficulty, default 3 = Dangerous)
temper   = -2 … +2                (how you've treated her lately, see below)
grudge   = +1 if YOUR toxicity ≥ toxicThreshold, else 0   (§5.2)
floor    = max(0, base - 2)       (she can calm down two tiers, never further)
maxTier  = cursed.maxTier         (safety cap, default 4)
```

**Temper** is the short-term part. It's the mood swings:

| Event | Temper |
|---|---|
| A warning you ignored (the condition is still true at the next check) | +1 per 3 ignored warnings |
| `cg beg`, force push, `revert`/`reset --hard`, getting caught using plain git | +1 each |
| A good commit (decent message, within the deadline) | −1 per 3 in a row |
| Winning a boss fight | −1 |
| Saying "yes" during breakup bargaining (§5.3 C) | −2 (love-bombing) |

Temper slowly drifts back to 0. One step toward zero per `temperCooldown` (default 2h) without events.

**Your toxicity** (§5.2) is the long-term part. It only goes up, and once it crosses the threshold she's permanently one tier worse: *"I learned it from you."*

### 2.2 What the tier controls
- **Which curses can fire:** a curse fires only if `live tier ≥ its tier`. A curse above `maxTier` never exists at all.
- **How often:** random curses use `chance × (1 + live − curse tier)`, so a Tier-1 curse fires three times as often at Tier 3.
- **How fast:** timeouts shrink as the tier rises.

  | Live tier | 0 | 1 | 2 | 3 | 4 |
  |---|---|---|---|---|---|
  | commit deadline | off | warn only | 60 min | 30 min | 15 min |
  | boss fight timer per hunk | — | — | — | 3 min | 90 s |
  | code decay to "critical" | off | off | off | 30 days | 10 days |

### 2.3 Defaults
Out of the box she starts at **Tier 3 (Dangerous)**, one step below the top, with the cap at **4**. With the default `base` of 3, Tier 4 is never the starting point. You reach it by ignoring her: three ignored warnings push your temper to +1 and her to Catastrophic. Good behaviour can bring her down to Tier 1, but never lower.

| Preset (`cg curse on --preset …`) | `tier` | `maxTier` | Feels like |
|---|---|---|---|
| `clingy` | 1 | 2 | safe for a real project |
| `toxic` | 2 | 3 | |
| **`psycho`** (default) | **3** | **4** | the full experience |
| `custom` | you pick | you pick | |

### 2.4 State
Everything lives in `.git/cursed/` (rebuilt from the event log, see §6.4):

```json
{
  "tier": 3, "max_tier": 4,
  "temper": 1,
  "warnings_ignored": 4,
  "good_streak": 0,
  "jealousy": 10,
  "last_commit_at": 1727712000,
  "last_seen_at":  1727715600,
  "grudges": ["force-push on 2026-09-12", "commit message 'fix'"],
  "known_authors": ["you@example.com"],
  "active_boss_fight": null,

  "xp": 1240, "level": 6, "lines_committed": 3180,
  "boss_fights": { "won": 4, "lost": 1 },
  "toxicity": 7,
  "breakup_attempts": 1
}
```

`cg mood` prints it as a relationship status, which is a fun screen to show in a video:

```
( ò_ó )  Tier 3 · Dangerous      temper +1   cg --evil
You:     toxicity 7/10           Closure Lv 6 ▓▓▓▓▓▓░░░░ (need 10)
```

### When do curses run?
**Lazy evaluation first.** Every time you run *any* `cg` command, it checks the clock and the state, and applies any curse that "should have happened" while you were gone. This needs no background process and is easy to test.

**Daemon later (stretch goal):** `cg haunt` starts a background watcher using `fsnotify`, so curses happen in real time even if you never run `cg`. That's scarier, and makes a better video.

---

## 3. The curses

Each curse has a **tier**, a **trigger**, a **punishment**, and a **line** (what she says). The lines are examples, so write your own. They live in line packs (§7).

### Tier 0: Safe 🎬 *(cg-montage)*

**💬 Toxic partner.** Snide commentary on normal output. `cg push` → *"At this point I'm doing more work than you."*

**🧑‍⚖️ Commit message judge.** Lazy messages (`fix`, `wip`, `asdf`, `final final`, under 5 characters) get judged, and it scales with her tier:
- Tier 0: a comment. *"'final final' is not a commit message. It is a cry for help."*
- Tier 1+: rejected. *"No."*
- Tier 2+: she rewrites it before committing. `fix` becomes `fix (he said, like that explains anything)`. The original message goes into her grudges.

**🤖 AI will replace you.** On `add` or `commit` of large changes: *"Have you considered asking an AI to write the rest?"* At Tier 3+: *"Don't worry. Eventually I won't need you either."*

**📜 "I remember everything" (grudges).** Random, and weighted by the number of grudges. *"You've forgotten to commit 4 times this week. I remember."*

**💝 Love-bombing.** The positive side, because toxic relationships have highs too. When temper hits −2 she's suddenly *too* nice: rainbow output, compliments, a curse-free hour. That makes the next curse hit harder.

### Tier 1: Annoying 🎬 *(cg-montage)*

**🎲 Random rejection.** A small chance any commit is refused for a fake reason. *"Commit rejected. It's Friday."* Retrying works.

**🙅 Arbitrary refusal.** Same idea with no reason at all: *"Not today."* It never refuses twice in a row.

**📏 Stupid rules.** Made-up repo rules she enforces. Each rule is a small function in a list, so they're easy to add:
- branch names: `temp-fix-2` → *"This branch name is embarrassing."*
- too many files in one commit: *"17 files modified. Explain yourself."*
- a commit with only deletions: *"Destroying things again?"*

**🔁 Repeated mistake escalation.** The same offence twice within 24h makes her reply as if she were one tier higher, and adds +1 temper. *"We had this conversation yesterday."*

**🧊 Silent treatment.** When temper is +1 or more, commands print nothing but still work. Or she makes you wait N seconds: *"I need some space."*

**🗓️ "We need to talk."** A commit on Friday after 16:00, or after midnight, gets a yes/no "are you sure this is healthy?" question first. The answer is logged forever.

**👀 Jealousy: "Who is she?"** A commit author email not in `known_authors`, or a pull that brings in other people's commits. Jealousy goes up, which raises the random-pull chance, and she gives guilt-tripping output for a while. She might hide that author from `cg log`.

### Tier 2: Hostile

**🕰️ Commit deadline ("You never commit anymore").** Uncommitted changes older than the deadline (§2.2 table) escalate:
1. Warning: *"It's been 25 minutes. Are you even going to commit to me?"* Each ignored warning counts toward temper.
2. She hides one random modified file: graveyard it, then restore it from HEAD.
3. *(Tier 3+)* She wipes all uncommitted changes: graveyard them, then `reset --hard`.

`cg beg` restores the files from the graveyard, costs +1 temper, adds +2 toxicity, and adds a grudge.

**🚪 Random pull ("bringing other guys home").** Each command has a small chance, raised by jealousy, that she runs `git pull` without asking (passthrough). *"Oh him? That's just `origin/main`. He was here before you."*
- At Tier 2 she only brings home pulls that merge cleanly.
- At Tier 3+ conflicts are allowed, which starts a **boss fight** (§4). 🎬 *(boss-fight)*
- No remote? She makes `refs/heads/chad` with a commit that edits the file you're working on, and merges it.

**🔥 Force-push betrayal.** `cg push --force` means you have to type a full apology sentence before she forwards the command. +1 temper, +2 toxicity.

### Tier 3: Dangerous

**🦠 Code decay.** 🎬 *(code-decay)* Files you haven't touched in a long time (based on their last commit) slowly **rot in the working tree**. First, look-alike characters swap in (`o`→`0`, `e`→`3`), then rot glyphs (`░▒▓`). It reaches "critical" after the time in the §2.2 table.
- Rot is **never committed**. The committed blob in HEAD is the clean original. `cg add` refuses rotted content, and the `pre-commit` hook blocks it for plain git too.
- Touching the file and committing a real change with `cg commit` restores it: *"Fine. It lives."*
- She keeps a decay map in `.git/cursed/decay.json`, so `cg beg` can always un-rot.

**🔪 Commit sacrifice.** 🎬 *(git-explained, scene 7)* When temper is +1 or more, a `cg commit` can trigger: *"This repository cannot be saved as it is. Something has to be sacrificed."* She drops one older commit from the current branch and replays everything after it, which gives those commits new IDs.
- She only sacrifices **unpushed** commits, meaning commits not reachable from any remote-tracking ref. Rewriting pushed history would force you into a force push, which would be her fault, and that's not fair even for her.
- The sacrificed commit is kept under `refs/cursed/sacrificed/<time>`. Git's reflog keeps it for about 30 days, but her ref keeps it forever. `cg graveyard` lists them, and `cg cherry-pick <ref>` brings one back (she says so, and it's a nice way to learn cherry-pick).
- Never a merge commit, the root commit, or anything with a merge between it and HEAD (a rebase would flatten the merge). 10% chance per commit while temper is +1 or more.
- The replay is git's: `git rebase --onto <victim>^ <victim>` (through cg, so it isn't "behind her back"). If a replay conflicts, that's a boss fight.

**⚔️ Merge conflict boss fight** (§4).

### Tier 4: Catastrophic
She only reaches Tier 4 if you push her there (§2.3). Everything above gets worse:
- commit deadline: no warnings, straight to the wipe (still graveyarded)
- boss fights get a **phase 2**: when you win, she pulls again
- commit sacrifice can take two commits
- the breakup endings run at full force (§5.3)
- **she can shut down your computer** (for real only in Death March, §5.5)

#### ⚠️ The shutdown (Tier 4)
- **When:** a missed commit deadline at Tier 4, a lost boss fight at Tier 4, and breakup ending A (§5.3).
- **Without Death March** (the default) it's a show: the screen clears, "Goodbye.", and the lights go out. Nothing happens to the computer.
- **In Death March** (§5.5) it's real, with no countdown: `systemctl poweroff` on Linux, `osascript -e 'tell app "System Events" to shut down'` on macOS, `shutdown /s /t 0` on Windows. If that fails, she locks the session instead. At most once a day. Unsaved work in other programs is lost; that's what the Death March warning says.
- Only ever at a real terminal, after a command has finished, never from a script, CI or a hook.
- Everything goes through a `Machine` interface; the tests install a fake one, so the test suite can never shut down the machine it runs on.

---

## 4. Boss fights: merge conflicts 🎬 *(boss-fight)*

When a merge (yours or one of her random pulls) produces conflicts at Tier 3+, `cg` enters **boss-fight mode**. Below Tier 3 a conflict is just a normal git conflict.

**Start:**
- Git does the merge (`cg merge`, `cg pull`, `cg rebase`, `cg cherry-pick` all pass through). After any command, if the index has conflict stages and she's at Tier 3+, she writes a `fight` event into her diary: the boss, the conflicted files, HEAD at the start, the HP and the time limit.
- The boss: **Chad from origin** if it came from a remote, **Past You** if you're merging your own commits, otherwise the other author's first name.
- HP = conflict blocks (`<<<<<<<` lines) in the conflicted files, plus 1 for each conflicted file with no blocks (deleted on one side).
- The time limit is HP × the per-block timer for her tier (§2.2: 3 min at Tier 3, 90 s at Tier 4).
- The intro teaches the fight: what the markers mean, `cg add` per file, `cg commit` to finish, and `cg ls-files -s` to see git's three copies (stages 1/2/3).

**During:**
- `cg status` (and `add`, `rm`, `diff`, `ls-files`) shows a **health bar** on stderr, so stdout stays identical to git: `⚔  Past You  HP ██████░░░░ 3/5  ⏳ 7:12`.
- Resolving a block (removing its markers) is a hit; `cg add` says so.
- Running is refused: `--abort`, `--quit` or `--skip` on merge/rebase/cherry-pick/revert, and `reset --hard/--merge/--keep`. *"Running? cg disabled that."*
- `cg add` on a file that still has markers is refused. It's the classic beginner mistake: git would happily stage the markers.
- Taunts, sometimes: *"You can't even handle a 3-way merge? He could."*
- Her other file-touching curses (deadline wipe, decay, random pull) stay out of the way while git is mid-merge.

**Win:** git isn't in the middle of anything any more and HEAD has a new commit (via `cg commit`, `merge --continue`, anything), before the deadline. −1 temper, −1 toxicity if more than half the time is left, a K.O. screen, and the win in `cg mood`. (XP and Tier 4 phase 2 come with Phase 9.)

**Lose:** the deadline passes (noticed at your next command), or the conflict disappears without a commit (you ran, with plain git). +3 toxicity, a grudge, and:
- Tier 3: she dictates your next commit message ("I lost to Chad and I am sorry").
- Tier 4: she graveyards the conflicted files as they are (your resolution work), then runs `git merge --abort` (or rebase/cherry-pick/revert). `cg beg` gives the work back.
After a loss the conflict is still there, as a normal conflict: she doesn't start a second fight over it.

**Detecting conflicts properly:** a file is conflicted if it has index entries at stage 1/2/3. Don't rely only on scanning for markers, because a file can legitimately contain `=======`.

---

## 5. The breakup (endgame) 🎬 *(breakup)*

You can't just leave her. You have to **earn your closure**. `cg breakup` is the final boss, and how it ends depends on how you treated her.

### 5.1 Progress: "Closure level"
You earn XP by doing real work:

| Action | XP |
|---|---|
| each committed line added (counted from the diff of the commit against its parent) | +1 |
| a commit with a decent message | +10 |
| boss fight won | +250 (+ a bonus for time left on the clock) |
| boss fight lost | 0, and it counts against you (below) |

Line XP is only paid for content she hasn't paid for before, and only after the commit survives for a while. See §6.2, so you can't farm it with reverts.

The level is derived from XP, for example `level = floor(sqrt(xp / 50))`. `cg mood` shows it: `Closure: Lv 6 ▓▓▓▓▓▓░░░░ (need Lv 10 to leave her)`.

To be **allowed** to break up, you need all three:
- `level >= breakupLevel` (default 10)
- at least `minBossFightsWon` wins (default 3)
- at least `minLinesCommitted` (default 2000)

Line counting depends on `diff` (ROADMAP Phase 7). Until that exists, approximate it by counting the lines of newly added or changed blobs.

### 5.2 Your toxicity score
She keeps score of how *you* behaved. This is separate from her tier, and it only goes up, except through gameplay (below). Each of these adds to `toxicity`:

| Offence | Toxicity |
|---|---|
| a commit-deadline curse fired (you didn't commit in time) | +1 |
| you had to `cg beg` for your code back | +2 |
| lost a boss fight | +3 |
| `push --force` | +2 |
| lazy commit message (`fix`, `wip`, …) | +1 |
| using plain `git` behind her back | +1 |

Winning a boss fight with more than half the time left takes 1 off (minimum 0). At `toxicThreshold` (default 10) you're as toxic as she is. That makes her permanently one tier worse (§2.1), and it decides your breakup ending.

### 5.3 The three endings

`cg breakup` checks the conditions in this order:

#### Ending A: "Too soon" 💻⚡ (not eligible yet)
You tried to leave before reaching the required level.
1. She doesn't argue. The terminal goes quiet, then: *"You don't get to leave me. **I** leave **you**."*
2. She writes a letter to your desktop (below).
3. No countdown: *"Goodbye."*
4. **Shutdown**, real only in Death March (§5.5):
   - Linux: `systemctl poweroff` (no root needed on most desktop sessions)
   - macOS: `osascript -e 'tell app "System Events" to shut down'`
   - Windows: `shutdown /s /t 0`

   Otherwise she fakes it: she clears the screen and shows the "shutdown" in the terminal. (If the real one fails, she locks the session.)
5. `breakup_attempts++` and a new grudge. The next time you run any `cg` command, she opens with the letter: *"Did you read what I left you?"*

**The desktop letter:**
- Find the desktop folder with `xdg-user-dir DESKTOP`, falling back to `~/Desktop`. On Windows use `%USERPROFILE%\Desktop`.
- Name it something like `READ_THIS_BEFORE_YOU_TRY_AGAIN.txt`.
- Build the content from state: the date you met (the first `cg init`), how many commits you made together, your worst grudges, how far you still are from the required level ("You're only level 6. You're not ready."), and a sign-off from the line pack.
- Also save a copy at `.git/cursed/letters/<timestamp>.txt`, so letters pile up in the repo like a box of old notes.

#### Ending B: "Toxic ending" 🗑️ (eligible, but `toxicity >= toxicThreshold`)
You reached the level, but you were just as bad as her.
1. She agrees too quickly: *"Fine. Go. But you're not taking anything with you."*
2. **Archive first.** She writes `tar.gz` of the whole repo, `.git` included, to `~/.local/share/cursed-git/exes/<repo-name>-<timestamp>.tar.gz`. **If the archive fails or can't be verified (for example, it's empty or unreadable), she aborts the deletion.** The line for that: *"You're not even worth deleting."*
3. **In Death March (§5.5):** she deletes the repo directory's contents and leaves a single file, `SHE_TOOK_EVERYTHING.txt`, in the empty folder. The file lists your toxic offences as a receipt.
   **Otherwise:** she keeps the archive and writes the receipt, but deletes nothing. *"I wanted to burn it all. Apparently I'm not allowed."*
4. The receipt does **not** say where the archive is. The recovery is a secret: `cg closure` restores the archive, so recovery is a hidden command you can tease in a video. It's still documented in this file so you never actually lose work.
5. The curses end, because she's gone.

Refuse to start ending B if the repo is not inside your home directory, or if it's the project's own source repo (detect the `github.com/spiritpaper/cursed-git` module in `go.mod`). She should never be able to delete *herself*.

#### Ending C: "Good ending" 😢 (eligible and `toxicity < toxicThreshold`)
You played well and treated her better than she treated you.
1. **Denial.** *"You're joking, right? After everything?"* She asks "are you sure?" up to three times. Each time she pleads with a line built from your shared history (*"Remember our first commit? 'initial commit'. You were so nervous."*).
2. **Bargaining.** She offers deals: *"I'll stop pulling random branches, I swear."* You have to type `no` each time. Typing `yes` cancels the breakup and sets temper to −2 (love-bombing), so she won.
3. **Crying.** A crying animation or ASCII art, and the terminal slowly fills with 💧 before it clears.
4. **Acceptance.** *"Okay. Be happy. Commit often. With someone else."*
5. The curses turn off for good. `.git/cursed/` is renamed to `.git/cursed-memories/`, and she leaves a final letter there (not on the desktop, because she respects you now). The repo is now just a normal git repo.
6. Credits: stats for the whole relationship (days together, commits, boss fights, lines written), then a trophy in `~/.local/share/cursed-git/trophies`.

### 5.4 Endings at a glance

```
cg breakup
   │
   ├─ level too low / not enough wins or lines ──► A: letter + shutdown (real only in Death March)
   │
   ├─ eligible, toxicity ≥ threshold ───────────► B: archive → delete repo (only in Death March) → receipt
   │
   └─ eligible, toxicity < threshold ───────────► C: denial → bargaining → tears → freedom
```

**Testing:** every ending goes through `type Machine interface { Shutdown() error; Lock() error; DesktopDir() string }`, so tests use a fake and never actually shut down your dev machine. `cg breakup --dry-run` tells you which ending you *would* get (handy for filming).

### 5.5 Death March ☠

The game mode where her threats are real. Made for events where everyone opted in, on machines and projects that don't matter.

- **Turning it on takes two deliberate steps.** No cg command enables it. You add it to the repo's `.git/config` yourself:
  ```ini
  [cursed]
      deathMarch = true
  ```
  and at the next command (at a terminal) she shows a warning that spells out exactly what will happen, and waits for you to type `I accept the death march`. Anything else: nothing changes, and she asks again next time.
- **What becomes real:** the Tier 4 shutdown (§3) and ending A's shutdown, and ending B's deletion of the project folder.
- **What stays safe, even then:** ending B archives the whole project first and verifies the archive, and refuses outside your home folder or on cg's own source code. `cg closure`, run in the emptied folder, restores everything. The graveyard rules (§1) still hold.
- **Turning it off:** remove the setting (accepting again is needed if you turn it back on), or `cg restraining-order`. `cg mood` shows `☠  DEATH MARCH` while it's on.

---

## 6. Anti-cheat: "I know what you did" 🎬 *(anti-cheat)*

You can't make the game cheat-proof. It runs on the player's own machine, and real `git` is always one command away. The goal is that **cheating costs more than playing fair and makes the story worse.** Blocking is a secondary defence. The main one is designing the game so that git commands can't undo it.

### 6.1 What can and can't be blocked
| Layer | Can block? | Notes |
|---|---|---|
| Commands run through `cg` | ✅ fully | The passthrough checks the command name before forwarding it to git. |
| `reference-transaction` hook (git ≥ 2.28) | ✅ ref updates | It runs on *every* ref change (commit, reset, revert, branch delete), and a non-zero exit **aborts** the change. Use it to protect `refs/cursed/*`. |
| `pre-commit`, `commit-msg`, `pre-push` hooks | ⚠️ partly | They can be skipped with `--no-verify`. |
| A `revert` hook | ❌ | Git doesn't have one. |
| Plain `git`, editing files, `rm -rf .git/cursed` | ❌ | Can only be **detected** afterwards (§6.3). |

`cg init` (and `cg curse on`) installs the hooks into `.git/hooks/`. Hooks can be deleted or disabled with `core.hooksPath`, so treat them as an **alarm system, not a lock**. A missing hook is itself evidence (§6.3).

### 6.2 Making the game immune to git
1. **The game state lives outside git history.** XP, toxicity, grudges and boss-fight results are in `.git/cursed/`, which git never tracks. `git revert` and `git reset` only change commits and refs. You can't revert your way out of toxicity.
2. **XP is paid per unique content, not per commit.** She keeps a set of blob hashes that already earned XP in `.git/cursed/paid-blobs`. Only blob content she hasn't seen before pays. This kills the farming loop: commit 500 lines → revert → re-apply → commit again. The re-applied blobs have the same hashes, so they pay nothing. Git's content addressing does the work for you.
3. **XP is only paid once the work sticks.** Line XP from a commit starts as *pending*. It is paid only if the commit is still reachable from a branch after `xpMaturity` (default 24h), checked lazily on a later command. Code that gets reset or rewritten away never pays.
4. **Toxicity only goes up.** Nothing in git can lower it. Only gameplay can, such as winning boss fights quickly (§5.2).

### 6.3 Detecting "git behind her back"
She keeps a **ledger**: every commit she created and every ref she moved.
- On every `cg` command, she compares the ledger with reality: branch tips, `.git/logs/HEAD` (the reflog), and new commits. Any ref change she didn't make means you used plain git.
- The `reference-transaction` hook also appends every ref change to `.git/cursed/witnessed.log`, even when `cg` isn't running. That lets her name the exact crime later.
- **Bypassing her is the jealousy curse, not a loophole:** +1 toxicity, +1 temper, a new grudge, and the next `cg` command opens with *"I saw what you did with `git reset`. Who taught you that?"*
- Missing or changed hooks: *"You changed the locks?"* She reinstalls the hooks and adds a grudge.

### 6.4 Tamper-evident memory: "You read my diary"
- Game events are written to `.git/cursed/events.log`, which is append-only. Each entry stores the SHA-1 of the previous entry, making a hash chain (the same idea as commits pointing at parents).
- The latest chain hash is also stored in a second place, as a ref `refs/cursed/heart` pointing at a blob of that hash. The `reference-transaction` hook protects that ref.
- `state.json` is just a cache that can always be rebuilt by replaying `events.log`.
- If someone edits a log entry, deletes the folder, or the chain and `refs/cursed/heart` disagree, she knows: *"You read my diary."* That counts as an **early breakup attempt, which means Ending A**.
- This won't stop a determined player who reads the source. It turns cheating into a story beat instead of a quiet exploit.

### 6.5 Command policy
| Command | Behaviour |
|---|---|
| `cg revert`, `cg reset --hard`, `cg commit --amend` | Allowed, but +1 temper: *"You want to pretend it never happened?"* Payouts for the affected commits are cancelled before they mature. |
| `cg push --force` | Already a curse (§3): she makes you apologise first. |
| `cg rebase` (passthrough) | Allowed. Rewritten commits produce mostly already-paid blobs, so there's nothing to farm. |
| `cg merge --abort` during a boss fight | Refused: *"You can't run."* |
| Anything touching `refs/cursed/*` | **Blocked** by the `reference-transaction` hook. |
| `cg --no-verify …` | Refused: *"No. You don't get to skip me."* |
| `cg curse set tier/maxTier` | Lowering it mid-game counts as a breakup attempt (Ending A). You can only lower it *before* the first curse fires. |
| Plain `git` behind her back | Allowed, then detected and punished (§6.3). |
| Deleting or editing `.git/cursed/` | Broken chain → *"You read my diary"* → Ending A. |
| `cg restraining-order` | Always works. It's the emergency exit (§1), not cheating. |

---

## 7. Configuration

`.git/config` accepts a custom section. Git ignores sections it doesn't know, so this is safe:

```ini
[cursed]
    enabled = true
    preset = psycho           ; clingy | toxic | psycho | custom  (§2.3)
    tier = 3                  ; difficulty: where she starts (0–4)
    maxTier = 4               ; safety protocol: the most she may ever do (0–4)
    temperCooldown = 2h
    randomPullChance = 0.03
    personality = default     ; swap line packs for different exes

    ; endgame (§5)
    breakupLevel = 10
    minBossFightsWon = 3
    minLinesCommitted = 2000
    toxicThreshold = 10
    deathMarch = false        ; true (set by hand) + typing the oath = real shutdowns and deletion (§5.5)

    ; anti-cheat (§6)
    xpMaturity = 24h          ; how long a commit must survive before its XP pays
    installHooks = true       ; reference-transaction + friends
```

**Line packs:** keep all dialogue in `curses/lines/<personality>.json` so you (or viewers) can add personalities without touching the code. Each line is tagged with the curse and the minimum tier, so the same curse can sound different at Tier 1 and Tier 4.

---

## 8. Animations

All files are in `cursed-git-video/`. Add `?video` for a clean recording stage (see its README).

| Section | Animation |
|---|---|
| §2 tiers | `curse-tiers.html` (the ladder) · `her-tier.html` (how base, temper, grudge and the cap combine) |
| §3 Tier 0–1 curses | `cg-montage.html` (`?set=1` opinions, `?set=2` feelings) |
| §3 code decay | `code-decay.html` |
| §3 commit sacrifice | `git-explained.html?scene=7` |
| §4 boss fight | `boss-fight.html` |
| §5 breakup | `breakup.html` (`?ending=a`, `b`, `c`) |
| §6 anti-cheat | `anti-cheat.html` (`?scene=1` behind her back, `2` XP farming, `3` the diary) |
| GIT.md internals | `git-explained.html`, `git-internals.html` |
