package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The corner map.
//
// The parchment map under M has always answered "where am I", and it answers it
// by taking over the screen — which means the question can only be asked
// standing still, and the answer has to be memorised before it goes away. That
// is fine for planning a journey and useless for the thing a player actually
// does with a map, which is glance at it.
//
// So this is the same information at a tenth the size and none of the ceremony:
// a window of the continent around the player, painted one pixel to the tile,
// with the followed destination marked on it. The full map keeps its job. This
// one only has to be readable out of the corner of an eye.

const (
	// miniTiles is how much continent the window shows, in tiles. Sixty-four of
	// a hundred and sixty across is a bit under half the width of the world:
	// wide enough that the next town is usually on it, narrow enough that a
	// pixel is still a place you could walk to rather than a county.
	miniTiles = 64
	// miniPad is the gap between the panel border and the screen edge.
	miniPad = 6
)

// minimap holds the painted continent and remembers what it was painted from.
type minimap struct {
	// canvas is the whole world at one pixel per tile, not just the window.
	//
	// Painting the world and then showing a slice of it, rather than painting
	// the slice, is what makes scrolling free: walking east moves the window by
	// a pixel and repaints nothing. It costs 160x120 pixels of texture, which
	// is less than one of the character portraits.
	canvas *ebiten.Image
	// pix is the staging buffer, reused. WritePixels wants the whole image at
	// once, and allocating 76 KB every time the player takes a step would be a
	// garbage collection nobody asked for.
	pix []byte

	// What the canvas was last painted from. The fog only changes when the
	// player moves and the pins only when something is discovered, so these two
	// are enough to know the picture is still true.
	at    core.Point
	found int
	drawn bool
}

// miniFog is unexplored ground: not black, because the panel sits over moving
// terrain and a black hole in the corner reads as a hole in the world.
var miniFog = color.RGBA{0x1A, 0x16, 0x22, 0xFF}

// repaint rebuilds the canvas if anything it depends on has changed.
func (m *minimap) repaint(g *Game) {
	found := 0
	for _, p := range g.World.POIs {
		if p.Discovered {
			found++
		}
	}
	if m.drawn && m.at == g.Walk.Tile && m.found == found {
		return
	}
	m.at, m.found, m.drawn = g.Walk.Tile, found, true

	if m.canvas == nil {
		m.canvas = ebiten.NewImage(world.Width, world.Height)
		m.pix = make([]byte, world.Width*world.Height*4)
	}

	// Straight into the pixel buffer rather than through nineteen thousand
	// one-pixel Rect calls. The parchment map can afford those because it is
	// built once when the screen opens; this one is rebuilt on every step.
	for y := 0; y < world.Height; y++ {
		for x := 0; x < world.Width; x++ {
			c := miniFog
			if g.World.IsExplored(x, y) {
				c = mapColor(g.World.At(x, y))
			}
			i := (y*world.Width + x) * 4
			m.pix[i], m.pix[i+1], m.pix[i+2], m.pix[i+3] = c.R, c.G, c.B, 0xFF
		}
	}
	for _, p := range g.World.POIs {
		if !p.Discovered {
			continue
		}
		c := poiColor(p.Kind)
		i := (p.Pos.Y*world.Width + p.Pos.X) * 4
		m.pix[i], m.pix[i+1], m.pix[i+2], m.pix[i+3] = c.R, c.G, c.B, 0xFF
	}
	m.canvas.WritePixels(m.pix)
}

// window is the top-left tile of the visible slice, clamped to the continent.
//
// Clamping rather than letting the window hang off the edge, so the corner of
// the world shows a corner of the world instead of a band of nothing. The cost
// is that the player stops being centred once they reach the coast, which is
// the correct thing for a map to do and what every paper one does anyway.
func miniWindow(at core.Point) (int, int) {
	x := core.Clamp(at.X-miniTiles/2, 0, world.Width-miniTiles)
	y := core.Clamp(at.Y-miniTiles/2, 0, world.Height-miniTiles)
	return x, y
}

// draw paints the panel in the top-right corner.
func (m *minimap) draw(g *Game, dst *ebiten.Image) {
	if g.World == nil || g.Player == nil {
		return
	}
	m.repaint(g)

	x := float64(render.ScreenW - miniPad - miniTiles)
	y := float64(miniPad)
	ui.Panel(dst, x-3, y-3, miniTiles+6, miniTiles+6)

	wx, wy := miniWindow(g.Walk.Tile)
	op := &ebiten.DrawImageOptions{}
	// Translate to the panel, and nothing else. A sub-image draws from its own
	// bounds origin — the same reason a sprite sheet's frame lands at the x,y
	// you ask for rather than at x,y plus wherever the frame sat in the sheet.
	// Subtracting the window offset here drew the slice up and to the left of
	// its own border, which looked like a second panel bleeding off the corner
	// of the screen.
	op.GeoM.Translate(x, y)
	dst.DrawImage(m.canvas.SubImage(
		image.Rect(wx, wy, wx+miniTiles, wy+miniTiles)).(*ebiten.Image), op)

	// The player, blinking, for the same reason the parchment map blinks: a
	// white dot among forty coloured ones is not findable, and a white dot that
	// is the only thing moving is findable instantly.
	if (g.Tick()/16)%2 == 0 {
		render.Rect(dst, x+float64(g.Walk.Tile.X-wx)-1, y+float64(g.Walk.Tile.Y-wy)-1,
			3, 3, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	}

	m.drawTracked(g, dst, x, y, wx, wy)
}

// drawTracked marks whatever is being followed, on the map if it is on the map
// and on the border pointing at it if it is not.
//
// This is the half the HUD line could not do. "Gorse Shrine 46" says how far
// and the arrowhead beside it says which way, but neither says *where* — and
// the difference between a bearing and a position is the difference between
// walking north-east and walking around a bay. Once the destination is a pin on
// a picture of the coastline, it is a route.
func (m *minimap) drawTracked(g *Game, dst *ebiten.Image, x, y float64, wx, wy int) {
	at, ok := g.tracked()
	if !ok {
		return
	}
	tx, ty := at.X-wx, at.Y-wy

	if tx >= 0 && tx < miniTiles && ty >= 0 && ty < miniTiles {
		// On the window: a ring around the pin rather than a dot on top of it,
		// so the marker says "this one" without hiding which kind it is.
		gx, gy := x+float64(tx), y+float64(ty)
		render.Rect(dst, gx-2, gy-2, 5, 1, render.ColGold)
		render.Rect(dst, gx-2, gy+2, 5, 1, render.ColGold)
		render.Rect(dst, gx-2, gy-1, 1, 3, render.ColGold)
		render.Rect(dst, gx+2, gy-1, 1, 3, render.ColGold)
		return
	}

	// Off the window: the same arrowhead the HUD uses, pinned to the border in
	// the direction of travel. Reusing the glyphs rather than drawing a second
	// kind of arrow, so the two marks a player learns are one mark.
	dir, ok := g.trackBearing()
	if !ok {
		return
	}
	ax := core.ClampF(x+float64(tx)-3, x, x+miniTiles-7)
	ay := core.ClampF(y+float64(ty)-3, y, y+miniTiles-7)
	// A dark backing, because a gold arrowhead over pale desert is a smudge.
	render.Rect(dst, ax-1, ay-1, 9, 9, color.RGBA{0x10, 0x0C, 0x18, 0xC0})
	drawCompass(dst, dir, ax, ay, render.ColGold)
}
