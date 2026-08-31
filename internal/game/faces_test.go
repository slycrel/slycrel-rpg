package game

import (
	"fmt"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"

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

// TestTheVendorsColumnDoesNotEatTheNames guards the trade the shop portrait
// made.
//
// The rows gave up 76 pixels so a vendor could stand beside them, and a shop
// row cuts its *label* to fit around its detail column — so that 76px comes out
// of item names, which in this game are the jokes. render.Trunc has a floor of
// 24px, meaning a long enough name in a narrow enough row is cut to three
// characters and a dot without anything reporting it.
//
// The assertion is not that names fit — they never all will — but that no two
// rows on one shelf cut down to the same string. Two identical rows is the
// point at which the truncation has stopped shortening a name and started
// destroying the information the row exists to carry.
func TestTheVendorsColumnDoesNotEatTheNames(t *testing.T) {
	g := storyGame(t)

	// The width a shop row actually gets, which is the whole subject here.
	const rowW = render.ScreenW - 80 - shopFaceCol

	// The floor is measured rather than chosen. Asserting "names must fit" is
	// false — they never all will, they are jokes and jokes are long — and
	// asserting no two rows collide turned out to be far too weak to fire: at
	// a column nearly three times the real one the truncated names were still
	// distinct. So this records the shortest any name is currently cut to and
	// fails if a change makes it shorter. It is a ratchet, not an ideal.
	worst, worstName, worstShown := 1<<30, "", ""
	// Measured at 32 with the column as it stands; set below that so content
	// can grow a longer name without tripping it, and high enough that a
	// meaningful width grab does. The first version used 14 and a column nearly
	// three times the real one sailed through it.
	const floor = 24

	for tier := 1; tier <= 5; tier++ {
		weapons, armors := g.Data.StockFor(tier)
		shields, charms := g.Data.SidearmsFor(tier)

		shelves := map[string][]struct{ name, detail string }{}
		add := func(shelf, name string, cost int) {
			shelves[shelf] = append(shelves[shelf], struct{ name, detail string }{
				name, fmt.Sprintf("%d coins", cost),
			})
		}
		for _, w := range weapons {
			if w.Cost > 0 {
				add("smith", w.Name, int(w.Cost))
			}
		}
		for _, a := range armors {
			if a.Cost > 0 {
				add("armourer", a.Name, int(a.Cost))
			}
		}
		for _, s := range shields {
			add("sidearms", s.Name, int(s.Cost))
		}
		for _, c := range charms {
			add("charms", c.Name, int(c.Cost))
		}

		for shelf, rows := range shelves {
			seen := map[string]string{}
			for _, r := range rows {
				// The same sum ui.Menu does: the detail is served first and the
				// label takes what is left, with Trunc's own 24px floor.
				room := rowW - render.TextW(r.detail) - 10 - 26
				shown := r.name
				if room < render.TextW(shown) {
					shown = render.Trunc(shown, core.MaxF(room, 24))
				}
				if prev, dup := seen[shown]; dup {
					t.Errorf("tier %d %s: %q and %q both show as %q in a %dpx row",
						tier, shelf, prev, r.name, shown, rowW)
				}
				seen[shown] = r.name
				// Only rows that were actually cut. An untruncated short name
				// says nothing about the column: "Table Leg" shows as "Table
				// Leg" whatever the width, and measuring it made the first
				// version of this test fail on a name that fits perfectly.
				if shown != r.name {
					if n := len([]rune(shown)); n < worst {
						worst = n
						worstName, worstShown = r.name, shown
					}
				}
			}
		}
	}

	if worst == 1<<30 {
		t.Log("nothing is truncated at this width")
		return
	}
	if worst < floor {
		t.Errorf("a shop row is cut to %d characters (%q shows as %q); the floor is %d.\n"+
			"Something took width from the rows -- most likely the vendor's column.",
			worst, worstName, worstShown, floor)
	}
	t.Logf("shortest visible name: %d characters (%q -> %q) in a %dpx row", worst, worstName, worstShown, rowW)
}
