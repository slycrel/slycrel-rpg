package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// mapScene is the parchment overview: everything you have walked past, drawn
// two pixels to the tile. Unexplored ground stays blank, so the map doubles as
// a record of where you have actually been.
type mapScene struct {
	under Scene
	// canvas is rebuilt only when the explored set changes, which is rarely.
	canvas *ebiten.Image
	cursor int // index into the discovered POI list, for the legend
}

const mapScale = 2

func newMapScene(g *Game) *mapScene {
	m := &mapScene{under: g.Top()}
	m.render(g)
	return m
}

func (m *mapScene) render(g *Game) {
	w, h := world.Width*mapScale, world.Height*mapScale
	if m.canvas == nil {
		m.canvas = ebiten.NewImage(w, h)
	}
	m.canvas.Fill(color.RGBA{0x1A, 0x16, 0x12, 0xFF})

	for y := 0; y < world.Height; y++ {
		for x := 0; x < world.Width; x++ {
			if !g.World.IsExplored(x, y) {
				continue
			}
			render.Rect(m.canvas, float64(x*mapScale), float64(y*mapScale),
				mapScale, mapScale, mapColor(g.World.At(x, y)))
		}
	}
	for _, p := range g.World.POIs {
		if !p.Discovered {
			continue
		}
		x, y := float64(p.Pos.X*mapScale), float64(p.Pos.Y*mapScale)
		render.Rect(m.canvas, x-1, y-1, mapScale+2, mapScale+2, color.RGBA{0x10, 0x0C, 0x0A, 0xFF})
		render.Rect(m.canvas, x, y, mapScale, mapScale, poiColor(p.Kind))
	}
}

// mapColor is the flat parchment tint for each terrain.
func mapColor(t world.Terrain) color.RGBA {
	switch t {
	case world.Ocean:
		return color.RGBA{0x22, 0x38, 0x60, 0xFF}
	case world.Shallows:
		return color.RGBA{0x34, 0x58, 0x82, 0xFF}
	case world.River:
		return color.RGBA{0x3C, 0x74, 0xA8, 0xFF}
	case world.Beach:
		return color.RGBA{0xC8, 0xB4, 0x84, 0xFF}
	case world.Plains, world.Meadow:
		return color.RGBA{0x78, 0x9E, 0x58, 0xFF}
	case world.Forest:
		return color.RGBA{0x44, 0x72, 0x46, 0xFF}
	case world.Deepwood:
		return color.RGBA{0x2E, 0x56, 0x38, 0xFF}
	case world.Hills:
		return color.RGBA{0x8C, 0x86, 0x5C, 0xFF}
	case world.Mountain:
		return color.RGBA{0x8A, 0x86, 0x8A, 0xFF}
	case world.Peak:
		return color.RGBA{0xDC, 0xDE, 0xE4, 0xFF}
	case world.Swamp:
		return color.RGBA{0x54, 0x66, 0x46, 0xFF}
	case world.Desert:
		return color.RGBA{0xD2, 0xB6, 0x7C, 0xFF}
	case world.Wasteland:
		return color.RGBA{0x86, 0x6A, 0x5E, 0xFF}
	case world.Road:
		return color.RGBA{0xB4, 0x9C, 0x74, 0xFF}
	}
	return color.RGBA{0x40, 0x40, 0x40, 0xFF}
}

func poiColor(k world.POIKind) color.RGBA {
	switch k {
	case world.KindCapital:
		return color.RGBA{0xFF, 0xE0, 0x60, 0xFF}
	case world.KindTown, world.KindVillage:
		return color.RGBA{0xF0, 0xC0, 0x90, 0xFF}
	case world.KindCastle, world.KindTower:
		return color.RGBA{0xC0, 0xC8, 0xF0, 0xFF}
	case world.KindShrine:
		return color.RGBA{0xF0, 0xF0, 0xC0, 0xFF}
	case world.KindCamp:
		return color.RGBA{0xF0, 0x90, 0x40, 0xFF}
	case world.KindRuin:
		return color.RGBA{0xB0, 0xA8, 0x98, 0xFF}
	default:
		return color.RGBA{0xE0, 0x40, 0x50, 0xFF}
	}
}

func (m *mapScene) Update(g *Game) error {
	if Cancel() || Confirm() || inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.Pop()
	}
	return nil
}

func (m *mapScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x0C, 0x0A, 0x10, 0xFF})

	cw := float64(world.Width * mapScale)
	ch := float64(world.Height * mapScale)
	x := (render.ScreenW - cw) / 2
	y := 26.0

	ui.TitledPanel(dst, "the known world", x-6, y-6, cw+12, ch+12)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(x, y)
	dst.DrawImage(m.canvas, op)

	// The player, blinking so it is findable at a glance.
	if (g.Tick()/16)%2 == 0 {
		px := x + float64(g.Walk.Tile.X*mapScale) - 1
		py := y + float64(g.Walk.Tile.Y*mapScale) - 1
		render.Rect(dst, px, py, mapScale+2, mapScale+2, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
	}

	discovered := 0
	for _, p := range g.World.POIs {
		if p.Discovered {
			discovered++
		}
	}
	render.Text(dst, fmt.Sprintf("%d of %d locations found", discovered, len(g.World.POIs)),
		x, y+ch+12, render.ColInkDim)
	render.TextRight(dst, "gold = capital - pale = settlement - red = trouble",
		x+cw, y+ch+12, render.ColInkFaint)
	render.TextCenter(dst, "M or X to close", render.ScreenW/2, render.ScreenH-16, render.ColInkFaint)
}
