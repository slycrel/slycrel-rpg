package world

import "github.com/slycrel/slycrel-rpg/internal/core"

// The inn's common room.
//
// Every other shop in a town is a counter with a person behind it: you walk in,
// you buy, you leave, and the room is a frame around a menu. An inn is the one
// that has always been more than that, because an inn is where the game puts
// the people it has nowhere else to put — the hireling who used to loiter in
// the street outside because there was no inside, and the townsfolk who say one
// line each and had to say it while standing in a road.
//
// So this room is laid out to be *entered* rather than transacted with: a bar
// along the top with stools at it, tables in the middle with people drinking at
// them, a fire on one wall, and trophies over the bar. None of it is
// interactive except the people, and that is the point — the furniture is there
// so that the people have somewhere to be.
const (
	innW = 19
	innH = 13

	// The bar, and the row of stools in front of it.
	barY    = 3
	stoolY  = barY + 1
	tableY  = 8
	hearthY = 8
)

// tapFrames are the frames of the 32-pixel furnishing sheet this room is built
// out of, named because a bare 53 in a layout is a number nobody can check.
//
// The sheet is sixteen columns wide, so a frame is row*16+col; see
// assets-raw/.../cozy furnishings 32x32.png.
const (
	frameRoundTable = 53  // row 3, col 5
	frameBench      = 52  // row 3, col 4
	frameCounter    = 49  // row 3, col 1
	frameHearth     = 112 // row 7, col 0
	frameAntlers    = 2   // row 0, col 2
	frameBoarHead   = 3   // row 0, col 3
	frameSausages   = 4   // row 0, col 4
	frameCrossSword = 9   // row 0, col 9
)

// tapTrophies is what hangs on the wall behind the bar. Drawn from the 32-pixel
// sheet and anchored a row down from the wall, so each one rides up over the
// stonework the way a mounted head does.
var tapTrophies = []int{frameBoarHead, frameAntlers, frameSausages, frameCrossSword}

// tapDrinkers is the pool for somebody sitting at a table with a cup. It is one
// sheet because it is the only sheet in the pack of a person sitting down; the
// variety in a room of them comes from where they sit and what they say.
var tapDrinkers = []string{"npc/monksittingdrinking_idle"}

// buildTaproom furnishes the room behind an inn's door and fills it.
//
// l is already a walled box of innW by innH with a floor, a door in the bottom
// wall and an exit standing on it.
func buildTaproom(g *core.RNG, l *LocalMap, wr Namer, name string) {
	l.Furnished = true

	// The bar: a solid run across the room with one way through it, and the
	// keeper on the far side. The same shape the other shops use, because it is
	// the shape that says at a glance which half of the room is yours — but
	// made of wood rather than of wall, because the one object that tells a
	// player which room they have walked into should not be a stone parapet.
	gap := g.Between(2, innW-3)
	for x := 1; x < innW-1; x++ {
		if x != gap {
			l.set(x, barY, LFurniture)
		}
	}
	keeper := core.Point{X: core.Clamp(gap, 2, innW-3), Y: barY - 1}
	if keeper.X == gap {
		keeper.X = core.Clamp(gap+1, 2, innW-3)
	}
	l.Entities = append(l.Entities, &Entity{
		Kind: EInn, Pos: keeper, Name: name, Shop: ShopInn,
		Line: wr.NPCLine(g), Sprite: shopSprites[ShopInn],
	})

	// The counter itself, laid over that run in two-tile lengths that meet
	// exactly: a 32-pixel sprite centred on a 16-pixel tile spans from half a
	// tile left of it to one and a half right, so segments two apart tile
	// seamlessly. Appended after the keeper so it draws over them — somebody
	// standing behind a bar is somebody you can see the top half of.
	for x := 1; x < innW-2; x += 2 {
		if x == gap || x+1 == gap {
			continue
		}
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: core.Point{X: x, Y: barY}, Name: "the bar",
			Sprite: "prop/cozy32", Still: true, Frame: frameCounter,
		})
	}

	// What is behind the bar and what is over it. Bottles from the small sheet
	// sit on the back shelf; trophies from the large one hang above them.
	stock := shopStock[ShopInn]
	for i, x := 0, 2; x < innW-2; x, i = x+2, i+1 {
		if x == keeper.X {
			continue
		}
		if i%3 == 0 {
			l.Entities = append(l.Entities, &Entity{
				Kind: EDecor, Pos: core.Point{X: x, Y: 1}, Name: "a trophy",
				Sprite: "prop/cozy32", Still: true,
				Frame: tapTrophies[(i/3)%len(tapTrophies)],
			})
			continue
		}
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: core.Point{X: x, Y: 1}, Name: "stock",
			Sprite: "prop/cozy16", Still: true, Frame: stock[i%len(stock)],
		})
	}

	// Stools along the customers' side of the bar. They are scenery: a stool
	// you can walk through is a stool, and a stool you cannot walk through is a
	// wall in the one part of the room a player has to cross to reach the
	// keeper.
	for x := 3; x < innW-2; x += 2 {
		if x == gap || x == gap-1 || x == gap+1 {
			continue
		}
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: core.Point{X: x, Y: stoolY}, Name: "a stool",
			Sprite: "npc/tabouret", Still: true,
		})
	}

	// The fire, on the left wall, where a fire goes.
	l.set(2, hearthY, LFurniture)
	l.set(3, hearthY, LFurniture)
	l.Entities = append(l.Entities, &Entity{
		Kind: EDecor, Pos: core.Point{X: 2, Y: hearthY}, Name: "the fire",
		Sprite: "prop/cozy32", Still: true, Frame: frameHearth,
	})

	// Two tables, each with somebody at it. A round table drawn from the large
	// sheet covers the square it stands on and the one to its right, so both
	// are walled off and the seats go either side of the pair.
	for _, tx := range []int{7, 13} {
		l.set(tx, tableY, LFurniture)
		l.set(tx+1, tableY, LFurniture)
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: core.Point{X: tx, Y: tableY}, Name: "a table",
			Sprite: "prop/cozy32", Still: true, Frame: frameRoundTable,
		})
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: core.Point{X: tx, Y: tableY + 2}, Name: "a bench",
			Sprite: "prop/cozy32", Still: true, Frame: frameBench,
		})
		for _, sx := range []int{tx - 1, tx + 2} {
			l.Entities = append(l.Entities, &Entity{
				Kind: ENPC, Pos: core.Point{X: sx, Y: tableY}, Name: wr.PersonName(g),
				Line: wr.NPCLine(g), Sprite: core.Pick(g, tapDrinkers),
			})
		}
	}

	// And the reason to come in. One hireling, at the quiet end of the bar,
	// where the person who wants to be hired sits in every story that has ever
	// had one.
	seat := core.Point{X: innW - 3, Y: stoolY}
	if seat.X == gap {
		seat.X--
	}
	l.Entities = append(l.Entities, rollRecruit(g, wr, seat))
}

// rollRecruit is somebody available for money.
//
// One roller rather than the two near-identical blocks that used to sit in the
// settlement and the wayside, because the odds on the interesting half — how
// often a hireling is not entirely a person — were written down twice and had
// already drifted apart once.
func rollRecruit(g *core.RNG, wr Namer, at core.Point) *Entity {
	class := core.Pick(g, recruitClasses)
	look := core.Pick(g, recruitLooks[class])
	// Roughly one hireling in three is not entirely a person. They are the ones
	// going cheap, because nobody else in town will take them.
	blood := ""
	if g.Chance(0.35) {
		blood = core.Pick(g, recruitBloods)
	}
	return &Entity{
		Kind: ERecruit, Pos: at, Name: wr.PersonName(g),
		Line:   wr.RecruitPitch(g, blood),
		Class:  class,
		Blood:  blood,
		Look:   look,
		Sprite: look + "/idle",
	}
}
