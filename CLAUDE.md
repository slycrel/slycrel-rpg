# Working on Slycrel

An open-world sword-and-sorcery RPG in Go + Ebitengine. `README.md` says what
the game is; `docs/PLAN.md` says where it is going, what is already built, and
why each decision went the way it did. Both are kept current — read them before
assuming anything about the state of the project.

## Commands

```bash
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

- **The game never comments on its own joke.** Bawdy, absurd, delivered
  completely straight, in the same flat voice as the damage numbers.
- **Everything that gives must take.** Lineages, affixes and charms are all
  authored trade-offs, and tests assert it. A table of pure upgrades makes "did
  I get the good one" the only question worth asking.
- **Flavour is data**, in `data/text/flavor.json` and the per-monster lines, so
  the writing can be revised without touching Go.
- **Never name something that might not exist.** The quest generator only names
  places and items it has checked for; a test asserts the same of save fixtures.
- **Gear whose name already ends in a flourish never gets an affix**, or you get
  "Runed Maul of the Last Word of the Last Word".
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
- **`saves/` is gitignored except `saves/fixtures/`**, which is committed: it is
  both the regression net and the set of playtest starting points.
- **The `autosave` slot is written before every fight** and offered back if the
  hero dies. It is an ordinary save in an ordinary slot, so it turns up in the
  load menu and can be loaded on purpose; `-demo` is excluded, or the tour would
  scribble over a real run.
- **`v1-solo.json` and `v2-company.json` must never be regenerated.** Their job
  is to be old saves — v1 predates the party, v2 predates the backstories — and
  rewriting either at the current version deletes the only evidence that the
  loader still reads the earlier format. `cmd/genfixtures` skips both.
- **The UI font is Latin-1 only.** `internal/render` folds typography to ASCII
  before drawing; do not bypass it, or an em-dash renders as `@`.
- **Screen capture is blocked on this machine.** Use `-demo`, or `\` in game to
  dump the framebuffer. `sips` crops a shot for a closer look.
- **The display session drops occasionally.** See the project memory; it is
  environmental, it recovers, and most of the suite runs without it.

## Scope discipline

`-demo` is a tour: one frame per screen. It has drifted once already (an
auto-fight window, and a `demoLevelParty` call that couples it to the balance
numbers). Patching it temporarily to stage a scenario is fine — revert it. For
reaching a specific game state, prefer a save fixture.
