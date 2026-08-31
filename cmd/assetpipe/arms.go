package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

// Arms are weapon icons, cut from the same paper-doll sheet the garments come
// from — columns 5-7 rather than 0-4.
//
// They replace the ability set, and that is a quality fix rather than a naming
// one. `spellsandabilityicons` draws a *spell slot*: a full-bleed square tile
// with a purple magical background and the weapon painted over it. Reduced to
// the 16px a menu row gives an icon, the background is most of what survives
// and every weapon in the shop is a purple smudge with a hint of grey in the
// corner. The crafting sheet's weapons are transparent cut-outs at native pixel
// scale, in the same visual language as the loot icons and the garments beside
// them.
//
// It also fixes what the ability set could not say. That set has three shapes —
// sword, axe, hammer — across four tiers, and this game has five weapon kinds,
// so daggers were drawn as throwing knives and every wand and staff in the game
// was drawn as a lightning bolt. Twenty-seven weapons shared seventeen icons and
// `4_weapon_sword` covered four of them.
//
// Licence: same pack and same Tier B as the garments; see ASSET-LICENSING.md.

// armsRows names each row of the sheet's weapon columns. Three columns is three
// variants of the shape rather than three tiers — the silhouettes genuinely
// differ, which is what makes them useful as separate pictures once `bands` has
// taken their colour away.
var armsRows = []string{
	"pick", "sword", "hammer", "axe", "dagger", "bow", "staff", "mace",
}

// armsCols are the sheet columns holding weapons.
var armsCols = []int{5, 6, 7}

// buildArms cuts the weapon cells to 16px icons.
func buildArms() error {
	sheetPath := filepath.Join(append([]string{rawRoot}, garbSheet...)...)
	sheet, err := readPNG(sheetPath)
	if err != nil {
		return fmt.Errorf("%w (run `assetpipe extract pixelartminingcrafting` first)", err)
	}

	outDir := filepath.Join(genRoot, "icons", "arms")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	total := 0
	for row, name := range armsRows {
		for variant, col := range armsCols {
			cell := image.Rect(col*garbCell, row*garbCell,
				(col+1)*garbCell, (row+1)*garbCell).Add(sheet.Bounds().Min)
			art := trimAlpha(sheet, cell)
			if art.Empty() {
				continue
			}
			out := filepath.Join(outDir, fmt.Sprintf("%s%d.png", name, variant+1))
			if err := writePNG(out, fitTo(sheet, art, IconPx)); err != nil {
				return err
			}
			total++
		}
	}
	if total == 0 {
		return fmt.Errorf("no weapons found on %s", sheetPath)
	}
	fmt.Printf("ok     arms   %3d weapons -> %dpx\n", total, IconPx)
	return nil
}

// fitTo places art in a size x size box, scaling it down only when it does not
// already fit.
//
// This is the opposite choice from the garments, and for the opposite reason. A
// coat is 14x17 — one row too tall — so cropping costs it a row of hem and
// keeps every remaining pixel where the artist put it. A staff is 25x27 and a
// bow 19x21, and cropping those to sixteen rows leaves a stub of haft and a
// piece of string: the shape *is* the length. Scaling a long thin diagonal is
// the lesser damage, and most of the set (pick, sword, hammer, axe, dagger)
// fits untouched anyway.
func fitTo(src image.Image, art image.Rectangle, size int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	w, h := art.Dx(), art.Dy()

	if w <= size && h <= size {
		return cropTo(src, art, size)
	}

	// Preserve aspect: the long side becomes the box, the short side follows.
	nw, nh := w, h
	if w*size > h*size {
		nw, nh = size, max(1, h*size/w)
	} else {
		nw, nh = max(1, w*size/h), size
	}
	dst := image.Rect((size-nw)/2, (size-nh)/2, (size-nw)/2+nw, (size-nh)/2+nh)

	// Nearest neighbour, not a smooth kernel. These are pixel art: an averaged
	// downscale turns a one-pixel-wide haft into a grey suggestion of a haft.
	xdraw.NearestNeighbor.Scale(out, dst, src, art, xdraw.Over, nil)
	return out
}
