# Credits

## Slycrel

- **Dave Dolinar, Jeremy Stone** — the original Pascal door game, Hermes II BBS,
  1994–96. The combat, levelling, and initiative maths in `internal/rules` are a
  direct port of their work.
- **Bill Dolinar** — the C++ port, 1997–2000.
- **Jeremy Stone** — this version.

## Art

All art is from the Humble *Complete RPG Creator Bundle* and remains the
property of its creators. See [docs/ASSET-LICENSING.md](docs/ASSET-LICENSING.md)
for terms.

### In the current build

| creator | packs | used for |
|---|---|---|
| **[REXARD](https://www.gamedevmarket.net/member/rexard)** | Character Avatar Icons, Mobs Avatar Icons, Monsters Avatar Icons | party portraits, monster and boss portraits |
| **[AfGameAssets / Pixogen](https://pixogenassets.itch.io/)** | Pixel Art Top-Down RPG Characters, Enemies, NPC | hero, foe, and NPC sprites |
| **[The Sound Guild](https://the-sound-guild.itch.io/)** | Ambience Sounds, Combat Sounds Bundle, Monster Sounds Vol. 1, Old Magician VO, User Interface SFX Bundle | all 33 sound cues |

Of 408 art keys, 316 are REXARD's and 92 are AfGameAssets'. All 33 audio cues
(81 files) are The Sound Guild's.

### Extracted and available, not yet loaded

Credited here in advance so the list does not have to be reconstructed later.

| creator | packs |
|---|---|
| **Ricardo Machado** ("Beowulf", Beowulfus Universum) | RPG Monster Loot, Magic Runes, Mini Animals, Mini Dungeons, Mini Heroes (Elves, Humans), Mini City, Mini City Interiors, Pixel RPG Dungeon Monsters |
| **[Seliel the Shaper](https://seliel-the-shaper.itch.io/)** | Mana Seed Pixel Art Tileset Collection |
| **Konstantin** (Infinity of Life) | Fantasy Nordic GUI |
| **EvilSystem** | RPG and MMO UI 4 |
| **Acasas** | Pixel Art Medieval Interiors 2 |
| various | GUI Pro Fantasy RPG, 2D Buildings Town, 2D Minimal Skill Icons, Spells and Ability Icons, Dialogue Boxes, Pixel Art Medieval Interiors, Pixel Art Farm Life, Pixel Art Green Temple, Pixel Art RPG VFX |

Attribution for the "various" row is incomplete — those packs shipped without
creator information, and the names should be recovered from the GameDev Market
listings before any release that uses them.

## Fonts

The game bundles no font. UI text renders with `basicfont.Face7x13` from
`golang.org/x/image` (BSD-3-Clause).

The bundle's asset packs do contain 37 TTFs — Vollkorn, Exo, Anton, Archivo
Black, Baloo Thambi, Berkshire Swash, Varela Round — all Google Fonts under the
SIL Open Font License 1.1. If one is ever adopted, take it from Google Fonts
with its `OFL.txt`, which the asset packs omit.

## Code

- **[Ebitengine](https://ebitengine.org/)** (Hajime Hoshi) — Apache-2.0
- **golang.org/x/image** — BSD-3-Clause
