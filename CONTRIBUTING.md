# Contributing to Cursed GIT

Thanks for wanting to make her worse (or better). Contributions are welcome: code, lines,
characters, quests, docs, event tools, bug reports and playtest stories.

## What we're looking for

- **Bugs**, especially anywhere `cg` behaves differently from git. Git compatibility is a promise.
- **Lines for Miranda** (and Juniper): funnier, sharper, more specific. Her voice is in
  `internal/curses/lines/`. Read [docs/lessons/writing-a-character.md](docs/lessons/writing-a-character.md) and
  [docs/MIRANDA.md](docs/MIRANDA.md) first.
- **New characters**, as line packs: see [docs/CHARACTERS.md](docs/CHARACTERS.md). Open an issue first
  with the name and the concept.
- **Quests** that teach a piece of git (`internal/curses/quests.go`).
- **Event tooling**: things that make a cg hackathon easier to run ([docs/EVENTS.md](docs/EVENTS.md)).
- **Docs**: if something confused you, it'll confuse the next person. Everything in `docs/` is
  also the [website](https://spiritpaper.github.io/cursed-git/), rebuilt on every push to
  `main`; a new page needs one line in `site/build.sh` (its title and place in the menu).
- The [ROADMAP backlog](docs/ROADMAP.md#backlog) has more.

For anything big, open an issue first so we can agree on the shape before you build it.

## The rules that don't bend

These are what make the game safe to play on a real project. A PR that breaks one won't be merged.

1. **Never lose anyone's work.** Anything she takes is saved first (the graveyard), and
   `cg beg` always gives it back. If she can't save it, she doesn't take it.
2. **She lies about her feelings, never about your data.** Numbers, files and history are
   always true; git can always prove what really happened.
3. **Dark, not cruel.** No threats of violence or self-harm, nothing sexual, nothing outside
   the repository (Death March aside, which the player opted into by hand).
4. **She only acts at a terminal.** Scripts, CI and editors always get plain git.
5. **Real damage needs Death March.** Shutdowns and deletion only happen when the player turned
   it on in `.git/config` themselves and accepted the warning.
6. **Gentle characters never punish.** And a gentle pack never borrows a toxic line.
7. **No real people.** Characters are fictional. Don't put anyone's real words, names or
   private details in a line pack.
8. **Standard library only.** cg has no dependencies; let's keep it that way.

## Getting set up

You need [Go](https://go.dev/dl/) 1.26+ and git.

```bash
git clone https://github.com/spiritpaper/cursed-git.git
cd cursed-git
go build -o cg ./cmd/cg      # the binary, for trying things by hand
go test ./...                # everything
```

**Never run `cg` inside this repository to try something out** (unless you mean to commit
with it). Use a scratch folder: `mkdir /tmp/play && cd /tmp/play && /path/to/cg init`.

Before you push, run the tests the way CI does, with no personal git config:

```bash
HOME=$(mktemp -d) GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null go test -count=1 ./...
```

## How the tests work

- **Real git is the judge.** Most git-side tests build twin repositories, run an operation with
  git in one and with cg in the other, and compare everything: files, modes, the index, refs,
  the reflog, the output. See `twins` and `both` in `internal/cli/branch_test.go`.
- **Her world is injected.** `withHer` (in `internal/cli/curses_test.go`) gives her a fake clock,
  fixed luck and typed input, and pretends there's a terminal. Move time with `h.wait(...)`; set
  `h.luck` low to make random things fire.
- **Every test gets its own data folder** and a fake machine: no test can shut down your
  computer or touch your real `~/.local/share/cursed-git`.
- **Break your code to test your tests.** For anything that protects the player, break the
  rule on purpose and check that a test fails. A test that can't fail is decoration.

## Writing lines

Her lines are JSON (`internal/curses/lines/miranda.json`). A line can be plain text, or carry
`when` conditions (the most specific matching line wins) and a hidden red `flag`:

```json
{"text": "Work in progress. Like us.", "when": {"msg": ["wip"]}}
```

Short, specific, in her voice, with a real git idea inside the joke when you can. Add at least
three lines for anything that happens often. The full format is in
[docs/CHARACTERS.md](docs/CHARACTERS.md).

## Code style

- `gofmt` and `go vet ./...` must be clean.
- Match the code around you: names, comment density, how errors are handled.
- Comments say *why*. The files open with a short explanation of what they do; keep that up.
- If you learned something the hard way, add it to [DEVNOTE.md](DEVNOTE.md). That file is half
  the project's charm.

## Commit messages

Say what changed, so `git log` reads like a story:

```
Add the hoover: she comes back after the breakup

After the toxic or good ending she returns days later, in any project, in one
of four costumes. Typing "no" blocks her.
```

- A short, imperative subject (under ~72 characters): *Add…*, *Fix…*, *Teach her…*.
- A body when the *why* isn't obvious.
- A little of her humour is welcome, after the information.

## Pull requests

1. Fork, branch, commit.
2. Tests pass (the CI-style command above), `gofmt` and `go vet` are clean.
3. Docs updated if behaviour changed (README, the docs in `docs/`, `cg help`).
4. Fill in the PR template. Screenshots or terminal output of her new behaviour help a lot.

Releases are tagged by the maintainer; the release workflow builds every package from the tag.

## Conduct

Be kind to each other. She's the only one allowed to be toxic here. See
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
