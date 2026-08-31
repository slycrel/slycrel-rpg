# Asset Map

What the game actually loads, where each piece comes from, which pieces a
program made, and what is still missing.

Third of three asset documents, and the only one about *this game*:

- [ASSET-INVENTORY.md](ASSET-INVENTORY.md) catalogues the purchased bundle — 78
  packs, 56,409 files — whether or not anything uses them.
- [ASSET-LICENSING.md](ASSET-LICENSING.md) is what may be done with them, and
  remains the authority on every licence question below.
- This file is the manifest: the 804 keys the game resolves at runtime.

The table is generated (`go run ./cmd/assetpipe map`) because a hand-counted
asset list is wrong within two commits. Everything outside the markers is
written by hand and is left alone by that command, because a decision needs a
person to have made it.

<!-- BEGIN generated: go run ./cmd/assetpipe map -->

**919 keys across 15 namespaces.**

| namespace | keys | sources |
|---|--:|---|
| `icon/` | 396 | `_generated/icons` 264, `_generated/bands` 132 |
| `mob/` | 168 | `mobsavataricons_windows` 133, `sci-ficharactersicons` 35 |
| `boss/` | 107 | `monstersavataricons_windows` 107 |
| `portrait/` | 76 | `characteravataricons_windows` 76 |
| `foe/` | 29 | `pixelartrpgtopdownenemies` 23, `_generated/foes` 6 |
| `hero/` | 28 | `pixelartrpgtopdowncharacters` 28 |
| `odd/` | 28 | `pixelartcyberpunkcity` 22, `pixelartwasteland` 6 |
| `ground/` | 25 | `manaseedpixelarttilesetcollection` 25 |
| `npc/` | 17 | `pixelartrpgnpc` 17 |
| `vfx/` | 17 | `pixelartrpgvfx` 17 |
| `poi/` | 9 | `_generated/icons` 9 |
| `wild/` | 8 | `_generated/wild` 8 |
| `prop/` | 6 | `manaseedpixelarttilesetcollection` 5, `_generated/props` 1 |
| `weather/` | 4 | `manaseedpixelarttilesetcollection` 4 |
| `decor/` | 1 | `_generated/decor` 1 |

<!-- END generated -->

## How art gets here

Nothing is generated at runtime. Every derived pixel is written once by
`cmd/assetpipe` into `assets-raw/_generated/`, which is gitignored like the rest
of the extraction tree, and `scripts/dist.sh` copies whatever the manifest names
— so a new pipeline step ships in release builds without touching the packaging.

```
assetpipe extract <pack>   unzip from the bundle into assets-raw/  (read-only source)
assetpipe build            run every step below, in order          <- the only one to remember
```

`build` is the whole derived-art pipeline: props, icons, garb, arms, poi, foes,
bands, manifest, audio, map. Each still exists as its own subcommand for working
on one of them, but the order between them is real and getting it wrong is
silent — `bands` reads what `icons`, `garb` and `arms` write plus the gear
tables, `manifest` enumerates everything above it, and `map` reads the manifest
back. Run `bands` before `arms` and it bands last run's weapons; run `manifest`
before `bands` and the new keys are simply absent, the game falls back, and the
audit still says "all referenced art resolves" because the content names what it
always named. Nothing anywhere reports it. So the order lives in
`cmd/assetpipe/build.go` as a list rather than in a paragraph as advice.

The tree is disposable and reproducible: deleting `assets-raw/_generated`
entirely and running `build` produces a byte-identical `manifest.json`.
`PROVENANCE.txt` is written beside the output saying which step made which
directory and from which pack.

## Decisions on the record

**Icons are pre-reduced, not scaled at draw time.** A menu fits an icon into a
16px box with `render.ScreenFit`, so a 128px painted icon drawn there keeps
every eighth pixel and discards the rest. Box-averaging in the pipeline is the
difference between a readable icon and a smear, and it costs nothing on the
low-end hardware this game is meant to run on.

**Tier is carried by a palette ramp, not a tint.** `bands` maps each pixel's
luminance onto a six-rung ramp — drab, leather, steel, silver, gold, rarefied —
and mixes back toward the original. A tint would be a coloured sheet of glass
over a brown coat; a ramp re-shades it, which is what makes a gilded coat read
as gilded. The ramp weights were not guessed: the first pass was too weak and
the middle rungs were mud, and the second made t3 teal, which is a lateral move
rather than a step up. Both were caught by rendering the sheet and looking at
it, not by a test.

**Armour got real pictures rather than a better recolour.** Armour spent the
project's life with nineteen pieces on six icons — the whole cloth lane wore a
tuft of fur from "Regrettable Rags" to "Shroud of Ongoing Argument". Before
generating anything, all three icon sets on disk were searched: the ability set
is 72 icons of swords, axes and hammers with no garment anywhere in it, the
skill set is 30 spell glyphs, and the loot set is 86 pictures of organs, gems
and berries of which exactly one (`monster_scales`) reads as armour. The answer
was in the bundle: `pixelartminingcrafting` is a paper-doll sheet whose rows 2
and 3 are five torso jerkins and five dresses. Cutting those beat both
recolouring a liver and drawing new art.

**The ability set was retired from gear, and that was a legibility fix.**
`spellsandabilityicons` draws a *spell slot*: a full-bleed square tile with a
purple magical background and the weapon painted over it. Reduced to the 16px a
menu row gives an icon, the background is most of what survives, and every
weapon in the shop was a purple smudge with a hint of grey in a corner. It also
could not say what it needed to — three shapes (sword, axe, hammer) across four
tiers against this game's five weapon kinds, so daggers were drawn as throwing
knives and every wand and staff was drawn as a lightning bolt. Twenty-seven
weapons shared seventeen pictures. `2dminimalskillicons` and its `Skill_Spear`
were checked and are the same full-bleed design, so they are no use either.

**Weapons scale to fit; garments crop. The two are opposite on purpose.** A coat
is 14x17 — one row too tall — so cropping costs a row of hem and leaves every
other pixel where the artist put it. A staff is 25x27 and a bow 19x21, and for
those the shape *is* the length: crop them and you get a stub of haft and a
piece of string. Scaling a long thin diagonal is the lesser damage, and most of
the weapon set (pick, sword, hammer, axe, dagger) fits untouched anyway. The
scale is nearest-neighbour, never a smooth kernel, because an averaged downscale
turns a one-pixel haft into a grey suggestion of a haft.

**Garments are cropped to 16px, never scaled.** The art is 14x17 to 11x22 inside
a 64px cell — taller than the box. Scaling to fit resamples every garment off
the pixel grid and narrows the dresses to a smear; cropping to the top sixteen
rows costs a torso one row of hem and a dress the bottom of its skirt, and both
still read. Decided by rendering the two side by side.

**Location markers are sourced now, and that overturned a written decision.**
`drawPOIMarker` painted its own rectangles for the project's whole life, with
the reason beside it: the building art in the bundle is 300-500px hero sprites
meant for a zoomed-in scene, and reducing one to a 16px cell is mush. That was a
fact about the packs extracted at the time, not about the bundle.
`pixelartrogue-likerpg` draws settlements natively at overworld scale, and nine
of them are wired: capital, town, village, castle, tower, shrine, camp, ruin and
the oddity's beacon. They are cut at native size — 10x25 to 28x28 against a 16px
grid — and `Ctx.World` anchors them on their base exactly as it does a
character, so a castle stands taller than its square instead of being squeezed
into it. Scaling to 16px was tried first and is precisely the mush the old
comment predicted.

The rectangles stay as the fallback rather than being deleted, because a marker
that fails to a coloured box is better than one that fails to nothing — and
dungeons and caves still use them. The sheet has a cave mouth, and it was cut,
wired and looked at in a frame: on the sheet it is an opening set into a block
of stone, so lifted out on its own it is a flat black rectangle that reads on
grass as a hole punched in the screen. The drawn marker has an outline and a
contact shadow and was built to sit on any terrain.

**The rune sheet has colour families, and replacements respect them.** Runes 1-6
are blue, 7-18 green, 19-30 orange through red, 31-42 crimson into void, 43-48
grey, 49-60 pink into purple, and 61-70 are the same stones with no glyph on
them. So the three colliding spells did not take the next free number: "Smoke
and Poor Manners" went to the grey band because that is what smoke is,
"Comprehensive Blight" went to green so it reads against "Comprehensive
Scorch" in red, and "Ambient Wrongness" went to the void band.

**A generator wipes its own output.** `bands` takes its source list from
`armor.json`, so a picture the table stops naming must stop shipping. The
manifest enumerates the directory, so a stale band would otherwise keep its key
and ride into every release build.

**No image model was installed, and none is needed yet.** Diffusion was
evaluated and set aside: it cannot do frame-to-frame consistency or a
four-direction walk cycle, and it does not author a 16px sprite — it makes
1024px pixel-art-*styled* images that need snapping to a grid and quantising to
a palette on the way down. Every generated pixel here is a deterministic
transform of art the project already owns, which also keeps the licence story
unchanged. If a model is ever used, note that **Mana Seed's licence forbids use
in an AI project**, so its tiles must never be fed to one; FLUX-schnell
(Apache-2.0) rather than FLUX-dev (non-commercial) is the licence-safe base.

## Extracted, not yet wired

Five packs were extracted after a sweep of the 40 the bundle still had
unopened. They are on disk and in nobody's manifest:

| pack | what it holds | why it is interesting |
|---|---|---|
| `pixelartrogue-likerpg` | two 512x512 sheets at 16px | map icons and eight overworld creatures are cut and wired. Still unused: the terrain set in six biome colourways, the remaining creature rows, and every creature's non-standing poses. |
| `pixelartdungeonlevel4` | 58 files, 64x64 | the Golem's 24 frames are stacked as `foe/golem/*`, and the five "torch" frames as `decor/brazier` — it is a floor-standing brazier, not a wall torch. Still unused: dungeon floors, doors and chests. |
| `pixelartwasteland` | 51 files, 64x64 | six pieces wired to `odd/`: a sofa, a stop sign, a road sign, a car, a barrel, a bin. Its streets, sand and grass are left alone — a second ground palette against a `groundMaterials` that is deliberately one. |
| `pixelartminingcrafting` | 3 sheets | fully mined: rows 2-3 cut for armour, columns 5-7 cut for weapons. Sheet 2 is ores and ingots, sheet 3 terrain and UI frames; neither is wired. |
| `2dpixelrpgmonsters` | one `.unitypackage` | not unpacked; `assetpipe extract` passes it through untouched. |

`allinonepackrpgmaker` (856 MB) was also extracted and is a poor fit: its Dark
Medieval Age set is genuinely fantasy, but the art is native 48px, not 3x-scaled
16px, so reducing it to this game's grid smears it. Delete it or leave it; it is
gitignored either way.

## Measured gaps

Counted against the content tables, not guessed:

| table | entries | distinct icons | sharing |
|---|--:|--:|---|
| items | 46 | 46 | none |
| weapons | 27 | 27 | none |
| armour | 19 | 19 | none |
| shields | 19 | 19 | none |
| charms | 12 | 12 | none |
| spells | 35 | 35 | none |

**Every shelf in the shop is now distinct.** `TestGearIconsAreDistinct` covers
all six tables and `TestIconsResolve` checks all six exist — the latter had
never looked at shields or charms, so two whole shelves could have named art
that was not there and nothing would have said so.

Charms and spells were closed with icons already in the manifest rather than
new art: 38 of the 70 runes and most of the 90 loot icons were unused. Neither
is banded, and neither should be — a charm has no better, only dearer, and a
rune is a rune.

`prop/` at 6 and `weather/` at 4 look like the thinnest namespaces and are not:
those keys are *sheets*, not pictures. The six props are Mana Seed tilesheets
sliced at 16x16, 16x32 and 32x32, and the four weather keys are 32x128
animation strips covering rain, storm and snow at two densities each. Counting
a sheet as one asset understates both by an order of magnitude. `vfx/` at 17 is
a genuine count.

Two of the three Phase 5 items in [PLAN.md](PLAN.md) are built: overworld
wandering monsters, which gave eight of the rogue-like pack's creatures a home
under `wild/` keyed by monster kind, and interior scenery, which turned out to
need no wall-mounting pass because the "torch" is a floor-standing brazier.

The spear is the one still open, and it is now known to be unclosable from what
we own: six sources checked and a composite attempted, all written up in
[PLAN.md](PLAN.md) so nobody repeats the search.

## What is not settled

This pass is not the last one, and these are the questions it left open.

- **Lane is under-encoded, and the armourer's frame confirms it.** Cloth reads
  as a dress and *both* other lanes as a jerkin, so light and heavy differ only
  by which jerkin and which tier — "Padded Jerkin" and "Ringmail, Secondhand"
  are the same silhouette. `model.CanWear` gates who may wear what, and "never
  offer a choice you are about to refuse" argues the picture should say so.
  There is no cheap fix in hand: the crafting sheet has torsos and dresses and
  no third shape, and the only mail-like pictures anywhere are the two scale
  icons already spent on the two scale coats. It wants either a third
  silhouette that does not exist yet or banding on two axes, which doubles the
  output and muddies tier.
- **Six ramp rungs is a guess that happens to fit.** Gear tiers run 0-5 today.
  If the tier band ever widens, the ramp needs rungs that stay distinguishable
  at 16px, and six is already close to the limit of what a palette can say.
- **The band ramp is the MMO rarity language.** Grey, brown, steel, silver,
  gold, violet is legible precisely because every player has seen it, which is
  also the argument that it is a borrowed idiom rather than this game's own.
  Worth revisiting if the interface ever grows a voice of its own.
- **Two heavy coats at tier 4 are nearly the same picture.** "Half-Plate, Dented
  Fondly" and "Full Plate 'Do Not Perceive Me'" are `jerkin3` and `jerkin4`
  banded gold, and they differ by a strap versus a clasp. Distinguishable on a
  close look, subtle at a glance.
- **Seen now, and the shelf is a colour block.** `demoOpenShop` takes the first
  shop in the interior and that is the smith under every seed tried, so the
  armourer was staged by patching the tour to prefer it, taking the frame, and
  reverting — which is what the scope note in CLAUDE.md allows. The frame is
  worth more than the contact sheet was: sorted by price, the shelf opens with
  four tier-1 pieces, and four tier-1 pieces are four brown blobs. Tier as
  colour is working exactly as designed, and the design's weakness is that a
  tier with several pieces in it hands them all the same colour and leaves the
  silhouette to carry the rest.
- **The oddity's groups are named, so art has to match the name.** Adding the
  wasteland road signs to `oddSigns` would have had the game call a matte red
  octagon "a lit sign", because the group is placed under one description and
  the cyberpunk signs in it are neon. They went to `oddClutter` — "something
  left here" — which claims nothing. Any new piece has to be read against its
  group's sentence before it goes in, and there are only five sentences.
- **The Golem is the only creature with a back and a side.** Its keys follow
  the hero sheets — `walk`/`up`/`side` plus three attacks — where every other
  foe carries the flat `idle`/`walk`/`attack`/`hit`/`dead`. Nothing yet reads
  the directional ones: interior foes are drawn from a single pooled key.
- **The spear is drawn rather than sourced, and it shows.** Six sources were
  checked and none has one (the list is in PLAN.md's Phase 5), so it was drafted
  by a local model via `cmd/pixelsmith` and committed as a grid. It is the
  sparsest icon in the weapon set — a pole with a small point — and earns its
  place only by being unmistakably not the wand it replaces. A hand-drawn
  replacement would be better and would cost one file.
- **The old polearm note, kept because it still describes the glaive:** There
  is no spear anywhere on disk — not in the crafting sheet, not in the ability
  set, not in the skill set. "Glaive, Overcompensating" uses `axe2`, which is
  genuinely a broad blade on a haft and reads right. "Spear, Regrettably Long"
  uses `staff3`, a long haft with a point, which is the closest silhouette
  available and still reads as a wand to anyone not looking carefully. It sits
  at tier 2 next to "Rod of Reasonable Objection" on `staff1`, which is the
  weakest pairing in the table.
- **The bow shape is cut and unused.** There is no ranged weapon kind; reach is
  a polearm's `range`. If one ever arrives, the picture is already there.
- **"Bare Hands" has an icon nothing draws.** The shop skips anything costing
  zero and the character sheet prints the weapon as text, so its `sword1_t0` is
  a formality that exists to satisfy `TestWeaponIconsAreDistinct`.
- **The terrain half of the rogue-like pack is deliberately unused.** Its six
  biome colourways are a whole second tileset, and `groundMaterials` is built on
  one Mana Seed palette on purpose — "so a single palette covers every biome and
  the whole map reads as one place rather than four tilesets bolted together".
  `ground/` at 25 keys is a complete seasonal set under a procedural fringing
  system, not a gap. Anything drawn from the other pack would have to earn its
  way past that sentence.
- **The generated tree is stamped, but only at directory granularity.**
  `PROVENANCE.txt` now says which step wrote which directory and from which
  pack. An individual file still does not carry its own origin, which is fine
  while every step owns a directory and would stop being fine if one ever
  wrote into another's.
