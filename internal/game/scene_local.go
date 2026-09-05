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
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/ui"
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
			ViewH: render.ScreenH - hudH,
		},
		foeTimer: 30,
	}
}

func (s *localScene) Update(g *Game) error {
	g.LocalWalk.Advance()
	g.localFollow.Advance()
	s.cam.Update()

	// The long story first: a leg is the thing the player went somewhere for,
	// and a companion's beat is a thing that happened on the way.
	if g.serviceSagas() {
		return nil
	}
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
		g.stepTownsfolk()
		g.Local.StepFoes(g.RNG)
	}

	if Confirm() && !g.LocalWalk.Moving() {
		// Interact with whatever is directly ahead — or underfoot, since a
		// doorway is stood on rather than faced. Both go through g.ahead's rule
		// about who has gone home, or the town would be empty to look at and
		// full to talk to.
		if e := g.ahead(); e != nil {
			g.interact(e)
			return nil
		}
		if e := g.Local.EntityAt(g.LocalWalk.Tile.X, g.LocalWalk.Tile.Y); e != nil && !g.abed(e) {
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

	// Walking into a thing is how you engage it. Anybody abed is walked
	// straight through, which is correct: they are not there.
	//
	// This used to be true of foes, bosses and the way out, and everything else
	// in a town blocked the step and then waited to be pressed at. Nothing was
	// gained by the wait: you cannot bump a shop counter by accident, because
	// you had to walk at it to get there. A sign is the one exception, and only
	// because it now says what it says on the ground next to it — popping a box
	// over the top of writing the player is already reading is the interface
	// telling them something twice.
	if e := g.Local.EntityAt(next.X, next.Y); e != nil && !g.abed(e) {
		g.LocalWalk.Face(d)
		s.moveDelay = 8
		if e.Kind != world.ESign && e.Kind != world.EDecor {
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
	// Time passes indoors too. A player who could stop the clock by standing
	// in a shop would have found the way to wait out every night in the game.
	g.Clock.Tick(1)
	g.sinceFight++

	// Interiors have their own ambush rate; the places people live in do not.
	//
	// Read off the map rather than off the location's kind. The kind test was
	// right until somewhere turned up that is neither a settlement nor
	// dangerous — a wayside is a fire with people round it, built on a
	// KindCamp, and the party would have been jumped in the middle of it.
	if !g.Local.Peaceful && g.ambushDue() {
		enc := g.Data.PickEncounter(g.RNG, g.Local.Biome, g.Local.POI.Level, g.encounterSize(1+g.RNG.Intn(2)))
		if len(enc.Monsters) > 0 {
			g.Push(newBattleScene(g, enc, "dark"))
		}
	}
}

// How far apart ambushes are underground.
//
// **A budget of steps, not a chance per step**, and the difference is the whole
// of what a playthrough complained about. The old roll was six per cent a step
// after a grace of six, which is memoryless: measured over a hundred and eighty
// thousand steps of walking real dungeons it averages a fight every 21.3 steps,
// which sounds reasonable, and *fifteen per cent of those gaps are eight steps
// or fewer* while the longest is 132. That is exactly the report — "every 5-8
// steps, with the occasional long stretch" — and it is not a rate problem. The
// mean was fine. The shape was wrong at both ends.
//
// Drawing the distance up front fixes both with one change: nothing can arrive
// inside ambushFloor steps of the last one, and nothing can go quiet for longer
// than ambushCeil. The mean goes up because the complaint was also that there
// were too many, but the reason it stops feeling arbitrary is the floor.
const (
	ambushFloor = 20
	ambushCeil  = 48
)

// ambushDue counts the step down and reports whether this is the one.
//
// The budget is drawn lazily, so a save written before it existed — which is
// every save written before today, and they carry a zero — draws one on the
// first step underground rather than firing an ambush immediately. That is the
// zero value meaning the safe thing rather than meaning "now", which is the
// trap this format has fallen into before.
func (g *Game) ambushDue() bool {
	if g.nextAmbush <= 0 {
		g.nextAmbush = g.RNG.Between(ambushFloor, ambushCeil)
		return false
	}
	g.nextAmbush--
	if g.nextAmbush > 0 {
		return false
	}
	g.nextAmbush = g.RNG.Between(ambushFloor, ambushCeil)
	return true
}

// interact runs whatever the entity does.
func (g *Game) interact(e *world.Entity) {
	switch e.Kind {
	case world.EShopDoor, world.EHouseDoor:
		g.enterRoom(e)

	case world.EResident:
		g.knock(e)

	case world.EExit:
		// Out of a shop is back into the town it is in; out of anywhere else is
		// back to the overworld.
		if g.inRoom {
			g.leaveRoom()
			return
		}
		g.Sound.Play("world/enter")
		g.Local = nil
		g.floor = 0
		g.Pop()

	case world.EDeeper:
		g.changeFloor(g.floor+1, world.EShallower)

	case world.EShallower:
		g.changeFloor(g.floor-1, world.EDeeper)

	case world.ENPC:
		g.talkTo(e)

	case world.ERecruit:
		g.offerRecruit(e)

	case world.ESign:
		g.Say(e.Name, e.Line)

	case world.EAltar:
		g.offerAltar(e)

	case world.EChest:
		g.spend(e)
		g.openChest(e, false)

	case world.EHoard:
		g.spend(e)
		g.openChest(e, true)

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
		// Greyed rather than offered and refused: the innkeeper can see your
		// purse from where they are standing.
		rows := []ui.MenuItem{
			{Label: "Sleep", Detail: fmt.Sprintf("%d coins", cost),
				Disabled: g.Player.Coins < int64(cost)},
			{Label: "Decline"},
		}
		g.AskAs(e.Name, g.roleOf(e), g.faceOf(e), fmt.Sprintf("%s\n\nA night costs %d coins. You have %d.",
			body, cost, g.Player.Coins), rows, func(g *Game, choice int) {
			if choice != 0 {
				return
			}
			if g.Player.Coins < int64(cost) {
				g.Say("", "You are turned away, politely, by someone who has done it many times today.")
				return
			}
			g.Player.Coins -= int64(cost)
			g.restParty()
			// The bed buys the morning, which is the whole answer to night
			// being dangerous: it is a thing you can pay to skip, and the
			// price is already the one thing that scales with your level.
			g.Clock.WakeAt(sky.Dawn)
			// A bed is where the run is written down. This is the whole of
			// what a night at an inn now buys beyond hit points, and the
			// reason its price is worth paying.
			g.autosave()
			g.Say("", "You sleep like something that has stopped worrying. "+
				"You wake fully restored, slightly sticky, and at dawn.")
		})

	case world.EFoe, world.EBoss:
		g.spend(e)
		// A lurking shape is not always a fight now. A boss always is: it is
		// the point of the room, and a room whose point turns out to be a
		// spring is a room that wasted the walk.
		if e.Kind == world.EFoe {
			omen := e.Omen
			if omen == world.OmenMystery {
				omen = resolveMystery(g.RNG)
			}
			if omen == world.OmenBoon {
				g.grantBoon(g.RNG, g.Local.POI.Name)
				return
			}
		}
		count := g.encounterSize(1 + g.RNG.Intn(2))
		level := g.Local.POI.Level
		if e.Kind == world.EBoss {
			// A boss stands alone whatever you brought with you. It is the
			// point of the room, and two of them is a different room.
			count = 1
			level += 3
		}
		// A visible foe on the floor is a fight you walked at, so it gets a
		// shape like any other. A boss does not: it is one named thing standing
		// in the room it is the point of, which is the brute shape written by
		// hand before shapes existed.
		enc := g.Data.PickEncounter(g.RNG, g.Local.Biome, level, count)
		if e.Kind == world.EBoss {
			enc = gamedata.Encounter{
				Monsters: g.Data.PickMonsters(g.RNG, g.Local.Biome, level, 1),
				Shape:    gamedata.ShapeBrute,
			}
		}
		mons := enc.Monsters
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
				g.advanceSagas(saga.Event{Kind: saga.Clear, POI: idx})
			}
		}
		g.Push(newBattleScene(g, enc, g.Local.POI.Name))
	}
}

// spend marks an interactable used, both on the live map and on the location
// itself, so it stays used after the interior is regenerated on a later visit.
func (g *Game) spend(e *world.Entity) {
	e.Used = true
	if p := g.here(); p != nil {
		p.MarkUsed(string(e.Kind), e.Pos, g.floor)
	}
}

// here is the location that owns what the player has spent.
//
// Usually it is simply the map they are standing on. Inside a shop it is not:
// the room is built from the town's seed and carries a POI of its own so the
// status bar can say "Inn", but that record is synthetic and is not in the
// world's list, so anything written to it is written to nothing. The town is
// the ledger — which is also the answer the save file needs, and it was already
// working this out for itself in two places.
func (g *Game) here() *world.POI {
	if g.inRoom && g.townPOI != nil {
		return g.townPOI
	}
	if g.Local != nil {
		return g.Local.POI
	}
	return nil
}

// openChest rolls contents scaled to the location's level band.
// openChest empties a chest, or the one at the bottom of the place.
//
// A hoard is the same routine paid three times over and guaranteed the gear
// roll, which is the difference between a chest on the way and the reason for
// the walk. Not a different table: a hoard that dropped things a chest cannot
// would be a second loot system to keep in step with the first, and the thing
// that makes the bottom of a tower worth reaching is how much rather than what.
func (g *Game) openChest(e *world.Entity, hoard bool) {
	mult := int64(1)
	if hoard {
		mult = 3
	}
	coins := int64(g.RNG.Between(8, 25)*core.Max(1, g.Local.POI.Level)) * mult
	g.Player.Coins += coins
	g.Sound.Play("world/chest")

	// One consumable, and a decent chance of something sellable.
	pool := []string{"Small Beer", "Field Poultice", "Bottled Nap", "Bitter Root", "Suspicious Pollen"}
	if g.Local.POI.Level >= 5 {
		pool = append(pool, "Physician's Draught", "Philosopher's Espresso")
	}
	body := fmt.Sprintf("%d coins", coins)
	if it, ok := g.Data.Item(core.Pick(g.RNG, pool)); ok {
		it.Count = 1 + g.RNG.Intn(2)
		g.Player.AddItem(it)
		body += "\n" + itemLine(it)
	}
	if g.RNG.Chance(0.4) {
		trinkets := []string{"Stolen Trinket", "Someone's Locket", "Cracked Crystal", "Hoard Coin"}
		if it, ok := g.Data.Item(core.Pick(g.RNG, trinkets)); ok {
			it.Count = 1
			g.Player.AddItem(it)
			body += "\n" + itemLine(it)
		}
	}
	find, ok := g.rollAffixedGear()
	// A hoard always has something in it worth carrying. Rolled twice rather
	// than given a better table, so it can still come up with nothing the
	// second time and the first roll is the one that usually answers.
	if !ok && hoard {
		find, ok = g.rollAffixedGear()
	}
	if ok {
		g.SayThen(e.Name, e.Line+"\n\n"+body, func(g *Game) {
			g.takeFind(find, "Under everything else:")
		})
		return
	}
	g.Say(e.Name, e.Line+"\n\n"+body)
}

// itemLine names something picked up and says what it is for.
//
// A chest used to report its contents as a list of bare names — "Bitter Root
// x2. Someone's Locket." — which tells a player nothing about whether they have
// found medicine, money or a quest item. The information was never missing; the
// description sits on the item and the pack shows it. It was simply not being
// said at the one moment somebody is looking straight at the thing and asking.
func itemLine(it model.Item) string {
	name := it.Name
	if it.Count > 1 {
		name = fmt.Sprintf("%s x%d", name, it.Count)
	}
	if what := itemPurpose(it); what != "" {
		return name + " - " + what
	}
	return name
}

// itemPurpose is the short form: what this is good for, in a few words.
// The item's own description is the joke; this is the answer.
func itemPurpose(it model.Item) string {
	switch it.Kind {
	case model.ItemHeal:
		return "drink it to heal"
	case model.ItemPsyche:
		return "drink it for psyche"
	case model.ItemBuff:
		return "drink it before a fight"
	case model.ItemRevive:
		return "stands somebody back up"
	case model.ItemCure:
		return "clears what you have caught"
	case model.ItemTrinket:
		return "worth something to a shopkeeper"
	case model.ItemKey:
		return "opens something, somewhere"
	case model.ItemCamp:
		return "sleep rough: half your pools back, no bed"
	}
	return ""
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

	// Only gear whose name is free of a flourish can take one, and only gear
	// the hero could actually put on.
	//
	// The second filter is what a hard class gate costs if nobody pays it: a
	// chest is the one place a *named* piece turns up, and a table with three
	// lanes in it would otherwise hand a mage a two-handed maul two thirds of
	// the time. That is not a find, it is a coin with a longer name on it.
	// Companions are deliberately not counted — the roll is for the person
	// opening the box, and a hireling can be let go tomorrow.
	weapons, armors := g.Data.StockForClass(tier, g.Player.Class)
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

// takeFind puts a piece of found equipment in the pack.
//
// It used to be a question — "Take it / Leave it" — with the numbers against
// what you were already wearing, because there was nowhere for a sword to go
// except onto the body, so accepting one meant discarding the old one and
// declining meant leaving it on the floor. A find below your current gear was
// therefore a prompt offering you a downgrade and nothing else.
//
// Equipment is carried now, so there is nothing to decide here. You pick it up.
// Whether to wear it is a question for the character sheet, and whether to sell
// it is a question for the next shop.
func (g *Game) takeFind(f find, where string) {
	var gear model.Carried
	switch {
	case f.weapon != nil:
		gear = model.Carried{Weapon: f.weapon}
	case f.armor != nil:
		gear = model.Carried{Armor: f.armor}
	default:
		return
	}
	g.Player.Carry(gear)
	g.Sound.Play("world/loot")
	g.Say("", fmt.Sprintf("%s %s.\n\n%s It goes in your pack.",
		where, article(gear.Titled()), carriedDescribe(gear)))
}

// article puts the right indefinite article on a name. Gear names are generated
// — "Actual Sword of Mild Regret" — so "a %s" produced "a Actual Sword" often
// enough to be the first thing anybody noticed about a find.
func article(name string) string {
	if name == "" {
		return name
	}
	switch name[0] {
	case 'A', 'E', 'I', 'O', 'U', 'a', 'e', 'i', 'o', 'u':
		return "an " + name
	}
	return "a " + name
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
	s.cam.CenterOn(px, py)
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
		if g.abed(e) {
			continue
		}
		drawEntity(g, ctx, e)
	}

	g.drawFollowers(ctx, g.localFollow)
	g.drawEscortee(ctx, g.localFollow)

	sp := g.Assets.Get(heroSpriteKey(g.Player, g.LocalWalk.Dir(), g.LocalWalk.Moving()))
	frame := g.Tick() / 6
	if !g.LocalWalk.Moving() {
		frame = g.Tick() / 14
	}
	ctx.Shadow(px, py)
	ctx.World(sp, frame, px, py, false)

	// A settlement is open to the sky and a dungeon is not, so the rain stops
	// at the door. The light still changes either way: a town at midnight is a
	// town at midnight whether or not anything is falling on it.
	g.drawSky(dst, g.weatherHere(), g.Local.Indoors)

	// Marks and labels go over the tint, because a name that dims at dusk is a
	// name nobody can read at the hour they most need it — and a star that dims
	// in the rain is worse, since the whole job of one is being spotted without
	// being looked for.
	s.drawMarks(g, ctx)
	s.drawLabels(g, dst)

	s.drawHUD(g, dst)
}

// labelRadius is how close a fixture has to be before it says what it is, in
// tiles. Four is roughly a building's width: near enough that a label belongs
// to the thing under it, far enough that a street is legible from the middle
// of it rather than one doorway at a time.
const labelRadius = 4

// drawLabels floats the name of every interactable in reach.
//
// The old rule was that a shop was a coloured box until you walked into it and
// read the modal that came up, which made finding the armourer a matter of
// trying every door. Nothing here is new information — it is the same Name the
// message box was already going to print — it is that information arriving
// before the interaction rather than as its reward.
func (s *localScene) drawLabels(g *Game, dst *ebiten.Image) {
	const ts = assetsys.TileSize
	ox, oy := s.cam.Offset()
	px, py := g.LocalWalk.Pixel()
	hx, hy := px+ox, py+oy
	here := g.LocalWalk.Tile
	poiIdx := g.currentPOIIndex()
	focus := g.ahead()
	if focus == nil {
		focus = g.Local.EntityAt(here.X, here.Y)
		if focus != nil && g.abed(focus) {
			focus = nil
		}
	}

	for _, e := range g.Local.Entities {
		if e.Used || g.abed(e) {
			continue
		}
		r := labelRange(e.Kind)
		if r == 0 || core.Abs(e.Pos.X-here.X) > r || core.Abs(e.Pos.Y-here.Y) > r {
			continue
		}
		// Gold is about how close it is, not about whether the text happens to
		// be a name: a shop keeps its name at any distance, and a shop across
		// the square is still across the square.
		//
		// The one exception is somebody holding something that is already
		// yours — a finished errand, an installment they have been carrying
		// since you left. That is worth saying out loud from across a street,
		// and it is the same gold the star over their head is using.
		d := labelDist(e.Pos, here)
		text, _, show := g.labelFor(e, d, poiIdx)
		if !show {
			continue
		}
		col := render.ColInkDim
		if d <= nameRadius || g.attention(e, poiIdx) == attentionOwed {
			col = render.ColGold
		}
		lines := []string{text}
		if e == focus {
			col = render.ColGold
			lines = []string{e.Name}
			// A sign has nothing to it *but* what it says, so standing at one
			// is the whole interaction. Reading it should not cost a keypress
			// and a box over the town it is standing in.
			if e.Kind == world.ESign && e.Line != "" {
				lines = render.Wrap(e.Line, 172)
			}
		}
		cx, by := float64(e.Pos.X*ts)+ts/2+ox, float64(e.Pos.Y*ts)+oy
		ui.Tag(dst, lines, clearOfHero(lines, cx, by, hx, hy), by, col)
	}
}

// clearOfHero slides a label sideways when it would otherwise be drawn across
// the character the player is steering, returning the centre to draw it at.
//
// A tag sits above its own tile, which is the right place until the thing being
// named is *below* the hero: the sprite is two tiles tall and stands on one, so
// the gate you walk out of puts its name across your own head. Sideways rather
// than up, for two reasons. There is far more room across a 480-pixel screen
// than there is between a doorway and the top of the frame, and lifting is what
// stacks two labels into one unreadable pile — everything driven out of the
// hero's way vertically arrives at the same height, while two things in
// different columns slide to different places.
//
// hx, hy is the hero's feet in screen pixels.
func clearOfHero(lines []string, cx, bottomY, hx, hy float64) float64 {
	// A box round the sprite rather than the sprite itself: the art differs
	// per walk sheet and a label does not need to be flush against it.
	const halfW, height, gap = 10.0, 30.0, 3.0
	w, h := ui.TagSize(lines)
	if cx+w/2 < hx-halfW || cx-w/2 > hx+halfW {
		return cx
	}
	if bottomY-h > hy || bottomY < hy-height {
		return cx
	}
	// Away from the hero, so a label stays on the side its own tile is on.
	if cx <= hx {
		return hx - halfW - gap - w/2
	}
	return hx + halfW + gap + w/2
}

// labelRange is how far off a kind is worth naming, in tiles, zero meaning
// never.
//
// Three answers rather than one, because the question a label answers is
// different for each. A door or a gate is a *destination*, and knowing which
// one is the armourer from across the square is the whole point. A townsperson
// is not: their name tells you nothing you would walk over for, and a capital
// holds ten of them milling around on the same streets, so naming them at
// range turns a market into a wall of text. And a foe is neither — walking
// into one starts a fight, and what it turns out to be is what the fight is
// for.
func labelRange(k world.EntityKind) int {
	switch k {
	case world.EFoe, world.EBoss:
		return 0
	case world.EDecor:
		// Furniture does not introduce itself. It never had a name worth
		// reading and it never had many of them in one room either — a shop is
		// eight pieces of stock on a wall, and eight plates saying "clutter"
		// is a room you cannot see for the labels on it.
		return 0
	case world.EShop, world.EInn, world.EShopDoor:
		// Always. A shop's sign is the one label that is pure navigation — it
		// is how you cross a town towards the armourer instead of walking into
		// buildings until one of them is the armourer — and a sign you have to
		// be within four tiles to read is a sign you find by the method it
		// exists to replace. Four buildings in a settlement, so this is four
		// plates on screen at worst.
		return alwaysLabelled
	}
	return labelRadius
}

// alwaysLabelled is a range larger than any interior, for the things whose
// label is directions rather than description.
const alwaysLabelled = 1 << 20

// labelShowing reports whether this thing currently has a tag over it.
//
// Used by the attention star, which has to know where the top of that tag is so
// it can sit above it rather than inside it.
func (g *Game) labelShowing(e *world.Entity, poiIdx int) bool {
	if e.Used || g.abed(e) {
		return false
	}
	r := labelRange(e.Kind)
	d := labelDist(e.Pos, g.LocalWalk.Tile)
	if r == 0 || d > r {
		return false
	}
	_, _, show := g.labelFor(e, d, poiIdx)
	return show
}

// tagPlateH is how tall a one-line floating tag is, measured the way ui.Tag
// builds it: the ink, plus the padding above and below the plate.
const tagPlateH = float64(render.TextInkH) + 5

// nameRadius is how close you have to be before a label says who somebody is.
//
// Inside it a label is gold and gives the name; outside it, anything with a
// placeholder gives that instead. Two rather than one, because one meant only
// the single thing you were facing was ever gold — you had to be pointed
// directly at a shop door to be told which shop it was, which is the state the
// labels were added to get rid of.
const nameRadius = 2

// labelFor returns what a thing's tag says at this distance, whether that is
// its actual name, and whether it gets a tag at all.
//
// Three bands, and the middle one is the point. Up close you are told what
// something is. Further off, a person becomes "someone" — which says there is
// somebody there worth the walk without spending the introduction before you
// have made it. A name handed to you for free from six tiles away is a name you
// never quite met anybody to learn.
//
// Only the kinds with a placeholder written for them do this. A shop's name is
// how you find the armourer rather than a reward for arriving at one, and
// turning it into "a building" at range would put back exactly the problem the
// labels were added to fix.
//
// And only people who have something. "Someone" over every villager in a
// capital is not a hint, it is wallpaper — ten of them, two of which overlap
// each other into "someonesomeone" — and it teaches the player that the word
// means nothing. Over the one who has an errand, it means go and look, which is
// the entire reason to draw it.
func (g *Game) labelFor(e *world.Entity, dist, poiIdx int) (text string, named, show bool) {
	if dist <= nameRadius {
		return e.Name, true, true
	}
	ph := g.Data.Text.LabelPlaceholder[string(e.Kind)]
	if ph == "" {
		return e.Name, true, true
	}
	switch g.attention(e, poiIdx) {
	case attentionOwed:
		return ph, false, true
	case attentionOffer:
		return ph, false, true
	}
	return "", false, false
}

// labelDist is how far something is, in the square measure the label bands use.
func labelDist(a, b core.Point) int {
	return core.Max(core.Abs(a.X-b.X), core.Abs(a.Y-b.Y))
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
		frame := g.Tick() / 12
		if e.Still {
			frame = e.Frame
		}
		// No shadow under furniture. A shadow says "this is standing here",
		// which is right for a person and wrong for a shelf against a wall —
		// it lifts the thing off the floor and it reads as floating.
		if !e.Still {
			ctx.Shadow(x, y)
		}
		ctx.World(g.Assets.Get(e.Sprite), frame, x, y, false)
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
	case world.EShopDoor, world.EHouseDoor:
		// A door in a wall, with a trade's colour on it. It is the front of a
		// building rather than a person now, so it is drawn as one: a plank
		// panel, a lintel, and a handle you can see from across a street.
		//
		// A house takes the plain lintel, which is what makes the coloured ones
		// worth having: four doors in a town mean something and the other ten
		// are doors.
		c = shopDoorColour(e.Shop)
		if e.Kind == world.EHouseDoor {
			c = color.RGBA{0x8C, 0x6E, 0x48, 0xFF}
		}
		render.Rect(ctx.Dst, bx+2, by+3, ts-4, ts-3, color.RGBA{0x5C, 0x3E, 0x22, 0xFF})
		render.Rect(ctx.Dst, bx+2, by+3, ts-4, 2, c)
		render.Rect(ctx.Dst, bx+ts-6, by+9, 2, 2, color.RGBA{0xE8, 0xC8, 0x70, 0xFF})
		render.Frame(ctx.Dst, bx+2, by+3, ts-4, ts-3, color.RGBA{0x2C, 0x1C, 0x10, 0xFF})
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
	case world.EHoard:
		// The same chest in gold, and bigger. It is the end of the place; a
		// player who has walked four floors for it should be able to see that
		// it is not the third chest of the afternoon.
		render.Rect(ctx.Dst, bx+1, by+4, ts-2, ts-6, color.RGBA{0xB0, 0x88, 0x30, 0xFF})
		render.Rect(ctx.Dst, bx+1, by+4, ts-2, 4, color.RGBA{0xF0, 0xD0, 0x70, 0xFF})
		render.Rect(ctx.Dst, bx+6, by+8, 4, 4, color.RGBA{0xFF, 0xF0, 0xB0, 0xFF})
		render.Frame(ctx.Dst, bx+1, by+4, ts-2, ts-6, color.RGBA{0x40, 0x30, 0x10, 0xFF})
		return
	case world.EDeeper, world.EShallower:
		// Four treads, each narrower than the last, with a dark line under
		// every one.
		//
		// The gaps are the whole drawing. The first version stacked three
		// three-pixel bars with no space between them, which at this size is a
		// grey rectangle — and a grey rectangle in the corner of a room is
		// scenery. Steps are read from the shadow under each tread, so that is
		// what is drawn.
		//
		// Which way it goes is in the label rather than in the picture: at
		// sixteen pixels the difference between up and down is one shaded edge,
		// and a player deciding whether to commit to another floor should not
		// have to read a shaded edge.
		for i := 0; i < 4; i++ {
			w := float64(ts-4) - float64(i)*3
			y := by + 3 + float64(i)*3
			render.Rect(ctx.Dst, bx+2, y, w, 2, color.RGBA{0xB4, 0xAC, 0xA0, 0xFF})
			render.Rect(ctx.Dst, bx+2, y+2, w, 1, color.RGBA{0x4C, 0x46, 0x40, 0xFF})
		}
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
	// What is ahead used to be named here, in the one corner of the screen
	// furthest from where the player is looking. It is named on the thing
	// itself now, so the corner goes back to the compass and the keys.
	g.drawStatusBar(dst, g.Local.POI.Name, "C sheet - H help")
}

// drawMarks stars everybody in the interior who has something for the player.
//
// Its own pass rather than a line inside the entity loop, and after the sky.
// The first version drew each mark straight after its owner's sprite, which put
// it under the weather tint and under the night tint — invisible in the rain,
// which is exactly the sort of bug that survives a test suite and dies in one
// frame.
func (s *localScene) drawMarks(g *Game, ctx *render.Ctx) {
	poiIdx := g.currentPOIIndex()
	for _, e := range g.Local.Entities {
		if e.Used || g.abed(e) {
			continue
		}
		g.drawAttention(ctx, e, g.attention(e, poiIdx), poiIdx)
		g.drawOmen(ctx, e)
	}
}

// drawOmen marks a lurking shape with what it will turn out to be.
//
// The same three marks the overworld uses, because a player who has learned
// that a green ring is worth walking to should not have to learn it twice. It
// is only ever on a foe: a chest is already a chest and an altar already says
// what it is, and a mark over furniture would dilute the one thing these marks
// are for.
func (g *Game) drawOmen(ctx *render.Ctx, e *world.Entity) {
	if e.Kind != world.EFoe {
		return
	}
	rows, col, ok := omenMark(e.Omen, g.Tick())
	if !ok {
		return
	}
	const ts = assetsys.TileSize
	ox, oy := ctx.Cam.Offset()
	top := float64(e.Pos.Y*ts + ts)
	if sp := g.Assets.Get(e.Sprite); sp != nil && e.Sprite != "" {
		top -= float64(sp.H - sp.Head)
	} else {
		top -= ts
	}
	x := float64(e.Pos.X*ts) + ox + (ts-7)/2
	y := top + oy - 6 + starBob[(g.Tick()/8)%len(starBob)]
	drawGlyph(ctx.Dst, rows, x+2, y+2, color.RGBA{0x10, 0x0C, 0x14, 0xD8})
	drawGlyph(ctx.Dst, rows, x, y, col)
}

// changeFloor rebuilds the interior one level along and stands the party on the
// stair they came out of.
//
// The arrival point is the *matching* stair rather than the floor's own entry,
// and that is the whole of what makes a staircase feel like one: going down and
// immediately turning round has to put you back where you were, not at the
// front door. Floors are deterministic from the location's seed and the floor
// number, so the stair on the other side is always there to be found — the
// fallback to Entry exists for the one case that cannot happen and would be a
// party standing in a wall if it did.
func (g *Game) changeFloor(to int, arriveAt world.EntityKind) {
	if g.Local == nil || to < 0 || to >= g.Local.Depth {
		return
	}
	poi := g.Local.POI
	g.floor = to
	g.Local = world.BuildLocal(poi, g.Write, to)

	at := g.Local.Entry
	for _, e := range g.Local.Entities {
		if e.Kind == arriveAt {
			at = e.Pos
			break
		}
	}
	g.LocalWalk.Place(at)
	g.reformLines()
	g.localFollow.Place(at)
	g.Sound.Play("world/enter")

	// Say where you are, because a floor is the one piece of state in this game
	// with no readout: the map looks like a map and the strip says the place,
	// and neither of those changes when you go down a flight.
	g.Log.AddColor(render.ColGold, "%s — %s", poi.Name, floorName(poi, to, g.Local.Depth))
}

// floorName is how a level of a place is described, which depends on which way
// the place goes.
func floorName(poi *world.POI, floor, depth int) string {
	if depth <= 1 {
		return poi.Tag
	}
	if poi.Kind == world.KindTower {
		if floor+1 == depth {
			return "the top"
		}
		if floor == 0 {
			return "the ground floor"
		}
		return fmt.Sprintf("floor %d", floor+1)
	}
	if floor == 0 {
		return "the way in"
	}
	if floor+1 == depth {
		return "the bottom"
	}
	return fmt.Sprintf("%d floors down", floor)
}
