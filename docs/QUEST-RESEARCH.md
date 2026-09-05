# What the literature says about repeatable quests

Researched 4 Sep 2026, at the end of the session that put people in the inn and
behind the doors of the houses. Written down because the useful half of it is
about work we have *not* done, and a report that lives in a chat log is a report
that gets researched twice.

Read the source-quality section before quoting anything out of this. The good
material is thinner than it looks, and the thin material is confident.

---

## Source quality

**Load-bearing** — peer-reviewed, GDC, or a studio talking about something it
shipped:

- Doran & Parberry, *A Prototype Quest Generator Based on a Structural Analysis
  of Quests from Four MMORPGs* (ACM PCG Workshop 2011).
  <https://ianparberry.com/pubs/pcg2011.pdf> — 750+ quests from EVE, WoW,
  EverQuest and Vanguard reduced to a grammar: 20 atomic actions and ~35 named
  strategies under 9 NPC motivations, each with preconditions and
  postconditions. The single most useful thing found.
- Breault, Ouellet & Davies, *Let CONAN tell you a story: Procedural quest
  generation* (IEEE Trans. CIAIG, 2018). <https://arxiv.org/pdf/1808.06217> — a
  working generator built on Doran & Parberry, explicitly designed to generate
  from world-state facts and to fight repetition with NPC-preference weighting.
- Alexander & Martens, *Deriving Quests from Open World Mechanics* (FDG 2017).
  <https://arxiv.org/pdf/1705.00341> — quests derived from Minecraft's own
  crafting and mining rules rather than laid over the top of them.
- Yu, Guzdial & Sturtevant, *The Definition-Context-Purpose Paradigm* (2021).
  <https://arxiv.org/pdf/2110.04148> — interviews with 15 developers at BioWare,
  Bungie, Ubisoft and others about what a quest is *for*.
- Hoge (Monolith), *Helping Players Hate (or Love) Their Nemesis*, GDC 2018 —
  the Nemesis system, from the people who built it.
- Flannum & Johanson (ArenaNet), *Designing Guild Wars 2 Dynamic Events*, GDC
  2010 — ambient and systemic triggering.
- *Missions Played As Anyone in Watch Dogs: Legion*, GDC AI Summit —
  procedurally-parameterised NPCs driving mission content. Their number: every
  line of quest dialogue needed about **20 variants** to cover the trait space.
- *Procedural Generation of Cinematic Dialogues in Assassin's Creed Odyssey*,
  GDC — about 15% of cinematic scenes are fully procedural in staging, framing
  and lighting. Presentation varied; mechanics untouched.
- *Procedural Quest Generation: Current and Future Industry Outlook*, Game
  Developer — a survey across 35+ studios, and the least glamorous of these.

**Corroborating only** — wikis, hobbyist blogs, SEO content. Useful for *what a
shipped game does mechanically*, worthless as design authority, and flagged as
such wherever it is used below: PC Gamer on fetch quests, Contains Moderate
Peril, Warcraft Wiki and Wowpedia on reputation and dailies, RimWorld Wiki,
Elite Dangerous Fandom, Mount & Blade wiki, gamedesignskills.com on chains.

**The gap, stated plainly.** No GDC talk or studio postmortem was found on
*guild quest-hub design as its own topic*. What exists is scattered inside
broader reputation writeups. And the framing/mechanics distinction used below is
a synthesis from what the sources show, not a consensus anybody stated — it is
not quotable as one.

---

## Archetypes we do not have

Our five kinds — fetch, cull, delve, deliver, escort — are three of Doran &
Parberry's ~35 strategies. Fetch and deliver are `get → goto → give`, cull is
`goto → kill`, delve is a nested `goto → explore`. The unused strategies are
the gaps, and each one is listed with the world state it needs, because an
archetype we cannot generate from is not an archetype we have.

| Archetype | World state it needs |
|---|---|
| **Scout** — visit a dangerous place and report | A location that exists and a per-location `visited` bit. No combat, no item. |
| **Guard** — hold somewhere for a while | A location, a spawnable threat, and a **duration** rather than a counter. |
| **Repair** — put something back together | A `damaged` flag on a structure, optionally plus a fetched material. |
| **Rescue** — get somebody out | A lair holding a named person, plus a destination. Pure composition of delve and escort. |
| **Bounty** — bring somebody in alive | A named wanted person, a capture item, and a receiving authority. Needs a "captured ≠ killed" resolution. |
| **Investigation** — follow a chain of people | An explicit `knows(person, fact)` predicate. |
| **Sabotage** — steal from one faction for another | Faction-versus-faction relationship state. |
| **Trade** — move goods between two needs | Settlement inventories, or surpluses and deficits. |
| **Nemesis** — a specific creature that remembers you | Per-creature memory of particular past encounters, and individual promotion. |

Nemesis is the odd one out and worth naming as such: it is the only archetype
in circulation that our "never name something that might not exist" rule does
not forbid and our architecture still cannot support, because it wants
narrative memory rather than an existence check. A quest here is a few indices
and a counter. A monster that remembers being burned last time is not.

**The deadline is the interesting inversion.** Doran & Parberry and CONAN both
note they cannot produce time-oriented quests, because neither models duration.
We can. `quest.Due`, `Expired` and `DueIn` shipped with escorts. On that one
axis the game is ahead of the papers, which is worth knowing before anybody
spends a week reimplementing something we already have.

---

## Not feeling repeated

**Generator-derivable** — no new authored content, just more state in the loop:

- **Write completions back into the world, then generate from the changed
  world.** CONAN's stated goal: the person who asked for wood has used it to
  build something by the time you come back, and asks for the next thing up.
  This is the highest-leverage item on the list *for us specifically*, because
  it is not a subsystem — it is the loop we already run, with the output wired
  to the input.
- **Weight quest selection by who is asking.** CONAN gives each NPC a cost
  weighting over actions, so a baker prefers wealth strategies and a guard
  prefers protection ones, and the same world state produces different quests
  depending on who is standing in front of you. Cheap: a tag or two per person.
- **Nest the kinds we have.** Any atomic action in the grammar can expand into a
  sub-quest instead of resolving trivially — a fetch whose `get` step needs a
  cull first, because the item only drops off something that has to be found.
  Multiplies apparent variety for zero new archetypes.
- **Downweight what the player was offered recently.** CONAN tracks recent
  motivations per player specifically to fight this.

**Needs authored content:**

- **Vary the framing, not the mechanics.** AC Odyssey does this with camera and
  lighting on the same underlying structure. Scaled down for us it is a slot
  grammar over (kind × motivation × relationship) filled from phrase banks —
  cheaper than unique quest text, not free.
- Watch Dogs Legion's ~20 variants per line is the honest price tag on that,
  and it scales with the trait space. Budget for it.
- **Chains and mid-quest complications.** The advice found on this is hobbyist
  tier and is not repeated here as doctrine. The structurally-grounded version
  of the same idea is nesting, above.

---

## Guilds

What shipped games actually get out of them:

- **RimWorld**: quest *availability* is gated by one relationship scalar. No
  separate quests are authored per rank; the same pool is filtered.
- **Elite Dangerous**: a small fixed set of mission templates — structurally
  close to our five — selected and parameterised by each faction's government
  type, economy and security. Identity comes from which templates a faction
  leans on and how it prices them, not from unique mechanics.
- **WoW reputation**: tiers gate recipes, mounts and cosmetics; guild reputation
  is earned from normal play rather than a separate grind. *(Wiki-tier
  sourcing.)*

Minimum viable structure, synthesised rather than cited: a faction, a per-player
counter, an ordered list of rank thresholds. Thresholds unlock bigger counters
and better pay on the **same** archetypes, a board at a settlement we already
generate, and one or two exclusive items at the top. Flavour comes from
weighting which kinds the guild hands out.

**Failure modes, documented:** WoW's Burning Crusade dailies reused a tiny pool
and drew immediate complaints; Blizzard's own fix was to widen the pool, not to
change the mechanic. And "the same quest with a different logo" — three factions
handing you a structurally identical errand with a different portrait on it.
For a world-state generator that risk is *structural*, not incidental, because
the quests are built from one small grammar by construction. The two
generator-derivable techniques above are the documented answer; per-guild
mechanics are not.

---

## Ranking, against what we have

| | Fit for generating from world state | Cost against the five kinds |
|---|---|---|
| Nesting the existing five | Excellent — free variety | Very low |
| Scout | Excellent | Very low; delve's targeting minus the combat |
| Guard | Excellent | Low; cull's spawn plumbing, a duration for a counter |
| Guild rank wrapper | Excellent | Low; a scalar and a threshold table over what we generate |
| Rescue | Good | Low-medium; delve and escort already exist |
| Repair | Good | Low-medium; one predicate plus fetch's gather step |
| Bounty | Good | Medium; a new resolution state |
| Sabotage | Good | Medium; needs faction-vs-faction state |
| Trade | Good | Medium; needs settlement inventories |
| Investigation | Conditional | Medium; needs a "who knows what" predicate |
| Nemesis | Poor | High; per-creature memory we do not keep |

---

## What we decided, 4-5 Sep 2026

**Not attribute bonuses.** *(Jeremy: "agree that's heavy.")* Difficulty is
headcount — that is what the `PARTY` section of `cmd/balance` measured — and a
stat lane outside the levelling curve is a mechanic the simulator cannot see,
which is the one thing this repo has written down as a rule. If it ever
happens it needs a `cmd/balance` section first, not after.

**Faction-unique gear instead.** *(Jeremy: "maybe some gear options that are
unique to the faction/guild could be nice.")* Gear already goes through the
affix and tier machinery the balance pass reads, so a guild-only item is a
reward the arbiter can see. This is the shape a guild reward should take.

**Rival cities and a brigands' guild: filed, not sold.** The idea was two large
cities within a hundred tiles that dislike each other, with misdemeanour quests
aimed at the competitor — steal a cow statue at night, paint a nobleman's wall
— and an "evil" faction to make those choices easier. *(Jeremy: "I'm not sold
on any of this, probably because this is expanding our sandbox without actually
moving the gameplay needle very much.")* That is the right read and it is the
same objection the literature raises about guilds generally: more places to be
is not more game. Written down because the *mischief register* is a genuinely
good fit for this world's voice and is worth reaching for if a faction system
ever does get built — the quests would be the existing kinds with the target
picked from a rival's roster, which is exactly the cheap version above.

**What is being done instead:** playtesting, better arcs, and the companions.
More descriptive story, paging so a box can hold it, and simple dialogue trees.
The report's own conclusion points the same way — the cheapest, highest-value
moves are not new archetypes, they are more feedback in the loop that exists.
