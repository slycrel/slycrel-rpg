package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The town goes to bed. The inn does not.
//
// Townsfolk and hirelings clearing off the street after dark is what makes a
// settlement feel like it keeps hours, and the comment on abed has always said
// the inn is the one thing that is definitely open — which was free to say
// while a town had no indoors. Now that the room behind the door is a map, the
// same rule emptied the taproom at dusk: a player who walked in at midnight
// found a lit fire, a laid bar and nobody at it, and the hireling they had come
// to town for had gone home from a building they were already inside.
func TestNobodyGoesHomeFromInsideTheInn(t *testing.T) {
	poi := &world.POI{Kind: world.KindTown, Seed: 77, Level: 3, Name: "Town", Tag: "tag"}
	g := storyGame(t)
	street := world.BuildLocal(poi, g.Write, 0)
	g.Clock = sky.Clock{Step: 400} // after nightAt
	if !g.Clock.Phase().Dark() {
		t.Fatal("the clock is not set to a dark phase, so this test proves nothing")
	}

	// Out on the cobbles, after dark, the street is empty of people.
	g.Local = street
	var folk *world.Entity
	for _, e := range street.Entities {
		if e.Kind == world.ENPC {
			folk = e
			break
		}
	}
	if folk == nil {
		t.Fatal("the town generated with nobody in it")
	}
	if !g.abed(folk) {
		t.Error("a townsperson is still standing in the street at night")
	}

	// Through the inn's door, the same hour, everyone is where they were.
	var door *world.Entity
	for _, e := range street.Entities {
		if e.Kind == world.EShopDoor && e.Shop == world.ShopInn {
			door = e
			break
		}
	}
	if door == nil {
		t.Fatal("the town generated without an inn")
	}
	g.Local = world.BuildShopRoom(poi, g.Write, door.Shelf, door.Shop, door.Name)
	seen := 0
	for _, e := range g.Local.Entities {
		switch e.Kind {
		case world.ENPC, world.ERecruit:
			seen++
			if g.abed(e) {
				t.Errorf("the %s in the taproom went home at %v", e.Kind, g.Clock.Phase().Name())
			}
		}
	}
	if seen == 0 {
		t.Fatal("the taproom generated empty, so this test proves nothing")
	}
}

// It does not rain on the tables.
//
// Which half of drawSky runs is decided by one boolean, and the call site
// worked it out from the POI kind — asking whether the *town* has a roof when
// the question is about the room. Everything in a settlement therefore got
// weather, the taproom included.
func TestTheSkyIsAskedOfTheMapNotTheLocation(t *testing.T) {
	g := storyGame(t)
	poi := &world.POI{Kind: world.KindTown, Seed: 77, Level: 3, Name: "Town", Tag: "tag"}
	if world.BuildLocal(poi, g.Write, 0).Indoors {
		t.Error("the street has a roof over it")
	}
	if !world.BuildShopRoom(poi, g.Write, 0, world.ShopInn, "Inn").Indoors {
		t.Error("it rains in the taproom")
	}
}
