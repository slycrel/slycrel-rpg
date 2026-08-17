package game

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
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
	g.localFollow.Advance()
	s.cam.Update()

	if g.serviceThreads() {
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

	if g.Local.POI.Kind.Settlement() {
		g.Sound.Ambience("amb/town")
	} else {
		g.Sound.Ambience("amb/dungeon")
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
		g.LocalWalk.Face(d)
		s.moveDelay = 8
		switch e.Kind {
		case world.EFoe, world.EBoss, world.EExit:
			g.interact(e)
		}
		return
	}

	if !g.Local.Walkable(next.X, next.Y) {
		g.LocalWalk.Face(d)
		s.moveDelay = 6
		return
	}

	from := g.LocalWalk.Tile
	g.LocalWalk.Step(next, d)
	g.localFollow.Step(from)
	s.moveDelay = 0
	s.steps++
	g.sinceFight++

	// Interiors have their own ambush rate; towns do not.
	if !g.Local.POI.Kind.Settlement() && g.sinceFight > 6 && g.RNG.Intn(100) < 6 {
		g.sinceFight = 0
		mons := g.Data.PickMonsters(g.RNG, g.Local.Biome, g.Local.POI.Level, g.encounterSize(1+g.RNG.Intn(2)))
		if len(mons) > 0 {
			g.autosave()
			g.Push(newBattleScene(g, mons, "dark"))
		}
	}
}

// interact runs whatever the entity does.
func (g *Game) interact(e *world.Entity) {
	switch e.Kind {
	case world.EExit:
		g.Sound.Play("world/enter")
		g.Local = nil
		g.Pop()

	case world.ENPC:
		g.talkTo(e)

	case world.ERecruit:
		g.offerRecruit(e)

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
				g.restParty()
				g.Player.Faith++
				g.Say("", "Something old and largely retired takes an interest. You are made whole, and faintly indebted.")
			})

	case world.EChest:
		g.spend(e)
		g.openChest(e)

	case world.EShop:
		g.Push(newShopScene(g, e))

	case world.EInn:
		// A bed each. The party is restored together because a companion left
		// to sleep in the road would be a rule nobody wants to remember.
		beds := len(g.Party())
		cost := (10 + g.Player.Level*4) * beds
		body := "A bed, a bolt on the door, and a landlord who does not ask questions."
		if beds > 1 {
			body = fmt.Sprintf("%s\n\n%d beds, since you brought people.", body, beds)
		}
		g.Ask(e.Name, fmt.Sprintf("%s\n\nA night costs %d coins.", body, cost),
			[]string{"Sleep", "Decline"}, func(g *Game, choice int) {
				if choice != 0 {
					return
				}
				if g.Player.Coins < int64(cost) {
					g.Say("", "You are turned away, politely, by someone who has done it many times today.")
					return
				}
				g.Player.Coins -= int64(cost)
				g.restParty()
				g.Say("", "You sleep like something that has stopped worrying. You wake fully restored and slightly sticky.")
			})

	case world.EFoe, world.EBoss:
		g.spend(e)
		count := g.encounterSize(1 + g.RNG.Intn(2))
		level := g.Local.POI.Level
		if e.Kind == world.EBoss {
			// A boss stands alone whatever you brought with you. It is the
			// point of the room, and two of them is a different room.
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
			if idx := g.currentPOIIndex(); idx >= 0 {
				g.noteQuestProgress(g.Quests.OnPOICleared(idx))
			}
		}
		g.autosave()
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
	g.Sound.Play("world/chest")

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
	if find, ok := g.rollAffixedGear(); ok {
		g.SayThen(e.Name, e.Line+"\n\n"+body, func(g *Game) { g.offerFind(find) })
		return
	}
	g.Say(e.Name, e.Line+"\n\n"+body)
}

// find is a named piece of equipment turned up in a chest. Exactly one of the
// two is set.
type find struct {
	weapon *model.Weapon
	armor  *model.Armor
}

// rollAffixedGear is a chest's chance of holding a named piece of equipment.
//
// Affixed gear is deliberately not sold: a shop is where you buy the tier you
// can afford, and a chest is where you find the thing with a name. That is the
// whole reason to open one rather than count the coins and move on. Whether it
// is an upgrade is genuinely a question, because every affix takes as well as
// gives.
func (g *Game) rollAffixedGear() (find, bool) {
	if g.Local == nil || !g.RNG.Chance(0.28) {
		return find{}, false
	}
	return g.rollAffixedGearOfTier(core.Clamp(1+g.Local.POI.Level/3, 1, 5))
}

// rollAffixedGearOfTier is the same roll banded to an explicit tier, so a
// creature can leave something behind as well as a chest.
func (g *Game) rollAffixedGearOfTier(tier int) (find, bool) {
	affix, ok := g.Data.PickAffix(g.RNG, tier)
	if !ok {
		return find{}, false
	}

	// Only gear whose name is free of a flourish can take one.
	weapons, armors := g.Data.StockFor(tier)
	weapons = slices.DeleteFunc(weapons, func(w model.Weapon) bool { return !model.Affixable(w.Name) })
	armors = slices.DeleteFunc(armors, func(a model.Armor) bool { return !model.Affixable(a.Name) })

	if g.RNG.Chance(0.5) && len(weapons) > 0 {
		w := core.Pick(g.RNG, weapons)
		w.Affix = &affix
		return find{weapon: &w}, true
	}
	if len(armors) == 0 {
		return find{}, false
	}
	a := core.Pick(g.RNG, armors)
	a.Affix = &affix
	return find{armor: &a}, true
}

// offerFind asks whether to swap for what was in the chest.
//
// It is a question rather than a pickup because an affix takes as well as
// gives, so a find is not reliably an upgrade — and because the alternative,
// equipping it automatically, would let a tier-two sword out of a low chest
// replace the tier-four one you paid for.
func (g *Game) offerFind(f find) {
	p := g.Player
	var name, keeping, gains string
	switch {
	case f.weapon != nil:
		name, keeping = f.weapon.Titled(), p.Weapon.Titled()
		gains = fmt.Sprintf("strike %d against your %d", f.weapon.Strike, p.Weapon.Strike)
	case f.armor != nil:
		name, keeping = f.armor.Titled(), p.Armor.Titled()
		gains = fmt.Sprintf("defence %d against your %d", f.armor.Defense, p.Armor.Defense)
	default:
		return
	}

	body := fmt.Sprintf("Under everything else: a %s.\n\n%s. %s",
		name, upper(gains), affixReads(f))
	g.Ask("", body, []string{"Take it", "Leave it"}, func(g *Game, choice int) {
		if choice != 0 {
			g.Log.AddColor(render.ColInkDim, "You leave the %s where it is.", name)
			return
		}
		switch {
		case f.weapon != nil:
			p.Weapon = *f.weapon
		case f.armor != nil:
			p.Armor = *f.armor
		}
		g.Sound.Play("world/equip")
		g.Log.AddColor(render.ColGold, "You take up the %s. The %s is left behind.", name, keeping)
	})
}

// affixReads spells out what the suffix does, since the name alone does not say
// and the whole decision turns on it.
func affixReads(f find) string {
	var a *model.Affix
	switch {
	case f.weapon != nil:
		a = f.weapon.Affix
	case f.armor != nil:
		a = f.armor.Affix
	}
	if a == nil {
		return ""
	}
	parts := []string{}
	for _, p := range []struct {
		n int
		s string
	}{
		{a.Bonus.Strike, "strike"}, {a.Bonus.Defense, "defence"},
		{a.Bonus.Strength, "strength"}, {a.Bonus.Dexterity, "dexterity"},
		{a.Bonus.Speed, "speed"}, {a.Bonus.Psyche, "psyche"},
	} {
		if p.n != 0 {
			parts = append(parts, fmt.Sprintf("%+d %s", p.n, p.s))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Being " + a.Suffix + " is worth " + strings.Join(parts, ", ") + "."
}

func upper(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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

	ground := g.ground()
	ox, oy := s.cam.Offset()
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			t := l.At(tx, ty)
			if t == world.LVoid {
				continue
			}
			// Ground blends; structures are drawn flat on top of it.
			if _, isGround := localMaterial[t]; isGround {
				ground.Draw(dst, float64(tx*ts)+ox, float64(ty*ts)+oy, tx, ty, g.localMaterialAt)
				continue
			}
			ctx.TileTinted(g.Assets.Get(structureTex[t]), 0, tx, ty, structureTint[t])
		}
	}

	// Clutter sits between the floor and anything standing on it.
	g.drawLocalDecor(dst, s.cam, x0, y0, x1, y1)

	// Entities, then the player, so the player draws over doorways.
	for _, e := range l.Entities {
		if e.Used && e.Kind != world.EExit {
			continue
		}
		drawEntity(g, ctx, e)
	}

	g.drawFollowers(ctx, g.localFollow)

	sp := g.Assets.Get(heroSpriteKey(g.Player, g.LocalWalk.Dir(), g.LocalWalk.Moving()))
	frame := g.Tick() / 6
	if !g.LocalWalk.Moving() {
		frame = g.Tick() / 14
	}
	ctx.Shadow(px, py)
	ctx.World(sp, frame, px, py, false)

	s.drawHUD(g, dst)
}

// Structures reuse ground swatches at a different value: a wall is cold stone
// at half brightness, a roof is warm dirt pushed red. Cheaper than sourcing
// separate art, and it keeps everything in one palette.
var (
	structureTex = map[world.LocalTile]string{
		world.LWall: "ground/winter_stone",
		world.LRoof: "ground/summer_dirt",
	}
	structureTint = map[world.LocalTile]color.Color{
		world.LWall: color.RGBA{0x6E, 0x6E, 0x82, 0xFF},
		world.LRoof: color.RGBA{0xD8, 0x6A, 0x50, 0xFF},
	}
)

func drawEntity(g *Game, ctx *render.Ctx, e *world.Entity) {
	const ts = assetsys.TileSize
	x := float64(e.Pos.X*ts) + ts/2
	y := float64(e.Pos.Y*ts) + ts

	// Only draw real art. Asking the registry rather than guessing from the
	// sprite's size is what keeps unresolved keys out of the world as magenta
	// boxes; they fall through to the markers below instead.
	if e.Sprite != "" && g.Assets.Has(e.Sprite) {
		ctx.Shadow(x, y)
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
	hint := "C sheet - H help"
	ahead := g.LocalWalk.Tile.Add(g.LocalWalk.Dir().Delta())
	if e := g.Local.EntityAt(ahead.X, ahead.Y); e != nil && !e.Used {
		hint = "Z: " + e.Name
	}
	g.drawStatusBar(dst, g.Local.POI.Name, hint)
}
