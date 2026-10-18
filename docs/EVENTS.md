# Running a Cursed GIT event

A playbook for hackathons, game jams, classes and conventions: everything an organizer needs,
from the invite to the awards. Players don't need to read this; they get the rules card.

## The idea in one paragraph

Every team builds their project with `cg` instead of `git`. Miranda, the toxic ex, lives in
their repository: she judges commit messages, punishes teams that don't commit, and turns
merge conflicts into timed boss fights. Teams earn closure XP with real work and try to reach
an ending before the event is over. Meanwhile they learn more git than most tutorials teach,
because the game only rewards doing git properly.

## Pick your tracks

Offer one or more. Tracks can run side by side; each team picks theirs.

| Track | Preset | Who it's for | What can happen |
|---|---|---|---|
| **Gentle** | `gentle` (Juniper) | beginners, classes, anyone who'd rather not | Nothing bad. Encouragement, quests, XP, achievements. |
| **Standard** | `hackathon` (Miranda) | most teams | Rewritten commit messages, deadlines, boss fights, files taken (always recoverable). |
| **Hardcore** | `hackathon` + Death March | volunteers, on event machines only | Her threats are real: shutdowns, and the toxic ending deletes the project (archived first). |

## Two weeks before

- [ ] Write your **event file** (start from [`tools/event-example.ini`](../tools/event-example.ini)): name, end
      time, preset, where to hand in cards, who to ask. Host it somewhere teams can reach (a gist,
      your event site) or hand it out as a file.
- [ ] Pick the **pace**. The default clocks suit a long project: 30 minutes to commit at Tier 3,
      3 minutes per conflict block. For a one-day weekend jam, set `speed = 2` (everything twice as
      fast) in the event file, or one clock at a time (`deadline = 20m`, `fightTime = 2m`,
      `decay = 6h`). `cg event` shows each team the clocks they're on.
- [ ] Decide your **awards** (see below) and put them in the invite.
- [ ] Put the **install line** in the invite, so teams arrive ready:
      ```bash
      curl -fsSL https://raw.githubusercontent.com/spiritpaper/cursed-git/main/install.sh | sh     # Linux, macOS
      irm https://raw.githubusercontent.com/spiritpaper/cursed-git/main/install.ps1 | iex          # Windows
      ```
- [ ] Mention the **content**: Miranda is a toxic ex. She's dark, not cruel (no threats of
      violence or self-harm, nothing sexual), but she is *mean*. The Gentle track is right there.
- [ ] If you run **Hardcore**: prepare event machines (nothing important on them), and a
      consent line for volunteers to sign (below).

## The day of: setup (10 minutes per team)

1. `cg version` works (and `git --version`).
2. In the team's project folder: `git init` if it isn't a repo yet, then
   `cg event join <your event file or URL>`.
3. Everyone on the team sets their name: `git config user.name "..."`, `git config user.email ...`.
4. `cg event` shows the event, the time left and the goals. Done.

Windows: she plays in **Windows Terminal, PowerShell and cmd**. In Git Bash she stays quiet
(cg works, she just can't tell it's a person).

## Kickoff (what to say, about 3 minutes)

> Tonight you're dating Miranda. She lives in your repo. Use `cg` wherever you'd type `git`.
> She hates lazy commit messages, hates waiting, and hates it when you use plain git behind her
> back. Commit often, write real messages, and she calms down. Ignore her and she gets worse.
>
> When two of you change the same lines, that's a **boss fight**: fix the conflict before the
> clock runs out. You can't run.
>
> You earn XP for real work. Get enough, win enough fights, and you can **break up** with her:
> how that ends depends on how you treated her.
>
> Lost something? `cg beg`. She keeps everything she takes. Need her gone right now?
> `cg restraining-order`, no questions asked. And `cg quest` teaches you git along the way.

## During the event

- **Every few hours:** collect cards (`cg card --json > team.json`) and drop them on the
  leaderboard: open [`tools/leaderboard.html`](../tools/leaderboard.html) (add `?event=Your Event Name`
  to the address for a title), drag the files in, put it on the projector.
- **Check-ins that work:** "fastest boss kill so far", "who's her favourite", "most toxic team",
  a live `cg card` screenshot from a volunteer.
- **The safe word is real:** if anyone is stressed, `cg restraining-order` stops her instantly.
  No penalty, no explanation needed. They keep playing as plain git, or switch to Juniper
  (`cg partner juniper` after the restraining order).

## Awards

Pick the ones that fit. Each one comes straight off the cards.

| Award | How to pick |
|---|---|
| **Closure** | highest level (ties: XP) |
| **Speedrunner** | fastest boss kill |
| **Heartbreaker** | reached the good ending |
| **Professor** | finished every quest |
| **Collector** | most achievements |
| **Toxic Twins** | highest toxicity (a dishonourable mention, bring a funny prize) |
| **Her Favourite** | top of `cg rivals` in the biggest team |
| **Best Card** | the best `cg card` screenshot, by vote |

Cards are self-reported (they come from each team's machine). For prizes that matter, look at
the team's `cg card` on their screen.

## Hardcore track: Death March

Only on machines and projects that don't matter. The organizer can't turn it on for anyone:
an event file that tries is refused. Each volunteer does it themselves:

```ini
# in the project's .git/config
[cursed]
    deathMarch = true
```

then accepts her warning by typing the oath. From then on, a Tier 4 punishment or a breakup
that's "too soon" **really shuts the machine down** (save everything in other programs!), and
the toxic ending **deletes the project folder**. It's archived and verified first, and
`cg closure` in the emptied folder brings it all back.

A consent line for volunteers:

> I'm playing Cursed GIT in Death March mode on a machine with nothing important on it. I know
> it can shut down this computer without warning and delete this project (which `cg closure`
> restores). I can stop at any time with `cg restraining-order`.

## Troubleshooting

| They say | It's probably |
|---|---|
| "She doesn't say anything." | Not a real terminal (an IDE's output pane, a script, Git Bash on Windows), or `CI` is set. Try a normal terminal. |
| "She won't let me commit." | A lazy message (`fix`, `wip`) at Tier 1, or a dictated apology after a lost fight. Write a real message. |
| "My files are gone!" | A missed deadline. `cg graveyard` lists what she took, `cg beg` gives it back. |
| "`cg merge --abort` doesn't work." | It's a boss fight. Resolve the conflict, or use `cg restraining-order`. |
| "She says who are you?" | No git identity: `git config user.name` and `user.email`. |
| "The computer turned off." | Death March, Tier 4. That's what they agreed to. Everything in git is safe. |
| "The project folder is empty." | Death March, toxic ending. `cg closure` in that folder. |
| "Can we play without her?" | `cg restraining-order` (she stops), or `cg partner juniper` after it (the gentle one). |

## Printables

- **Rules card for every table:** [`tools/rules-card.html`](../tools/rules-card.html). Open it, print it (A4 or Letter, two per page).
- **Leaderboard for the projector:** [`tools/leaderboard.html`](../tools/leaderboard.html).

## Feedback

Tell us how it went: [playtest feedback](https://github.com/spiritpaper/cursed-git/issues/new?template=playtest.md).
Cursed GIT is made by spiritpaper; support the project at [spiritpaper.github.io/#support](https://spiritpaper.github.io/#support).
