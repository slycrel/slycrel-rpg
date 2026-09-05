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
	"foe/golem/walk",
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
	ENPC   EntityKind = "npc"
	EShop  EntityKind = "shop"
	EInn   EntityKind = "inn"
	EChest EntityKind = "chest"
	// EHoard is the chest at the end of a place, as opposed to the ones on the
	// way. Its own kind rather than a flag on EChest so that whatever decides
	// what is inside can be told which of the two it is opening.
	EHoard EntityKind = "hoard"
	ESign  EntityKind = "sign"
	EFoe   EntityKind = "foe" // a visible wandering monster
	// EDeeper leads one floor further into a place and EShallower leads one
	// back. Named for the direction of travel rather than for up or down,
	// because a tower's "further in" is upstairs and a cave's is not, and the
	// code that moves the player should not have to know which it is standing
	// in. What the *player* is told is up or down, and that comes off the POI
	// kind at the moment the words are written.
	EDeeper    EntityKind = "deeper"
	EShallower EntityKind = "shallower"
	EExit      EntityKind = "exit"    // leave back to the overworld
	EBoss      EntityKind = "boss"    // the thing the dungeon is about
	EAltar     EntityKind = "altar"   // shrines: a blessing with strings attached
	ERecruit   EntityKind = "recruit" // someone outside the inn, available for money
	// EDecor is scenery. It stands there, it is solid, and walking into it does
	// nothing — there is no case for it in `interact` and it is excluded from
	// the bump that would call one. It exists so a dungeon can have a fire in
	// it without the fire being a thing to press at.
	EDecor EntityKind = "decor"
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
	// Omen is what walking into this will turn out to be, for the things in an
	// interior that are an encounter rather than furniture.
	//
	// The sprite underneath stays decoration and is picked at random from a
	// pool of five, which is what it always was — a playthrough called those
	// icons random because they are, and the encounter is rolled fresh on
	// contact so they could never have been anything else. What that report
	// actually wanted was to know what it was walking into, and that is this
	// field rather than the picture.
	Omen   Omen
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
	// Floor is which level of the place this is, counting from the way in, and
	// Depth is how many there are. Both are one and nought for everywhere that
	// has only ever had a single storey.
	Floor, Depth int
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
func BuildLocal(poi *POI, w Namer, floor int) *LocalMap {
	depth := Depth(poi)
	floor = core.Clamp(floor, 0, depth-1)
	// Each floor is its own stream, derived from the location's seed so it is
	// the same floor every time it is walked into, and *not* forked — Fork
	// ignores its receiver, so forking on the floor number alone would build
	// the identical second storey under every tower on the continent.
	g := core.NewRNG(poi.Seed + int64(floor)*floorStride)

	var l *LocalMap
	switch poi.Kind {
	case KindCapital, KindTown, KindVillage, KindCastle:
		l = buildSettlement(g, poi, w)
	case KindDungeon, KindCave:
		l = buildDungeon(g, poi, w, floor, depth)
	case KindOddity:
		l = buildOddity(g, poi, w)
	case KindTower:
		l = buildTower(g, poi, w, floor, depth)
	default:
		l = buildSite(g, poi, w)
	}
	l.Floor, l.Depth = floor, depth
	// Replay what the player has already dealt with here, on this floor.
	for _, e := range l.Entities {
		if poi.IsUsed(string(e.Kind), e.Pos, floor) {
			e.Used = true
		}
	}
	return l
}

// floorStride separates one floor's stream from the next.
//
// A large odd number rather than 1, so that the second floor of a place is not
// the ground floor of whatever location happens to be seeded one along.
const floorStride = 1_000_003

// Depth is how many levels a location has.
//
// Most have one and always did. The three that go further are the ones whose
// whole idea is depth — a dungeon, a cave, a tower — and how far is read off
// the location's own level so that the deep places are the dangerous ones,
// which is already how the world is banded.
//
// Two at level one and up to four at the top, because the floor above this is
// not free: every one of them is a walk, and a five-storey tower at level two
// is four rooms of the same fight between the player and the point of it.
func Depth(poi *POI) int {
	switch poi.Kind {
	case KindCave, KindTower:
	default:
		// A dungeon deliberately stays one floor, and it is the interesting
		// exclusion. It is already the deep place — fifty-two by forty, eleven
		// rooms, a boss in the furthest one — and giving it three of those
		// would not make it deeper, it would make it the same dungeon three
		// times with the same boss at the end of the third. Depth is worth
		// having where it replaces a walk with a descent, not where it
		// multiplies one.
		//
		// It is one line if that turns out to be wrong.
		return 1
	}
	return core.Clamp(2+poi.Level/5, 2, 4)
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
	var innAt building
	for i, s := range shops {
		if i >= len(built) {
			break
		}
		b := built[i]
		door := core.Point{X: b.x + b.w/2, Y: b.y + b.h - 2}
		kind := EShop
		if s.kind == ShopInn {
			kind = EInn
			inn, innAt = door, b
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
		// Outside the building, always. See openNearWhere.
		if p, ok := openNearWhere(g, l, inn, 2, 5, func(p core.Point) bool {
			return p.X < innAt.x || p.X >= innAt.x+innAt.w ||
				p.Y < innAt.y || p.Y >= innAt.y+innAt.h
		}); ok {
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
	// Lit ones only. The group is placed under the name "a lit sign", so the
	// wasteland pack's road signs are not in it however much they look the
	// part: a matte red octagon on a post is not lit, and the one thing this
	// zone cannot afford is a description that does not match the picture.
	// They stand in the clutter instead, where nothing is claimed about them.
	oddSigns = []string{"odd/sign1", "odd/sign2", "odd/sign3", "odd/sign4", "odd/sign5"}
	oddDaubs = []string{"odd/daub1", "odd/daub2", "odd/daub3", "odd/daub4"}
	oddBins  = []string{"odd/bin1", "odd/bin2", "odd/bin3", "odd/bin4", "odd/bin5", "odd/barrel"}
	// "Something left here", which is true of all of it. A sofa sitting
	// outdoors on a paved strip is the best thing in this list, because it is
	// the one nobody can even mistake for architecture.
	oddClutter = []string{"odd/barrier1", "odd/barrier2", "odd/lanterns",
		"odd/car", "odd/car2", "odd/sofa", "odd/stopsign", "odd/sign6"}
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
				Sprite: core.Pick(g, foeSprites), Wander: true, Omen: rollOmen(g),
			})
		}
	}
	return l
}

// buildDungeon carves rooms and links them with elbow corridors, then seeds
// foes, chests, and a boss in the room furthest from the entrance.
func buildDungeon(g *core.RNG, poi *POI, wr Namer, floor, depth int) *LocalMap {
	// A floor of a many-levelled place is smaller than a place with one floor,
	// and that is the whole of keeping this honest. A cave with three storeys
	// of a dungeon's plan is three dungeons, which is not depth — it is the
	// same walk repeated with the reward moved to the end of the third one.
	// Divided so that the total is a little over one dungeon rather than three.
	w, h, want := 52, 40, 11
	if depth > 1 {
		w, h, want = 34, 26, 6
	}
	l := newLocal(poi, w, h, LVoid)
	l.Biome = "dungeon"
	l.Indoors = true

	type room struct{ x, y, w, h int }
	var rooms []room
	for tries := 0; tries < 500 && len(rooms) < want; tries++ {
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
	// The way back, which is out of the place on the ground floor and up a
	// flight on every other. Both stand on the tile you arrive at, so walking
	// in and walking straight back out is one step either way.
	l.Entities = append(l.Entities, wayBack(poi, entry, floor))

	// The far room is where the point of the floor goes: the stairs on the way
	// down, and whatever is waiting at the bottom on the last one.
	far, farD := rooms[0], -1
	for _, r := range rooms[1:] {
		if d := center(r).Manhattan(entry); d > farD {
			far, farD = r, d
		}
	}
	if floor+1 < depth {
		l.Entities = append(l.Entities, deeperStair(poi, center(far)))
	} else if !poi.Cleared {
		l.Entities = append(l.Entities, &Entity{
			Kind: EBoss, Pos: center(far), Name: "something large",
			Line: "It has been waiting. It is not happy about the wait.",
			// A golem, since the line promises something large and a
			// necromancer is a man in a robe. It is the only sprite in the
			// game with a back and a side of its own.
			Sprite: "foe/golem/walk",
		})
		// And the reason for the walk, beside it. A hoard is the one chest in
		// a location that is worth the trip rather than worth opening on the
		// way past.
		if p, ok := openNear(g, l, center(far), 1, 4); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: EHoard, Pos: p, Name: "a hoard",
				Line: "Whatever was guarding this is the thing you just walked past.",
			})
		}
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
				Omen: rollOmen(g),
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
		// Somebody lit these, once. Placed inside a room by the same rule the
		// chests use rather than by findOpen, because a brazier is solid and one
		// dropped in a one-tile corridor would wall off whatever is past it.
		if g.Chance(0.5) {
			p := core.Point{X: g.Between(r.x, r.x+r.w-1), Y: g.Between(r.y, r.y+r.h-1)}
			if l.EntityAt(p.X, p.Y) == nil {
				l.Entities = append(l.Entities, &Entity{
					Kind: EDecor, Pos: p, Name: "a brazier", Sprite: "decor/brazier",
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
					Sprite: core.Pick(g, foeSprites), Wander: true, Omen: rollOmen(g),
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
	return openNearWhere(g, l, at, min, radius, nil)
}

// openNearWhere is openNear with a veto: ok may refuse a tile that is free and
// walkable but wrong for other reasons.
//
// It exists because "beside the inn door" and "inside the inn" are the same
// search. A building's interior is walkable floor and its door sits one tile in
// from the bottom wall, so a five-tile ring around that door covers most of the
// room behind it — and the hireling loitering outside the inn was regularly
// loitering in the middle of it, which reads as a man who lives there rather
// than one waiting to be asked.
func openNearWhere(g *core.RNG, l *LocalMap, at core.Point, min, radius int,
	ok func(core.Point) bool) (core.Point, bool) {
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
				if ok != nil && !ok(p) {
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

// The stairs, and the words for them.
//
// A tower's "further in" is upstairs and a cave's is downstairs, and the code
// that moves the player should not have to know which it is standing in — so
// the entity kinds are named for the direction of travel and only the writing
// knows about up and down. It is the one place in this file where the fiction
// and the mechanism deliberately disagree.
func goesUp(poi *POI) bool { return poi.Kind == KindTower }

func deeperStair(poi *POI, at core.Point) *Entity {
	if goesUp(poi) {
		return &Entity{
			Kind: EDeeper, Pos: at, Name: "stairs up",
			Line: "They keep going. Somebody built this to be climbed.",
		}
	}
	return &Entity{
		Kind: EDeeper, Pos: at, Name: "stairs down",
		Line: "Down, and colder.",
	}
}

// wayBack is what stands on the tile you arrive at: the door out on the ground
// floor, and the stairs you came by on every other.
func wayBack(poi *POI, at core.Point, floor int) *Entity {
	if floor == 0 {
		return &Entity{
			Kind: EExit, Pos: at, Name: "the way out",
			Line: "Daylight. Probably.",
		}
	}
	if goesUp(poi) {
		return &Entity{
			Kind: EShallower, Pos: at, Name: "stairs down",
			Line: "Back the way you came.",
		}
	}
	return &Entity{
		Kind: EShallower, Pos: at, Name: "stairs up",
		Line: "Back toward the daylight.",
	}
}

// buildTower stacks a single room per floor, with a stair at each end.
//
// A tower was a one-room site for the life of the project, which is a tower in
// name and a shed in fact. The shape is deliberately not the dungeon's: a
// dungeon is a plan you get lost in and a tower is a climb, so each floor is
// one room you cross, and the interest is what is standing in it rather than
// which way to go. That also makes the last floor mean something — you have
// walked past everything to reach it.
func buildTower(g *core.RNG, poi *POI, wr Namer, floor, depth int) *LocalMap {
	// Sized to fit the screen in one go: thirty tiles across and seventeen
	// down is what 480x270 shows, so a floor of twenty-six by fifteen is a
	// room you can see the far end of from the door. That is the whole design
	// — a dungeon is a plan you get lost in and a tower is a climb, and a
	// climb where you cannot see the stairs is just a smaller dungeon.
	const w, h = 26, 15
	l := newLocal(poi, w, h, LVoid)
	l.Biome = poiBiome(poi.Kind)
	l.Indoors = true
	l.rect(1, 1, w-2, h-2, LFloor)
	for x := 0; x < w; x++ {
		l.set(x, 0, LWall)
		l.set(x, h-1, LWall)
	}
	for y := 0; y < h; y++ {
		l.set(0, y, LWall)
		l.set(w-1, y, LWall)
	}

	entry := core.Point{X: w / 2, Y: h - 2}
	top := core.Point{X: w / 2, Y: 2}
	l.Entry = entry
	l.Entities = append(l.Entities, wayBack(poi, entry, floor))

	if floor+1 < depth {
		l.Entities = append(l.Entities, deeperStair(poi, top))
	} else if !poi.Cleared {
		l.Entities = append(l.Entities, &Entity{
			Kind: EBoss, Pos: top, Name: "something large",
			Line:   "Whatever this tower is for, it is for this.",
			Sprite: "foe/golem/walk",
		})
		if p, ok := openNear(g, l, top, 1, 4); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: EHoard, Pos: p, Name: "a hoard",
				Line: "The top of a tower, and this is what was up here.",
			})
		}
	}

	// Two to four things standing in the room, and sometimes something to open.
	//
	// Never within a couple of tiles of either stair. A creature standing in
	// front of a staircase hides it — character art is sixty-four pixels tall
	// on a sixteen-pixel grid, so a foe one square below the stairs draws over
	// them completely — and the stairs are the only landmark a floor has. It
	// is a floor you can see the far end of, and this is what keeps the far end
	// worth seeing.
	clear := func(p core.Point) bool {
		return chebyshev(p, top) > 2 && chebyshev(p, entry) > 2
	}
	for i, n := 0, g.Between(2, 4); i < n; i++ {
		p, ok := openNearWhere(g, l, core.Point{X: w / 2, Y: h / 2}, 2, 7, clear)
		if !ok {
			continue
		}
		l.Entities = append(l.Entities, &Entity{
			Kind: EFoe, Pos: p, Name: "a lurking shape",
			Sprite: core.Pick(g, foeSprites), Wander: g.Chance(0.6),
			Omen: rollOmen(g),
		})
	}
	if g.Chance(0.5) {
		if p, ok := openNearWhere(g, l, core.Point{X: w / 2, Y: h / 2}, 2, 7, clear); ok {
			l.Entities = append(l.Entities, &Entity{
				Kind: EChest, Pos: p, Name: "a chest", Line: wr.SignText(g),
			})
		}
	}
	if p, ok := openNear(g, l, core.Point{X: 3, Y: 3}, 0, 3); ok {
		l.Entities = append(l.Entities, &Entity{
			Kind: EDecor, Pos: p, Name: "a brazier", Sprite: "decor/brazier",
		})
	}
	return l
}
