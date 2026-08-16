package core_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
)

func TestWalkerInterpolatesAcrossExactlyItsDuration(t *testing.T) {
	const dur = 8
	w := core.NewWalker(dur)
	w.Place(core.Point{X: 2, Y: 2})
	if w.Moving() {
		t.Fatal("a walker placed on a tile is moving")
	}

	w.Step(core.Point{X: 3, Y: 2}, core.DirRight)
	if !w.Moving() {
		t.Fatal("a walker that has just stepped is not moving")
	}
	// The tile updates immediately; only the drawn position lags, which is what
	// lets collision and interaction read Tile without waiting for the tween.
	if w.Tile != (core.Point{X: 3, Y: 2}) {
		t.Errorf("the walker's tile is %v, want the destination", w.Tile)
	}

	for i := 0; i < dur; i++ {
		if !w.Moving() {
			t.Fatalf("the step finished after %d of %d ticks", i, dur)
		}
		w.Advance()
	}
	if w.Moving() {
		t.Errorf("the step is still running after its full %d ticks", dur)
	}
	// Advancing past the end must not overshoot the destination.
	x, y := w.Pixel()
	w.Advance()
	if nx, ny := w.Pixel(); nx != x || ny != y {
		t.Errorf("advancing a finished walker moved it from (%v,%v) to (%v,%v)", x, y, nx, ny)
	}
}

// A walker with no duration set must still finish rather than divide by zero
// and never arrive.
func TestWalkerWithNoDurationStillArrives(t *testing.T) {
	var w core.Walker
	w.Place(core.Point{})
	w.Step(core.Point{X: 1}, core.DirRight)
	for i := 0; i < 100 && w.Moving(); i++ {
		w.Advance()
	}
	if w.Moving() {
		t.Fatal("a zero-duration walker never finished its step")
	}
}

// Facing turns on the spot: walking into a wall answers the key without moving.
func TestFaceTurnsWithoutMoving(t *testing.T) {
	w := core.NewWalker(8)
	at := core.Point{X: 5, Y: 5}
	w.Place(at)

	before, beforeY := w.Pixel()
	w.Face(core.DirUp)
	if w.Dir() != core.DirUp {
		t.Errorf("facing up left the walker looking %v", w.Dir())
	}
	if w.Tile != at {
		t.Errorf("facing moved the walker to %v", w.Tile)
	}
	if w.Moving() {
		t.Error("facing started a step")
	}
	if x, y := w.Pixel(); x != before || y != beforeY {
		t.Errorf("facing moved the drawn position to (%v,%v)", x, y)
	}
}

func TestSettleFinishesAStepAtOnce(t *testing.T) {
	w := core.NewWalker(30)
	w.Place(core.Point{})
	w.Step(core.Point{X: 1, Y: 0}, core.DirRight)
	w.Settle()
	if w.Moving() {
		t.Fatal("the walker is still moving after Settle")
	}
	// And it has to be settled *at the destination*, not somewhere along the way.
	x, _ := w.Pixel()
	want := float64(1*core.TileSize) + core.TileSize/2
	if x != want {
		t.Errorf("the settled walker is drawn at x=%v, want %v", x, want)
	}
}

// The drawn anchor is the bottom-centre of the tile, which is what the renderer
// expects; getting it wrong puts every sprite half a tile out.
func TestPixelAnchorsToBottomCentre(t *testing.T) {
	w := core.NewWalker(8)
	w.Place(core.Point{X: 3, Y: 4})
	x, y := w.Pixel()
	if x != 3*core.TileSize+core.TileSize/2 {
		t.Errorf("x anchor is %v, want the tile's horizontal centre", x)
	}
	if y != 4*core.TileSize+core.TileSize {
		t.Errorf("y anchor is %v, want the tile's bottom edge", y)
	}
}

func TestDirBetween(t *testing.T) {
	o := core.Point{X: 4, Y: 4}
	cases := []struct {
		to   core.Point
		want core.Dir
	}{
		{core.Point{X: 5, Y: 4}, core.DirRight},
		{core.Point{X: 3, Y: 4}, core.DirLeft},
		{core.Point{X: 4, Y: 3}, core.DirUp},
		{core.Point{X: 4, Y: 5}, core.DirDown},
		{o, core.DirDown}, // no movement at all falls back to facing the camera
	}
	for _, c := range cases {
		if got := core.DirBetween(o, c.to); got != c.want {
			t.Errorf("DirBetween(%v, %v) = %v, want %v", o, c.to, got, c.want)
		}
	}
	// Horizontal wins on a diagonal, matching the four-way sprite sheets.
	if got := core.DirBetween(o, core.Point{X: 5, Y: 5}); got != core.DirRight {
		t.Errorf("a diagonal step faced %v, want the horizontal component", got)
	}
}
