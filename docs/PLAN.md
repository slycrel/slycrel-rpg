# Slycrel — implementation plan

Written 15 Aug 2026, at the end of the session that built the foundation.

## The four decisions everything else hangs off

| | | |
|---|---|---|
| **Engine** | Go + Ebitengine v2 | Lets the combat/levelling maths port straight out of `../new-slycrel`. Everything is code and JSON, so there is no GUI editor in the loop and no binary project file to merge. Ships as one binary. |
| **World** | Ultima-style overworld + zoom-in | A coarse continent you walk across; entering a point of interest loads a detailed local scene. Cheapest way to make a genuinely large world, and the strongest fit for a throwback. |
| **Combat** | Turn-based battle screen | Showcases the bundle's 250 creature portraits and 1,000+ spell icons, and inherits the original's stat maths almost unchanged. |
| **Canon** | Fresh world, old bones | New setting, names and lore built for a bawdy, straight-faced tone; the proven combat, initiative, levelling and disposition maths carried over from 1994. |

## Tone

Bawdy, absurd, and delivered completely straight. The rule the writing follows:
**the game never comments on its own joke.** A rooster is "ungovernable", a
goblin is a "middle manager", a demon is "mid-level" — and the game reports
this in the same flat voice it uses for damage numbers. Over-the-top comes from
the world behaving this way sincerely, not from nudging the player.

All flavour text is data (`data/text/flavor.json` plus per-monster lines), so
the writing can be revised without touching Go.

**It used to call itself 18+, and that has gone.** *(Jeremy's: "we're lucky to
be PG-13 at this point, I think that's sort of misleading on what it actually
is.")* He is right, and the interesting part is that the label was describing
an intention rather than the game. What actually shipped is innuendo, comic
violence and a vending machine — a rating badge on that promises something the
content does not deliver, which is a worse first impression than no badge at
all. The title screen, the README, the distribution read-me and this document
all describe the tone instead of rating it, which they can do accurately.

## Where it stands now

Built and running:

- Title → character creation in two passes: who you are (name, one of ~68
  portraits, one of four walk sheets) and then what you do (three classes,
  rolled stat previews). The look is decoupled from the class, so a fighter can
  walk around in robes
- Overworld: 160×120 generated continent, fractal-noise elevation/moisture/
  temperature, island falloff, downhill rivers, ~40 points of interest placed on
  suitable terrain and spaced apart, roads connecting every settlement to the
  capital, difficulty banded by distance from the capital
- Fog of exploration and a parchment map screen (`M`)
- Random encounters weighted by terrain danger, with a post-fight grace period
- Interiors generated per location kind: walled towns with streets, buildings,
  shops, an inn and townsfolk; dungeons with rooms, corridors, wandering foes,
  chests and a boss; small sites for ruins, towers, shrines, camps and oddities
- Turn-based battle: initiative, targeting, techniques, items, defend, flee,
  monster AI that turtles and bolts when nearly dead, damage floaters, hit
  flashes, screen shake, a paced combat transcript, disposition narration
- Loot, XP, banked level-ups, shops (buy/sell, stock scaling with settlement
  size), inns, chests, altars
- Character sheet and pack with out-of-combat item use
- Quests: four kinds — fetch, cull, delve, deliver — generated from nearby
  locations, biomes and drop tables, with a log, hand-in, and rewards
- Party: up to two hirelings, standing outside inns, rolled at the hero's level
  for a fee and a standing cut of every haul. They act on their own policy —
  the one the balance simulator plays — take a share of the incoming damage,
  level separately, and follow the hero in a line that tracks his path rather
  than his position. The cut is a purse they keep and spend themselves: on
  arriving somewhere with the right counter they buy the next thing they are
  behind on, hand back whatever came off, and their sheet says what they are
  saving for next
- Effects have a side: which half of the field an effect lands on is derived
  from its kind, not stored, so a heal can never be aimed at a monster and a
  stun can never be aimed at a friend. Anything beneficial — a heal, a blessing,
  a revive, a potion out of the pack — can be pointed at any party member, with
  the cursor skipped when there is only one person it could mean
- Part-monster hirelings: six lineages, each with a stat trade-off, a discount
  on the fee because nobody else will take them, and one technique gated on
  ancestry that no hero of any class or level can learn
- Status effects: one timed list per combatant, replacing the four ad-hoc
  mechanisms the battle screen had grown. Poison and burning tick at the end of
  a round, weakness and blessings net against each other, a stun costs a turn.
  Fourteen monsters apply one on a landed hit, the simulator applies and ticks
  them so the balance report stays honest, and antidotes take the harm while
  leaving the help
- Equipment: four slots (weapon, armour, shield, charm) with the effective
  totals read by the dice, and an authored affix table banded by gear tier.
  Affixes are found in chests and offered rather than equipped, since every one
  is a trade. Sidearms sit a band behind the main gear in the "on curve"
  assumption, and are absent from the first band entirely, which is what kept
  the tuned curve intact
- Outfitting the company: the shop counter turns to any party member, supplies
  bought for a hireling go in their own pack and are drunk without asking, and
  everything comes back on dismissal
- Getting back up: a fallen companion is out of the fight rather than dead and
  stands up afterwards, or sooner via an item or Reknit. A hero who falls with
  *anybody* hired is carried to the nearest town for a third of the purse and a
  point of Shame, keeping every yard of progress; a hero who falls alone wakes
  up at the last place they slept, with everything since undone
- Companion backstories: nine authored threads, one per lineage plus three for
  the ordinary hirelings, each a short ordered chain of beats that surfaces
  while you travel together. The writing is authored and the staging is cast
  from the world at the moment somebody is hired, then frozen into the save, so
  a thread can never name a place this continent does not contain. Every one
  ends in a choice rather than a payout, and the choice is a trade: the ending
  that pays is not the ending that settles anything
- Balance: a simulator over the real rules, and a tuning pass that removed a
  damage cliff at level 5, made monsters scale with the encounter, and flattened
  endurance from 12-fights-then-3 to a steady 4-9 per rest
- Icons: 229 indexed across three sets, wired through items, gear and
  techniques, box-reduced to 16px at pipeline time
- Scenery: trees, boulders, ferns, water plants and cacti scattered per
  terrain from a position hash, with a pipeline step that converts Mana Seed's
  baked purple drop shadows into real translucent ones
- Terrain: quarter-tile corner autotiling over Mana Seed ground textures, with
  per-tile texture and dither phase so long boundaries do not repeat; a
  priority order that puts water under sand under soil under grass, with roads
  laid over the lot
- Sound: 33 cues over 81 source files — combat, interface, world and four
  ambience beds — with per-cue variant selection, a persisted volume/mute
  setting, and an audit pass that decodes every file
- Save and load: three slots, a pause menu, and `-load` to boot straight into a
  save. A save is the seed plus what the player changed, so it is ~6 KB of
  mostly readable JSON and doubles as a test fixture. An autosave is written
  before every encounter and offered back to a hero who dies, because the fight
  that killed you was rolled at you rather than chosen
- The matchup axis: monsters have a ward as well as armour, and some attack
  with magic. Constructs and armoured humanoids stop steel and nothing else,
  demons and fey are the reverse, beasts sit in the middle. The player's answer
  is a charm slot they are free to skip — magical attackers do not appear below
  level 10, which is where the counter starts being sold
- The false retreat: a thief's alternative to swinging when both sides are
  nearly finished. Sold less often than a real escape and punished when it is
  not bought, so it is a gamble rather than a better attack
- Equipment is carried, not just worn. Bought, found or taken off, it sits in
  the pack; the character sheet puts it on and swaps whatever comes off back
  into the pack; shops buy it back. Techniques that work outside a fight are in
  the same list
- A home region: within fourteen tiles of the start, an encounter cannot exceed
  the player's own level. The cap lifts as they level, so it retires itself
- Reputation on two axes: Fame is what the deeds are worth, Renown is how well
  the face is known, and they are earned by different things so they come
  apart. Shop prices read the face, hiring fees read the deeds, and
  townspeople open with a line about you rather than their own. The corner
  worth having is high fame and low renown — the stories travel and you do not
- And two more that are the other kind of number: Honor and Faith are banked
  rather than read. Faith goes in the plate and buys anonymity back out of it,
  lifting shame and renown together; Honor is what seeing a companion's story
  through is worth, and it is spent on what the next hireling asks for
- 74 monsters across ten biomes, in five encounter shapes, 27 weapons in five lanes, 19 armours in three
  weights, 14 shields in three lanes and 5 caster talismans, 12 charms,
  10 affixes, 46 items,
  35 techniques
  (thirty of them party-facing, lingering, aimed at everything, two-sided, or
  gated on a hireling's ancestry),
  9 companion backstories and 4 for the people who stay put
- A sky: a clock measured in steps, and weather derived from the seed rather
  than stored. Night is dark and hunting, bad weather is dark and quiet, and
  the dangerous night is the clear one. A bed buys the morning
- Asset pipeline: inventory, selective extraction, manifest generation, and an
  audit that reports which art keys still fall back to placeholders

Everything on the roadmap is built. What is left is playing it and
finding out what it needs — the compass below is the first thing that came
back that way.

Played twice end to end. The second pass is what turned up the home region, the
starting kit, equipment as inventory, and four separate cases of the game
knowing something and not saying it — what a shop item does, what a chest just
gave you, which keys exist, and which build is running.

## Architecture, and why

**Scene stack, not a state enum.** A shop, a battle, a message box, and the map
each push themselves over whatever was underneath and pop back. No screen needs
to know who called it. Overlay screens draw the scene beneath them, so a battle
gets a dimmed backdrop of the place it started.

**Everything generated takes an `*RNG`, never a global.** `core.RNG.Fork(label,
salt)` derives child streams deterministically, which is what lets a point of
interest regenerate its own interior from `poi.Seed` on every visit instead of
being stored. A seed reproduces the continent, the towns, and the jokes. Fork
derives its stream from the label and the salt *only* and never reads the
parent's state, so the salt has to carry whatever should vary — `poi.Seed` for
an interior, the run's seed for anything cast per-run.

**Art never blocks the build.** `assetsys` falls back to a generated
placeholder for any key it cannot resolve — dithered pixel-art tiles for
terrain, a magenta marker for anything else. The manifest points either at
curated files under `assets/` or straight into `assets-raw/`, so curation is
incremental and reversible.

**The maths is pure.** `internal/rules` has no I/O and no globals, so balance is
testable and a simulation harness is cheap to add later.

**Domain logic stays out of the scene layer.** Ebitengine opens a window at
package init on macOS, so anything that imports it — directly or through
`assetsys` — cannot run without a display. `internal/game` is the scene stack
and the drawing, and everything that is neither lives outside it: the roster and
the marching order in `internal/party`, the tile walker and `TileSize` in
`internal/core`. This started as a way to keep working through a flaky display,
but it is the right shape regardless: `game` shrank by three hundred lines and
the party rules, which had already produced two bugs found by reading rather
than by tests, became something a test can reach.

## Roadmap

### Phase 0 — what playing it keeps finding

Both playthroughs found the same *kind* of thing: not missing features, but
features the game had and would not talk about. The pause menu had Save behind
a switch that dispatched on row number while the rows had moved; the quest log
had a key nobody advertised; a chest listed names with no idea what they were
for; the shop was two columns of prices. None of it needed building. All of it
needed saying.

The lesson is cheap to state and easy to forget: **when something is reported
missing, check first whether it is merely unreachable.** Four of the six items
from the second session were that, and the fix each time was a sentence rather
than a system.

### Phase 1 — make it feel like a game *(done: save/load, audio, quests, party)*

### Phase 2 — depth

1. ~~**Party, fourth pass.**~~ *(Built.)* What was left after outfitting:
   letting a companion spend their own cut rather than only re-arming on
   level-up, and giving them an opinion about the gear you hand them.

   **The cut was a subtraction and nothing else.** A percentage came off every
   haul, left the purse and went nowhere at all, while the companion re-armed
   for free into the full on-curve kit on every level-up. Those are two halves
   of one idea that never met: the money and the gear had no relationship, so
   neither number could be read against the other, and a player could not tell
   whether the standing charge on everything they found was expensive because
   nothing on the screen was its other half.

   So the cut is a purse they carry (`model.Character.Coins`, which allies
   already had and Recruit already zeroed, so the zero in every old save is the
   honest answer). `gamedata.Wants` says what they are behind on and
   `gamedata.Shop` spends it, one piece at a time, on arriving somewhere with a
   counter. Four things fell out of building it.

   - **The target is `Equip`, not a second opinion about it.** "On curve" has
     exactly one definition in this game and it is load-bearing enough to have
     its own test; a companion's kit is now a lagging indicator of that rather
     than a copy of the rule. `TestACompanionCatchesUpToTheCurveAndStopsThere`
     holds both ends — they converge on it, and they do not buy past it.
   - **They compare price tags, not stats.** It is the only comparison that
     works in all four slots: a charm has no better, only dearer, which is the
     rule `TestTheShelfNeverGradesACharm` already holds at the counter. It is
     also what somebody saving up actually does.
   - **A village has a smith and no armourer**, so a companion who walked out
     of one wearing a breastplate would be the game naming something that was
     not there. Which counters are open is read off the map `BuildLocal` just
     built rather than off the kind of settlement, because a second copy of
     that rule is a second copy to drift.
   - **What comes off is the employer's**, into the hero's pack, exactly as
     every other route gear takes. It is the honest answer to who paid for the
     replacement, and it stops an upgrade being a cost with nothing coming back.

   **And the report grew the half it had been refusing to measure.** THE
   COMPANY'S SHARE said in as many words that converting a cut to coins would
   need a model of what a haul is worth, and inventing one is how a report
   starts measuring a fiction. It is not invented now — a haul is `CoinAward`
   plus what `RollLoot` fetches at `rules.SellPrice`, rolled off the same
   `PickMonsters` the game rolls, at the group sizes a company draws. (Which
   moved `sellRate` out of the shop screen and into `rules`, because the
   arbiter pricing a haul at a different rate from the counter would be worse
   than not pricing it.)

   What WHAT THE CUT BUYS says is that the cut covers 8%, 24%, 38% and 64% of
   the next tier's kit across the four bands. That is the finding, and it is a
   shape rather than a shortfall: a sidearm band is a quarter of a main one, so
   the cut keeps an arm and a charm current by itself and chips at the rest,
   while the drops column — what the same fights put in the *hero's* hands — is
   larger than the cut at every tier. The two are a division of labour. The cut
   buys the cheap slots slowly; the main slots are hand-me-downs, which is why
   the sheet says what they are saving for. It is a shopping list addressed to
   the person holding the pack.

   The column climbing the whole way is the half that has to hold. A share that
   bought a smaller fraction of the shelf every tier would be a mercenary
   getting relatively poorer the longer they worked for you.

   **The opinion turned out to be two sentences, and the second one is the
   feature.** What a companion says about a gift is flavour — three banks in
   `flavor.json`, keyed on whether it was the thing they were saving for,
   something dearer than they had, or neither. What follows it is not: the line
   that names where the cut is aimed *now*. A gift is the one way a player can
   steer a companion's spending, so the useful thing to say is not "thank you",
   it is "the cut goes toward armour now".

   Two notes from the screen. The sheet says the *slot* rather than the item —
   naming it came out as "saving for Staff of the." at the width the portrait
   leaves, and the slot is the more useful half anyway, since what the player
   does with it is go to the pack. And the transcript says slots too, for a
   harder reason: the walking-around screen shows exactly one rendered row of
   the log, so a sentence over about sixty characters appears on screen as its
   own second half. The purchase line is written last for the same reason —
   the housekeeping note about where the old coat went was winning the only
   visible row.

   **A frame caught the sheet overflowing, and it had been for a while.** Eight
   stat rows is reachable — a caster with an ancestry, a story, something in
   their pack and now a purse — and the charm row was printing through the
   bottom of the frame into the hint underneath. The panel is 232 now and the
   footer moved with it, counted rather than eyeballed: text drawn at `y` inks
   to `y+12`, the gear rows are the last four slots, so the frame has to end at
   least fourteen below the last of them.

   A resurrecting NPC was considered here and deliberately left out. With the
   fallen standing up when a fight ends and a fallen hero carried to town by
   the company, there is no state a run can reach where somebody stays dead and
   needs paying for — so a healer at a counter would be a door that never
   opens. The apothecary stocks the revive items instead, which is the same
   idea placed where it does something. If death ever gets harsher, this is the
   first thing to revisit.
2. ~~**Companion backstories.**~~ *(built — `internal/thread`.)* The shape the
   plan guessed at is the shape it took: an authored skeleton (a beat list with
   roles) cast from whatever the seed generated at first contact and then frozen
   into the save. Two details only turned up in the building.

   The first is that which roles a skeleton needs is read out of its own
   writing. Putting `{X}` in a line is the whole of adding an antagonist, so
   there is no second list to forget to update — and the test that catches an
   unfilled placeholder is a scan for a surviving `{`, which also catches a
   typo nobody implements.

   The second is that a counted beat has to be cast from where the company is
   *now*, not from the place the thread ends at. The counted beats fire in the
   stretch before the story has even named a destination, so an antagonist
   drawn from the far end is one the player has no reason to be near, and the
   thread stalls on "put down three of them" for the rest of the run.

3. ~~**NPC backstories.**~~ *(Built.)* The same mechanism for the townspeople
   who used to have one line each. The prediction here was right about the
   machinery — `internal/thread` already had the casting, the beat chain and
   the choice at the end, and all that was missing was an address — and wrong
   about the interesting part. It is not "closer to how a quest works". It is a
   different *rhythm*, and that rhythm is the whole feature.

   A companion's thread is told **while it happens**, because they are standing
   next to you. A resident's is told **in installments on your return**, because
   they were not there for any of it. So `Advance` parks a resident's beat in
   `Owed` rather than firing it, one at a time — a resident holding something
   they have not had the chance to say does not start accumulating the next
   thing. Go away for a month and you get one conversation, not the whole story
   at once. What somebody who cannot follow you has to offer is the shape of a
   serial, and that turned out to be worth more than the location key.

   `Return` is the trigger only they can use: `Town` narrowed to the one
   settlement they are actually standing in. Four skeletons, marked `resident`
   in the same `threads.json` and cast from a separate pool by `CastResident`,
   because writing for somebody behind a counter reads as nonsense out of a
   hireling's mouth.

   The opening beat fires on casting rather than waiting for a trigger. A
   resident is cast the first time you speak to them, so without that the first
   conversation would be them reciting the journal note for a story they have
   not told you yet — "come back when you have killed four of those", from
   somebody who has not said hello.

   Two zero-value traps, both found by tests rather than by play. Residency was
   first derived from `HomePOI >= 0`, which made every thread in every save
   written before this existed a resident of whichever location happens to be
   first on the map; it reads off the skeleton now, which is the one place the
   answer is authored. And `Log.Drop`, `Log.For` and `ForResident` all key on a
   name — names are unique inside a company and not across a continent, so a
   hireling and a shopkeeper can both be Marta, and letting the hireling go was
   taking the shopkeeper's story with them.

   Gated three ways: this person is the storyteller (a stable hash of where
   they stand, like the errand giver's, on a different salt so the two are not
   the same roll wearing different thresholds), one per settlement, two running
   at once. Across four continents that puts somebody in 43 of 56 towns while
   the player carries at most two — which is the balance worth having, since a
   town nobody in it has anything going on is the state this was fixing.

### Phase 3 — the world reacting

4. **Alternative progression arcs.** *(Jeremy's idea; acknowledged as a rabbit
   hole, so scoped deliberately.)* Right now there is one way to be correctly
   levelled, and `gamedata.Equip` is where it is written down: best weapon and
   armour of your tier, sidearms a band behind. The balance report measures that
   one build, so "on curve" and "the way we expect you to play" are the same
   sentence — and the equipment pass showed how load-bearing that is, since
   changing the assumption moved the whole report without a single stat
   changing.

   The goal is two or three builds that are each viable rather than one that is
   correct: something like an attrition build (heavy armour, a shield, low
   damage, long fights), a glass cannon (best weapon, charms that trade defence
   for strike, fights that end in two rounds or badly), and a company build
   (cheaper personal gear, coin spent on hirelings and their supplies).

   The mechanism is the same shape as what exists: `Equip` becomes several named
   archetypes, and the report grows a column per archetype instead of one set of
   numbers.

   **The report half is built, and it has answered.** `gamedata.Archetypes` holds
   three builds — balanced (the original assumption, unchanged, and still what
   `Equip` means), attrition, and duelist — and `cmd/balance` gained an ARCS
   section that runs the same simulation over each, plus a WHY table underneath
   it. What it says:

   - **All three builds are live.** Balanced wins the stretch fights at 3
     levels, attrition at 2, duelist at 2, and the widest gap at any level is
     6.6 points. Attrition owns the early game, balanced the top end.
   - Getting there took three fixes, each found by the report and each
     measured on its own. The weapon band steps ran +2, +5, +5, +4, so at
     tier 3 attrition was buying a +1 shield step with a -5 weapon step; the
     tops are now 5, 9, 13, 17, 21, even throughout and with the same
     endpoints, held by `TestWeaponBandsStepEvenly`. Four levels had no
     monster in the biome they are sent to. And the stretch probe was rolling
     the local region, which mostly could not supply the fight it asked for.
     Worst gap went 13.5 → 7.9 → 6.6 across the three.
   - Armour steps are still uneven (+3, +4, +6, +4) and deliberately left
     alone: nothing in the report blames them for anything yet, and pinning a
     rule that has not earned itself is how a table ends up shaped by its
     tests rather than by play.
   - A sidearm band is worth about a quarter of a main-gear band, which caps
     how different two builds can be. That is authored, not accidental:
     `TestShieldsStaySecondaryToArmour` holds a shield under half the body
     armour of its own band on purpose. Widening the arcs means revisiting
     that rule deliberately, not inflating the shield table until it goes red.
   - A glass cannon cannot be built at all. Nothing outside the weapon slot
     adds strike, so the most offensive build the tables allow is "best weapon
     available", which every build already has. If the offensive arc matters,
     that gap is the thing to fill.

   Two lessons from doing it, both of which cost a wrong answer first. An
   archetype that underspends measures the spec rather than the content — the
   first draft of the duelist gave up an armour band *and* the shield to buy one
   charm band and lost by 22 points, which said nothing about anything. And the
   on-level column cannot discriminate: every build wins 96-100% of on-level
   fights by design, so the comparison has to be made on the stretch fights.

   Still open, and the expensive half: making the content actively support the
   arcs. The report now says what would have to move, which is what it was for.

   **And the first thing it said had to move was the baseline itself.** A
   survey of what was unfinished turned this up, and it is the most expensive
   kind of bug: not a wrong number, a right number measured against a wrong
   assumption.

   The fourth archetype, `warden`, was balanced with the silvered shield
   instead of the wall and nothing else changed — same slots, same bands, same
   spend to the coin at every level. It was added to ask whether the ward lane
   was worth anything. The answer, at six times the usual sample:

   | level | wall | silvered | |
   |---|---|---|---|
   | 1 | 86.5% | 86.0% | wall +0.5 |
   | 3 | 68.0% | 67.1% | wall +0.9 |
   | 5 | 79.9% | 79.6% | wall +0.3 |
   | 7 | 91.4% | 92.8% | **silvered +1.4** |
   | 9 | 75.6% | 77.8% | **silvered +2.2** |
   | 11 | 57.0% | 62.5% | **silvered +5.5** |
   | 13 | 50.6% | 61.8% | **silvered +11.2** |

   That is not a trade. It is a trade with one side worth ten times the other:
   the wall's best level is worth nine tenths of a point, the silvered
   shield's is worth eleven. And the crossover lands exactly where the content
   says it should — nothing casts below level ten and half of what lands on
   you by thirteen is magical — so **the tables were never the problem**. What
   was wrong was that `Equip` picked the wall, and picked it for the least
   defensible reason available: `ArmBlock` is the zero value of
   `Archetype.Arm`, and nobody had ever set the field.

   The cost of that was not confined to the report. `Equip` is what dresses
   every hireling, every save fixture and — since the party pass — every
   companion's shopping list. Every companion in the game above level six was
   being handed the strictly worse off-arm, for free, forever.

   So: `balanced` carries `ArmByLevel`, `gamedata.LaneForLevel` answers it, and
   the crossover is a named constant at level 6. `warden` is retired, because
   the right response to an archetype that beats the baseline at identical
   spend is not to keep it in the table, it is to stop the baseline making that
   mistake.

   **The measurement is permanent now, in its own LANES section**, and that is
   the part worth keeping. A crossover justified by a retired archetype is a
   number nobody can check; LANES runs all three lanes against each other at
   every level, with the cost column printed to prove the spend is identical,
   and prints the crossover it measures beside the level `Equip` actually
   switches at — with a WARNING when they drift apart.

   **And the instrument immediately corrected the finding that motivated it,
   which is the best argument for building instruments.** Two columns said
   "the silvered shield beats the wall". Three columns say something sharper
   and less flattering:

   - **Below level six the lane does not matter at all.** All three come in
     within a point of each other and the sign of the gap flips from level to
     level. The wall was not a wrong choice down there; it was not a choice.
   - **From level six the wall is the worst of the three and never recovers.**
     Not second of two — worst of three, trailing the best lane by 1.0, then
     2.7, 4.5, 2.4, 5.0, 7.0, 5.7, 11.1, 6.5.
   - **Spiked and silvered are close enough to each other to be a coin flip.**
     Spiked takes 7, 10 and 11; silvered takes 8, 9, 12 and 13; they tie at
     14. So the numbers do not pick between them, which means the principle
     has to: a baseline must not have an opinion about offence, and spiked
     trades guard for strike. The build that makes that trade properly is the
     duelist, and it already exists. Silvered it is.

   The other correction was to the section's own arithmetic. Its first draft
   read the crossover off the first row where the silvered column happened to
   be higher, and duly reported **level one** — off a 0.7-point wobble in a
   band where nothing casts. It takes a named threshold now (`laneNoise`, one
   point, which is above the observed flapping below six) and asks for the
   first level from which the wall never again comes within it. That answers
   six, which is what the constant says, and the two are now checked against
   each other on every run rather than agreed with once.

   **Every number in the four paragraphs above is inside the noise, and the
   threshold in the last one is the reason.** `laneNoise` was set at one point
   because that is "above the observed flapping below six" — but the flapping
   below six *is* the measurement of the noise, and reading a threshold off it
   by eye put it four times too low. The rows where all three lanes dress the
   character identically spread by up to 4.3 points. So "the wall trails by
   1.0, then 2.7, 4.5..." is a list of coin flips, "spiked and silvered are a
   coin flip" was true for the wrong reason, and the crossover at six was read
   off nothing. Retiring the warden was still right; it just was not winning
   for the reason it was credited with. See "The fifth zero value, and the
   threshold nobody measured" below, which rebuilt LANES per class, on two
   axes, with the floor taken off those identical rows — and moved the
   constant to level ten and the spiked lane.

   The lesson generalises, and it is the same one the equipment pass learned
   the first time: **`gamedata.Equip` is an assumption, and an assumption that
   has never been compared with its alternatives is a guess with a test around
   it.** Three of its four slots had been argued over in this document. The off
   arm had a zero value.

   **And then the fourth slot turned out to be worse.** Looking for the same
   defect elsewhere found the charm slot picking `cs[len(cs)-1]` — the last row
   of `charms.json`. Not a zero value this time but a defended one, and the
   defence was a claim about the content: every charm gives with one hand and
   takes with the other, so there is no better one, so any pick is as good as
   any other. That claim has a test behind it at the shop counter
   (`TestTheShelfNeverGradesACharm`, which refuses to mark a charm green on
   exactly that basis) and it is the reason nobody had ever looked.

   It is measurable, and it is wrong. Measured per class on the stretch fights
   and again on fights-per-rest:

   | band | winner | what `Equip` wore | worst case |
   |---|---|---|---|
   | tier 1 | Heavy Knucklebone | Heavy Knucklebone | — |
   | tier 2 | Cracked Spectacles | Courier's Anklet | −2.1 |
   | tier 3 | Saint's Fingerbone | **The Quiet Stone** | −12.5 (Thief, level 11) |
   | tier 4 | Earplugs of the Unmoved | **Somebody Else's Medal** | −5.8 |

   File order landed on the loser in three bands out of four. At level eleven a
   Thief was wearing the worst charm on the shelf: 12.5 points of win rate and
   a third of its fights per rest, given away for nothing, to every hireling as
   well as to the balanced build.

   The obvious objection is that a single fight cannot see a psyche charm,
   since psyche is the currency of the *next* fight — so the measurement runs
   an endurance column too, chains of fights until something drops. It says
   the same thing, for every class including the Mage: The Quiet Stone's four
   points of pool are worth 7.0 fights per rest against Saint's Fingerbone's
   9.1. Four points of pool does not buy a fight. Six points of ward does.

   So `gamedata.CharmValue` scores the trade and `Equip` acts on it, and the
   **CHARMS** section re-derives the ranking on every run and warns when the
   scoring and the fights disagree — the same contract LANES has. The weights
   are read off the measurement rather than reasoned out, and the ordering they
   produce matches the measured ordering in all five bands.

   **And then the table itself turned out to be measuring the wrong thing.**
   The plan had already written the rule down — *an archetype that underspends
   measures the spec, not the content; read the cost column before believing a
   gap* — and ARCS had been failing it at every level since it was written:

   | level | balanced | attrition | duelist |
   |---|---|---|---|
   | 1 | 54 | 74 (**+37%**) | 74 (**+37%**) |
   | 7 | 500 | 448 (−10%) | 575 (**+15%**) |
   | 11 | 1140 | 1045 (−8%) | 1310 (**+15%**) |
   | 13 | 2220 | 2260 (+2%) | 2600 (**+17%**) |

   The duelist carried fifteen to eighteen per cent more gear than the baseline
   at every level from five up and duly won more levels. Both rivals outspent
   balanced by 37% at level one and both beat it. None of that was a fact about
   a build. There is no point authoring content to support three arcs while the
   instrument cannot tell a build from a budget.

   So every build shops with the same purse now — what balanced costs *that
   class* at that level, per class because a Thief's on-curve kit is about a
   tenth cheaper than a Fighter's. `Tables.EquipWithin` fits a shape to a
   budget: bands give way in the order the tables are already described in
   (flourishes, then the coat, then the weapon last, since a build that sold
   its weapon has stopped being itself), and leftover money goes into the
   sidearms only, because letting spare coin buy a better sword would turn
   "attrition with money left over" into "balanced".

   **At equal spend the answer changed, and then an adversarial review showed
   the new answer was also wrong.** Both readings came out of the same defect
   in the instrument, one level up from the ones being hunted: ARCS averaged
   its three classes into one row, and an average is not a measurement of
   anything that exists.

   Only a Fighter may hold a two-handed weapon. So the "duelist" row averaged
   one real duelist, one build that falls back to a one-hander (the Thief), and
   — for the Mage, who cannot hold one at all — very nearly balanced. Measured
   per class at level thirteen:

   | class | balanced | duelist | duelist's spend |
   |---|---|---|---|
   | Fighter | 69.6% | **75.3%** | 88% |
   | Thief | **72.8%** | 71.5% | 100% |
   | Mage | **62.1%** | 59.3% | 100% |

   The Fighter duelist beats the baseline by 5.7 points *while spending twelve
   per cent less* — which is the exact criterion `warden` was retired for — and
   the mean of that and the two losses is "within a point or two", which is
   what the table printed and what this document concluded from. The row is
   per class now.

   **And the review found the fourth zero value, inside that same row.**
   `duelist` never set `Arm`, so on the classes that cannot make its trade the
   fallback one-hander leaves the arm free and fills it with `ArmBlock` — the
   wall, at the levels LANES says the wall is worst. Fourth instance of the
   same defect, in the build the headline conclusion was read off, found by a
   reviewer after three rounds of this document congratulating itself for
   spotting the pattern.

   Two other things the review was right about, both the same shape as
   `reportSlotValue`'s charm column — prose claiming what the code does not
   measure:

   - **The verdict never read the endurance column.** The per-rest figure was
     stored and printed while `wins` and `worstGap` were computed from win rate
     alone, so the commit that said "it loses the fight and wins the afternoon,
     and the verdict moved" was describing a flip that actually required
     attrition to top a *win-rate* level. It counts both axes now, with a named
     threshold, because LANES learned in the same session that reading a winner
     off whichever column is higher reports noise as a result — and ARCS was
     still doing it.
   - **The spend column printed one class of three.** `budgetFor` was called
     with the last of `AllClasses` and `cost` was whatever survived the last
     loop, so both described the Mage. "Attrition is the one build that cannot
     spend its purse" was therefore false: at level thirteen the *Fighter
     duelist* comes in at 88%, worse than any attrition row.

   What survives all that: every build now takes cells on both axes — balanced
   1 fight and 7 rests, attrition 1 and 5, duelist 2 and 4 — so the answer to
   the roadmap item is still yes, there is more than one way to be correctly
   levelled. What does not survive is the reason given for it. The arcs are
   real *per class*, and they are different arcs for different classes, which
   is a more interesting answer than the one the averages allowed.

   Two smaller things fell out. `EquipWithin`'s first draft assumed a two-handed
   build has no off arm, which is what `Hands: 2` says — but the arm is closed
   by the weapon actually obtained, and a Mage cannot hold a two-hander, so
   three classes' worth of level-one duelists carried a shield the fitter could
   not take off, eight coins over a purse of sixty-six. The archetype's own
   comment had recorded that subtlety years earlier and the fitter still walked
   into it. And the WHY table's charm column was reading
   `cs[len(cs)-1].Bonus.Defense` — the last row of the file, on the one axis
   charms mostly do not carry — printing `0, 0, 1, 2` for the life of the
   report. It reads `1.0, 3.8, 2.6, 4.8` in CharmValue units now. The arbiter
   had the same bug as the thing it was arbitrating, in two slots out of four.

   **The deeper finding is a content one, and it is left open deliberately.**
   Two bands have a right answer rather than a choice — 6.0 and 5.4 points
   between best and worst, on both axes at once — which is the premise of
   `TestTheShelfNeverGradesACharm` failing in the arbiter while passing in the
   test suite. The cause is not that ward is overpowered: it is that the ward
   charms carry six to fourteen points of their stat where their rivals carry
   one to four of theirs. `Forged Guild Licence` is worth 1.4 against
   `Saint's Fingerbone`'s 5.0 and they sit on the same shelf at the same tier.

   The brief for the pass that fixes it, written down while the numbers are in
   hand: **a band should offer charms that win on different axes**, not three
   charms of which one wins both. Tier three currently has a fight charm, a
   worse fight charm, and a charm for an axis the game does not reward. What it
   wants is one that takes the fight, one that takes the afternoon, and one
   that is cheap — and the magnitudes have to be comparable, which today they
   are not. Doing that is authoring, and doing it against `CharmValue` rather
   than against the fights would be shaping the table to its own scoring
   function, which is the failure mode this whole section exists to avoid.

   **Found on the way, and fixed: the report had been understating danger since
   it was written.** Two separate causes, both invisible except as a level that
   looked suspiciously comfortable.

   Four levels were sent to a biome with nothing to fight at their level —
   `biomeForLevel` puts 7-8 in the swamp and the swamp had no level 7 or 8
   monsters, so `PickMonsters` fell back and the band was spent beating up
   juniors. Four monsters filled them, and level 7 fighter endurance came back
   from ten fights per rest to nine.

   Worse, the "three levels over" column — the report's only measure of what
   wandering costs — rolled the *local* biome at level+3, which eight of
   fourteen levels cannot supply. It now rolls the region three levels further
   out, which is what straying means in a world where danger radiates outward.
   The column fell by up to thirty points (level 2 fighter, 91.5% → 59.1%).
   Nothing about the game changed; only what the arbiter was looking at.

   The report now checks both conditions itself and prints where it is lying.
   The only remaining short probes are levels 12-14, where mountain is already
   the outermost region and there is nowhere further to stray.

5. ~~**Fleeing should pay something, and the thief should be the one it pays.**~~
   *(Built — the feint, the routed monster's purse, and the thief's two perks;
   see below.)* *(Jeremy's, in response to the flee work: "fake flee + backstab
   might be a way around 0 rewards at the cost of survivability; having to run
   away might not be enjoyable.")*

   The point stands and it is a hole in what was just built. Teaching the
   simulator to run fixed a measurement — the thief's speed was invisible and
   it was being scored on how well it dies in fights it would never have
   finished — but it does not make running *fun*. A retreat currently pays
   nothing at all: no experience, no coin, no drop, and the fight was still
   fought. The class whose survival plan is leaving is the class whose plan is
   to come away empty.

   The proposal: a thief manoeuvre that spends the escape rather than taking
   it. Feign the rout, and if it lands you get a strike at backstab damage
   against a creature that has stopped defending; if it does not, you have
   spent your round pretending and the thing opposite gets a free one. So the
   trade is the reward you would have forfeited against the survivability you
   would have banked, chosen in the moment rather than by class.

   *(Built.)* `rules.CanFeint` / `FeintChance` / `FeintDamage` / `FeintPunish`,
   offered in the battle menu under Flee to thieves from level 4, and modelled
   in `SimulateFight` so the report can see it.

   Three things came out of building it. It is not an alternative to running —
   that was the first model and it barely fired, because `wantsOut` only
   triggers while the other thing is above half health and a false retreat is
   only worth selling when it is nearly dead. It is an alternative to
   *swinging*: the gambit when you are nearly out of hit points and so is the
   thing opposite. Second, the decision has to read the target's armour, since
   `PlayerDamage` subtracts Defense before the bonus multiplies anything.
   Third, a simulator that took every gamble on offer reported the trick as a
   straight downgrade, so the policy has a floor — the move arrives when the
   thief does and becomes worth using when their dexterity does.

   The rule "worse than fleeing on average" holds where it matters and has one
   deliberate exception: against something faster than you, `FleeChance` floors
   out and lying is the better bet. That niche is the point of the move.

   *(Also built, and it took a second pass at what "fleeing pays nothing" meant.)*
   The feint answered the thief's half. It did not answer the plainer version
   of the same complaint, which came back from play as "monsters running away
   is lame at levels 1-3" — and the lameness was never that they ran, it was
   that a *routed monster paid nothing*. Something only bolts below fifteen
   percent health, so by then the player has done nearly all of the work, and
   at levels one to three an encounter is one or two creatures, so the whole
   thing could evaporate on a coin flip the player had no part in.

   A routed monster now pays: coins in full, because something running is not
   stopping to pick anything up, and half the experience, because you did not
   finish it. The drop table stays on kills — a runner is wearing what it was
   wearing. And the last one standing bolts at a third of the usual rate, on
   the grounds that it has nobody left to run behind. That last part is the
   half the simulator can see, and it moved: win rates fall by up to two and a
   half points at the top of the table, which is monsters that would have left
   now staying and swinging.

   `model.Monster.Dead` already meant "out of the fight" and was being read as
   "killed", which is exactly why a runner was worth nothing. It has `Fled`
   beside it now.

   **Jeremy's two-for-one is built.** The thief pays for one restorative and
   leaves with two — heal, revive and cure, not the whole shelf. That is not a
   flourish, it is the right shape for the actual constraint: the thief has
   *no* healing technique at all (the shared-looking ones in `spells.json` are
   blood-gated hireling moves, not class-agnostic), its only sustain is two
   drains, and its recovery is therefore entirely items. A discount on them is
   the class's real defensive stat.

   His framing beat the version this would otherwise have shipped as. A
   percentage off the sticker is a number nobody notices; walking out with two
   is a thing you watch happen. It is quoted in the shop row *before* the
   purchase — "36 for two" where everybody else reads "36" — because a perk the
   game will not talk about is a perk nobody knows they have.

   The second half is the two features meeting: a thief also gets the drop
   table off the creatures that *ran*, on the grounds that something turning to
   run past you is least careful about what it is holding. What that does to
   the game is better than the loot is — a fight that ends with the last thing
   bolting is a disappointment to everybody else and a payday to the thief.
   Compensating a class by changing how it reads an event beats adding a number
   to it.

   Neither perk is a new attack, which is Jeremy's framing and the right one:
   the interesting thief is "the monster cannot hurt you if it is not there",
   not "the thief also hits hard".

   `cmd/balance` grew a SUPPLIES section, because this one lives entirely on
   the buying side where the fight simulator cannot see it — ENDURANCE says "no
   potions" in its own header, and teaching `SimulateFight` to drink would
   re-tune every endurance number in the report to answer a question about
   prices. So it is measured where it happens: cost per point of effect, with
   and without the thief. The psyche and buff rows are identical in both
   columns, which is the table saying out loud that the perk is sustain rather
   than shopping.

6. ~~**Day/night and weather.**~~ *(Built.)* `internal/sky`, pure and headless,
   holding a clock and a derivation of what the weather is doing.

   The design is one idea, and it is what makes the feature worth playing
   rather than worth looking at: **night and weather pull in opposite
   directions.** Night is dark and hunting — you see less and more things want
   you, and what turns up is a level meaner. Bad weather is dark and *quiet* —
   you see less and nothing else wants to be out in it either. So the dangerous
   night is the clear one, a storm is cover, and the two compose into four
   kinds of evening rather than one slider from good to bad. A table where
   every row moved the same way would mean the correct play is always "wait for
   a clear noon", and there would be nothing to read in the sky.

   Ignorable, and progressively more expensive to keep ignoring, which is the
   brief. A bed at an inn buys the morning — `Clock.WakeAt(Dawn)`, always
   forwards — so night is a thing you can pay to skip at a price that already
   scales with your level. Nothing is gated on the clock anywhere.

   Time is measured in **steps**, not frames. A clock on wall time would
   advance while somebody read a shop menu, and standing still would be a way
   to wait out the night — which is the opposite of what the inn is for. Steps
   indoors count too, or a shop would be a place to stop time.

   Weather is **derived, not stored**: `sky.At(seed, clock, biome)`, the same
   trick the scenery uses. A seed reproduces its weather exactly, walking from
   forest into mountain turns rain into snow with nothing tracking a boundary,
   and there is no second copy in the save to disagree with the world it
   belongs to. The clock is the only new field in a save file, and its absence
   in an older one reads as the first dawn of the run — the only answer an old
   save can honestly give.

   `LevelShift` is the load-bearing number: one level, at night, applied
   *before* the home region clamps the roll to the player's own level, so the
   ground around the capital stays predictable after dark. It is one level
   because the DANGER table already spends four hundred thousand fights
   establishing what one level over costs — which is how this was added without
   re-tuning anything. The new SKY section in `cmd/balance` states the
   multipliers rather than re-simulating them, because two of the three terms
   are already measured and the third (how often a step becomes a fight at all)
   is not simulated anywhere: `SimulateFight` begins once a fight exists.

   "Which NPCs are out" turned out to be: the people, not the buildings.
   Townsfolk and the hopefuls loitering outside inns go home after dusk;
   counters, beds, altars, chests and anything with teeth stay put. A merchant
   will always take money and a dungeon does not keep hours. It is never a dead
   end — arrive at midnight with an errand to hand in, take a room, do it in
   the morning — which turns "the town is asleep" from an obstruction into the
   reason the bed exists.

   Two drawing notes worth keeping. The tint is a **multiply** over the
   finished frame (`render.Multiply`), not a translucent rectangle: a
   rectangle washes the image toward the colour, lifting the blacks and
   flattening the contrast, where multiplying keeps the darks dark and moves
   the rest — which is what a change in the light actually does. And it goes
   over the world but *under* the HUD, so the status bar stays legible at
   midnight. The rain sheet is 32 pixels wide, so tiling it straight across put
   fifteen identical columns on screen in lockstep and read as vertical
   stripes; every column runs a different frame now, on strides of three and
   five so the rows do not fall back into step either.

   The status bar says "night, rain" in three words, because none of this is
   discoverable from a tint. Clear weather says only the time — "day, clear" is
   three quarters of a run's status bar spent saying nothing.
7. ~~**Faction and reputation.**~~ *(built.)* Reputation is two numbers now,
   which was Jeremy's design and is the whole of why it works: `Fame` is what
   the deeds are worth and `Renown` is how well the face is known. They are
   earned by different things — deeds by errands, levels and backstory endings;
   renown by being *seen*, which means walking into a town for the first time,
   hiring somebody outside an inn, or being carried home through the gate — so
   they come apart, and the corners where they disagree are the point.

   `rules.Read` turns the pair into a standing, and three things read it. A
   shopkeeper marks up the face they recognise, which is not the same as
   thinking well of it, so the legend nobody has placed pays the sticker price
   and the celebrity pays for the privilege — that is the Robin Hood corner
   paying out. A mercenary asks the opposite question, so being a name is a
   discount at the inn and a markup at the counter, and being notorious is
   hazard pay one way and a surcharge the other. And townspeople open with a
   line about you rather than their own, some of the time — some, because a
   town where everybody comments on you has stopped being a place and started
   being a mirror.

   The character sheet says both numbers, and the faint line under the portrait
   names the corner and explains it in one sentence: "A rumour: the stories
   travel, not you."

   `Honor` and `Faith` are live too, and the split that made them worth keeping
   is that they are a *different kind of number*. Fame, Renown and Shame are
   what the world reads — you cannot pay them down, only outweigh them. Honor
   and Faith are what you banked on purpose, and the only interesting question
   about a banked number is what you traded it for. Four numbers on a sheet is
   three too many if they all do the same job; two ledgers with two jobs is a
   reason for each of them.

   **Faith** is put in the plate and spent at the same altar. Confessing lifts
   shame — and takes the same number of points of *renown* with it, which is
   the whole design. Lifting shame and fame together would be free and useless,
   because `Read` weighs one against the other and the character would end up
   standing exactly where they started: a button that does nothing, dressed as
   a sacrament. Taking renown instead means the deeds survive and the face
   stops being known. What penance actually sells is anonymity, which is the
   way out of Notorious and costs the renown that Celebrated is made of.

   It also happens to be the exact inverse of being carried home: a rescue adds
   a point of shame *and* a point of renown, because being carried through the
   gate is the most public thing that can happen to anybody. A confession
   removes one of each. The shrine is the thing that unhappens the walk of
   shame, and `TestPenanceUndoesBeingCarriedHome` is what says so.

   **Honor** is what you did when it cost you — seeing a companion's story
   through to an ending that paid nothing, settling somebody's debt out of your
   own purse — and it is spent at the hiring board. It moves the ongoing *cut*
   rather than the fee up front, which is why it does not double up with
   Standing: what a mercenary charges to walk out of the gate is a question
   about who you are, and what they want off every haul afterwards is a
   question about whether you will still be there at the end of it. Those have
   different answers. Letting somebody go in the middle of their own story is
   the one thing in the game that costs honour, and it should be — nothing else
   the player does is as plainly a decision to stop being there.

   The authoring rule that keeps honour from being the coins axis renamed: it
   has to disagree with the payout sometimes. `debt-collector` puts honour on
   *both* endings — settling the debt yourself and standing between them and
   the creditor are both keeping faith, and they differ on money, fame and
   shame instead. `beast-litter` puts it on neither, because going into the den
   with somebody is courage, not loyalty. `TestHonorIsNotJustTheMoneyAxisRenamed`
   asserts both shapes exist; it caught the first pass, where all nine threads
   turned on honour.

   The altar fix worth recording separately: it used to set `Used` on the live
   entity instead of going through `spend`, so it came back every time the
   interior regenerated. A shrine you could walk out of and back into was an
   unlimited full heal for 25 coins, and would now have been an unlimited
   supply of faith as well.

   `cmd/balance` grew a COMPANY'S SHARE section, because the cut is the one
   term in the economy the fight simulator cannot see. Two hirelings at the top
   of the roll take 36% of every haul before the hero touches it, and honour
   swings that band from 28% to 44%. Nothing else in the report said so.


8. ~~**A reason to be here.**~~ *(Built.)* `internal/saga`: a chain of places
   strung across the continent with an authored ending on the far end.

   It is the third system on the same split, and by now that split is the house
   style — the *writing* is authored, in `data/text/sagas.json`, and the
   *staging* is drawn from the world at cast time and frozen into the save.
   Quests are generated and forgettable on purpose; a companion's thread is
   authored and belongs to a person; a saga is authored and belongs to the map.

   **The one idea that makes it work: legs are spread across the reach of the
   continent, each aimed at its share of the way out.** Nothing in the package
   gates on level, checks a flag, or refuses to advance. The difficulty curve
   does the pacing on its own, because the danger of a region is already a
   function of how far out it is. A generator that picked places at random
   would have needed a gate, and a gate is a thing that says no.

   That claim was false for a while, and the fix is the reason `cmd/balance`
   grew a SAGA section. The first version took the *nearest* qualifying place
   further out than the last — strictly increasing, and therefore passing
   `TestSpinesPointOutward` — which packed all five legs into the near end at
   6, 11, 12, 16 and 17 tiles. Every one of those sits inside the eighteen-tile
   radius `RegionLevel` reads, so the country around the last leg *was* the
   country around the first. The far end came out rougher than the near end in
   6 stagings out of 16; it is 16 of 16 now, with legs at 14, 31, 46, 60 and 78
   tiles and region levels climbing 2, 5, 5, 6, 7.

   Fixing that raised a second question the same section could answer. Spreading
   evenly from zero put *first* legs 13 to 27 tiles out, against a home region
   that ends at 14 — the one stretch of ground the opening was tuned for. The
   spine is offered at the gate on the first morning and the compass points
   straight at it, so that was the story handing a level-one character a
   difficulty nobody chose. Legs now interpolate from the home radius to the far
   edge rather than from zero, and first legs land 7 to 21 tiles out, with the
   stragglers being continents that have no second settlement any closer.

   The lesson is the one already written down twice: an assertion nobody
   measured is a guess. The test that was supposed to protect this pinned the
   wrong property — "each leg is further than the last" is true of a cluster —
   and only building the instrument found it. `world.RegionLevel` was hoisted
   out of the scene layer so the report reads the game's own arithmetic rather
   than a copy, which is the only reason its column means anything.

   Three triggers, all of them things the game already notices for quests: a
   door, a cleared location, a kill. A saga that needed its own bookkeeping
   would be one that quietly stopped advancing the next time somebody
   rearranged a scene, and it is the one system a player cannot route around.

   **The spine** is cast at `startRun` and its opening is pushed *under* the
   welcome box, so the controls are read first and the story second. Two are
   authored. *The Register* is a clerk's list of standing places that has
   stopped agreeing with the country, and the fifth entry is not there. *The
   Instalments* is a two-hundred-year-old quarterly debt paid in parcels, and
   the far end is a flat stone with two centuries of them stacked on it,
   unopened.

   **Arcs** are the same machinery, shorter, and *found* rather than given:
   three legs, rolled against a location's own seed when you walk into
   somewhere nobody sent you, capped at two. *The Wrong Grave* is a marker with
   your name on it and fresh flowers. *The Long Bet* is a wager whose date has
   not happened yet and whose other party is a fixture.

   `TestEverySagaCanActuallyBeFinished` plays each one through on four real
   continents, firing exactly the events the game fires and nothing else. It is
   the test the feature rests on: a spine is five places and possibly several
   hours, and a leg that can never come due does not announce itself — it
   becomes a dead entry at the top of the journal forever.

   Two bugs found by reading rather than running, while the display was down.
   The arc cap counted *stories* rather than arcs, so a finished spine freed a
   slot for a third arc. And the arc roll was `p.Seed % 6 != 0` — a location's
   seed comes off an RNG and can be negative, Go's `%` keeps the sign of the
   dividend, so half the continent was silently ineligible in a way that would
   have looked exactly like the rate being what it is.

### Phase 4 — polish

9. **A UX pass, held until the features stop moving.** *(Jeremy's call, and the
   right one: interface work done against a moving target gets redone.)* Things
   are being noticed as they turn up and parked here rather than fixed one at a
   time, because half of them are the same missing capability wearing different
   hats and a single pass will be cheaper than eight patches.

   **The first pass is done.** It was the right call to batch them: three of the
   five were the same missing capability, and fixing that once fixed all three.

   - `Ask` only took strings, so a box could offer a thing and then refuse it —
     the ending of a backstory quoted a price, let you pick it, and only then
     said you could not pay. `AskMenu` takes rows, so a choice can carry a
     price and be greyed out in advance. The inn, the altar and the hiring
     board went the same way, and all four now say what is in your purse next
     to what the thing costs.
   - Menus have a real section header instead of a disabled row with dashes
     round it, which is a heading the player spends a moment trying to select.
   - A companion's sheet names their backstory. The panel grew from 208 to 220
     to fit it, and that is the last of the room: the footer sits at 244 and
     the frame ends at 236, so the next row has to come out of something else.
   - The selected row in every menu was losing the bottom two pixels of every
     letter off the bottom of the highlight bar. See the commit; I got that one
     wrong twice before measuring it.

   **Second pass: character creation is two screens.** *(Jeremy's, and his own
   structure: "maybe split that into 2 sections, name + portrait + avatar, then
   stats?")* One screen used to carry all of it, and it was at its limit before
   the portrait and the walk sheet were choices at all — up and down picked a
   class, left rerolled the numbers, right rerolled a name, and each of those
   needed a footer line that read like a keyboard shortcut list.

   Now: *who you are* (name, one of ~68 portraits, one of four walk sheets),
   then *what you do* (class and the throw behind it). Up and down pick a
   field, left and right change the one you are on, and each row carries what
   it is currently holding in its detail column — so which arrow does what is
   answered by looking rather than by remembering. The face gets a filmstrip of
   its five neighbours under it, because cycling one at a time through
   sixty-eight faces with only a counter to say where you are is the kind of
   control that gets used twice and then held down.

   The look is decoupled from the class, which is the substantive half. Three
   of the four walk sheets used to be unreachable to a player who wanted the
   class the fourth one came with; a fighter can now walk around in robes.
   `model.Sprite` and `model.Portrait` already existed as overrides — hirelings
   have been setting them since the party landed — so the hero picking them
   needed no new state, only a screen.

   The face roster is *probed* against the asset registry rather than listed,
   because a hard-coded roster of 68 art keys is 68 chances to name something
   that is not there, and the manifest is generated from whatever happened to
   be extracted. The audit checks all four sheets and every offered face now
   (154 keys, up from 82), plus the fallback portrait unconditionally: a
   fallback that is only checked because the roster usually contains it is a
   fallback that can break silently.

   `-demo` grew two frames for these, which is the tour doing its job rather
   than drifting — creation was never captured, and one of the two screens is
   the only place in the game where art is chosen rather than issued.

   One thing fell out: `audiosys.Bank.Enabled` now tolerates a nil receiver, so
   a headless Game can run `startRun` without a panic the first time something
   makes a noise.

   ~~**Third pass, and it has a shape rather than a list: a settings screen.**~~
   *(Built.)* *(Jeremy's, and it arrived as three things that turn out to be
   one.)*

   - **Combat timing is a notch too fast.** His framing is the useful part:
     what is currently "fast" should sit a couple of notches down as "slow",
     with the default somewhere in the middle. That re-anchors the scale rather
     than adding a number — the top of the range is what is wrong, not the
     spacing between the steps.
   - **Sound has settings and nowhere good to set them.** They live on the
     pause menu, which is where a player goes to put the run down, not where
     they go to adjust it.
   - **Key bindings.** The input helpers were written as a single-file change
     for exactly this, and `S` being the down key along with `J`, `A`, `D`,
     `H`, `K` and `L` is the standing argument for letting somebody move them.

   One screen, three sections, reachable from the pause menu and from the
   title — because half of these are wanted before a run starts and the other
   half during one. It is the same batching argument as the first pass: three
   settings scattered across three menus is three places to look, and the
   fourth one will land somewhere else again.

   All three are on `internal/game/scene_settings.go`, reachable from the
   pause menu and from the title. What building it turned up:

   - **The pace ladder is 30, 45, 60 ticks and the default is the middle.**
     The 30 that shipped for months is still there as *fast*, deliberately:
     the complaint was never that it was wrong, it was that it was the only
     one, and somebody who liked it should get it exactly rather than a new
     number that is nearly the same. A round of three against four queues
     seven messages, so the ends of the range are three seconds of waiting
     against seven.
   - **`internal/prefs` exists because there were about to be three of these.**
     The audio bank had been quietly owning `saves/settings.json` since sound
     landed — reading it, writing it, knowing where the saves directory is,
     none of which is anything to do with playing a sound. Pace and bindings
     beside it meant either a second file describing the same thing or a
     second writer on the same one, and two writers on one file is one of them
     losing. The bank is *told* its volume now and writes changes back through
     a callback. Every field's zero value is "never touched it", which is the
     only answer a file written before that field existed can give.
   - **The key-name table is derived, not typed.** `Key.String()` returns ""
     for the undefined ones, so walking `0..KeyMax` is both the table and its
     own filter — a hand-written list of a hundred and forty key names is a
     hundred and forty chances to name one that does not exist. Bindings store
     names rather than numbers, which is the only form that survives a version
     bump of the engine and the only form a person can read.
   - **A rebind replaces the whole list rather than joining it**, because
     somebody rebinding Down is doing it *because* something else wants `S`,
     and an "add" would have left the collision in place. Refusals are
     up front: a key already on another action names that action, and the
     screenshot key cannot be taken at all — a player who bound Cancel to
     backslash would have quietly disabled the only camera this project has.
     "Restore the original keys" is a row rather than a buried command,
     because it is the way out of a keyboard somebody has made unusable and a
     way out you have to already know about is not one.
   - **The title screen dispatched on row number**, with a comment four lines
     above it observing that the pause menu had had exactly that bug. Three
     rows forever, right up until it got a fourth. Fixed to dispatch on the
     label before Settings was inserted.
   - **A tour frame caught the sound row calling a `-mute` run "unavailable".**
     Silence somebody asked for and silence the game arrived at had been one
     flag; they are `hushed` and `off` now, because telling a player who typed
     `-mute` that their installation is broken is a different sentence
     entirely. Turning the volume up on the settings screen lifts a `-mute`,
     since that is a more recent statement of what they want than the command
     line was.

   **Fourth pass, done, and the two things it fixed had one cause between
   them.**

   - ~~**The walking-around screen shows the last *row* of the transcript.**~~
     `Log.AddColor` pre-wrapped every message at `ScreenW-40` and stored the
     rows, which quietly turned the log from a list of things that happened
     into a list of rows — and everything downstream inherited it. The
     overworld draws one and got a sentence's tail presented as the sentence.
     Worse, `DrawWrapped`'s stated promise that an entry goes in whole or not
     at all was operating on rows too, so the battle log's guarantee was never
     the guarantee it advertised.

     An entry is an entry now and wrapping is the drawer's business, which is
     where it has to be anyway: the two panels that show this are different
     widths, and the old code wrapped against a width neither of them used.
     `Log.Draw` takes the room it has, so a line too long for the ticker is cut
     by `Trunc` with a mark on it rather than beheaded.
   - ~~**The death screen has never been captured.**~~ It is in the tour now,
     staged after the demo save rather than by losing a fight — `offerRewind`
     needs a save of this run to offer back, and it is the real path that
     should be captured: the black the battle faded into, the question on top
     of it, and the cost of saying no written out.

   Add to this as things turn up; do not fix them in ones. What is left is the
   art pass below, which is a different kind of job.

10. ~~**Curated UI art from the 4,488-file GUI Pro kit, replacing the procedural
    panels.**~~ *(Done, and it went the other way.)*

    The survey came back negative, which is a result rather than a failure. All
    four GUI kits in the bundle — GUI Pro FantasyRPG, RPG & MMO UI 4, Fantasy
    Nordic, Dialogue Boxes — are painted mobile and MMO interfaces at two to
    four times this game's scale, with three-pixel outlines and soft drop
    shadows that turn to mush against a 7x13 bitmap font and a 16-pixel tile.
    Two of them shipped only 4K preview banners and PSDs. The nearest thing to
    a match in 4,488 files is GUI Pro's `ItemFrame_01`: a thin gold border with
    clipped corners — which is a description of `ui.Panel`, written before any
    of it was opened.

    So nothing was imported, and the pass became: take the vocabulary that is
    already here and apply it to the screens that had none.

    - **`ui.Slot`** — a frame around a picture, in Panel's language. The battle
      screen had three monster portraits floating on a black field with nothing
      to say where one ended and the next began; they are framed now, and the
      frame carries state, so a dead thing is a dim picture in a dim frame
      rather than a dim picture on the same bright field as its neighbours. The
      character sheet's portrait, the creation screen's, and its filmstrip all
      use the same call.
    - **`ui.Cursor`** — the selection pointer was a `">"` set in the body font,
      the last piece of chrome that was a character pretending to be a shape.
      It carried the font's spacing and its drop shadow, so it read as the
      first letter of the label. It is a drawn triangle now.
    - **The title screen shows the continent you are about to play.** It was the
      one screen in the game with no art on it at all — a star field and a
      horizon line — and what this game has more of than anything else is
      terrain. It costs one world generation, and it makes the seed at the
      bottom of the screen legible as a promise rather than a number. Dimmed
      with a multiply and vignetted top and bottom, because the thing in front
      of it is three words somebody has to read.

    `render.VFade` came out of that last one. The first vignette was eight
    banded rectangles and the seams between them read as a rendering bug; it is
    a per-row ramp now.

    The lesson worth keeping is in CLAUDE.md so nobody runs the survey twice.
11. ~~**Title screen art, transitions, particles from the 115-file VFX pack.**~~
    *(Effects built; the title landed with the art pass above.)*

    Unlike the GUI kits, this pack is what it says: pixel art, at this game's
    scale. Every sheet is 64x384 — six 64x64 frames stacked vertically — which
    is exactly a portrait slot, and the loader already slices sheets row-major.
    Seventeen of the 115 are extracted, being the ones something actually
    plays; naming all of them would put a hundred keys in the audit that
    nothing reads.

    A burst plays where a blow lands. That is the whole feature and it is
    deliberately not a mechanic — nothing in `rules` reads it, no number
    changes, and deleting the file would not alter how the game plays. What it
    adds is the one thing the transcript is worst at: **where**. With three
    monsters on screen, "Bosk hits the Overfamiliar Spider" is a sentence you
    have to read, and a burst on the middle portrait is not.

    The effect comes off the technique's *kind* by default, with `Spell.VFX`
    able to override it — the table carries the rule and the data carries the
    exceptions, the same split the rest of the content follows. Five techniques
    take an override because they have a character their kind cannot know
    about: spark is lightning, scorch is fire, unmake is void, haymaker is a
    rock, cold comfort is ice.

    Three things worth keeping:

    - **`SpellDamage` first mapped to the electric explosion, which is four
      thin cyan strokes on a transparent field with two blank frames at the
      front.** The commonest technique in the game played nothing anybody could
      see. Caught off a captured frame, which is the only way to check art.
    - **Party-side bursts were drawn and then painted over.** They land on the
      party panel, so drawing them in the same pass as the monsters' put them
      underneath the thing they were aimed at. There are two passes now, and
      retirement moved out of `Draw` — a draw that rewrites the scene's state
      is a draw nobody can call twice, and this one is called twice.
    - **The slash art cycles on a counter, not a roll.** Drawing from the
      shared generator to pick something purely cosmetic would move every
      damage roll after it, so a build with effects and one without would play
      the same seed differently — for a decision the player cannot see the
      result of.

    `TestEveryCombatEffectIsPlayedBySomething` checks coverage in both
    directions. The audit already catches a key with no art; this catches art
    with no key, which is the quieter failure — a file extracted, counted in
    the audit's total, and never once reaching the screen. It caught five.
12. ~~Balance simulation: run 10,000 headless fights per level band against the
    pure `rules` package and tune the curve.~~ *(Built, and then some: this is
    what `cmd/balance` became.)* It defaults to 2,000 fights per data point and
    takes `-fights`, it has thirteen sections rather than one curve, and three
    of them exist because a mechanic turned out to be invisible to it —
    SHAPES, SUPPLIES and WHAT THE CUT BUYS. The line worth keeping from the
    original phrasing is "against the pure `rules` package": it plays the
    game's own code rather than a copy of the maths, which is the only reason
    any of its numbers mean anything.

### Phase 5 — art with nowhere to go

Three things came out of the asset work, and all three are the same shape: the
picture exists and the game has no place that could show it. That is the
opposite of the usual problem here and worth keeping separate from it, because
"look for the capability before building it" does not apply — the capability
genuinely is not there. Full detail in [ASSET-MAP.md](ASSET-MAP.md).

1. ~~**Wandering monsters on the overworld.**~~ *(Built.)* `pixelartrogue-likerpg` holds
   roughly seventeen creatures in six poses each, drawn at 16px — skeletons,
   slimes, crabs, bats, a phoenix, a thing made of rock. There is nowhere to put
   them. Battle art is `mob/`, painted portraits reduced from 128px, and 74 of
   its 168 keys are in use, so it is not short. Interior foes are `foe/`, 64px
   walkers. The overworld draws no creatures at all: an encounter is a dice roll
   in tall grass with nothing on screen before it fires.

   So the art is not the feature. The feature is **a creature you can see
   coming**, and the case for it is that the current model gives the player no
   information and no agency — walking through grass is a slot machine, and the
   only lever is walking somewhere else. A visible wanderer turns the same
   encounter into a decision made in advance: go around it, or go at it.

   **The answer to all four questions was "keep it one system."** The roll is
   untouched — same `dangerRoll`, same rate, same tables, same `PickEncounter`
   — and what changed is what a hit *produces*. Instead of a battle screen it
   puts a creature four to six tiles off, and the fight happens when the two of
   you meet. The encounter is decided at the moment it appears and carried on
   the wanderer, so the silhouette is telling the truth about what is in there;
   the sprite is keyed by `model.MonsterKind`, which is the one property of a
   monster worth knowing before you commit.

   It drifts until it notices you (within four tiles) and closes after that,
   which is the interior foes' rule plus a distance check. It never un-notices,
   or it would jitter on the boundary. It gives up at fourteen tiles or ninety
   of its own steps.

   **Nothing about it is saved.** A wanderer is weather, not furniture: the save
   format is seed plus deltas, and a persistent creature would have to be a
   delta, which means every file written before today loads with none and every
   file after carries a list of animals. Walk away and it is gone; come back and
   the grass rolls you a new one.

   One fight is given up that the old model would have had: if there is nowhere
   in the ring to stand — a beach, a spit of land — the roll does not become a
   creature. That is the price of never lying about what is coming, and the rate
   is otherwise identical because at most one wanderer exists at a time and the
   roll does not fire again while one is out.

   `-demo` still cannot show it — `demoWalk` teleports the player rather than
   stepping, so the tour never runs the encounter roll — and the answer to that
   is tests rather than a longer tour. `internal/world` covers the movement;
   `internal/game/wanderer_test.go` covers the half that lives in the scene,
   including the one that matters most: **only one encounter is ever out**,
   which is the whole difference between this and a second encounter system.
   Each was checked by breaking the thing it guards and watching it fail.

   The creature does not animate — the sheet has a walk pose per row and only the
   standing one is cut, because legibility decides whether the feature works and
   animation does not.

2. ~~**Decor a room can be dressed with.**~~ *(Built, and smaller than it
   looked.)* The "torch" is not a torch. The five frames are a stone brazier
   standing on the floor — 19x38 of art inside a 64-tall frame — so the pass
   that knows a wall from a floor and mounts something on it was never needed.
   A brazier is just a thing that stands somewhere.

   What it did need was an entity that nothing interacts with. `EDecor` is
   solid, drawn, and has no case in `interact`; it is excluded from the bump
   that would call one, alongside `ESign`. Dungeon rooms get one on a coin
   flip, placed by the same room-rectangle rule the chests use rather than by
   `findOpen`, because a brazier is solid and one dropped in a one-tile
   corridor would wall off whatever is past it.

   It animates for free: `drawEntity` already ticks a sprite's frames, so
   stacking the five files into a strip was the whole of it.

3. ~~**A spear that exists.**~~ *(Drawn, by a local model, and it holds its own
   on the shelf.)* Polearms are the one weapon kind with no picture anywhere.
   "Glaive, Overcompensating" borrows an axe head on a haft and reads correctly;
   "Spear, Regrettably Long" borrows a staff with a point and reads as a wand,
   one tier from an actual rod.

   Six places were checked, and the point of writing them down is so nobody
   spends the afternoon again:

   - the crafting sheet — eight weapon shapes, no spear
   - the ability set — sword, axe and hammer across four tiers, nothing else
   - `2dminimalskillicons` — ships a `Skill_Spear`, which is a full-bleed tile
     with a purple background and is not a spear, it is a flame
   - the rogue-like sheet — two character rows *are* holding spears, and the
     spear is part of the figure's own connected component, not liftable
   - GUI Pro's 361 PictoIcons — has a `halberd`, but they are flat white
     anti-aliased silhouettes that blur to nothing at 16px beside the shaded
     cuts, and the halberd reads as an axe anyway
   - a filename sweep of all 44 extracted packs and every zip in the bundle

   Composing one from a `staff` haft and a `dagger` blade was tried too, and
   reads as two objects with a gap between them.

   So it was drafted by a local language model — `cmd/pixelsmith` — and what
   was learned doing it is worth more than the icon:

   - **A language model, not an image model.** The target is sixteen pixels on a
     six-colour palette. Diffusion draws a thousand pixels of something
     pixel-art-*shaped* and leaves you to reduce it, which is the exact failure
     the ability icons already demonstrate. A language model emits the grid:
     right size, right palette, no resampling, every cell inspectable.
   - **Showing example grids teaches copying, not style.** Seeded with the real
     icons, qwen3:30b returned `sword1` back almost cell for cell, twice, and
     devstral spent a batch redrawing `staff3`. Useless as a spear and worse
     than useless as an artefact: a near-copy of a purchased sprite *is* the
     purchased sprite, which must not be committed to a public repository.
   - **Removing the examples removes the ability.** With style described in
     prose instead, the same models produced unstructured blobs. The grids were
     doing real work.
   - **What broke the deadlock was decomposition.** A weapon icon is mostly a
     straight line, and a straight line is the part a program draws better than
     a model. So the tool lays the haft on the set's own diagonal and asks the
     model only for the head — a 6x6 shape, twenty-five cells, a task these
     models can actually do. Every candidate after that change was a connected,
     correctly-proportioned hafted weapon.
   - **The join must be found, not promised.** Telling the model "the haft will
     meet your bottom-left corner" does not make it draw there. The tool now
     locates the lowest-leftmost cell the model actually inked and runs the haft
     from beneath it.

   A second pass got it from passable to good, and every step of it was a
   correction to the *tool* rather than to the prompt:

   - **A constructed example beats both showing icons and showing nothing.** A
     plain geometric taper — a triangle this draws itself, nobody's art — is the
     middle ground: it carries the notation and the idea of narrowing without
     giving the model anything worth copying. Heads went from knobs to
     recognisable points the run it was added.
   - **The example has to be drawn on the set's own axis.** The first one was a
     vertical triangle, and an axis-aligned leaf pasted onto a diagonal haft
     reads as a flag on a pole. Redrawn as a taper along the diagonal, which is
     the orientation everything in the pack is drawn in.
   - **A scorer written for the wrong axis rejects everything.** The head filter
     originally demanded "taller than wide", which is true of a vertical leaf
     and false of a diagonal blade — whose bounding box is square. The moment
     the prompt asked for a diagonal point the filter threw away every reply.
     It measures along the anti-diagonal now: a tip of one or two cells at the
     far end, widening back towards the haft.
   - **The haft's shading was invented and should have been copied.** It laid
     the shadow cell *below* the body cell; `mace1` and `pick1` both put it to
     the right on the same row. The difference is one pixel per step and it is
     the difference between a stick and a haft — invisible alone, obvious in a
     row of eight.

   The result now carries the same weight as the weapons either side of it and
   reads unmistakably as a spear rather than as the wand it replaced.

   What is committed is the *grid*, not a PNG. `assetpipe build` must stay
   byte-reproducible and a model is not, so `data/art/spear.txt` holds sixteen
   lines of palette indices and the pipeline renders them — verified by deleting
   the whole generated tree twice and getting the same file hash. The grid also
   contains no purchased pixels: it says which of six slots each cell uses, and
   the palette is read out of the extracted pack at build time by
   `internal/pixelpal`, which exists because the two tools disagreed about which
   colour slot 4 was and produced a valid grid that rendered yellow.

### Phase 6 — people with faces

*(Started.)* Everything a person said came through one bottom-anchored strip:
the same box for a signpost, a chest, the game narrating, and somebody offering
you a job. A quest-giver had exactly the same presence as a plank with writing
on it.

**A conversation is now its own layout.** A face on the left in a `ui.Slot`, the
name as the panel's title, what they said in a column beside the portrait, and
the choice underneath it — so the eye goes down one column instead of crossing
back. It stops just above the status strip rather than covering it, because
where you are and what you are carrying are both things you want in hand while
deciding whether to take a job.

Three things fell out of building it.

- **The box already had a `portrait` field, declared and never used.** Somebody
  started this before and stopped. It was dead code, not a capability — the
  usual rule about checking whether a thing is merely unreachable cut the other
  way this time.
- **A face has to be stable, and nothing stores one.** A town's people are
  regenerated from the world seed on every visit, so a face cannot be assigned
  and kept. `faceFor` hashes the name, which is the only thing about a
  townsperson that is both theirs and stable. The baker is the same baker on
  your third visit, which is the entire point — a portrait that changed between
  conversations would make a town feel less peopled rather than more. Nothing is
  saved; it derives from data the save already holds.
- **It exposed a mismatch that had always been there and was invisible.**
  Hirelings drew a battle portrait at random from a pool of ten. Nobody outside
  a fight had a face, so nobody could tell. The moment the hiring conversation
  showed one, you agreed terms with a dark-haired mage and a redhead walked off
  with your money. A recruit now keeps the face they were wearing when you
  talked to them.

A caption under the face says what they are — "blacksmith", "fey mage" — wrapped
to the portrait's width over two lines rather than truncated, because "undead
ma." is not a shorter way of saying "undead mage". It is deliberately empty for
an ordinary townsperson: "villager" under every second face is noise that
teaches the player to stop reading it.

**Faces are pooled by what somebody is.** Hashing a name across all seventy-six
portraits put a helmeted knight behind the apothecary's counter, because the
pack is mostly adventurers. `internal/game/faces.go` holds the pools and three
rules hold them together: a face belongs to a person rather than to a
conversation, the pool is chosen by something that cannot change about them, and
nothing is saved because every input is already in the seed.

- **Recruits** get the armour and the helmets — the pool that matters most,
  since a hireling's face follows them into the party panel and every battle
  after it.
- **Each counter** has its own: a smith built like one, an apothecary who looks
  like they know something, an innkeeper pleased you came in.
- **Errand-givers by errand.** A cull is asked for by somebody it has happened
  to, a delve by somebody frightened enough to send a stranger into a hole.
- **Everybody else** is the plain-faces pool, which is also what an oddity's
  residents draw from: the joke there is that nobody in the frame is in on it.
- **The cultists are in no pool at all.** One of them has glowing eyes, and a
  cultist behind the counter is the game telling a joke about its own asset
  pack.

Keying a face on the errand needed one thing fixed first. `wantsToAsk` was
already deterministic — whether somebody has an errand is a hash of where they
stand and the settlement's seed — but *which* errand came off the run RNG at the
moment of the first conversation. A face built on that would appear when they
handed you a job and vanish when you finished it. `questFaceKind` hashes the
kind the same way and `quest.Generate` takes it as a preference, honoured when
the settlement can support it. The face uses the preference whether or not a
quest exists and never reads back off the quest: a settlement with no dungeon in
reach produces a different errand, and there a face that stays put is worth more
than a face that matches the job.

**The shop gave up a column.** A counter was the last place in the game where
you dealt with a person and never saw one. Tucking the portrait into a corner
does not work — a list that reaches the panel edge has no spare corner, and both
attempts collided with the rows — so the rows are 54px narrower and the vendor
stands in the gap. No caption under it: the panel title is already the counter's
name.

**The captions are written now.** Six per counter and six per errand kind,
picked by the same name hash the faces use, so two smiths in two towns read
differently and the same smith reads the same way every visit. Three passes at
the voice were commissioned in parallel and the final set is mixed from all
three — flat occupational ("smith, no eyebrows"), quiet grievance ("smith,
apprentice fled") and something very slightly wrong that nobody finds remarkable
("smith, never burns") — because a town where every caption is the same joke is
a town telling one joke six times.

The trade is always the first word and the joke rides on top of it: a player who
cannot tell the smith from the innkeeper has lost what the plain word gave them.
The exception is the shop, which drops the trade word because the panel is
already titled Blacksmith — and pays for the redundancy in item names, since
every pixel that column takes is a pixel the rows have to cut.

Recruits keep a derived caption. Blood and class decide what that person may
wield and wear, so the caption is load-bearing there in a way it is nowhere
else, and a joke in the slot would cost the player the fact they need before
paying.

`TestCaptionsFitUnderAPortrait` measures every caption in both places it is
drawn. The widths were wrong at first in a way that only measuring found: the
brief asked for twenty-two characters over two lines, and "apothecary," is
seventy-seven pixels on its own against a seventy-six pixel frame, so a whole
class of caption could never have fitted however short the rest of it was.

## Since the roadmap

**A followed destination, and a compass.** *(Jeremy's, on seeing the saga
journal: "Do we need an active quest in the journal, with a compass (or maybe
hint is easier) that points towards its goal?")*

Yes, and it was a real gap that the saga work made obvious rather than created.
A destination was *named* in the journal and *pinned* on the map, and between
those two facts was the actual business of getting there: open the map, find
the pin, remember roughly where it was, close the map, walk, repeat. Worst
exactly where the long stories send you, because a spine's legs are
deliberately further out each time.

Two halves, and **the tracking is the more important one**. "Where is it" is
the second question a journal with six entries raises; the first is "which of
these am I doing", and a compass with nothing selected has nothing to point at.
Z on any journal row follows it — sagas, errands and backstories alike, through
one `destinationOf` because a player selecting a row does not care which system
it came out of. A fetch quest with nowhere of its own points at whoever asked
for it, once the counter is full.

The status bar carries it in the corner that used to say "M map - H help"
forever, which stops being news after five minutes and is what the help screen
is for. Name, distance in tiles, and a seven-pixel arrowhead.

It also follows things on its own, so the feature works for a player who never
finds it: a saga leg coming due sets the next destination, and so does taking
an errand — but only when nothing is already being followed, so an explicit
choice is never overridden by one.

Three details worth keeping. The arrowheads are hand-set rather than
rasterised, because a filled triangle at seven pixels comes out as a smear that
reads differently in each direction. A bearing counts as diagonal only when
neither axis dominates by more than about two to one — equal wedges would make
almost everything diagonal on a 160x120 map, and an arrow that is diagonal nine
times in ten has stopped saying anything. And `Track.On` is an explicit flag
rather than `POI == -1`, which is the resident-thread lesson applied before it
could bite: a zero value that means something real turns every old save into a
silent claim about location zero.

**Four small things off one round of play.** *(Jeremy's, in a batch.)*

- **Save and Load ahead of Sound** in the pause menu, because that is the order
  they are wanted in: a player opens that menu to put the run down, and sets the
  volume once. Safe to reorder now only because dispatch is on the label rather
  than the row number, which is a bug this menu has already had.
- **The slot list opens on the most recently written save**, in both directions.
  Loading, it is the run you were in; saving, it is the slot you have been
  using. Landing on slot one every time makes overwriting the wrong save the
  default action.
- **A roll at character creation is coloured against the band its own class
  rolls it in.** Eight Strength is a poor Fighter and a good Mage, and a player
  choosing between the three cannot be expected to know either band. This is
  why `rules` grew a `startingBands` table: the numbers were in a switch inside
  `NewCharacter`, and colouring a roll means knowing what the roll could have
  been. One copy, two readers, and a test that rolls four thousand characters
  per class to check the table still describes the roller — including that the
  band is not *wider* than the roll, since slack at the top would make a perfect
  roll one that can never come out green.
- **Gear on a shop counter shows what it is worth against what is worn**, affix
  included, with an empty shield arm reading as nothing so the first shield is
  the upgrade it is. Charms are exempt and that is the interesting part: every
  charm in the table gives with one hand and takes with the other, so there is
  no better one, and a green charm would be the interface contradicting the
  content. `TestTheShelfNeverGradesACharm` holds the line.

The colours live on the detail column rather than the label, because that is
the one part of a menu row whose colour survives the cursor landing on it — a
label goes dark on the selection bar, and green on gold is unreadable.

**The combat menu opens on the technique you cast last.** A fight is mostly the
same two or three moves in some order, and scrolling past the same four
techniques every round to reach the one being used is friction that stays
invisible until somebody counts the keypresses. Recorded only once the psyche
is actually paid, so a technique selected and then backed out of at the
targeting step does not become the one the menu opens on; saved, because the
autosave means a death is followed by fighting the same fight again and that is
exactly when a reset cursor would be felt.

`ui.Menu.Select` refuses a disabled row rather than snapping past it. Parking on
a technique the player can no longer pay for means the first thing they press is
declined, which is the opposite of what greying it out is for.

Looking for that turned up an accident: one `Menu` serves the root list and the
technique list, and `SetItems` preserves the index — so opening Techniques from
row one landed on the *second* technique, and Item from row two on the second
item, for no reason anybody chose. Both start at the top now.

**Shipping it.** `make-dist.command` builds a zip per platform into `dist/`:
a macOS universal binary (arm64 and Intel joined with `lipo`, so a friend does
not have to know which machine they have), a Windows exe cross-compiled from
the Mac, and beside each the 721 asset files the two manifests actually name —
96 MB out of a 16.7 GB bundle, at the paths the manifests already record, so
nothing has to be rewritten. `FindRoot` already walks up from the executable,
so the dist needed no code change at all.

Two things that had to be got right and one that did not matter:
`-H=windowsgui` on the Windows build, or double-clicking opens a terminal
window behind the game and leaves it there. A READ ME FIRST that explains
Gatekeeper and SmartScreen, because neither build is code-signed and without
that paragraph a friend simply cannot start it. And `du` reported the zips 16 MB
heavier than they are, because it counts blocks allocated rather than bytes —
cosmetic, but a script that misreports its own output is a script nobody
trusts the rest of.

Shipping with the art baked in is the licensed use case rather than an edge
around it; `docs/ASSET-LICENSING.md` says so at length, and says equally
clearly why none of it may be committed to a public repo.

**Death is a screen now, and the checkpoint moved.** *(Jeremy's.)*

The battle fades to black over about two seconds and the question arrives on
the black rather than over the corpse. Every key is ignored while it happens —
somebody mashing Z through the last round should not skip the one moment the
game takes for itself. A cubic ease was the first attempt and it spent the
first second doing nothing visible and then slammed shut, which is a delay
followed by a cut rather than a fade; it is linear.

The autosave moved off "before every fight" and onto **rest**: a bed, an altar,
the first morning. Checkpointing each encounter meant a death cost one fight,
which is barely a cost — the run resumed from a step already taken. Costing
everything since the last stop is what gives the inn a job beyond hit points
and turns "should I pay for a bed" into a question.

That is a *time* penalty rather than an in-game one, which Jeremy flagged, and
the answer is that the game already has the other kind and now they pair up:
die with somebody hired and you are carried to town for a third of the purse
and a point of Shame, keeping every yard of progress. Die alone and you pay in
replayed minutes instead. Hiring a companion converts a time penalty into a
money one, which is a better reason to hire than "an extra sword".

And death goes back to **the newest save belonging to that character**, not to
the autosave slot by name — also Jeremy's, and a real hole. Save by hand, play
for half an hour, die, and the old behaviour would have offered the checkpoint
from before the save you deliberately made. Worse, the autosave outlives the
run that wrote it, so a fresh character could have been handed somebody else's.
`save.LatestForRun` matches on seed and hero name, and its test writes four
saves across three runs to check it picks the right one.

**Hirelings answer back, and two of them can now patch you up.** *(Jeremy's,
as four questions about companions. Three were "does this exist", which is the
failure mode this project keeps finding, so they were checked rather than
answered.)*

*Firing one* already existed — `R` on their sheet, with a confirmation, and the
footer advertises it whenever a companion is being looked at. Exchanging is
fire-then-hire; there is no swap.

*Talking to one* did not, and that was a strange gap: nine authored backstories
and no way to ask after any of them. Everything a hireling had to say, they
said at you on a schedule. `B` on their sheet now asks, and it answers whatever
question they are actually being asked — the ending if one is waiting, else
what they are waiting on with its counter, else something about themselves if
the continent could not stage them a story at all.

*Healing you* was the interesting one, and the first answer given was wrong.
The spell dump defaulted a missing `class` field to "any" and `reconstitute`
was read as a general technique; it is `"blood": "ooze"`, a lineage perk no
hero can learn. So the real picture was worse than reported: **only a Mage
hireling could heal the player**, via `mend`. `secondwind` and `reconstitute`
were both self-only and `reknit` cannot reach a hero, because the fight ends
the instant one falls.

`secondwind` targets one now, so a Fighter hireling patches you up from level
three — and its cost went 3 to 4, because reach is a real gain and the
alternative was a level-three Fighter technique strictly beating the Mage's
level-one one. `reconstitute` targets one as well, which costs nothing to weigh
since no hero can learn it and makes a part-ooze hireling a medic. A Thief
hireling still never heals anybody, which is not an oversight: that class has no
healing technique by design and the two-for-one at the counter is what it gets
instead.

Measured rather than assumed: the combat table moved by at most 2.3 points, all
of it at level three where the cost increase bites, and ENDURANCE did not move
at all.

Still open, and Jeremy's: **a hotkey for the autosave slot.** It is written
before every fight and the death prompt is currently the only way back to it.

**The friction pass.** *(Jeremy's, in a batch of five, and they turned out to be
one complaint: the game kept asking the player to do a second thing after they
had already done the first.)*

- **The world stopped going under the status bar.** `render.Camera` grew a
  `ViewH` — how much of the frame is actually *looked at* — and clamps and
  centres on that rectangle rather than on the whole screen. The bug was worst
  in towns, and it was two bugs: at a map's bottom edge the camera clamped to
  the full frame, so the last rows of the world (the gate among them) sat
  behind the HUD, and the compensating `py-hudH/2` on the centring had its sign
  the wrong way round, which parked the hero forty-four pixels *below* the
  middle of what could be seen rather than above it.
- **A town has four gates now**, one in each wall, opening onto the street
  cross that was already there. The player still arrives at the south one,
  because that is where the road is; what changed is that leaving from the far
  side of a fifty-six-tile capital is not a walk back across it. No RNG is
  consumed by any of this, so every existing save still generates its own town.
- **Things say what they are without being asked.** `ui.Tag` floats a name on a
  dark plate over whatever it belongs to: locations on the continent, doors and
  gates and chests in an interior. Three ranges rather than one, because the
  question is different for each — a door is a destination and worth naming
  from across the square, a townsperson's name is not and a capital has ten of
  them, and a foe is neither, since walking into one starts a fight and what it
  turns out to be is what the fight is for. A signpost the player is facing
  shows *what it says* rather than what it is, which is the whole of a sign.

  Two details. A tag slides sideways when it would be drawn over the hero, not
  up: there is far more room across a 480-pixel screen than between a doorway
  and the top of the frame, and lifting is what stacks two labels into one pile,
  since everything driven out of the hero's way vertically arrives at the same
  height. And tags are drawn *over* the night tint, because a name that dims at
  dusk is a name nobody can read at the hour they most need it.

  The status bar's right-hand corner stopped naming what was ahead, since the
  thing ahead now names itself. That corner is the furthest point on the screen
  from where the player is looking.
- **Walking into something is doing it.** Foes, bosses and the way out already
  worked this way; shop counters, inns, chests, altars and the hiring board
  blocked the step and then waited to be pressed at. Nothing was gained by the
  wait — you cannot bump a counter by accident, because you had to walk at it
  to get there. On the continent, stepping onto a location enters it. A sign is
  the one exception, and only because it says what it says on the ground next
  to it now, so a box over the top would be the interface saying it twice.

  The trap here is the doorstep: walking out of a town leaves the hero standing
  on its tile, and without a guard the step out is the step back in, forever.
  `Game.arrived` is that guard, and it is on the *game* rather than on the
  overworld scene because the hero is put down in three ways that are not a
  step — a new run, a loaded save, and being carried home by the company — and
  all three can land on a town. Being carried home is the one that makes it
  worth a field: that is a place you were taken to, and walking straight
  through the door afterwards is the game taking one more decision off somebody
  who has just had a bad afternoon.
- **Any key gets on with it.** A box that is only *reporting* — a chest's
  contents, a sign, the end of a fight — closes on anything. "Z to continue" is
  a rule that has to be taught and then remembered, for a screen whose entire
  content is "you have read this". A box with a *choice* in it still wants a
  deliberate key, because there the wrong answer costs something. `Keystroke`
  excludes the screenshot keys, which is load-bearing: dumping the framebuffer
  is how anything in this game gets looked at, and a dismiss-on-anything box
  that closed itself the instant you tried to photograph it would be a box
  nobody could photograph.

**Equipment has lanes, and the mage's lane is magic.** *(Jeremy's: "revamp
equipment to be class specific — wands or staves that do spells for the mage,
2H weapons only for the fighter", and its second half, "it seems better to just
whack things with a good weapon than use technique like you might expect".)*

The gate is **hard** — Jeremy's call, and the right one. A soft version, where
anybody may hold anything and off-class gear simply works badly, reads on a shop
counter as a number that is wrong for reasons nobody explained, and it leaves
"what should I be saving up for" answerable only by arithmetic the player cannot
see. A refusal is legible, and it lets the shelf grey a row out in advance,
which is the rule every other menu in this game already follows. The counter
says `thief, mage only` where the comparison would have gone.

Five lanes of weapon — dagger, blade, blunt, polearm, focus — plus a hand count,
and three weights of armour. Each class gives something up, and that is the
point of having lanes at all:

- A **Fighter** reaches the biggest numbers in the table, and the weapons that
  get there need both hands, so the shield arm is the price. Two-handed gear
  carries −2 speed and −1 dexterity as well: hit harder, hit less often, act
  later. It is the duelist archetype's whole content, and the report says that
  build now wins four of the seven measured levels rather than two.
- A **Thief** tops out a band below on strike and buys dexterity back with it,
  and wears light armour that carries the same. They also lose nothing relative
  to before, which was deliberate — they were already the weakest class in the
  report and a lane pass that cost them anything would have been a nerf wearing
  a feature's clothes.
- A **Mage** cannot wear steel or hold a shield, and their weapon is a rod that
  is nearly useless to swing. What they get for it is that the rod is what their
  magic is made of.

**The wand question was Jeremy's, and it is the one that made the feature
work.** "Can't tell if we're getting an auto-attack here, a free spell, or a
discount." The answer is the first: **a focus weapon's ordinary attack is a
bolt, and it is free.** Without that, a caster holding a stick with strike five
has a plain Attack worth nothing, so every round they cannot afford a technique
is a round they are worse than a Fighter with a table leg. Making the free action
itself magical is what "magic is what a mage does" has to mean if it is going to
mean anything — and it is why no in-combat psyche regeneration was needed, which
was the other candidate and the one Jeremy did not pick.

It is resisted by whichever of the target's two defences is **thinner**. Going
through the ward alone was the tidy answer and the wrong one: ward outgrows
armour at the top of the monster table, so a level-thirteen Mage's free round
against a dragon landed for about three, and the class stopped having a free
action exactly where every other class's got better — death three levels over
went to 47% against a brief that allows 36. A bolt is a shove of raw force
rather than a shaped working, so it goes where the thing is thinnest. The
interesting choice survives: a dagger still carries strength behind it and still
hits a dragon harder than the rod does.

`SpellPower` grew a focus term at half the bolt's rate. Three terms, each a
different thing the player did: Power is the technique, Psyche is what they are,
Focus is what they are holding. Paying focus at full rate in spells too
compounded three growth curves into one build and put a level-eleven Mage at 94%
on the stretch fights a Fighter was losing half of.

**Technique costs what the class pays.** Mage 1.0, Thief 1.15, Fighter 1.3,
rounded up and floored at one. The arithmetic is not the point — a Fighter's
pool was always small and their techniques always rationed — the point is that
the surcharge is *stated*, on the row, before the cast. It cost something to add:
a level-twelve Fighter had nine psyche against a Haymaker costing eleven, so the
technique at the top of their own list was a row that could be read and never
selected. Martial psyche growth went from nothing-or-one per level to one-or-two,
which is what makes the surcharge a price rather than a wall.

**Getting your breath back.** *(Jeremy's, mid-build: "might be worth a bonus to
psyche back on end of combat, same with health".)* A share of what the fight
actually took, handed to anybody still standing — won or run from, since a
retreat already pays no experience, coin or drop.

A share of the **spend**, not of the pool, and that is the whole of making it
safe. A flat tenth of maximum hit points is larger than what a level-one fight
costs, so the character heals faster than the world hurts them: the first draft
put a level-one Fighter at forty fights on one rest against sixteen before, and
`TestEnduranceHoldsAcrossLevels` said so. A share of the damage taken is a
discount on the encounter — it can never be net positive, it scales with how bad
the fight was without being told the level, and it moves endurance by a factor
anybody can compute. Per fight rather than per round, because regeneration
inside a fight makes a long fight cheaper than a short one and quietly makes
stalling the correct play.

**What the report said, and what it took.** Ten passes. The instrument is a
diff against the previous run's stretch-fight column, per class, because the
question is not "is this balanced" but "did this move anybody off where they
were". Landed at Fighter +0.6, Thief +3.7, Mage +2.2 mean points across
fourteen levels, with the thief's drift deliberately left positive.

Four things only the report could have said:

- The first pass moved the Fighter's on-curve build onto one-handed weapons and
  it fell eight points, because the old table's top-of-tier entries were mostly
  two-handed — so "best weapon of your tier" had silently meant a two-hander
  *and* a shield for the whole life of the game. The one-handed ladder had to be
  the old ladder for anything else to be comparable.
- The Mage's first draft was +9 on average and +20 at level twelve, and none of
  it was the bolt: the probe said their free round was *worse* than the sword
  they used to carry at every level. It was that a lower free-action floor makes
  more techniques worth paying for, so they cast in rounds they used to swing in.
- Cloth armour with a tier-five ward of 14 — the same as the best ward charm —
  made a Mage take less damage than a Fighter in plate, because half of what
  attacks you at that level attacks the ward. Robes carry a third of that now.
- A `winRate` helper in the test suite was rolling the *local* biome for its
  "three levels over" probe, which is the same mistake `cmd/balance` had already
  found and fixed in its own stretch column. It reads the region that far out
  now. The failing assertion was noise at 250 samples; the fix was the
  measurement, not the threshold.

Left standing and stated rather than tuned away: a level-thirteen Mage three
levels over dies 40% of the time against a brief that allows 36. It was 36.3%
before, and it is the same cell that was already the worst in the table. The
class is fragile by construction now — cloth and eleven guard against plate and
twenty — and the report prints the miss.

**A technique that is worth more than a swing.** *(The other half of Jeremy's
complaint, and his own list of what would fix it: damage that scales off the
rod, more lingering and multi-target effects, technique priced by class, and —
his addition — "some spells give a positive effect for you and negative for your
opponent, or vice versa".)*

Eight new techniques and two new kinds. Before this, most of each class's list
was a bigger swing that cost psyche: the Mage had three single-target damage
techniques out of eight, the Fighter three of six. A technique whose only
argument is "more damage this round" is a number, and the reason to spend psyche
on one has to be that it does something a swing cannot — reach everything, last
past the round, or move a stat.

The two new kinds are Jeremy's pairing, one in each direction:

- **`sap`** takes the edge off them and puts it on you: the same magnitude of
  weakness on the target and blessing on the caster, in one technique. The
  caster's half lands *once*, however many it reached — otherwise pointing it at
  three things would be three blessings, and a technique whose value scales with
  how outnumbered you are is the wrong shape for the one meant to even a fight
  up.
- **`pact`** is the other direction: it hits far above its band and the caster
  wears the weakness for the rest of the fight. `PactCost` is derived from the
  technique's own power rather than authored beside it, because two numbers that
  have to move together are one number and a rule — a pact whose power was
  raised in a balance pass and whose cost was not would silently become the free
  lunch the kind exists to not be.

The battle menu quotes both halves in the detail column — `10 SP  -4` in red,
`6 SP  +3` in green — because that is the one part of a menu row whose colour
survives the cursor landing on it, and because a technique that charges you
something and does not say so is the exact failure this project keeps finding.
There is room for about nine characters after a long name, which is why it is a
signed number rather than a sentence; the transcript says it in words the first
time anybody casts one.

**Two holes in the simulator turned up on the way, and both were older than this
pass.** `SimulateFight` called `PlayerAttack` with zero buffs and read monster
damage without `OffenseMod`, so a blessing was worth something in the game and
nothing in the report, and a weakened monster hit at full strength. Every
existing weaken, bless and smoke bomb had been invisible to the balance pass
since the effects system landed. Fixing it is most of why the stretch bands
softened when the new techniques went in — the report was catching up with a
game that already behaved this way.

The simulator's policy learned to open with a sap and to weigh a pact on what is
left of it after the caster has paid. Both conditions are read off the board —
"am I already blessed", "is anything still above half" — rather than remembered,
which is what keeps it a policy rather than state the simulator has to carry.

Tuned in two passes: the first draft put the Thief +5.1 mean points and dropped
the whole +3 band five points below the brief, because a sap-all at power 4
against three creatures is a sixteen-point swing per round bought with one. At
power 3 it lands at Fighter −0.8, Thief +1.9, Mage −0.7 against the numbers
before any of this started.

**And a menu bug found by measuring rather than by playing.** Fitting a
two-sided technique's second number into the detail column meant working out how
much room a row actually has, and the answer was that "Strongly Worded Unmaking"
left twenty-eight pixels for a price needing thirty-five — so the strongest
technique a Mage owns had been showing its cost as "12." while the row above it
showed a whole number. `Menu.Draw` served the label first and gave the detail
whatever was left, and the label always wins that fight, because names in this
game are jokes and jokes are long. It is the other way round now: the detail is
measured first and the label is cut to fit around it. The detail column is where
the price, the count and the verdict live — it is the half a player is reading
the row *for*, and the half short enough to always fit.

**What is deliberately not different.** A Fighter and a Thief buy the same
one-handed weapon on curve. The Fighter's lane advantage is an *option* — the
two-hander nobody else may hold — rather than a better default, and the ARCS
section is where it shows up. Making the default differ would have meant one of
them was simply behind.

**The caster gets the off arm back.** *(Jeremy's, on reading the level-thirteen
Mage's death rate: "do we have any magic mitigation that serves as something
like a shield (but magic) for the mage? Maybe some kind of absorption from
trinkets that only a mage benefits from.")*

The instinct was right and the first shape of it would have been wrong, which is
what measuring first was for. What was actually landing on each class at level
thirteen, three levels over:

| | guard | ward | hp | steel hits for | magic hits for |
|---|---|---|---|---|---|
| Fighter | 26 | 0 | 108 | 11.9 | 22.0 |
| Thief | 23 | 0 | 117 | 14.0 | 21.8 |
| Mage | 11 | 10 | 88 | 25.1 | 12.0 |

A Mage was already taking *half* what a Fighter takes from magic — the robe's
ward was doing its job — and more than double from steel. More ward would have
been an upgrade to the column they were already winning. So the answer is
absorption, which is indifferent to what the blow is made of.

And there was a structural hole underneath the numbers: **a Mage had three
equipment slots and everybody else had four.** The off arm was dead for them,
because the only thing that went on it was a plank they cannot hold and cast at
the same time. A quarter of the equipment system belonged to two classes out of
three.

**`ShieldTalisman`** fills it. Same slot, same shop counter, same struct; what
differs is that it carries `Absorb` instead of `Defense` and only a caster may
hold one. At the start of every fight it raises `EffectBarrier`, a pool that
every point of incoming damage comes off before anything reaches the body, of
any kind, until it is gone.

Spent rather than timed, and that is the whole difference between it and a point
of armour. Armour shaves every blow forever and is worth most in a long grind; a
barrier stops a fixed amount and then it is over, so it is worth most against the
opening exchange — which is the shape a Mage needs, because their pool is small
enough that the fights they lose are the ones where the first two blows land.
`Soak` is the one condition in the list consumed by being used, which is why it
does not go through `Apply`: `Apply` adds power and a barrier only loses it.

**Six tuning passes, and the interesting thing they found is that the Mage's
wins and deaths move together.** A barrier converts near-deaths into wins, so
every increase fixed the death cell and inflated the win rate by the same
motion; every damage cut did the reverse. Cutting the psyche curve to
compensate took the level-thirteen Mage from too strong to dying in 43% of
on-level-plus-two fights, which is a class that has stopped working rather than
one that has been tuned.

What broke the deadlock was noticing the *shape*. A Mage at thirteen was
bimodal: three castings of Unmaking that either finished the fight or did not,
and between them a bolt worth about five against a ward-nineteen dragon. Win
rate and death rate were high at the same level because the outcome was a coin
flip, and the DANGER brief — which wants a smooth ramp — punishes bimodality
from both ends. So the fix was to raise the *floor* and lower the *burst*:
`focusBite` went from 0.85 to 1.15 and the top of the Mage's damage list came
down (Unmaking 26 to 19, Scorch 14 to 11), leaving the same total output spread
across the rounds instead of piled into three of them. Death at three levels
over fell from 46% to 36%, which is the brief's own ceiling and the exact number
the game had before any of this session started.

Landed at Fighter −1.2, Thief +0.5, Mage +1.2 mean points against the numbers
from the beginning of the session.

Two smaller things fell out. `reportSlotValue` was reading the last row of each
sidearm band for its shield column, which since talismans exist is a thing that
blocks nothing — the column had gone to zeroes and one negative number. It reads
the best *buckler* now and prints the barrier as a column of its own, with a
paragraph saying the two are not in the same unit: a shield step is a point off
every blow forever, a barrier step is a lump stopped once. And the barrier's pip
in the party panel needed its own colour, because the default is weakness's
purple and a caster would have learned that they begin every fight cursed.

**And the ARCS section came back with something, which is what it is for.** Two
corrections and one finding.

The corrections. The duelist was skipping the off arm *by fiat*, and once
casters had something to put there that made "duelist" mean "the balanced build
minus a talisman" for one class in three — not a different build, a worse one,
averaged in and dragging the whole column to winning nothing at all. The arm is
closed by the weapon now, which `EquipAs` reads off the hand count, so a class
that cannot hold a two-hander simply does not have this build rather than being
charged for one. And giving it a best-in-tier charm on top took it straight to
winning seven levels out of seven: an archetype that *overspends* measures the
spec exactly as surely as one that underspends, which is the lesson this section
has now taught twice in opposite directions. The charm comes a band behind, the
same as balanced's, and nothing but the hands moves.

Two-handed weapons are also priced at parity with the one-handed top of their
band now. They cost thirty per cent more before, so the duelist was outspending
the comparison by ten to fifteen per cent and the cost column said so. Same
price, different shape, is a better shop decision anyway.

**The finding: at equal cost, the two-hander beats the shield at six levels out
of seven, by six to fourteen points.** That is not the talisman and it is not
new — it arrived with the weapon lanes and was previously masked by the duelist
being penalised in two of three classes. The cause is the one the WHY table has
been printing all along: a sidearm band is worth about a quarter of a main-gear
band, so giving up a shield costs almost nothing and picking up five points of
strike is worth a great deal. `TestShieldsStaySecondaryToArmour` is the rule
holding shields there, deliberately.

So the next thing this section wants is the shield table revisited on purpose —
the test allows up to half the body armour of the band, and shields currently
sit at a third of that. Doing it here would have been scope creep on a question
about mages; recording it is the report's job and this is the record.

**And the fixtures grew a caster, because the net had a class-shaped hole in
it.** Every fixture was a Fighter, so the focus weapon, the cloth lane, the
talisman and the three fields they added to the save format had nothing standing
on them. `caster.json` is a level nine Mage with a rod, a robe and a ward-knot.

Writing it found a real break in the fixture generator, which is the second time
this session that "reach into the table and take a row" has turned out to be a
thing that stopped being safe when lanes arrived. The affixed-weapon fixture took
the first affixable tier-four row in the file and handed it to the hero — which
is now a dagger going to a Fighter, and a two-hander going to somebody already
wearing a shield. `TestFixturesHoldTheRunInvariants` checks the whole set against
the class gate now: a fixture in a state the game cannot produce is worse than no
fixture, because it is a starting point for a playtest of a game nobody is
playing.

**And a bug the frame found that had been there since the first commit.**
Staging a caster's fight to look at the barrier turned up this in the transcript:

    {A} is suddenly behind them, which is rude and effective.%!(EXTRA string=Nolwenn Ash)

`model.Spell.Cast` was documented as taking a `"%s"` and passed through
`fmt.Sprintf`, and not one line in the table has ever contained one — they all
name the caster as `{A}`, the same placeholder the rest of the writing uses. So
*every technique cast in the history of the project* printed its raw placeholder
and a trailing format error into the combat log.

Nothing caught it because nothing reads the combat log except a person looking
at a captured frame, which is the failure mode this project keeps rediscovering
and the whole reason `-demo` exists. It is substitution now, and
`TestTechniqueFlavourNamesTheCasterAndNothingElse` refuses a line that has no
`{A}`, a stray `%`, or a placeholder nothing fills — the same scan
`internal/thread` already runs over the backstories.

The focus weapons' verbs went back into the data at the same time. The bolt was
logging a hard-coded "bolt" while every rod in the table carried an authored verb
nothing ever printed; they are "spark at", "object to", "overrule", "threaten"
and "sentence" now, alongside the mace's "clobber", because flavour is data and
a verb written into the battle screen is a verb the content files cannot revise.

**A technique says what it does now.** *(Jeremy's: "for the technique, maybe
left/right could show a popover or tooltip for a description.")*

Left or right in the technique list opens a panel over the transcript — the one
panel nobody is reading while they choose — naming the move and what it does.
The command panel is fifty-eight pixels holding three rows and every other pixel
on that screen belongs to something, which is why this had been open ever since
the two-sided techniques went in.

The text is **derived from the rules, not authored beside each row**. A `desc`
field on thirty-five techniques would be thirty-five numbers to keep in step with
a balance pass, and the first one that drifted would be a lie the player has no
way to catch. It quotes a real magnitude off `rules.SpellPower` for the character
holding it, so a better rod and a level-up both move the number on screen — and
it says the class surcharge out loud, because a Fighter reading "6 psyche" beside
a Mage's "4" for a similar-looking move is owed the reason.

`-demo` captures the screen with the popover open. That is one frame covering
two things rather than the tour drifting: the list is still visible underneath,
and a tour that skipped it would never capture the screen this feature exists to
be.

**The attack row names the weapon.** *(Also Jeremy's, mid-session.)* It said
"Attack" with the weapon in the detail column, which is the wrong way round
twice: the player knows the first row is the attack, and what they want off it
is which of the two things in the pack they are holding. It also could not have
survived the detail column being measured first — a thirty-four-character weapon
name would have squeezed the label out entirely.

Which turned up that `chooseRoot` dispatched on `b.menu.Index`. That is the bug
the pause menu already had once, and this menu was one row away from it: the
attack row's label is no longer a constant, and False retreat already appears
and disappears under everything else. The rows carry a `command` tag now, and
`-demo` drives the menu by that rather than by position.

**The off arm is three shelves.** *(Jeremy's: "let's balance the shield/sidearm
for both competitive scaling and possibly different playstyles via bonus effects
or humorous anecdotes.")*

Scaling first: the block ladder went from 1/2/3/5/6 to 1/3/5/8/10, which is the
ceiling `TestShieldsStaySecondaryToArmour` allows — half the body armour of the
band. Shields had been sitting at a third of what the rule permits.

Then lanes. Every band from tier one stocks a **wall** (most guard, costs
speed), a **silvered** one (guard traded for ward), a **spiked** one (guard
traded for strike) and a **talisman**, and each has an anecdote. The lane is
read out of what the item does rather than tagged beside it, for the same reason
`PactCost` is derived: the first shield whose bonus was retuned without its tag
would be filed under a lane it no longer belongs to.

**And that produced the finding the ARCS section exists for.** Raising the
shields barely moved the duelist's dominance — the two-hander still won six
levels of seven. The reason was in the WARD table all along: *nothing that
attacks with magic exists below level ten, and by thirteen two thirds of the
blows landing on you are magical.* A shield stops steel. So "give up the shield
for a two-hander" was being measured against the one shield that does nothing
about the half of the fight that matters.

`Archetype` grew an `Arm`, and a fourth build — **warden**, identical to balanced
in every slot and every band, differing only in which of the three things in the
sidearm band it picks up. At twelve thousand fights a point the order is:

    duelist  attrition  duelist  duelist  warden  warden  warden

Levels one to seven belong to the two-hander and nine to thirteen to the
silvered shield, which is the monster table's own shift from steel to magic read
back as a build. That is three arcs that are each right somewhere, which is what
this section has been asking for since it was written.

*(The second half of that reading did not survive being measured properly. The
shift from steel to magic is real and the WARD table still shows it; what does
not follow is that the answer to it is a shield. WARD prices the entire ward
slot at nought to three points even at thirteen against magical attackers only,
and the silvered shield pays three points of guard for fifteen of ward to buy
it. It is now the worst of the three lanes at the top of the game on the axis
that kills you. See "The fifth zero value, and the threshold nobody measured".)*

Balanced now wins nothing, and that is not a fault to fix. It is the
straightforward build, it is what `Equip` means, and every other number in the
report is measured against it — a middle option that is never best and never
worst is what a baseline is. The report says so in as many words.

Two interface corrections fell out. The shelf was grading all four lanes on
`Defense`, so a fifty-two-coin shrine plate read as "+1" — true of the number it
was measured on and useless about the thing being bought; it grades inside the
lane on that lane's own figure now, and a shield of a *different* lane counts as
nothing, because swapping the wall for the silver is a change of plan rather
than a step up a ladder. And the shop's description area went from two lines to
three, because fourteen off-arm entries with something to say for themselves was
cutting the last clause off half the shelf with no ellipsis to say so.

**Camping, and the number that produced it.** *(Jeremy asked what would make
this more enjoyable. The answer was a division nobody had done.)*

ENDURANCE says how many fights one rest buys. PROGRESSION says how many fights a
level costs. The quotient is how many round trips to an inn a level takes, and
nothing was watching it because no single section could see it:

    level  2   0.2 trips        level  9   2.5 trips
    level  4   1.5             level 12   4.9
    level  5   0.4             level 13   7.7
    level  7   2.1             level 14   7.8

A fortyfold climb, and eleven and a half at the far end of the finer probe. The
same walk, eleven times, for one level. Two things fall out. Most of the growth
is endurance collapsing — eighteen fights a rest down to two and a half —
rather than the XP curve, so the lever is the *trip* and not the fights. And the
column sawtooths on the gear-band boundary: 1.5 trips at level four and 0.4 at
level five, because the shop tier turns over. You are weakest at the end of each
band and strongest at the start, and nothing says so.

PROGRESSION prints the column now. That is the first half of the fix and the
half that lasts longest — a mechanic the report cannot see is a mechanic the
balance pass is lying about, and this is the first *pacing* one it has measured
at all.

**The second half is a camp kit.** Half of both pools back for the whole
company, where you stand. It deliberately does none of the other three things an
inn does: it does not fill the pools, it does not wake you at dawn, and it does
not write a checkpoint. Those are what a bed sells and they are why one is still
worth paying for at level fourteen. A camp buys the walk, not the safety.

Where and when you lie down is the decision. `rules.CampChance` reads the sky's
own prowl multiplier — the same one the encounter roll reads, so a clear night
is the dangerous one here for exactly the reason it is dangerous on the road —
how rough the country is, and whether you are indoors, which doubles it: a
dungeon is somewhere with things already living in it. Being found costs the
fight and most of the rest but not all of it, because a roll that took the whole
night as well would read as a punishment for having tried.

Two kits on the shelf, and the only difference between them is the odds. What
three times the money buys is not a better night, it is a smaller chance of it
going wrong.

**And the oddity finally is one.** *(Jeremy: "lean into the oddity part of
things too.")*

The asset budget has said since it was first counted that two thirds of the
bundle is sci-fi and cyberpunk "which this game has no use for — except as an
over-the-top joke zone, which is exactly what an oddity location is for". That
was a promise for the whole life of the project, and an oddity was a ruin with a
different tagline: the same blob of ground, the same lurking shapes.

It is a short paved strip now with the wrong furniture on it. The shape is half
the joke — whatever this was, somebody laid it out expecting traffic, and the
forest has come back up to the kerb on both sides. At the far end is a stairway
going down into the ground under a roof, with no building attached. There is a
lit humming box that takes a coin and gives you something cold, which the shop
code understands perfectly well as an apothecary. There is signage in a script
nobody in the realm writes, paint on walls that are not there, road barriers,
and bins that are bins rather than chests.

**Only the furniture was imported, and that is the joke working rather than a
limit.** The pack ships cyberpunk characters too and they are deliberately left
out: the people standing in an oddity are ordinary villagers with ordinary
sprites who treat a lit box as a wall with a slot in it. Somebody in the frame
dressed for the machine would be somebody on screen who is in on it, and the
rule this game's writing has followed since the first commit is that nobody ever
is. `oddityVoice` is a bank of its own for the same reason: the place has to not
sound like the rest of the continent, and a shared bank cannot hold a rule about
tone.

`world.OddityArt` is enumerated from the same slices the generator picks from,
so the audit checks every one. A vending machine that failed to resolve would be
a magenta box standing in a field, which is a description of the joke rather
than the joke.

**Encounters have a shape now.** *(Jeremy's, off the list of things that would
make this more enjoyable.)*

The complaint the report had been making about itself for a long time without
anybody hearing it: every build wins 96 to 100 per cent of on-level fights *by
design*, which is why ARCS and DANGER only ever compare builds three levels
over. The fight a player is supposed to be having is decided before it starts,
and the only thing that varies is how much it costs.

The fix is not to make on-level fights harder — that breaks the brief and turns
the game into a coin flip. It is to make them different from each other. And
everything needed was already built and unused: an armour axis and a ward axis
that no encounter ever put in the same room, multi-target techniques with
nothing to point them at, and speed and plating spread across the monster table
that `PickMonsters` averaged flat.

Five compositions, and the point of each is what it asks you to do rather than
how hard it hits:

- **mixed**, which is what the game threw before there were shapes, and stays
  the plurality at about 45%. A texture that happens every time is the texture.
- **pack** — more of them, each smaller and quicker. What a wide swing is for.
- **brute** — one of them, scaled up rather than picked from above its band,
  which keeps the promise an encounter level makes. The dungeon boss has worked
  this way since it was written.
- **escort** — something that attacks with magic standing behind something that
  stops steel. Kill order is the whole fight.
- **mismatch** — one thing that stops steel beside one that stops magic, so no
  single answer covers the room.

`PickMonsters` is untouched and still means "n creatures at level L": every
control in the report uses it, and a measurement wants one variable. The game
calls `PickEncounter`, which is that with a shape on it. `SimulateFight` split
into a thin wrapper over `SimulateGroup`, because a shape does not survive being
flattened into "n definitions all at level L" — which is the only thing the old
signature could say — and a mechanic the simulator cannot see is one the balance
pass is lying about.

The shape is said out loud, in the line the transcript already opens with. A
composition cannot be read off three portraits in the second before you choose,
and "one of them, and it is enormous" is the difference between a fight and this
kind of fight.

**What the new SHAPES section found, in four passes.** The first draft was not a
set of compositions, it was a difficulty spread: a pack won 37% against mixed's
83% and killed you 45% of the time. Cutting bodies swung it to 93%. It landed at
+2 bodies, 55% hit points and 72% offense — the offense cut is what did the work,
because attacks are what kill you and hit points are only what makes it take a
while.

The finding worth keeping is the mismatch, which was **backwards**. Picking the
two most *lopsided* creatures guarantees each is soft on the other axis, so a
fighter deletes the warded one in a round and grinds the plated one alone: the
shape reduced the effective number of enemies and came out the easiest fight in
the game at 99% win. It picks by *resistance* now — the best-armoured and the
best-warded thing in the band, each of which has to beat the other on its own
axis by three.

And that produced the second finding, which is a content gap the "seen" column
states rather than guesses at: **the mismatch is almost absent from the ordinary
biomes.** The fantasy rosters were never written to contrast. The one place it
appears reliably is the oddity, because that roster was authored for it — which
means the one place in the game where the matchup axis is the whole encounter is
the one where everything is the wrong century.

The controls did not move: fighter −0.6, thief +1.1, mage +0.4, unchanged to the
decimal, because `PickMonsters` is what they call.

**And the oddity got things to fight.** *(The other half of "find a way to get
some of those other assets in".)* Two more packs off the bundle: the sci-fi
character icons are 256px whole images from the same vendor family as the
monster portraits already in use, so eight new creatures slotted into the battle
screen with no rendering work at all. Mostly constructs — plating and no ward,
which is what stops steel and nothing else — with a couple that are the other
way round.

They are bureaucrats. A Maintenance Unit halfway through a task that has not
looked up, a Compliance Officer that says "you are not on the list, there is a
list", an Attendant that has swept the steps since before the village, and a
Courtesy Announcement that apologises for the delay while it kills you. The
magical ones sit at level ten and twelve, because the rule that nothing casts
before the answer is on a shelf is enforced by a test and applies to the joke
zone too.

**Then it went out to people, and the next five things came back from playing
it.** *(v0.1.0 is the first tagged build: two zips on the Releases page, macOS
universal and Windows, art and audio inside. The repository went public at the
same time — which the asset audit had already assumed it was, and which the
licences have always allowed: shipping the art inside a game is the use they
are written for.)*

Four of the five were Phase 0 again, and one of them exactly so.

**The companion errands.** Reported as "a little buggy — when they start it is
difficult to understand what you should do". Nothing was buggy. The tracker, the
compass, the journal, the map pin and `destinationOf` all already handled a
companion's backstory; `showThreadDestination` simply never called
`trackIfIdle`, where `showSagaDestination` two files away does, with a comment
explaining exactly why. And it opened with `if p.Discovered { return }`, so a
companion sending you somewhere you had already walked past produced *nothing at
all* — no pin, no compass, no line in the log, which is indistinguishable from
the story being broken. Revealing is a one-time thing and following is not. Two
lines, and a test that fails without them.

The journal had the matching gap: `Thread.Progress` answers only for the counted
triggers, so "walk to the ruin" showed a title with an empty column beside it.
It now says how far, in tiles, and only for the beats that are actually asking
you to go somewhere.

**The save timestamp** was reported as wrong-but-right-after-you-re-save, which
named the cause precisely: the age string was written into the menu row at
refresh time, so it was true for the instant the screen opened and drifted from
then on — and saving is the only action that refreshes. Written times on disk
were correct to the second in every file. Both halves, one cause, recomputed in
`Update` now. Also on the title screen, which is built at launch and then sits
there: `Continue — just now` stayed "just now" for as long as nobody pressed
anything.

**The seed was a number the game showed you and would not take back.** It was on
the title screen and on the pause menu and it is the thing the entire continent
is a function of, and the only way to set it was a command-line flag. There is a
World row at the top of the character screen now: type it, or left/right for a
fresh one. It rerolls the person as well as the world, deliberately — everything
on that screen is forked off the seed, so applying it to half of them would make
the same number mean two different runs depending on the order the keys were
pressed. A test compares a typed 1994 against a launched one, down to each
class's throw.

**The corner map** is the one thing here that did not already exist. The full
map answered "where am I" by taking over the screen, which means the question
could only be asked standing still and the answer had to be memorised before it
went away. This is the same information at a tenth the size: a sixty-four-tile
window of the continent, one pixel to the tile, with the followed destination
ringed on it — or, when it is off the window, the same arrowhead the status bar
uses, pinned to the border. The bearing was already there. What it could not say
was *where*, and the difference between a bearing and a position is the
difference between walking north-east and walking around a bay.

It paints the whole world into a texture and shows a slice, so walking east
moves the window and repaints nothing. The one bug in it was invisible to both
tests and reading, and obvious in the first frame: a sub-image draws from its
own bounds origin, so subtracting the window position from the translate — which
looks like the obvious correction — drew the slice up and left of its own
border, reading as a second panel bleeding off the corner of the screen.

**"Do the random messages from townsfolk mean anything?"** Half of them already
did, which was worth finding out before writing any: `townLine` has always had a
one-in-two chance of answering with a reaction to your reputation instead of the
villager's own line. What was missing was the other axis. A stranger could tell
you what they thought of you and never once notice the state you were visibly
in — walking around alone, or at a third health, or carrying four hirelings'
worth of unspent coin.

There is an `advice` block in `flavor.json` now, keyed on what is true about the
run, and a third of the time somebody says it. A third, because a town where
everybody tells you what to do next has stopped being a place.

The ordering in `adviceKey` is the part that matters. Money problems come before
every suggestion that costs money, so a broke player is never told to go and buy
a bed — that is the same failure as a shop row you cannot afford taking your
keypress and saying no, and it is the rule the counter and the hiring board
already follow by greying things out. A test walks the whole cross of purse and
health and asserts nobody is ever pointed at something they cannot do.

**And a star over anybody holding something.** A settlement draws eight or ten
identical villagers, exactly one of whom has the errand, and which one is a hash
of where they are standing — stable, and completely invisible. Since walking
into things became how you use them, the way to find that person was to bump
every human in town.

A star rather than the exclamation point, which does the same job without
reading as somebody else's icon. Pale for something on offer, gold for something
already yours: a finished errand to hand in, or an installment somebody has been
holding since you left.

The mark is capped the way the systems behind it are capped — one errand and one
story per settlement — so a town shows at most three. Marking every villager
whose hash said yes would have starred five people in a town that can produce
one, which is not a hint but a lie with a shape, and the player finds out by
bumping all five. A test pins the ceiling and fails at 5-of-7 without the
gating. A job already taken is deliberately unmarked: it is the one villager in
town whose news you have heard.

Three things about it were only findable in a frame. The mark drew *under* the
weather, because it went in with the sprites and `drawSky` runs after them —
invisible in rain, which is the weather you most want to be told things in. It
drew on the tall hirelings' faces, because character art is anchored on its feet
inside a generous box and a fixed offset assumes every head is the same height
above its tile; `Sprite.Head` is the mirror of the `Foot` that already existed
for the same reason at the other end. And its first colour was the exact tan of
a villager's hair.

**The hireling now waits outside the inn rather than in it.** A building's
interior is walkable floor and its door sits one tile inside the bottom wall, so
the five-tile ring `openNear` searched around that door covered most of the room
behind it. About a third of the time the person loitering outside the inn was
loitering in the middle of it, which reads as a man who lives there rather than
one waiting to be asked.

**Four things off a screenshot.** The star was too subtle — pale blue on grass
is a mark you find by looking for it, which is the one thing a mark must not be.
Both states are gold now, which is the one colour on this palette that nothing
in the world is, with a hard shadow two pixels down and right; they are told
apart by movement instead, the owed one pulsing to near-white.

**Selling was one keypress per object**, and a trip home from a dungeon is
twenty or thirty trinkets — a player holding a key down to convert a known
quantity of junk into a known quantity of coin, which is a chore with a menu in
front of it. A row now sells its whole stack, and the first row on the sell tab
clears out everything nothing is waiting on.

That sweep was one commit from being bound to `S`, which is the down key on
WASD: scrolling the sell list would have emptied the pack. As a row it needs no
binding, quotes its price in the same column as everything else, and is absent
rather than greyed when there is nothing to sweep. It also refuses to touch
anything an active fetch quest is counting — the single-row sell still can,
because that is somebody deliberately selling a named thing they are looking at.

**Buying still does not equip**, and that rule stays: equipping on purchase used
to destroy whatever came off, which is how a 240-coin glaive silently ate a
96-coin spear. But the rule was written to stop gear being thrown away, not to
make putting on a sword a trip to another screen. A strictly better piece now
asks on the spot, reusing the shelf's own comparison rather than a second
opinion about which of a rod's numbers matters. Only strictly better: a worse or
equal piece is a real decision, and a charm has no better at all by
construction.

**And the reported bug.** The shop's description strip and its purchase note
share one line, and the note wins while it is set — but nothing ever cleared it.
So a single purchase permanently disabled the one line that said what anything
was, and the shop went on describing the thing you had already bought. It is
cleared on cursor movement now: whatever the last deal was, it stopped being the
answer the moment you looked at something else.

**The status bar was overlapping itself**, and the cause was a column of fixed x
offsets that had never met a long name. "Sister Agatha Blunt Two Drinks In"
printed straight through the weather; 1225 coins printed straight through the
tracker. Both rows are laid out from an anchor now, with the variable parts
fitted to what is left.

Which thing gives way is decided by which thing *moves*. A hero's name is fixed
for a whole run, so the clock is laid out after it and sits in the same place
every frame; anchoring on the place instead would slide the clock about every
time you walked from a wood into a town. And the place is the one of the three
repeated elsewhere — floating over the location, and on both maps — so it is the
one that costs least to cut.

**The battle screen is side-on now.** Monsters used to be a row across the top
with the transcript across the middle and the company sharing the bottom with
the command list, which put the two things you are comparing — your people and
their people — at opposite ends of the screen with a wall of text between them.
Deciding who to hit meant looking up; deciding whether you could afford to meant
looking down.

So: the company down the left, one above another, with faces big enough to be
faces and hit points as a number rather than only a bar. Whatever is in front of
you on the right, in a grid — three across at most, and four goes two-and-two
rather than three-and-one, because four is the most an ordinary encounter sends
and a square reads as a group where a row with a straggler under it reads as a
mistake. The two things that are words rather than pictures share the bottom.

The command panel overdraws the left end of the bar rather than sitting beside
it, so the transcript runs the whole width and only gives up room while there is
something to press. When the round is resolving there is no panel at all —
which is the one moment anybody reads a combat log — and the victory summary,
experience and coins and what fell out, gets the bar to itself with "Press Z"
tucked in the corner.

That change immediately produced a line reading "Weather settles into place. 8."
at the top of the panel: the wrapped log filled its last row with the tail of a
sentence whose beginning did not fit. Not a shorter version of what happened, a
different and wrong one, with nothing on screen to say it had been cut. An entry
goes in whole or not at all now — three things that happened beats three and a
half.

Up and down now step the target cursor by a row. They did nothing at all when
the foes were a single line, which was honest then and would read as a broken
key in front of a two-by-two.

**And the names came apart.** Half the roster is written "Crab, Territorial" — a
species and a characterisation, where the characterisation is the joke — so
printing both under a portrait meant the funniest column in the tables was the
half that got truncated away. "Goblin Middle Manag." says neither thing.

The field shows what it is; the epithet waits for the target cursor, where there
is a whole panel to say it in and where it is finally worth reading — the player
is looking at four portraits deciding which to hit, and "Territorial" is the
entire answer to what sort of crab this is. The group letter stays with the
head, or two crabs would label identically at the exact moment you are choosing
between them.

The other half of the roster is not written that way at all: "Owl That Knows",
"Something That Was A Diver" are whole phrases and the phrase is the joke. A
test asserts the split is lossless rather than clever, and the name wraps to two
lines whenever the field is a single row — which is what rescues those.

**And then the names went back together.** *(Jeremy's: "we should help the
jokes land somehow with showing name + subtext for all the monsters. I think
that's taking some of the fun out of things without it.")*

He is right, and a captured frame showed it was worse than a preference. Five
creatures were labelled "Goblin Middle" and "Overfamiliar" — the first row of a
wrapped name with the rest silently dropped, and nothing on screen to say so.
That is the transcript's own bug, the one two paragraphs up, moved under a
portrait: a thing shown in part, reading as a different and shorter thing.

So the plate carries both halves now, always. The species in ink and the
epithet under it in dim, because the plate should answer "which one do I hit"
before it answers "what am I looking at". The vertical room was there and
nobody had asked for it: a single row of creatures has 176 pixels of field to
hang a 96-pixel portrait in, so the fourth line is free. The stacked layout has
88 a row and pays for its third line with the portrait, down to 36 — which is
the party panel's size, and the trade is worth it, because a six-strong pack is
the case where the labels are doing the most work.

The line counts are not taste. They are what the widest names in the roster
need at three columns — "Living Armour" is two lines by itself and "Two People
Inside" is another two — and `TestEveryMonsterNameFitsItsPlate` is what said
so, having first failed at three. It walks the whole roster against every
layout from one creature to six, which is what a pack shape sends, so the
failure it catches is somebody adding a monster whose name is longer than the
shelf it has to sit on.

**And prose went the other way, to the species alone.** The taunt panel read
"Wolf, Deeply Unimpressed has watched you fight before. Wolf, Deeply
Unimpressed was not impressed then either" — the joke told twice, in the middle
of a sentence, where it is a parsing problem rather than a joke. `{T}` and every
combat line take `Monster.Short()` now. The two halves finally do different
jobs: the plate is where the joke is read, and the transcript is where the
fight is. They also compose, which was the accident worth keeping — the plate
says the wolf is Deeply Unimpressed and the taunt says it was not impressed
last time either.

`model.SplitName` is where the comma rule lives now rather than beside the
battle screen, because the writing needs the same split and two copies of
"find the comma, keep the group letter with the species" is two copies to
disagree.

**The icon went inside its own name.** *(Jeremy's, on a frame of five: "we
should have the first part of the name above, and the other 2 lines below each
icon. I think properly grouped that would work better.")*

All three lines had been underneath, which is a caption rather than a label: in
a field of five there were as many gaps as names and nothing said which went
with which. The species sits over the portrait now and the epithet under the
health meter, so each creature is inside its own writing.

The plate is measured from the names actually in the fight rather than fixed
per layout, because the two are different questions. Three wolves and a goblin
need two lines above — "Goblin Middle Manager A" does not fit one at three
columns — and two below; two short-named creatures need one and one and should
get a bigger portrait for it. The field takes the maximum each way so the row
stays level, since portraits of different sizes in one row read as a rendering
fault rather than as a fit.

Two details did the actual work. The species hangs *from the portrait* rather
than from the top of the space reserved for it, or a one-line name in a
two-line plate floats a whole line clear of its own picture — which is the gap
the rearrangement was meant to close. And `monPlateFits` is a pure function
separate from the measuring, because that half has a right answer: the vertical
room is finite, the portrait pays for every line, and when something has to
give it is the epithet, since the species answers "which one do I hit" and the
epithet only answers "what sort of thing is that".

**What it cannot fix is the crowded field**, and that is worth writing down
because it is the argument for the next idea rather than a defect to patch. A
stacked layout has 88 pixels a row and three columns of 107. At that width
neither "Goblin Middle Manager A" nor "Deeply Unimpressed" fits one line, so a
five- or six-strong pack spends its plate on two lines of species and one
truncated line of epithet over a 34-pixel portrait. Every part of that is the
same shortage: too many separate slots for the room available.

**Which is what staggering identical creatures would solve.** *(Jeremy's:
"instead of A/B/C guys, we could stagger the icons to show how many,
potentially only allowing the front one to be damaged. We'd have to test, but
it would add depth to combat.")* Three wolves are three slots, three copies of
the same picture and three copies of the same joke — the wallpaper argument the
labels already lost once. One slot with three icons offset behind each other is
one name, one epithet, one big portrait, and a field of six becomes a field of
two. The layout problem disappears rather than being managed.

The mechanical half is a real change and not a free one: "only the front one
can be hit" turns a group into a queue, which is a different fight. It makes
area attacks worth having, it makes the order the game kills things matter, and
it takes away the player's choice of which of three identical wolves to finish.
That is depth and it is also a balance question — `SimulateFight` would have to
learn the rule before the report could say what it costs. Untried, and worth
trying in that order: the simulator first, then the screen.

**Labels grew a third band.** Gold used to mean the one thing you were pointed
directly at and grey meant everything else in range, which made being told which
shop is which a matter of facing its door. Gold is a radius now.

Beyond it, a person becomes "someone" rather than giving their name — a name
handed over from six tiles away is a name you never quite met anybody to learn —
and the first draft of that put "someone" over every villager in the capital,
where two of them standing together overlapped into "someonesomeone". Which is
the whole argument against it: ten of them is wallpaper, and wallpaper teaches
the player the word means nothing. It is drawn only over somebody who actually
has something now, which is what makes it mean go and look.

Shops, inns and signs keep their names at every distance. Their name is not a
reward for arriving, it is how you find the armourer without trying every door,
and that was the problem the labels were added to fix in the first place. A test
asserts the line between the two.

The star does not hand over to the tag; it moves out of its way. Standing down
was the first answer, on the grounds that two marks for one fact is one too
many — which is true of the fact and wrong about the mark. A star is what the
eye catches while crossing a street, and having it wink out at four tiles means
the thing you were walking towards stops being marked at the moment you commit
to walking towards it. A name answers a different question from a star. So when
a tag is up the star is lifted clear of the plate, which is what the geometry
was asking for all along.

And shops and inns are labelled from anywhere in the town they are in. Four
tiles was the range, and a town is fifty-odd tiles across, so the sign that
exists to point you at the armourer could only be read once you were close
enough to have already found the armourer. Directions are useless at the
destination.

## Testing without a screen

*(Jeremy's: "would be interested in figuring out testing in an offscreen view
rather than needing my laptop screen to be on and available.")*

**The diagnosis first, because it is not what it looks like.** Ebitengine's
`internal/ui` runs `newUserInterface()` at *package init*, so importing
`ebiten/v2` — which `internal/game` does, directly and through `assetsys` — is
enough to trigger it. That enumerates monitors, and with none it dies. On
darwin it dies badly: `currentMouseLocation` dereferences the nil that
`primaryMonitor` is documented to return, which is a segfault rather than an
error, and it happens with `-run XXX` and no tests selected at all.

Two things worth knowing about it. The nil deref is an upstream bug and not the
real obstacle — `initializeGLFW` twenty lines further on checks for exactly
that case and returns a clean `"ui: no monitor was found"`, so fixing the
darwin path only turns a segfault into a legible panic. Ebitengine has no
headless mode and no environment variable for one; `EBITENGINE_GRAPHICS_LIBRARY`
picks between Metal and OpenGL, not between having a screen and not. And
v2.10's alphas have not fixed the deref either, so there is nothing to upgrade
to.

**And "no monitor" includes a display that has gone to sleep.** That is the
whole of the intermittency: the suite passes, the laptop idles out, the next
run segfaults, and twenty minutes later it passes again.

So the answer is not to go without a display, it is to use one nobody is
looking at. Linux has Xvfb; macOS has no equivalent, which is why
`scripts/test-headless.command` is a container rather than a script. It runs
the whole sweep — `go test ./internal/...`, `vet`, `-audit`, `-demo`,
`cmd/balance` — against an X server drawing into memory, with Mesa's llvmpipe
supplying the GL. First run compiles Ebitengine's cgo and takes a few minutes;
the build and module caches are named volumes, so every run after is about a
second.

Two things it cost to get right, both of which look like hangs:

- **`xvfb-run` never ran the command.** The X server came up, `go version`
  never executed, and the container sat at nought per cent with an empty log
  for fifteen minutes. Whatever its wait-for-ready loop wants is not present in
  a container with no session. Starting Xvfb directly and watching for its own
  socket is four lines and cannot hang for a reason nobody can see.
- **There is no sound card, and the game exits over it.** `oto` opens ALSA's
  `default` device when a real bank is built, which `-demo` and `-audit` both
  do, and the game quit on an audio error before drawing anything. A null PCM
  in `/etc/asound.conf` is the honest fix: the tour runs silent by design and
  the audit is checking that files decode, not that they can be heard.

The host path still works and is still the fast one when the screen is awake.
This is the fallback that does not care.

## The thief's off arm, and a fifth zero value

*(Jeremy's: "Worth adding dual wielding for the thief? Seems like that class is
lacking in flavour just a little.")*

The instinct was right and the cause is not what it looks like. The thief is
not short of options — it has the **widest** weapon access of the three (18 of
27 weapons, against the fighter's 15 and the mage's 11) and the same number of
techniques as the fighter. What it lacks is one specific thing, and the
adversarial review had already brushed against it: every class's off arm can be
traded for something except the thief's.

| class | the off arm can be |
|---|---|
| Fighter | a plank, or closed for a two-hander |
| Mage | a talisman — its own slot, in its own unit |
| Thief | a plank |

So chasing the flavour question found the mechanic, and then measuring the
mechanic per class found a fifth instance of the defect this document has now
spent four rounds on. **`gamedata.LaneForLevel` takes a level and no class**,
and the right lane is not the same for both classes that can hold a plank:

| level | | wall | spiked | silvered | handed out |
|---|---|---|---|---|---|
| 11 | Thief | 62.6% | **73.5%** | 67.6% | silvered |
| 12 | Thief | 52.7% | **64.2%** | 59.1% | silvered |
| 13 | Fighter | 59.7% | **80.0%** | 69.3% | silvered |
| 14 | Fighter | 66.7% | **77.9%** | 69.3% | silvered |

The spiked lane — guard traded for reach — is the best off arm at the top of
the game for *both* plank classes, by up to eleven points, and the game hands
out the silvered one because one global constant decides for everybody. That
constant was set off the averaged version of this table, by me, three commits
ago.

**It is not being changed on this measurement.** Three thousand fights a cell
is enough to see an eleven-point gap and not enough to re-pin a constant that
moves every number in the report, and the last four rounds of this have all
been "decide, then measure". The next pass measures first, per class, at the
sample the DANGER table uses, and only then moves `LaneForLevel` to take a
class.

*(It did, and the numbers in the table above are half noise — see "The fifth
zero value, and the threshold nobody measured" below. The conclusion held, the
per-class part did not: the constant is now the wall below ten and the spiked
shield above it, and it stayed class-blind because the two plank classes cross
within a level of each other.)*

LANES is per class now regardless, which is an instrument fix rather than a
content one. It also exposed that a Mage's three columns are the same build
measured three times — it cannot hold a plank, so `pickSidearm` falls back to
the talisman whichever lane is asked for — so the old averaged row was
answering a question about two classes with a third mixed into it.

**On dual wielding itself.** The honest reading is that the thief already has
an offensive off arm and the game is not giving it to them, so the first fix is
the lane rule and not new content. What dual wielding would add on top is real
but smaller than it looks: the strike lane is a *sidearm*, so
`TestShieldsStaySecondaryToArmour` caps what it can carry, and a spiked shield
steps +2 where a weapon band steps +5. A second weapon in that arm would scale
on the weapon table instead, which is the one legitimate way past that ceiling
— it is not a shield, so the rule that holds shields under half the body armour
of their band does not apply to it.

That is worth having, and it is a bigger job than it sounds: a fifth equipment
slot means a new field in the save (nil for "none", which is the safe zero),
a rule for what a second weapon actually does, `SimulateFight` taught to model
it before the report can price it, a lane for `EquipAs`, a row on a character
sheet that is already at its height limit, and a shop that sells it. The order
is the same one the stagger idea gets: the simulator first, then the screen.

And the flavour half is nearly free either way. "Spiked Boss, Regrettable" is
already a thief's offensive off arm wearing a shield's name; a second dagger in
the strike lane is the same mechanic with the right fiction on it.

## Parry, and whether the classes actually play differently

*(Jeremy's: "Wondering if a parry bonus (instead of block) might apply to the
thief to help keep that survivability balanced, but maybe that's already there
so no harm no foul. That's closer to illusion than actual difference, and
hopefully we're starting to converge on actual different gameplay for the
different classes, even if ultimately it's the same (kill stuff, don't die).")*

**It is not already there.** There is no evasion of any kind on the player's
side. `MonsterDamage` rolls between 0.35 and 1.35 times a creature's Offense
and subtracts flat `Defense` — or `Ward`, if the attack is magical — and that
is the whole of defence. Every monster attack connects. Dexterity feeds the
*player's* miss chance, the crit roll and the feint odds; Speed feeds
initiative and the flee roll. Neither does anything defensive.

Worth noting on the way past: the miss roll runs one direction only. A player
can whiff against a fast creature; a creature never whiffs. That is probably
right — it keeps incoming damage predictable, which is what the DANGER table is
built on — but it means "dodge" already exists in the maths and only the
monsters are exempt from it.

**And it would not be illusion.** Flat reduction and a chance to negate are
different arithmetic against a spread roll, not two names for one thing:

- Flat −N comes off every hit and clips the small ones to nothing. It is worth
  most where hits are many and small.
- A chance to negate removes a share of the *total* regardless of hit size. It
  is worth the same against a big hit as a small one, so relative to armour it
  gains as hits grow.

Monster Offense grows with level, so those two curves diverge with level rather
than staying parallel. A parrying thief and an armoured fighter would not be
the same character with different words on the sheet: they would take damage in
different shapes — the fighter chipped steadily, the thief untouched and then
hit hard — and the thief's existing kit already leans that way. It has no
healing technique at all, its sustain is items, and its signature move is a
gamble. High variance is what that class already is.

There is also a precedent for the shape of the change. The Mage's talisman
carries `Absorb` rather than `Defense`: a pool spent once per fight against
damage of any kind. That is already defence in a different *unit* for one
class. Parry would be a second, and it would go to the class that currently has
none.

**On the wider question, which deserves a straight answer.** The classes are
differentiated today almost entirely *outside* the fight. The thief has no
healing, buys two restoratives for one, works the drop table of creatures that
ran, and can feint. The mage has the talisman, the deepest psyche pool, and
spell damage that scales on psyche and focus. The fighter has heavy armour and
the two-handed lane. Those are real and they are mostly economic.

Inside the fight all three run one loop: roll damage, subtract guard. The only
class-shaped terms in that loop are Focus, which exists for the caster, and the
miss and crit terms, which read Dexterity. So the observation is accurate — the
combat core is currently the same game three times, and the differences sit
around it rather than in it.

A parry term would be the first thing to make the *defensive* maths differ by
class, which is the half where the sameness is most felt.

**Half of it is built, because half of it was a hole rather than a feature.**
*(Jeremy's, in the same breath: "we should change flee to auto-defend, and have
a higher chance to have monsters miss. Nothing like starting a fight you know
you can't win and can't flee and die.")*

A failed escape used to cost the entire round and do nothing at all — "turns to
run and immediately reconsiders", and then every creature present hit for full
against a character who had spent their turn. `FleeChance` floors at ten per
cent, which is where a slow company against a fast creature sits, so escaping
from there expects nine failures. Nine rounds of damage taken with nothing
given back and nothing turned aside, against monsters that cannot miss. A whole
fight is about six rounds. That is not a risk with a button on it, it is a
death spiral, and it fires exactly in the situation the button exists for.

A failed flee braces now: the whole party, since fleeing is a party action and
it would be strange for the one who called the retreat to be the only one who
stopped covering up. `Defending` already halves incoming damage and the Defend
command already used it; the round is no longer spent on nothing.

`SimulateFight` learned it in the same commit, because a mechanic the simulator
cannot see is a mechanic the balance pass is lying about — and this one lands
on the exact measurement the whole flee feature was built to fix. The report
would otherwise have gone on pricing retreat as strictly worse than it is.

The other half — monsters missing — is the parry idea in its general form and
stays unbuilt for the reason below.

**And it is the largest-blast-radius change proposed so far**, which is the
reason it is written down here rather than started. `MonsterDamage` is what
every number in the report is built on: DANGER, ENDURANCE, ARCS, LANES, WARD
and the whole curve. Adding a term to it re-tunes all of them at once, and the
last five findings in this document were all instruments lying about content.
The order is the one the stagger and dual-wield ideas got, and more so here:
teach `SimulateFight` the term, run the report, read what moved, and only then
decide what the number should be.

## Class identity: the shape it should take

*(Jeremy's, opening the design rather than the build: "Assuming we add parry +
dodge for the thief, parry + block for the fighter, and absorb + reflect for
the mage. Do we also need additional offensive uniqueness? ... sometimes you
could gain bonus actions as you leveled ... Maybe we give that to the fighter,
and a counterattack on parry for the thief? And maybe vampiric shielding or
healing for the mage?")*

The problem this is answering is real and measured: the classes are
differentiated almost entirely outside the fight — the thief's item economy,
the mage's pool, the fighter's two-hander — while inside it all three run one
loop, roll damage and subtract guard. What follows is the intent, not a build
order, and the parts of the proposal that were changed are changed for stated
reasons.

### One unit each, and they must be different arithmetic

**Parry and dodge are the same mechanic under two names.** Both are a chance to
take zero. Assigning parry+block to the fighter and parry+dodge to the thief
gives the *fighter* two structurally different defences and the thief the same
one twice, which is backwards from what the scheme is for. So:

| class | unit | worth most against | how it feels |
|---|---|---|---|
| Fighter | **block** — flat −N off every hit | many small hits | predictable chip |
| Thief | **dodge** — a chance of nothing at all | indifferent to hit size | untouched, then hit hard |
| Mage | **absorb** — a pool spent once | a single burst | fine until it is not |

Those are three different curves against a spread roll, and they diverge as
monster Offense grows rather than staying parallel — which is the whole test of
whether a difference is real or cosmetic. Two of them already exist: `Defense`
is block, and the talisman's `Absorb` is already defence in its own unit for
one class. Only dodge is new.

### The active layer hangs off the unit

- **Thief: a counterattack on a successful dodge.** Jeremy's, and the right
  shape: it converts defence into offence, which is the framing this document
  already uses for the class — "the monster cannot hurt you if it is not
  there". It also inverts something: a thief surrounded gets more counters, so
  being outnumbered stops being uniformly bad for the one class whose plan is
  not being hit.
- **Mage: vampiric shielding, not vampiric healing.** Also Jeremy's, and the
  first half is the better one. The mage already has the deepest psyche pool
  and the only healing techniques in the game; lifesteal would add sustain to
  the class that has the most of it. Damage dealt refilling `Absorb` feeds the
  unit the class already owns, and makes its defence something earned by
  attacking — a different loop from the other two rather than a bigger number.
- **Reflect is left out** unless the mage still reads thin after that. Absorb,
  a vampiric refill and reflect is three defensive terms on the class that is
  least short of identity.

### Offensive uniqueness: not three, and not yet

The defensive units already produce offensive difference, because each one
*fails* differently and that changes what a player does about it. The genuine
gap in the scheme is that the fighter has no active — and bonus actions fill it
exactly.

**Which is why bonus actions are the one to hold.** A weapon band is +5 strike.
An extra attack is a second entire swing: on the rounds it fires it is worth
more than every gear step in the game put together. The arcs work in this
document just spent itself getting three builds within 7.2 points of each other
at equal spend, and one class taking occasional double rounds would blow that
open in a way no gear table could answer — you can re-price a charm, you cannot
re-price "sometimes twice".

So it is wanted, and it goes last and alone.

### Step one, built: dodge

*(Jeremy: "let's build your proposal and see how it shakes out in practice.")*

`rules.DodgeChance` is Thief-only and reads **Speed**, which until now did
nothing defensive at all — initiative and the flee roll were its whole job —
and which the Thief rolls highest by a clear margin, 9-13 against a Fighter's
6-9. Dexterity was the alternative and is already spoken for: the player's own
miss chance, the crit roll and the feint, whose note is explicit that footwork
of the deceptive kind is dexterity. Getting out of the way is not a lie, it is
a race.

The mechanic went into `SimulateFight` before the battle screen, and the
constant was chosen from a sweep rather than picked and confirmed. **Both of
those mattered, because the first two attempts were wrong.**

**The first sweep measured nothing.** It varied `dodgeBase` and reported almost
no effect — because the speed term dominates the base, so "base 0%" was already
a 16% dodge at level seven and "base 20%" was the 30% cap. A sweep that never
varied the thing it was sweeping, which is the same defect this document has
spent a month finding in other instruments. Sweeping the finished probability
from nothing to full is what actually priced it.

**Then it was measured on the wrong axis.** The Thief is behind on endurance —
5.9 fights to a rest against a Fighter's 9.3 at level seven — so endurance is
where dodge was tuned, and at full strength it brought the class to parity
there. It also took the Thief's stretch win rate from 68.3% to 82.8% at level
eleven and 71.2% to 86.7% at thirteen, making it the best survivor in the game
by eighteen points over the Fighter. A dodge is worth far more in one fight
than across an afternoon: thirty per cent of blows missing is thirty per cent
less damage *now*, while over a chain the Thief's small pool and absent healing
reassert. The constant has to be set on the axis that binds and checked on the
other.

**And the endurance gap was the wrong target anyway.** ENDURANCE says "no
potions" in its own header, and the Thief's sustain *is* potions — it has no
healing technique at all and buys two restoratives for one, which SUPPLIES
measures precisely because the fight simulator cannot. So the gap dodge was
being aimed at is understated by construction in the only table that reports
it.

**The third shape is the one that shipped.** The original scaled at two points
of chance per point of speed, so it grew from 16% at level three to a pinned
30% from eleven — a defence that compounded fastest exactly where the class was
already strongest and did least where it was weakest. Flattened to 8% base,
0.4% a point, capped at 12%, it runs 9-12% across the whole game and is worth
+3.3 to +7.3 points of stretch win rate at every level rather than +15 at the
top.

Where that leaves the Thief, against the Fighter, on the stretch fights:

| level | 3 | 5 | 7 | 9 | 11 | 13 |
|---|---|---|---|---|---|---|
| Fighter | 80.2% | 96.5% | 99.3% | 88.8% | 71.6% | 68.2% |
| Thief, no dodge | 66.5% | 65.4% | 85.0% | 71.4% | 68.2% | 72.2% |
| Thief, with | 70.8% | 72.7% | 88.3% | 74.7% | 74.7% | 78.0% |

Behind at four levels of six and ahead at the top two, where it was already
ahead before the change. That is a class arc rather than a buff: squishier
early, and coming into its own as the speed advantage compounds — which is the
fantasy the class is named for.

`TestDodgeStaysFlatAcrossTheLevels` pins the shape rather than the numbers: the
rate may not vary by more than six points across the speed gaps the real
rosters produce, because a defence that compounds with level is the thing that
had to be fixed twice.

### Step two, built: the counterattack a dodge earns

Jeremy's, and the shape was right: it converts defence into offence, which is
the framing this class already had — the monster cannot hurt you if it is not
there — and a creature that has just committed to a blow is a creature with its
weight in the wrong place.

It reads `PlayerDamage`, so the counter carries the thief's weapon, its strike
and the target's armour exactly as an ordinary swing does. It does not carry a
crit roll: a free hit that can also spike is two pieces of luck on one event.
And it is worth 55% of a swing rather than a full one, which sits deliberately
below the feint's bonus — the feint is a decision the player made and paid for
with a whole round if it fails, and a thing that happens to them for free must
not hit harder than a thing they chose.

**The measurement said nothing until it was taken against a group**, and that
is the part worth keeping. One-on-one, every weight from nothing to a full
swing moves the stretch win rate by one to three points, non-monotonically —
noise. Which is not surprising once stated: a dodge fires on *incoming* blows,
so its value scales with how many things are swinging, and every section of the
report that sets the curve fights one creature at a time.

Against three creatures on level:

| level | Fighter | no counter | at 0.55 | at a full swing |
|---|---|---|---|---|
| 5 | 84.0% | 34.7% | 38.3% | 41.9% |
| 9 | 40.1% | 8.8% | 12.6% | 13.9% |
| 13 | 33.6% | 31.0% | 34.8% | 39.3% |

About four points at the chosen weight and eight at a full swing. So the
mechanic is negligible where the curve is set and meaningful where the fantasy
lives, which is the right way round.

**Two things that probe turned up and did not fix.**

The first is a hole in the measuring rather than the game: three creatures
*three levels over* is a win rate of nought for every class including the
Fighter, so the stretch column — which is how every build in this document is
compared — is saturated and useless for group fights. Nothing here reads it
that way.

The second is a class gap nobody has looked at. Against three on level at level
five the Fighter wins 84.0% and the Thief 34.7%; at five creatures it is 9.0%
against 0.4%. That is a forty-nine point gap and it is not a bug — flat
reduction comes off *every* hit, so armour is worth more the more hits arrive,
which is precisely the property the design section claims for block. It is the
scheme working. But it means the Thief's dodge, being a chance rather than a
subtraction, does not scale into a crowd the way armour does, and no section of
the report shows per-class group fights at all. Worth a section before anything
else is tuned for crowds.

### The blind spot, measured: CROWDS

The counterattack probe turned up a hole big enough to stop for, and it is not
a tuning question. **Two blind spots met.** SHAPES measures compositions but
rotates the class per fight, so it reports an average and cannot see a class
that folds in a crowd. And the stretch column — three levels over, which is how
every build in this document is compared — is *saturated* for groups: three
creatures three over is a win rate of nought for every class including the
Fighter. So the comparison every conclusion here rests on says nothing whatever
about group fights, and groups are most of the game. SHAPES puts mixed at
42-57% of encounters and packs at another 21-25%.

`CROWDS` measures it: per class, per level, on level, from one creature to six.

| creatures swinging | widest gap between two classes at one level |
|---|---|
| 1 | 4.6 points |
| 2 | 34.6 |
| 3 | **70.6** |
| 4 | 54.4 |

The classes sit within five points of each other one-on-one and seventy apart
against three. Everything this document has concluded about class balance was
measured in the left-hand column.

**And the class in trouble is the Mage, not the Thief.** At three creatures on
level it posts 4.0%, 12.4%, 14.8%, 4.4%, 29.2% and 3.2% across the levels,
against a Fighter's 17.8, 83.0, 52.8, 40.8, 94.4 and 33.8. The section warns on
it: the Mage is the class left behind in fourteen of the cells where the fight
is winnable by anybody at all.

That is the design's own prediction arriving as a measurement. Absorb is a pool
spent once, which is the worst possible unit against a crowd — it is gone after
the first blow and the next five arrive against nothing. Flat reduction comes
off *every* blow, so armour is worth more the more blows there are; a chance to
take nothing is worth the same share whatever the count. Those three units were
chosen to diverge and they diverge hardest exactly here.

Which makes **vampiric shielding the right next step for a reason better than
the one it was proposed for**. It was on the list to give the Mage something
earned by attacking; what CROWDS says is that the Mage's unit does not survive
contact with more than one creature, and a pool that refills as you deal damage
is a pool that answers a crowd. The gap this has to close is measurable and
large, which is a better brief than "the class could use some flavour".

Two caveats the columns cannot state for themselves, both in the section's own
prose. Four and six creatures are party-sized rolls measured against one
character, since `EncounterSize` scales what the world sends with how many
people walk behind you — those columns are "a solo hero after their company
died", not what a party of three walks into. And level eleven is soft for
everybody, because mountain's roster near that band is weak: every class posts
its best numbers there and that is a fact about the monster tables.

The section also had the aggregation defect before it shipped. Its first
version took the widest gap across every level at once, which compares a
Fighter at eleven with a Mage at thirteen and calls the difference a fact about
classes — the same defect this report has been finding all month, written fresh
into the instrument built to find it.

### Step three, built: the mage's barrier refills

Proposed as flavour — something the class earns by attacking rather than
another number — and CROWDS gave it a far better brief before it was written: a
barrier is a pool spent once, which is the worst possible unit against a crowd.
It is gone after the first blow and the next five arrive against nothing.

`rules.Siphon` turns 35% of damage dealt back into barrier, capped at twice
what the talisman put up. The gate is the *item* rather than the class, because
the talisman is the unit — a Mage holding nothing on that arm has no pool to
refill, and nobody else can hold one at all. It goes through the one closure
every kind of player damage passes through, so a swing, a technique, a feint
and a counter all feed it; a siphon that only some of them fed would be a
mechanic that depended on which button was pressed.

Deliberately **not** healing. The Mage already has the deepest psyche pool and
the only healing techniques in the game; lifesteal would hand more sustain to
the class that has the most of it, and would be a fourth answer to "do not die"
rather than a repair of the one it owns.

What it did, and where it was measured:

| Mage against three, on level | 5 | 7 | 9 | 11 | 13 |
|---|---|---|---|---|---|
| before | 12.4% | 14.8% | 4.4% | 29.2% | 3.2% |
| after | 42.4% | 47.6% | 14.2% | 58.4% | 17.0% |
| Fighter | 83.0% | 52.8% | 40.8% | 94.4% | 33.8% |

And on the whole table: the widest gap between two classes at one level falls
from 70.6 points to 43.0 at three creatures and from 54.4 to 38.8 at four, and
the section's warning goes from the Mage trailing in fourteen of the winnable
cells to seven. Still behind everywhere, which is correct — it is the fragile
class — but it can now fight a crowd rather than folding to one.

**One-on-one it does nothing at all**, at any share from nothing to 0.80,
because a duel is already a hundred per cent for every class at every level
below thirteen. That is the point rather than a disappointment: the mechanic
cannot disturb the curve, because the curve is set in the column it does not
touch. It is also why it could never have been priced there — the same trap the
thief's counter fell into one step earlier.

Level three is unchanged because the Mage has no talisman at that tier yet. The
unit arrives when the item does, which is how the off arm has always worked.

### Step four, built: the fighter swings twice, sometimes

The fighter's active, and the last of the four because it is the largest thing
in the scheme. It is also the smallest number in it, deliberately.

`rules.ExtraSwing` is Fighter-only and reads Speed above a floor of 14 — where
a Fighter arrives around the middle of a run, since it rolls 6-9 at level one
and climbs about a point a level. So the reward is for having levelled rather
than for the class one picked, which is what "as you levelled" meant. It grows
at 1.5% a point past the floor and stops at 15%, which puts a top-end Fighter
around one round in ten.

Only a plain swing repeats. A technique repeating would spend the psyche twice
for one decision, and a flee or a feint repeating is not a sentence that means
anything.

It is the right shape for the class the tropes want brainless: there is nothing
to decide, it simply happens. The other two actives are conditional on being
attacked — a counter needs a dodge, a siphon needs a blow to answer — and the
fighter should not have to wait for permission.

**What the caution was for, and what it turned out to be.** The fear was the
arcs: three builds sit within seven points of each other at equal spend, and a
class taking occasional double rounds could blow that open. It did not, and the
reason is worth writing down rather than being relieved about — ARCS compares
builds *within* a class, so a class-gated mechanic lifts all three Fighter
builds equally and none of the others. The widest gap in any cell moved 13.0 to
13.7 points, which is noise. **The thing to have watched was the class
comparison, not the build comparison**, and that is COMBAT and CROWDS.

There the effect is real, small, and confined to the top: the Fighter's stretch
column moves +1.5 points at level eleven and +4.0 at thirteen, and its
three-creature crowd numbers move nought below level nine and +3 to +6 above it.
Below level eight it does nothing at all, because the speed floor has not been
reached.

That is the timid end on purpose. A weapon band is +5 strike and a second swing
is an entire extra attack; the report can re-price a charm and cannot re-price
"sometimes twice", so the number that shipped is the one that could be raised
later rather than the one that has to be walked back. Whether one round in ten
*feels* like the trope is a judgement the report cannot make — it is the
question to take to a playthrough.

### Where the scheme stands

All four built, and the shape it was designed for is what the tables now show:

| class | unit | active |
|---|---|---|
| Fighter | block — flat off every hit | a second swing, sometimes |
| Thief | dodge — a chance of nothing | a counter off the dodge |
| Mage | absorb — a pool | the pool refills from damage dealt |

Two of the three answer "do not die" *while* attacking, and the three units are
different arithmetic rather than three sizes of the same number. The loop is
still kill things and do not die, which was never the thing to fix.

### The order, and why it is an order

Every term here changes `PlayerAttack` or `MonsterDamage`, which is what
DANGER, ENDURANCE, ARCS, LANES and WARD are all built on. Five new terms at
once means the report cannot attribute anything to anything. This document has
five separate findings this month of an instrument quietly measuring something
other than what its prose claimed; the answer is one term at a time, each one
taught to `SimulateFight` first and read in the report before the next starts.

1. **Dodge for the thief.** Smallest self-contained change, gives the class the
   unit it is the only one without, and it is the term the rest hang off.
2. **Counterattack on a dodge.** Rides on 1 and needs no new state.
3. **Vampiric shielding for the mage.** Feeds an existing unit rather than
   adding one.
4. **Bonus actions for the fighter.** Last, alone, and with the arcs table
   watched closely.

An honest note on where this lands. Even with all four, the loop is still kill
things and do not die — that was never the thing to fix. What changes is that
the three classes would be answering "do not die" with three different kinds of
arithmetic instead of the same subtraction, and answering it *while* attacking
in two cases out of three. That is as far as mechanics can carry class
identity; the rest of it is what the writing already does.

## The fifth zero value, and the threshold nobody measured

The plan above said `LaneForLevel` wanted a per-class measurement at DANGER's
sample, taken *before* deciding rather than after, and that the answer was
expected to be "the spiked lane, for both plank classes, and probably per
class". Three of those four expectations were wrong, and the one that mattered
was not about the constant at all.

**LANES was measuring on one axis, and it was the axis the spiked lane is built
to win.** Win rate on the stretch fights asks "did you kill it". The spiked
shield trades guard for strike. Handing that comparison to a lane that sells
damage decides it before a die is rolled — the mirror of tuning something on
the axis it is behind on rather than the one that binds. An off arm is
defensive gear, so the second axis is the death rate five levels over, which is
the band DANGER already establishes is where the death-versus-flee split is the
only thing still legible. Both axes are printed now, and so is the rest of the
+5 outcome — won, fled, died — because "died less" has two mechanisms and only
the ratio between those three rows says which one is operating.

**LANES was calling a gap of one point a result, and the experiment that says
otherwise was already in its own table.** The threshold was a named constant,
`laneNoise = 1.0`, with a paragraph of reasoning attached and no measurement
under it. Meanwhile every row where all three lanes dress the character
identically — a Mage at any level, since it cannot hold a plank and
`pickSidearm` falls back to the talisman; anybody at all below the level the
baseline affords an off arm — is three identical builds measured three times.
The spread across those columns is sampling wobble by construction. There are
twenty of them, they run to **4.3 points** on the win axis and 3.8 on the death
axis at three times the old sample, and the threshold was 1.0.

So the crossover had been read off noise. The rebuilt section marks those rows
`null` in the table, takes the floor off them, prints the mean and the worst,
and says out loud that it is a floor measured at the null rows' own rates
rather than at every row's.

### What the measurement said

Per class, two axes, 2000 fights a cell, floor from the null rows:

| | won +3 | lived +5 |
|---|---|---|
| Fighter | level 10 | level 10 |
| Thief | level 11 | level 13 |
| Mage | no lane to choose: every row is the talisman |

The wall is not measurably behind anything until level ten. The constant said
six. And the two plank classes cross within a level of each other on the axis
that can see them, so **the constant does not want a class parameter** — which
was the open question, and this is its answer. The Thief's defensive crossover
lands three levels later only because its gaps spend three levels sitting just
under the floor while pointing the same way.

At the top of the game — levels 12-14 averaged — the picture is not close:

| class | axis | wall | spiked | silvered |
|---|---|---|---|---|
| Fighter | won +3 | 60.8% | **74.9%** | 69.1% |
| Fighter | won +5 | 37.0% | **54.5%** | 45.0% |
| Fighter | fled +5 | 30.3% | 22.2% | 19.5% |
| Fighter | died +5 | 32.7% | **23.2%** | 35.5% |
| Thief | won +3 | 68.2% | 76.8% | 74.2% |
| Thief | won +5 | 41.8% | **52.3%** | 49.0% |
| Thief | fled +5 | 35.4% | 28.9% | 25.4% |
| Thief | died +5 | 22.8% | **18.7%** | 25.6% |

The spiked lane wins the *defensive* axis, and the flee row says how: it flees
less than the wall does and still dies nine points less often. It is not
escaping. It is killing the thing before the thing kills you. The dose-response
backs it — the strike bonus steps 2, 4, 6, 8 across the shield bands and the
advantage appears exactly as it grows.

The three lanes' whole kits are within 1.5% of each other on price from level
seven up, which the section now prints as a table rather than asserting in a
comment. At levels four to six the silvered shield is 6.6-7.4% dearer, which is
the band the old constant switched in.

### What changed, and the design position that fell

`wardFromLevel = 6` is now `strikeFromLevel = 10`, and `LaneForLevel` returns
`ArmBlock` below it and `ArmStrike` above.

That reverses a stated design position, and the position deserves its epitaph:
*the spiked lane trades guard for strike, which is an offensive choice, and a
baseline with an opinion about offence is not a baseline.* The argument is
sound. Its premise is false at the top of the game, where the spiked shield
does not trade anything — it is the best defensive item on the arm as well as
the best offensive one. A design position whose premise has been measured away
is not a position.

**The check was rewritten to ask what the constant decides**, which is not the
crossover. Comparing a derived crossover against the constant warns about
arithmetic; the question that matters is whether the lane `Equip` hands is ever
behind the shelf. It fires when the same challenger beats the handed lane by
more than the floor at two consecutive levels — because fifty-odd comparisons
against a floor read off twenty null rows will throw single levels over it
whatever is true, and three did here, nominating a different lane each time.

**And the check was run against the old constant to watch it fail.** With
`ArmWard` from six restored it reports six warnings, the loudest being "Fighter
at 8-14 holds the silvered lane; the spiked one is 10.3 better on lived". A
check nobody has seen fail is a check nobody has tested.

### What this leaves open, which is a content question

**The ward lane is now the thing to watch.** It is the best of the three in one
cell of the twenty-eight LANES measures, which is what a coin looks like, and
at the top of the game it is the *worst* thing on the arm on the axis that
kills you: fewest escapes, most deaths. The cause is legible — it pays three
points of guard for fifteen of ward, and the WARD section prices the entire
ward slot at nought to three points even at level thirteen against magical
attackers only. A band of three where one is never the answer has stopped being
a choice, which is the charm-band finding in a different slot. Not fixed here,
because retuning a table until the answer comes out differently is
decide-then-measure wearing a lab coat.

### What it moved in the rest of the report

`Equip` is the on-curve assumption, so this moves everything. Read the shift as
the assumption changing, not the content:

- **ARCS**: balanced went from winning 2 cells outright to 3 and the duelist
  from 6 to 3 — the baseline caught up because it stopped carrying a bad arm.
  The widest gap grew from 11.9 to 13.7 points.
- **COMBAT**: levels 6-9 lost one to three points of stretch win rate, all of
  it under the noise floor, because those levels moved from the silvered shield
  back to the wall. Levels 10-14 gained.
- **CROWDS**: the Mage is now the class left behind in 8 cells rather than 7,
  which is not the Mage changing — it holds a talisman — but the other two
  getting better above it.
- **DANGER** fails the same two bands by the same margins it failed before.
- Five save fixtures were regenerated; the diff is shields and nothing else.

## World size: the oldest question, and it was the wrong question

`placePOIs` grades every location by how far it sits from the capital, and the
divisor was half the diagonal of the map rectangle:

```go
maxDist := math.Hypot(float64(Width), float64(Height)) / 2   // 100
level := 1 + int(d/maxDist*11)
```

The continent is a blob inside that rectangle. Measured over forty seeds, the
farthest *walkable tile* from the capital is 49 to 72 tiles out, mean 59 — so
the land reaches 59% of the divisor and the formula spent the other 41% of its
range off the edge of the world.

What that cost, over the same forty seeds and 1,778 locations:

| level | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10-14 |
|---|---|---|---|---|---|---|---|---|---|---|
| locations a map | 4.2 | 6.7 | 10.2 | 9.8 | 8.0 | 4.0 | 1.3 | 0.25 | 0.03 | **0** |

**The highest place that has ever existed in this game is level 9, and it
turned up once in forty maps.** Levels 10 to 14 have never been generated. Every
monster above level nine, the tier-four and tier-five gear bands, the LANES
crossover this session moved to level ten, the whole of CROWDS above the middle
— all of it describes country the player cannot be sent to.

**Why "make the world bigger" would have changed nothing.** That is how this sat
on the open list under the heading "world size" for months, and it is worth
being exact about: the formula is a *fraction* of the divisor, so it is scale
invariant. Doubling `Width` and `Height` doubles both `d` and `maxDist` and
leaves every level exactly where it was. The variable was never the size of the
map. It was what the size was being measured against.

**And why nothing caught it.** The balance report reads `biomeForLevel`, a
lookup table from level to biome name, rather than the map. So it happily
measured the danger curve at levels 10 through 14 without ever asking whether
the world could produce them, and the only section that touches a real
continent — SAGA — was reporting the answer in plain sight: its final leg, the
end of the game's longest story, was landing in level-7 country.

### The fix

`gradeByDistance` now runs after placement and divides by the map's own reach —
the distance to the farthest location actually placed — in thirteen steps, so
the outermost location on every map is level 14.

Twelve steps was the first attempt, on the reasoning that the top two bands are
what a dungeon's `+2` is for. That reproduced the original bug two notches up:
reaching 14 then requires a dungeon to happen to be the farthest thing on the
map, and across ten seeds none was. It also does not survive contact with
`encounterLevel`, which blends two parts the player's level with one part the
region's — so a level-12 region hands a level-14 player a level-12 fight, and
the top of the game has nothing in it. The dungeon bonus still does its work
everywhere it has room, which is everywhere but the rim.

Normalising per map also means a cramped continent gets a steep ramp rather
than no endgame, which is the property that was missing: the ramp now spans
whatever the noise produced.

| level | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 | 13 | 14 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| before | 4.2 | 6.7 | 10.2 | 9.8 | 8.0 | 4.0 | 1.3 | .25 | .03 | 0 | 0 | 0 | 0 | 0 |
| after | 1.3 | 1.5 | 2.1 | 3.3 | 4.0 | 4.5 | 4.7 | 5.2 | 5.0 | 4.0 | 3.2 | 2.6 | 1.9 | 1.1 |

SAGA is the visible consequence, and it is the whole point of the change: the
spine's five legs used to run 2/3/5/6/7 in region level and now run 4/7/10/12/14.
A story that ends at the top of the content is a story the content was written
for.

`TestTheWorldReachesTheTopOfItsOwnContent` is the net under it, and it asserts
the property rather than the arithmetic: a spread of seeds reaches the top band,
each map spans at least eight levels of its own, and no band between one and
thirteen is empty across ten maps. A different formula could satisfy all three,
which is the point of writing it that way.

### What this does not fix

`encounterLevel` is `(player*2 + region) / 3`, so the world can still only ever
hand you about a third of the gap between your level and the ground you are
standing on. That is a deliberate blend and it now works as designed — a level-3
character who walks to the rim meets level-8 country after the night shift and
the jitter, which is the "+5 by poor choices" the DANGER brief asks for and
could not previously produce. It is only worth restating because the brief's
bands and the world's bands finally mean the same thing, and before this commit
they did not.

## Three policies that were reading a different game

Chasing "the ward lane is never the answer" did not end at the shield table. It
ended at three places where code that *estimates* what the rules do had drifted
from what the rules actually do, and every one of them was under numbers this
document has been quoting for months.

The pattern is worth the name, because it is not the same as the rule
`cmd/balance` already carries. "A mechanic the simulator cannot see is a
mechanic the balance pass is lying about" covers a missing mechanic. This is a
*present* mechanic being misread by a second copy of the arithmetic:

> **A policy that estimates what the rules do is a second copy of the rules.**

Both halves always look right on their own, which is why none of these was
found by reading.

### The retreat, the gamble and the heal were pricing magic at 1.6×

`incomingPerRound` estimates a blow at 0.85 of offence — the mean of
`MonsterDamage`'s roll. Its magical branch swapped the guard for ward and left
the roll alone, while `MonsterDamage` scales a magical roll by `magicBite`
(0.62) first. So the policy priced a caster's swing at 0.85 of offence when the
caster swings for 0.53.

Worst exactly where it matters: the top of the game, where half the blows
landing are magical, on the three decisions that decide whether a fight is
survived.

Fixing it moved the measured shape of the game. At level 13, five levels over,
a Fighter now flees 8.9% of the time rather than 14.1% and wins 65.3% rather
than 50.2% — it stops running from damage that was never landing. And the WARD
section, whose whole subject is ward, roughly tripled what it says the slot is
worth: nought to 2.7 points before, up to 5.5 after.

### A Fighter cast spells because the swing was described by a guess

`freeSwingWorth` is the bar a paid technique has to clear. Its caster branch
shares `FocusBolt`'s own arithmetic and is exact. Its melee branch said
`Str()/2 + Strike()`. `PlayerDamage` rolls between `str*0.65 + strike` and 1.25
of that: at level thirteen a Fighter's swing averages about 46 and the proxy
said 36.5.

So techniques worth barely a third of a swing measured as worth a round. And
because `SpellPower` reads `MaxPsy`, **buying psyche made a Fighter cast more
and get worse** — EXCHANGE priced a point of psyche for a Fighter at minus one
at every level it sampled. A stat measuring as a trap, entirely because the
policy misread the alternative.

`PlayerDamage`'s bounds are `SwingBand` now and `freeSwingWorth` takes its
mean. Psyche prices at 0.00 to 0.10 for a Fighter and a Thief afterwards and
stays 0.48 to 1.07 for a Mage, which is what a caster's currency should read.

A Fighter at the top of the game gained six points of win rate on the stretch
fights from this alone.

### And a pointer into the content tables

`model.Shield.Extra` is a `*Bonus`, so every copy of a shield shares the Bonus
off the row it came from. Nothing in the game had ever written through it. The
first thing that did — the new EXCHANGE section, adding six points of ward to
price the slot — buffed the entire tier a little more on each of four thousand
iterations, until both sides of the comparison were saturated and ward priced
out at a flat nought. Nothing failed. The number was plausible.

`Shield.Nudged` is the only sanctioned way to change a shield now, the field
says so, and a test holds it.

## EXCHANGE: what a point of anything is worth

Three content tables spend in points — shields trade guard for ward or strike,
charms trade one stat for another, affixes do both — and none of them had ever
been told the rate. "Fifteen points of ward for three of guard" is neither
generous nor stingy until the two are in one currency.

EXCHANGE nudges the balanced build by K points of one stat and reads the
difference off the two bands LANES uses, per point, per class. Two things make
it able to resolve a point of ward, and both are the interesting part:

**Common random numbers.** Fight *f* forks its generator from the cell and the
fight number, so the baseline and all seven nudges meet the same subject in the
same encounter with the same opening rolls. Only the nudge differs. That took
the error bar from 0.92 per point on the death column to 0.24.

**And then the noise floor had to be replaced.** Pairing makes a nudge that
changes nothing return a bit-identical stream and a floor of exactly nought,
which would license believing any number in the table. LANES' trick — read the
floor off rows where three different-looking builds are secretly the same build
— is unavailable. The honest replacement is two independent halves of every
estimate, and how far they disagree is the error bar.

The exact `0.00` rows are worth keeping for a different reason: a Mage swings a
Focus, so `Weapon.Strike` is read by nothing it does, the paired runs never
diverge by a die, and the answer is not "small" but *identical*. A stat that
does nothing says so in a distinctive voice, and one that does not would be a
plumbing bug this catches.

What it says, at the top of the game, per point of win rate on the stretch
fights: **strike 1.0, strength 0.7, ward 0.6, speed 0.25, dexterity 0.25,
defense 0.14.** Guard has collapsed as a currency by level thirteen, because
on-curve armour already cancels most of a physical roll. That is the whole
explanation of why the wall lane lost, and it is the number the shield table
now has to be authored against.

## What the charm bands look like once they are priced

The brief said two of five bands had a right answer rather than a choice.
Priced against EXCHANGE it was worse than that, and worse in two directions:

| tier | | | |
|---|---|---|---|
| 1 | Lucky Tooth −0.8 | Heavy Knucklebone +0.8 | |
| 2 | Spectacles 0.1 | Anklet −0.4 | |
| 3 | Licence 1.4 | Fingerbone 3.0 | Quiet Stone −0.8 |
| 4 | Ring 1.7 | Earplugs 5.2 | Medal 1.1 |
| 5 | Collar 8.2 | Pendant −0.1 | |

The ward charms dominate because ward carries 6 to 14 points where its rivals
carry 1 to 4 — the brief's diagnosis, now with a rate under it. And the bands
*without* a ward charm fail the other way: tier two is a choice between two
items worth nothing, because dexterity and speed price at 0.17 and 0.29 a point
against strength and strike at 0.53 and 0.72.

Every charm is re-authored to land inside its band and to differ in which axis
it buys. Prices are flat within a band, because once the values match a price
spread is a signal with nothing behind it. On the fights: no band fails, the
widest is 3.0 against a threshold of 3.0, and the rest run 0.4 to 2.0.

**And `CharmValue`'s weights are measurements now.** They were read off five
argmax comparisons, and the comment under them was honest that seven continuous
weights cannot be recovered from five discrete choices. They are EXCHANGE's
all-class, both-axis means now. The old numbers had ward at 1.0 against a
measured 0.22 and dexterity at 1.0 against 0.17, which is exactly why the
balanced build reached for the ward charm in every band that had one.

Still class-blind, and that is now a known cost with a number on it rather than
an oversight: a point of psyche is worth 0.68 to a Mage at thirteen against
−0.01 to a Fighter.

## What the sawtooth is, and why "level eleven is soft" was a misreading

The open list said level eleven was soft and blamed mountain's roster. It is
not, and it was never a roster hole. Averaged across classes, COMBAT's "over"
column runs:

| levels | 4-6 | 7-9 | 10-12 | 13-14 |
|---|---|---|---|---|
| win rate | 90.8 / 91.0 / 90.9 | 97.3 / 96.8 / 84.2 | 86.1 / 81.0 / 72.3 | 80.2 / 77.6 |

About thirteen points of decay *inside* each band, restored by the next gear
step at 4, 7, 10 and 13. Twelve is the hard level and thirteen is the easy one,
because twelve is the last level before a band.

That is banded gear meeting a smooth monster curve, and it is the rhythm the
game is built on — you get stronger when you shop and weaker as you outlevel
your kit, which is what gives a shop counter a job. It is named here rather
than fixed, because the alternative is unbanding the gear.

Five creatures were added to the top of the roster all the same, for a
different reason: the world fix means levels 10 to 14 are reachable for the
first time, and what is up there was two to four creatures a level. DANGER's
+5 band moved from UNDER by 12.2 to UNDER by 7.2.

## Multiplayer: not decided, but inventoried

*(Jeremy's call: leave the question open, and stop it taxing every save-format
change. So this is the inventory a later decision would otherwise have to start
by making — what in the format is already owner-agnostic, what is not, and what
each of the second group would actually cost.)*

The heritage is real: the original was a door game, and `new-slycrel` was built
session-per-user against a shared store. But nothing has been built for it here
in three format versions, and the deadline this document set — "decide before
the save format hardens" — has passed rather than been met. What follows is
why that is less expensive than it sounds.

### Already owner-agnostic, and free

**The world.** `save.File` stores a `Seed` and nothing else about the terrain,
and `world.Generate` is pure — same seed, same continent, on any machine. Two
players on one seed are standing in the same world without a byte being shared.
This is the expensive half of multiplayer in most games and it is already done,
for reasons that had nothing to do with multiplayer.

**Everything derived.** The weather is derived from seed, clock and biome and
deliberately not stored. Location interiors regenerate from `POI.Seed`. The
monster tables, the gear tables and the writing are content, identical for
everybody. None of it needs an owner because none of it is per-run state.

**The per-character record.** `model.Character` carries no identity beyond a
name — no player id, no ownership. A party of four characters and a party of
one hero plus three hirelings are the same shape to every rule in
`internal/rules`, which is the observation the "pets and familiars are cheap"
note already made from the other direction.

### Not owner-agnostic, and what each one costs

**`POIs []POIState` is the hard one.** It is parallel to the generated location
list and records `Discovered`, `Visited`, `Cleared` and `Used`. Every one of
those is a fact about *a* player, stored as a fact about *the* world. Two
players sharing a seed would need them per-player, or would have to agree that
clearing a dungeon clears it for everybody — which is a design decision, not a
format one, and it is the decision that actually has to be made first. `Used`
in particular exists so an emptied chest does not refill: shared, the second
player finds every chest already looted; per-player, they both loot the same
chest, which is a different game.

**`Fog`** is the same shape and the easy version of it: plainly per-player,
plainly cheap, a bitset per player rather than one.

**`Sagas`, `Quests`, `Threads`** are all per-player already in substance —
they are one run's progress through authored content — and would simply move
inside whatever per-player record gets introduced. `Threads` is keyed to its
owner *by name*, which is the one place a second player with an
identically-named companion would collide.

**The autosave slot** is a single file per profile and would need an owner.
Trivial, but it is the kind of thing that is trivial only while somebody
remembers it.

**`Skim` and `Wants`.** A companion's cut goes into their own purse and they
spend it at the right counter. If party slots become other people, that whole
mechanism has nothing to do and the economy loses a sink it was tuned with.
This is the second design decision, and it is not small: *do other players
occupy the hireling slots, or walk beside them?*

### What that adds up to

The format costs are a per-player struct holding `Fog`, `POIState`, `Sagas`,
`Quests`, `Threads`, `At`, `Facing`, `Inside` and `Clock`, with `Seed` and the
version staying at the top level. That is a mechanical refactor, and the two
committed compatibility fixtures (`v1-solo.json`, `v2-company.json`) would keep
loading by reading a single-player file as a one-player list — which is the
same shape of change `Track` already made when it learned to carry an explicit
`On bool` rather than deriving a flag from a zero value.

The *design* costs are the two questions above, and neither is answerable by
looking at the code: whether the world's cleared state is shared, and whether
other players stand where the hirelings stand. Both change what the game is.
Neither gets cheaper by waiting, and neither gets more expensive either — which
is the actual answer to why the missed deadline did not cost anything.

**The one thing worth doing now, and it is done:** nothing in this list is
load-bearing on a decision, so no future save-format change needs to consult
it. Add fields as the game needs them, keep the zero-value rule (a field added
today is absent in every file written before today and must unmarshal to the
safe answer), and if multiplayer ever happens, the work is the per-player
struct plus two design decisions rather than an archaeology exercise.

## The ward lane loses its own matchup, and the reason is structural

The open question left on items 2 and 4 was the one the design states: a player
is allowed to skip ward, and skipping it is supposed to get progressively worse.
That is a claim about a *matchup*, and LANES was averaging over a roster that is
a bit over half magical at the rim — which is exactly the operation that hides
one. The section had been reporting "the ward lane is never the answer" without
ever asking the question in the form the design makes it.

LANES splits the death rate at +5 by what is swinging now. Levels 12-14:

| class | against | wall | spiked | silvered |
|---|---|---|---|---|
| Fighter | steel | 27.0% | **16.7%** | 27.0% |
| Fighter | magic | 40.9% | **32.7%** | 42.5% |
| Thief | steel | 23.6% | **19.2%** | 23.0% |
| Thief | magic | 29.3% | **28.0%** | 40.0% |

**The silvered shield is the worst of the three against casters.** Not diluted,
not averaged away — worst, in the one fight it exists for, by 9.8 points for a
Fighter and 12.0 for a Thief. No reweighting of the roster rescues that.

### Why, and why it is not a number to nudge

The trade at tier four is eleven points of ward against six of strike. Six of
strike wins, and it wins *on the death rate*, because killing the caster is what
stops the caster casting. Ward is flat subtraction against a bounded roll: at
the rim a magical hit rolls `Between(6, 25)` before ward, so the first points of
ward buy a lot and the eleventh buys almost nothing, and the whole eleven
cannot buy as much as ending the fight two rounds sooner.

That is a structural property of "flat reduction against a bounded roll",
not a mispriced constant, and it is the same arithmetic CROWDS already explains
from the other direction — flat reduction comes off every blow, so it is worth
more the more blows arrive, and worth less the fewer rounds there are.

SHAPES had found the corroborating half of this independently and it is worth
reading the two together: the escort shape's matchup axis only ever bites in the
oddity zone, "because it needs one creature that beats another by three on
armour while being beaten by three on ward, and the fantasy rosters were never
written to contrast."

### What the options actually are

Deliberately not chosen here, because all three are design decisions rather
than tuning:

1. **Change what ward does.** Flat subtraction is the problem; a percentage, or
   a chance to take nothing, would scale with the size of what it is resisting
   instead of vanishing against it. This is a rules change and it moves every
   ward number in the game.
2. **Give the ward lane a second thing.** The lane is currently guard-for-ward
   and nothing else. If ward cannot carry a slot alone, the item can carry
   something else beside it — which is what the *charm* bands were re-authored
   to do, and it worked there.
3. **Retire the lane.** Two lanes rather than three, and let the ward slot be
   the charm's job, where a smaller amount of ward next to another stat does
   trade. `Shield.Lane()` reads the lane off what the item does, so this costs
   a content edit and no code.

What is *not* an option is raising the ward number, and the measurement above is
what says so: eleven is already past the point where more helps.

## What is open, and in what order

Rewritten after the session that answered items 2, 4 and half of 6, and found
that answering them meant fixing the instruments first. The ordering is a
recommendation and the reasons are the useful part.

Four of the eight things on the original list are closed, one is withdrawn as a
misreading, and the two biggest are untouched. The pattern in what closed is
worth naming before the list: **every one of them turned out to be an
instrument problem wearing content's clothes.** The ward lane looked like a
badly priced item and was a one-sided derivative. Level eleven looked like a
roster hole and was a gear-band sawtooth. The world's missing top half looked
like "make the map bigger" and was a divisor. A stat that made a Fighter worse
looked like a trade and was a policy comparing against a bad guess. When
something in this game reads as a content fault, price the instrument first.

### 1. None of it has been played

Four combat mechanics went in against a simulator in one sitting: a dodge, a
counter, a refilling barrier and a second swing. Every one of them is measured
and not one has been felt. The report cannot answer whether a Thief that dodges
and counters *plays* like a Thief, or whether one round in ten is often enough
for a second swing to read as a trope rather than a glitch.

**Still the top of the list, and now with more riding on it.** The fixtures it
was waiting for exist — `rogue.json` is a level-11 Thief, `caster.json` a Mage
with a talisman. And a great deal has moved underneath since: the second swing
was re-entering the technique chooser and casting, so what a Fighter does on
that round has *never* been what the report described; the retreat policy has
been rewritten twice; the world reaches its own top band for the first time.
Everything below this line is arithmetic that has never been felt by anybody.

### 2. The fifth zero value — done, and the ward lane is what it left behind

`LaneForLevel` is re-pinned and has been re-pinned twice more since, each time
by a rules fix rather than a content change, which is the apparatus working.
The sections above have the numbers.

What replaces it is **the ward lane**, and the measurement it was waiting for
has been taken: LANES splits the death rate at +5 by what is swinging, and the
silvered shield is the *worst* of the three against casters — by 9.8 points for
a Fighter and 12.0 for a Thief. It loses the one fight it exists for.

So this is no longer a tuning job. Eleven points of ward is already past where
more helps, and the section above lays out the three options, all of which are
design decisions: change what ward does, give the lane a second stat to carry,
or retire it to two lanes and let the charm slot sell ward, where a smaller
amount beside another stat does trade. **Jeremy's call, not a number to nudge.**

### 3. Two designs finished and unbuilt — still the biggest thing on the list

Untouched, and deliberately: both are multi-day features touching the battle
screen, and half-building either is worse than not starting. A simulator that
knows about staggering and a screen that does not is the "mechanic the
simulator cannot see" rule pointed the other way.

**Staggering identical creatures** into one slot with a count, where only the
front one can be hit. It solves two problems at once — the crowded field's
layout, where six creatures at three columns cannot fit their own names, and
the wallpaper of three identical portraits carrying three copies of one joke —
and it changes the fight, turning a group into a queue. `SimulateFight` has to
learn the rule before CROWDS can price it, and the screen has to learn it in
the same pass.

**An off-hand weapon for the Thief.** The class already has an offensive off
arm and the lane rule is not giving it to them. Its own value is real but
smaller than it looks: a sidearm band steps +2 where a weapon band steps +5,
and a weapon in that arm is the one legitimate way past
`TestShieldsStaySecondaryToArmour`, because it is not a shield. Note that
EXCHANGE now prices what it would buy — strike at about 1.0 a point at the top
of the game — so the design can be costed before it is built rather than after.

### 4. The charm bands are done; the shield cap turns out to be the wrong lever

**The charm bands trade now** — see the section above. Every band comes in
inside the threshold on the fights, and `CharmValue`'s weights are EXCHANGE's
measurements rather than a fit to five argmax comparisons.

**The shield cap is not what is holding the off-arm band together, and this is
worth correcting because the document has named it three times.** Only the wall
sits at the cap; the other two lanes are one to three points under it in every
band. So the cap constrains exactly one lane — and it is the lane whose problem
is not headroom. EXCHANGE prices guard at 0.14 a point at level thirteen
against strike at 1.0, so doubling the wall's guard would buy it about one point
of win rate against the spiked lane's six. **Raising the cap cannot make the
wall competitive at the top of the game**, and the reason is that guard as a
currency has collapsed there, not that the wall is short of it.

That measurement has been taken and it came back negative — the silvered shield
loses to the spiked one against casters too. See the section above; what is left
of this item now lives under item 2, as a design decision about what ward is.

### 5. Things the report can see and nobody has tuned

**CROWDS is measured and untouched** — and reading `EncounterSize` and the pack
shape together, which nothing did, turned the item into a different one.

The distribution is now printed under the table. A solo hero meets one or two
creatures in three quarters of encounters and three or more in the rest; hiring
moves the mean from 1.90 to 2.62. So the right-hand columns are a tail rather
than a curiosity, and the weighting the item asked for exists.

But the thing that stops this being tunable is not a missing weighting, it is a
**missing instrument: there is no party simulator.** `rules.SimulateGroup` takes
one `Character`. Every row in CROWDS is therefore one character against N
creatures, and the moment the company is what changes the rolls, the report can
price what the world sends but not what the party does about it — which is why
the four- and six-creature columns have always had to be read as "a solo hero
after their company was killed".

That is the job. `ChooseAllyMove` already exists and the battle screen already
runs a party turn, so the pieces are there; what is missing is the simulator
that plays them without a screen. Until it exists, tuning these numbers is
tuning a game nobody plays — a solo hero does not walk into the four-creature
fights, and a party does, and the party is the thing that cannot be measured.

**Level eleven is soft** — withdrawn, and it was a misreading. Eleven is not
soft, the roster was not the cause, and the shape is a sawtooth keyed to the
gear bands: about thirteen points of decay inside each band, restored at 4, 7,
10 and 13. Twelve is the hard level. See the section above; it is named rather
than fixed, because the alternative is unbanding the gear. Five creatures went
into the top of the roster anyway, for the different reason that the world fix
made levels 10-14 reachable at all.

**ARCS tests gear builds, not playstyles** — one of the three is now measured,
and the other two are each blocked on one missing thing, named rather than
faked. *(Jeremy's: "at some point we might need full testing against each class
and each flavour of playstyle.")*

`rules.Policy` is the hook, with the zero value meaning the competent default
every other section measures — which is the only shape it may take, since a
policy field whose zero value meant something new would silently re-measure the
entire report the day it was added.

**The never-run player** is the PLAYSTYLES section, and it starts there because
the retreat is the most load-bearing judgement in the simulator. Standing your
ground buys up to 8.1 points of win rate and costs up to **40.1 points of death
rate**, and the widest cost falls on the Thief, the class whose survival plan
*is* leaving: at level nine, five over, it dies in 69.6% of fights instead of
29.5%.

That is also the bound this report badly needed on its own arithmetic. Every
other section shares one retreat policy, so a bug in it moves every section
together and cancels out of every comparison — and two such bugs turned up in a
single evening. This is the only column that can see the judgement rather than
through it.

**The item-user** needs the simulator to carry a pack, which it deliberately
does not: "no potions" in every heading is a decision, and SUPPLIES prices the
counter instead. Building it means a buying policy as well as a using one,
which is a second set of judgement calls to get wrong.

**The player who fights behind two companions** needs the party simulator that
item 5's CROWDS entry also wants. Same missing instrument, twice — which is the
argument for building it.

### 6. The two oldest questions — world size is answered

**World size** is done, and the answer was that it was the wrong question: the
map's difficulty ramp was normalised against the bounding rectangle rather than
against the land, so 41% of the range fell off the edge of the continent and no
location above level 9 had ever been generated. Making the map bigger would
have changed nothing, the formula being scale-invariant. See the section above.

**Multiplayer** is deliberately still open, and it no longer costs anything to
leave that way. The section above inventories what in the save format is
already owner-agnostic (the world, being a seed; everything derived; the
per-character record) and what is not (`POIs`, `Fog`, and the autosave slot),
and finds that the format work is a mechanical refactor into a per-player
struct. What is left is two *design* questions that no amount of reading the
code will answer — whether a cleared dungeon is cleared for everybody, and
whether other players stand where the hirelings stand — and neither gets
cheaper or dearer by waiting. Which is the actual answer to the missed
deadline: it did not cost anything.

### Not yet needed, and worth knowing they are cheap

*(Jeremy's list.)* Pets and summoned familiars would cost less than they sound:
`internal/party` is roster and marching order and is already generic over
characters. Daze and sleep exist as `model.EffectStun` and want content rather
than mechanism. Neither is a reason to reach for them, but neither is a
multi-week job if the classes need more separation after being played.

## Open questions

- **How big should the world be?** 160×120 crosses in a couple of minutes on
  foot. Larger needs fast travel; smaller needs denser points of interest.
  Untouched since it was written, and two things now lean on the answer that
  did not exist then: a saga deals its legs out at deliberately increasing
  distance, and the compass makes crossing the map a matter of holding a
  direction. Both of those make a bigger continent *more* walkable, which is an
  argument for growing it — and both also mean the cost of getting the answer
  wrong has gone up.
- ~~**Death.**~~ *Settled, and then settled again.* A hero who falls with
  anybody hired is carried to the nearest town for a third of the purse and a
  point of Shame — and any hireling counts, standing or not, because a
  companion out of hit points is out of the fight rather than dead and gets up
  the moment it stops. Offering a reload for a death somebody was present for
  was the game contradicting its own fiction.

  A hero who falls alone gets the reload instead, back to their last rest. So
  the two deaths cost different currencies: coins and reputation with a company,
  replayed minutes without one.

  The fee is a third of the purse **capped at 250**. A share is the right shape
  at the bottom and the wrong one at the top — the same rule that stings at
  level two confiscates at level twelve — and past the cap the cost of dying
  holds still while the cost of *not* having hired anybody keeps rising, which
  is the direction the pressure should point.

  And it is a choice rather than a deduction: **pay them, or they leave.** The
  fee used to come out of the purse on the way past, which made the whole
  business something that happened to the player. The person owed it is
  standing right there with an obvious second option. Losing them is usually
  the more expensive answer — a companion is the reason there is a rescue at
  all, and going solo means the next death costs replayed hours instead of
  coins — which is exactly why it is worth offering. It also costs a point of
  honour if they were mid-story, because that is what walking away from
  somebody's story costs everywhere else.
- **Multiplayer.** `new-slycrel` was built session-per-user with a shared store,
  because the original was a door game. This was tagged "worth deciding before
  save format hardens", and **that deadline has passed rather than been met**:
  the format is seed-plus-deltas, it has been through three versions, and
  `v1-solo.json` and `v2-company.json` are committed specifically so the loader
  can never stop reading the earlier two. Whatever is decided now is decided
  against a format that already has compatibility obligations. The honest
  options are to design for it deliberately, accepting that the save file grows
  an owner, or to close the question and say in this document that the game is
  single-player — which is what it has been in practice for its whole life.

## Asset budget

The bundle is 78 packs / 56,409 files / 16.7 GB, inventoried in
[ASSET-INVENTORY.md](ASSET-INVENTORY.md). 43 packs are extracted and 919 keys
are indexed. Roughly two thirds of the bundle is sci-fi, cyberpunk, futuristic
or children's-voice content that this game has no use for — except as an
over-the-top joke zone, which is exactly what an "oddity" location is for, and
that is cashed now rather than promised: two cyberpunk packs are extracted for
the furniture alone.

What the game *loads*, as opposed to what the bundle holds, is now its own
document: [ASSET-MAP.md](ASSET-MAP.md), whose table is generated from the
manifest by `assetpipe map` and whose prose is not. It carries the reasoning for
every pipeline step that makes art, the packs extracted but not yet wired, the
icon gaps measured against the content tables, and — the part worth keeping — a
list of what the last pass left unsettled.

The 40 unopened packs were swept once. Most of it is audio, sci-fi, or GUI kits
the earlier survey already rejected; five small pixel-art packs were extracted
because they are on style and on grid, and four of them are still unwired.
