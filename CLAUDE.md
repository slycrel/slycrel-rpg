# Working on Slycrel

An open-world sword-and-sorcery RPG in Go + Ebitengine. `README.md` says what
the game is; `docs/PLAN.md` says where it is going, what is already built, and
why each decision went the way it did. Both are kept current — read them before
assuming anything about the state of the project.

## Commands

```bash
./play.command                       # build this checkout and play it (double-clickable)
./make-dist.command                  # shareable zips for macOS and Windows into dist/
go run ./cmd/slycrel                 # play
go run ./cmd/slycrel -seed 1994      # the same continent every time
go run ./cmd/slycrel -demo           # scripted tour, one frame per screen into shots/
go run ./cmd/slycrel -audit          # content against the art manifest, then exit
go run ./cmd/slycrel -load saves/fixtures/battered.json   # start from a known state
go run ./cmd/balance                 # win rates, endurance, progression, economy
go run ./cmd/genfixtures             # rewrite save fixtures after a world-gen change
go test ./internal/...
```

## Cadence

Package tests while working. The full sweep — `go test ./internal/...`, `-audit`,
`-demo`, and `cmd/balance` if rules or content changed — goes just before a
commit, not after every edit.

**When something is reported missing, check first whether it is merely
unreachable.** Two playthroughs produced eleven complaints and seven of them
were features the game already had and would not talk about: Save behind a
switch that dispatched on row numbers after the rows had moved, the quest log
on an unadvertised key, a chest listing item names with no hint what they were
for, the shop showing two columns of prices. The fix each time was a sentence,
not a system. Look for the capability before building it.

Read the diff before committing anything stateful. In the session that built the
party, status effects and equipment, twelve bugs surfaced: five from reading the
code, five from looking at demo frames, and two from tests — and both of those
merely confirmed a hypothesis already in hand. Tests are the regression net, not
the discovery mechanism. Do not write tests for things that cannot break.

## Git

Branch, fast-forward merge into `main`, delete the branch, push. `main` is the
default branch; `master` does not exist. Commit messages carry the reasoning —
what was tried, what the numbers did, what was deliberately left out — because
that is the record nobody else is keeping.

## The balance report is the arbiter

`internal/rules` is pure: no I/O, no globals, RNG passed in. `cmd/balance` plays
that same code rather than a copy of the maths, which is the only reason its
numbers mean anything. Two rules follow:

- **A mechanic the simulator cannot see is a mechanic the balance pass is lying
  about.** When monsters started inflicting poison, `SimulateFight` had to learn
  to apply and tick it.
- **`gamedata.Equip` is the "on curve" assumption**, and it is load-bearing. The
  equipment pass moved the entire report without changing a single stat, purely
  by making that assumption richer. If the report shifts, suspect the assumption
  before the content. `Equip` is `Archetypes[0]`, the balanced build, and must
  stay that way — it dresses hirelings and fixtures as well as the simulator,
  and `TestEquipIsStillTheBalancedArchetype` is what says so.
- **An archetype that underspends measures the spec, not the content.** The ARCS
  section compares builds by trading gear bands between slots, so a build that
  simply buys less loses for uninteresting reasons. Read the cost column before
  believing a gap. The same goes for the on-level column: every build wins
  96-100% of on-level fights by design, so comparisons are made on the stretch
  fights three levels over.

The shape of the curve is pinned by tests — `TestDamageHasNoCliff`,
`TestOnLevelFightsAreWinnable`, `TestDangerRadiatesOutward`,
`TestEnduranceHoldsAcrossLevels` — which assert shape, not exact values, so
content can be added freely.

## Where things live, and why

Ebitengine opens a window at package init on macOS, so anything importing it —
directly or through `assetsys` — cannot run without a display. `internal/game`
is the scene stack and the drawing; everything that is neither lives outside it:
the roster and marching order in `internal/party`, the tile walker and
`TileSize` in `internal/core`, the maths in `internal/rules`, the errands in
`internal/quest` and the companion backstories in `internal/thread`. Keep new
domain logic out of `internal/game`, which should be left holding the wiring:
which event fires which trigger, and where it is safe to put a box on screen.

## Content conventions

- **The game never comments on its own joke.** This is why an oddity imports
  cyberpunk *furniture* and not cyberpunk *people*: a villager treating a
  vending machine as a wall with a slot in it is the joke, and somebody in the
  frame dressed for the machine would be somebody on screen who is in on it. Bawdy, absurd, delivered
  completely straight, in the same flat voice as the damage numbers.
- **Everything that gives must take.** Lineages, affixes and charms are all
  authored trade-offs, and tests assert it. A table of pure upgrades makes "did
  I get the good one" the only question worth asking.
- **An encounter has a shape, and `PickMonsters` is not it.** `PickEncounter` is
  what the game throws — a composition with a name the transcript says out loud.
  `PickMonsters` still means "n creatures at level L" and every control in
  `cmd/balance` calls it, because a measurement wants one variable. Shapes are
  measured in their own SHAPES section against `rules.SimulateGroup`, which
  exists because a composition does not survive being flattened into a list of
  definitions and a level.
- **Flavour is data**, in `data/text/flavor.json` and the per-monster lines, so
  the writing can be revised without touching Go.
- **Never name something that might not exist.** The quest generator only names
  places and items it has checked for; a test asserts the same of save fixtures.
- **Gear whose name already ends in a flourish never gets an affix**, or you get
  "Runed Maul of the Last Word of the Last Word".
- **Never write `a %s` around a generated name.** Monsters, gear and places are
  all generated, so "a Actual Sword of Mild Regret" and "a Owl That Knows" are
  the default outcome, not the unlucky one. Use `article()` in `internal/game`,
  and do not pluralise one either — the tests in `internal/thread` assert both
  for the backstory writing.
- **Equipment has lanes, and the gate is hard.** Five weapon kinds, three
  armour weights, two kinds of off-arm item, and `model.CanWield` / `CanWear` /
  `CanHoldShield` decide who may hold what. Anything that dresses a character
  goes through `Tables.EquipAs`, which honours it; anything that reaches into a
  table by hand does not, and that is how a Fighter ended up holding a dagger in
  a save fixture. The empty string is the legacy kind on all three and means
  "anyone", which is the only answer an old save can give.
- **The off arm is three lanes plus the caster's**, and `model.Shield.Lane()`
  reads which from what the item does rather than from a tag. A wall sells
  guard, a silvered one sells ward, a spiked one sells strike, and they are
  graded on the shelf against their own lane's number — ranking all of them by
  `Defense` tells somebody shopping for anti-magic that a shrine plate is worth
  "+1". Which lane is right moves with the game: nothing casts below level ten
  and two thirds of what lands on you by thirteen is magical.
- **A caster's off arm holds a talisman, not a shield**, and what it carries is
  `Absorb` rather than `Defense`: a pool spent once per fight against damage of
  any kind. It is not more ward, and that was measured rather than assumed — a
  Mage was already taking half what a Fighter takes from magic and double from
  steel, so a bigger ward would have been an upgrade to the column they were
  winning.
- **Equipment is carried, not just worn.** `model.Carried` is the pack version;
  buying, finding and taking something off all go there, and `Character.Equip`
  swaps rather than replaces. Nothing should ever destroy a piece of gear.
- **A companion backstory is authored writing over generated staging.** The
  skeletons in `data/text/threads.json` may only name `{N}`, `{P}`, `{X}` and
  `{I}`; `internal/thread` reads which of those a skeleton needs out of its own
  text, so adding a placeholder is the whole of adding a role. Two rules are
  enforced by tests: every thread has an ending that costs nothing (a broke
  player must never be stuck holding a story), and no ending may beat another on
  every axis (otherwise the choice is a formality with a menu in front of it).

## Gotchas

- **`core.RNG.Fork` never reads its receiver.** It derives the child stream from
  the label and the salt alone, so `g.RNG.Fork("thing", 0)` is the same stream in
  every run of every seed. Whatever should vary has to go into the salt —
  `poi.Seed` for an interior, `g.Seed` for anything cast once per run.
- **World regeneration needs the real writer.** Location names are drawn from
  the same `*core.RNG` that places the locations, so a stub namer builds a
  *different continent*. Anything comparing a save against its world must use
  `content.New(&tables.Text)`. (`internal/gamedata`'s world tests use a stub
  deliberately — they assert structural properties, not identity.)
- **A zero value that means something real will be wrong in every old save.**
  The save format is seed plus deltas, so a field added today is *absent* in
  every file written before today and unmarshals to zero. `Thread.HomePOI >= 0`
  meaning "this belongs to a resident" turned every companion thread in every
  existing save into a resident of whichever location happens to be first on the
  map. Derive the flag from something authored (the skeleton), or carry an
  explicit `On bool` whose zero value is the safe answer — which is what
  `Track` does, having learned it the hard way one commit earlier.
- **`saves/` is gitignored except `saves/fixtures/`**, which is committed: it is
  both the regression net and the set of playtest starting points.
- **The `autosave` slot is written where the player is safe** — a bed, an
  altar, the first morning — and offered back when the hero dies alone. It used
  to be written before every fight, which made a death cost one fight, which is
  barely a cost. Checkpointing at rest is what gives the inn a job beyond hit
  points. It is also in the Load menu now, as a fourth row on the load side
  only: offering it as a save destination would let a player overwrite the
  safety net by hand. `-demo` is excluded from writing it — the guard is inside
  `autosave()` so it travels to any new call site — or the tour would scribble
  over a real run.
- **`v1-solo.json` and `v2-company.json` must never be regenerated.** Their job
  is to be old saves — v1 predates the party, v2 predates the backstories — and
  rewriting either at the current version deletes the only evidence that the
  loader still reads the earlier format. `cmd/genfixtures` skips both.
- **A menu dispatches on what a row means, never on where it is.** The pause
  menu learned this once and the battle menu was one change away from it: rows
  carry a tag in `Data` and the switch reads that. The attack row's label is the
  weapon's name, so it is not even readable as a constant any more.
- **Never offer a choice you are about to refuse.** `g.AskMenu` takes rows
  rather than strings, so a price goes in the detail column and an option
  nobody can afford is greyed out in advance. The inn, the shrine, the hiring
  board and the backstory endings all quote what you are holding next to what
  the thing costs; anything new that charges for something should do the same.
- **`render.Text` does not put ink where you think.** Text drawn at `y` inks
  `y+2` through `y+12`, which is why `render.TextInkTop` and `TextInkH` exist —
  measured off a render rather than derived from the font. Anything drawing a
  background behind text has to use them or it clips the letters.
- **The UI font is Latin-1 only.** `internal/render` folds typography to ASCII
  before drawing; do not bypass it, or an em-dash renders as `@`.
- **The bundle has no UI art that suits this game, and that has been checked.**
  All 4,488 GUI PNGs are painted mobile and MMO interfaces at two to four times
  a 480x270 framebuffer's scale: three-pixel outlines and soft drop shadows
  against a 7x13 bitmap font. The closest match in the whole bundle is GUI Pro's
  `ItemFrame_01` — a thin gold border with clipped corners, which is a
  description of `ui.Panel`. So `internal/ui` being procedural is a decision,
  not a placeholder, and the art pass went the other way: extend the vocabulary
  already there (`ui.Slot`, `ui.Cursor`) to the screens that had none. Do not
  re-run this survey; the packs have not changed.
- **Screen capture is blocked on this machine.** Use `-demo`, or `\` in game to
  dump the framebuffer. `sips` crops a shot for a closer look.
- **The display session drops occasionally.** See the project memory; it is
  environmental, it recovers, and most of the suite runs without it.

## Scope discipline

`-demo` is a tour: one frame per screen. It has drifted once already (an
auto-fight window, and a `demoLevelParty` call that couples it to the balance
numbers). Patching it temporarily to stage a scenario is fine — revert it. For
reaching a specific game state, prefer a save fixture.
