package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The overworld's visible encounters, tested here because the tour cannot show
// them.
//
// `-demo` teleports the player with demoWalk rather than stepping, so it never
// runs the encounter roll and never produces a wanderer. The frame that proved
// this feature works was staged by hand and reverted, which is fine once and no
// use as a regression net. The world package already tests the movement — that a
// spawn lands somewhere legal, that a creature which has noticed you arrives,
// that it gives up. What follows is the half that lives in the scene: whether
// the roll still behaves like one encounter system.

// fakeEncounter is enough of an encounter to be carried and handed on.
func fakeEncounter(t *testing.T, g *Game) gamedata.Encounter {
	t.Helper()
	for biome, defs := range g.Data.Monsters {
		if len(defs) > 0 {
			return gamedata.Encounter{Monsters: []*model.Monster{{Def: defs[0], Name: biome}}}
		}
	}
	t.Skip("no monsters in the content to build an encounter from")
	return gamedata.Encounter{}
}

// TestARolledEncounterBecomesACreatureRatherThanAFight is the whole promise of
// the feature in one assertion: the roll no longer cuts to combat.
func TestARolledEncounterBecomesACreatureRatherThanAFight(t *testing.T) {
	g := storyGame(t)
	s := &overworldScene{}
	depth := len(g.stack)

	s.spawnWanderer(g, g.Walk.Tile, fakeEncounter(t, g))

	if len(s.wanderers) == 0 {
		t.Fatal("a hit on the roll produced no creature")
	}
	if len(g.stack) != depth {
		t.Error("a hit on the roll pushed a scene; it should have put something in the grass")
	}
	// King-move distance, computed here because world's own is unexported and
	// this is asserting the contract rather than borrowing the implementation.
	dx, dy := s.wanderers[0].w.Pos.X-g.Walk.Tile.X, s.wanderers[0].w.Pos.Y-g.Walk.Tile.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	d := dx
	if dy > d {
		d = dy
	}
	if d < world.WanderSpawnMin || d > world.WanderSpawnMax {
		t.Errorf("spawned %d tiles away, want %d..%d", d, world.WanderSpawnMin, world.WanderSpawnMax)
	}
}

// TestOnlyOneEncounterIsEverOut is the rate guarantee, and the reason this is
// one encounter system rather than two.
//
// The roll fires as often as it always did, but a hit while a creature is
// already out would stack them — and the balance report's assumptions about how
// often a player fights would quietly stop describing the game. The guard is a
// single condition in tryStep and nothing else enforces it.
func TestNoMoreThanTheCapIsEverOut(t *testing.T) {
	g := storyGame(t)
	s := &overworldScene{}
	enc := fakeEncounter(t, g)
	s.spawnWanderer(g, g.Walk.Tile, enc)
	if len(s.wanderers) == 0 {
		t.Skip("nowhere to stand near the start tile")
	}

	// Walk about. Every step rolls, and every hit would replace the creature if
	// the guard were not there.
	for i := 0; i < 400; i++ {
		for _, d := range []core.Dir{core.DirUp, core.DirRight, core.DirDown, core.DirLeft} {
			s.tryStep(g, d)
		}
	}
	if len(s.wanderers) > wanderCap {
		t.Errorf("%d creatures are out against a cap of %d", len(s.wanderers), wanderCap)
	}
}

// TestTouchingACreatureStartsTheFightItWasCarrying pins the other half of the
// promise: what you saw is what you get.
//
// The encounter is chosen when the creature appears and stored on the scene, so
// the fight has to come from there rather than from a fresh roll at the moment
// of contact. It also has to clear itself — a creature left standing after its
// own fight would start it again on the next tick.
func TestTouchingACreatureStartsTheFightItWasCarrying(t *testing.T) {
	g := storyGame(t)
	g.Party()
	s := &overworldScene{}
	enc := fakeEncounter(t, g)
	s.spawnWanderer(g, g.Walk.Tile, enc)
	if len(s.wanderers) == 0 {
		t.Skip("nowhere to stand near the start tile")
	}

	// Walk it onto the player.
	s.wanderers[0].w.Pos = g.Walk.Tile
	depth := len(g.stack)
	s.stepWanderer(g)

	if len(g.stack) != depth+1 {
		t.Fatalf("contact pushed %d scenes, want 1", len(g.stack)-depth)
	}
	if len(s.wanderers) != 0 {
		t.Error("the creature is still standing there after starting its own fight")
	}
	if g.sinceFight != 0 {
		t.Error("the grace period was not reset, so the next roll fires immediately")
	}
}

// TestACreatureThatLosesYouIsGone keeps the scene's copy in step with the
// world's rule. world.Wanderer.Step reports that it has given up; the scene has
// to act on it, or a creature that stopped following would sit in the grass for
// the rest of the run and block every future roll.
func TestACreatureThatLosesYouIsGone(t *testing.T) {
	g := storyGame(t)
	s := &overworldScene{}
	s.spawnWanderer(g, g.Walk.Tile, fakeEncounter(t, g))
	if len(s.wanderers) == 0 {
		t.Skip("nowhere to stand near the start tile")
	}

	s.wanderers[0].w.Life = 0 // out of patience
	s.wanderTick = 0
	s.stepWanderer(g)

	if len(s.wanderers) != 0 {
		t.Error("a creature that gave up is still out, and is blocking the roll")
	}
}
