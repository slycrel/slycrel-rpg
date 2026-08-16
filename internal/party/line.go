package party

import "github.com/slycrel/slycrel-rpg/internal/core"

// Line is the company's marching order: one walker per companion, following
// the hero.
//
// It is never saved. A companion's position is re-derived from where the hero
// is standing whenever a map is entered, which is indistinguishable from having
// walked in together and is one fewer thing that can be wrong in a save file.
type Line []core.Walker

// Fit resizes the line to n followers, standing any new ones on at.
func Fit(l Line, n int, at core.Point, dur float64) Line {
	if len(l) > n {
		return l[:n]
	}
	for len(l) < n {
		w := core.NewWalker(dur)
		w.Place(at)
		l = append(l, w)
	}
	return l
}

// Place teleports the whole line onto a tile, for entering a location or
// loading a save.
func (l Line) Place(at core.Point) {
	for i := range l {
		l[i].Place(at)
	}
}

// Step walks the company forward one tile.
//
// Each companion steps onto the tile the one ahead of it has just left, which
// is why the line bends around corners instead of cutting them: what it follows
// is the leader's history, not the leader.
func (l Line) Step(leaderFrom core.Point) {
	next := leaderFrom
	for i := range l {
		from := l[i].Tile
		if from != next {
			l[i].Step(next, core.DirBetween(from, next))
		}
		next = from
	}
}

// Advance progresses every follower's interpolation one tick.
func (l Line) Advance() {
	for i := range l {
		l[i].Advance()
	}
}

// Settle finishes every follower's step immediately, so a scripted capture does
// not depend on how many ticks the tweens have had.
func (l Line) Settle() {
	for i := range l {
		l[i].Settle()
	}
}
