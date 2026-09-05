package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/world"
)

// A hireling taken out of the inn stays taken.
//
// Everything a player spends in a location is recorded against that location's
// POI and replayed the next time it is built. A shop room is built from the
// town's seed but carries a POI of its own — a synthetic record holding the
// shop's name so the status bar can say "Inn" — which is not in the world's
// list, so anything written to it is written to nothing. While the only thing
// worth spending in a town was out on the street this could not come up. The
// day the hireling moved indoors it did: hire them, walk out, walk back in, and
// there they are on the same stool, ready to be hired again.
//
// Two halves have to hold for that not to happen. The town has to be the ledger
// rather than the room (Game.here), and the room needs an address of its own
// (world.ShopFloorOf) — without the second, a hireling hired at {16,4} in the
// inn would also delete whatever stands on {16,4} out in the street.
func TestAHirelingHiredInTheInnIsGoneWhenYouComeBack(t *testing.T) {
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

	g.enterShop(door)
	recruit := firstOfKind(g.Local, world.ERecruit)
	if recruit == nil {
		t.Fatal("nobody is for hire in the taproom")
	}
	seat := recruit.Pos
	g.spend(recruit)

	// Out and back in. The room is rebuilt from the seed either way; what has
	// to survive is the record of what happened in it.
	g.leaveShop()
	if got := len(liveEntities(g.Local)); got != streetBefore {
		t.Errorf("hiring somebody indoors removed %d thing(s) from the street",
			streetBefore-got)
	}
	g.enterShop(door)
	if again := firstOfKind(g.Local, world.ERecruit); again != nil {
		t.Errorf("the hireling is back on the stool at %v, offering again", again.Pos)
	}
	// And it is that seat that was spent, not the room wholesale.
	if !poi.IsUsed(string(world.ERecruit), seat, world.ShopFloorOf(door.Shelf)) {
		t.Errorf("the town has no record of a hireling leaving %v", seat)
	}
	if poi.IsUsed(string(world.ERecruit), seat, 0) {
		t.Errorf("a hireling hired in the inn was filed at %v on the street", seat)
	}
}

// townWithAnInn finds a settlement big enough to have one. A village gets a
// smith and an apothecary and nothing else.
func townWithAnInn(t *testing.T, g *Game) *world.POI {
	t.Helper()
	for _, p := range g.World.POIs {
		if !p.Kind.Settlement() {
			continue
		}
		if doorTo(world.BuildLocal(p, g.Write, 0), world.ShopInn) != nil {
			return p
		}
	}
	t.Fatal("no settlement on the continent has an inn")
	return nil
}

func doorTo(l *world.LocalMap, kind world.ShopKind) *world.Entity {
	for _, e := range l.Entities {
		if e.Kind == world.EShopDoor && e.Shop == kind {
			return e
		}
	}
	return nil
}

func firstOfKind(l *world.LocalMap, kind world.EntityKind) *world.Entity {
	for _, e := range l.Entities {
		if e.Kind == kind && !e.Used {
			return e
		}
	}
	return nil
}

func liveEntities(l *world.LocalMap) []*world.Entity {
	var out []*world.Entity
	for _, e := range l.Entities {
		if !e.Used {
			out = append(out, e)
		}
	}
	return out
}
