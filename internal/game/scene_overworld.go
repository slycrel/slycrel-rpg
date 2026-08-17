package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/sky"
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
	g.follow.Advance()
	s.cam.Update()

	// Somebody in the company may have something to say about their own life.
	// Checked before input so the box goes up on the step that earned it, and
	// the frame ends there so nothing else can be pushed on top of it.
	if g.serviceThreads() {
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.Push(newMapScene(g))
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Push(newPauseScene(g))
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		g.Push(newHelpScene(g))
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyJ) {
		g.Push(newQuestScene(g))
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
		g.Walk.Face(d)
		s.moveDelay = 6
		return
	}

	from := g.Walk.Tile
	g.Walk.Step(next, d)
	g.follow.Step(from)
	// Rough terrain costs a pause before the next step is accepted.
	s.moveDelay = g.World.At(next.X, next.Y).Info().Cost * 2

	// A step is what time is measured in, so this is where the clock moves.
	g.Clock.Tick(1)
	// And how far it reveals depends on whether you can see. Six was the
	// number before there was a sky; it is still the number at noon.
	g.World.Reveal(next, sky.Sight(g.Clock.Phase(), g.weatherAt(next)))
	g.sinceFight++
	g.travelWithCompany()

	if poi := g.World.POIAt(next.X, next.Y); poi != nil && !poi.Visited {
		poi.Visited = true
		// Renown is earned by being seen, not by doing anything. Walking into
		// a town for the first time is the cheapest way there is to be seen,
		// and a player who stays out of them stays unplaceable — which is the
		// corner the whole two-axis idea exists to make reachable.
		if poi.Kind.Settlement() {
			g.Player.Renown++
		}
		g.Log.AddColor(render.ColGold, "%s - %s", poi.Name, poi.Tag)
	}

	// Encounter roll, with a short grace period after the last fight so you
	// are not immediately re-jumped while limping away.
	if g.sinceFight > 4 {
		// The sky multiplies the terrain's own roll rather than replacing it,
		// so somewhere quiet stays proportionally quiet after dark: the road
		// home does not become the swamp because the sun went down.
		if biome, hit := g.World.RollEncounter(g.RNG, next, g.Player.Level,
			sky.Prowl(g.Clock.Phase(), g.weatherAt(next))); hit {
			g.sinceFight = 0
			level := g.encounterLevel(next)
			count := 1
			if g.RNG.Chance(0.35) {
				count = 2
			}
			if g.RNG.Chance(0.10) {
				count = 3
			}
			mons := g.Data.PickMonsters(g.RNG, biome, level, g.encounterSize(count))
			if len(mons) > 0 {
				g.autosave()
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

// homeRadius is how far the ground around the capital stays a place you can
// learn the game in.
//
// The danger formula reads the level of every location within eighteen tiles,
// and the capital does not get a clear eighteen tiles to itself — a level-four
// ruin sixteen tiles out was enough to hand a level-one character encounters at
// level three, in whatever biome the noise happened to put there. Measured
// across eight seeds, a fresh character was meeting +2 fights in hills and
// mountains, which the DANGER table rates at 14 to 36 per cent death with the
// gear they can afford on the first morning.
//
// So there is a home region now. Inside it the roll cannot exceed the player's
// own level, which does not make it safe — an on-level fight is still a fight,
// and every other rule is unchanged — it makes it *predictable*, which is the
// thing the opening was missing.
const homeRadius = 14

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
	lv = core.Clamp(lv+g.RNG.Between(-1, 1), 1, 14)

	// What is out at night is a level meaner. Added before the home clamp
	// below, so the ground around the capital is still the ground around the
	// capital after dark — the whole point of a home region is that it is
	// predictable, and a rule that quietly suspended itself at night would be
	// the least predictable thing in the game.
	lv = core.Clamp(lv+g.Clock.Phase().LevelShift(), 1, 14)

	// Close to home, nothing is above your weight. The cap lifts as you level,
	// so the home region stops being a special case on its own rather than
	// needing a second rule to retire it.
	if at.Manhattan(g.World.Start) <= homeRadius {
		lv = core.Min(lv, g.Player.Level)
	}
	return lv
}

// enterPOI builds the interior and switches scenes.
func (g *Game) enterPOI(poi *world.POI) {
	g.Local = world.BuildLocal(poi, g.Write)
	g.LocalWalk = core.NewWalker(7)
	g.LocalWalk.Place(g.Local.Entry)
	// The company comes in through the gate stacked on the hero and spreads
	// back out over the first few steps.
	g.reformLines()
	g.localFollow.Place(g.Local.Entry)
	g.Sound.Play("world/enter")
	if idx := g.poiIndex(poi); idx >= 0 {
		g.noteQuestProgress(g.Quests.OnEnteredPOI(idx))
		g.threadsOnEnteringPOI(idx)
	}
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

	ground := g.ground()
	ox, oy := s.cam.Offset()
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			ground.Draw(dst, float64(tx*ts)+ox, float64(ty*ts)+oy, tx, ty, g.materialAt)
		}
	}

	// Scenery sits between the ground and everything that walks on it.
	g.drawDecor(dst, s.cam, x0, y0, x1, y1)

	// Locations, drawn as markers on top of terrain.
	for _, p := range g.World.POIs {
		if p.Pos.X < x0 || p.Pos.X > x1 || p.Pos.Y < y0 || p.Pos.Y > y1 {
			continue
		}
		drawPOIMarker(ctx, p)
	}

	// The company, then the player, so nothing ever covers the character you
	// are steering.
	g.drawFollowers(ctx, g.follow)

	sp := g.Assets.Get(heroSpriteKey(g.Player, g.Walk.Dir(), g.Walk.Moving()))
	frame := 0
	if g.Walk.Moving() {
		frame = g.Tick() / 6
	} else {
		frame = g.Tick() / 14
	}
	ctx.Shadow(px, py)
	ctx.World(sp, frame, px, py, false)

	// The light and the weather, over the world and under the interface.
	g.drawSky(dst, g.weatherAt(g.Walk.Tile), false)

	s.drawHUD(g, dst)
}

// drawPOIMarker paints the icon for a location. These are drawn rather than
// sourced: the building art in the bundle is 300-500px hero sprites meant for a
// zoomed-in scene, and scaling one down to a 16px overworld cell is mush. At
// this size a location wants a silhouette, not a portrait.
func drawPOIMarker(ctx *render.Ctx, p *world.POI) {
	const ts = assetsys.TileSize
	ox, oy := ctx.Cam.Offset()
	x := float64(p.Pos.X*ts) + ox
	y := float64(p.Pos.Y*ts) + oy

	var body, roof color.RGBA
	switch p.Kind {
	case world.KindCapital:
		body, roof = color.RGBA{0xE4, 0xCF, 0x9E, 0xFF}, color.RGBA{0xC8, 0x4A, 0x3C, 0xFF}
	case world.KindTown:
		body, roof = color.RGBA{0xD4, 0xBC, 0x8C, 0xFF}, color.RGBA{0xB0, 0x42, 0x36, 0xFF}
	case world.KindVillage:
		body, roof = color.RGBA{0xC0, 0xA8, 0x7C, 0xFF}, color.RGBA{0x94, 0x5E, 0x38, 0xFF}
	case world.KindCastle:
		body, roof = color.RGBA{0xBC, 0xC0, 0xCC, 0xFF}, color.RGBA{0x56, 0x5E, 0x78, 0xFF}
	case world.KindTower:
		body, roof = color.RGBA{0xA8, 0xA0, 0xC0, 0xFF}, color.RGBA{0x4A, 0x42, 0x6C, 0xFF}
	case world.KindShrine:
		body, roof = color.RGBA{0xEC, 0xE4, 0xB8, 0xFF}, color.RGBA{0xD8, 0xB0, 0x48, 0xFF}
	case world.KindCamp:
		body, roof = color.RGBA{0xB0, 0x94, 0x66, 0xFF}, color.RGBA{0xE8, 0x88, 0x34, 0xFF}
	case world.KindRuin:
		body, roof = color.RGBA{0xA6, 0x9E, 0x8C, 0xFF}, color.RGBA{0x68, 0x64, 0x5C, 0xFF}
	default: // dungeon, cave, oddity: a hole in the world
		body, roof = color.RGBA{0x3A, 0x30, 0x3A, 0xFF}, color.RGBA{0x12, 0x0E, 0x16, 0xFF}
	}
	shade := scale(body, 0.72)
	outline := color.RGBA{0x1C, 0x16, 0x14, 0xC0}

	// A contact shadow first, so the icon reads as standing on the terrain.
	render.Rect(ctx.Dst, x+3, y+13, 10, 2, color.RGBA{0, 0, 0, 0x50})

	switch p.Kind {
	case world.KindDungeon, world.KindCave, world.KindOddity:
		render.Rect(ctx.Dst, x+2, y+5, 12, 9, body)
		render.Rect(ctx.Dst, x+4, y+7, 8, 7, roof)
		render.Frame(ctx.Dst, x+2, y+5, 12, 9, outline)
	case world.KindTower:
		render.Rect(ctx.Dst, x+5, y+3, 6, 11, body)
		render.Rect(ctx.Dst, x+8, y+3, 3, 11, shade)
		render.Rect(ctx.Dst, x+4, y+1, 8, 3, roof)
		render.Frame(ctx.Dst, x+5, y+3, 6, 11, outline)
	case world.KindShrine:
		render.Rect(ctx.Dst, x+6, y+5, 4, 9, body)
		render.Rect(ctx.Dst, x+3, y+3, 10, 3, roof)
		render.Frame(ctx.Dst, x+3, y+3, 10, 3, outline)
	case world.KindCamp:
		render.Rect(ctx.Dst, x+4, y+11, 8, 2, body)
		render.Rect(ctx.Dst, x+6, y+4, 2, 8, roof)
		render.Rect(ctx.Dst, x+8, y+6, 2, 6, shade)
	case world.KindRuin:
		render.Rect(ctx.Dst, x+2, y+6, 3, 8, body)
		render.Rect(ctx.Dst, x+7, y+9, 3, 5, shade)
		render.Rect(ctx.Dst, x+11, y+5, 3, 9, roof)
	default: // settlements: a gabled house
		render.Rect(ctx.Dst, x+3, y+7, 10, 7, body)
		render.Rect(ctx.Dst, x+9, y+7, 4, 7, shade) // sunlit from the left
		render.Rect(ctx.Dst, x+2, y+3, 12, 4, roof)
		render.Rect(ctx.Dst, x+2, y+3, 12, 1, scale(roof, 1.25))
		render.Rect(ctx.Dst, x+7, y+10, 3, 4, color.RGBA{0x38, 0x26, 0x1C, 0xFF})
		render.Frame(ctx.Dst, x+3, y+7, 10, 7, outline)
	}
}

// scale multiplies a colour's channels, clamping at full.
func scale(c color.RGBA, f float64) color.RGBA {
	ch := func(v uint8) uint8 {
		n := float64(v) * f
		if n > 255 {
			n = 255
		}
		return uint8(n)
	}
	return color.RGBA{ch(c.R), ch(c.G), ch(c.B), c.A}
}

func (s *overworldScene) drawHUD(g *Game, dst *ebiten.Image) {
	hint := "M map - H help"
	if poi := g.World.POIAt(g.Walk.Tile.X, g.Walk.Tile.Y); poi != nil {
		hint = "Z to enter"
	}
	g.drawStatusBar(dst, g.World.Describe(g.Walk.Tile), hint)
}
