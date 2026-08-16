package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// IconPx is the size every icon is reduced to. Menus draw icons 1:1 at this
// size, so nothing is rescaled at runtime.
const IconPx = 16

// iconSources are the directories to reduce, and the key prefix each takes.
var iconSources = []struct {
	prefix string
	dir    []string // path parts under assets-raw
}{
	{"loot", []string{"beowulfsrpgmonsterloots", "Beowulf_RPG_Monsters_Loot", "monster_loots_size_32x32"}},
	{"rune", []string{"magicrunespixelartassetpack", "Beowulf's_Magic_Runes", "runes_size_x_32x32"}},
	{"ab", []string{"spellsandabilityicons_windows", "png", "128x128"}},
}

// buildIcons writes 16px copies of every icon.
//
// Doing this here rather than scaling at draw time is the difference between a
// readable icon and a smear. Ebitengine samples nearest-neighbour by default,
// so a 128px painted icon drawn into a 16px box keeps literally every eighth
// pixel and discards the rest. Box-averaging the full source instead keeps the
// shape legible, and the pixel-art sets halve cleanly at exactly 2x2.
func buildIcons() error {
	total := 0
	for _, src := range iconSources {
		dir := filepath.Join(append([]string{rawRoot}, src.dir...)...)
		files := pngsIn(dir)
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "  no icons under %s; skipping\n", dir)
			continue
		}
		outDir := filepath.Join(genRoot, "icons", src.prefix)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		for _, f := range files {
			img, err := readPNG(f)
			if err != nil {
				return err
			}
			small := boxDown(img, IconPx)
			out := filepath.Join(outDir, iconName(src.prefix, f)+".png")
			if err := writePNG(out, small); err != nil {
				return err
			}
			total++
		}
		fmt.Printf("ok     %-6s %3d icons -> %dpx\n", src.prefix, len(files), IconPx)
	}
	if total == 0 {
		return fmt.Errorf("no icons found; run `assetpipe extract tier1` first")
	}
	return nil
}

// iconName derives the key fragment for a source file, matching the naming the
// manifest builder uses so the two stay in step.
func iconName(prefix, path string) string {
	n := base(path)
	switch prefix {
	case "loot":
		n = strings.TrimSuffix(n, "_x")
		// Drop the "monloot_NN_" catalogue prefix.
		if i := strings.Index(n, "_"); i >= 0 {
			if j := strings.Index(n[i+1:], "_"); j >= 0 {
				n = n[i+1+j+1:]
			}
		}
	case "rune":
		if i := strings.LastIndex(n, "_"); i >= 0 {
			n = n[i+1:]
		}
	}
	return slug(n)
}

// boxDown reduces an image to size*size by averaging each source block. Alpha
// is premultiplied during the average so transparent pixels do not drag their
// colour into the result, which is what produces dark halos around cut-outs.
func boxDown(src image.Image, size int) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	sw, sh := b.Dx(), b.Dy()

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			x0, x1 := b.Min.X+x*sw/size, b.Min.X+(x+1)*sw/size
			y0, y1 := b.Min.Y+y*sh/size, b.Min.Y+(y+1)*sh/size
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}

			var rs, gs, bs, as, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					r, g, bl, a := src.At(sx, sy).RGBA() // already premultiplied
					rs += uint64(r)
					gs += uint64(g)
					bs += uint64(bl)
					as += uint64(a)
					n++
				}
			}
			if n == 0 {
				continue
			}
			ar := as / n
			if ar == 0 {
				continue
			}
			// Un-premultiply back to straight alpha for NRGBA storage.
			un := func(v uint64) uint8 {
				c := v / n * 0xFFFF / ar
				if c > 0xFFFF {
					c = 0xFFFF
				}
				return uint8(c >> 8)
			}
			out.SetNRGBA(x, y, color.NRGBA{un(rs), un(gs), un(bs), uint8(ar >> 8)})
		}
	}
	return out
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
