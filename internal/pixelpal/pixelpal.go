// Package pixelpal derives the weapon palette that authored icons are drawn
// against.
//
// It exists because two build tools need the same answer and got different
// ones. `cmd/pixelsmith` shows a model the palette and asks for a grid of slot
// numbers; `cmd/assetpipe` renders that grid back. If the two disagree about
// which colour slot 4 is, the icon comes out wrong in a way nothing detects —
// the grid is valid, the render succeeds, and the spear is simply yellow. That
// happened, before this package existed: one tool sampled eight icons and the
// other sampled all twenty-four, and the second set contains the gold colourway,
// which pushed a yellow into slot 4.
//
// So the rule lives here once. It is not in cmd/ because both binaries import
// it, and it is not in the game because the game never sees it.
package pixelpal

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
)

// Sources are the icons the palette is read from: the first colourway of each
// weapon shape.
//
// The first only, deliberately. The sheet draws every weapon three times — steel,
// gold, and a red-jewelled one — and taking all of them yields nine colours with
// two yellows in the middle of the ordering. An authored icon is banded by
// `assetpipe bands` afterwards like every other piece of gear, so its base art
// wants the neutral colourway and nothing else.
var Sources = []string{
	"sword1", "axe1", "hammer1", "mace1", "dagger1", "staff1", "pick1", "bow1",
}

// Colour is one palette slot: the colour, and what it is for. The role text is
// what a model is told, and a model told "colour 3" with no further detail
// lights its icons from nowhere.
type Colour struct {
	RGBA color.NRGBA
	Role string
}

// roles describes the slots in the order Load returns them. Ordering is by how
// much of the weapon set each colour covers, which is stable across machines
// because ties break on the colour value.
var roles = []string{
	"mid steel, the body of a blade or a haft",
	"dark steel, the outline and the shadowed side",
	"pale steel, the highlight along the lit edge",
	"dark red, only for a binding, a grip wrap or a gem",
	"bright red, the same but for its lit edge",
	"muted steel, between the body and the shadow",
}

// Load reads the palette out of the cut weapon icons in dir.
func Load(dir string) ([]Colour, error) {
	counts := map[color.NRGBA]int{}
	for _, name := range Sources {
		f, err := os.Open(filepath.Join(dir, name+".png"))
		if err != nil {
			return nil, fmt.Errorf("%w (run `assetpipe arms` first)", err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		count(img, counts)
	}
	if len(counts) == 0 {
		return nil, fmt.Errorf("no colours found in %s", dir)
	}

	cols := make([]color.NRGBA, 0, len(counts))
	for c := range counts {
		cols = append(cols, c)
	}
	sort.Slice(cols, func(i, j int) bool {
		if a, b := counts[cols[i]], counts[cols[j]]; a != b {
			return a > b
		}
		return key(cols[i]) < key(cols[j])
	})

	out := make([]Colour, len(cols))
	for i, c := range cols {
		role := "an extra tone, unused by the examples"
		if i < len(roles) {
			role = roles[i]
		}
		out[i] = Colour{RGBA: c, Role: role}
	}
	return out, nil
}

func count(img image.Image, into map[color.NRGBA]int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A > 0 {
				c.A = 255
				into[c]++
			}
		}
	}
}

func key(c color.NRGBA) int { return int(c.R)<<16 | int(c.G)<<8 | int(c.B) }

// Hex renders a colour the way a prompt wants to read it.
func Hex(c color.NRGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }
