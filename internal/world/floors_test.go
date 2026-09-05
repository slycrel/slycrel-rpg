package world

import (
	"fmt"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
)

type floorNamer struct{}

func (floorNamer) PlaceName(*core.RNG, string) string    { return "Place" }
func (floorNamer) PlaceTag(*core.RNG, string) string     { return "tag" }
func (floorNamer) PersonName(*core.RNG) string           { return "Person" }
func (floorNamer) NPCLine(*core.RNG) string              { return "line" }
func (floorNamer) SignText(*core.RNG) string             { return "sign" }
func (floorNamer) RecruitPitch(*core.RNG, string) string { return "pitch" }
func (floorNamer) Oddity(*core.RNG, string) string       { return "odd" }

func deepPOI(kind POIKind, seed int64, level int) *POI {
	return &POI{Kind: kind, Seed: seed, Level: level, Name: "Probe", Tag: "tag"}
}

func count(l *LocalMap, k EntityKind) int {
	n := 0
	for _, e := range l.Entities {
		if e.Kind == k {
			n++
		}
	}
	return n
}

// Every floor has a way back and exactly one way on, and the reward is at the
// end rather than on the way.
//
// The failure this catches is a floor with no stairs on it, which is a party
// walled into a place with no way out and no way to know it — the map has no
// readout for "this floor is a dead end", and the only symptom would be a
// player wandering a complete floor twice.
func TestEveryFloorCanBeLeftAndOnlyTheLastPays(t *testing.T) {
	for _, kind := range []POIKind{KindTower, KindCave} {
		for _, level := range []int{1, 6, 12} {
			poi := deepPOI(kind, 7, level)
			depth := Depth(poi)
			if depth < 2 {
				t.Fatalf("%s at level %d has %d floors; the point of it is depth", kind, level, depth)
			}
			for f := 0; f < depth; f++ {
				l := BuildLocal(poi, floorNamer{}, f)
				last := f+1 == depth

				// One way back, always: the door out on the ground floor and
				// the stairs you came by on every other.
				back := count(l, EExit) + count(l, EShallower)
				if back != 1 {
					t.Errorf("%s floor %d of %d has %d ways back", kind, f, depth, back)
				}
				if f == 0 && count(l, EExit) != 1 {
					t.Errorf("%s ground floor does not open onto the world", kind)
				}
				if f > 0 && count(l, EShallower) != 1 {
					t.Errorf("%s floor %d has no stairs back", kind, f)
				}

				// One way on, except at the end.
				if want := 1; !last && count(l, EDeeper) != want {
					t.Errorf("%s floor %d of %d has %d ways on, want %d",
						kind, f, depth, count(l, EDeeper), want)
				}
				if last && count(l, EDeeper) != 0 {
					t.Errorf("%s last floor still leads somewhere", kind)
				}

				// The boss and the hoard are the end of the place and nowhere
				// else. A hoard on the first floor is the walk made pointless.
				if got := count(l, EBoss); (got > 0) != last {
					t.Errorf("%s floor %d of %d has %d bosses", kind, f, depth, got)
				}
				if got := count(l, EHoard); (got > 0) != last {
					t.Errorf("%s floor %d of %d has %d hoards", kind, f, depth, got)
				}
			}
		}
	}
}

// A floor is the same floor every time it is walked into, and is not the floor
// above it.
//
// Both halves matter and they fail differently. Interiors are rebuilt from the
// location's seed on every entry, so a floor that regenerated would refill its
// chests each time you took the stairs; and a floor derived by forking on the
// floor number alone would be identical everywhere, because core.RNG.Fork
// ignores its receiver — which is a gotcha this repository has already written
// down once.
func TestAFloorIsStableAndItsOwn(t *testing.T) {
	poi := deepPOI(KindTower, 31, 10)
	depth := Depth(poi)
	sig := func(l *LocalMap) string {
		s := ""
		for _, e := range l.Entities {
			s += fmt.Sprintf("%s%v;", e.Kind, e.Pos)
		}
		return s
	}
	seen := map[string]int{}
	for f := 0; f < depth; f++ {
		a := sig(BuildLocal(poi, floorNamer{}, f))
		b := sig(BuildLocal(poi, floorNamer{}, f))
		if a != b {
			t.Errorf("floor %d was built differently the second time", f)
		}
		if was, dup := seen[a]; dup {
			t.Errorf("floor %d is identical to floor %d", f, was)
		}
		seen[a] = f
	}

	// And two towers are not the same tower, which is the failure a shared
	// fork salt would produce and which the per-floor check above cannot see.
	other := deepPOI(KindTower, 32, 10)
	if sig(BuildLocal(poi, floorNamer{}, 1)) == sig(BuildLocal(other, floorNamer{}, 1)) {
		t.Error("two different towers have the same first floor")
	}
}

// Spending something on one floor must not spend it on another.
//
// The used-key was kind and position, and floors put the same position on every
// storey — so opening the chest at (10,10) on the ground floor would have
// emptied the one at (10,10) three floors up, before the player ever saw it.
// A save written before floors existed carries no floor at all, and a nought
// there is the ground floor, which is what those places had.
func TestSpendingIsPerFloor(t *testing.T) {
	poi := deepPOI(KindCave, 5, 9)
	poi.MarkUsed("chest", core.Point{X: 10, Y: 10}, 0)

	if !poi.IsUsed("chest", core.Point{X: 10, Y: 10}, 0) {
		t.Error("a chest opened on the ground floor did not stay opened")
	}
	if poi.IsUsed("chest", core.Point{X: 10, Y: 10}, 2) {
		t.Error("opening a chest on one floor emptied the one below it")
	}
}

// Somewhere with one floor still has one floor.
//
// Depth is the switch the whole feature hangs off, and everything that is not a
// tower or a cave must come back one — a settlement that grew a staircase would
// put a flight of stairs in a market square.
func TestOnlyTheDeepPlacesAreDeep(t *testing.T) {
	for _, kind := range []POIKind{
		KindCapital, KindTown, KindVillage, KindCastle,
		KindRuin, KindShrine, KindCamp, KindOddity, KindDungeon,
	} {
		if d := Depth(deepPOI(kind, 3, 14)); d != 1 {
			t.Errorf("%s has %d floors; only towers and caves go deeper", kind, d)
		}
	}
}

// A wayside is somewhere you are safe, and somewhere with somebody in it.
//
// Both halves are the point of it. It is reached by walking into a green mark
// on the road, so it has to pay out — a clearing with nothing in it is the
// marker lying — and it must not then ambush you, which is a real hazard rather
// than a hypothetical: the interior ambush used to ask the location's *kind*
// whether the place was a settlement, and a wayside is built on a KindCamp,
// which is not one.
func TestAWaysideIsSafeAndWorthFinding(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		l := BuildWayside(seed, 5, floorNamer{})
		if !l.Peaceful {
			t.Fatalf("seed %d: a fire with people round it is not marked peaceful", seed)
		}
		if count(l, EExit) != 1 {
			t.Errorf("seed %d: %d ways back to the road", seed, count(l, EExit))
		}
		// Somebody to trade with, always. That is the reason for the walk.
		if count(l, EShop) != 1 {
			t.Errorf("seed %d: %d people selling anything", seed, count(l, EShop))
		}
		// And nothing that fights, ever. A boon that turns out to hold a
		// monster is the mark meaning two things.
		for _, k := range []EntityKind{EFoe, EBoss, EDeeper, EShallower} {
			if got := count(l, k); got != 0 {
				t.Errorf("seed %d: a wayside has %d %s", seed, got, k)
			}
		}
		// Everything in it has to be standing somewhere you can reach.
		for _, e := range l.Entities {
			if !l.At(e.Pos.X, e.Pos.Y).Info().Passable {
				t.Errorf("seed %d: %s is standing in the scenery at %v", seed, e.Kind, e.Pos)
			}
		}
	}
}

// Two waysides are two different waysides.
//
// Nothing about one is saved — it is weather rather than furniture, the same
// argument the wanderer type makes — so the seed is drawn fresh each time and
// the only thing that would make them all the same is a generator that ignores
// it.
func TestNoTwoWaysidesAreTheSame(t *testing.T) {
	sig := func(l *LocalMap) string {
		s := ""
		for _, e := range l.Entities {
			s += fmt.Sprintf("%s%v;", e.Kind, e.Pos)
		}
		return s
	}
	seen := map[string]bool{}
	same := 0
	for seed := int64(1); seed <= 30; seed++ {
		k := sig(BuildWayside(seed, 5, floorNamer{}))
		if seen[k] {
			same++
		}
		seen[k] = true
	}
	if same > 2 {
		t.Errorf("%d of 30 waysides were repeats of one already generated", same)
	}
}
