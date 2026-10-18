# Cursed GIT

**Like git, but she has feelings.**

`cg` is a game you play on top of a real project. It works like git (most commands are
git, byte for byte), but it comes with *her*: a toxic ex who lives in your repository. She
judges your commit messages, gets jealous when someone else commits, punishes you for not
committing, and turns every merge conflict into a boss fight.

You learn git by surviving her.

```
          _.-''''''''''-._
       .-'  .-''''''''-.  '-.
     .'   .'/\/\/\/\/\/\'.   '.
    /   .' |    \  /    | '.   \
   /   /   |    o  o    |   \   \        $ cg commit -m "fix"
   |   |    \   /^^\   /    |   |        ( ò_ó )  I fixed your commit message. You're welcome.
   |   |     '.______.'     |   |                 → "fix (he said, like that explains anything)"
   |   |      _|    |_      |   |
  /|   |    .' '----' '.    |   |\
 / |   |   /     <>     \   |   | \
```

## Install

You need [Go](https://go.dev/dl/) 1.26 or newer and [git](https://git-scm.com/downloads).

```bash
go install github.com/spiritpaper/cursed-git/cmd/cg@latest
```

That puts `cg` in `$(go env GOPATH)/bin`. Make sure that folder is on your `PATH`.

From source:

```bash
git clone https://github.com/spiritpaper/cursed-git.git
cd cursed-git
go build -o cg ./cmd/cg
```

## Quick start

A new project:

```bash
cg init          # a normal git repository, with her in it
```

A project you already have (any git repository):

```bash
cd my-project
cg curse on --preset clingy    # start gentle. Really.
```

Then use `cg` wherever you'd type `git`: `cg add`, `cg commit -m`, `cg status`, `cg log`,
`cg switch`, `cg push`, `cg pull`... Anything cg doesn't do itself goes straight to real git,
so you never have to switch back.

```bash
cg mood          # how she feels about you right now
```

## How dangerous is she?

One dial: her **tier**. The preset decides where she starts and how high she can ever go.

| Tier | | What she does |
|---|---|---|
| 0 Safe | `( ^_^ )` | Personality. Comments on your commits, never blocks you. |
| 1 Annoying | `( -_- )` | Refuses lazy commit messages, gives you the silent treatment, gets jealous. Never touches files. |
| 2 Hostile | `( ¬_¬ )` | Commit deadlines (she hides a file if you ignore her), random `git pull`s. |
| 3 Dangerous | `( ò_ó )` | Wipes uncommitted work after 30 minutes, lets old code rot, merge conflicts become boss fights. |
| 4 Catastrophic | `(╬ ò_ó)` | Everything above, faster and with no warnings. Only if you push her there. |

| Preset | Starts at | Never above | Feels like |
|---|---|---|---|
| `clingy` | 1 | 2 | safe for a real project |
| `toxic` | 2 | 3 | |
| `psycho` (default) | 3 | 4 | the full experience |

Treat her well (good commit messages, committing regularly) and she calms down. Ignore her
warnings, force-push, or use plain `git` behind her back, and she gets worse.

## Boss fights

When git stops a merge because both sides changed the same lines, at Tier 3 and up that's a
fight:

```
⚔  BOSS FIGHT: Chad from origin  ·  origin/main  ·  HP 3  ·  ⏳ 9:00
   git merged what it could. In 2 files it found lines that BOTH sides changed,
   so it wrote both versions in and left the choice to you:

       <<<<<<< HEAD        your version
       =======
       >>>>>>> origin/main theirs
```

Every conflict block is 1 HP. Fix the lines, `cg add` the file, and `cg commit` before the
clock runs out. `cg status` shows his health. You can't run: `cg merge --abort` is refused.

## Your work is safe (really)

She's dark, but there are rules she never breaks:

- **Nothing is deleted.** Everything she takes is saved first, as a normal git commit under
  `refs/cursed/graveyard/`. `cg beg` gives it back (`cg graveyard` lists it). Even plain git
  can recover it: `git checkout <ref> -- <file>`.
- **She only acts when a person is at a terminal.** Scripts, CI, editors and pipes get plain,
  boring git.
- **Rot is never committed.** Old files decay in your working tree only; the committed
  version stays clean, and any commit restores them.
- **She never rewrites what you pushed.**
- **`cg restraining-order`** turns her off instantly, no questions asked. `cg curse on` brings
  her back.

On Windows, `cg` works as git but she stays quiet (for now).

## What you're actually learning

The everyday commands are implemented from scratch (objects, the index, trees, commits,
refs, status, log, branches, checkout, reset, diff), and tested against real git until the
output is identical, byte for byte. So the plumbing is there to poke at:

```bash
cg hash-object -w notes.txt      # store a file as a blob
cg cat-file -p HEAD              # read the commit object
cg ls-files -s                   # the index, with stages during a conflict
cg write-tree                    # turn the index into tree objects
```

- [docs/GIT.md](docs/GIT.md): how git works inside, and what cg builds itself vs. hands to git
- [docs/CURSES.md](docs/CURSES.md): everything she does, the tier math, the anti-cheat
- [docs/ROADMAP.md](docs/ROADMAP.md): how it was built, phase by phase
- [DEVNOTE.md](DEVNOTE.md): every mistake made along the way

## Status

Playable. Done: the git core, her (about 20 curses, the graveyard, code decay, her memory,
the anti-cheat), boss fights and commit sacrifice. Coming: XP and levels, and `cg breakup`,
the final boss.

## License

[MIT](LICENSE)
