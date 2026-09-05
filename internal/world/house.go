package world

import "github.com/slycrel/slycrel-rpg/internal/core"

// Somebody's house.
//
// A settlement puts up more buildings than it has trades, and the ones left
// over have always been scenery: a shell with a three-tile alcove inside it
// that a player could step into and find nothing, because there was nothing to
// find. They are rooms now, for the same reason the shops became rooms — the
// generator can already say where a building is and who is in a town, and what
// it could not say was where those two facts meet.
//
// What is behind the door is one person and their furniture. The person is the
// point: what they do when you walk in is not written here but read off your
// standing, which is the one number in the game that says how a town takes you
// and until now was only ever a line of dialogue. See Game.knock.
const (
	houseW = 13
	houseH = 9
)

// houseBeds, houseTables and houseCorners are the kit, as frames of the
// 32-pixel furnishing sheet. Three slots along the back wall, one pick each, so
// that ten houses in a capital are ten rooms rather than one room ten times.
var (
	houseBeds    = []int{38, 39, 54, 55, 86, 87}
	houseTables  = []int{32, 48, 64, 80}
	houseCorners = []int{22, 24, 60, 76, 16, 112}
)

// BuildHouseRoom is the inside of one of a settlement's ordinary buildings.
//
// idx is the building's index in the town, which is the same number the door
// carries and the same number RoomFloorOf turns into an address — so a house
// and a shop cannot be built from the same stream and cannot record what
// happened in them at the same place.
func BuildHouseRoom(poi *POI, wr Namer, idx int) *LocalMap {
	g := core.NewRNG(poi.Seed + int64(idx)*7717 + 991)
	room := &POI{
		Kind: poi.Kind, Level: poi.Level, Seed: poi.Seed,
		Name: "a house", Tag: "inside",
	}
	l := newLocal(room, houseW, houseH, LWall)
	l.Biome = "plains"
	l.Indoors = true
	l.Peaceful = true
	l.Furnished = true
	l.Floor, l.Depth = RoomFloorOf(idx), 1
	l.rect(1, 1, houseW-2, houseH-2, LFloor)

	door := core.Point{X: houseW / 2, Y: houseH - 2}
	l.set(door.X, houseH-1, LDoor)
	l.Entry = door
	l.Entities = append(l.Entities, &Entity{
		Kind: EExit, Pos: door, Name: "the door", Line: "Back to the street.",
	})

	// Three pieces along the back wall, two tiles apiece: a bed, a table, and
	// whatever this household has instead of the other two.
	for _, f := range []struct {
		at    core.Point
		frame int
	}{
		{core.Point{X: 2, Y: 2}, core.Pick(g, houseBeds)},
		{core.Point{X: 5, Y: 2}, core.Pick(g, houseTables)},
		{core.Point{X: 9, Y: 2}, core.Pick(g, houseCorners)},
	} {
		l.set(f.at.X, f.at.Y, LFurniture)
		l.set(f.at.X+1, f.at.Y, LFurniture)
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: f.at, Name: "furniture",
			Sprite: "prop/cozy32", Still: true, Frame: f.frame,
		})
	}

	// And one more piece against a side wall, so that the middle of the room is
	// somewhere the eye stops rather than a square of empty boards. Which wall
	// comes off the seed: the difference between ten houses and one house ten
	// times is entirely in this sort of thing.
	side := core.Point{X: 2, Y: 5}
	if g.Chance(0.5) {
		side.X = houseW - 4
	}
	l.set(side.X, side.Y, LFurniture)
	l.set(side.X+1, side.Y, LFurniture)
	l.Entities = append(l.Entities, &Entity{
		Kind: EDecor, Pos: side, Name: "furniture",
		Sprite: "prop/cozy32", Still: true, Frame: core.Pick(g, houseCorners),
	})

	// And whoever lives here, standing in the middle of their own floor.
	l.Entities = append(l.Entities, &Entity{
		Kind: EResident, Pos: core.Point{X: houseW / 2, Y: 4},
		Name: wr.PersonName(g), Line: wr.NPCLine(g),
		Sprite: core.Pick(g, folkSprites),
	})

	markUsed(l, poi)
	return l
}
