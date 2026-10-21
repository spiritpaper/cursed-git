# How to play

Everything a player needs, from your first commit with her to the breakup. Want to know
*why* she does what she does? That's [How the game works](CURSES.md).

## 1. Install her

On Linux or macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/spiritpaper/cursed-git/main/install.sh | sh
```

On Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/spiritpaper/cursed-git/main/install.ps1 | iex
```

You also need [git](https://git-scm.com/downloads). Homebrew, Scoop, .deb, .rpm, Nix, Docker
and building from source are all in the [README](../README.md#install). `cg version` checks
that it worked.

**Just curious?** `docker run -it --rm ghcr.io/spiritpaper/cursed-git` is a throwaway
playground with her already in it, and nothing touches your machine.

## 2. Let her move in

| You have | Type |
|---|---|
| a new project | `cg init` |
| a project already in git | `cg curse on --preset clingy` (start gentle, really) |
| an event invite | `cg event join <the file or link the organizer gave you>` |
| a wish for someone nice | `cg init --preset gentle` (Juniper, no curses ever) |

The preset sets how bad she can get: `clingy` (Tier 1–2), `toxic` (2–3), `psycho` (3–4, the
default) or `hackathon` (3–4, sized for an event). Make sure git knows who you are first:
`git config user.name "…"` and `git config user.email …`.

**Don't play on something important.** Nothing she does loses work (everything she takes,
she saves first), but the point of the game is that she gets in your way.

## 3. The everyday loop

Use `cg` everywhere you'd type `git`. Every git command works: what cg doesn't do itself, it
hands straight to git.

```bash
cg add .
cg commit -m "add the login form"
cg push
```

What keeps her calm is what makes you good at git:

- **Commit often.** Changes left sitting uncommitted make her impatient (30 minutes at Tier 3,
  counted from when she first sees them).
- **Write real commit messages.** `fix` and `wip` get refused or rewritten.
- **Pull before you push**, and keep in sync with your team (see pull requests, below).
- **Don't go behind her back** with plain `git`. She reads the reflog.

Three good commits in a row calm her down a step. Keep it up and she love-bombs you for an
hour: compliments and no curses at all. (Don't ruin it with a lazy message.)

## 4. Checking on her

| Command | Shows you |
|---|---|
| `cg mood` | her tier and temper, your toxicity, your closure level, what she says you owe |
| `cg card` | the whole relationship on one screen (screenshot it) |
| `cg achievements` | 27 to collect |
| `cg quest` | a git lesson to do next, checked against your repo and paid in XP |
| `cg rivals` | in a team repo: everyone who commits, and her favourite |
| `cg face` | her |

## 5. When things go wrong

**"She took my files!"** You had uncommitted work for too long. `cg graveyard` lists
what she's holding and `cg beg` gives it back. It always works, and it costs you +2 toxicity.
(Plain git can get it back too: `git checkout <ref> -- <file>`.)

**"She refused my commit."** Usually a lazy message: write a real one. Sometimes she just
says no for a made-up reason: try again. After midnight or late on a Friday she asks if
you're sure; answer `y`.

**"My file is full of ░▒▓."** Code decay: a file nobody touched for a long time. Git still
has the clean version. Make any commit and it comes back.

**"A commit disappeared."** A sacrifice. `cg graveyard` lists it and
`cg cherry-pick <ref>` brings it back. (She only ever takes commits you haven't pushed.)

**"She merged somebody's branch!"** A pull request from a teammate. If it went in clean,
`cg reset --keep ORIG_HEAD` takes it back (and keeps your uncommitted work). If it conflicted, it's a boss fight: see below.

**"She's quiet."** She only plays at a real terminal. In an editor's output pane, a script,
CI or Git Bash on Windows, `cg` is plain git.

## 6. Boss fights

When two sides changed the same lines (your merge, a pull, a rebase, one of her pull
requests), git stops and asks you to choose. At Tier 3 and up, that's a timed fight.

```
⚔  BOSS FIGHT: Sam  ·  origin/login  ·  HP 2  ·  ⏳ 6:00
```

1. Open each conflicted file. Every block looks like this:
   ```
   <<<<<<< HEAD
   your version
   =======
   their version
   >>>>>>> origin/login
   ```
2. Keep the right lines (yours, theirs, or a mix) and delete the three marker lines.
   Each block you fix is a hit.
3. `cg add <file>` for each file. (She won't let you stage a file that still has markers.)
4. `cg commit` finishes him. `cg status` shows his health and the clock.

You can't run: `cg merge --abort` and friends are refused. Lose and she writes your next
commit message for you; at Tier 4 she calls the merge off herself (your work goes to the
graveyard). Win and you get +250 XP.

`cg ls-files -s` shows git's three copies of a conflicted file: 1 is the common ancestor,
2 is yours, 3 is theirs. `cg quest` has a lesson on exactly this.

## 7. Pull requests from your team

Once your project has a remote, she sometimes fetches it, finds a branch somebody pushed that
you don't have, and merges it for you:

```
📬 Pull request #3 from Sam: origin/login → main · 2 commits
```

- **In sync** (you pull often and don't edit the same lines at once): it goes in clean,
  as a real merge commit, and you get +25 XP.
- **Out of sync:** at Tier 2 she closes it and tells you to fix it yourself
  (`cg merge origin/login`, while it's still small). At Tier 3 and up it's a boss fight
  against whoever wrote it.

Your uncommitted work is safe: if the pull request touches a file you're still editing, it
waits and comes in right after your next commit. Work in other files is left alone.

## 8. Stopping, pausing, switching

| You want | Type |
|---|---|
| her to stop **right now**, no questions | `cg restraining-order` |
| her back | `cg curse on` |
| someone kind instead | `cg partner juniper` (after a restraining order or a breakup) |
| her to forget this project | `cg forget` (`--all`: every project) |

The restraining order always works, has no penalty, and is the right answer whenever you're
stressed or have a real deadline.

## 9. Breaking up

You earn **closure XP** with real work: +1 per committed line (once the commit has survived a
day), +10 per decent commit message, +250 per boss fight won. To be allowed to leave you
need level 10, three fights won and 2,000 lines (much less at an event). `cg mood` shows how
far you are.

```bash
cg breakup --dry-run     # which ending you'd get, without doing it
cg breakup               # the final boss
```

- **Too soon:** she leaves *you*. A letter on your Desktop, and the lights go out (a show in
  the terminal; nothing happens to your computer).
- **Toxic** (your toxicity is 10+): she takes everything and leaves a receipt. Outside Death
  March nothing is deleted.
- **Good:** denial, bargaining, tears, and freedom. Juniper is waiting.

## 10. Death March

The mode where her threats are real: Tier 4 losses **shut the computer down**, and the toxic
ending **deletes the project folder** (archived first; `cg closure` in that folder brings it
back). Only on machines and projects that don't matter, and only if you turn it on yourself:

```ini
# in the project's .git/config
[cursed]
    deathMarch = true
```

Then she shows a warning and waits for you to type the oath. Nobody else can turn it on for
you, not even an event organizer. Full details: [How the game works §6](CURSES.md#6-death-march-).

## 11. Questions

**Is my code safe?** Yes. Everything she takes is saved as a git commit first, she never
rewrites what you pushed, rot is never committed, and without Death March nothing reaches
outside the repository. That's [the promise](CURSES.md#1-the-promise-nobody-loses-work).

**Can I use plain git?** Any time. She'll notice and be upset (+1 toxicity), but nothing
breaks. cg and git share the repository byte for byte: see [GIT.md](GIT.md).

**Does it work with my editor's git panel, CI, scripts?** Yes, they get plain git. She only
acts when a person is typing.

**Can I make the clocks faster or slower?** Yes, in `.git/config`: `speed`, `deadline`,
`fightTime`, `decay`. See [the settings](CURSES.md#8-settings).

**Found a bug, or have a story?** [Open an issue](https://github.com/spiritpaper/cursed-git/issues).
