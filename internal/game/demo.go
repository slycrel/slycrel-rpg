package game

import (
	"fmt"
	"os"

	"github.com/slycrel/slycrel-rpg/internal/quest"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Demo mode drives the scene stack from a script and writes a screenshot at
// each stop, then exits. It exists because the visual half of this game cannot
// be asserted in a unit test: the fastest way to find out that a panel is
// drawing off-screen or a portrait is scaling wrong is to look at a frame.
//
// It deliberately manipulates scenes directly rather than simulating key
// presses, so a captured frame is never at the mercy of input timing.

// log reports a demo-script problem without aborting the tour.
func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "demo: "+format+"\n", args...)
}

// demoStep is one scripted beat.
type demoStep struct {
	at   int    // ticks after demo start
	shot string // non-empty means capture this frame under that name
	do   func(g *Game)
}

type demoScript struct {
	start int
	steps []demoStep
	next  int
	// autoFight keeps committing the hero to an attack whenever a battle hands
	// control back, so the tour plays an encounter through to its end instead of
	// capturing one frozen round. It is the only part of the script that reacts
	// to the game rather than to the clock, and it earns that: a fight taken to
	// the finish is what exercises companion turns, someone going down, the
	// spoils split and the party getting back up.
	autoFight bool
}

// StartDemo puts the game into scripted capture mode.
func (g *Game) StartDemo() {
	g.demo = &demoScript{start: g.tick, steps: buildDemoSteps()}
}

// InDemo reports whether scripted capture is running.
func (g *Game) InDemo() bool { return g.demo != nil }

func buildDemoSteps() []demoStep {
	return []demoStep{
		{at: 10, shot: "01-title"},

		{at: 20, do: func(g *Game) {
			g.startRun("Bosk", "the Regrettable", model.ClassFighter)
			g.dropOverlays() // startRun opens a welcome box; not wanted in the shot
		}},
		{at: 50, shot: "02-overworld"},

		// Walk a few tiles so the shot is not taken standing on the capital.
		{at: 55, do: func(g *Game) { g.demoWalk(6) }},
		{at: 75, shot: "03-overworld-wilds"},

		{at: 85, do: func(g *Game) { g.Push(newMapScene(g)) }},
		{at: 100, shot: "04-map"},
		{at: 105, do: func(g *Game) { g.Pop() }},

		{at: 115, do: func(g *Game) { g.Push(newStatusScene(g)) }},
		{at: 130, shot: "05-character"},
		{at: 135, do: func(g *Game) { g.Pop() }},

		// A settlement interior, and the shop inside it.
		{at: 145, do: func(g *Game) { g.demoEnter(func(p *world.POI) bool { return p.Kind.Settlement() }) }},
		{at: 175, shot: "06-town"},
		{at: 180, do: func(g *Game) { g.demoOpenShop() }},
		{at: 195, shot: "07-shop"},
		{at: 200, do: func(g *Game) { g.dropOverlays() }},

		// An errand from a townsperson, then the log it lands in.
		{at: 204, do: func(g *Game) { g.demoTakeQuest() }},
		{at: 214, shot: "07b-quest-offer"},
		{at: 218, do: func(g *Game) { g.demoChoose(0) }},
		{at: 222, do: func(g *Game) { g.Push(newQuestScene(g)) }},
		{at: 232, shot: "07c-quest-log"},
		{at: 236, do: func(g *Game) { g.dropOverlays() }},

		// Take somebody on, then walk far enough that the line strings out
		// behind the hero rather than staying stacked on their tile.
		{at: 242, do: func(g *Game) { g.demoHire() }},
		{at: 252, shot: "07d-hire"},
		{at: 256, do: func(g *Game) { g.demoChoose(0) }},
		{at: 260, do: func(g *Game) { g.dropOverlays(); g.demoWalkLocal(6) }},
		{at: 268, shot: "07e-company"},
		{at: 272, do: func(g *Game) {
			g.Push(newStatusScene(g))
			if s, ok := g.Top().(*statusScene); ok {
				s.who = 1 // the hireling's sheet, not the hero's a second time
			}
		}},
		{at: 282, shot: "07f-companion-sheet"},
		{at: 286, do: func(g *Game) { g.dropOverlays() }},

		// A battle, staged against a group so the multi-target layout shows —
		// and, now that there is a company, the party panel with it.
		//
		// The party is brought up to the level a company of two would plausibly
		// be at before picking a fight with three of anything. Two level-one
		// characters against three monsters is not a fight, it is a formality,
		// and a tour that always ends in a wipe never shows what happens after
		// one. It also leaves a more useful save fixture behind.
		{at: 292, do: func(g *Game) {
			g.demoLevelParty(4)
			mons := g.Data.PickMonsters(g.RNG, "forest", 3, 3)
			if len(mons) > 0 {
				g.Push(newBattleScene(g, mons, "woods"))
			}
		}},
		{at: 306, shot: "08-battle"},
		{at: 312, do: func(g *Game) { g.demoOpenTechniques() }},
		{at: 324, shot: "08b-techniques"},
		{at: 330, do: func(g *Game) { g.demoRootMenu() }},

		// Handing a companion something out of the pack, which is the cursor
		// the party-versus-self rework exists for.
		{at: 336, do: func(g *Game) { g.demoOfferItem() }},
		{at: 348, shot: "08c-on-whom"},
		{at: 354, do: func(g *Game) { g.demoRootMenu() }},

		{at: 360, do: func(g *Game) { g.demoBattleAdvance() }},
		{at: 390, shot: "09-battle-resolving"},

		// Then let it run to a conclusion rather than leaving it mid-round.
		{at: 396, do: func(g *Game) { g.demo.autoFight = true }},
		{at: 1100, shot: "09b-battle-over"},
		{at: 1106, do: func(g *Game) { g.demo.autoFight = false; g.demoEndBattle() }},
		{at: 1116, shot: "09c-after"},

		// A level-1 party against three of anything is a staged fight, not a
		// fair one, and it usually ends badly. This is a tour rather than a
		// playthrough, so patch the party up and carry on.
		{at: 1126, do: func(g *Game) {
			g.dropOverlays()
			if g.Player.HP <= 0 {
				g.restParty()
			}
		}},

		// A dungeon, and actually loot it, so the fixture this leaves behind
		// has spent interactables in it rather than being a fresh run.
		{at: 1136, do: func(g *Game) {
			g.dropToOverworld()
			g.demoEnter(func(p *world.POI) bool {
				return p.Kind == world.KindDungeon || p.Kind == world.KindCave
			})
		}},
		{at: 1166, shot: "10-dungeon"},
		{at: 1171, do: func(g *Game) { g.demoLoot() }},
		{at: 1181, do: func(g *Game) { g.dropOverlays() }},

		{at: 1187, do: func(g *Game) { g.Push(newPauseScene(g)) }},
		{at: 1199, shot: "11-pause"},
		{at: 1204, do: func(g *Game) { g.Push(newSlotScene(g, slotSave)) }},
		{at: 1216, shot: "12-save"},
		{at: 1222, do: func(g *Game) { g.dropOverlays() }},

		// Leave the fixture behind.
		{at: 1226, do: func(g *Game) {
			if err := g.SaveTo("demo"); err != nil {
				logf("could not write the demo save: %v", err)
			}
		}},

		{at: 1236, do: func(g *Game) { g.Quit() }},
	}
}

// updateDemo runs the script. It returns true when it handled this tick, in
// which case the normal scene update is skipped for steps that mutate the
// stack, keeping captures reproducible.
func (g *Game) updateDemo() {
	d := g.demo
	if d.autoFight {
		if b, ok := g.Top().(*battleScene); ok && b.mode == modeRoot {
			g.demoBattleAdvance()
		}
	}
	elapsed := g.tick - d.start
	for d.next < len(d.steps) && d.steps[d.next].at <= elapsed {
		s := d.steps[d.next]
		d.next++
		if s.do != nil {
			s.do(g)
		}
		if s.shot != "" {
			g.pendingShot = "demo-" + s.shot
		}
	}
	if d.next >= len(d.steps) {
		g.Quit()
	}
}

// dropOverlays pops everything above the base gameplay scene.
func (g *Game) dropOverlays() {
	for len(g.stack) > 1 {
		switch g.stack[len(g.stack)-1].(type) {
		case *overworldScene, *localScene:
			return
		}
		g.stack = g.stack[:len(g.stack)-1]
	}
}

// demoWalk teleports the player a few tiles off the capital so the overworld
// shot shows terrain rather than a building.
func (g *Game) demoWalk(n int) {
	p := g.Walk.Tile
	for i := 0; i < n; i++ {
		for _, d := range []core.Dir{core.DirDown, core.DirRight, core.DirUp, core.DirLeft} {
			q := p.Add(d.Delta())
			if g.World.Walkable(q.X, q.Y) && g.World.POIAt(q.X, q.Y) == nil {
				p = q
				break
			}
		}
	}
	g.Walk.Place(p)
	g.World.Reveal(p, 8)
}

// demoEnter finds the nearest location matching pred and goes inside.
func (g *Game) demoEnter(pred func(*world.POI) bool) {
	var best *world.POI
	bestD := 1 << 30
	for _, p := range g.World.POIs {
		if !pred(p) {
			continue
		}
		if d := p.Pos.Manhattan(g.Walk.Tile); d < bestD {
			best, bestD = p, d
		}
	}
	if best == nil {
		return
	}
	g.Walk.Place(best.Pos)
	g.World.Reveal(best.Pos, 8)
	g.enterPOI(best)
	g.dropOverlays()
}

// demoOpenShop walks the entity list for a merchant and opens its counter.
func (g *Game) demoOpenShop() {
	if g.Local == nil {
		return
	}
	for _, e := range g.Local.Entities {
		if e.Kind == world.EShop {
			g.Push(newShopScene(g, e))
			return
		}
	}
}

// demoLoot opens every chest in the current interior, so the saved fixture
// carries spent interactables and the loot that came out of them.
func (g *Game) demoLoot() {
	if g.Local == nil {
		return
	}
	for _, e := range g.Local.Entities {
		if e.Kind == world.EChest && !e.Used {
			g.spend(e)
			g.openChest(e)
			g.dropOverlays() // openChest pushes a message box
		}
	}
}

// demoHire takes on the companion loitering outside the town's inn.
//
// The tour grants the fee rather than earning it: a level-one fighter cannot
// afford a hireling, and what the capture is for is the party interface. If the
// settlement the tour walked into is a village — no inn, so nobody standing
// outside one — the recruit is conjured, the same way demoTakeQuest asks the
// generator directly rather than hoping a villager has an errand.
func (g *Game) demoHire() {
	if g.Local == nil || g.PartyFull() {
		return
	}
	level := core.Max(1, g.Player.Level)
	for _, e := range g.Local.Entities {
		if e.Kind == world.ERecruit && !e.Used {
			g.Player.Coins += rules.HireCost(level, model.MonsterKind(e.Blood))
			g.offerRecruit(e)
			return
		}
	}
	conjured := &world.Entity{
		Kind: world.ERecruit, Name: "Nessa",
		Line:  "\"I fight for money. I've tried the other reasons.\"",
		Class: string(model.ClassMage), Blood: string(model.KindFey), Look: "hero/druid",
	}
	g.Player.Coins += rules.HireCost(level, model.MonsterKind(conjured.Blood))
	g.offerRecruit(conjured)
}

// demoWalkLocal steps the party through the interior so the follower line has
// spread out before the shot rather than sitting stacked on the hero's tile.
// Movement is snapped rather than animated, because a capture must not depend
// on how many ticks the tween has had.
func (g *Game) demoWalkLocal(n int) {
	if g.Local == nil {
		return
	}
	for i := 0; i < n; i++ {
		for _, d := range []core.Dir{core.DirDown, core.DirLeft, core.DirRight, core.DirUp} {
			q := g.LocalWalk.Tile.Add(d.Delta())
			if !g.Local.Walkable(q.X, q.Y) || g.Local.EntityAt(q.X, q.Y) != nil {
				continue
			}
			from := g.LocalWalk.Tile
			g.LocalWalk.Place(q)
			g.LocalWalk.Face(d)
			g.localFollow.Step(from)
			break
		}
	}
	g.localFollow.Settle()
}

// demoLevelParty brings everyone up to a level and re-arms them, so a staged
// fight is one the company could actually have walked into.
func (g *Game) demoLevelParty(level int) {
	for _, c := range g.Party() {
		for c.Level < level {
			rules.LevelUp(g.RNG, c)
		}
		if c.TotalXP < rules.XPForLevel(level) {
			c.TotalXP = rules.XPForLevel(level)
		}
		g.Data.Equip(c)
	}
}

// demoEndBattle closes a finished battle the way a keypress would, so that
// whatever the ending actually does — the company carrying a fallen hero to
// town, in particular — happens rather than being skipped.
//
// A real defeat is left alone: that path resets the stack to the title screen,
// and the tour's staged fight is deliberately unfair, so honouring it would end
// the tour two thirds of the way through every time the dice went badly.
func (g *Game) demoEndBattle() {
	b, ok := g.Top().(*battleScene)
	if !ok || b.mode != modeDone || b.result == 2 {
		return
	}
	g.Pop()
	b.onPopped(g)
}

// demoRootMenu returns a battle to its top-level command list.
func (g *Game) demoRootMenu() {
	if b, ok := g.Top().(*battleScene); ok {
		b.setRootMenu(g)
	}
}

// demoOfferItem opens the pack and commits to the first thing in it, which
// lands the scene on the "on whom" cursor whenever there is a company to choose
// between. With a solo hero the cursor is skipped by design, so this captures
// the root menu instead and the shot is still honest about what happens.
func (g *Game) demoOfferItem() {
	b, ok := g.Top().(*battleScene)
	if !ok {
		return
	}
	b.menu.Index = 2 // Item
	b.chooseRoot(g)
	if b.mode == modeItem {
		b.menu.Index = 0
		b.chooseItem(g)
	}
}

// demoOpenTechniques drops into the spell list so a capture shows the icons.
func (g *Game) demoOpenTechniques() {
	b, ok := g.Top().(*battleScene)
	if !ok {
		return
	}
	b.menu.Index = 1 // Technique
	b.chooseRoot(g)
}

// demoTakeQuest opens a conversation with someone who has something to ask.
// The demo forces the case rather than waiting for it: whether a given
// villager has an errand is a stable hash, so a scripted tour would otherwise
// have to walk the whole town hoping.
func (g *Game) demoTakeQuest() {
	if g.Local == nil {
		return
	}
	for _, e := range g.Local.Entities {
		if e.Kind != world.ENPC {
			continue
		}
		g.talkTo(e)
		m, ok := g.Top().(*messageScene)
		if !ok {
			continue
		}
		// A plain conversation is also a message box; only an offer has
		// choices attached to it.
		if len(m.choices) > 0 {
			return
		}
		g.Pop()
	}
	// Nobody in this town happened to be a giver; ask the generator directly
	// so the capture still shows the interface.
	if idx := g.currentPOIIndex(); idx >= 0 {
		if q, ok := quest.Generate(g.RNG, g.World, g.Data, g.Write, idx, "Someone"); ok {
			g.offerQuest(&world.Entity{Name: q.Giver}, q)
		}
	}
}

// demoChoose answers an open prompt, which is how the tour accepts a quest.
func (g *Game) demoChoose(choice int) {
	m, ok := g.Top().(*messageScene)
	if !ok {
		return
	}
	g.Pop()
	if m.onChoose != nil {
		m.onChoose(g, choice)
	}
}

// demoBattleAdvance commits the player to an attack so the capture includes a
// resolving round with damage numbers and a populated transcript.
func (g *Game) demoBattleAdvance() {
	b, ok := g.Top().(*battleScene)
	if !ok {
		return
	}
	b.beginTargeting(modeRoot)
	b.confirmTarget(g)
}
