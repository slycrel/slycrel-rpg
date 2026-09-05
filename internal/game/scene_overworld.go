package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/ui"
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
	// mini is the corner map. Held on the scene rather than on the Game so its
	// texture goes away with the screen that shows it.
	mini minimap

	// wanderers are the rolled encounters standing in the grass. Held on the
	// scene and not on the Game because they must never be saved: they are a
	// consequence of a roll, not a fact about the world, and the save format is
	// seed plus deltas.
	//
	// A slice, and that is a fix rather than a feature. There used to be one
	// slot, on the reasoning that it kept the encounter rate exactly what it
	// had always been — but it did not, it switched the roll *off*: no new
	// encounter could be rolled while a creature was out, and a creature is out
	// for up to WanderLife of its own steps whether or not it ever comes near
	// you. Measured over two hundred thousand steps, 58 to 67 per cent of all
	// walking happened with the encounter system disabled, and a third of the
	// creatures that caused it drifted off without ever being met. That is what
	// "few and far between" was: not a rate, a gate.
	wanderers  []wanderer
	wanderTick int
}

// wanderCap is how many creatures may be out at once, and it is the dial.
//
// Measured over two hundred thousand walking steps, as steps between fights at
// levels one and three:
//
//	cap 1   97 / 84    58-67% of steps had a creature out
//	cap 2   57 / 47    69-79%
//	cap 3   46 / 35    72-83%
//
// Two, because the jump from one is most of the available gain and the point of
// the visible model is that an encounter is a thing you can see coming and
// decide about. At three, something is on screen for better than four fifths of
// the walking and the ground stops reading as ground — that is not a decision
// any more, it is weather. If the world still feels empty, this is the constant
// to move, and the row above says what moving it costs.
const wanderCap = 2

// wanderer is one rolled encounter and what it is standing in, kept together so
// the fight it becomes is the fight that was rolled for the tile it appeared on.
type wanderer struct {
	w   *world.Wanderer
	enc gamedata.Encounter
	in  string // terrain name, for the battle screen's title
}

func newOverworldScene(g *Game) *overworldScene {
	s := &overworldScene{}
	s.cam = render.Camera{
		W: world.Width * assetsys.TileSize, H: world.Height * assetsys.TileSize, Clamp: true,
		ViewH: render.ScreenH - hudH,
	}
	s.idleTimer = 600
	// Wherever the hero is when this screen appears counts as already arrived.
	// A new run starts in the capital's own tile and a loaded one can be stood
	// anywhere, so a zero value here would walk the player straight through a
	// door they did not open.
	g.arrived = g.Walk.Tile
	return s
}

func (s *overworldScene) Update(g *Game) error {
	g.Walk.Advance()
	g.follow.Advance()
	s.cam.Update()

	// Somebody in the company may have something to say about their own life.
	// Checked before input so the box goes up on the step that earned it, and
	// the frame ends there so nothing else can be pushed on top of it.
	// The long story first: a leg is the thing the player went somewhere for,
	// and a companion's beat is a thing that happened on the way.
	if g.serviceSagas() {
		return nil
	}
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

	// Walking onto a location walks in. Z was a second thing to do after the
	// thing you had already done: nobody steps onto a town's tile by accident,
	// the marker is named on the ground in front of them now, and a confirm
	// key that only ever means yes is a key that only ever gets pressed.
	//
	// Held until the step animation lands, so the hero is standing in the
	// gateway rather than halfway to it when the screen changes.
	if !g.Walk.Moving() && g.Walk.Tile != g.arrived {
		g.arrived = g.Walk.Tile
		if poi := g.World.POIAt(g.Walk.Tile.X, g.Walk.Tile.Y); poi != nil {
			g.enterPOI(poi)
			return nil
		}
	}

	// Z still enters, which matters for exactly one case: you have just walked
	// out and want back in without stepping off the doorstep first.
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

	s.stepWanderer(g)

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
	if g.sinceFight > 4 && len(s.wanderers) < wanderCap {
		// The sky multiplies the terrain's own roll rather than replacing it,
		// so somewhere quiet stays proportionally quiet after dark: the road
		// home does not become the swamp because the sun went down.
		if biome, hit := g.World.RollEncounter(g.RNG, next, g.Player.Level,
			sky.Prowl(g.Clock.Phase(), g.weatherAt(next))); hit {
			level := g.encounterLevel(next)
			count := 1
			if g.RNG.Chance(0.35) {
				count = 2
			}
			if g.RNG.Chance(0.10) {
				count = 3
			}
			// The encounter is rolled either way, because a boon still needs
			// something to stand there and something to become if a mystery
			// turns out badly. What the omen decides is which of those happens
			// when the two of you meet.
			enc := g.Data.PickEncounter(g.RNG, biome, level, g.encounterSize(count))
			if len(enc.Monsters) > 0 {
				s.spawnWanderer(g, next, enc, rollOmen(g.RNG))
			}
		}
	}
}

// spawnWanderer turns a hit on the encounter roll into something standing in
// the grass, rather than into a battle screen.
//
// The kind comes off the first monster in the encounter, which is the one the
// composition is named for, so the silhouette is telling the truth about what
// is in there. If there is nowhere to stand — a beach, a spit of land, a tile
// ringed by water — the roll simply does not become a fight. That is the one
// fight the visible model gives up that the invisible one would have had, and
// it is a fair price for never lying about what is coming.
func (s *overworldScene) spawnWanderer(g *Game, at core.Point, enc gamedata.Encounter, omen world.Omen) {
	kind := string(enc.Monsters[0].Def.Kind)
	w := g.World.SpawnWanderer(g.RNG, at, kind, omen)
	if w == nil {
		return
	}
	s.wanderers = append(s.wanderers, wanderer{
		w: w, enc: enc, in: g.World.At(at.X, at.Y).Name(),
	})
}

// stepWanderer moves the creature and decides whether it has reached you.
//
// It moves on its own slow tick rather than per player step, so standing still
// is not safety — a thing that only advanced when you did would be a puzzle
// about not moving rather than a creature.
func (s *overworldScene) stepWanderer(g *Game) {
	// Standing on one is a fight before anything else happens, so a creature
	// that arrived last tick does not get a free move first.
	for i := range s.wanderers {
		if s.wanderers[i].w.Pos == g.Walk.Tile {
			s.startWanderFight(g, i)
			return
		}
	}
	s.wanderTick--
	if s.wanderTick > 0 {
		return
	}
	s.wanderTick = 20

	// One pass, dropping the ones that gave up. Walked backwards so removing
	// an element cannot skip the next one.
	for i := len(s.wanderers) - 1; i >= 0; i-- {
		if !s.wanderers[i].w.Step(g.RNG, g.World, g.Walk.Tile) {
			s.wanderers = append(s.wanderers[:i], s.wanderers[i+1:]...)
		}
	}
	for i := range s.wanderers {
		if s.wanderers[i].w.Pos == g.Walk.Tile {
			s.startWanderFight(g, i)
			return
		}
	}
}

// startWanderFight is what happens when the two of you meet, which is not
// always a fight any more.
//
// The others stay where they are. They are still out there, which is the whole
// argument for letting more than one exist: walking away from a fight into the
// arms of the thing behind it is a consequence the visible model can have and
// the invisible one could not.
func (s *overworldScene) startWanderFight(g *Game, i int) {
	if i < 0 || i >= len(s.wanderers) {
		return
	}
	it := s.wanderers[i]
	s.wanderers = append(s.wanderers[:i], s.wanderers[i+1:]...)
	g.sinceFight = 0

	// A mystery is not decided until it is reached, which is the only thing
	// that makes walking to one a decision rather than a collection.
	omen := it.w.Omen
	if omen == world.OmenMystery {
		omen = resolveMystery(g.RNG)
	}
	if omen == world.OmenBoon {
		g.grantBoon(g.RNG, it.in)
		return
	}
	g.Push(newBattleScene(g, it.enc, it.in))
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
const homeRadius = world.HomeRadius

// encounterLevel blends the player's level with how far out they have wandered,
// so walking somewhere you should not be is dangerous immediately rather than
// only once the plot says so.
func (g *Game) encounterLevel(at core.Point) int {
	lv := (g.Player.Level*2 + g.World.RegionLevel(at)) / 3
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
	// Whatever put you here, you are here: the doorstep is a tile you have
	// arrived at, so walking back out of it does not walk straight back in.
	g.arrived = poi.Pos
	g.floor = 0
	g.Local = world.BuildLocal(poi, g.Write, g.floor)
	g.LocalWalk = core.NewWalker(7)
	g.LocalWalk.Place(g.Local.Entry)
	// The company comes in through the gate stacked on the hero and spreads
	// back out over the first few steps.
	g.reformLines()
	g.localFollow.Place(g.Local.Entry)
	g.Sound.Play("world/enter")
	if idx := g.poiIndex(poi); idx >= 0 {
		g.noteQuestProgress(g.Quests.OnEnteredPOI(idx))
		g.sagasOnEnteringPOI(idx)
		g.threadsOnEnteringPOI(idx)
	}
	g.Push(newLocalScene(g))
	g.Log.AddColor(render.ColGold, "You enter %s.", poi.Name)
	// Everyone else has their own errands. See companyShops.
	g.companyShops()
}

func (s *overworldScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x0A, 0x0C, 0x14, 0xFF})

	px, py := g.Walk.Pixel()
	s.cam.CenterOn(px, py)
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
		drawPOIMarker(ctx, g.Assets, p)
	}

	// The creatures the rolls produced, drawn with the things that stand on the
	// ground rather than after the sky, so the rain falls on them too.
	for _, it := range s.wanderers {
		w := it.w
		if w.Pos.X < x0 || w.Pos.X > x1 || w.Pos.Y < y0 || w.Pos.Y > y1 {
			continue
		}
		if sp := g.Assets.Get("wild/" + w.Kind); sp != nil {
			ctx.World(sp, 0, float64(w.Pos.X*ts)+ts/2, float64(w.Pos.Y*ts)+ts, false)
		}
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

	// Names over the tint, so a place is still readable after dark.
	s.drawLabels(g, dst, x0, y0, x1, y1)

	// And what each thing in the grass is worth walking to, over the tint for
	// the same reason: the mark is the one piece of information the player has
	// before they commit, and a mark you cannot see in the rain is a mark for
	// the weather you least want to be surprised in.
	s.drawOmens(g, dst, x0, y0, x1, y1)

	// The corner map over the weather and under the status strip, which is the
	// order they mean: it is interface, and the strip is the interface it
	// belongs to.
	s.mini.draw(g, dst)

	s.drawHUD(g, dst)
}

// drawOmens paints the mark over each thing standing in the world.
//
// Over the creature rather than instead of it. The playthrough that produced
// this asked for one icon per kind of event and also said it liked seeing
// monsters on the map, and those are not in tension once the two jobs are
// separated: the silhouette is the world having things in it, and the mark is
// the information. A creature you can see is a creature you can walk around;
// the mark is what tells you whether you want to.
func (s *overworldScene) drawOmens(g *Game, dst *ebiten.Image, x0, y0, x1, y1 int) {
	const ts = assetsys.TileSize
	ox, oy := s.cam.Offset()
	for _, it := range s.wanderers {
		w := it.w
		if w.Pos.X < x0 || w.Pos.X > x1 || w.Pos.Y < y0 || w.Pos.Y > y1 {
			continue
		}
		rows, col, ok := omenMark(w.Omen, g.Tick())
		if !ok {
			continue
		}
		// Above the artwork rather than above the tile, which is the same
		// measurement the attention star makes and for the same reason: the
		// creature silhouettes run from thirteen pixels to twenty-five, so a
		// fixed offset sits on the tall ones' heads.
		top := float64(w.Pos.Y*ts + ts)
		if sp := g.Assets.Get("wild/" + w.Kind); sp != nil {
			top -= float64(sp.H - sp.Head)
		} else {
			top -= ts
		}
		x := float64(w.Pos.X*ts) + ox + (ts-7)/2
		y := top + oy - 6 + starBob[(g.Tick()/8)%len(starBob)]
		drawGlyph(dst, rows, x+2, y+2, color.RGBA{0x10, 0x0C, 0x14, 0xD8})
		drawGlyph(dst, rows, x, y, col)
	}
}

// poiLabelRadius is how close a location has to be before it says its name, in
// tiles. Wide enough to name the thing you are walking toward and everything
// on the way, narrow enough that a road between two towns is not a wall of
// text.
const poiLabelRadius = 7

// drawLabels floats the name of every nearby location.
//
// The overworld's markers are silhouettes — a gabled house, a hole in a hill —
// which say what *kind* of place something is and nothing about which one. The
// name was already known to the game and printed to the log once, on the step
// that entered it, which is exactly one step too late to be a decision.
func (s *overworldScene) drawLabels(g *Game, dst *ebiten.Image, x0, y0, x1, y1 int) {
	const ts = assetsys.TileSize
	ox, oy := s.cam.Offset()
	px, py := g.Walk.Pixel()
	hx, hy := px+ox, py+oy
	here := g.Walk.Tile
	for _, p := range g.World.POIs {
		if p.Pos.X < x0 || p.Pos.X > x1 || p.Pos.Y < y0 || p.Pos.Y > y1 {
			continue
		}
		if !g.World.IsExplored(p.Pos.X, p.Pos.Y) {
			continue
		}
		if core.Abs(p.Pos.X-here.X) > poiLabelRadius || core.Abs(p.Pos.Y-here.Y) > poiLabelRadius {
			continue
		}
		// Gold within one step, which under auto-entry is the last moment the
		// player can still choose not to. The tagline stays out of it: it runs
		// to seventy characters and a plate that wide over the terrain is a
		// billboard, not a label.
		col := render.ColInkDim
		if p.Pos.Manhattan(here) <= 1 {
			col = render.ColGold
		}
		lines := []string{p.Name}
		cx, by := float64(p.Pos.X*ts)+ts/2+ox, float64(p.Pos.Y*ts)+oy+2
		ui.Tag(dst, lines, clearOfHero(lines, cx, by, hx, hy), by, col)
	}
}

// poiArt is the sourced marker for each kind of location, cut by
// `assetpipe poi`. A kind with no entry, or one whose art is not in the
// manifest, falls through to the rectangles below.
var poiArt = map[world.POIKind]string{
	world.KindCapital: "poi/capital",
	world.KindTown:    "poi/town",
	world.KindVillage: "poi/village",
	world.KindCastle:  "poi/castle",
	world.KindTower:   "poi/tower",
	world.KindShrine:  "poi/shrine",
	world.KindCamp:    "poi/camp",
	world.KindRuin:    "poi/ruin",
	world.KindOddity:  "poi/oddity",
	// Dungeons and caves are deliberately absent: see cmd/assetpipe/poi.go.
	// They fall through to the procedural hole, which is drawn to read on any
	// ground and does.
}

// drawPOIMarker paints the icon for a location.
//
// It used to draw its own rectangles, and the reason stood for a long time:
// the building art in the bundle is 300-500px hero sprites meant for a
// zoomed-in scene, and scaling one down to a 16px overworld cell is mush. That
// was a fact about the packs that had been extracted, not about the bundle —
// `pixelartrogue-likerpg` draws its settlements natively at overworld scale.
//
// The markers stand taller than their tile, which is deliberate: `Ctx.World`
// anchors a sprite on its base exactly as it does a character, so a castle
// rises off the square it occupies instead of being squeezed into it. The
// rectangles remain underneath as the fallback, because a marker that fails to
// a coloured box is better than one that fails to nothing.
func drawPOIMarker(ctx *render.Ctx, assets *assetsys.Registry, p *world.POI) {
	const ts = assetsys.TileSize

	if key := poiArt[p.Kind]; key != "" && assets.Has(key) {
		ctx.World(assets.Get(key), 0,
			float64(p.Pos.X*ts)+ts/2, float64(p.Pos.Y*ts)+ts, false)
		return
	}

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
	// The location under your feet is named on the map beside you rather than
	// in this corner, which the compass has a better use for.
	g.drawStatusBar(dst, g.World.Describe(g.Walk.Tile), "M map - H help")
}
