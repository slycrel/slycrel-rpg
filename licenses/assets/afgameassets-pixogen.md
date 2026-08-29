# AfGameAssets / Pixogen

Covers the three packs in the shipping manifest —
`pixelartrpgtopdowncharacters`, `pixelartrpgtopdownenemies`, `pixelartrpgnpc` —
and `pixelartrpgvfx`, which is extracted but unreferenced. These supply the
hero, foe and NPC sprites: 92 of the art keys the game loads.

Not legal advice. A record of what was checked, when, and what it showed.
Investigated 2026-08-29.

## What was established

**The creator renamed.** AfGameAssets is now Pixogen.
`afgameassets.itch.io` 302-redirects to `pixogenassets.itch.io`, and the Unity
Asset Store lists the publisher as Pixogen. The packs on disk still carry the
old name in their folder names, which is why the two have to be recorded as one
party.

**These are Unity Asset Store packs.** This is the useful finding and it came
off local evidence rather than the web. `pixelartrpgvfx` ships a `ReadMe.txt`
— the only text file in any of the four — and it reads:

> Thank you for purchasing!
> If you have any problems, please don't hesitate to mail me
> (afgameassets@gmail.com).
>
> Please rate this product on the Unity Asset Store if you like it!

followed by import instructions that are Unity's and nobody else's: set the
texture type to "sprite (2D and UI)", set Sprite mode to "multiple", drag the
prefabs into a canvas. Whatever storefront the bundle came through, the packs
themselves were built for and sold on the Unity Asset Store.

**The Unity listing.** *Pixel Art Top-Down RPG Characters*, publisher Pixogen,
sold under the **Standard Unity Asset Store EULA** as an **Extension Asset**
with a **Single Entity** licence. Same for Enemies and NPC.
<https://assetstore.unity.com/packages/2d/characters/pixel-art-top-down-rpg-characters-186264>

**What that EULA permits**, per Unity's own EULA FAQ
(<https://assetstore.unity.com/browse/eula-faq>):

- Commercial use, as long as the asset is embedded and integrated into a game
  or other digital product.
- Distribution inside a finished product that "contains a substantial amount of
  original creative work" and has "purpose, features, and function beyond the
  distribution of assets".
- **Not** a product "designed to allow your end users to extract or download
  assets separately".
- Modification of an Extension Asset needs the publisher's consent. Slycrel does
  not modify these sprites — they are loaded as shipped, by manifest key — so
  this does not bite, and it is a reason not to start.
- Attribution is not required. It is given anyway, in `CREDITS.md`.

That is the same shape as every other licence family in this collection, and it
permits exactly what this project does with them.

## What could not be established, and why

The itch.io product pages list a downloadable file called **"License of
AFGameAssets"** (18 kB). It is gated behind ownership on itch.io and its text is
not published anywhere. It was not obtained.

It also probably does not govern this build, for the reason
`docs/ASSET-LICENSING.md` already gives about the Beowulf packs' itch.io RTF: a
storefront's licence document is the grant that came with a purchase *from that
storefront*. These packs arrived in a Humble bundle, not from itch.io, and not
from the Unity Asset Store either. So the operative grant is the Humble bundle's
— which is the one document nobody has yet produced, and which is
[gap 1](../../docs/ASSET-LICENSING.md), not this one.

The Unity EULA above is therefore evidence of the creator's terms as they
express them through their own primary storefront, and not a licence anyone
holds. It is recorded because it is the best public statement of what the
creator permits, and it happens to permit this.

## What would close it

Only the account holder can do either.

1. Open the bundle in the Humble **library** and look for a licence entry
   alongside the asset zips.
2. Mail afgameassets@gmail.com — the address the creator puts in their own
   readme — and ask which licence covers a Humble bundle copy. Save the reply
   next to this file. A dated answer from the creator outranks every inference
   in this document.
