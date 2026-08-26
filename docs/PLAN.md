# Slycrel — implementation plan

Written 15 Aug 2026, at the end of the session that built the foundation.

## The four decisions everything else hangs off

| | | |
|---|---|---|
| **Engine** | Go + Ebitengine v2 | Lets the combat/levelling maths port straight out of `../new-slycrel`. Everything is code and JSON, so there is no GUI editor in the loop and no binary project file to merge. Ships as one binary. |
| **World** | Ultima-style overworld + zoom-in | A coarse continent you walk across; entering a point of interest loads a detailed local scene. Cheapest way to make a genuinely large world, and the strongest fit for a throwback. |
| **Combat** | Turn-based battle screen | Showcases the bundle's 250 creature portraits and 1,000+ spell icons, and inherits the original's stat maths almost unchanged. |
| **Canon** | Fresh world, old bones | New setting, names and lore built for an 18+ tone; the proven combat, initiative, levelling and disposition maths carried over from 1994. |

## Tone

Bawdy, absurd, and delivered completely straight. The rule the writing follows:
**the game never comments on its own joke.** A rooster is "ungovernable", a
goblin is a "middle manager", a demon is "mid-level" — and the game reports
this in the same flat voice it uses for damage numbers. Over-the-top comes from
the world behaving this way sincerely, not from nudging the player.

All flavour text is data (`data/text/flavor.json` plus per-monster lines), so
the writing can be revised without touching Go.

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
  level separately, re-arm out of their cut, and follow the hero in a line that
  tracks his path rather than his position
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
- 66 monsters across nine biomes, 14 weapons, 10 armours, 6 shields, 12 charms,
  10 affixes, 42 items, 27 techniques
  (eleven of them party-facing, lingering, or gated on a hireling's ancestry),
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

1. **Party, fourth pass.** What is left after outfitting: letting a companion
   spend their own cut between towns rather than only re-arming on level-up,
   and giving them an opinion about the gear you hand them.

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

5. **Fleeing should pay something, and the thief should be the one it pays.**
   *(Jeremy's, in response to the flee work: "fake flee + backstab might be a
   way around 0 rewards at the cost of survivability; having to run away might
   not be enjoyable.")*

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
12. Balance simulation: run 10,000 headless fights per level band against the
   pure `rules` package and tune the curve.

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

## Open questions

- **How big should the world be?** 160×120 crosses in a couple of minutes on
  foot. Larger needs fast travel; smaller needs denser points of interest.
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
  because the original was a door game. Worth deciding before save format
  hardens.

## Asset budget

The bundle is 78 packs / 56,409 files / 16.7 GB, inventoried in
[ASSET-INVENTORY.md](ASSET-INVENTORY.md). 33 packs (2.8 GB) are extracted and
384 keys are indexed. Roughly two thirds of the bundle is sci-fi, cyberpunk,
futuristic or children's-voice content that this game has no use for — except
as an over-the-top joke zone, which is exactly what an "oddity" location is
for.
