package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Bands are pre-generated tier recolours of an icon, and they exist because
// armour had nineteen pieces sharing six pictures. The whole cloth lane — from
// "Regrettable Rags" at tier 0 to "Shroud of Ongoing Argument" at tier 5 — drew
// the same tuft of fur, so the only thing in a shop row that answered "which of
// these two coats is the better one" was the price column. Weapons never had
// the problem: the ability icon pack ships its own numbered tiers and the data
// already uses them (`1_weapon_sword`, `2_weapon_sword`, `4_weapon_sword`).
// This gives the loot icons the same thing.
//
// Recolouring is done here rather than by tinting at draw time for two reasons.
// It costs nothing at runtime on hardware that has no business rendering
// anything expensive, and — more to the point — a per-pixel ramp is not a tint:
// it re-shades the icon through a new palette, which is what makes a gilded
// coat read as gilded rather than as a brown coat behind yellow glass.
//
// Licence note: every tier in ASSET-LICENSING.md permits modifying and
// recolouring the art for use in the game. The loot icons are Tier A (Machado),
// whose worst-case reading assigns the derivative back to the artist — which
// costs this project nothing, since it never licenses the icons onward, but is
// the reason the output stays under the gitignored `_generated` tree with
// everything else derived from the bundle.

// band is one rung of the quality ladder: three palette stops the source
// luminance is mapped through, and how far to move the original towards them.
//
// The weight matters more than the colours. At 1.0 every icon in a band becomes
// the same coloured blob and the pack stops being readable as objects; at 0.2
// the ladder is invisible at 16px. The values below were picked by rendering
// the six armour icons across all six bands and looking at the sheet.
type band struct {
	name              string
	shadow, mid, high color.NRGBA
	weight            float64
}

// bands is the ladder, indexed by gear tier. The progression is deliberately
// the one every player already knows how to read — rag, leather, steel,
// silvered, gilded, rarefied — because an icon has sixteen pixels a side and no
// room to teach a new vocabulary.
var bands = []band{
	{"t0", color.NRGBA{34, 30, 28, 255}, color.NRGBA{84, 76, 68, 255}, color.NRGBA{132, 124, 112, 255}, 0.80},
	{"t1", color.NRGBA{48, 32, 20, 255}, color.NRGBA{122, 84, 48, 255}, color.NRGBA{196, 152, 100, 255}, 0.72},
	{"t2", color.NRGBA{28, 38, 48, 255}, color.NRGBA{92, 112, 132, 255}, color.NRGBA{176, 196, 214, 255}, 0.80},
	{"t3", color.NRGBA{46, 54, 74, 255}, color.NRGBA{140, 156, 186, 255}, color.NRGBA{234, 242, 255, 255}, 0.82},
	{"t4", color.NRGBA{62, 40, 8, 255}, color.NRGBA{176, 128, 28, 255}, color.NRGBA{252, 220, 122, 255}, 0.84},
	{"t5", color.NRGBA{52, 32, 76, 255}, color.NRGBA{146, 96, 200, 255}, color.NRGBA{238, 206, 255, 255}, 0.86},
}

// bandRoot holds the generated variants, alongside the reduced icons.
func bandRoot() string { return filepath.Join(genRoot, "bands") }

// buildBands writes a full ladder for every icon the armour table names.
//
// The source list comes from the content rather than from a hardcoded slice
// here, so adding a coat that reaches for a new picture regenerates the right
// files instead of silently producing none.
func buildBands() error {
	icons, err := armorIcons()
	if err != nil {
		return err
	}
	if len(icons) == 0 {
		return fmt.Errorf("no armour icons found in data/items/armor.json")
	}

	// Wipe first. The source list comes from the content, so a coat that stops
	// naming a picture has to stop shipping one too — the manifest enumerates
	// this directory, and a stale band would otherwise keep its key and travel
	// into every release build.
	if err := os.RemoveAll(bandRoot()); err != nil {
		return err
	}

	total := 0
	for _, key := range icons {
		set, name, ok := sourceIcon(key)
		if !ok {
			fmt.Fprintf(os.Stderr, "  skipping %s: not an icon key\n", key)
			continue
		}

		src := filepath.Join(genRoot, "icons", set, name+".png")
		img, err := readPNG(src)
		if err != nil {
			return fmt.Errorf("%s: %w (run `assetpipe icons` first)", key, err)
		}

		outDir := filepath.Join(bandRoot(), set)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		for _, b := range bands {
			out := filepath.Join(outDir, name+"_"+b.name+".png")
			if err := writePNG(out, applyBand(img, b)); err != nil {
				return err
			}
			total++
		}
	}
	fmt.Printf("ok     bands  %3d icons x %d tiers -> %d files\n", len(icons), len(bands), total)
	return nil
}

// armorIcons reads the distinct icon keys the armour table names, sorted so the
// pipeline's output does not depend on map iteration order.
func armorIcons() ([]string, error) {
	f, err := os.Open(filepath.Join("data", "items", "armor.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []struct {
		Icon string `json:"icon"`
	}
	if err := json.NewDecoder(f).Decode(&rows); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		set, name, ok := sourceIcon(r.Icon)
		if !ok {
			continue
		}
		if src := set + "/" + name; !seen[src] {
			seen[src] = true
			out = append(out, r.Icon)
		}
	}
	sort.Strings(out)
	return out, nil
}

// sourceIcon resolves an icon key to the reduced picture a band is cut from.
//
// It accepts both spellings on purpose. "icon/loot/fur_tuft" is a coat that has
// not been banded yet; "icon/band/loot/fur_tuft_t3" is one that has. Reading
// its own output back is what makes the pass idempotent — armour.json now names
// band keys, and running `assetpipe bands` a second time has to find the same
// six source pictures rather than none.
func sourceIcon(key string) (set, name string, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) == 4 && parts[0] == "icon" && parts[1] == "band" {
		parts = []string{"icon", parts[2], trimBandSuffix(parts[3])}
	}
	if len(parts) != 3 || parts[0] != "icon" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// trimBandSuffix drops a "_t3" written by a previous run, and leaves anything
// else alone — the ability set genuinely ships names ending in digits.
func trimBandSuffix(name string) string {
	for _, b := range bands {
		if s, cut := strings.CutSuffix(name, "_"+b.name); cut {
			return s
		}
	}
	return name
}

// applyBand re-shades an icon through a band's palette.
//
// Each pixel's luminance picks a colour off the three-stop ramp, and the result
// is mixed back towards the original by the band's weight. Mixing rather than
// replacing is what keeps a pelt looking like a pelt: replacing outright throws
// away every hue the artist used to say what the object is, and the pack turns
// into six identically-shaped smudges per tier.
//
// Alpha is carried through untouched. Anything fully transparent is left alone
// rather than ramped, or the ramp's shadow stop bleeds into the cut-out edge
// and every icon grows a dark fringe — the same failure boxDown avoids by
// premultiplying.
func applyBand(src image.Image, b band) *image.NRGBA {
	r := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))

	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			c := color.NRGBAModel.Convert(src.At(r.Min.X+x, r.Min.Y+y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			// Rec.601 luma, which tracks perceived brightness closely enough
			// for a sixteen-pixel icon and costs three multiplies.
			l := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255

			ramp := rampAt(b, l)
			out.SetNRGBA(x, y, color.NRGBA{
				mix(c.R, ramp.R, b.weight),
				mix(c.G, ramp.G, b.weight),
				mix(c.B, ramp.B, b.weight),
				c.A,
			})
		}
	}
	return out
}

// rampAt interpolates the band's three stops at a luminance in [0,1].
func rampAt(b band, l float64) color.NRGBA {
	switch {
	case l <= 0:
		return b.shadow
	case l >= 1:
		return b.high
	case l < 0.5:
		return lerp(b.shadow, b.mid, l*2)
	default:
		return lerp(b.mid, b.high, (l-0.5)*2)
	}
}

func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

// mix moves orig towards want by w.
func mix(orig, want uint8, w float64) uint8 {
	return uint8(float64(orig) + (float64(want)-float64(orig))*w)
}
