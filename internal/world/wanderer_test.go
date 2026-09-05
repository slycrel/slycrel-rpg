package world

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
)

// stubNamer keeps world generation off the content package. These tests assert
// structural properties of a wanderer's movement, never a name, so a stub is
// the right namer here for the same reason it is in gamedata's world tests.
type stubNamer struct{}

func (stubNamer) PlaceName(*core.RNG, string) string    { return "Placename" }
func (stubNamer) PlaceTag(*core.RNG, string) string     { return "tag" }
func (stubNamer) PersonName(*core.RNG) string           { return "Person" }
func (stubNamer) NPCLine(*core.RNG) string              { return "line" }
func (stubNamer) SignText(*core.RNG) string             { return "sign" }
func (stubNamer) RecruitPitch(*core.RNG, string) string { return "pitch" }
func (stubNamer) Oddity(*core.RNG, string) string       { return "oddity" }

// walkableNear finds open ground to test around, since a random continent's
// centre may well be ocean.
func walkableNear(m *Map) core.Point {
	for r := 0; r < 60; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				p := core.Point{X: Width/2 + dx, Y: Height/2 + dy}
				if m.Walkable(p.X, p.Y) && m.POIAt(p.X, p.Y) == nil {
					return p
				}
			}
		}
	}
	return core.Point{X: Width / 2, Y: Height / 2}
}

// TestWandererSpawnsWhereItCanBeSeen pins the three things a spawn must never
// do, all of which produce a square the player cannot read: stand on the
// player, stand on a location marker, or stand in the sea.
func TestWandererSpawnsWhereItCanBeSeen(t *testing.T) {
	m := Generate(1994, stubNamer{})
	g := core.NewRNG(7)
	at := walkableNear(m)

	spawned := 0
	for i := 0; i < 400; i++ {
		w := m.SpawnWanderer(g, at, "beast", OmenHostile)
		if w == nil {
			continue // no room in the ring, which is allowed
		}
		spawned++
		d := chebyshev(w.Pos, at)
		if d < WanderSpawnMin || d > WanderSpawnMax {
			t.Fatalf("spawned %d tiles away, want %d..%d", d, WanderSpawnMin, WanderSpawnMax)
		}
		if w.Pos == at {
			t.Fatal("spawned on the player")
		}
		if !m.Walkable(w.Pos.X, w.Pos.Y) {
			t.Fatalf("spawned on unwalkable ground at %v", w.Pos)
		}
		if m.POIAt(w.Pos.X, w.Pos.Y) != nil {
			t.Fatalf("spawned on a location at %v", w.Pos)
		}
	}
	if spawned == 0 {
		t.Fatal("nothing spawned in 400 tries on open ground")
	}
}

// TestWandererClosesOnAStandingTarget is the whole feature in one assertion: a
// creature that has noticed you has to actually arrive.
//
// It is worth a test because the closing rule is hand-rolled rather than a
// pathfinder — it steps the larger axis first and falls back to the other when
// blocked — and "gets stuck against a lake forever" is exactly the kind of
// failure that looks fine in the one frame anybody checks.
func TestWandererClosesOnAStandingTarget(t *testing.T) {
	m := Generate(1994, stubNamer{})
	g := core.NewRNG(11)
	at := walkableNear(m)

	arrived, tried := 0, 0
	for i := 0; i < 60; i++ {
		w := m.SpawnWanderer(g, at, "beast", OmenHostile)
		if w == nil {
			continue
		}
		tried++
		// Give it a generous budget: the distance is at most WanderSpawnMax,
		// so anything much over that is it going round something.
		for step := 0; step < 40; step++ {
			if w.Pos == at {
				arrived++
				break
			}
			if !w.Step(g, m, at) {
				break
			}
		}
	}
	if tried == 0 {
		t.Fatal("nothing spawned to chase with")
	}
	// Not all of them: a creature can spawn across an inlet it cannot walk
	// round in the budget, and that is honest behaviour rather than a bug.
	if arrived*2 < tried {
		t.Errorf("only %d of %d wanderers reached a standing target; the closing rule is not converging", arrived, tried)
	}
}

// TestWandererGivesUp keeps a creature from following the player across the
// continent forever, which is what would happen if either bound came undone.
func TestWandererGivesUp(t *testing.T) {
	m := Generate(1994, stubNamer{})
	g := core.NewRNG(3)
	at := walkableNear(m)

	t.Run("by distance", func(t *testing.T) {
		w := &Wanderer{Pos: at, Kind: "beast", Life: WanderLife}
		far := core.Point{X: at.X + WanderGiveUp + 5, Y: at.Y}
		if w.Step(g, m, far) {
			t.Error("kept following a target beyond WanderGiveUp")
		}
	})

	t.Run("by patience", func(t *testing.T) {
		w := &Wanderer{Pos: at, Kind: "beast", Life: 2}
		// Somewhere it can see but never reach, so only Life can end this.
		target := core.Point{X: at.X + WanderNotice + 1, Y: at.Y}
		alive := 0
		for i := 0; i < WanderLife*2; i++ {
			if !w.Step(g, m, target) {
				break
			}
			alive++
		}
		if alive > 2 {
			t.Errorf("lived %d steps on a Life of 2", alive)
		}
	})
}
