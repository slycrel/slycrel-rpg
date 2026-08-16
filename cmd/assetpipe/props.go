package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// genRoot holds pipeline-derived art. It lives under the gitignored extraction
// directory on purpose: these are modified copies of purchased assets, and the
// repository does not redistribute those in any form.
var genRoot = filepath.Join(rawRoot, "_generated")

// shadowPalette is the two colours Mana Seed uses for baked drop shadows.
//
// The sheets have no partial alpha at all — every pixel is 0 or 255 — so a
// bush carries its shadow as solid purple. Composited over our ground that
// reads as a hard crescent rather than a shadow. Swapping those exact colours
// for translucent black is the whole fix.
//
// Matching on exact colour rather than "dark and blue" matters: the same sheets
// contain water at rgb(0,42,82) and rgb(36,111,166), which a looser rule would
// eat along with the shadows.
var shadowPalette = []color.RGBA{
	{49, 41, 90, 255},
	{74, 66, 132, 255},
}

// shadowInk is what a baked shadow becomes: cool, dark, and see-through enough
// that the ground texture still reads underneath it.
var shadowInk = color.RGBA{14, 10, 22, 105}

// propSheets are the sheets worth de-shadowing. The 16px sheets have no baked
// shadows and are read straight from the pack.
var propSheets = []string{
	filepath.Join("manaseedpixelarttilesetcollection", "20.04c - Summer Forest",
		"packaged", "summer sheets", "summer 32x32.png"),
}

// buildProps rewrites the prop sheets with real translucent shadows.
func buildProps() error {
	made := 0
	for _, rel := range propSheets {
		src := filepath.Join(rawRoot, rel)
		f, err := os.Open(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skipping %s: %v\n", filepath.Base(rel), err)
			continue
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("decoding %s: %w", filepath.Base(rel), err)
		}

		out, swapped := deshadow(img)
		dst := filepath.Join(genRoot, "props", filepath.Base(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		w, err := os.Create(dst)
		if err != nil {
			return err
		}
		err = png.Encode(w, out)
		w.Close()
		if err != nil {
			return fmt.Errorf("writing %s: %w", dst, err)
		}
		fmt.Printf("ok     %-28s %d shadow pixels softened\n", filepath.Base(rel), swapped)
		made++
	}
	if made == 0 {
		return fmt.Errorf("no prop sheets found; run `assetpipe extract tier1` first")
	}
	return nil
}

// deshadow replaces baked shadow colours with translucent ink, returning the
// new image and how many pixels changed.
func deshadow(src image.Image) (*image.NRGBA, int) {
	b := src.Bounds()
	out := image.NewNRGBA(b)
	swapped := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := src.At(x, y).RGBA()
			c := color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}
			if c.A == 255 && isShadow(c) {
				c = color.NRGBA(shadowInk)
				swapped++
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return out, swapped
}

func isShadow(c color.NRGBA) bool {
	for _, s := range shadowPalette {
		if c.R == s.R && c.G == s.G && c.B == s.B {
			return true
		}
	}
	return false
}
