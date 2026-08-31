# Working on Slycrel

An open-world sword-and-sorcery RPG in Go + Ebitengine. `README.md` says what
the game is; `docs/PLAN.md` says where it is going, what is already built, and
why each decision went the way it did. Both are kept current — read them before
assuming anything about the state of the project.

## Commands

```bash
./play.command                       # build this checkout and play it (double-clickable)
./scripts/test-headless.command      # the whole sweep in a container, no screen needed
./make-dist.command                  # shareable zips for macOS and Windows into dist/
go run ./cmd/slycrel                 # play
go run ./cmd/slycrel -seed 1994      # the same continent every time
go run ./cmd/slycrel -demo           # scripted tour, one frame per screen into shots/
go run ./cmd/slycrel -audit          # content against the art manifest, then exit
go run ./cmd/slycrel -load saves/fixtures/battered.json   # start from a known state
go run ./cmd/balance                 # win rates, endurance, progression, economy
go run ./cmd/genfixtures             # rewrite save fixtures after a world-gen change
go run ./cmd/assetpipe build         # rebuild all derived art, in order, then the manifest
go run ./cmd/pixelsmith gen -name x -head 6   # draft an icon with a local model, for shapes the bundle lacks
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
- **And a policy that estimates what the rules do is a second copy of the
  rules.** Different failure, same cost, and harder to see because both halves
  look right alone. `incomingPerRound` priced a caster's blow at 0.85 of offence
  where `MonsterDamage` swings for 0.53, so the retreat, the gamble and the heal
  all over-read magical damage by three fifths at exactly the levels where half
  the blows are magical. `freeSwingWorth` described a swing as `Str()/2 +
  Strike()` where `PlayerDamage` rolls half again that — so a Fighter cast
  techniques worth a third of a swing, and buying psyche made it *worse*, which
  read as a trap stat until the policy was fixed. Both are now one line calling
  the real arithmetic. When a stat measures as harmful, suspect the policy that
  chooses between it and the alternative before suspecting the stat.
- **`gamedata.Equip` is the "on curve" assumption**, and it is load-bearing. The
  equipment pass moved the entire report without changing a single stat, purely
  by making that assumption richer. If the report shifts, suspect the assumption
  before the content. `Equip` is `Archetypes[0]`, the balanced build, and must
  stay that way — it dresses hirelings and fixtures as well as the simulator,
  and `TestEquipIsStillTheBalancedArchetype` is what says so.
- **Every build in ARCS shops with the same purse**, which is what balanced
  costs *that class* at that level; `Tables.EquipWithin` fits a shape to a
  budget. This is not a refinement, it is the difference between measuring a
  build and measuring a budget: the duelist used to carry 15-18% more gear than
  the baseline and duly won more levels. The spend column is printed so the
  residue is visible — gear is banded, so a build whose next upgrade costs more
  than it has left stops short, and attrition stops 8-10% short at every level.
  The on-level column still cannot discriminate: every build wins 96-100% of
  on-level fights by design, so comparisons are made on the stretch fights
  three levels over.

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
- **Everything that gives must take, and EXCHANGE is what says how much.**
  Lineages, affixes and charms are all authored trade-offs, and tests assert it.
  A table of pure upgrades makes "did I get the good one" the only question
  worth asking — but so does a table whose trades are priced by eye, because
  "nine points of ward for two of guard" is neither generous nor stingy until
  both are in one currency. The EXCHANGE section of `cmd/balance` measures what
  a point of each stat buys, per class, on both of LANES' bands, and the content
  tables are authored against it. At the top of the game: strike 1.0, strength
  0.7, ward 0.6, speed and dexterity 0.25, **defense 0.14** — guard has
  collapsed as a currency by level thirteen because on-curve armour already
  cancels most of a physical roll, which is why a lane that sells guard cannot
  be rescued by selling more of it.
- **An encounter has a shape, and `PickMonsters` is not it.** `PickEncounter` is
  what the game throws — a composition with a name the transcript says out loud.
  `PickMonsters` still means "n creatures at level L" and every control in
  `cmd/balance` calls it, because a measurement wants one variable. Shapes are
  measured in their own SHAPES section against `rules.SimulateGroup`, which
  exists because a composition does not survive being flattened into a list of
  definitions and a level.
- **Gear icons are banded by tier, and the band is baked.** `assetpipe bands`
  writes a palette-ramp recolour of a gear icon per tier into
  `assets-raw/_generated/bands/`, so a shop row says which of two coats is the
  better one without the player reading the price. Nothing tints at draw time.
  The band is part of the key — `icon/band/garb/jerkin3_t4` — so a content edit
  picks the rung, and `TestArmorIconsAreDistinct` refuses two pieces on one
  picture. **Look in every icon set before concluding art is missing**: armour
  wore a tuft of fur for the life of the project because the loot pack was the
  only set anyone checked, and the answer was ten garments on a paper-doll sheet
  in an unopened pack. `docs/ASSET-MAP.md` is the standing list, with what the
  last pass deliberately left unsettled.
- **`assetpipe build` is the pipeline, and the order in it is load-bearing.**
  Ten steps, and running them out of order fails silently in the worst way:
  `bands` before `arms` bands last run's weapons, and `manifest` before `bands`
  leaves the new keys absent, whereupon the game falls back and `-audit` still
  reports "all referenced art resolves" because the content names what it always
  named. Nothing anywhere says a word. Use `build`; the individual subcommands
  are for working on one step. The tree is disposable — delete
  `assets-raw/_generated` and rebuild, and the manifest comes back byte-identical.
- **An icon the bundle does not have is drawn as a grid, never as a PNG.**
  `cmd/pixelsmith` drafts one with a local model and `data/art/<name>.txt` holds
  the result as sixteen lines of palette indices; `assetpipe drawn` renders it.
  A model is not deterministic and `build` must stay byte-reproducible, so the
  model never runs in the pipeline — and the committed grid contains no
  purchased pixels, only which of six slots each cell uses. Both tools read the
  palette from `internal/pixelpal`: they once disagreed about slot 4 and
  rendered a valid grid in the wrong colour, which nothing detects. The method,
  and the five ways it went wrong, are in `docs/DRAWING-WITH-A-MODEL.md` — read
  it before drafting a second one, and look for the picture in the bundle first
  regardless.
- **A pipeline step wipes its own output directory.** `bands` takes its source
  list from the content, so a picture the table stops naming has to stop
  shipping — the manifest enumerates the directory, and a stale file would keep
  its key and ride into every release build.
- **Flavour is data**, in `data/text/flavor.json` and the per-monster lines, so
  the writing can be revised without touching Go.
- **Never name something that might not exist.** The quest generator only names
  places and items it has checked for; a test asserts the same of save fixtures.
- **Gear whose name already ends in a flourish never gets an affix**, or you get
  "Runed Maul of the Last Word of the Last Word".
- **A creature's name plate says both halves, and prose says the species.**
  `model.SplitName` divides "Crab, Territorial" at the comma and keeps the
  group letter with the species; a name with no comma is all species, which is
  right for "Owl That Knows" where the phrase is the joke. The portrait carries
  both lines, always, because an epithet that waits for the target cursor is a
  joke that lands once a fight. Prose uses `Monster.Short()` — "Wolf B bites
  Bosk", not "Wolf, Deeply Unimpressed B bites Bosk", and a taunt naming the
  creature twice made that unreadable. `TestEveryMonsterNameFitsItsPlate` walks
  the roster against every layout, so a new monster with a long name fails a
  test rather than appearing on screen as "Goblin Middle".
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
  "+1". **Which lane is right moves with the game, and `Equip` moves with it**:
  `gamedata.LaneForLevel` takes the wall below level ten and the spiked one
  from ten up, because below ten nothing on the shelf is measurably better than
  anything else and from ten the spiked shield wins on both axes — and it wins
  the *defensive* one, which is the finding, because it flees less than the
  wall does and still dies nine points less often. It kills the thing. The
  crossover is measured by the LANES section of `cmd/balance`, which warns
  when the constant and the numbers drift apart, and which asks the question
  the constant actually decides: is the lane `Equip` hands ever behind the
  shelf, at two levels running, for the same challenger? It was `ArmBlock` for
  the life of the report purely because that is the zero value of the field,
  and then it was `ArmWard` from level six for two sessions on the strength of
  a table that averaged three classes and called a one-point gap a result.
- **A caster's off arm holds a talisman, not a shield**, and what it carries is
  `Absorb` rather than `Defense`: a pool spent once per fight against damage of
  any kind. It is not more ward, and that was measured rather than assumed — a
  Mage was already taking half what a Fighter takes from magic and double from
  steel, so a bigger ward would have been an upgrade to the column they were
  winning.
- **Every slot in `Equip` is an assumption, and two of them were never made.**
  The off arm took `ArmBlock` because it is the zero value of the field; the
  charm took `cs[len(cs)-1]` because it is the last row of the file. Both cost
  real points for years — the charm one cost a level-11 Thief 12.5 points of
  win rate and a third of its endurance. `LaneForLevel` and `CharmValue` are
  the decisions that replaced them, and LANES and CHARMS in `cmd/balance` are
  what keep them honest: each re-derives its answer every run and warns when
  the constant and the fights disagree. If you add a slot, the question to ask
  is not "does this compile" but "what measured this".

  **And read a group fight before believing a class comparison.** The classes
  sit within five points of each other one-on-one and seventy apart against
  three; every section that sets the curve fights one creature at a time, and
  the stretch column is saturated for groups — three creatures three levels
  over is a nought for everybody. CROWDS is the section that measures it.

  **Five occurrences now, and read a report per class before believing it.**
  The fifth was `LaneForLevel`, which took a level and no class, and it is
  fixed — but the fix was the *instrument*, not the constant, and that is the
  part worth keeping. **A threshold nobody measured is a decision nobody made.**
  LANES gated its crossover on a named constant of 1.0 points, with a paragraph
  of reasoning and no measurement under it, while the table it gated already
  contained the experiment: every row where all three lanes dress the character
  identically — a Mage, who cannot hold a plank; anybody below the level the
  baseline affords an off arm — is three identical builds measured three times,
  and the spread across those columns is sampling wobble by construction. It
  runs to four points. So the crossover had been read off noise, the switch was
  four levels early, and it switched to the lane that is *worst* at the top.
  Look for the null rows already in a table before inventing a threshold for
  it, and print them, marked, so the reader can check. **And check that a check
  can fail**: this one was run against the old constant to watch it fire before
  it was believed passing against the new one.

  The fourth was `duelist`, which never set `Arm` — so on the two classes that
  cannot hold a two-hander the fallback leaves the arm free and fills it with
  the wall. It was invisible because ARCS averaged its three classes into one
  row, which also hid a Fighter duelist beating the baseline by 5.7 points at
  88% of its purse. An average is not a measurement of anything that exists;
  every other section of this report is per class and ARCS was the outlier.

  The third is the one worth remembering: the
  `attrition` archetype had no `Arm` field either, so the build whose entire
  identity is the off arm carried the worst off-arm lane from level six up. It
  read as a trap — losing every level, by a margin that widened to 14.5 points
  — and it was a zero value. Naming the field moved the widest gap in ARCS from
  16.0 points to 7.2 and turned "one arc and a trap" into three live builds.
- **A companion's cut is a purse, not a subtraction.** `Skim` goes into the
  ally's own `Coins`; `gamedata.Wants` says what they are behind on and
  `Shop` spends it when the company walks into a settlement with the right
  counter. The target is `Equip` — one definition of "on curve", and a
  companion's kit lags it — and the comparison is on *price*, which is the only
  one that works in all four slots since a charm has no better, only dearer.
  Nothing re-arms anybody for free any more.
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

- **A fraction of a bounding box is not a fraction of the land.** Every
  location's difficulty was `d / (half the map's diagonal)`, and the continent
  is a blob inside that rectangle: the land only reaches 59% of the way to the
  corners, so 41% of the range fell off the edge of the world and no location
  above level 9 was ever generated, in any seed, for the life of the project.
  The trap has two halves worth remembering separately. The formula was
  scale-invariant, so "make the world bigger" — which is how it sat on the open
  list for months — would have moved nothing at all. And nothing caught it
  because `cmd/balance` reads `biomeForLevel`, a lookup from level to biome
  name, rather than the map: the report measured a danger curve at levels 10-14
  without ever asking whether the world could produce them. `gradeByDistance`
  now normalises against the map's own reach, and
  `TestTheWorldReachesTheTopOfItsOwnContent` asserts the property rather than
  the arithmetic.
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
- **A fixture embeds the gear it was written with, icons and all**, so an art
  pass or an `Equip` change leaves every one of them stale — and nothing
  catches it: `TestIconsResolve` walks the tables, not the saves, so a fixture
  naming a retired icon key shows a placeholder in a playtest and passes the
  suite. Re-run `cmd/genfixtures` after either, and read the diff: it should be
  gear and icons only.
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
- **Preferences live in `internal/prefs`, and nothing else owns that file.**
  `saves/settings.json` holds volume, combat pace and key bindings; the audio
  bank is *told* its settings and writes back through a callback rather than
  reading the file itself. Every field's zero value means "never set it" —
  which is why an unset pace is the middle rung and not a combat step of no
  ticks. Key bindings are stored as `ebiten.Key.String()` names, and the
  name table is built by walking `0..KeyMax` rather than typed out.
- **A menu dispatches on what a row means, never on where it is.** The pause
  menu learned this once and the battle menu was one change away from it: rows
  carry a tag in `Data` and the switch reads that. The attack row's label is the
  weapon's name, so it is not even readable as a constant any more.
- **Never offer a choice you are about to refuse.** `g.AskMenu` takes rows
  rather than strings, so a price goes in the detail column and an option
  nobody can afford is greyed out in advance. The inn, the shrine, the hiring
  board and the backstory endings all quote what you are holding next to what
  the thing costs; anything new that charges for something should do the same.
- **A detail column is a fact about the moment it was rendered.** The load
  menu's "4m ago" was written into the row at refresh time, so it froze the
  instant the screen opened and only ever corrected itself on a save — which is
  the one action that refreshes. Anything derived from the wall clock has to be
  recomputed in `Update`, not baked in when the rows are built. The times are
  already parsed and in hand, so re-ageing costs nothing; re-running `refresh`
  would re-read every save file on disk sixty times a second.
- **Nothing on a strip is positioned by a constant that assumes another thing's
  width.** The status bar was a column of fixed x offsets, which held until the
  generator produced "Sister Agatha Blunt Two Drinks In" or a purse reached four
  figures — then the name printed through the weather and the coins printed
  through the tracker. Lay a row out from its anchor and fit the variable parts
  to what is left, and decide *which* thing gives way by which thing moves: a
  hero's name is fixed for a run, a place name changes every time you walk
  through a gate.
- **The walking-around screen shows one row of the transcript.** `Log.AddColor`
  wraps at `ScreenW-40` before storing, and `Game.drawStatusBar` draws the last
  *row* — so anything much over sixty characters logged outside a battle
  appears on screen as its own second half. Keep overworld and local lines
  short, and when several go in at once, put the one worth reading last.
- **A wrapped log fills its last row with a sentence's tail.** Taking the newest
  N *rendered* rows out of a log means the oldest visible entry can lose its
  beginning — "Weather settles into place. 8." is not a shorter version of what
  happened, it is a different sentence, and nothing on screen says it was cut.
  `Log.DrawWrapped` takes an entry whole or not at all.
- **A panel does not clip what you draw in it.** `ui.TitledPanel` now truncates
  its own title, but `Log.Draw`, `render.Text` and a pre-wrapped blurb will all
  happily run out of the box they were meant for and across whatever is beside
  it. Anything drawn into a panel narrower than the screen needs the width
  passed in — `Log.DrawWrapped` and `techniqueBlurb` both take one for exactly
  this reason.
- **A note that outlives the cursor is a screen that has stopped answering.**
  The shop's description strip and its "you bought a thing" line share one
  space, and the note wins while it is set — so a note never cleared on
  movement meant one purchase permanently disabled the only line that said what
  anything was. Anything written in reaction to an action has to be cleared by
  the next navigation, not by the next action.
- **`S` is the down key, and so are `J`, `A`, `D`, `H`, `K`, `L`.** WASD and vi
  keys between them claim most of the alphabet's convenient letters, so a new
  hotkey has to be checked against `upKeys`/`downKeys`/`leftKeys`/`rightKeys`
  before it is bound. Bulk-selling was one commit from living on `S`, which is
  the key that scrolls the list it would have emptied. When a bulk action needs
  a home, a row is nearly always better than a key: it costs no binding, it
  quotes its price in the detail column like every other row, and it can grey
  itself out — which is the rule the inn and the hiring board already follow.
- **Interface goes over the sky tint.** `drawSky` runs after the entity pass, so
  anything drawn with the sprites gets the weather and the night painted over
  it. The labels already sit in a later pass for this reason; the attention
  stars had to move there too, having been invisible in rain — which is the
  weather you most want to be told things in. If it is meant to be read rather
  than inhabited, draw it after the sky.
- **`Sprite.Head` is where the art starts, and it is not the top of the frame.**
  Character art is anchored on its feet inside a generous box, so how far a
  head is above its tile depends entirely on the sheet: townsfolk are about a
  tile, winged hirelings better than two. Anything floating over a character —
  a mark, a bubble — has to measure with `sp.H - sp.Head`, exactly as `Foot`
  answers the same question at the other end. A fixed offset lands on the tall
  ones' faces.
- **A sub-image draws from its own bounds origin.** `canvas.SubImage(rect)` then
  `GeoM.Translate(x, y)` puts the slice at x,y — exactly as a sprite sheet's
  frame does. Subtracting the sub-rectangle's own position, which looks like the
  obvious correction, offsets it by that much and drew the corner map up and
  left of its own border, reading as a second panel bleeding off the screen.
  Nothing catches this but a frame.
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
- **The display session drops when the screen sleeps**, and Ebitengine cannot
  init without a monitor — `internal/game` then fails at *package init*, even
  with `-run XXX`. `./scripts/test-headless.command` runs the whole sweep in a
  container against Xvfb and does not care: no arguments runs
  `go test ./internal/...`, or pass any command (`./scripts/test-headless.command
  go run ./cmd/slycrel -demo`). First run is minutes, the rest are seconds.
  The host path is still faster when the screen is awake.
- **A worktree has no art.** `assets-raw/` is gitignored, so a `git worktree`
  gets the manifest and none of the files it points at, and the two tests that
  probe the registry fail on missing portraits and effects — which reads as a
  regression and is a missing directory. Symlink it
  (`ln -s /path/to/checkout/assets-raw assets-raw`); the headless script
  follows the link and bind-mounts the target, because a symlink to an absolute
  host path means nothing inside the container.

## What the tour cannot reach, the tests must

`-demo` is a tour and does not exercise the game: `demoWalk` teleports rather
than stepping, so the encounter roll never runs and a wandering monster can
never appear in it; `demoOpenShop` takes the first counter, which is the smith
under every seed tried. Both features were verified by patching the tour, taking
a frame, and reverting — fine once, useless as a regression net.

So anything the tour cannot reach gets a test instead of a longer tour. Growing
`-demo` to cover everything is the drift the scope note below is about. And a
guard is only a guard if it has been seen to fail: every one of these was
checked by breaking the thing it protects.

- `TestOnlyOneEncounterIsEverOut` — the rate guarantee, and the single condition
  that keeps this one encounter system rather than two.
- `TestTheVendorsColumnDoesNotEatTheNames` — a ratchet on how far a shop row is
  truncated, since the vendor's portrait was paid for in item names. Its floor
  is *measured*, not chosen: the first version asserted "no two rows collide"
  and a column nearly three times the real one sailed straight through it.

## Scope discipline

`-demo` is a tour: one frame per screen. It has drifted once already (an
auto-fight window, and a `demoLevelParty` call that couples it to the balance
numbers). Patching it temporarily to stage a scenario is fine — revert it. For
reaching a specific game state, prefer a save fixture.
