# Slycrel

An open-world sword-and-sorcery RPG in Go. Walk a generated continent, wander
into places that are worse than they look, fight things in a turn-based battle
screen, sell their component parts to a man who does not meet your eye.

Descended from **Slycrel**, a text RPG door game written in Pascal for the
Hermes II BBS on Mac Classic circa 1994–96 by Dave Dolinar and Jeremy Stone,
later ported to C++ by Bill Dolinar, and recreated in Go in `../new-slycrel`.
This project is a fresh world built on the old bones: the combat, levelling,
and initiative maths in `internal/rules` are a direct port, so the balance has
already been playtested by a few hundred BBS users, three decades ago.

**This game is 18+.** The humour is bawdy, the violence is comic, and the
tone-setting rule is that nobody in the world ever acknowledges the joke.

| | |
|---|---|
| ![The overworld: roads and terrain with locations marked](docs/screenshots/overworld.png) | ![A turn-based battle against three monsters](docs/screenshots/battle.png) |
| Overworld — roads, terrain, points of interest | Battle — portraits, transcript, commands |
| ![A town of red-roofed buildings along a paved street](docs/screenshots/town.png) | ![A dungeon of rooms and corridors with lurking creatures](docs/screenshots/dungeon.png) |
| Town — generated from the location's seed | Dungeon — rooms, foes, chests, a boss |
| ![A parchment map showing explored terrain](docs/screenshots/map.png) | ![The character sheet and pack](docs/screenshots/character.png) |
| The map — only what you have walked past | Character sheet and pack |

## Running it

```bash
go run ./cmd/slycrel               # a new continent every launch
go run ./cmd/slycrel -seed 1994    # the same continent every time
go run ./cmd/slycrel -scale 4      # bigger window (integer scales only)
go run ./cmd/slycrel -audit        # check content against the art manifest, then exit
go run ./cmd/slycrel -load saves/demo.json   # start from a save
go run ./cmd/slycrel -mute        # run silent
```

`-load` takes any save file, which makes saves useful as test fixtures: the
`-demo` tour leaves one at `saves/demo.json` with coins, loot, a partly
explored map and a half-looted dungeon, so a playtest can start somewhere
interesting instead of at level one.

Requires Go 1.25+. The art lives outside the repo; see **Assets** below.

## Controls

| | |
|---|---|
| Arrows / WASD | walk |
| Z, Enter, Space | confirm, talk, enter a location |
| X, Esc | back out |
| M | the map of everywhere you have been |
| C or I | character sheet and pack |
| Esc | pause: sound, save, load, abandon the run |
| `\` or F12 | screenshot to `shots/` (F12 needs standard function keys on macOS) |

## What is in the box

- **A generated continent** — 160×120 tiles of coast, forest, hills, mountains,
  mire, desert and scorched waste, with rivers that run downhill to the sea and
  roads that connect every settlement to the capital. Danger radiates outward
  from the capital, so walking somewhere you should not be is punished
  immediately rather than when the plot says so.
- **~40 points of interest** per world — capital, towns, villages, castles,
  dungeons, caves, ruins, towers, shrines, camps, and whatever an "oddity"
  turns out to be this time. Each generates its own interior on demand from its
  seed, so leaving and coming back gives you the same town without storing it.
- **Turn-based battles** against up to three monsters, with initiative,
  targeting, techniques, items, defending, and fleeing.
- **54 monsters** across nine biomes, each with its own attack verbs, defensive
  flavour, taunts, death lines, and drop table.
- **Shops, inns, chests, altars and townsfolk** who all have opinions.
- **Blended terrain** — real pixel-art ground textures with quarter-tile
  autotiling, so grass fringes dirt and sand fringes the sea instead of meeting
  at hard grid edges. Roads outrank the land they cross; rock and snow sit on
  top of everything.
- **Sound** — 33 cues drawn from the bundle's sound packs: combat impacts,
  interface clicks, coins, chests, and four looping ambience beds that follow
  the ground underfoot. Each cue has several source files and picks one at
  random, so a sword landing forty times does not click the same way twice.
  There is also a retired magician who occasionally comments on your victories.
- **Save and load** — three slots, reachable from the pause menu (Escape) or
  Continue on the title. A save is the seed plus whatever you changed, so it is
  a few kilobytes of readable JSON rather than a dump of the map.

## Project layout

```
cmd/
  slycrel/            the game
  assetpipe/          inventory, extract, and index the source art bundle
internal/
  core/               RNG that forks deterministically, grid maths
  model/              characters, monsters, gear, spells
  rules/              combat / levelling / loot maths, ported from the original
  world/              continent generation, terrain, points of interest, interiors
  gamedata/           JSON content loading
  content/            the writing room: names, signs, taunts, combat narration
  assetsys/           art registry, frame slicing, generated placeholders
  render/             camera, sprites, text, the fixed 480x270 framebuffer
  ui/                 panels, meters, menus, message log
  game/               scene stack and every screen
data/
  monsters/           one file per biome
  items/              weapons, armor, consumables, spells
  text/               the word banks the generators recombine
assets/manifest.json  asset key -> file, generated by assetpipe
docs/                 the plan, and the asset inventory
```

## Assets

The art comes from the Humble *Complete RPG Creator Bundle* — 78 packs, 56,409
files, 16.7 GB. None of it is committed. `assets-raw/` is gitignored and
rebuilt from the purchased zips on demand:

```bash
go run ./cmd/assetpipe inventory        # writes docs/ASSET-INVENTORY.md
go run ./cmd/assetpipe extract tier1    # the ~33 packs the game currently uses (2.8 GB)
go run ./cmd/assetpipe manifest         # index art into assets/manifest.json
go run ./cmd/assetpipe audio            # index sound into assets/audio.json
go run ./cmd/assetpipe find viking walk # locate a file in what is extracted
```

Set `SLYCREL_BUNDLE` if the bundle is not at `~/Desktop/RPG Maker Stuff`.

Anything the manifest does not cover is generated procedurally at runtime, so
the game always runs and curation can proceed one asset at a time without ever
leaving the build broken. Terrain still autotiles with generated textures if
the bundle is absent; only the look changes, not the behaviour.
`-audit` reports exactly what is still falling back, and decodes every sound
file to prove the cues will actually play rather than just that the JSON
parsed.

## Shipping a build

```bash
./scripts/dist.sh                 # host platform
./scripts/dist.sh windows amd64   # cross-compiles from macOS or Linux, no cgo
./scripts/licenses.sh             # refresh licenses/ after a dependency change
```

Produces `dist/slycrel-<os>-<arch>/` and a zip beside it: the binary, `data/`,
and only the ~92 MB of art and audio the manifests actually reference — not the
16.7 GB bundle. An 81 MB zip that runs standalone.

Ebitengine reaches DirectX and Win32 through purego, so **Windows
cross-compiles cleanly with nothing but Go**. macOS and Linux need cgo and must
be built on the platform they target.

The binary finds `data/` next to itself as well as via the working directory, so
a double-clicked build works even though its working directory is `/`.

Builds contain third-party art and audio. Shipping that inside a game is
licensed; publishing the folder as an asset pack is not. `dist/` is gitignored.

## Testing

```bash
go test ./internal/...
```

They target the things that fail silently: loot tables referring to items that
do not exist, a biome with no spawnable monsters, an XP curve that stops being
monotonic, world seeds that generate an uninhabitable island, and interiors
with no exit.

For the visual half there are two more tools. 
```bash
go run ./cmd/slycrel -demo     # scripted tour: one frame per screen into shots/
go run ./cmd/slycrel -keylog   # trace every key the engine reports, to stderr
```

`-demo` drives the scene stack directly rather than faking input, so a captured
frame never depends on input timing. It is the fastest way to find a panel that
draws off-screen — and it is how the bug where `Replace` quit the game on the
title screen was caught.

To drive the *real* input paths from a script on macOS, note that the terminal
takes focus back the instant the shell command returns, so focusing and typing
have to happen inside one `osascript` invocation:

```applescript
tell application "System Events"
  set frontmost of process "slycrel" to true
  delay 0.8
  key code 6    -- z, confirm
  key code 125  -- arrow down, walk
  key code 42   -- backslash, screenshot to shots/
end tell
```

This needs Terminal to hold **Accessibility** permission (System Settings →
Privacy & Security). It does *not* need Screen Recording: the backslash binding
dumps the game's own framebuffer, which is cleaner than a screen grab because
it captures exact pixels with no window chrome or display scaling. Note that
F12 also works but only if function keys are set to standard on macOS.

## Licence

**Everything in this repository is MIT** — engine, rules, content tables, docs,
the asset pipeline, the manifest. There is no carve-out, because no art or audio
is committed here in the first place.

**The art and audio are separately licensed and live outside version control.**
They come from a commercial bundle, belong to their creators, and are not
required to run the game — anything missing falls back to a generated
placeholder.

| | |
|---|---|
| [LICENSE](LICENSE) | MIT, covering this repository |
| [NOTICE](NOTICE) | what is and is not covered, and why |
| [CREDITS.md](CREDITS.md) | the creators |
| [licenses/](licenses/THIRD-PARTY.md) | vendored licences for every linked Go module |
| [docs/ASSET-LICENSING.md](docs/ASSET-LICENSING.md) | what the asset licences permit and forbid |
| [docs/ASSET-INVENTORY.md](docs/ASSET-INVENTORY.md) | what is in the bundle |

Note for anyone building a release: shipping a compiled game with the art packed
into it **is** permitted, and players need no bundle of their own. The art is
kept out of this repository because a public repository would hand over the
assets *as assets*, which is the thing the licences actually forbid.

## History

- **1994–96** — original Pascal version, Hermes II BBS (Dave Dolinar, Jeremy Stone)
- **1997–2000** — C++ port (Bill Dolinar)
- **2026** — Go recreation of the original, `../new-slycrel`
- **2026** — this: the open-world one
