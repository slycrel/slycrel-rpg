package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
)

// Location markers for the overworld, cut from the rogue-like pack's two
// sheets.
//
// These overturn a decision that was correct when it was made. `drawPOIMarker`
// has always painted its own rectangles, with the reason written next to it:
// "the building art in the bundle is 300-500px hero sprites meant for a
// zoomed-in scene, and scaling one down to a 16px overworld cell is mush". That
// was true of every pack extracted at the time. `pixelartrogue-likerpg` was not
// one of them, and it holds twenty settlement silhouettes drawn natively at
// overworld scale — castles with crenellations, thatched huts, a stone tower, a
// crystal shrine, and a cave mouth. The procedural markers stay as the fallback,
// because a marker that fails to a coloured rectangle is better than one that
// fails to nothing.
//
// They are cut at native size rather than squeezed into a tile. Most are 20-28px
// against a 16px grid, and `render.Ctx.World` already anchors a sprite on its
// base and lets it stand taller than its square — which is how every character
// in the game is drawn, and how an overworld map has always drawn a town.
// Scaling them to 16px was tried and is exactly the mush the old comment
// predicted: the capital's keep loses its tower and the stone tower loses its
// crenellations.
//
// Licence: AFGameAssets / Pixogen, the same creator as the three packs already
// shipping, covered by the Tier C reading in ASSET-LICENSING.md.

// poiSheets are the two sheets, relative to the pack directory.
var (
	poiPack   = []string{"pixelartrogue-likerpg", "Pixel Art Dungeon Crawler - AFGameAssets - V2"}
	poiSheetB = "PixelArtDungeonCrawler2.png"
)

// poiCut is one marker: which sheet, and where on it.
//
// The rectangles were found by walking the sheets for opaque islands rather
// than by eye, and they are literal because the sheets are fixed files — there
// is no grid to derive them from. The icons sit at irregular offsets, several
// straddling the 16px cell lines.
type poiCut struct {
	name  string
	sheet string
	x, y  int
	w, h  int
}

var poiCuts = []poiCut{
	// Settlements, largest to smallest. A capital has to out-read a town at a
	// glance from across the map, so the sizes are doing work here.
	{"capital", poiSheetB, 386, 290, 28, 28}, // walled complex with a round keep
	{"town", poiSheetB, 483, 231, 26, 19},    // two spired halls joined
	{"village", poiSheetB, 420, 328, 22, 20}, // a cluster of thatch
	{"castle", poiSheetB, 420, 228, 24, 26},  // one tall multi-storey keep
	{"tower", poiSheetB, 491, 324, 10, 25},   // pale stone, narrow, unmistakable
	{"shrine", poiSheetB, 490, 266, 13, 18},  // a crystal on a stem
	{"camp", poiSheetB, 392, 204, 14, 14},    // a timber shelter, small and temporary
	{"ruin", poiSheetB, 453, 200, 21, 13},    // a low wall with nothing standing on it

	// No cave mouth. The sheet has one, and it was cut, wired and looked at:
	// on the sheet it is an opening set into a block of stone, so lifted out on
	// its own it is a flat black rectangle, and on grass it reads as a hole
	// punched in the screen rather than a hole in the world. Dungeons and caves
	// keep the procedural marker, which was drawn to sit on any terrain and has
	// the outline and contact shadow to do it.

	// The oddity gets the beacon tower: something is wrong here, said without
	// the marker being in on the joke.
	{"oddity", poiSheetB, 458, 322, 10, 28},
}

// buildPOI cuts the location markers.
func buildPOI() error {
	outDir := filepath.Join(genRoot, "icons", "poi")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	sheets := map[string]image.Image{}
	for _, c := range poiCuts {
		if sheets[c.sheet] == nil {
			p := filepath.Join(append(append([]string{rawRoot}, poiPack...), c.sheet)...)
			img, err := readPNG(p)
			if err != nil {
				return fmt.Errorf("%w (run `assetpipe extract pixelartrogue-likerpg` first)", err)
			}
			sheets[c.sheet] = img
		}
		src := sheets[c.sheet]
		r := image.Rect(c.x, c.y, c.x+c.w, c.y+c.h).Add(src.Bounds().Min)
		if !r.In(src.Bounds()) {
			return fmt.Errorf("%s: cut %v is outside the sheet %v", c.name, r, src.Bounds())
		}
		// Written at the exact size of the artwork, with no padding. A
		// Sprite's Foot counts the transparent rows below the art, so a marker
		// in a padded box would float that many pixels off its tile; at zero
		// padding the base of the building sits on the base of the square.
		out := filepath.Join(outDir, c.name+".png")
		dst := image.NewNRGBA(image.Rect(0, 0, c.w, c.h))
		draw.Draw(dst, dst.Bounds(), src, r.Min, draw.Src)
		if err := writePNG(out, dst); err != nil {
			return err
		}
	}
	fmt.Printf("ok     poi    %3d markers\n", len(poiCuts))
	return nil
}
