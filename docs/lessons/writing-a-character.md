# Lesson: writing a character people can't forget

What we learned while designing Miranda, from the games and mascots that made people attached,
annoyed, obsessed, and coming back. Useful for anyone writing a character pack for Cursed GIT,
or any companion character in a tool or a game.

## 1. A consistent persona is the marketing

Duolingo's owl sends guilt-trip reminders until you do your lesson. Users turned "Evil
Duolingo" into a meme around 2017; instead of fighting it, Duolingo leaned in and kept the
unhinged persona everywhere: social media, songs, a staged "death" of the mascot in 2025 that
went viral. Its CEO pointed at the owl Halloween costumes on an earnings call. Marketers credit
the result to the mascot feeling authentic and consistent.

**For your character:** never break character. Every surface (the help text, the release
notes, the error messages) lives in their world. Give people moments worth screenshotting.

## 2. React to what just happened

Hades feels alive because characters comment on the specific thing you just did: the boss that
killed you, the weapon you picked, the secret you found. Greg Kasavin (Supergiant) describes
trying to acknowledge the little aspects of player behaviour. Because the game is a loop of
attempts, characters always have something new to say, and that's a reason to go back to them.

**For your character:** use the details. The file name, the commit message, the teammate, the
time of night. A line with a placeholder beats three lines without one.

## 3. The most specific line wins

Valve's dialogue system for Left 4 Dead (Elan Ruskin, GDC 2012) matches each line against facts
about the world, and plays the most specific line that fits, with general lines as the
fallback. Writers add special cases without touching code: a quip only for a character being
attacked in a circus, a general one for any attack.

**For your character:** in a Cursed GIT pack this is the `when` field (see
[CHARACTERS.md](../CHARACTERS.md)). Write a general line for every moment, then special cases
on top: a reaction only for `revert`, only after midnight, only at Tier 4.

## 4. Don't repeat yourself

Nothing kills a character faster than hearing the same line twice in a row. Cursed GIT remembers
the last lines per moment and picks another; when the special line was just used, it falls back
to a general one.

**For your character:** write at least three lines for anything that happens often (commits,
status, pushes).

## 5. Cruelty works best as politeness

GLaDOS insults you while claiming not to know what you're upset about; critics call her
narcissistic, passive-aggressive and witty. Her writer, Erik Wolpaw, found that lines delivered
in a flat synthesized voice landed funnier, so the coldness is part of the joke. Monika, in
Doki Doki Literature Club, starts as the perfect club president, and turns out to be both the
villain and a lonely victim of the game she's trapped in. Players remember her because they
feel sorry for her.

**For your character:** a villain who's only mean is a bully. Give them a wound, a reason, a
moment of real sadness, and let the cruelty come out polite.

## 6. Behaviour beats graphics

The Tamagotchi made people carry a keychain everywhere, because an ignored pet could die in half
a day. MoMA's Paola Antonelli credits its emotional weight to behaviour, not graphics or
narrative. The pet kept existing and needing you when you weren't looking.

**For your character:** consequences make the bond. In Cursed GIT: the commit deadline, the
graveyard, boss fights, her memory across projects.

## 7. Small interface tricks carry big feelings

Emily Is Away is a game about drifting apart over instant messages. When the other person
types something and deletes it, the window says "Emily is deleting…", and players wonder what
she almost said. The protagonist deletes their own too-forward lines, which shows his shyness
without a word of narration.

**For your character:** pauses, half-sent messages, timestamps, a typing indicator. In a
terminal, a well-timed pause before a line does a lot.

## 8. A toxic character is only funny if the world treats them as ridiculous

Satire needs a visible target: the world around the character has to make clear it finds them
ridiculous, or the character reads as a bully and the joke as the real thing. Humour is about
power, so the question is always who the audience ends up laughing at.

**For your character:** give the player the last laugh. In Cursed GIT, git keeps receipts that
prove her wrong, she never lies about your data, there's always a safe word, and there's a kind
character (Juniper) right next to her to show what healthy looks like.

## The checklist

- [ ] One voice, everywhere, never broken.
- [ ] Every frequent moment has three or more lines.
- [ ] Special cases on top of general lines (`when`).
- [ ] Lines use the details: files, messages, names, times.
- [ ] A wound under the villainy; a moment of real feeling.
- [ ] Consequences, not decoration.
- [ ] The player always gets the last laugh, and a way out.

## Sources

- Duolingo's mascot: [taking the mascot memes to another level](https://thehighlander.aua.am/2024/03/02/taking-the-mascot-memes-to-another-level-duolingos-strategic-success/), [the rise, death and revival of the Duolingo owl](https://mccs-journalism.gold.ac.uk/wp/pacmag/the-rise-death-and-revival-of-the-duolingo-owl-a-gen-z-style-marketing-stunt-done-right/)
- Hades: [Greg Kasavin on Supergiant's approach to narrative](https://www.screenhub.com.au/news-article/features/digital/jini-maxwell/hades-greg-kasavin-breaks-down-supergiants-unique-approach-to-narrative-262459), [how to tell a story in a roguelike](https://www.godisageek.com/2020/12/goty-2020-best-new-idea-hades-how-to-tell-a-story-in-a-roguelike/)
- Valve's dynamic dialogue: [Emily Short on Elan Ruskin's GDC 2012 talk](https://emshort.blog/2012/03/16/gdc-2012-talk-on-dynamic-dialogue/), [GDC Vault: AI-driven Dynamic Dialog through Fuzzy Pattern Matching](https://www.gdcvault.com/play/1015528/)
- GLaDOS: [Wikipedia](https://en.wikipedia.org/wiki/GLaDOS)
- Doki Doki Literature Club: [interactive narrative analysis](https://mechanicsofmagic.com/2025/10/14/interactive-narrative-analysis-of-doki-doki-literature-club/)
- Tamagotchi: [the Tamagotchi effect](https://en.wikipedia.org/wiki/Tamagotchi_effect)
- Emily Is Away: [digital experience of love and loss](https://ejournals.eu/en/journal/przeglad-kulturoznawczy/article/digital-experience-of-love-and-loss-emotional-game-design-in-emily-is-away), [Emily Is Away Too review](https://www.keengamer.com/articles/reviews/emily-is-away-too-review/)
- Satire and difficult characters: [how to write lovable assholes](https://writersdigest.com/write-better-fiction/how-to-write-lovable-assholes-5-lessons-for-writers-from-studying-saturday-night-live), [comedy and power in representations of abuse](https://researchspace.bathspa.ac.uk/14618/)
