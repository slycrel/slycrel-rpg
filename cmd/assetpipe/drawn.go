package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/pixelpal"
)

// Icons that were drawn rather than cut, rendered from grids under data/art/.
//
// One exists: a spear. Polearms are the only weapon kind with no picture
// anywhere in the bundle, and six sources plus a composite attempt failed to
// produce one — the search is written up in PLAN.md's Phase 5. So it was drafted
// with a local language model by `cmd/pixelsmith` and picked by eye.
//
// The committed artefact is the *grid*, not the PNG, and that is the whole
// design:
//
//   - `assetpipe build` must stay byte-reproducible, and a model is not
//     deterministic. Rendering a fixed grid is; running a model in the pipeline
//     would mean the manifest changed every time anybody built it.
//   - The grid holds no purchased pixels. It says which of six palette slots
//     each cell uses, and the palette is read back out of the extracted pack at
//     build time. So a public, art-free repository stays art-free: what is
//     committed is a 16x16 block of digits.
//   - It is reviewable in a diff. A pull request can show that a pixel moved.
//
// Licence note: the palette comes from `pixelartminingcrafting`, Tier B in
// ASSET-LICENSING.md, whose terms permit derivative works. Mana Seed art was
// deliberately kept out of this entirely — its licence forbids use in an AI
// project, and that carve-out is the reason the style seeds are drawn only from
// the crafting pack's weapons.
var drawnDir = filepath.Join("data", "art")

// drawnPalette defers to internal/pixelpal, which is also what pixelsmith shows
// the model. One rule, one order, in one place.
func drawnPalette() ([]color.NRGBA, error) {
	cs, err := pixelpal.Load(filepath.Join(genRoot, "icons", "arms"))
	if err != nil {
		return nil, err
	}
	out := make([]color.NRGBA, len(cs))
	for i, c := range cs {
		out[i] = c.RGBA
	}
	return out, nil
}

// buildDrawn renders every grid in data/art into the weapon icon set, so a
// drawn icon is banded and manifested exactly like a cut one.
func buildDrawn() error {
	entries, err := os.ReadDir(drawnDir)
	if os.IsNotExist(err) {
		fmt.Println("ok     drawn    0 grids (none authored yet)")
		return nil
	} else if err != nil {
		return err
	}

	pal, err := drawnPalette()
	if err != nil {
		return err
	}

	outDir := filepath.Join(genRoot, "icons", "arms")
	n := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(drawnDir, e.Name()))
		if err != nil {
			return err
		}
		img, err := renderGrid(string(raw), pal)
		if err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		name := strings.TrimSuffix(e.Name(), ".txt")
		if err := writePNG(filepath.Join(outDir, name+".png"), img); err != nil {
			return err
		}
		n++
	}
	fmt.Printf("ok     drawn  %3d grids -> %dpx\n", n, IconPx)
	return nil
}

// renderGrid turns the digit grid into an image. A digit outside the palette is
// an error rather than a skipped pixel: a grid that names a colour the weapon
// set does not contain is a grid drawn against the wrong reference, and drawing
// it anyway would put an icon on the shelf that quietly does not match.
func renderGrid(text string, pal []color.NRGBA) (*image.NRGBA, error) {
	img := image.NewNRGBA(image.Rect(0, 0, IconPx, IconPx))
	y := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if y >= IconPx {
			return nil, fmt.Errorf("more than %d rows", IconPx)
		}
		for x, r := range line {
			if x >= IconPx {
				return nil, fmt.Errorf("row %d is longer than %d", y+1, IconPx)
			}
			if r == '.' {
				continue
			}
			i := int(r - '1')
			if r < '1' || i >= len(pal) {
				return nil, fmt.Errorf("row %d: %q is not a palette slot (1..%d)", y+1, r, len(pal))
			}
			img.SetNRGBA(x, y, pal[i])
		}
		y++
	}
	if y == 0 {
		return nil, fmt.Errorf("empty grid")
	}
	return img, nil
}
