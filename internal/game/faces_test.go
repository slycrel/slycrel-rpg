package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/render"
)

// TestCaptionsFitUnderAPortrait is the guard on a thing that has already gone
// wrong twice today in the other direction.
//
// A caption is drawn under a 76px portrait and wrapped to that width over at
// most two lines. Anything longer is not shortened, it is silently dropped —
// render.Wrap emits a third line and the draw loop stops at two — so an
// over-long caption loses its ending and nothing anywhere says so. "undead ma."
// is not a shorter way of saying "undead mage", and a caption that ends
// mid-clause is worse than no caption.
//
// A single word wider than the portrait is worse still: it cannot be wrapped at
// all, so it runs out through the panel border, which a panel does not clip.
func TestCaptionsFitUnderAPortrait(t *testing.T) {
	pools := map[string]captionPool{"innkeeper": capInnkeeper}
	for kind, p := range vendorCaptions {
		pools["vendor "+string(kind)] = p
	}
	for kind, p := range questCaptions {
		pools["quest "+string(kind)] = p
	}

	// Two columns, because the same caption is drawn in two places at two
	// widths: the conversation panel's left column and the narrower strip the
	// shop's rows give up. It only has to fail in one of them to be broken, and
	// the shop is the narrow one.
	widths := map[string]int{"conversation": captionW, "shop": shopCaptionW}

	for what, pool := range pools {
		for _, caption := range pool {
			for where, width := range widths {
				// The shop drops the trade word, so it is a different string at
				// a different width and has to be measured as one. Held in its
				// own variable: assigning back to caption would leak the
				// shortened form into the other width, and map iteration order
				// would decide whether the test noticed.
				text := caption
				if where == "shop" {
					text = shopCaption(caption)
				}
				lines := render.Wrap(text, float64(width))
				if len(lines) > roleLines {
					t.Errorf("%s: %q wraps to %d lines in the %s column, and only %d are drawn",
						what, text, len(lines), where, roleLines)
				}
				for _, ln := range lines {
					if w := render.TextW(ln); w > float64(width) {
						t.Errorf("%s: %q has an unbreakable run %.0fpx wide, against the %dpx %s column",
							what, text, w, width, where)
					}
				}
			}
		}
	}
}

// TestEveryCaptionedPoolHasSomethingInIt catches the half-done edit: a pool
// declared, wired into the switch, and left empty, which shows as no caption at
// all rather than as an error.
func TestEveryCaptionedPoolHasSomethingInIt(t *testing.T) {
	for kind, p := range vendorCaptions {
		if len(p) == 0 {
			t.Errorf("vendor %q has no captions", kind)
		}
	}
}
