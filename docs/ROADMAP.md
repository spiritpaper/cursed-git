# Cursed GIT — Technical Roadmap

A learn-by-building path. Each phase ends with something you can **verify against real git** and **show on camera**. The phases are ordered so that each one only uses things built in earlier phases.

**The golden testing trick:** do the same operation with `git` in one temp dir and with `cg` in another, using fixed dates (`GIT_AUTHOR_DATE`, `GIT_COMMITTER_DATE`). The hashes must be **identical**. Automate this with Go tests using `t.TempDir()`.

**Never test `cg` inside this project's own folder.** Use a scratch repo like `/tmp/playground`, or you'll corrupt the repo that holds your code (Phase 0 explains how that already happened).

---

## Phase 0: Fix the foundation ✅ *(Episode 1: "She broke my repo before I even wrote her")*

What's wrong right now:

1. **It doesn't compile.** `main.go` imports `cursedgit.com/core`, but there are no `.go` files in `core/`. They're in `core/commands/` and `core/util/`, and both declare `package core`. **In Go, one directory is one package**, and the import path is the directory path. Suggested layout:
   ```
   main.go                       // just calls cli.Run(os.Args[1:])
   internal/cli/                 // command dispatch + passthrough
   internal/repo/                // find .git, paths, config
   internal/object/              // blob/tree/commit read+write, hashing
   internal/index/               // .git/index read+write
   internal/refs/                // HEAD, branches, packed-refs
   internal/commands/            // init, add, commit, status, log…
   internal/curses/              // state, triggers, punishments, lines
   ```
   Then imports look like `cursedgit.com/internal/object`.
2. **`hashObjects.go` / `catFile.go`** have no return statements, and `hex` is imported but unused. Both are compile errors.
3. **`add` destroys the index.** It writes the raw file contents into `.git/index`. That's why this repo's real `git` says `index file corrupt`: `.git/index` is literally a copy of `main.go`. To fix this repo, delete `.git/index` (you have no commits, so nothing is lost), then run `git status`.
4. `IndexEntry.StatData [62]byte` is wrong. The stat data is 40 bytes, see GIT.md §4.
5. `github.com/gorilla/mux` is an HTTP router you don't use. Run `go mod tidy`.
6. Build the binary as `cg` (`go build -o cg .`). The old name `gc` clashed with Graphviz and with `git gc`, so `main.go`'s `<gc help>` messages should say `cg` too.
7. `init`: real git re-running `init` is harmless ("Reinitialized…"). Keep your insult, but maybe also add `refs/heads` checks and `cursed/` creation there.

✅ **Done when:** `go build` works, and `cg init` in `/tmp/playground` followed by `git status` shows no errors.

**Status (2026-10-17): done.** `internal/cli` + `internal/commands`, `go vet` clean, tests compare against real git (`go test ./...`). Lessons are in `DEVNOTE.md`.

---

## Phase 1: Objects ✅ *(Ep. 2: "Everything in git is a hash")*
- `hash-object [-w]`: header + SHA-1 + zlib + write to `objects/xx/…`
- `cat-file -t|-s|-p`: read + zlib decompress + parse the header
- **Verify:** `cg hash-object -w f && git cat-file -p <hash>` works, and the hash matches `git hash-object f`.
- **Learn:** `crypto/sha1`, `compress/zlib`, `io.Reader`, `[]byte` vs `string`.

**Status (2026-10-17): done.** `internal/object` (hash, write, read, short names, trees) and `internal/repo.Find`. Tests check the git-internals demo hashes and read/write both ways with real git. Packed objects aren't read yet (Phase 5 fallback).

## Phase 2: The index ✅ *(Ep. 3: "Reverse-engineering a binary file")*
- Parse `.git/index` (create one with real git first and study it with `xxd`)
- Write it back: `DIRC` header, sorted entries, padding, SHA-1 trailer, `index.lock` + rename
- `cg add <paths>` (files + directories, recursive) and `cg ls-files --stage`
- **Verify:** `cg add` then `git ls-files --stage` shows correct entries, and `git status` agrees.
- **Learn:** `encoding/binary`, `bytes.Buffer`, `os.Lstat` + `syscall.Stat_t`, `filepath.WalkDir`.

**Status (2026-10-17): done.** `internal/index` (v2/v3 read, v2 write, extensions skipped, `index.lock`), `internal/ignore` (`.gitignore` + `.git/info/exclude`), `cg add` and `cg ls-files`. Re-encoding a git-made index is byte-identical, and `cg add .` on a copy of this project stages exactly what `git add .` stages. Next up: Phase 3, and then cg starts tracking itself.

## Phase 3: Trees and commits ✅ *(Ep. 4: "My first commit that real git can read")*
- `write-tree`: group index entries by directory, build **bottom-up**, sort names git's way
- `commit-tree`, reading user name/email from config
- Refs: resolve `HEAD` (symbolic, detached, unborn), `update-ref` with a lock file
- `cg commit -m "…"` = write-tree + commit-tree + move the branch
- Also append to `.git/logs/HEAD` if you want `git reflog` to see it
- **Verify:** `cg commit` then `git log --stat` and `git fsck` are clean. With fixed dates, the commit hash matches git's.

**Status (2026-10-17): done.** `write-tree`, `commit-tree`, `update-ref`, `symbolic-ref`, `rev-parse` and `commit -m`, with ref locking, `packed-refs` and reflogs. cg reproduces the exact commit hashes from `git-internals.html` (`fe16bf5`, `dd70c3f`), and git and cg can take turns committing on the same branch. **From here on, cg tracks itself.**

## Phase 4: Reading history ✅ *(Ep. 5)*
- `cg log`: follow `parent` from HEAD, formatted like git
- `rev-parse`: `HEAD`, `HEAD~2`, branch names, short hashes
- `packed-refs` support

**Status (2026-10-17): done.** `cg log` (default and `--oneline`, `-n`), revisions like `HEAD~2`, `main^`, `HEAD~1^2`, `@`, and packed objects read through `git cat-file --batch` (so cg keeps working after `git gc` or a pull). Output is byte-identical to `git log --no-decorate`. Branch decorations like `(HEAD -> main)` aren't printed yet.

## Phase 5: Status + passthrough ✅ *(Ep. 6: "Git does the hard parts for me")*
- `status`: HEAD tree ↔ index ↔ working tree, stat-data shortcut, basic `.gitignore`
- **Passthrough** for every unknown command (GIT.md §5), with the exit code propagated
- The pack fallback: if a loose object is missing, ask `git cat-file`
- **Verify:** `cg pull` / `cg push` work via git, and `cg log` still works after a pull.

**Status (2026-10-17): done.** `cg status` (long, `-s`, `--porcelain`, `-b`, ahead/behind) matches git's output, with `cg` in the hints. Every command cg doesn't have, and every option it doesn't support, runs through real git with the same terminal and exit code. Packed objects are read through one long-running `git cat-file --batch`. Rename detection in status isn't done yet.

## Phase 6: Curses v1 ✅ *(Ep. 7: "I gave my git trust issues")*
Do this now instead of at the end. It's the fun part, and it keeps the project going.
- `.git/cursed/state.json` load/save, `cg mood`, `cg breakup`
- A hook system in the CLI: `beforeCommand(cmd)` / `afterCommand(cmd, result)` that every command (including passthrough) goes through
- **Graveyard first**: snapshot the working tree into objects + `refs/cursed/graveyard/<ts>`, and `cg beg` to restore. Build the safety net *before* the knife.
- **The tier engine** (CURSES.md §2): `live = clamp(base + temper + grudge, floor, maxTier)`, the temper events and cooldown, the presets (default `psycho`: start at 3, cap at 4), and `cg mood`. Write table tests for the formula first; it's pure math and the easiest part to get right.
- Every curse declares its minimum tier, so the cap works as the safety protocol automatically
- Curses, Tier 0–2: toxic partner, commit message judge, AI will replace you, grudges, random rejection, arbitrary refusal, stupid rules, repeated mistakes, silent treatment, jealousy, commit deadline (warn + hide one file)
- Tier 3, first one: **code decay**. It only needs the working tree and HEAD, so it doesn't have to wait for merges.
- **Anti-cheat basics** (CURSES.md §6): the event log with a hash chain plus `refs/cursed/heart`, the ledger of what she did, comparing the ledger with refs and the reflog on every command, and the command policy in the passthrough
- Install the `reference-transaction` hook from `cg init` to protect `refs/cursed/*` and log every ref change. **Verify:** `git update-ref -d refs/cursed/heart` is refused, and a plain `git commit` is detected on the next `cg` command.
- Inject the clock and the RNG (`type Clock interface{ Now() time.Time }`, `rand.New(seed)`) so curses are **testable**. You don't want to wait 30 minutes in a unit test.

**Status (2026-10-17): done.** The diary (hash-chained `events.log` + `refs/cursed/heart`, state rebuilt by replaying it), the tier engine and presets, hooks around every command (including passthrough), the graveyard + `cg beg`, `cg mood`, `cg curse on/off`, `cg restraining-order`, and these curses: commit message judge, random rejection, arbitrary refusal, stupid branch names, too many files, only deletions, AI will replace you, toxic partner, grudges, repeated mistakes, silent treatment, we need to talk, jealousy, love-bombing, force-push apology, commit deadline (warn → hide one file → wipe, always into the graveyard). She only acts at a real terminal; scripts, CI and tests get plain git.

### Phase 6b ✅ (2026-10-17)
Behind-her-back detection from the reflog, her git hooks (`reference-transaction` + `pre-commit`, never overwriting yours), code decay (rot with ░▒▓ only, restored by any commit, can't be committed), random pull (`--ff-only`, 20 s timeout, never asks for passwords), and the "alive" layer: absence greetings, callbacks to old commit messages, your name, your favourite file, late-night talks, dramatic pauses. Two hard limits for her writing: she lies about her feelings, never about your data; dark, not cruel.

Still open: lowering `tier`/`maxTier` mid-game = breakup attempt (needs Phase 9), and Tier 3+ random pulls that merge (done in Phase 8).

## Phase 7: Branches and checkout ✅ *(Ep. 8)*
- `branch`, `switch`/`checkout`: write the target tree to the working dir, rebuild the index, and refuse if there are uncommitted changes that would be lost
- `restore`, `reset` (soft/mixed/hard)
- `diff`: implement the **Myers diff algorithm**, which is a great episode by itself

**Status (2026-10-17): done.** `branch` (list, create, `-d`/`-D`), `switch`, `checkout` (branch, `-b`, commit, `-- <paths>`), `reset` (soft/mixed/hard, paths), `restore` (`--staged`, `--source`), and `diff` (worktree, `--cached`, one or two commits) with a Myers diff and git's change compaction. Tested with twin repos: git does an operation in one, cg in the other, and every file, mode, index entry, ref, reflog line and output must match. Known difference: on ambiguous inputs git's diff can pick a different (equally short) alignment, mostly because of its indent heuristic; cg's patches are always correct.

## Phase 8: Merge and the boss fight *(Ep. 9–10: "Boss fight: merge conflicts")*
**Merging is git's job.** cg is a game that sits on top of a real project, not a git rewrite. `cg merge`, `cg pull`, `cg rebase` and `cg cherry-pick` pass straight through to git (GIT.md §5). What cg needs is to *understand* a conflict that git left behind:
- read the conflict stages 1/2/3 from the index, and `MERGE_HEAD`
- `cg status` during a merge: "You have unmerged paths", "both added", "deleted by us", ...
- `cg commit` finishes a merge: a commit with two parents (`MERGE_HEAD`), then clean up the merge files
- **Verify:** start a merge with `cg merge` (git), finish it with `cg commit`, and compare with a twin repo where git does both.
- Then the **boss fight**: timer, HP bar, taunts, win/lose tables, random pulls that trigger it
- **Commit sacrifice**: drop an unpushed commit and let `git rebase` replay the rest. A conflicting replay is a boss fight too.

**Status (2026-10-17): boss fights done.** Merging goes through git; cg understands what git leaves behind (`status` during a merge, `commit` with `MERGE_HEAD`). At Tier 3+ any conflict becomes a fight (`internal/curses/boss.go`): Chad / Past You / the author, HP = conflict blocks, a clock per block, an HP bar, no running, no staging markers, K.O. screen on the winning commit, dictated commit message (Tier 3) or an abort into the graveyard (Tier 4) on a loss. Her random pull merges for real at Tier 3+. Commit sacrifice is done too: an unpushed commit goes to `refs/cursed/sacrificed/`, `git rebase --onto` replays the rest, a conflicting replay is a boss fight, and `cg cherry-pick` brings it back. Still open: Tier 4 phase 2 and XP for wins (Phase 9).

## Phase 9: The breakup endgame *(Ep. 11: "I tried to break up with my git")*
- XP, levels and the toxicity score (CURSES.md §5.1–5.2), shown in `cg mood`
- XP paid per unique blob, with `xpMaturity` (CURSES.md §6.2). **Verify:** commit → revert → re-commit earns XP only once.
- Line counting from commit diffs, reusing the Phase 7 diff
- `cg breakup` with the three endings, plus `--dry-run`
- Ending A: the desktop letter (`xdg-user-dir`), OS-specific shutdown behind a `Machine` interface, and the fake-shutdown fallback
- **Tier 4 shutdown** (CURSES.md §3 Tier 4): real only in Death March (CURSES.md §5.5), no countdown, once a day at most, fake shutdown otherwise, `Machine` interface for tests
- Ending B: `archive/tar` + `compress/gzip`, verify the archive, *then* delete; `cg closure` restores
- Ending C: the dialogue state machine (denial → bargaining → crying → acceptance) and the credits
- `cg restraining-order`, the no-questions-asked emergency off switch
- **Verify:** fake `Machine` in tests; run ending B on a throwaway repo and restore it with `cg closure`, then run `git fsck` on the result.
- **Learn:** `archive/tar`, `runtime.GOOS`, `os/exec`, and writing terminal animations with ANSI escape codes.

**Status (2026-10-17): done.** XP (per unique blob, pending until it survives `xpMaturity`, so reverts can't farm it), closure levels in `cg mood`, `cg breakup` with all three endings and `--dry-run`, the desktop letter (and a copy in `.git/cursed/letters/`), the receipt, `cg closure`, the credits and the trophy. The destructive parts follow the audience: cg is a game for opt-in events (hackathons, conventions), so real shutdowns and real deletion only happen in **Death March** (CURSES.md §5.5), turned on by hand in `.git/config` and accepted by typing an oath. Without it: an animation and a receipt. Tests run with a fake `Machine` and an isolated data folder. Tier 4 boss fights get phase 2. Still open: personality packs (`personality =` in config), and the gentle mode.

## Phase 10: Launch *(final episode)*
- Commit the animations (`cursed-git-video/`, `code-animations-kit-v2/`) by removing them from `.gitignore`
- A proper README: a GIF hero from `cg-montage.html`, the tier ladder, the install steps, and links to the episodes
- Render the animation set and embed the clips in the README and the docs

## Phase 11: Made for events *(the hackathon update)*
cg is played at opt-in events (hackathons, conventions, coding challenges). These make it a better game there. One step at a time, each with tests and its own commit:

1. ✅ **Achievements.** Unlocked from her diary, shown on `cg card` and in `cg achievements`: First Blood, Speedrun, Ghosted, Rebase Survivor, Serial Dater, and more. Pure replay of events, so they can't be faked without breaking the diary. *(Done 2026-10-18: 21 achievements, announced when they unlock, `internal/curses/achievements.go`.)*
2. **Quests that teach git.** She sets a task ("make a branch and merge it back", "cherry-pick something", "resolve a conflict without `--abort`"), checks it against the real repository, and pays XP. The learning goal, as gameplay.
3. **A `hackathon` preset.** Clocks and breakup thresholds scaled to an event's length (`cursed.eventHours`), so a 24h event can actually reach an ending.
4. **Event leaderboard.** `cg card --json`, and a page organizers can put on a projector: every team's card, fastest boss kill, most toxic team.
5. **Rivalry.** Two people committing to the same repo: she plays them against each other.

## Phase 12: Beyond the toxic ex
1. **Personality packs.** `cursed.personality = <name>` picks `lines/<name>.json`; the default stays her.
2. **The gentle mode.** A partner (girlfriend or boyfriend) who isn't toxic: no curses, no punishments, just git with a new look and someone cheering you on.
3. **Standing alone.** cg works without git installed (packfile reader, then fetch/push/merge of its own), and offers to install official git for the commands it doesn't have yet.
4. **Launch (Phase 10).** Commit the animations, a GIF hero in the README, the rendered clips.

## Stretch goals
- `cg haunt` daemon (fsnotify) for real-time curses
- TUI for boss fights (Bubble Tea: `github.com/charmbracelet/bubbletea`)

---

## Recommended references
- *Pro Git*, chapter 10 "Git Internals": free at git-scm.com/book
- `Documentation/gitformat-index.txt` in the git source tree: the exact index spec
- "Write yourself a Git!" (wyag.thb.lt): the same journey in Python, useful for comparison
- James Coglan, *Building Git*: the deepest treatment, including Myers diff and merges

## Video tips
- Record the **failures**: wrong hashes, the corrupted index, git refusing your trees. Those are the most relatable moments.
- End each episode with a real-git verification on screen (`git fsck` passing is a great payoff shot).
- Keep a `DEVLOG.md` with dates, bugs and "aha" moments. It becomes your script.
