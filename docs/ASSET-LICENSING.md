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

The runtime-load design exists because **the repository is public**, not because
shipping the game is restricted. Distributing Slycrel with the art baked in, to
anyone, free or paid, is exactly what these licences are for. The two models
coexist: an art-free public repo for the code, and release builds that contain
the art. See [Can, can't, must](#can-cant-must).

There is **no single "Humble EULA"** to track down — see
[The governing licence](#the-governing-licence). Humble is the storefront, not
the licensor. The evidence points at GameDev Market's Pro Licence as the stock
document for this bundle, which is permissive: commercial use, unlimited
projects, derivative works, no credit required, no redistribution.

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

## The governing licence

Humble does not publish a universal asset EULA, and there is no stock document
sitting somewhere unfound. Humble runs asset bundles in partnership with a
marketplace, and **that marketplace's standard licence is the operative grant**.

This bundle is a GameDev Market bundle. The fingerprints are unambiguous:

- `pixelrpgdungeonsmonsters/README_gdm.txt` — "gdm", and the text directs the
  reader to "gamedevmarket asset page"
- `beowulfsrpgdungeontilesets/readme.txt` — "check the gamedevmarket asset page"
- Two packs carry it in the folder name: `guipro_fantasyrpg_gamedevmarket`,
  `robots_gamedevmarket`
- REXARD and Seliel the Shaper both distribute through GameDev Market

### GameDev Market Pro Licence

The terms, which are considerably more generous than anything found on disk:

- Non-exclusive, **perpetual** licence
- Create Derivative Works from the assets
- Use in **both** Monetized and Non-Monetized Media Products
- **No restriction on the number of projects**
- Distribute and sell the product for any fee you set
- **No attribution required**
- May **not** sell, share, transfer, sublicense, or redistribute the asset or a
  derivative other than as part of the media product
- May **not** let end users extract the assets from the product

### The Humble amendment

When an earlier Humble/GameDev Market RPG bundle shipped carrying the
single-project clause, the resulting complaints produced a public correction:
*"we are in the process of removing this clause, so for the purpose of any
Humble Bundle purchases, all assets can be used in multiple projects."* Bundle
purchases are not single-product licences.

### What this means for the Tier A licence file

The `license_agreement.rtf` in three Beowulf packs is headed **"itch.io Asset
Licence Agreement"**, and clause 1.2 states it takes effect "on the date of
purchase from Itchi.io — please refer to your Itch.io invoice." This bundle was
not an itch.io purchase. That RTF is a stale file the creator packages into his
zips for his itch customers; it is not the grant that came with this purchase.

Its harshest terms therefore probably do **not** bind here — the one-product
limit, the assignment of derivative works back to the artist, and the duty to
email on release. Treat the RTF as informational about the creator's
preferences rather than as the contract.

This is an inference from strong circumstantial evidence, not a settled fact.
Two things would resolve it (see [Gaps](#gaps-to-close)).

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

This is the most restrictive text in the collection — but see
[the note above](#what-this-means-for-the-tier-a-licence-file): it is an
itch.io agreement that most likely does not govern a bundle purchase. Recorded
here as the worst case, not the presumed case. The terms that matter:

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

**Narrowed, 2026-08-29.** Written up in full at
[`licenses/assets/afgameassets-pixogen.md`](../licenses/assets/afgameassets-pixogen.md),
which ships in every build. The short version:

`pixelartrpgvfx` turns out to carry a `ReadMe.txt` — the only text file in any of
the four, and missed by the original sweep. It says "Please rate this product on
the Unity Asset Store if you like it" and gives import instructions that are
Unity's and nobody else's. These are **Unity Asset Store packs**, sold under the
**Standard Unity Asset Store EULA** as Extension Assets on a Single Entity
licence, publisher Pixogen. Unity's own EULA FAQ permits commercial use embedded
in a product, forbids letting end users extract the assets, and requires no
attribution — which is the same shape as every other family here and permits
exactly what this project does.

That is evidence of the creator's terms, not a grant anybody holds: these
arrived in a Humble bundle, not from Unity and not from itch.io, so the
operative document is still the bundle's. The itch.io licence file remains
purchase-gated and unobtained, and by the same reasoning applied to the Beowulf
RTF above, it would not have governed this copy anyway.

So this is no longer the one live unknown. It has collapsed into
[gap 1](#gaps-to-close) — the Humble bundle's own grant — along with everything
else.

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
amazing"*.

**This tier is now live.** `assets/audio.json` wires 33 cues drawn from 81 files
across all five packs — 32 from the UI SFX bundle, 26 combat, 11 magician VO, 8
monster, 4 ambience. They ship in any build produced by `scripts/dist.sh`.

Nothing about that is likely to be a problem: these arrived through the same
bundle as everything else, and using them in a game is the purchase's whole
purpose. But it is the one creator in the collection for whom **no terms text
exists anywhere on disk**, so it is also the least documented thing being
shipped. The Sound Guild is credited in `CREDITS.md`, which satisfies the only
request the flyer actually makes.

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

The distinction that governs everything here is **the art as art** versus **the
art inside a game**. Every licence in this collection forbids the first and
exists to permit the second. Distributing Slycrel is the licensed use case, not
an edge case around it.

### Can

- **Ship Slycrel to anybody, with the art baked in.** Give a build to a friend,
  put it on itch, sell it on Steam. Players need no bundle of their own and no
  licence of their own. This is what the purchase bought.
- **`go:embed` the art**, pack it into an archive, or ship an assets folder next
  to the binary. All fine — these are packaging choices, not licensing ones.
- Charge money for it, at any price.
- Keep the repository MIT-licensed and public, with the art left out of it.
- Publish screenshots, GIFs, and trailers, including on store pages.
- Modify, recolour, re-tile, and re-cut the art for use in the game.
- Use the art across as many projects as desired (Pro Licence; see the Tier A
  caveat if that reading turns out to be wrong).

### Can't

- **Commit art, audio, or fonts to git in a public repo.** Not a sample, not a
  thumbnail, not a "just one sprite to make the README nicer." A public repo
  hands over the assets as assets to anyone who clones it — no game involved.
  This, and not distribution, is the real reason the runtime-load design exists.
- Publish the art *as art*: an asset pack, a spritesheet download, a modding
  SDK, an engine or toolkit others build games with.
- Sell, share, transfer, or sublicense the assets outside a game of yours.
- Use the art in a logo, trademark, app icon, store banner, or box art.
- Print anything (Tier A explicitly; assume the same elsewhere).
- Use Mana Seed art in an engine product, a blockchain project, or an AI one.
- Under the worst-case Tier A reading only: claim ownership of your edits.

### On "don't let users extract the assets"

This clause appears in the Pro Licence and in Tier A, and it is narrower than it
sounds. It forbids *authorizing or facilitating* extraction — shipping a
documented asset directory, an unpacker, an export feature. It does not require
that extraction be technically impossible. Every 2D game ever shipped can have
its sprites ripped by a determined player with a hex editor; no licence demands
otherwise.

Practically: prefer `go:embed` or a packed archive over a browsable
`assets-raw/`-style tree with original pack names and folder structure intact.
That is a sensible nudge, not a hard requirement.

### Must

- **Credit the creators.** Strictly, the Pro Licence does not require it, and if
  that licence governs then nothing here is mandatory. But Tier A demands it,
  Tier E asks for it, and the cost of a `CREDITS.md` is nil against the cost of
  being wrong about which licence applies. Write it. There isn't one yet.
- Under the worst-case Tier A reading only: email Machado on release, and track
  one-asset-one-product across projects. Both fall away if the Pro Licence
  governs, which is the likelier reading.

---

## On hosting the assets for the game to download

Considered and rejected. The reasoning is worth recording, because the idea
recurs and because the licensing analysis is not the obvious one.

### It is not the grey area it looks like

Downloading assets on first run is ordinary practice — Unity Addressables, UE
pak chunks, mobile on-demand resources — and it is *not* a licensing problem
provided the delivery is genuinely part of the game: the payload serves the
game, arrives through the game, and is not a browsable library.

The failure mode is the bucket, not the download. A public URL serving PNGs is a
distribution channel for **the art as art**, and anyone with `curl` is a
recipient who never ran the game. That is squarely what the licences forbid, and
it is worse than committing to git, because it would be infrastructure operated
deliberately rather than a mistake in `.gitignore`.

### Encryption does not change the analysis

An embedded key is a speed bump, not a legal instrument. The key ships inside
the binary, so it is extractable by anyone who cares — but that is the lesser
point. The real point is that the licence question is *who receives the assets*,
not *how hard they were to read*. Encryption changes the difficulty and leaves
the recipients identical, so it moves nothing that matters.

Where obfuscation does have value is the narrow "don't facilitate extraction"
clause: a packed, non-obvious format is better than a browsable tree of original
pack names. That is a reason to pack, not a reason to build a key exchange.

### The decisive argument is that it solves nothing

Embedding the art in the build is already permitted. So the CDN buys no rights
that `go:embed` does not, while adding:

- hosting cost and an availability dependency — the game breaks when the bucket
  moves, expires, or gets a bill
- a first-run failure mode on a game that currently has none
- asset/binary version skew to manage
- a new and serious licensing risk surface, in exchange for none removed

A self-contained build has none of these. **Embed the art in release builds and
keep the repository art-free.** That is the whole solution.

### The one case that genuinely cannot be solved this way

If the goal were to let *contributors* clone the repo and get working art, no
hosting scheme fixes it. Developers receiving assets outside a game are
receiving assets as assets, whatever the transport — that is the prohibited case
by definition, and a licence-checking download gate would not cure it.

The existing answer is the right one and should stay: procedural placeholders so
the build is never broken, plus `assetpipe` for contributors who own the bundle.

---

## Gaps to close

1. **Confirm the bundle licence in writing.** There is no stock Humble EULA to
   download; the working conclusion is that GameDev Market's Pro Licence
   governs. Two ways to settle it:
   - Check the bundle's page in the Humble **library** (not the store page).
     GameDev Market bundles sometimes include a licence PDF as its own download
     entry, separate from the asset zips.
   - Failing that, email Humble support with the order number and ask which
     licence applies. This is a routine question and they answer it. Save the
     reply next to the bundle — a dated email from the storefront is better
     evidence than any inference in this document.

   The `.webarchive` saved beside the bundle is **not** evidence. It captured
   only Humble's page chrome; it contains zero occurrences of "asset", "RPG
   Creator", or any creator name.
2. ~~**Obtain "License of AFGameAssets"**~~ *Closed as far as it can be from
   outside the account — see [Tier C](#tier-c--licence-exists-but-was-not-shipped)
   and `licenses/assets/afgameassets-pixogen.md`.* The file is purchase-gated on
   itch.io and would not govern a Humble copy in any case. What remains is
   either gap 1 above, or a one-line mail to afgameassets@gmail.com asking which
   licence covers a bundle copy — the address is in the creator's own readme,
   and a dated reply outranks every inference in this document.
3. **Resolve The Sound Guild terms** before any audio is wired in.
4. **Surface `CREDITS.md` in-game.** The file now exists and ships in every
   build, but a credits screen would be better. Attribution for roughly nine
   packs is still incomplete — they shipped with no creator information, and the
   names need recovering from the GameDev Market listings before any release
   that uses them.
5. **Consider opaque asset paths in `dist/`.** Builds currently reproduce the
   `assets-raw/<packname>/...` tree verbatim, which is a browsable directory of
   original pack names — precisely what the "don't facilitate extraction" note
   above advises against. It is a nudge rather than a breach, and the fix is to
   have `scripts/dist.sh` copy to flattened names and rewrite the manifest it
   emits. Deliberately not done yet: it trades a verified-working build for
   marginal hygiene.
5. **Keep `assetpipe`'s original pack folder names.** They are the provenance
   trail that made this audit possible; a flattened or renamed asset tree would
   destroy the link between a file and its licence.
