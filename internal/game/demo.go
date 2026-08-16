package game

import (
	"fmt"
	"os"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
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

		// A battle, staged against a group so the multi-target layout shows.
		{at: 210, do: func(g *Game) {
			mons := g.Data.PickMonsters(g.RNG, "forest", 3, 3)
			if len(mons) > 0 {
				g.Push(newBattleScene(g, mons, "woods"))
			}
		}},
		{at: 240, shot: "08-battle"},
		{at: 246, do: func(g *Game) { g.demoOpenTechniques() }},
		{at: 258, shot: "08b-techniques"},
		{at: 264, do: func(g *Game) {
			if b, ok := g.Top().(*battleScene); ok {
				b.setRootMenu(g)
			}
		}},
		{at: 270, do: func(g *Game) { g.demoBattleAdvance() }},
		{at: 300, shot: "09-battle-resolving"},

		// A level-1 fighter against three of anything is a staged fight, not a
		// fair one, and it usually ends badly. This is a tour rather than a
		// playthrough, so patch the hero up and carry on.
		{at: 305, do: func(g *Game) {
			g.dropOverlays()
			if g.Player.HP <= 0 {
				g.Player.HP = g.Player.MaxHP
			}
		}},

		// A dungeon, and actually loot it, so the fixture this leaves behind
		// has spent interactables in it rather than being a fresh run.
		{at: 315, do: func(g *Game) {
			g.dropToOverworld()
			g.demoEnter(func(p *world.POI) bool {
				return p.Kind == world.KindDungeon || p.Kind == world.KindCave
			})
		}},
		{at: 345, shot: "10-dungeon"},
		{at: 350, do: func(g *Game) { g.demoLoot() }},
		{at: 360, do: func(g *Game) { g.dropOverlays() }},

		{at: 366, do: func(g *Game) { g.Push(newPauseScene(g)) }},
		{at: 378, shot: "11-pause"},
		{at: 383, do: func(g *Game) { g.Push(newSlotScene(g, slotSave)) }},
		{at: 395, shot: "12-save"},
		{at: 401, do: func(g *Game) { g.dropOverlays() }},

		// Leave the fixture behind.
		{at: 405, do: func(g *Game) {
			if err := g.SaveTo("demo"); err != nil {
				logf("could not write the demo save: %v", err)
			}
		}},

		{at: 415, do: func(g *Game) { g.Quit() }},
	}
}

// updateDemo runs the script. It returns true when it handled this tick, in
// which case the normal scene update is skipped for steps that mutate the
// stack, keeping captures reproducible.
func (g *Game) updateDemo() {
	d := g.demo
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

// dropToOverworld unwinds back to the continent view.
func (g *Game) dropToOverworld() {
	for len(g.stack) > 1 {
		if _, ok := g.stack[len(g.stack)-1].(*overworldScene); ok {
			return
		}
		g.stack = g.stack[:len(g.stack)-1]
	}
	g.Local = nil
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

// demoOpenTechniques drops into the spell list so a capture shows the icons.
func (g *Game) demoOpenTechniques() {
	b, ok := g.Top().(*battleScene)
	if !ok {
		return
	}
	b.menu.Index = 1 // Technique
	b.chooseRoot(g)
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
