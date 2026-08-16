package core

// Walker is a grid-stepping actor with smooth interpolation between tiles.
// Movement is tile-locked (it is that kind of game) but drawn continuously, so
// it reads as walking rather than teleporting.
//
// It lives in core rather than with the scenes because it is grid arithmetic
// and nothing else — it composes Point, Dir and TileSize, touches no art and
// draws nothing. Keeping it here is what lets the movement of the player and of
// the company behind them be tested without a window to draw into.
type Walker struct {
	Tile Point
	prev Point
	t    float64 // progress from prev to Tile, in [0,1]
	dir  Dir
	// dur is how many ticks a step takes.
	dur float64
}

// NewWalker returns a walker whose steps take dur ticks. A dur of zero falls
// back to a sensible pace rather than dividing by it.
func NewWalker(dur float64) Walker { return Walker{dur: dur} }

// Place teleports the walker with no animation.
func (w *Walker) Place(p Point) { w.Tile, w.prev, w.t = p, p, 1 }

// Moving reports whether a step is in progress.
func (w *Walker) Moving() bool { return w.t < 1 }

// Step begins a move to p, facing d.
func (w *Walker) Step(p Point, d Dir) {
	w.prev = w.Tile
	w.Tile = p
	w.dir = d
	w.t = 0
}

// Face turns on the spot. Walking into a wall does this, so that the sprite
// answers the key even when the step is refused.
func (w *Walker) Face(d Dir) {
	w.prev = w.Tile
	w.dir = d
	w.t = 1
}

// Settle finishes whatever step is in progress immediately, which is how a
// scripted capture avoids depending on how many ticks the tween has had.
func (w *Walker) Settle() { w.t = 1 }

// Advance progresses the interpolation one tick.
func (w *Walker) Advance() {
	if w.dur <= 0 {
		w.dur = 8
	}
	if w.t < 1 {
		w.t += 1 / w.dur
		if w.t > 1 {
			w.t = 1
		}
	}
}

// Pixel returns the walker's interpolated position in world pixels, centred on
// its tile horizontally and at the tile's bottom edge vertically, which is the
// anchor the renderer's world-space draw expects.
func (w *Walker) Pixel() (float64, float64) {
	fx := float64(w.prev.X) + (float64(w.Tile.X)-float64(w.prev.X))*w.t
	fy := float64(w.prev.Y) + (float64(w.Tile.Y)-float64(w.prev.Y))*w.t
	return fx*TileSize + TileSize/2, fy*TileSize + TileSize
}

// Dir returns the current facing.
func (w *Walker) Dir() Dir { return w.dir }
