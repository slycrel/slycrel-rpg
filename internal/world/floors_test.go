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

// A shop is a room you can cross, with somebody to trade with and a way out.
//
// The room replaced a cupboard: a town's buildings have had doors since they
// were generated, and what was behind one was three to six tiles by one to
// three, with the keeper standing in it because there was nowhere else. The
// things that have to be true of the replacement are the things that were not
// true of that.
func TestAShopIsARoomWithSomebodyInIt(t *testing.T) {
	poi := &POI{Kind: KindTown, Seed: 21, Level: 5, Name: "Town", Tag: "tag"}
	for _, kind := range []ShopKind{ShopSmith, ShopArmorer, ShopApothecary, ShopInn} {
		for idx := 0; idx < 4; idx++ {
			l := BuildShopRoom(poi, floorNamer{}, idx, kind, "The Shop")

			if !l.Peaceful {
				t.Errorf("%s: a room in a town is not marked peaceful", kind)
			}
			if count(l, EExit) != 1 {
				t.Errorf("%s: %d ways out", kind, count(l, EExit))
			}
			// Exactly one counter, and it is the right sort: an inn takes a
			// bed and everything else takes a shelf.
			keepers := count(l, EShop) + count(l, EInn)
			if keepers != 1 {
				t.Errorf("%s: %d people behind the counter", kind, keepers)
			}
			if kind == ShopInn && count(l, EInn) != 1 {
				t.Errorf("an inn's counter is a %v, not a bed", EShop)
			}
			if kind != ShopInn && count(l, EShop) != 1 {
				t.Errorf("%s sells from a bed", kind)
			}
			// An inn is the room the game puts people in. The hireling used to
			// stand in the street outside it, where a player who never walked
			// that particular stretch of cobbles never met one; the point of
			// the room is that they are in it, so that is what is checked.
			if kind == ShopInn {
				if count(l, ERecruit) != 1 {
					t.Errorf("an inn with %d hirelings in it", count(l, ERecruit))
				}
				if n := count(l, ENPC); n < 2 {
					t.Errorf("a taproom with %d drinkers in it", n)
				}
			} else if count(l, ERecruit) != 0 {
				t.Errorf("%s: a hireling is loitering in a shop", kind)
			}
			// Stock on the wall, which is what says what the trade is before
			// anybody speaks.
			if count(l, EDecor) == 0 {
				t.Errorf("%s has nothing on its walls", kind)
			}
			// Nobody stands inside the furniture, and everybody can be
			// walked up to.
			//
			// Decor is exempt from the first half and only from the first
			// half: a table is a solid tile with a picture of a table on it,
			// which is how a room gets furniture you cannot walk through. A
			// *person* on a solid tile is a person nobody can reach.
			for _, e := range l.Entities {
				if e.Kind == EDecor {
					continue
				}
				if !l.At(e.Pos.X, e.Pos.Y).Info().Passable {
					t.Errorf("%s: %s is inside the furniture at %v", kind, e.Kind, e.Pos)
				}
				// And the room is a room: everything worth walking up to has
				// to be reachable from the door, which the counter and the
				// furniture must therefore not seal off.
				if !reaches(l, l.Entry, e.Pos) {
					t.Errorf("%s: %s at %v is walled off from the door", kind, e.Kind, e.Pos)
				}
			}
			if keeperAt(l).X < 0 {
				t.Errorf("%s: nobody behind the counter at all", kind)
			}
		}
	}
}

func keeperAt(l *LocalMap) core.Point {
	for _, e := range l.Entities {
		if e.Kind == EShop || e.Kind == EInn {
			return e.Pos
		}
	}
	return core.Point{X: -1}
}

// reaches is a flood fill from one tile to another over passable tiles.
//
// A person is not a wall, and it does not need to be told so: people stand on
// floor, which is half of what the caller is checking. Furniture is: a table is
// an impassable tile with a picture of a table on it, so a fill that stepped
// over anything with an entity on it would walk through the tables and call a
// room crossable that is not.
func reaches(l *LocalMap, from, to core.Point) bool {
	if to.X < 0 {
		return false
	}
	seen := map[core.Point]bool{from: true}
	queue := []core.Point{from}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p == to {
			return true
		}
		for _, d := range []core.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			n := core.Point{X: p.X + d.X, Y: p.Y + d.Y}
			if seen[n] || !l.At(n.X, n.Y).Info().Passable {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return false
}

// What has a roof over it, and what only looks like it does.
//
// Indoors decides two things a player notices immediately — whether it rains on
// them and whether the people are still there after dark — and for most of its
// life it decided neither, because it was set by three builders and read by
// nobody. The drawing code asked the POI kind instead, which gets the two
// interesting cases backwards: a town is outdoors and the room behind its inn
// door is not, and both are the same POI.
func TestARoofIsAPropertyOfTheMapAndNotOfTheKind(t *testing.T) {
	roofed := map[POIKind]bool{KindDungeon: true, KindCave: true, KindTower: true}
	kinds := []POIKind{
		KindCapital, KindTown, KindVillage, KindCastle,
		KindDungeon, KindCave, KindTower, KindOddity, KindCamp, KindRuin,
	}
	for _, k := range kinds {
		poi := &POI{Kind: k, Seed: 4242, Level: 3, Name: "Somewhere", Tag: "tag"}
		if got := BuildLocal(poi, floorNamer{}, 0).Indoors; got != roofed[k] {
			t.Errorf("%s is built indoors=%v, want %v", k, got, roofed[k])
		}
	}
	// And every room behind a door is, whatever the town around it is.
	town := &POI{Kind: KindTown, Seed: 4242, Level: 3, Name: "Town", Tag: "tag"}
	for _, kind := range []ShopKind{ShopSmith, ShopArmorer, ShopApothecary, ShopInn} {
		if !BuildShopRoom(town, floorNamer{}, 0, kind, "The Shop").Indoors {
			t.Errorf("the %s is a room in a town with the sky in it", kind)
		}
	}
}

// Everything a town built that is not a trade is somebody's house, and behind
// the door is a room.
//
// The leftover buildings were scenery with a three-tile alcove in them that a
// player could step into and find nothing, because there was nothing to find.
// The two halves that have to hold: every one of them opens onto a room, and no
// two of them open onto the *same* room — the door carries an index and that
// index is both the seed of what is behind it and the address anything spent in
// there is filed under, so a collision would make two houses one house.
func TestEveryBuildingThatIsNotATradeIsSomebodysHouse(t *testing.T) {
	// A castle is the exception, and on purpose. All four trades fit in one
	// before anything is left over, and how many buildings a plot takes is a
	// rejection loop against a street cross — on some seeds a castle is four
	// buildings and all four of them are counters. Somewhere with no houses in
	// it is a fort, which is a reasonable thing for a castle to be; somewhere
	// with buildings it did not put anybody in is the bug this is looking for.
	homes := map[POIKind]bool{KindCapital: true, KindTown: true, KindVillage: true}
	for _, kind := range []POIKind{KindCapital, KindTown, KindVillage, KindCastle} {
		poi := &POI{Kind: kind, Seed: 909, Level: 4, Name: "Somewhere", Tag: "tag"}
		l := BuildLocal(poi, floorNamer{}, 0)

		shelves := map[int]EntityKind{}
		houses := 0
		for _, e := range l.Entities {
			switch e.Kind {
			case EShopDoor, EHouseDoor:
			default:
				continue
			}
			if was, dup := shelves[e.Shelf]; dup {
				t.Errorf("%s: a %s and a %s both open onto room %d",
					kind, was, e.Kind, e.Shelf)
			}
			shelves[e.Shelf] = e.Kind
			if e.Kind == EHouseDoor {
				houses++
			}
		}
		if houses == 0 && homes[kind] {
			t.Errorf("%s has no houses in it, only shops", kind)
		}
		// And no doorway is a hole in a wall with nothing behind it.
		if t.Failed() {
			continue
		}
		for shelf, k := range shelves {
			if k != EHouseDoor {
				continue
			}
			room := BuildHouseRoom(poi, floorNamer{}, shelf)
			if count(room, EResident) != 1 {
				t.Errorf("%s: house %d has %d people living in it",
					kind, shelf, count(room, EResident))
			}
			if count(room, EExit) != 1 {
				t.Errorf("%s: house %d has %d ways out", kind, shelf, count(room, EExit))
			}
			if !room.Indoors || !room.Peaceful {
				t.Errorf("%s: house %d is indoors=%v peaceful=%v",
					kind, shelf, room.Indoors, room.Peaceful)
			}
			for _, e := range room.Entities {
				if e.Kind == EDecor {
					continue
				}
				if !room.At(e.Pos.X, e.Pos.Y).Info().Passable {
					t.Errorf("%s: house %d has a %s inside the furniture at %v",
						kind, shelf, e.Kind, e.Pos)
				}
				if !reaches(room, room.Entry, e.Pos) {
					t.Errorf("%s: house %d walls its %s off from the door",
						kind, shelf, e.Kind)
				}
			}
		}
	}
}
