package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The drawing half of the clock. internal/sky decides what it is doing; this
// decides what that looks like, which is the split every other domain package
// in here follows.

// weatherAt is what the sky is doing over a tile of the continent.
func (g *Game) weatherAt(at core.Point) sky.Weather {
	if g.World == nil {
		return sky.Clear
	}
	return sky.At(g.Seed, g.Clock, g.World.At(at.X, at.Y).Biome())
}

// weatherHere is the same question wherever the player currently is, indoors or
// out. A location's biome comes off the point of interest, so a cave in the
// mountains gets the mountains' weather — which matters at the door and, since
// interiors are drawn without a sky, nowhere else.
func (g *Game) weatherHere() sky.Weather {
	if g.Local != nil {
		return sky.At(g.Seed, g.Clock, g.Local.Biome)
	}
	return g.weatherAt(g.Walk.Tile)
}

// skyLine is the one phrase the status bar uses to say what it is like out
// there: "night, rain". Clear weather says only the time, because "day, clear"
// is three quarters of a run's worth of status bar spent saying nothing.
func (g *Game) skyLine() string {
	p := g.Clock.Phase()
	if w := g.weatherHere(); w != sky.Clear {
		return p.Name() + ", " + w.Name()
	}
	return p.Name()
}

// abed reports whether an entity has gone in for the night.
//
// The plan's phrase for this was "which NPCs are out", and the answer is: the
// people, not the buildings. Townsfolk and the hopefuls loitering outside inns
// go home after dusk. Counters, beds, altars, chests and anything with teeth
// stay exactly where they are — a merchant will always take money, and a
// dungeon does not keep hours.
//
// It is never a dead end, which is what makes it usable at all. A player who
// arrives at midnight with an errand to hand in can take a room and do it in
// the morning, and the inn is the one thing that is definitely open. That turns
// "the town is asleep" from an obstruction into the reason the bed exists.
//
// Only ever consulted in a settlement. A camp full of bandits does not empty
// out because it got late.
func (g *Game) abed(e *world.Entity) bool {
	if e == nil || g.Local == nil || !g.Local.POI.Kind.Settlement() {
		return false
	}
	if !g.Clock.Phase().Dark() {
		return false
	}
	switch e.Kind {
	case world.ENPC, world.ERecruit:
		return true
	}
	return false
}

// ahead is the interactable the player is facing, skipping anybody who has gone
// home. One helper rather than three, because the hint on the status bar, the
// key that acts on it and the sprite on the floor have to agree — a name in the
// corner for somebody who is not drawn is worse than either.
func (g *Game) ahead() *world.Entity {
	if g.Local == nil {
		return nil
	}
	at := g.LocalWalk.Tile.Add(g.LocalWalk.Dir().Delta())
	e := g.Local.EntityAt(at.X, at.Y)
	if e == nil || e.Used || g.abed(e) {
		return nil
	}
	return e
}

// Tints, as straight colours multiplied over the finished frame.
//
// Dusk is warm and night is cold, which is the whole of making one read as the
// end of a day and the other as a different place. Both are deliberately shy of
// what looks right in a screenshot: a tint dark enough to be atmospheric is a
// tint that makes the terrain unreadable, and the player still has to be able
// to tell a forest from a swamp at two in the morning.
var phaseTint = map[sky.Phase]color.RGBA{
	sky.Dawn:  {0xE8, 0xD0, 0xC8, 0xFF},
	sky.Dusk:  {0xC8, 0x9C, 0x8C, 0xFF},
	sky.Night: {0x6C, 0x78, 0xA8, 0xFF},
}

// drawSky lays the time of day and the weather over a finished world frame.
//
// Over, rather than under: it is the last thing that happens before the HUD, so
// the status bar stays legible at midnight. A tint that also dimmed the numbers
// would be a mood at the cost of the interface.
func (g *Game) drawSky(dst *ebiten.Image, w sky.Weather, indoors bool) {
	if tint, ok := phaseTint[g.Clock.Phase()]; ok {
		render.Multiply(dst, tint)
	}
	// Nothing falls on you indoors. A dungeon is dark for its own reasons and
	// always has been, and rain in a cellar would be the funniest bug in the
	// game for about one screenshot.
	if !indoors && w.Falling() {
		g.drawFalling(dst, w)
	}
}

// fallSheet is the art for each kind of weather, and how fast it runs.
//
// The pack ships light and heavy versions of both rain and snow and suggests
// overlaying them at different speeds. That is what storm is: the same rain
// twice, out of step with itself, which reads as much heavier than either layer
// does alone and costs nothing but a second pass.
type fallSheet struct {
	key   string
	every int // ticks per frame
	tint  color.RGBA
}

var fallSheets = map[sky.Weather][]fallSheet{
	sky.Rain: {
		{"weather/rain_light", 4, color.RGBA{0xB0, 0xC4, 0xE0, 0x70}},
	},
	sky.Storm: {
		{"weather/rain_heavy", 5, color.RGBA{0x98, 0xAC, 0xCC, 0x70}},
		{"weather/rain_light", 3, color.RGBA{0xC0, 0xD4, 0xF0, 0x90}},
	},
	sky.Snow: {
		{"weather/snow_light", 9, color.RGBA{0xFF, 0xFF, 0xFF, 0xC0}},
		{"weather/snow_heavy", 6, color.RGBA{0xE0, 0xE8, 0xF8, 0x90}},
	},
}

// drawFalling tiles the weather sheet across the screen.
//
// The sheets are two tiles wide and eight high, so a screen takes fifteen
// columns and three rows of them. They are drawn at a fixed screen position
// rather than scrolled with the camera: rain does not belong to the ground, and
// weather that slid sideways when the player walked would read as the world
// moving rather than the water falling.
//
// Every column runs a different frame of the animation. Tiling one 32-pixel
// sheet straight across put fifteen identical columns on screen in lockstep,
// which read as vertical stripes rather than as rain — the eye finds a repeat
// that regular instantly. Stepping the frame per column costs nothing, since
// every frame is already loaded, and turns the same eight pictures into
// something with no period the eye can catch.
func (g *Game) drawFalling(dst *ebiten.Image, w sky.Weather) {
	const (
		sheetW = 32
		sheetH = 128
	)
	for _, f := range fallSheets[w] {
		sp := g.Assets.Get(f.key)
		if sp == nil || sp.Count() == 0 {
			continue
		}
		frame := g.Tick() / core.Max(1, f.every)
		col := 0
		for x := 0; x < render.ScreenW; x += sheetW {
			// Two coprime strides — three across, five down — so neither the
			// columns nor the rows fall back into step with each other.
			row := 0
			for y := -sheetH; y < render.ScreenH; y += sheetH {
				render.ScreenTinted(dst, sp, frame+col*3+row*5, float64(x), float64(y), f.tint)
				row++
			}
			col++
		}
	}
}
