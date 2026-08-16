package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// localScene is the inside of a point of interest: a town, a dungeon, a hole
// in a hill with opinions. Same movement model as the overworld, different map
// and a lot more things to walk into.
type localScene struct {
	cam       render.Camera
	moveDelay int
	foeTimer  int
	steps     int
}

func newLocalScene(g *Game) *localScene {
	l := g.Local
	return &localScene{
		cam: render.Camera{
			W: float64(l.W * assetsys.TileSize), H: float64(l.H * assetsys.TileSize), Clamp: true,
		},
		foeTimer: 30,
	}
}

func (s *localScene) Update(g *Game) error {
	g.LocalWalk.Advance()
	s.cam.Update()

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Push(newPauseScene(g))
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) || inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.Push(newStatusScene(g))
		return nil
	}

	// Wandering foes drift on a slow tick so they are avoidable but present.
	s.foeTimer--
	if s.foeTimer <= 0 {
		s.foeTimer = 34
		g.Local.StepFoes(g.RNG)
	}

	if Confirm() && !g.LocalWalk.Moving() {
		// Interact with whatever is directly ahead.
		ahead := g.LocalWalk.Tile.Add(g.LocalWalk.Dir().Delta())
		if e := g.Local.EntityAt(ahead.X, ahead.Y); e != nil {
			g.interact(e)
			return nil
		}
		if e := g.Local.EntityAt(g.LocalWalk.Tile.X, g.LocalWalk.Tile.Y); e != nil {
			g.interact(e)
			return nil
		}
	}

	if s.moveDelay > 0 {
		s.moveDelay--
	}
	if !g.LocalWalk.Moving() && s.moveDelay == 0 {
		if d, ok := HeldDir(); ok {
			s.tryStep(g, d)
		}
	}
	return nil
}

func (s *localScene) tryStep(g *Game, d core.Dir) {
	next := g.LocalWalk.Tile.Add(d.Delta())

	// Walking into a blocking entity is how you engage it.
	if e := g.Local.EntityAt(next.X, next.Y); e != nil {
		g.LocalWalk.Step(g.LocalWalk.Tile, d)
		g.LocalWalk.t = 1
		s.moveDelay = 8
		switch e.Kind {
		case world.EFoe, world.EBoss, world.EExit:
			g.interact(e)
		}
		return
	}

	if !g.Local.Walkable(next.X, next.Y) {
		g.LocalWalk.Step(g.LocalWalk.Tile, d)
		g.LocalWalk.t = 1
		s.moveDelay = 6
		return
	}

	g.LocalWalk.Step(next, d)
	s.moveDelay = 0
	s.steps++
	g.sinceFight++

	// Interiors have their own ambush rate; towns do not.
	if !g.Local.POI.Kind.Settlement() && g.sinceFight > 6 && g.RNG.Intn(100) < 6 {
		g.sinceFight = 0
		mons := g.Data.PickMonsters(g.RNG, g.Local.Biome, g.Local.POI.Level, 1+g.RNG.Intn(2))
		if len(mons) > 0 {
			g.Push(newBattleScene(g, mons, "dark"))
		}
	}
}

// interact runs whatever the entity does.
func (g *Game) interact(e *world.Entity) {
	switch e.Kind {
	case world.EExit:
		g.Local = nil
		g.Pop()

	case world.ENPC:
		g.Say(e.Name, e.Line)

	case world.ESign:
		g.Say(e.Name, e.Line)

	case world.EAltar:
		g.Ask(e.Name, e.Line+"\n\nThe offering plate is right there. It is a large plate.",
			[]string{"Pray (25 coins)", "Leave it alone"}, func(g *Game, choice int) {
				if choice != 0 {
					return
				}
				if g.Player.Coins < 25 {
					g.Say("", "You do not have 25 coins. The god notices this and says nothing, which is worse.")
					return
				}
				g.Player.Coins -= 25
				e.Used = true
				g.Player.HP = g.Player.MaxHP
				g.Player.Psyche = g.Player.MaxPsyche
				g.Player.Faith++
				g.Say("", "Something old and largely retired takes an interest. You are made whole, and faintly indebted.")
			})

	case world.EChest:
		g.spend(e)
		g.openChest(e)

	case world.EShop:
		g.Push(newShopScene(g, e))

	case world.EInn:
		cost := 10 + g.Player.Level*4
		g.Ask(e.Name, fmt.Sprintf("A bed, a bolt on the door, and a landlord who does not ask questions.\n\nA night costs %d coins.", cost),
			[]string{"Sleep", "Decline"}, func(g *Game, choice int) {
				if choice != 0 {
					return
				}
				if g.Player.Coins < int64(cost) {
					g.Say("", "You are turned away, politely, by someone who has done it many times today.")
					return
				}
				g.Player.Coins -= int64(cost)
				g.Player.HP = g.Player.MaxHP
				g.Player.Psyche = g.Player.MaxPsyche
				g.Say("", "You sleep like something that has stopped worrying. You wake fully restored and slightly sticky.")
			})

	case world.EFoe, world.EBoss:
		g.spend(e)
		count := 1 + g.RNG.Intn(2)
		level := g.Local.POI.Level
		if e.Kind == world.EBoss {
			count = 1
			level += 3
		}
		mons := g.Data.PickMonsters(g.RNG, g.Local.Biome, level, count)
		if len(mons) == 0 {
			return
		}
		if e.Kind == world.EBoss {
			// Bosses are a scaled-up version of whatever lives here, with a
			// title, because a named enemy is worth three unnamed ones.
			m := mons[0]
			m.MaxHP = m.MaxHP * 2
			m.HP = m.MaxHP
			m.Name = "The " + m.Def.Name
			g.Local.POI.Cleared = true
		}
		g.Push(newBattleScene(g, mons, g.Local.POI.Name))
	}
}

// spend marks an interactable used, both on the live map and on the location
// itself, so it stays used after the interior is regenerated on a later visit.
func (g *Game) spend(e *world.Entity) {
	e.Used = true
	if g.Local != nil {
		g.Local.POI.MarkUsed(string(e.Kind), e.Pos)
	}
}

// openChest rolls contents scaled to the location's level band.
func (g *Game) openChest(e *world.Entity) {
	coins := int64(g.RNG.Between(8, 25) * core.Max(1, g.Local.POI.Level))
	g.Player.Coins += coins

	// One consumable, and a decent chance of something sellable.
	pool := []string{"Small Beer", "Field Poultice", "Bottled Nap", "Bitter Root", "Suspicious Pollen"}
	if g.Local.POI.Level >= 5 {
		pool = append(pool, "Physician's Draught", "Philosopher's Espresso")
	}
	body := fmt.Sprintf("%d coins.", coins)
	if it, ok := g.Data.Item(core.Pick(g.RNG, pool)); ok {
		it.Count = 1 + g.RNG.Intn(2)
		g.Player.AddItem(it)
		body += fmt.Sprintf("\n%s x%d.", it.Name, it.Count)
	}
	if g.RNG.Chance(0.4) {
		trinkets := []string{"Stolen Trinket", "Someone's Locket", "Cracked Crystal", "Hoard Coin"}
		if it, ok := g.Data.Item(core.Pick(g.RNG, trinkets)); ok {
			it.Count = 1
			g.Player.AddItem(it)
			body += "\n" + it.Name + "."
		}
	}
	g.Say(e.Name, e.Line+"\n\n"+body)
}

func (s *localScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x08, 0x08, 0x0E, 0xFF})
	if g.Local == nil {
		return
	}
	l := g.Local

	px, py := g.LocalWalk.Pixel()
	s.cam.CenterOn(px, py-hudH/2)
	ctx := &render.Ctx{Dst: dst, Cam: s.cam}

	const ts = assetsys.TileSize
	x0 := core.Max(0, int(s.cam.X)/ts-1)
	y0 := core.Max(0, int(s.cam.Y)/ts-1)
	x1 := core.Min(l.W-1, x0+render.ScreenW/ts+2)
	y1 := core.Min(l.H-1, y0+render.ScreenH/ts+2)

	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			t := l.At(tx, ty)
			if t == world.LVoid {
				continue
			}
			ctx.Tile(g.Assets.Get(t.Info().Tile), 0, tx, ty)
		}
	}

	// Entities, then the player, so the player draws over doorways.
	for _, e := range l.Entities {
		if e.Used && e.Kind != world.EExit {
			continue
		}
		drawEntity(g, ctx, e)
	}

	sp := g.Assets.Get(heroSpriteKey(g.Player, g.LocalWalk.Dir(), g.LocalWalk.Moving()))
	frame := g.Tick() / 6
	if !g.LocalWalk.Moving() {
		frame = g.Tick() / 14
	}
	ctx.World(sp, frame, px, py, false)

	s.drawHUD(g, dst)
}

func drawEntity(g *Game, ctx *render.Ctx, e *world.Entity) {
	const ts = assetsys.TileSize
	x := float64(e.Pos.X*ts) + ts/2
	y := float64(e.Pos.Y*ts) + ts

	// Only draw real art. Asking the registry rather than guessing from the
	// sprite's size is what keeps unresolved keys out of the world as magenta
	// boxes; they fall through to the markers below instead.
	if e.Sprite != "" && g.Assets.Has(e.Sprite) {
		ctx.World(g.Assets.Get(e.Sprite), g.Tick()/12, x, y, false)
		return
	}

	// Fallback markers, so an entity is never invisible just because its art
	// has not been chosen yet.
	ox, oy := ctx.Cam.Offset()
	bx, by := float64(e.Pos.X*ts)+ox, float64(e.Pos.Y*ts)+oy
	var c color.RGBA
	switch e.Kind {
	case world.EExit:
		c = color.RGBA{0xE0, 0xC0, 0x60, 0xFF}
		render.Frame(ctx.Dst, bx+2, by+2, ts-4, ts-4, c)
		return
	case world.EShop:
		c = color.RGBA{0x60, 0xA0, 0xE0, 0xFF}
	case world.EInn:
		c = color.RGBA{0xE0, 0x90, 0x50, 0xFF}
	case world.ENPC:
		c = color.RGBA{0xC8, 0xC8, 0xD8, 0xFF}
	case world.EChest:
		render.Rect(ctx.Dst, bx+2, by+6, ts-4, ts-8, color.RGBA{0x8C, 0x5E, 0x2C, 0xFF})
		render.Rect(ctx.Dst, bx+2, by+6, ts-4, 3, color.RGBA{0xC0, 0x90, 0x40, 0xFF})
		render.Rect(ctx.Dst, bx+6, by+9, 4, 3, color.RGBA{0xE8, 0xC8, 0x70, 0xFF})
		render.Frame(ctx.Dst, bx+2, by+6, ts-4, ts-8, color.RGBA{0x30, 0x20, 0x10, 0xFF})
		return
	case world.EAltar:
		render.Rect(ctx.Dst, bx+4, by+5, ts-8, ts-6, color.RGBA{0xB8, 0xB0, 0x90, 0xFF})
		render.Rect(ctx.Dst, bx+2, by+3, ts-4, 3, color.RGBA{0xE8, 0xE0, 0xB8, 0xFF})
		render.Frame(ctx.Dst, bx+4, by+5, ts-8, ts-6, color.RGBA{0x40, 0x3C, 0x30, 0xFF})
		return
	case world.ESign:
		c = color.RGBA{0x90, 0x80, 0x60, 0xFF}
	case world.EBoss:
		c = color.RGBA{0xE0, 0x40, 0x40, 0xFF}
	default:
		c = color.RGBA{0xA0, 0x50, 0x70, 0xFF}
	}
	render.Rect(ctx.Dst, bx+3, by+3, ts-6, ts-6, c)
	render.Frame(ctx.Dst, bx+3, by+3, ts-6, ts-6, color.RGBA{0, 0, 0, 0x80})
}

func (s *localScene) drawHUD(g *Game, dst *ebiten.Image) {
	// Naming what is directly ahead means interaction is never a guess.
	hint := "C character"
	ahead := g.LocalWalk.Tile.Add(g.LocalWalk.Dir().Delta())
	if e := g.Local.EntityAt(ahead.X, ahead.Y); e != nil && !e.Used {
		hint = "Z: " + e.Name
	}
	g.drawStatusBar(dst, g.Local.POI.Name, hint)
}
