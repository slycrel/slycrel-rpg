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
- Getting back up: a fallen companion is out of the fight rather than dead and
  stands up afterwards, or sooner via an item or Reknit. A hero who falls with
  somebody still standing is carried to the nearest town for a large share of
  the purse and a point of Shame; a hero who falls alone still ends the run
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
  (eleven of them party-facing, lingering, or gated on a hireling's ancestry)
- Asset pipeline: inventory, selective extraction, manifest generation, and an
  audit that reports which art keys still fall back to placeholders

Deliberately not built yet: equipment slots, status effects.

## Architecture, and why

**Scene stack, not a state enum.** A shop, a battle, a message box, and the map
each push themselves over whatever was underneath and pop back. No screen needs
to know who called it. Overlay screens draw the scene beneath them, so a battle
gets a dimmed backdrop of the place it started.

**Everything generated takes an `*RNG`, never a global.** `core.RNG.Fork(label,
salt)` derives child streams deterministically, which is what lets a point of
interest regenerate its own interior from `poi.Seed` on every visit instead of
being stored. A seed reproduces the continent, the towns, and the jokes.

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

1. **Party, third pass.** What the targeting rework did not reach: outfitting a
   companion at a shop rather than letting them re-arm off-screen out of their
   cut, and letting a companion carry a pack of their own.

   A resurrecting NPC was considered here and deliberately left out. With the
   fallen standing up when a fight ends and a fallen hero carried to town by
   the company, there is no state a run can reach where somebody stays dead and
   needs paying for — so a healer at a counter would be a door that never
   opens. The apothecary stocks the revive items instead, which is the same
   idea placed where it does something. If death ever gets harsher, this is the
   first thing to revisit.
2. **Companion backstories.** A hireling arrives with a name, a lineage and a
   pitch, and nothing behind it. Give each one a two- or three-step thread that
   surfaces as you travel with them — the previous employer a part-undead is
   still technically contracted to, the arrangement a part-demon will not
   discuss — resolved at a place the world already generated.
3. **NPC backstories.** The same shape for the townspeople who currently have
   one line each.

   These two want one mechanism, and it is a step up from what `internal/quest`
   does now. The existing generator picks a verb, an object and a place, checks
   they exist, and hands back a counter: stateless, interchangeable, and
   deliberately basic. A backstory is a small *ordered* thread that remembers
   where it got to, with a fixed cast. The likely shape is an authored skeleton
   — a beat list with roles (a giver, a place, a thing, an antagonist) — cast
   from whatever the seed generated at first contact and then frozen into the
   save. That keeps the writing hand-made and the casting generated, and it
   means a thread can never name a place this world does not contain.

### Phase 3 — the world reacting

4. **Day/night and weather.** The tileset collection includes a weather-effects
   pack. Changes encounter tables and which NPCs are out.
5. **Faction and reputation.** `Fame`, `Honor`, `Faith` and `Shame` exist on
   the character and currently do almost nothing. Gate content on them.
6. **A reason to be here.** A generated main thread that strings together
   five or six POIs into something with an ending.

### Phase 4 — polish

7. Curated UI art from the 4,488-file GUI Pro kit, replacing the procedural panels.
8. Title screen art, transitions, particles from the 115-file VFX pack.
9. Balance simulation: run 10,000 headless fights per level band against the
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
