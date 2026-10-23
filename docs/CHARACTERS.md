# Characters

Who you play with. Every character is a **line pack**: one JSON file of what they say, plus a
few keys about who they are. The game rules (tiers, curses, fights, XP) are the same code for
everyone; the pack decides the voice, the name, the rival, and whether the character is
toxic at all.

## Who ships today

| Character | Pack | Mode | Who she is |
|---|---|---|---|
| **Miranda** | `miranda` (default) | toxic | The toxic ex. Judges your commits, gets jealous, turns conflicts into boss fights. Her rival is Chad. |
| **Juniper** | `juniper` | gentle | The partner who cherishes you. No curses, no punishments, nothing taken. She still counts XP, quests and achievements. |

`cg partner` lists everyone; `cg partner <name>` switches. You can't swap a toxic partner out
while she's still around: break up with her first (`cg breakup`), or file the restraining order.
After Miranda's good ending, the credits introduce Juniper.

## How a pack works

`internal/curses/lines/<name>.json` (ships with cg) or `$CURSED_GIT_HOME/personalities/<name>.json`
(yours: `~/.local/share/cursed-git/personalities/`). Yours wins if both exist.

```json
{
  "_name":  ["Miranda"],
  "_rival": ["Chad"],
  "_mode":  ["toxic"],

  "curse_on": ["Oh. You're back. I knew you'd come back."],
  "rewrote":  ["I fixed your commit message. You're welcome."]
}
```

- **`_name`**: shown on the card ("Sam & Miranda") and in how-to-play.
- **`_rival`**: the boss when a pull brings conflicts ("Chad from origin").
- **`_mode`**: `toxic` (the full game) or `gentle` (Tier 0 forever, no punishments).
- **Every other key** is a moment, with one or more lines (she picks one). Placeholders like
  `{name}`, `{msg}`, `{boss}` are listed in `miranda.json`'s `_about`.
- **`when`: special cases** (Valve's approach: the most specific line that matches wins). A line
  can be an object with conditions instead of plain text:
  ```json
  "commit_reaction": [
    {"text": "Work in progress. Like us.", "when": {"msg": ["wip", "work in progress"]}},
    {"text": "\"{msg}\" at {hour}. Who are you committing for?", "when": {"msg": ["wip"], "hour": "1-4"}}
  ]
  ```
  Conditions: `msg` (whole words or phrases in the commit message, any case), `msg_emoji`,
  `tier` (`"3"` or `"3-4"`), `hour` (`"22-4"` wraps past midnight), `weekday`, and `min` /
  `max` on `good_streak`, `toxicity`, `fights_won`, `fights_lost`, `commits`, `grudges`, `xp`,
  `level`, `temper`. Every condition must hold; the line with the most conditions wins; plain
  lines are the fallback. She remembers her last few lines per moment and won't repeat them.
- **`flag`**: a line can raise one of the red flags (`"flag": "leash"`, see MIRANDA.md §3).
  It's counted on the card (🚩 n/11), never named in the game.
- **`commit_reaction`** is special: she only speaks there if a `when` line matches (so she
  reacts to `revert` or `refactor`, and stays quiet on ordinary messages).
- **Missing lines:** a toxic pack falls back to Miranda's line for that moment, so a new
  character can start with a handful of lines. A gentle pack never borrows: a missing line is
  silence, so a kind character can't accidentally say something cruel.

Pick one with `cg partner <name>`, or by hand: `git config cursed.personality <name>`.

## Art and portraits

Text art (ASCII, or braille pictures like the ones on emojicombos.com) lives in a gallery:

```
cg art                        list the pictures
cg art miranda                show one
cg art add cat cat.txt        keep your own (or leave out the file, paste it, press Ctrl-D)
cg art rm cat                 throw yours away
```

Pictures ship in `internal/curses/art/<name>.txt`, and yours go to
`$CURSED_GIT_HOME/art/<name>.txt` (`~/.local/share/cursed-git/art/`). Yours wins if both
exist, so `cg art add miranda my-miranda.txt` gives her a new look, and `cg art rm miranda`
brings the old one back. Names are lowercase letters, digits, `-` and `_`. A picture is at
most 64 KB, and anything that could talk to the terminal (escape codes, carriage returns) is
stripped.

The file name decides where a picture shows up:

| Name | Shown |
|---|---|
| `miranda` | her portrait, once, when she arrives (`cg init`) |
| `miranda-content`, `-unimpressed`, `-judging`, `-hostile`, `-furious` | her mood at Tier 0 to 4: `cg mood`, and `cg face <mood>` |
| `miranda-love` | while she's love-bombing you |
| `miranda-crying` | the good ending of the breakup |
| anything else | only in the gallery (`cg art <name>`) |

Swap `miranda` for any character's pack name (`juniper-content`, `rex-furious`). A mood
without a picture shows the little ASCII face, and the help screen always does. So one
picture is enough to start: `cg art add miranda-furious angry.txt` and only her worst mood
changes.

Miranda's portrait is braille art from
[emojicombos.com](https://emojicombos.com/anime-ascii-art), 93 columns wide, so give your
terminal room. She has no mood pictures yet.

## Adding a character (the plan)

1. **Name and concept, from the author.** Names are a creative call: the author picks them.
   Next in line: a toxic ex-boyfriend (name and his rival's name to be decided), and more
   gentle partners.
2. **Pick the mode.** Toxic characters get every curse; gentle ones get none.
3. **Read the lesson:** [lessons/writing-a-character.md](lessons/writing-a-character.md).
4. **Write the pack.** Start from `miranda.json` (toxic) or `juniper.json` (gentle) and rewrite
   every line in the new voice. Toxic packs: at least `curse_on`, the judge (`lazy_*`,
   `rewrote`), the fight lines (`fight_*`), and the breakup lines (`breakup_*`); the rest falls
   back. Gentle packs: every moment that can happen at Tier 0 (see `juniper.json` for the list).
5. **Her two rules still apply.** She lies about her feelings, never about your data. Dark, not
   cruel: no threats of self-harm or violence, nothing sexual, nothing outside the repository.
6. **Test it.** A test like `TestCustomToxicPack` (personality_test.go): the name on the card,
   a line of their own, and (toxic) the fallback. For a gentle pack, the Juniper tests:
   nothing punished, nothing taken, no fights.
7. **Ship it.** Drop the file in `internal/curses/lines/`; it's embedded at build time and
   shows up in `cg partner`.

## Ideas for later

- Per-character rules: a pack could ask for a preset (Juniper always means `gentle`).
- Community packs: a folder of packs people send in, installable with one command.
