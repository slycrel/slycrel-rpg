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

**897 keys across 13 namespaces.**

| namespace | keys | sources |
|---|--:|---|
| `icon/` | 395 | `_generated/icons` 263, `_generated/bands` 132 |
| `mob/` | 168 | `mobsavataricons_windows` 133, `sci-ficharactersicons` 35 |
| `boss/` | 107 | `monstersavataricons_windows` 107 |
| `portrait/` | 76 | `characteravataricons_windows` 76 |
| `hero/` | 28 | `pixelartrpgtopdowncharacters` 28 |
| `ground/` | 25 | `manaseedpixelarttilesetcollection` 25 |
| `foe/` | 23 | `pixelartrpgtopdownenemies` 23 |
| `odd/` | 22 | `pixelartcyberpunkcity` 22 |
| `npc/` | 17 | `pixelartrpgnpc` 17 |
| `vfx/` | 17 | `pixelartrpgvfx` 17 |
| `poi/` | 9 | `_generated/icons` 9 |
| `prop/` | 6 | `manaseedpixelarttilesetcollection` 5, `_generated/props` 1 |
| `weather/` | 4 | `manaseedpixelarttilesetcollection` 4 |

<!-- END generated -->

## How art gets here

Nothing is generated at runtime. Every derived pixel is written once by
`cmd/assetpipe` into `assets-raw/_generated/`, which is gitignored like the rest
of the extraction tree, and `scripts/dist.sh` copies whatever the manifest names
— so a new pipeline step ships in release builds without touching the packaging.

```
assetpipe extract <pack>   unzip from the bundle into assets-raw/  (read-only source)
assetpipe icons            box-reduce every icon set to 16px       -> _generated/icons/
assetpipe garb             cut garment cells from a paper-doll sheet -> _generated/icons/garb/
assetpipe arms             cut weapon cells from the same sheet     -> _generated/icons/arms/
assetpipe poi              cut overworld location markers          -> _generated/icons/poi/
assetpipe props            translucent shadows on Mana Seed props  -> _generated/props/
assetpipe bands            tier recolours of the gear icons        -> _generated/bands/
assetpipe manifest         rebuild assets/manifest.json from all of the above
assetpipe map              rebuild the table in this file
```

Order matters in one place: `bands` reads the output of `icons`, `garb` and
`arms`, and reads `armor.json` and `weapons.json` to learn *which* pictures to
band. Run it after all three.

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
| `pixelartrogue-likerpg` | two 512x512 sheets at 16px | map icons are cut and wired. Still unused: a full overworld terrain set in six biome colourways and ~17 creatures x 6 poses. |
| `pixelartdungeonlevel4` | 58 files, 64x64 | a Golem with 24 frames — front/back/side x walk and attack — which is exactly the `foe/` convention. Plus dungeon floors, doors, chests, a 5-frame torch loop. |
| `pixelartwasteland` | 51 files, 64x64 | cars, stop signs, a sofa, barrels: oddity furniture in the same register as the cyberpunk city pack already wired to `odd/`. |
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
| armour | 19 | 19 | none |
| weapons | 27 | 27 | none |
| shields | 19 | 19 | none |
| charms | 12 | 10 | 2 |
| spells | 35 | 32 | 3 |

Charms and spells are what is left, and both are small. Neither is banded:
shields and charms already have one picture each with nothing shared in the
shield table, and banding a set to fix two collisions would generate art nothing
asks for.

Thin namespaces: `prop/` at 6 and `weather/` at 4 are the smallest things in the
manifest by a wide margin, and `vfx/` at 17 is not much better.

## What is not settled

This pass is not the last one, and these are the questions it left open.

- **Lane is not encoded in armour art.** Cloth reads as a dress and everything
  else as a jerkin, so light and heavy are told apart only by which jerkin and
  which tier. `model.CanWear` gates who may wear what, and "never offer a choice
  you are about to refuse" argues the picture should say so. The alternative is
  banding on two axes, which doubles the output.
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
- **None of this has been seen in an armourer's shop.** The `-demo` tour lands
  on a Blacksmith, and the character sheet lists armour as text with no icon.
  The icons were verified at pixel level and by `-audit`, and the menu fits a
  16x16 source into a 16px box, so the fit is identity — but a real frame of the
  armourer would be better evidence, and wants a save fixture rather than a
  patched tour.
- **Polearms have no shape, and the two in the game are borrowing one.** There
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
- **`_generated` has no provenance record.** A file under it does not say which
  step wrote it or from what. The table above infers it from the directory,
  which works only as long as each step owns a directory.
