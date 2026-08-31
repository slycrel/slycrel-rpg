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

**804 keys across 12 namespaces.**

| namespace | keys | sources |
|---|--:|---|
| `icon/` | 311 | `_generated/icons` 239, `_generated/bands` 72 |
| `mob/` | 168 | `mobsavataricons_windows` 133, `sci-ficharactersicons` 35 |
| `boss/` | 107 | `monstersavataricons_windows` 107 |
| `portrait/` | 76 | `characteravataricons_windows` 76 |
| `hero/` | 28 | `pixelartrpgtopdowncharacters` 28 |
| `ground/` | 25 | `manaseedpixelarttilesetcollection` 25 |
| `foe/` | 23 | `pixelartrpgtopdownenemies` 23 |
| `odd/` | 22 | `pixelartcyberpunkcity` 22 |
| `npc/` | 17 | `pixelartrpgnpc` 17 |
| `vfx/` | 17 | `pixelartrpgvfx` 17 |
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
assetpipe props            translucent shadows on Mana Seed props  -> _generated/props/
assetpipe bands            tier recolours of the gear icons        -> _generated/bands/
assetpipe manifest         rebuild assets/manifest.json from all of the above
assetpipe map              rebuild the table in this file
```

Order matters in one place: `bands` reads `icons` and `garb` output, and reads
`data/items/armor.json` to learn *which* pictures to band. Run it after both.

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

**Garments are cropped to 16px, never scaled.** The art is 14x17 to 11x22 inside
a 64px cell — taller than the box. Scaling to fit resamples every garment off
the pixel grid and narrows the dresses to a smear; cropping to the top sixteen
rows costs a torso one row of hem and a dress the bottom of its skirt, and both
still read. Decided by rendering the two side by side.

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
| `pixelartrogue-likerpg` | two 512x512 sheets at 16px | a full Ultima-style overworld terrain set in six biome colourways, ~17 creatures x 6 poses, and map icons for villages, towns, castles and towers. Same creator as three packs already shipping. |
| `pixelartdungeonlevel4` | 58 files, 64x64 | a Golem with 24 frames — front/back/side x walk and attack — which is exactly the `foe/` convention. Plus dungeon floors, doors, chests, a 5-frame torch loop. |
| `pixelartwasteland` | 51 files, 64x64 | cars, stop signs, a sofa, barrels: oddity furniture in the same register as the cyberpunk city pack already wired to `odd/`. |
| `pixelartminingcrafting` | 3 sheets | rows 2-3 cut for armour. **Columns 5-7 are weapons** — pickaxe, sword, hammer, axe, dagger, bow, staff, mace in three tiers — and are not cut yet. |
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
| shields | 19 | 19 | none |
| **weapons** | **27** | **17** | **7 icons shared; `4_weapon_sword` covers four weapons** |
| charms | 12 | 10 | 2 |
| spells | 35 | 32 | 3 |

Weapons are now the worst of these and for the same reason armour was: the
ability set has three weapon shapes (sword, axe, hammer) across four tiers, and
the game has five weapon kinds. **Daggers, bows and staves have no icon of their
own at all** — they borrow swords and lightning glyphs.

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
- **`_generated` has no provenance record.** A file under it does not say which
  step wrote it or from what. The table above infers it from the directory,
  which works only as long as each step owns a directory.
