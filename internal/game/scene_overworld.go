package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// hudH is the height of the status strip along the bottom of the screen.
const hudH = 44

// overworldScene is the continent view: walk anywhere, step on a location to
// enter it, get jumped in the tall grass.
type overworldScene struct {
	cam       render.Camera
	moveDelay int
	// idleTimer occasionally drops an ambient line into the log so standing
	// still is not silent.
	idleTimer int
}

func newOverworldScene(g *Game) *overworldScene {
	s := &overworldScene{}
	s.cam = render.Camera{
		W: world.Width * assetsys.TileSize, H: world.Height * assetsys.TileSize, Clamp: true,
	}
	s.idleTimer = 600
	return s
}

func (s *overworldScene) Update(g *Game) error {
	g.Walk.Advance()
	s.cam.Update()

	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.Push(newMapScene(g))
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Push(newPauseScene(g))
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) || inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.Push(newStatusScene(g))
		return nil
	}

	// Enter a location.
	if Confirm() && !g.Walk.Moving() {
		if poi := g.World.POIAt(g.Walk.Tile.X, g.Walk.Tile.Y); poi != nil {
			g.enterPOI(poi)
			return nil
		}
		g.Log.Add("%s", g.Write.Idle(g.RNG))
	}

	if s.moveDelay > 0 {
		s.moveDelay--
	}
	if !g.Walk.Moving() && s.moveDelay == 0 {
		if d, ok := HeldDir(); ok {
			s.tryStep(g, d)
		}
	}

	// The bed follows the ground underfoot. Ambience is a no-op when the key
	// has not changed, so setting it every tick costs nothing.
	g.Sound.Ambience(ambienceFor(g.World.At(g.Walk.Tile.X, g.Walk.Tile.Y).Biome()))

	// Ambient colour while wandering.
	s.idleTimer--
	if s.idleTimer <= 0 {
		s.idleTimer = 700 + g.RNG.Intn(900)
		g.Log.AddColor(render.ColInkDim, "%s", g.Write.Idle(g.RNG))
	}
	return nil
}

func (s *overworldScene) tryStep(g *Game, d core.Dir) {
	next := g.Walk.Tile.Add(d.Delta())
	if !g.World.Walkable(next.X, next.Y) {
		// Face the obstacle anyway so the sprite turns; it feels responsive.
		g.Walk.Step(g.Walk.Tile, d)
		g.Walk.t = 1
		s.moveDelay = 6
		return
	}

	g.Walk.Step(next, d)
	// Rough terrain costs a pause before the next step is accepted.
	s.moveDelay = g.World.At(next.X, next.Y).Info().Cost * 2
	g.World.Reveal(next, 6)
	g.sinceFight++

	if poi := g.World.POIAt(next.X, next.Y); poi != nil && !poi.Visited {
		poi.Visited = true
		g.Log.AddColor(render.ColGold, "%s - %s", poi.Name, poi.Tag)
	}

	// Encounter roll, with a short grace period after the last fight so you
	// are not immediately re-jumped while limping away.
	if g.sinceFight > 4 {
		if biome, hit := g.World.RollEncounter(g.RNG, next, g.Player.Level); hit {
			g.sinceFight = 0
			level := g.encounterLevel(next)
			count := 1
			if g.RNG.Chance(0.35) {
				count = 2
			}
			if g.RNG.Chance(0.10) {
				count = 3
			}
			mons := g.Data.PickMonsters(g.RNG, biome, level, count)
			if len(mons) > 0 {
				g.Push(newBattleScene(g, mons, g.World.At(next.X, next.Y).Name()))
			}
		}
	}
}

// ambienceFor maps a monster-table biome to a looping bed. Most of the
// overworld gets birdsong; only the deep woods earn their own.
func ambienceFor(biome string) string {
	if biome == "forest" {
		return "amb/forest"
	}
	return "amb/plains"
}

// encounterLevel blends the player's level with how far out they have wandered,
// so walking somewhere you should not be is dangerous immediately rather than
// only once the plot says so.
func (g *Game) encounterLevel(at core.Point) int {
	region := 1
	for _, p := range g.World.POIs {
		if d := p.Pos.Manhattan(at); d < 18 {
			if p.Level > region {
				region = p.Level
			}
		}
	}
	lv := (g.Player.Level*2 + region) / 3
	return core.Clamp(lv+g.RNG.Between(-1, 1), 1, 14)
}

// enterPOI builds the interior and switches scenes.
func (g *Game) enterPOI(poi *world.POI) {
	g.Local = world.BuildLocal(poi, g.Write)
	g.LocalWalk = walker{dur: 7}
	g.LocalWalk.Place(g.Local.Entry)
	g.Sound.Play("world/enter")
	g.Push(newLocalScene(g))
	g.Log.AddColor(render.ColGold, "You enter %s.", poi.Name)
}

func (s *overworldScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x0A, 0x0C, 0x14, 0xFF})

	px, py := g.Walk.Pixel()
	s.cam.CenterOn(px, py-hudH/2)
	ctx := &render.Ctx{Dst: dst, Cam: s.cam}

	// Only draw the tiles the camera can see, plus a one-tile margin.
	const ts = assetsys.TileSize
	x0 := int(s.cam.X)/ts - 1
	y0 := int(s.cam.Y)/ts - 1
	x1 := x0 + render.ScreenW/ts + 3
	y1 := y0 + render.ScreenH/ts + 3

	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			t := g.World.At(tx, ty)
			ctx.Tile(g.Assets.Get(t.Info().Tile), 0, tx, ty)
		}
	}

	// Locations, drawn as markers on top of terrain.
	for _, p := range g.World.POIs {
		if p.Pos.X < x0 || p.Pos.X > x1 || p.Pos.Y < y0 || p.Pos.Y > y1 {
			continue
		}
		drawPOIMarker(ctx, p)
	}

	// The player.
	sp := g.Assets.Get(heroSpriteKey(g.Player, g.Walk.Dir(), g.Walk.Moving()))
	frame := 0
	if g.Walk.Moving() {
		frame = g.Tick() / 6
	} else {
		frame = g.Tick() / 14
	}
	ctx.World(sp, frame, px, py, false)

	s.drawHUD(g, dst)
}

// drawPOIMarker paints a small procedural icon for a location. These are
// deliberately readable at a glance rather than pretty: silhouette first,
// detail later when curated building art replaces them.
func drawPOIMarker(ctx *render.Ctx, p *world.POI) {
	const ts = assetsys.TileSize
	ox, oy := ctx.Cam.Offset()
	x := float64(p.Pos.X*ts) + ox
	y := float64(p.Pos.Y*ts) + oy

	var body, roof color.RGBA
	switch p.Kind {
	case world.KindCapital:
		body, roof = color.RGBA{0xD8, 0xC0, 0x90, 0xFF}, color.RGBA{0xC0, 0x50, 0x40, 0xFF}
	case world.KindTown:
		body, roof = color.RGBA{0xC8, 0xB0, 0x84, 0xFF}, color.RGBA{0xA8, 0x48, 0x3C, 0xFF}
	case world.KindVillage:
		body, roof = color.RGBA{0xB8, 0xA0, 0x78, 0xFF}, color.RGBA{0x8C, 0x5A, 0x38, 0xFF}
	case world.KindCastle:
		body, roof = color.RGBA{0xB0, 0xB4, 0xC0, 0xFF}, color.RGBA{0x50, 0x58, 0x70, 0xFF}
	case world.KindTower:
		body, roof = color.RGBA{0x9C, 0x94, 0xB4, 0xFF}, color.RGBA{0x44, 0x3C, 0x64, 0xFF}
	case world.KindShrine:
		body, roof = color.RGBA{0xE4, 0xDC, 0xB0, 0xFF}, color.RGBA{0xD0, 0xA8, 0x40, 0xFF}
	case world.KindCamp:
		body, roof = color.RGBA{0xA8, 0x8C, 0x60, 0xFF}, color.RGBA{0xE0, 0x80, 0x30, 0xFF}
	case world.KindRuin:
		body, roof = color.RGBA{0x9C, 0x94, 0x84, 0xFF}, color.RGBA{0x60, 0x5C, 0x54, 0xFF}
	default: // dungeon, cave, oddity: a hole in the world
		body, roof = color.RGBA{0x30, 0x28, 0x30, 0xFF}, color.RGBA{0x14, 0x10, 0x18, 0xFF}
	}

	switch p.Kind {
	case world.KindDungeon, world.KindCave, world.KindOddity:
		render.Rect(ctx.Dst, x+3, y+5, 10, 9, body)
		render.Rect(ctx.Dst, x+5, y+7, 6, 7, roof)
	case world.KindTower:
		render.Rect(ctx.Dst, x+5, y+2, 6, 12, body)
		render.Rect(ctx.Dst, x+4, y+1, 8, 3, roof)
	case world.KindShrine:
		render.Rect(ctx.Dst, x+6, y+4, 4, 10, body)
		render.Rect(ctx.Dst, x+3, y+6, 10, 3, roof)
	case world.KindCamp:
		render.Rect(ctx.Dst, x+3, y+10, 10, 3, body)
		render.Rect(ctx.Dst, x+6, y+5, 4, 5, roof)
	case world.KindRuin:
		render.Rect(ctx.Dst, x+2, y+7, 3, 7, body)
		render.Rect(ctx.Dst, x+7, y+9, 3, 5, body)
		render.Rect(ctx.Dst, x+11, y+6, 3, 8, roof)
	default: // buildings
		render.Rect(ctx.Dst, x+2, y+7, 12, 7, body)
		render.Rect(ctx.Dst, x+1, y+3, 14, 4, roof)
		render.Rect(ctx.Dst, x+6, y+10, 4, 4, color.RGBA{0x40, 0x2C, 0x20, 0xFF})
	}
	render.Frame(ctx.Dst, x+0, y+0, ts, ts, color.RGBA{0, 0, 0, 0x30})
}

func (s *overworldScene) drawHUD(g *Game, dst *ebiten.Image) {
	hint := "M map - C character"
	if poi := g.World.POIAt(g.Walk.Tile.X, g.Walk.Tile.Y); poi != nil {
		hint = "Z to enter"
	}
	g.drawStatusBar(dst, g.World.Describe(g.Walk.Tile), hint)
}
