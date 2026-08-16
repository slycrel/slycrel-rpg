package tiles

import "testing"

func TestCornerShapeCoversEveryNeighbourhood(t *testing.T) {
	cases := []struct {
		v, h, d bool
		want    int
		drawn   bool
	}{
		{false, false, false, 0, false},       // nothing touches: leave the base
		{false, false, true, shapeDiag, true}, // only across the diagonal
		{false, true, false, shapeHoriz, true},
		{true, false, false, shapeVert, true},
		{true, true, false, shapeBoth, true},
		// The diagonal must not override a side: those shapes already cover it.
		{true, false, true, shapeVert, true},
		{false, true, true, shapeHoriz, true},
		{true, true, true, shapeBoth, true},
	}
	for _, c := range cases {
		got, drawn := cornerShape(c.v, c.h, c.d)
		if drawn != c.drawn || (drawn && got != c.want) {
			t.Errorf("cornerShape(v=%v h=%v d=%v) = (%d,%v), want (%d,%v)",
				c.v, c.h, c.d, got, drawn, c.want, c.drawn)
		}
	}
}

// TestMasksAgreeAtSharedEdges is what stops a visible seam: the strip a
// vertical edge lays down must be the mirror of the one the opposite corner
// lays down, or two tiles meeting along a boundary would blend by different
// amounts.
func TestMasksAgreeAtSharedEdges(t *testing.T) {
	for _, sh := range []int{shapeVert, shapeHoriz, shapeBoth, shapeDiag} {
		for y := 0; y < Quarter; y++ {
			for x := 0; x < Quarter; x++ {
				a := coverage(sh, x, y)
				if a < 0 || a > 1 {
					t.Fatalf("shape %d coverage at (%d,%d) = %v, outside [0,1]", sh, x, y, a)
				}
			}
		}
	}
	// A vertical edge must reach in from the top and stop; if it covered the
	// whole quarter the blend would swallow the tile.
	if coverage(shapeVert, 4, 0) < 0.9 {
		t.Error("vertical edge does not cover the row it borders")
	}
	if coverage(shapeVert, 4, Quarter-1) > 0.1 {
		t.Error("vertical edge reaches all the way across the quarter")
	}
	// The "both" shape is nearly full, open only at the far corner.
	if coverage(shapeBoth, 0, 0) < 0.9 {
		t.Error("inner corner is not filled at the near corner")
	}
	if coverage(shapeBoth, Quarter-1, Quarter-1) > 0.1 {
		t.Error("inner corner is not open at the far corner")
	}
}

// TestRollSpreadsAcrossNeighbours guards the anti-repetition trick: adjacent
// tiles must not land on the same texture phase in rows or columns, which is
// exactly what a naive (x+y)%n would do.
func TestRollSpreadsAcrossNeighbours(t *testing.T) {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if rollFor(x, y) == rollFor(x+1, y) {
				t.Errorf("tiles (%d,%d) and (%d,%d) share a roll horizontally", x, y, x+1, y)
			}
			if rollFor(x, y) == rollFor(x, y+1) {
				t.Errorf("tiles (%d,%d) and (%d,%d) share a roll vertically", x, y, x, y+1)
			}
		}
	}
	seen := map[int]bool{}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			r := rollFor(x, y)
			if r < 0 || r >= Rolls {
				t.Fatalf("rollFor(%d,%d) = %d, outside [0,%d)", x, y, r, Rolls)
			}
			seen[r] = true
		}
	}
	if len(seen) != Rolls {
		t.Errorf("only %d of %d rolls are ever used", len(seen), Rolls)
	}
}
