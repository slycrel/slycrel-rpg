package world

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
)

// LocalTile is a cell inside a point of interest.
type LocalTile uint8

// The interior tile set. These map onto assetsys "tile/..." keys via
// localTileInfo, and stay deliberately coarse: interiors are about what is
// standing on them, not the floor.
const (
	LFloor LocalTile = iota
	LGrass
	LCobble
	LWall
	LWater
	LDoor
	LVoid
	LStair
	LRoof
)

// shopSprites maps a merchant to a townsperson sheet that suits the trade.
var shopSprites = map[ShopKind]string{
	ShopSmith:      "npc/shieldman_idle",
	ShopArmorer:    "npc/shieldman_plant_staff",
	ShopApothecary: "npc/librarian_idle",
	ShopInn:        "npc/barman_idle",
}

// folkSprites is the pool for ordinary townspeople.
var folkSprites = []string{
	"npc/monksittingdrinking_idle",
	"npc/librarian_books",
	"npc/monk_surprised",
	"npc/tentaclebutcher_idle",
	"npc/shieldman_surprised",
}

// recruitClasses is what a hireling can be, as model.Class strings.
var recruitClasses = []string{"Fighter", "Thief", "Mage"}

// recruitBloods is the ancestry a hireling can carry, as model.MonsterKind
// strings. It mirrors model.Lineages, which is the list that says what each one
// actually does; anything named here and missing there is simply ignored.
var recruitBloods = []string{"beast", "fey", "undead", "demon", "ooze", "aberrant"}

// recruitLooks are the walk sheets each trade can turn up wearing, keyed by the
// same strings as recruitClasses.
//
// They are the hero sheets rather than the townsperson ones, because a
// companion has to hold up as a follower on the map at the size the player
// does. They are keyed by class rather than picked freely so that somebody
// selling themselves as a mage does not walk over dressed as a swordsman — and
// the druid sheet, which no player class uses, is in the caster's list so that
// a hireling can read as somebody else rather than as a recolour of you.
var recruitLooks = map[string][]string{
	"Fighter": {"hero/fighter"},
	"Thief":   {"hero/thief"},
	"Mage":    {"hero/mage", "hero/druid"},
}

// foeSprites is the pool for the shapes that lurk in interiors.
var foeSprites = []string{
	"foe/ghost/idle",
	"foe/salamander/idle",
	"foe/mage/idle",
	"foe/beaver/walk",
	"foe/necromancer/walk",
}

type localInfo struct {
	Tile     string
	Passable bool
}

var localTileInfo = [...]localInfo{
	LFloor:  {"tile/floor", true},
	LGrass:  {"tile/grassfloor", true},
	LCobble: {"tile/cobble", true},
	LWall:   {"tile/wall", false},
	LWater:  {"tile/river", false},
	LDoor:   {"tile/road", true},
	LVoid:   {"tile/void", false},
	LStair:  {"tile/cobble", true},
	LRoof:   {"tile/roof", false},
}

// Info returns the descriptor for t.
func (t LocalTile) Info() localInfo {
	if int(t) >= len(localTileInfo) {
		return localTileInfo[LVoid]
	}
	return localTileInfo[t]
}

// EntityKind is what an interactable does when you walk into it.
type EntityKind string

// The interactable roster.
const (
	ENPC     EntityKind = "npc"
	EShop    EntityKind = "shop"
	EInn     EntityKind = "inn"
	EChest   EntityKind = "chest"
	ESign    EntityKind = "sign"
	EFoe     EntityKind = "foe"     // a visible wandering monster
	EExit    EntityKind = "exit"    // leave back to the overworld
	EBoss    EntityKind = "boss"    // the thing the dungeon is about
	EAltar   EntityKind = "altar"   // shrines: a blessing with strings attached
	ERecruit EntityKind = "recruit" // someone outside the inn, available for money
)

// Entity is something standing in a local map.
type Entity struct {
	Kind   EntityKind
	Pos    core.Point
	Name   string
	Line   string // dialogue or description
	Sprite string // assetsys key
	Shop   ShopKind
	// Class is the trade a recruit plies, as a model.Class string. It is held
	// untyped because generation must not drag the character model into world
	// building; the hiring code converts it.
	Class string
	// Blood is a recruit's non-human ancestry, as a model.MonsterKind string,
	// empty for the ordinary people who make up most of the roster.
	Blood string
	// Look is a recruit's walk-sheet prefix, e.g. "hero/druid". Sprite is one
	// frame of it for standing in the street; the character carries the prefix
	// so it can face four ways once it is following you around.
	Look string
	Used bool // chests opened, foes killed, altars prayed at
	// Wander is set on foes that move on their own.
	Wander bool
	facing core.Dir
}

// ShopKind selects a merchant's inventory.
type ShopKind string

// The four places that will take your money.
const (
	ShopNone       ShopKind = ""
	ShopSmith      ShopKind = "smith"      // weapons
	ShopArmorer    ShopKind = "armorer"    // armor
	ShopApothecary ShopKind = "apothecary" // potions and junk
	ShopInn        ShopKind = "inn"        // rest
)

// LocalMap is the interior of a point of interest.
type LocalMap struct {
	POI      *POI
	W, H     int
	Tiles    []LocalTile
	Entities []*Entity
	Entry    core.Point
	// Biome selects the monster table for interior encounters.
	Biome string
	// Indoors suppresses the overworld's ambient weather and changes music.
	Indoors bool
}

// At returns the tile at x,y, out-of-bounds reading as void.
func (l *LocalMap) At(x, y int) LocalTile {
	if x < 0 || y < 0 || x >= l.W || y >= l.H {
		return LVoid
	}
	return l.Tiles[y*l.W+x]
}

func (l *LocalMap) set(x, y int, t LocalTile) {
	if x < 0 || y < 0 || x >= l.W || y >= l.H {
		return
	}
	l.Tiles[y*l.W+x] = t
}

// Walkable reports whether a position is open, accounting for blocking
// entities. Foes block so you have to engage them rather than stroll past.
func (l *LocalMap) Walkable(x, y int) bool {
	if !l.At(x, y).Info().Passable {
		return false
	}
	for _, e := range l.Entities {
		if e.Pos.X == x && e.Pos.Y == y && e.Kind == EFoe && !e.Used {
			return false
		}
	}
	return true
}

// EntityAt returns an unused entity standing at x,y.
func (l *LocalMap) EntityAt(x, y int) *Entity {
	for _, e := range l.Entities {
		if e.Pos.X == x && e.Pos.Y == y && !e.Used {
			return e
		}
	}
	return nil
}

func (l *LocalMap) fill(t LocalTile) {
	for i := range l.Tiles {
		l.Tiles[i] = t
	}
}

func (l *LocalMap) rect(x, y, w, h int, t LocalTile) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			l.set(xx, yy, t)
		}
	}
}

// BuildLocal generates the interior of a POI. The result is a pure function of
// poi.Seed, so leaving and re-entering gives you the same town — but the fresh
// RNG fork means it costs nothing to store.
func BuildLocal(poi *POI, w Namer) *LocalMap {
	g := core.NewRNG(poi.Seed)
	var l *LocalMap
	switch poi.Kind {
	case KindCapital, KindTown, KindVillage, KindCastle:
		l = buildSettlement(g, poi, w)
	case KindDungeon, KindCave:
		l = buildDungeon(g, poi, w)
	case KindOddity:
		l = buildOddity(g, poi, w)
	default:
		l = buildSite(g, poi, w)
	}
	// Replay what the player has already dealt with here.
	for _, e := range l.Entities {
		if poi.IsUsed(string(e.Kind), e.Pos) {
			e.Used = true
		}
	}
	return l
}

func newLocal(poi *POI, w, h int, base LocalTile) *LocalMap {
	l := &LocalMap{POI: poi, W: w, H: h, Tiles: make([]LocalTile, w*h)}
	l.fill(base)
	return l
}

// buildSettlement lays out a walled town: a ring wall with one gate, a cross of
// paved streets, and buildings in the quadrants. Shops go in the buildings
// nearest the gate, because that is where a merchant would actually stand.
func buildSettlement(g *core.RNG, poi *POI, wr Namer) *LocalMap {
	size := map[POIKind][2]int{
		KindCapital: {56, 40},
		KindTown:    {44, 32},
		KindVillage: {34, 26},
		KindCastle:  {38, 30},
	}[poi.Kind]
	l := newLocal(poi, size[0], size[1], LGrass)
	l.Biome = "plains"

	// Ring wall with a gate at the bottom.
	for x := 0; x < l.W; x++ {
		l.set(x, 0, LWall)
		l.set(x, l.H-1, LWall)
	}
	for y := 0; y < l.H; y++ {
		l.set(0, y, LWall)
		l.set(l.W-1, y, LWall)
	}
	gateX, gateY := l.W/2, l.H/2

	// Paved main streets, laid before the gates so every gate opens onto one.
	l.rect(gateX-1, 1, 3, l.H-2, LCobble)
	l.rect(1, gateY-1, l.W-2, 3, LCobble)

	// A gate in each wall. The player arrives at the south one — that is where
	// the road is — but a walled town with a single way out means crossing it
	// twice for every errand, and the streets already run to all four sides.
	l.Entry = core.Point{X: gateX, Y: l.H - 2}
	for _, gate := range []struct {
		at   core.Point
		name string
	}{
		{core.Point{X: gateX, Y: l.H - 1}, "south gate"},
		{core.Point{X: gateX, Y: 0}, "north gate"},
		{core.Point{X: 0, Y: gateY}, "west gate"},
		{core.Point{X: l.W - 1, Y: gateY}, "east gate"},
	} {
		l.set(gate.at.X, gate.at.Y, LDoor)
		l.Entities = append(l.Entities, &Entity{
			Kind: EExit, Pos: gate.at,
			Name: gate.name, Line: "Back out into the world.",
		})
	}

	// Buildings, avoiding the streets.
	type building struct{ x, y, w, h int }
	var built []building
	overlaps := func(b building) bool {
		if b.x < 2 || b.y < 2 || b.x+b.w > l.W-2 || b.y+b.h > l.H-2 {
			return true
		}
		// Keep clear of the street cross.
		if b.x <= gateX+2 && b.x+b.w >= gateX-2 {
			return true
		}
		if b.y <= l.H/2+2 && b.y+b.h >= l.H/2-2 {
			return true
		}
		for _, o := range built {
			if b.x < o.x+o.w+2 && b.x+b.w+2 > o.x && b.y < o.y+o.h+2 && b.y+b.h+2 > o.y {
				return true
			}
		}
		return false
	}

	target := map[POIKind]int{KindCapital: 14, KindTown: 9, KindVillage: 5, KindCastle: 7}[poi.Kind]
	for tries := 0; tries < 900 && len(built) < target; tries++ {
		b := building{
			x: g.Between(2, l.W-8), y: g.Between(2, l.H-8),
			w: g.Between(5, 8), h: g.Between(4, 6),
		}
		if overlaps(b) {
			continue
		}
		built = append(built, b)
		l.rect(b.x, b.y, b.w, b.h, LWall)
		l.rect(b.x+1, b.y, b.w-2, 2, LRoof) // a course of tile along the top
		l.rect(b.x+1, b.y+2, b.w-2, b.h-3, LFloor)
		// Door on the wall facing the nearest street.
		dx := b.x + b.w/2
		l.set(dx, b.y+b.h-1, LDoor)
	}

	// Shops: a village gets the essentials, a capital gets everything.
	shops := []struct {
		kind ShopKind
		name string
	}{
		{ShopSmith, "Blacksmith"},
		{ShopApothecary, "Apothecary"},
		{ShopInn, "Inn"},
		{ShopArmorer, "Armorer"},
	}
	if poi.Kind == KindVillage {
		shops = shops[:2]
	}
	inn := core.Point{X: -1}
	for i, s := range shops {
		if i >= len(built) {
			break
		}
		b := built[i]
		door := core.Point{X: b.x + b.w/2, Y: b.y + b.h - 2}
		kind := EShop
		if s.kind == ShopInn {
			kind = EInn
			inn = door
		}
		l.Entities = append(l.Entities, &Entity{
			Kind: kind, Pos: door, Name: s.name, Shop: s.kind,
			Line:   wr.NPCLine(g),
			Sprite: shopSprites[s.kind],
		})
	}

	// Someone loitering outside the inn, available for money. Only settlements
	// big enough to have an inn get one, which gives a village a reason to be
	// somewhere you pass through and a town a reason to be somewhere you stop.
	if inn.X >= 0 {
		if p, ok := openNear(g, l, inn, 2, 5); ok {
			class := core.Pick(g, recruitClasses)
			look := core.Pick(g, recruitLooks[class])
			// Roughly one hireling in three is not entirely a person. They are
			// the ones going cheap, because nobody else in town will take them.
			blood := ""
			if g.Chance(0.35) {
				blood = core.Pick(g, recruitBloods)
			}
			l.Entities = append(l.Entities, &Entity{
				Kind: ERecruit, Pos: p, Name: wr.PersonName(g),
				Line:   wr.RecruitPitch(g, blood),
				Class:  class,
				Blood:  blood,
				Look:   look,
				Sprite: look + "/idle",
			})
		}
	}

	// Townsfolk milling about on the streets.
	folk := map[POIKind]int{KindCapital: 10, KindTown: 7, KindVillage: 4, KindCastle: 6}[poi.Kind]
	for i := 0; i < folk; i++ {
		p, ok := findOpen(g, l, 200)
		if !ok {
			break
		}
		l.Entities = append(l.Entities, &Entity{
			Kind: ENPC, Pos: p, Name: wr.PersonName(g),
			Line: wr.NPCLine(g), Sprite: core.Pick(g, folkSprites),
		})
	}

	// A sign by the gate, because someone always puts a sign by the gate.
	// Nudged off anybody already standing there: it goes up last, so its fixed
	// address was landing on whichever townsperson had wandered to the gate.
	signAt := core.Point{X: gateX + 2, Y: l.H - 3}
	if !elbowRoom(l, signAt) {
		if p, ok := openNear(g, l, signAt, 1, 3); ok {
			signAt = p
		}
	}
	l.Entities = append(l.Entities, &Entity{
		Kind: ESign, Pos: signAt,
		Name: "a weathered sign", Line: wr.SignText(g),
	})
	return l
}

// oddFurniture is the joke zone's kit, and the whole of what makes an oddity a
// place rather than a ruin with a different label.
//
// Everything here is the wrong century and none of it will discuss that. The
// rule the writing follows everywhere else is the load-bearing one: nobody
// standing in an oddity is in on it. The residents are ordinary villagers with
// ordinary sprites who treat a lit humming box as a wall with a slot in it,
// because a game that put a cyberpunk character in the frame would have somebody
// on screen who knows it is funny.
var (
	oddVending = []string{"odd/vending1", "odd/vending2", "odd/vending3", "odd/vending4"}
	oddSigns   = []string{"odd/sign1", "odd/sign2", "odd/sign3", "odd/sign4", "odd/sign5"}
	oddDaubs   = []string{"odd/daub1", "odd/daub2", "odd/daub3", "odd/daub4"}
	oddBins    = []string{"odd/bin1", "odd/bin2", "odd/bin3", "odd/bin4"}
	oddClutter = []string{"odd/barrier1", "odd/barrier2", "odd/lanterns", "odd/car"}
)

// OddityArt is every sprite the joke zone can put on the ground, for the audit.
// Read off the same slices the generator picks from, so the two cannot drift.
func OddityArt() [][]string {
	return [][]string{oddVending, oddSigns, oddDaubs, oddBins, oddClutter, {"odd/metro"}}
}

// buildOddity lays out a short paved strip with the wrong furniture on it.
//
// A street rather than the blob every other small site gets, because the shape
// is half the joke: whatever this was, it was laid out by somebody who expected
// traffic, and the forest has come back up to the kerb on both sides.
func buildOddity(g *core.RNG, poi *POI, wr Namer) *LocalMap {
	l := newLocal(poi, 34, 26, LGrass)
	l.Biome = poiBiome(poi.Kind)

	// The strip: paving down the middle, grass either side, and the ragged
	// edges where it stops for no reason.
	midX := l.W / 2
	l.rect(midX-3, 2, 7, l.H-4, LCobble)
	for i := 0; i < 30; i++ {
		x, y := g.Between(midX-4, midX+4), g.Between(2, l.H-3)
		if g.Chance(0.5) {
			l.set(x, y, LGrass)
		} else {
			l.set(x, y, LCobble)
		}
	}

	l.Entry = core.Point{X: midX, Y: l.H - 3}
	l.Entities = append(l.Entities, &Entity{
		Kind: EExit, Pos: l.Entry, Name: "the road back", Line: "Leave. Slowly.",
	})

	// The thing at the far end, which is what the place is about. It is a
	// staircase down into the ground with a roof over it and no building
	// attached, and the game never explains it because nobody in the game can.
	l.Entities = append(l.Entities, &Entity{
		Kind: ESign, Pos: core.Point{X: midX, Y: 3},
		Name: "a stairway going down", Sprite: "odd/metro",
		Line: "Steps, under a roof, going down into the ground. They are swept. " +
			"Somebody sweeps them.",
	})

	// A machine that takes money and gives you something cold. It is an
	// apothecary as far as the shop code is concerned, which is the correct
	// amount of explanation.
	if p, ok := openNear(g, l, core.Point{X: midX, Y: l.H / 2}, 1, 6); ok {
		l.Entities = append(l.Entities, &Entity{
			Kind: EShop, Pos: p, Name: "a lit humming box", Shop: ShopApothecary,
			Sprite: core.Pick(g, oddVending), Line: wr.Oddity(g, "machine"),
		})
	}

	// Signage nobody can act on, in a script nobody writes.
	for i := 0; i < g.Between(2, 4); i++ {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: ESign, Pos: p, Name: "a lit sign",
				Sprite: core.Pick(g, oddSigns), Line: wr.Oddity(g, "sign"),
			})
		}
	}
	for i := 0; i < g.Between(1, 3); i++ {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: ESign, Pos: p, Name: "paint on a wall",
				Sprite: core.Pick(g, oddDaubs), Line: wr.Oddity(g, "sign"),
			})
		}
	}

	// Bins. Not chests — bins. Somebody has usually been through them.
	for i := 0; i < g.Between(1, 3); i++ {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: EChest, Pos: p, Name: "a metal drum",
				Sprite: core.Pick(g, oddBins), Line: wr.Oddity(g, "trash"),
			})
		}
	}

	// Furniture with no opinion, purely to stand about being wrong.
	for i := 0; i < g.Between(1, 3); i++ {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: ESign, Pos: p, Name: "something left here",
				Sprite: core.Pick(g, oddClutter), Line: wr.Oddity(g, "sign"),
			})
		}
	}

	// People, who live here, and for whom none of the above is remarkable.
	for i := 0; i < g.Between(2, 4); i++ {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: ENPC, Pos: p, Name: wr.PersonName(g),
				Line: wr.Oddity(g, "person"), Sprite: core.Pick(g, folkSprites),
			})
		}
	}

	// And something with teeth, because it is still a place on the map with a
	// level band attached to it.
	for i := 0; i < g.Between(1, 3); i++ {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: EFoe, Pos: p, Name: "a lurking shape",
				Sprite: core.Pick(g, foeSprites), Wander: true,
			})
		}
	}
	return l
}

// buildDungeon carves rooms and links them with elbow corridors, then seeds
// foes, chests, and a boss in the room furthest from the entrance.
func buildDungeon(g *core.RNG, poi *POI, wr Namer) *LocalMap {
	l := newLocal(poi, 52, 40, LVoid)
	l.Biome = "dungeon"
	l.Indoors = true

	type room struct{ x, y, w, h int }
	var rooms []room
	for tries := 0; tries < 500 && len(rooms) < 11; tries++ {
		r := room{
			x: g.Between(1, l.W-10), y: g.Between(1, l.H-9),
			w: g.Between(5, 9), h: g.Between(4, 7),
		}
		bad := false
		for _, o := range rooms {
			if r.x < o.x+o.w+2 && r.x+r.w+2 > o.x && r.y < o.y+o.h+2 && r.y+r.h+2 > o.y {
				bad = true
				break
			}
		}
		if bad {
			continue
		}
		rooms = append(rooms, r)
		l.rect(r.x, r.y, r.w, r.h, LFloor)
	}
	if len(rooms) == 0 { // degenerate seed: give the player a box, not a crash
		l.rect(2, 2, 12, 10, LFloor)
		rooms = append(rooms, room{2, 2, 12, 10})
	}

	center := func(r room) core.Point { return core.Point{X: r.x + r.w/2, Y: r.y + r.h/2} }
	for i := 1; i < len(rooms); i++ {
		a, b := center(rooms[i-1]), center(rooms[i])
		x, y := a.X, a.Y
		for x != b.X {
			if x < b.X {
				x++
			} else {
				x--
			}
			l.set(x, y, LFloor)
		}
		for y != b.Y {
			if y < b.Y {
				y++
			} else {
				y--
			}
			l.set(x, y, LFloor)
		}
	}

	// Wall off everything touching open floor so the cavern has edges.
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			if l.At(x, y) != LVoid {
				continue
			}
			for _, d := range []core.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {X: 1, Y: 1}, {X: -1, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}} {
				if l.At(x+d.X, y+d.Y) == LFloor {
					l.set(x, y, LWall)
					break
				}
			}
		}
	}

	entry := center(rooms[0])
	l.Entry = entry
	l.Entities = append(l.Entities, &Entity{
		Kind: EExit, Pos: entry, Name: "the way out",
		Line: "Daylight. Probably.",
	})

	// The boss sits in whichever room is furthest from the entrance.
	far, farD := rooms[0], -1
	for _, r := range rooms[1:] {
		if d := center(r).Manhattan(entry); d > farD {
			far, farD = r, d
		}
	}
	if !poi.Cleared {
		l.Entities = append(l.Entities, &Entity{
			Kind: EBoss, Pos: center(far), Name: "something large",
			Line:   "It has been waiting. It is not happy about the wait.",
			Sprite: "foe/necromancer/walk",
		})
	}

	// Wandering foes and chests in the other rooms.
	for i, r := range rooms {
		if i == 0 {
			continue
		}
		n := g.Between(1, 2)
		for k := 0; k < n; k++ {
			p := core.Point{X: g.Between(r.x, r.x+r.w-1), Y: g.Between(r.y, r.y+r.h-1)}
			if l.EntityAt(p.X, p.Y) != nil {
				continue
			}
			l.Entities = append(l.Entities, &Entity{
				Kind: EFoe, Pos: p, Name: "a lurking shape",
				Sprite: core.Pick(g, foeSprites), Wander: g.Chance(0.6),
			})
		}
		if g.Chance(0.45) {
			p := core.Point{X: g.Between(r.x, r.x+r.w-1), Y: g.Between(r.y, r.y+r.h-1)}
			if l.EntityAt(p.X, p.Y) == nil {
				l.Entities = append(l.Entities, &Entity{
					Kind: EChest, Pos: p, Name: "a chest",
					Line: wr.SignText(g),
				})
			}
		}
	}
	return l
}

// buildSite handles the small one-scene locations: ruins, towers, shrines,
// camps, and whatever an "oddity" turns out to be this time.
func buildSite(g *core.RNG, poi *POI, wr Namer) *LocalMap {
	l := newLocal(poi, 30, 24, LVoid)
	l.Biome = poiBiome(poi.Kind)

	// A rough blob of ground rather than a rectangle, so ruins look ruined.
	cx, cy := l.W/2, l.H/2
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			dx, dy := float64(x-cx)/float64(l.W/2), float64(y-cy)/float64(l.H/2)
			if dx*dx+dy*dy < 0.85+g.Float()*0.25 {
				l.set(x, y, LFloor)
			}
		}
	}
	if poi.Kind == KindShrine || poi.Kind == KindTower {
		l.rect(cx-4, cy-3, 9, 7, LCobble)
	}
	// Broken walls scattered around for ruins and camps.
	if poi.Kind == KindRuin || poi.Kind == KindCamp {
		for i := 0; i < 14; i++ {
			x, y := g.Between(2, l.W-3), g.Between(2, l.H-3)
			if l.At(x, y) == LFloor {
				l.set(x, y, LWall)
			}
		}
	}

	l.Entry = core.Point{X: cx, Y: l.H - 3}
	for l.At(l.Entry.X, l.Entry.Y) != LFloor && l.Entry.Y > 2 {
		l.Entry.Y--
	}
	l.Entities = append(l.Entities, &Entity{
		Kind: EExit, Pos: l.Entry, Name: "the road back", Line: "Leave.",
	})

	switch poi.Kind {
	case KindShrine:
		l.Entities = append(l.Entities, &Entity{
			Kind: EAltar, Pos: core.Point{X: cx, Y: cy},
			Name: "a cracked altar", Line: wr.SignText(g),
		})
	case KindCamp:
		for i := 0; i < 3; i++ {
			if p, ok := findOpen(g, l, 120); ok {
				l.Entities = append(l.Entities, &Entity{
					Kind: ENPC, Pos: p, Name: wr.PersonName(g),
					Line: wr.NPCLine(g), Sprite: core.Pick(g, folkSprites),
				})
			}
		}
	default:
		n := g.Between(2, 4)
		for i := 0; i < n; i++ {
			if p, ok := findOpen(g, l, 120); ok {
				l.Entities = append(l.Entities, &Entity{
					Kind: EFoe, Pos: p, Name: "a lurking shape",
					Sprite: core.Pick(g, foeSprites), Wander: true,
				})
			}
		}
	}
	if g.Chance(0.7) {
		if p, ok := findOpen(g, l, 120); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: EChest, Pos: p, Name: "a strongbox",
				Line: wr.SignText(g),
			})
		}
	}
	return l
}

func poiBiome(k POIKind) string {
	switch k {
	case KindRuin:
		return "wasteland"
	case KindTower, KindShrine:
		return "dungeon"
	case KindOddity:
		// Its own roster, because a place where everything is the wrong century
		// should not be defended by wolves. Mostly constructs, which stop steel
		// and nothing else, with a couple that are the other way round — which
		// makes the joke zone the one place in the game where the matchup axis
		// is the whole encounter rather than an occasional surprise.
		return "oddity"
	default:
		return "forest"
	}
}

// elbowRoom reports that nothing else is standing on or beside a cell.
//
// Character art is four tiles tall on a one-tile grid, so two people on
// neighbouring squares are drawn almost entirely on top of each other and read
// as one shape somebody has got stuck inside. Placement only avoided the exact
// same tile, and a sixth of everybody in a town came out touching somebody else.
func elbowRoom(l *LocalMap, p core.Point) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if l.EntityAt(p.X+dx, p.Y+dy) != nil {
				return false
			}
		}
	}
	return true
}

// findOpen looks for a walkable cell with nobody on or next to it.
//
// Two passes: the first insists on the elbow room, the second takes any free
// tile. A cramped interior with more people than corners still gets everybody
// placed — it just stops being the first answer rather than the only one.
func findOpen(g *core.RNG, l *LocalMap, tries int) (core.Point, bool) {
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < tries; i++ {
			p := core.Point{X: g.Between(1, l.W-2), Y: g.Between(1, l.H-2)}
			if !l.At(p.X, p.Y).Info().Passable || l.EntityAt(p.X, p.Y) != nil {
				continue
			}
			if pass == 0 && !elbowRoom(l, p) {
				continue
			}
			return p, true
		}
	}
	return core.Point{}, false
}

// openNear finds a free walkable tile within radius of at, searching outward
// so the result hugs the anchor. Used to stand someone beside a door rather
// than at a random address in the same town.
// The minimum is what keeps somebody standing *beside* the anchor rather than
// inside them: at four tiles tall, a person one square from the innkeeper is
// drawn almost entirely over the innkeeper, and the hireling loitering outside
// the inn was doing exactly that in a third of all towns.
func openNear(g *core.RNG, l *LocalMap, at core.Point, min, radius int) (core.Point, bool) {
	if min < 1 {
		min = 1
	}
	for r := min; r <= radius; r++ {
		var ring []core.Point
		var roomy []core.Point
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if core.Abs(dx) != r && core.Abs(dy) != r {
					continue // interior of the ring was covered by a smaller r
				}
				p := core.Point{X: at.X + dx, Y: at.Y + dy}
				if p.X < 1 || p.Y < 1 || p.X >= l.W-1 || p.Y >= l.H-1 {
					continue
				}
				if l.At(p.X, p.Y).Info().Passable && l.EntityAt(p.X, p.Y) == nil {
					ring = append(ring, p)
					if elbowRoom(l, p) {
						roomy = append(roomy, p)
					}
				}
			}
		}
		// Prefer a spot nobody is already standing beside, but do not walk
		// further from the anchor to get one: being beside the door is the
		// point of this function.
		if len(roomy) > 0 {
			return core.Pick(g, roomy), true
		}
		if len(ring) > 0 {
			return core.Pick(g, ring), true
		}
	}
	return core.Point{}, false
}

// StepFoes moves wandering foes one tile at random. Called on a slow tick so
// they drift rather than twitch.
func (l *LocalMap) StepFoes(g *core.RNG) {
	for _, e := range l.Entities {
		if e.Kind != EFoe || !e.Wander || e.Used {
			continue
		}
		if !g.Chance(0.35) {
			continue
		}
		d := core.Dir(g.Intn(4))
		n := e.Pos.Add(d.Delta())
		if l.At(n.X, n.Y).Info().Passable && l.EntityAt(n.X, n.Y) == nil {
			e.Pos = n
			e.facing = d
		}
	}
}

// Facing reports which way an entity is looking, for sprite selection.
func (e *Entity) Facing() core.Dir { return e.facing }
