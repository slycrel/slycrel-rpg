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
  mostly readable JSON and doubles as a test fixture
- 62 monsters across nine biomes, 14 weapons, 10 armours, 6 shields, 9 charms,
  10 affixes, 42 items, 27 techniques
  (eleven of them party-facing, lingering, or gated on a hireling's ancestry),
  9 companion backstories
- Asset pipeline: inventory, selective extraction, manifest generation, and an
  audit that reports which art keys still fall back to placeholders

Deliberately not built yet: backstories for the townspeople, alternative
progression arcs, day/night and weather, anything that reads Fame or Shame.

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
   numbers. That is the cheap part and worth doing on its own, because it turns
   "is this balanced" into "is each of these playable", which is the actual
   question. The expensive part is making the content support all three — the
   charm and affix tables would need enough spread that a glass cannon has
   something to buy — and that is where the rabbit hole is. Do the report first
   and let it say whether the content already supports more than one arc.

5. **Day/night and weather.** The tileset collection includes a weather-effects
   pack. Changes encounter tables and which NPCs are out.
6. **Faction and reputation.** `Fame`, `Honor`, `Faith` and `Shame` exist on
   the character and currently do almost nothing. Gate content on them.
7. **A reason to be here.** A generated main thread that strings together
   five or six POIs into something with an ending.

### Phase 4 — polish

8. **A UX pass, held until the features stop moving.** *(Jeremy's call, and the
   right one: interface work done against a moving target gets redone.)* Things
   are being noticed as they turn up and parked here rather than fixed one at a
   time, because half of them are the same missing capability wearing different
   hats and a single pass will be cheaper than eight patches.

   Noticed so far, from building the backstories:

   - `Ask` has no disabled option. An ending the player cannot afford is
     offered, selected, and only then refused with a line of text. The same gap
     is why a menu section header has to be a disabled row with dashes round it
     (`- the company -` in the journal).
   - A price is quoted with the purse off screen. The ending menu says
     "Buy out the terms (418)" and nothing on that screen says what you have.
   - A companion's sheet does not mention their own backstory. There was no
     room left on the 208-pixel panel, so the journal is the only place it
     appears — which is the wrong place to look for "who is this person".
   - The status panel is at its height limit generally. Four gear rows under six
     stat rows already forced it from 200 to 208, and the next thing that wants
     a row has nowhere to go.

   Jeremy has his own list from playing. Add to this as things turn up; do not
   fix them in ones.

9. Curated UI art from the 4,488-file GUI Pro kit, replacing the procedural panels.
10. Title screen art, transitions, particles from the 115-file VFX pack.
11. Balance simulation: run 10,000 headless fights per level band against the
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
