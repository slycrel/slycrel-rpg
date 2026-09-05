package game

import (
	"image/color"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Going in through a door in a town and coming back out.
//
// A room behind a door is a map inside a map, and the way it is held is the way floors
// are held: there is one g.Local, and moving between rooms replaces it and
// rebuilds the other one on the way back. Interiors are deterministic from the
// location's seed, so rebuilding is lossless — anything spent replays from the
// POI's own record — and the alternative, a stack of live maps, would be a
// second lifetime to get wrong for no gain.
//
// **The save format learns nothing.** A party standing in a shop is recorded as
// standing at the door of it, because the door is where walking out puts them
// and a save is a place you come back to rather than a photograph. That is one
// field on Game and no fields anywhere else.

// enterRoom swaps the town for the room behind one of its doors.
func (g *Game) enterRoom(e *world.Entity) {
	if g.Local == nil || g.Local.POI == nil {
		return
	}
	g.roomReturn = g.LocalWalk.Tile
	g.inRoom = true
	// The room gets a floor of its own so that what is spent in it — a hireling
	// taken off a stool in the inn — is recorded at an address the street does
	// not also use. See world.RoomFloorOf and Game.here.
	g.floor = world.RoomFloorOf(e.Shelf)
	if e.Kind == world.EHouseDoor {
		g.Local = world.BuildHouseRoom(g.Local.POI, g.Write, e.Shelf)
	} else {
		g.Local = world.BuildShopRoom(g.Local.POI, g.Write, e.Shelf, e.Shop, e.Name)
	}
	g.LocalWalk = core.NewWalker(7)
	g.LocalWalk.Place(g.Local.Entry)
	g.reformLines()
	g.localFollow.Place(g.Local.Entry)
	g.Sound.Play("world/enter")
	g.Push(newLocalScene(g))
}

// leaveRoom rebuilds the settlement and puts the party back on its doorstep.
func (g *Game) leaveRoom() {
	poi := g.townPOI
	if poi == nil {
		// Nothing to go back to, which should not happen and would strand the
		// party in a room with a door that does nothing. Out to the overworld
		// is the honest failure.
		g.inRoom = false
		g.floor = 0
		g.Local = nil
		g.Pop()
		return
	}
	g.inRoom = false
	g.floor = 0
	g.Local = world.BuildLocal(poi, g.Write, 0)
	g.LocalWalk = core.NewWalker(7)
	g.LocalWalk.Place(g.roomReturn)
	g.reformLines()
	g.localFollow.Place(g.roomReturn)
	g.Sound.Play("world/enter")
	g.Pop()
	g.Log.AddColor(render.ColInkDim, "Back on the street.")
}

// shopDoorColour is the lintel over a shop's door, per trade.
//
// The same four colours the interface already uses for these trades, so a
// player who has learned that blue is the smith on one screen has learned it on
// the other. A door with no colour is a door nobody can pick out of a street of
// buildings, which is what the shopkeeper standing in it used to be for.
func shopDoorColour(k world.ShopKind) color.RGBA {
	switch k {
	case world.ShopSmith:
		return color.RGBA{0x60, 0xA0, 0xE0, 0xFF}
	case world.ShopArmorer:
		return color.RGBA{0xA0, 0xA8, 0xB8, 0xFF}
	case world.ShopApothecary:
		return color.RGBA{0x70, 0xC0, 0x78, 0xFF}
	case world.ShopInn:
		return color.RGBA{0xE0, 0x90, 0x50, 0xFF}
	}
	return color.RGBA{0xC0, 0xB0, 0x90, 0xFF}
}
