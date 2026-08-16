# Asset Licensing

An audit of the art and audio this project draws on, what each licence actually
permits, and where the paperwork is missing. Companion to
[ASSET-INVENTORY.md](ASSET-INVENTORY.md), which catalogues *what* is in the
bundle; this file covers *what may be done with it*.

Not legal advice. This is a reading of the licence texts found on disk plus the
creators' published storefront terms, current as of 2026-08-15.

---

## The short version

The repository as it stands is clean. No art, audio, or font file is tracked in
git; `assets-raw/`, `shots/`, `saves/`, and the built binary are all ignored,
and `assets/manifest.json` contains only file *paths*. The MIT `LICENSE` already
carves art and audio out of its grant. The only bundle-derived pixels in version
control are the six PNGs in `docs/screenshots/`, and every licence family below
explicitly permits screenshots.

The runtime-load design — art resolved from the player's own copy of the bundle,
procedural placeholders when it is absent — is not merely convenient. It is the
only distribution model most of these licences allow. Treat it as permanent.

Two documents are missing and should be obtained (see [Gaps](#gaps-to-close)):
the Humble bundle EULA, and the AfGameAssets licence covering three of the six
packs the game currently loads.

---

## What the game actually loads

`assets/manifest.json` resolves 384 keys across six packs. Everything else in
`assets-raw/` is extracted but unreferenced.

| pack | keys | creator | licence tier |
|---|--:|---|---|
| `mobsavataricons_windows` | 133 | REXARD | [B — storefront standard](#tier-b--no-file-on-disk-storefront-terms-govern) |
| `monstersavataricons_windows` | 107 | REXARD | B |
| `characteravataricons_windows` | 76 | REXARD | B |
| `pixelartrpgtopdowncharacters` | 28 | AfGameAssets / Pixogen | [C — licence not in hand](#tier-c--licence-exists-but-was-not-shipped) |
| `pixelartrpgtopdownenemies` | 23 | AfGameAssets / Pixogen | C |
| `pixelartrpgnpc` | 17 | AfGameAssets / Pixogen | C |

So ~82% of the art in the current build is REXARD (permissive) and ~18% is
AfGameAssets (terms unverified). No Beowulf pack — the most restrictive family —
is wired in yet. No audio is used at all.

---

## Licence tiers across the extracted packs

### Tier A — Explicit licence text on disk

Three packs ship `license_agreement.rtf`, an itch.io Asset Licence Agreement by
Ricardo Machado ("Beowulf"): `beowulfsrpgmonsterloots`,
`magicrunespixelartassetpack`, `minianimalsassetpack`.

Six further packs in `assets-raw/` are by the same creator and ship only a
"thanks for purchasing" readme. Assume the same terms govern them:
`beowulfsrpgdungeontilesets`, `miniadventureheroeselves`,
`miniadventureheroeshumans`, `minicityassetpack`, `minicityinteriorstilesets`,
`pixelrpgdungeonsmonsters`.

This is the most restrictive family in the collection. The terms that matter:

- **One Media Product per purchase** (3.1). A sequel is a separate product and
  needs a separate licence (3.3C).
- **Bundle clause** (3.3B): because these came in a bundle, assets may be spread
  across multiple products *provided no single asset appears in more than one*.
  That is a bookkeeping obligation if Slycrel ever has a sibling project.
- **Derivative works belong to the artist** (5). Edits are permitted, but the
  purchaser assigns all IP in those edits back to Machado. Recolouring or
  re-tiling his art does not make it yours.
- **Credit is contractual**, not courtesy (3.1D) — the artist's name under
  "additional art assets" or similar.
- **Notify the artist on release** (3.1E), by email.
- **No redistribution, sublicensing, or transfer** outside the product (3.2C).
- **No letting players extract the assets** (3.2C).
- **No use in a logo, trademark, or service mark** (3.2B).
- **Digital only** — no print, no merchandise.
- **Screenshots and trailers are explicitly allowed**, and publicly showing them
  ties the licence to the project shown (3.1A/B).
- **Not usable via an engine or dev tool** you distribute to others (3.2C).

### Tier B — No file on disk, storefront terms govern

The bulk of the collection, including all three REXARD avatar packs the game
loads today. No licence file was included in the zips — the operative grant is
the Humble bundle EULA plus the originating storefront's standard licence.

The REXARD icon sets (`characteravataricons_windows`, `mobsavataricons_windows`,
`monstersavataricons_windows`) sell through GameDev Market and the Unity Asset
Store. GameDev Market's Pro Licence permits commercial and non-commercial use in
an unlimited number of projects, permits derivative works, and forbids
redistributing the assets as assets. That is a comfortable position, and it
covers the majority of the current build.

Same tier, not currently loaded: `2dbuildingstown`, `2dminimalskillicons`,
`guipro_fantasyrpg_gamedevmarket`, `rpgandmmoui4` (EvilSystem),
`spellsandabilityicons_windows`, `dialogueboxes_windows`,
`pixelartmedievalinteriors_windows`, `pixelartmedievalinteriors2` (Acasas),
`pixelartfarmlifeset`, `pixelartgreentemple`, `fantasynordicgui`,
`pixelartrpgvfx`.

### Tier C — Licence exists, but was not shipped

The AfGameAssets packs — `pixelartrpgtopdowncharacters`,
`pixelartrpgtopdownenemies`, `pixelartrpgnpc`, `pixelartrpgvfx` — are sold on
itch.io (now under the Pixogen name) with a downloadable "License of
AFGameAssets" file. **That file is not in the Humble zips**, and therefore not on
disk. Three of these four packs are in the shipping manifest.

This is the one live unknown in the current build.

### Tier D — Well-documented public licence

`manaseedpixelarttilesetcollection` (Seliel the Shaper). Published at
selieltheshaper.weebly.com/user-license.html. Commercial use permitted,
unlimited projects, edits permitted, non-transferable without written
permission. Two carve-outs worth remembering: **the art may not go into an
engine or toolkit others build games with**, and **it may not be used in
blockchain or AI projects**.

### Tier E — Audio

Five packs by The Sound Guild: `ambiencesoundspack`,
`combatsoundsbundlecollection`, `monstersoundsvolume1`, `oldmagicianvoicepack`,
`userinterfacesfxbundle`. The only document included is a newsletter flyer whose
sole licensing content is *"if you can credit me in your project it would be
amazing"*. No terms text on disk. Nothing is used yet; resolve terms before
wiring audio in.

### Tier F — Fonts

37 TTFs across `dialogueboxes_windows` and `fantasynordicgui`: Vollkorn, Exo,
Anton, Archivo Black, Baloo Thambi, Berkshire Swash, Varela Round. All are
Google Fonts, all under SIL Open Font License 1.1. `guipro_fantasyrpg` similarly
points at Alata.

These are the only assets in the entire collection that may legitimately be
redistributed — but OFL requires shipping the licence text alongside, and none
of the packs include an `OFL.txt`. If a font is ever bundled, take it from
Google Fonts with its `LICENSE` file rather than from the asset pack.

Currently moot: the game renders with `basicfont.Face7x13` from
`golang.org/x/image` (BSD-3-Clause), so no font is bundled at all.

---

## Can, can't, must

### Can

- Ship Slycrel, free or commercially, with art resolved at runtime from the
  player's own copy of the bundle. This is the current design.
- Keep the repository MIT-licensed and public.
- Publish screenshots, GIFs, and trailers, including on store pages.
- Modify, recolour, re-tile, and re-cut the art for use in the game.
- Use the REXARD and Mana Seed art across as many projects as desired.

### Can't

- **Commit art, audio, or fonts to git.** Not a sample, not a thumbnail, not a
  "just one sprite to make the README nicer."
- **`go:embed` any asset**, or otherwise bake art into a release binary handed
  to people who do not own the bundle. That is redistribution.
- Publish a release archive, installer, or itch build containing the art.
- Use the art in a logo, app icon, store banner, or box art.
- Ship a modding SDK, asset pack, or engine that exposes the art to others.
- Print anything (Tier A explicitly; assume the same elsewhere).
- Claim ownership of edits to Tier A art — clause 5 assigns them to the artist.
- Use Mana Seed art in an engine product, a blockchain project, or an AI one.

### Must

- **Credit the creators.** Contractual for Tier A, requested for Tier E,
  conventional everywhere else. There is no `CREDITS.md` yet.
- **Email Machado on release** if any Beowulf-family art ships (clause 3.1E).
- **Track one-asset-one-product** if Tier A art is ever spread across projects.

---

## Gaps to close

1. **Obtain the Humble bundle EULA.** It is the master grant for all 78 packs
   and no copy exists locally. Community reporting indicates Humble's asset
   bundle EULA was originally single-product and was later revised to permit
   multiple products — which revision applies to this purchase matters, and only
   the document settles it. Download it from the Humble library page and store
   it beside the bundle.
2. **Obtain "License of AFGameAssets"** from the itch.io product pages for the
   three packs currently in the manifest. This is the only unverified licence
   affecting the build as it stands.
3. **Resolve The Sound Guild terms** before any audio is wired in.
4. **Add `docs/CREDITS.md`** and surface it in-game. Tier A makes this a term of
   the licence, not a nicety.
5. **Keep `assetpipe`'s original pack folder names.** They are the provenance
   trail that made this audit possible; a flattened or renamed asset tree would
   destroy the link between a file and its licence.
