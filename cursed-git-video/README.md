# Cursed Git: video animations

Nine animations, one look. Open any file in a browser to watch and click through it.
Add `?video` to the address for a clean full-screen stage with no buttons (that's what you record).

| File | What it is | Length of one loop | Useful options |
|---|---|---|---|
| `git-explained.html` | How Git works, 7 scenes: snapshots, branches, merge, rebase, pull, conflicts, **Commit Sacrifice** curse | 15–25 s per scene | `?scene=1` … `?scene=7` (one scene, looping) |
| `cg-montage.html` | cg answers 11 commands with curses; face and patience get worse; ends on "You were warned." | ~75 s | `?start=7` (start at line 7) |
| `curse-tiers.html` | Escalation ladder, Tier 0 Safe → Tier 4 Catastrophic | 13 s full climb, ~6 s per card | `?tier=0` … `?tier=4` (chapter card) |
| `code-decay.html` | parser.go rots over 30 days, then a commit restores it | ~37 s | |
| `boss-fight.html` | Random Pull → Merge Conflict boss fight (pixel RPG) | ~45 s | |
| `git-internals.html` | Inside cg: what each command does in `.git`, 13 scenes (see below) | 23–37 s per scene | `?scene=1` … `?scene=13`, `?noloop` (stop after one scene) |
| `her-tier.html` | How her live tier is computed: base + temper + grudge, clamped by the safety cap (CURSES.md §2) | ~59 s, 5 chapters of 8–17 s | `?chapter=1` … `?chapter=5` |
| `breakup.html` | `cg breakup`: the closure check, then Ending A (letter + shutdown), B (repo deleted + receipt) or C (tears + credits) | A ~47 s · B ~31 s · C ~53 s | `?ending=a`, `b`, `c` |
| `anti-cheat.html` | "I know what you did": plain git behind her back, XP farming with reverts, editing her diary | ~21 s · ~28 s · ~21 s | `?scene=1` … `?scene=3`, `?noloop` |

`cg-montage.html` has two line sets: `?set=1` "has opinions" (the original, ~75 s) and `?set=2` "has feelings" (jealousy, silent treatment, deadlines, love-bombing, ~66 s).

All animations are 16:9.

## Record them as MP4

One-time setup (Node 18+ and ffmpeg installed):

```bash
npm install
npm run setup
```

Then, for example:

```bash
node tools/record.mjs git-explained.html --seconds 22 --query "scene=2" --out branches.mp4
node tools/record.mjs git-explained.html --seconds 26 --query "scene=7" --out sacrifice.mp4
node tools/record.mjs cg-montage.html    --seconds 78 --out montage.mp4
node tools/record.mjs curse-tiers.html   --seconds 7  --query "tier=3" --out tier3.mp4
node tools/record.mjs curse-tiers.html   --seconds 14 --out tiers-full.mp4
node tools/record.mjs code-decay.html    --seconds 37 --out decay.mp4
node tools/record.mjs boss-fight.html    --seconds 46 --out boss.mp4
node tools/record.mjs cg-montage.html    --seconds 67 --query "set=2" --out feelings.mp4
node tools/record.mjs her-tier.html      --seconds 59 --out her-tier.mp4
node tools/record.mjs breakup.html       --seconds 48 --query "ending=a" --out ending-a.mp4
node tools/record.mjs breakup.html       --seconds 32 --query "ending=b" --out ending-b.mp4
node tools/record.mjs breakup.html       --seconds 54 --query "ending=c" --out ending-c.mp4
node tools/record.mjs anti-cheat.html    --seconds 29 --query "scene=2&noloop" --out xp-farming.mp4
```

**Ubuntu 26.04 (or any system where `npm run setup` says Playwright has no Chromium build):** use the system Chromium instead:

```bash
sudo snap install chromium            # if you don't have it
CHROMIUM_PATH=/snap/bin/chromium node tools/record.mjs her-tier.html --seconds 59
```

Add `--scale 2` for 4K renders (sharper text after YouTube compression), and `--fps 60` for extra smooth motion.
Record while online so the fonts load.

## Inside cg: scene list

Every hash, size and byte in these scenes comes from a real git 2.43 repository, so they match what your own cg should produce for the same files.

| # | Scene | Length | Record |
|---|---|---|---|
| 1 | `cg init` | 30 s | `node tools/record.mjs git-internals.html --query "scene=1&noloop" --seconds 31 --out 01-init.mp4` |
| 2 | `hash-object` | 37 s | `--query "scene=2&noloop" --seconds 38` |
| 3 | `cat-file` | 27 s | `--query "scene=3&noloop" --seconds 28` |
| 4 | `update-index` · `ls-files` | 34 s | `--query "scene=4&noloop" --seconds 35` |
| 5 | `write-tree` · `ls-tree` | 36 s | `--query "scene=5&noloop" --seconds 37` |
| 6 | `commit-tree` | 35 s | `--query "scene=6&noloop" --seconds 36` |
| 7 | `update-ref` · `symbolic-ref` · `rev-parse` | 28 s | `--query "scene=7&noloop" --seconds 29` |
| 8 | `cg add` | 29 s | `--query "scene=8&noloop" --seconds 30` |
| 9 | `cg commit` | 28 s | `--query "scene=9&noloop" --seconds 29` |
| 10 | `cg log` | 23 s | `--query "scene=10&noloop" --seconds 24` |
| 11 | `cg status` | 28 s | `--query "scene=11&noloop" --seconds 29` |
| 12 | `cg push` | 33 s | `--query "scene=12&noloop" --seconds 34` |
| 13 | `cg pull` | 34 s | `--query "scene=13&noloop" --seconds 35` |

The demo repo: `README.md` = `# cursed-git` and `src/cg.go` = `package main`, author `dev <dev@cursed.git>`, timestamp `1759500000 +0330`.
Captions and timing live in each scene's `run` function inside `git-internals.html`.

## Editing the content

- **Montage jokes:** the `LINES` list at the top of the script in `cg-montage.html`. Each line has `cmd`, `reply`, `mood` (0–4) and `curse`.
- **Tier names and examples:** the `TIERS` list in `curse-tiers.html` and `her-tier.html` (keep them in sync with docs/CURSES.md §2).
- **Her tier story:** the `CHAPTERS` list in `her-tier.html`. Each beat changes the state (`d`), logs a line and lets her speak.
- **Breakup stats, letter and receipt:** `PLAYERS`, `LETTER` and `RECEIPT` at the top of the script in `breakup.html`.
- **Git scenes:** each scene in `git-explained.html` is a short script of `cmd(...)`, `addCommit(...)`, `setBranch(...)` and `beat(...)` calls.
- **cg's faces:** `( ^_^ )  ( -_- )  ( ¬_¬ )  ( ò_ó )  (╬ ò_ó)`, one per tier 0–4. They're the same in every file, so cg stays one character across the video. Special faces: `( ♥_♥ )` love-bombing, `( °_° )` shock, `( ;_; )` pleading, `( ╥_╥ )` crying.
