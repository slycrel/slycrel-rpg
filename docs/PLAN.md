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

- Title → character creation (three classes, rolled stat previews, name generator)
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
  somebody still standing is carried to the nearest town for a large share of
  the purse and a point of Shame; a hero who falls alone still ends the run
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
- 66 monsters across nine biomes, 14 weapons, 10 armours, 6 shields, 12 charms,
  10 affixes, 42 items, 27 techniques
  (eleven of them party-facing, lingering, or gated on a hireling's ancestry),
  9 companion backstories
- Asset pipeline: inventory, selective extraction, manifest generation, and an
  audit that reports which art keys still fall back to placeholders

Deliberately not built yet: backstories for the townspeople, day/night and
weather, anything that reads Honor or Faith, and the thief's discount on
healing items.

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

3. **NPC backstories.** The same mechanism for the townspeople who currently
   have one line each. `internal/thread` already has the casting, the beat
   chain and the choice at the end; what it does not have is a way for a thread
   to belong to somebody who stays put. A companion's thread is keyed to a name
   in the party and advanced by things the party does — a townsperson's would
   need to be keyed to a location and advanced by returning to it, which is
   closer to how a quest already works than to how a thread does.

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

   **Still open, and Jeremy's:** the thief should get more out of buying
   healing items — two for the price of one, because they steal one and pay for
   the other. That is not a flourish, it is the right shape for the actual
   constraint. The thief has *no* healing technique at all (the shared-looking
   ones in `spells.json` are blood-gated hireling moves, not class-agnostic),
   and its only sustain is two drains. Its recovery is therefore entirely
   items, which makes a discount on them the class's real defensive stat.

6. **Day/night and weather.** The tileset collection includes a weather-effects
   pack. Changes encounter tables and which NPCs are out.
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

   The character sheet says both numbers and the word for the corner: "Fame /
   Renown 11 / 2, they call you a rumour. The stories travel. You do not."

   Still open, and the reason this was worth building rather than gating on:
   `Honor` and `Faith` are still inert. Faith at least has a source — the
   shrine — so it is the next one with anywhere to go.


8. **A reason to be here.** A generated main thread that strings together
   five or six POIs into something with an ending.

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

   Add to this as things turn up; do not fix them in ones. What is left is the
   art pass below, which is a different kind of job.

10. Curated UI art from the 4,488-file GUI Pro kit, replacing the procedural panels.
11. Title screen art, transitions, particles from the 115-file VFX pack.
12. Balance simulation: run 10,000 headless fights per level band against the
   pure `rules` package and tune the curve.

## Open questions

- **How big should the world be?** 160×120 crosses in a couple of minutes on
  foot. Larger needs fast travel; smaller needs denser points of interest.
- ~~**Death.**~~ *Settled.* The company carries a fallen hero to the nearest
  town for a large share of the purse and a point of Shame. Dying with nobody
  left standing still ends the run, so permadeath is intact for anyone playing
  alone, and the hirelings are the thing that buys it off — which makes the fee
  a reason to hire rather than a softening of the stakes.
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
