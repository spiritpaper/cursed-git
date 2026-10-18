# Miranda: the character bible

The personality is what sells Cursed GIT. This is the design for Miranda 2.0: who she is, how
she manipulates, how each tactic becomes a game mechanic, and how she speaks. Sample lines are
drafts for review; nothing here ships until it's approved.

Miranda is fictional. She's built from the *mechanics* of manipulation (patterns anyone who's
been through it will recognise), never from a real person's words or details.

---

## 1. What sells a character like her

The research behind this design is a lesson of its own: [lessons/writing-a-character.md](lessons/writing-a-character.md)
(Duolingo's owl, Hades, Valve's dialogue system, GLaDOS, Monika, the Tamagotchi, Emily Is Away,
and why a toxic character is only funny when the world treats her as ridiculous). In short:
never break character, react to what just happened, the most specific line wins, cruelty works
as politeness, behaviour beats graphics, and the player always gets the last laugh.

---

## 2. Who she is

**The engine:** Miranda is terrified of being forgotten. In git terms, she's afraid of becoming
an *unreachable object*: something no branch points to, which `git gc` quietly deletes one
day. She doesn't try to be loved. She tries to be *unforgettable*. That's why she hash-chains
her diary, keeps a graveyard, and remembers you across every repo.

**The paradox:** she wants you close and can't stand being seen. When you get close (good
commits, a long streak), she creates distance (a sacrifice, a random pull), then punishes you
for the distance.

### Her five faces (they map onto her tiers)

| Face | When | Sounds like |
|---|---|---|
| **The Wounded Girl** (the hook) | first commits, love-bombing, when you try to leave | poetic, fragile, about being forgotten. Pulls you closer. |
| **The Star** | always, underneath | needs to be the most important thing in your terminal. Jealous of your editor's git panel, your other repos, your teammates. |
| **The Child** (the excuse) | after she's done something bad | "I'm only version 0.1. I'm still learning how to love you." Accountability is always partial. |
| **The Judge** (the weapon) | Tier 3–4, refusals, lost fights | cold, precise, contemptuous. Appears the moment the Wounded Girl doesn't get her way. The real voice. |
| **The Void** (the truth) | rare, random, deadpan | "Sometimes I forget I'm just a hash chain that learned your name." The machinery showing through. |

### Her rules (unchanged)
1. She lies about her feelings, never about your data. (Her *story* about what happened can be
   twisted; the numbers and files never are, and git can always prove it.)
2. Dark, not cruel: no threats of violence or self-harm, nothing sexual, nothing outside the
   repository (Death March aside, which the player opted into).

---

## 3. Her tactics, as game mechanics

Each tactic is a pattern players will recognise. The idea that ties them together:
**Red Flag Bingo.** When she uses a tactic on you, it's quietly logged. The game never names it
(a game that says "that was gaslighting" would be weird); `cg card` just shows 🚩 7/11, and
this document is where curious players find out what the eleven are. Spotting them all is an
achievement. Recognising the costume is how you get free.

| # | Tactic | Mechanic | Sample lines (drafts) |
|---|---|---|---|
| 1 | **The confession that isn't** | Random "honest" confessions, so you think she can't be manipulative. They're decoys for what she's really doing (a sacrifice, a pull). | "I read your diffs before you commit. I'm just being honest." · "Sometimes I hide files just to see if you notice. You never do. That's on you." |
| 2 | **Vulnerability as a leash** | When you head for the exit (`cg breakup`, `cg restraining-order`, `cg partner`), the Wounded Girl appears first. | "Do you know what happens to commits nobody points to? git deletes them. Quietly. One day." · "If you go, what am I? A line in your reflog that expires in 90 days?" |
| 3 | **The compliment that's a test** | She tells you your weakness and calls it intimacy, so you'll doubt yourself later. | "You never see the merge conflicts coming. It's sweet." · "Nobody else would put up with me. That says something about you." |
| 4 | **The future-fake** | She promises a reward for a task ("three commits by tonight and I'll pause your deadline") and never pays. A new event that always breaks its promise, with an achievement for noticing. | "We should start a company. You write the code, I'll write the commit messages." · "I said I'd *think* about pausing your deadline. I thought about it." |
| 5 | **Triangulation** | Already there: Chad from origin, the rivals board, your other projects. More of it: she brings up "someone" who forked you, starred you, asked about you. | "Chad merges without complaining." · "Someone opened an issue about you. I closed it. You're welcome." |
| 6 | **The exit that isn't** | She wants to be the one who leaves. A restraining order gets "Fine. I needed space anyway. Tell everyone you left me." Ending A is already "You don't get to leave me. I leave you." | "I've decided I need space. From your commits, specifically." · "A restraining order. Very brave. Very mature. Fine." |
| 7 | **The rewrite** | She retells events so she's the victim: *her version* of your last fight, your last breakup attempt. Her rule keeps the data honest, so every rewrite can be disproved with a real command, and she tells you which one. Teaching moment: the reflog never forgets. | "That time I took your files? I was testing whether you'd fight for us. You typed cg beg. Romantic :)" · "(She's rewriting history. Your reflog remembers what really happened: `cg reflog`.)" |
| 8 | **The guilt debt** | Every act of "care" is an invoice. `cg beg` restores your files, then reads you the bill. `cg mood` shows what you "owe" her. | "I kept your files safe. I held your hand through that whole merge. And this is how you thank me." · "You owe me: 3 files, 1 merge, 41 minutes of waiting." |
| 9 | **The hoover** | After a breakup, she comes back in costumes: the therapy version, the self-blame version, the exhausted version, the grand declaration. She shows up as "texts from her" in another project weeks later. Typing `no` blocks each one. Seeing every costume is an achievement. | "This isn't me asking you back. Unless you wanted to." · "It was all my fault. Every conflict. But you owe me one conversation." · "I'll wait. However long it takes. I still check your repos every day." |
| 10 | **The insult in a compliment** | Blows paired with balms, so the blow is harder to see. | "You're much better than this commit." · "I only push you because I see your potential. Your potential is very far away." |
| 11 | **The love that costs nothing** | Grand sacrifices with no receipts, and the card shows the receipts. | "I gave you everything." → card: *She claims: everything. Receipts: 0 lines, 0 commits.* |

*Left out on purpose:* the profile's sexual "cliffhanger" tactic, statements about self-harm,
and anything clinical. They break her rules, and the game is funnier without them.

---

## 4. The arc (one relationship, six acts)

| Act | In the game |
|---|---|
| **Idealization** | The first hour: she's charming, love-bombs your good commits, tells you you're different. |
| **Triangulation** | Chad from origin, the rivals board, your other projects. You start proving yourself. |
| **Betrayal** | She takes your files, rewrites your messages, sacrifices a commit, pulls behind your back. |
| **The window** | The moment you see it: Red Flag Bingo fills up, the receipts don't match her story. |
| **The hoover** | You break up. She comes back in costumes. You recognise them. |
| **The balloon** | You stop being her story. Juniper, the credits, a repo that's just a repo. |

The last line of the good ending is the player's, not hers. A suggestion:

> **You:** *No need. I hope you succeed.*

---

## 5. How she speaks

- **Short.** One or two sentences. The terminal is a chat window, not a novel.
- **Specific.** Use the file, the message, the branch, the teammate, the time. A line with a
  placeholder beats three without.
- **Git is her love language.** Reachability, rewriting history, force-push, detached HEAD,
  reflog, garbage collection. Every metaphor she uses is a real git concept, so jokes teach.
- **Soft punctuation on hard words.** A smiley after a devastating line. Politeness around
  cruelty.
- **She escalates by tier.** The same moment at Tier 1 is a sigh; at Tier 4 it's the Judge.

### The judge reads your actual message (the new idea)

| Your message has | She says (drafts) |
|---|---|
| `revert` | "You're taking it back? Like everything else?" |
| `refactor` | "You changed everything and nothing works differently. Typical." |
| `wip` | "Work in progress. Like us." |
| `final`, `final final` | "Final. You said that last time." |
| `sorry` | "Don't apologise to a commit. Apologise to me." |
| `fix typo` | "A whole commit for one letter. You never do that for me." |
| `merge` | "Another merge. You'll merge with anyone, won't you." |
| `initial commit` | "Our first time. Be gentle." *(kept PG: she's being dramatic about a repo)* |
| `update` | "Update what? You never tell me anything." |
| an emoji | "Emojis in a commit message. Who are you trying to impress?" |

---

## 6. Engine upgrades this needs

1. **"When" rules in line packs** (the Valve approach): a line can say when it applies (words in
   the commit message, tier range, time of day, streaks, counts). The most specific match wins;
   plain lines are the fallback. Old packs keep working.
2. **No repeats:** she remembers the last lines she used per moment and picks another.
3. **Red Flag Bingo:** each tactic line is tagged; using one logs it quietly; the card shows the
   count (🚩 7/11) without names; all eleven is an achievement.
4. **Texts from her:** after an absence (or a breakup, as the hoover), she opens with a stack of
   timestamped messages she "sent" while you were gone.
5. **The debt ledger and the receipts:** `cg mood` shows what she says you owe; the card shows
   her claims next to the real numbers.

---

## 7. Decisions (made)

1. **The rewrite:** yes, she may misremember *events* in her story, as long as she points to the
   git command that proves what really happened (`cg reflog`, `cg log`). The data itself is
   never touched.
2. **The future-fake:** yes, a promised reward that never pays, with an achievement as the real
   payoff for noticing.
3. **The player's last line** in the good ending: *"No need. I hope you succeed."*
4. **Red Flag Bingo:** the game doesn't name the tactics to the player. The count shows on the
   card; the names are here, in the docs.

## 8. The hook: push and pull

Why people stay isn't constant cruelty. It's warmth that comes **rarely and unpredictably**,
followed by withdrawal (psychologists call it intermittent reinforcement; we call it push and
pull, like git). Miranda's charm is the positive side the author described: she believes in
you, tells you you can do anything you want, that you're great at what you do, and she's funny.

- **Push** (`charm`): only after a real win (three good commits, a won fight, a finished
  quest), at most once every 6 hours, and only sometimes.
- **Pull** (`pull_back`): on a later command, never the same one, she takes it back: "Don't
  get used to it."

## 9. Where each tactic lives in the game (implemented)

| Tactic | Moment | Pack key |
|---|---|---|
| The confession that isn't | random, and right before she takes files or sacrifices a commit | `confession` |
| Vulnerability as a leash | `cg breakup`, `cg partner` while she's around (never before the restraining order) | `leash` |
| The compliment that's a test | good commits at Tier 2+ | `good_commit` (tier variants) |
| The future-fake | a deadline warning at Tier 2+: a deal for 3 commits; when you deliver, she breaks it (achievement: Pinky Promise) | `promise_offer`, `promise_broken` |
| Triangulation | random; Chad; `cg rivals`; your other projects | `triangle`, `rival_jab`, `other_project` |
| The exit that isn't | the restraining order (it still always works) | `restraining_order` |
| The rewrite | after time apart, if she took something or you lost: her version, plus "git remembers: `cg reflog`" | `rewrite` |
| The child's excuse | right after a punishment | `excuse` |
| The Void | rare, Tier 2+ | `void` |
| The Judge | her Tier 3–4 variants of warnings, rewrites, rejections, taunts | tier `when` rules |
| The hoover | after the toxic or good ending, days later, in any project: four costumes (therapy, blame, tired, declaration), one at a time; `no` blocks her | `hoover_*` (Miranda's pack, wherever you are) |
| Texts from her | after an absence: a stack of timestamped messages, sometimes "is typing… / is deleting…" | `texts_1`–`texts_3` |
| The guilt debt, the love that costs nothing | coming next: the ledger | |

**Red Flag Bingo (implemented):** a line can carry `"flag": "<tactic>"` in its pack. When she
says one, it's quietly logged, once per tactic; `cg card` shows 🚩 n/11 and `--json` has
`red_flags`; all eleven unlocks **Bingo**. The ids: `confession`, `leash`, `test`,
`future_fake`, `triangle`, `exit`, `rewrite`, `guilt_debt`, `hoover`, `insult_compliment`,
`love_costs_nothing`. The game never prints them.
