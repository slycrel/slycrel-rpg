package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/thread"
)

// What the player is currently following, and which way it is.
//
// The journal names a destination and the map has a pin on it, and between
// those two facts is the actual business of getting there: open the map, find
// the pin, remember roughly where it was, close the map, walk, repeat. That is
// worst exactly where the long stories send you, because a saga's legs are
// deliberately further out each time.
//
// Two halves, and the tracking is the more important one. "Where is it" is the
// second question a journal with six entries in it raises; the first is "which
// of these am I doing". A compass with nothing selected has nothing to point
// at.

// Track is the thing being followed.
//
// On is explicit rather than POI being -1, which is the same lesson the resident
// threads taught: a zero value that means something real is a zero value that
// turns every old save and every unset struct into a silent claim about
// location zero.
type Track struct {
	On    bool   `json:"on"`
	POI   int    `json:"poi"`
	Label string `json:"label"`
}

// trackPOI starts following a location.
func (g *Game) trackPOI(idx int, label string) {
	if g.World == nil || idx < 0 || idx >= len(g.World.POIs) {
		return
	}
	g.Track = Track{On: true, POI: idx, Label: label}
}

// trackIfIdle follows something only when nothing is being followed already, or
// when what was being followed has been arrived at.
//
// This is what makes the feature work for a player who never finds it. A saga
// leg coming due sets the next destination automatically; an explicit choice in
// the journal is never overridden by one.
func (g *Game) trackIfIdle(idx int, label string) {
	if g.Track.On && g.Track.POI != idx && !g.atTracked() {
		return
	}
	g.trackPOI(idx, label)
}

// atTracked reports whether the player is standing on what they are following.
func (g *Game) atTracked() bool {
	if !g.Track.On || g.World == nil || g.Track.POI < 0 || g.Track.POI >= len(g.World.POIs) {
		return false
	}
	return g.World.POIs[g.Track.POI].Pos == g.Walk.Tile
}

// trackLine is what the status bar says about it: the name, and how far.
//
// The distance is in tiles and deliberately not in anything friendlier. A
// player who has walked anywhere at all knows what forty tiles feels like, and
// "a long way" would be the game declining to answer the question.
func (g *Game) trackLine() (string, bool) {
	if !g.Track.On || g.World == nil || g.Track.POI < 0 || g.Track.POI >= len(g.World.POIs) {
		return "", false
	}
	p := g.World.POIs[g.Track.POI]
	if p.Pos == g.Walk.Tile {
		return render.Trunc(g.Track.Label, 150) + " - here", true
	}
	return fmt.Sprintf("%s %d", render.Trunc(g.Track.Label, 130),
		p.Pos.Manhattan(g.Walk.Tile)), true
}

// trackBearing is which of eight ways the tracked place lies, as an index into
// compassGlyphs, and whether there is one at all.
func (g *Game) trackBearing() (int, bool) {
	if !g.Track.On || g.World == nil || g.Track.POI < 0 || g.Track.POI >= len(g.World.POIs) {
		return 0, false
	}
	p := g.World.POIs[g.Track.POI]
	dx, dy := p.Pos.X-g.Walk.Tile.X, p.Pos.Y-g.Walk.Tile.Y
	if dx == 0 && dy == 0 {
		return 0, false
	}
	return bearing(dx, dy), true
}

// bearing reduces a vector to one of eight compass points.
//
// A direction counts as diagonal when neither axis dominates the other by more
// than about two to one. Splitting the circle into eight equal wedges instead
// would make almost everything diagonal — most places on a 160x120 map are
// off-axis by a little — and an arrow that is diagonal nine times out of ten is
// an arrow that has stopped saying anything.
func bearing(dx, dy int) int {
	ax, ay := core.Abs(dx), core.Abs(dy)
	diagonal := ax*2 > ay && ay*2 > ax
	switch {
	case diagonal && dx > 0 && dy < 0:
		return dirNE
	case diagonal && dx > 0:
		return dirSE
	case diagonal && dy > 0:
		return dirSW
	case diagonal:
		return dirNW
	case ax > ay && dx > 0:
		return dirE
	case ax > ay:
		return dirW
	case dy > 0:
		return dirS
	}
	return dirN
}

// The eight points, in the order compassGlyphs lists them.
const (
	dirN = iota
	dirNE
	dirE
	dirSE
	dirS
	dirSW
	dirW
	dirNW
)

// compassGlyphs are 7x7 arrowheads, hand-set rather than rasterised.
//
// A triangle computed and filled at this size comes out as a smear that reads
// differently in each direction — the diagonals in particular. Seven pixels
// square is small enough that drawing them by hand is quicker than getting a
// rasteriser to agree with itself, and the result is exact.
var compassGlyphs = [8][]string{
	dirN: {
		"...#...",
		"..###..",
		".##.##.",
		"#..#..#",
		"...#...",
		"...#...",
		"...#...",
	},
	dirNE: {
		"..#####",
		"....###",
		"...#.##",
		"..#...#",
		".#.....",
		"#......",
		".......",
	},
	dirE: {
		"...#...",
		"....#..",
		".....#.",
		"#######",
		".....#.",
		"....#..",
		"...#...",
	},
	dirSE: {
		".......",
		"#......",
		".#.....",
		"..#...#",
		"...#.##",
		"....###",
		"..#####",
	},
	dirS: {
		"...#...",
		"...#...",
		"...#...",
		"#..#..#",
		".##.##.",
		"..###..",
		"...#...",
	},
	dirSW: {
		".......",
		"......#",
		".....#.",
		"#...#..",
		"##.#...",
		"###....",
		"#####..",
	},
	dirW: {
		"...#...",
		"..#....",
		".#.....",
		"#######",
		".#.....",
		"..#....",
		"...#...",
	},
	dirNW: {
		"#####..",
		"###....",
		"##.#...",
		"#...#..",
		".....#.",
		"......#",
		".......",
	},
}

// drawCompass paints one arrowhead at x,y.
func drawCompass(dst *ebiten.Image, dir int, x, y float64, c color.Color) {
	if dir < 0 || dir >= len(compassGlyphs) {
		return
	}
	for row, line := range compassGlyphs[dir] {
		for col, ch := range line {
			if ch == '#' {
				render.Rect(dst, x+float64(col), y+float64(row), 1, 1, c)
			}
		}
	}
}

// destinationOf reports where a journal row points, and what to call it.
//
// One function rather than three, because the three kinds of outstanding thing
// have to answer the same question the same way — a player selecting a row does
// not care which system it came out of.
func (g *Game) destinationOf(data any) (int, string, bool) {
	switch d := data.(type) {
	case *saga.Saga:
		if idx := d.Place(); idx >= 0 {
			return idx, d.PlaceName(), true
		}
	case *quest.Quest:
		if d.TargetPOI > 0 || d.TargetName != "" {
			return d.TargetPOI, d.TargetName, true
		}
		// A fetch or a cull has nowhere of its own to point at, so the answer
		// is where it gets handed in — which is genuinely the next place the
		// player wants to be once the counter fills.
		if d.Complete() {
			return d.GiverPOI, g.poiName(d.GiverPOI), true
		}
	case *thread.Thread:
		if d.IsResident(&g.Data.Threads) {
			return d.HomePOI, g.poiName(d.HomePOI), true
		}
		if d.PlacePOI >= 0 {
			return d.PlacePOI, g.poiName(d.PlacePOI), true
		}
	}
	return 0, "", false
}
