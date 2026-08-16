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
- 54 monsters across nine biomes, 14 weapons, 10 armours, 37 items, 16 techniques
- Asset pipeline: inventory, selective extraction, manifest generation, and an
  audit that reports which art keys still fall back to placeholders

Deliberately not built yet: save/load, audio, and curated tile art.

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

## Roadmap

### Phase 1 — make it feel like a game *(next session)*

1. **Save/load.** JSON in `saves/`. The world is a seed plus the mutable bits:
   player, POI discovered/visited/cleared flags, explored fog. Small file.
2. **Audio.** The bundle has ~11,000 sounds already extracted: 719 combat, 883
   UI, 132 monster, 100 ambience, and a 291-file "old magician" voice pack that
   is begging to be a narrator. Ebitengine wants Vorbis or WAV; a batch
   convert into `assets/audio/` plus a small mixer package.
3. **Real overworld tiles.** Mana Seed ships 22 Tiled `.tmx` maps and 127 `.tsx`
   tilesets on a 16px grid. Write a `.tsx` reader and an autotile/Wang lookup so
   coast, forest edges and cliffs blend instead of tiling flat.
4. **Icons everywhere.** 720 spell icons, 480 stat icons, 273 loot icons are
   extracted. Wire `Item.Icon`, `Weapon.Icon`, `Spell.Icon` through to the menus.

### Phase 2 — depth

5. **Quests.** Generated from POI pairs: someone in a town wants a thing from a
   dungeon. Small state machine, big payoff for a generated world.
6. **Party members.** `model.Character` and the battle scene are both written
   for one hero but structured for a slice. Recruitable NPCs from taverns.
7. **Equipment slots and affixes.** Currently one weapon and one armour.
   Add shield/trinket slots and a suffix generator ("of the Damp", "of Poor
   Decisions") that rolls stat modifiers.
8. **Status effects.** Poison, burn, stun already have hooks in the spell kinds;
   generalise into a timed-effect list on combatants.

### Phase 3 — the world reacting

9. **Day/night and weather.** The tileset collection includes a weather-effects
   pack. Changes encounter tables and which NPCs are out.
10. **Faction and reputation.** `Fame`, `Honor`, `Faith` and `Shame` exist on
    the character and currently do almost nothing. Gate content on them.
11. **A reason to be here.** A generated main thread that strings together
    five or six POIs into something with an ending.

### Phase 4 — polish

12. Curated UI art from the 4,488-file GUI Pro kit, replacing the procedural panels.
13. Title screen art, transitions, particles from the 115-file VFX pack.
14. Balance simulation: run 10,000 headless fights per level band against the
    pure `rules` package and tune the curve.

## Open questions

- **How big should the world be?** 160×120 crosses in a couple of minutes on
  foot. Larger needs fast travel; smaller needs denser points of interest.
- **Death.** Currently returns to the title. Permadeath fits the BBS heritage;
  a resurrection cost fits the tone better and the original had a healer.
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
