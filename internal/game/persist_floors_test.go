package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// What was spent on the third floor stays spent on the third floor.
//
// world.UsedKey carries a Floor, and the comment on it says exactly why: a
// chest at (10,10) on the second storey of a tower is not the chest at (10,10)
// on the first, and without it opening one empties both. save.UsedEntity does
// not carry a Floor at all, so both halves of the round trip drop it — every
// mark is written floorless and read back as the ground floor.
//
// Which means the invariant that comment is defending holds right up until the
// player saves. Three features are affected and all three of them shipped
// after the field did: the floors of towers and caves, the rooms behind a
// town's doors, and what a household remembers.
func TestWhatWasSpentUpstairsStaysSpentUpstairs(t *testing.T) {
	g := storyGame(t)
	poi := g.World.POIs[0]
	at := core.Point{X: 10, Y: 10}

	// The same square, spent on three different levels of the same location:
	// the ground floor, the third storey, and the room behind a door.
	floors := []int{0, 2, world.RoomFloorOf(2)}
	for _, f := range floors {
		poi.MarkUsed("chest", at, f)
	}

	if err := g.Restore(g.Snapshot()); err != nil {
		t.Fatalf("a save this game just wrote would not load: %v", err)
	}

	back := g.World.POIs[0]
	for _, f := range floors {
		if !back.IsUsed("chest", at, f) {
			t.Errorf("the chest at %v on floor %d came back unopened", at, f)
		}
	}
	// And nothing was invented on a floor nobody spent anything on, which is
	// the other half of the same bug: a mark flattened to the ground floor does
	// not only lose its own address, it takes somebody else's.
	if back.IsUsed("chest", at, 1) {
		t.Errorf("a chest was marked used on floor 1, where nothing was opened")
	}
}

// The same thing, through the feature that most obviously shows it.
//
// A hireling is hired in the inn, which is a room with a floor number of its
// own so that spending them does not also delete whatever stands on that square
// out in the street. Save and load, and both halves of that come apart at once:
// they are back on the stool, and the street has lost somebody.
func TestAHirelingHiredInTheInnIsStillHiredAfterALoad(t *testing.T) {
	g := storyGame(t)
	poi := townWithAnInn(t, g)

	g.townPOI = poi
	g.Local = world.BuildLocal(poi, g.Write, 0)
	g.floor = 0
	door := doorTo(g.Local, world.ShopInn)
	if door == nil {
		t.Fatal("the town generated without an inn")
	}
	streetBefore := len(liveEntities(g.Local))

	g.enterRoom(door)
	recruit := firstOfKind(g.Local, world.ERecruit)
	if recruit == nil {
		t.Fatal("nobody is for hire in the taproom")
	}
	seat := recruit.Pos
	g.spend(recruit)
	g.leaveRoom()

	if err := g.Restore(g.Snapshot()); err != nil {
		t.Fatalf("a save this game just wrote would not load: %v", err)
	}

	// Find the same town again: Restore rebuilds the world from the seed, so
	// the POI pointers are new.
	var after *world.POI
	for _, p := range g.World.POIs {
		if p.Name == poi.Name && p.Kind == poi.Kind {
			after = p
		}
	}
	if after == nil {
		t.Fatal("the town is not on the continent after a load")
	}
	if !after.IsUsed(string(world.ERecruit), seat, world.RoomFloorOf(door.Shelf)) {
		t.Errorf("the hireling is back on the stool at %v after a load", seat)
	}
	street := world.BuildLocal(after, g.Write, 0)
	if got := len(liveEntities(street)); got != streetBefore {
		t.Errorf("loading removed %d thing(s) from the street: a hireling hired "+
			"indoors was filed at the same address as a square outside",
			streetBefore-got)
	}
}

// A save with a hole in it loads anyway.
//
// Three of the lists in a save file are slices of *pointers* — the hirelings,
// the errands and the backstories — and a null in any of them unmarshals to a
// nil that nothing checks. Restore hands the allies straight through, and the
// first thing it does afterwards is walk them to catch up their threads, which
// dereferences the lot. So one bad row in a file takes the whole run down, on
// load, with a segfault rather than a message.
//
// The player is checked, and has been since the format existed. These three
// were not, and they are the ones a save is most likely to have grown or lost
// a row in.
//
// Dropped rather than refused, for the reason save.List already gives about
// unreadable files: a corrupt save should not make the game unusable. A
// hireling who is a null is a hireling who is gone, which is a state the game
// has a word for.
func TestASaveWithAHoleInItStillLoads(t *testing.T) {
	g := storyGame(t)
	f := g.Snapshot()

	f.Allies = append(f.Allies, nil)
	f.Quests = append(f.Quests, nil)
	f.Threads = append(f.Threads, nil)
	live := len(f.Allies) - 1

	if err := g.Restore(f); err != nil {
		t.Fatalf("a save with one null row in it was refused: %v", err)
	}
	for i, c := range g.Allies {
		if c == nil {
			t.Fatalf("ally %d is nil after a load; the next thing to touch it "+
				"takes the process down", i)
		}
	}
	if len(g.Allies) != live {
		t.Errorf("%d allies loaded, %d were real", len(g.Allies), live)
	}
	for i, q := range g.Quests.Quests {
		if q == nil {
			t.Fatalf("errand %d is nil after a load", i)
		}
	}
	for i, th := range g.Threads.Threads {
		if th == nil {
			t.Fatalf("backstory %d is nil after a load", i)
		}
	}
}
