package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
)

// Garb is armour art, cut out of the crafting pack's paper-doll sheet.
//
// It exists because armour had no pictures of its own. Weapons have had them
// all along — the ability set ships numbered tiers of sword, axe and hammer —
// but there is not one coat, robe or helmet anywhere in it, so the armour table
// borrowed from the monster-loot set and dressed nineteen pieces in six
// pictures of offal and pelts. The whole cloth lane wore a tuft of fur.
//
// The loot set was searched before this was written, and it genuinely has no
// answer: eighty-six icons of organs, gems, berries and potions, of which one
// (monster_scales) reads as armour. Cutting real garments out of a sheet the
// bundle already contains beats both recolouring a liver and drawing new art.
//
// Licence: the pack ships no licence file, which puts it in Tier B of
// ASSET-LICENSING.md — storefront terms, the permissive end of the collection,
// where re-cutting and recolouring for use in the game are both explicit. The
// cuts land under the gitignored `_generated` tree like everything else derived
// from the bundle.

// garbSheet is the paper-doll sheet, an 8x8 grid of 64px cells. Columns 0-4 of
// a row are the same garment in five colourways; columns 5-7 are weapons, which
// this pass ignores because the ability set already covers them better.
var garbSheet = []string{
	"pixelartminingcrafting", "Pixel Art Top Down Crafting Mining",
	"PixelArtMiningCrafting.png",
}

// garbCell is the sheet's grid pitch.
const garbCell = 64

// garbRows names the two rows worth cutting. Row 2 is torsos — jerkins and
// doublets, which carry a strap, a lacing or a clasp and so stay distinguishable
// from one another after a band has flattened their colours. Row 3 is dresses,
// which is what the cloth lane wants.
//
// The other six rows are hair, gloves, boots and trousers. This game equips a
// coat and nothing else, so they would be pictures of a slot that does not
// exist.
var garbRows = []struct {
	row  int
	name string
}{
	{2, "jerkin"},
	{3, "robe"},
}

// buildGarb cuts the garment cells to 16px icons.
//
// The art is 14x17 to 11x22 inside its cell — taller than the 16px box a menu
// row fits an icon into — so something has to give. Cropping to the top sixteen
// rows beats scaling to fit, and it was decided by looking at both: a torso
// loses one row of hem, which is invisible, and a dress loses the bottom of its
// skirt, which still reads as a dress. Scaling to fit resamples every garment
// off the pixel grid and narrows the dresses to a smear.
func buildGarb() error {
	sheetPath := filepath.Join(append([]string{rawRoot}, garbSheet...)...)
	sheet, err := readPNG(sheetPath)
	if err != nil {
		return fmt.Errorf("%w (run `assetpipe extract pixelartminingcrafting` first)", err)
	}

	outDir := filepath.Join(genRoot, "icons", "garb")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	total := 0
	for _, r := range garbRows {
		for col := 0; col < 5; col++ {
			cell := image.Rect(col*garbCell, r.row*garbCell,
				(col+1)*garbCell, (r.row+1)*garbCell).Add(sheet.Bounds().Min)
			art := trimAlpha(sheet, cell)
			if art.Empty() {
				continue
			}
			name := fmt.Sprintf("%s%d", r.name, col+1)
			out := filepath.Join(outDir, name+".png")
			if err := writePNG(out, cropTo(sheet, art, IconPx)); err != nil {
				return err
			}
			total++
		}
	}
	if total == 0 {
		return fmt.Errorf("no garments found on %s", sheetPath)
	}
	fmt.Printf("ok     garb   %3d garments -> %dpx\n", total, IconPx)
	return nil
}

// trimAlpha shrinks a rectangle to the opaque pixels inside it, which is how a
// 64px cell becomes the garment sitting in the middle of it.
func trimAlpha(src image.Image, r image.Rectangle) image.Rectangle {
	out := image.Rectangle{Min: r.Max, Max: r.Min}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if _, _, _, a := src.At(x, y).RGBA(); a == 0 {
				continue
			}
			if x < out.Min.X {
				out.Min.X = x
			}
			if y < out.Min.Y {
				out.Min.Y = y
			}
			if x >= out.Max.X {
				out.Max.X = x + 1
			}
			if y >= out.Max.Y {
				out.Max.Y = y + 1
			}
		}
	}
	if out.Empty() {
		return image.Rectangle{}
	}
	return out
}

// cropTo copies art into a size x size box, centred horizontally and anchored at
// the top. Anything past the bottom edge is cut rather than squeezed, so every
// pixel drawn is a pixel the artist placed.
func cropTo(src image.Image, art image.Rectangle, size int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))

	w, h := art.Dx(), art.Dy()
	if w > size {
		// Nothing on this sheet is wider than the box; if a future row is,
		// centre the crop rather than favouring one shoulder.
		art.Min.X += (w - size) / 2
		w = size
	}
	if h > size {
		h = size
	}
	dst := image.Rect((size-w)/2, 0, (size-w)/2+w, h)
	draw.Draw(out, dst, src, art.Min, draw.Src)
	return out
}
