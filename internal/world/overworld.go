package world

import (
	"fmt"
	"math"

	"github.com/slycrel/slycrel-rpg/internal/core"
)

// Overworld dimensions in tiles. At 16px per tile this is a 2560x1920 pixel
// continent — about 21 screens wide, which takes a couple of real minutes to
// cross on foot and leaves room for roughly forty points of interest.
const (
	Width  = 160
	Height = 120
)

// POIKind is the class of a point of interest.
type POIKind string

// The places worth walking to.
const (
	KindCapital POIKind = "capital"
	KindTown    POIKind = "town"
	KindVillage POIKind = "village"
	KindCastle  POIKind = "castle"
	KindDungeon POIKind = "dungeon"
	KindCave    POIKind = "cave"
	KindRuin    POIKind = "ruin"
	KindTower   POIKind = "tower"
	KindShrine  POIKind = "shrine"
	KindCamp    POIKind = "camp"
	KindOddity  POIKind = "oddity"
)

// Settlement reports whether the kind has shops and a bed.
func (k POIKind) Settlement() bool {
	switch k {
	case KindCapital, KindTown, KindVillage, KindCastle:
		return true
	}
	return false
}

// UsedKey identifies one spent interactable inside a location.
//
// Keyed by kind and position rather than by index into the entity list,
// because that list is not stable: clearing a dungeon drops its boss from
// generation and shifts every index after it. A position never moves.
type UsedKey struct {
	Kind string `json:"kind"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	// Floor is which level of a many-levelled place this was on.
	//
	// Omitted when it is the ground floor, which is what every save written
	// before interiors had floors is saying — and it is saying it correctly,
	// since those places had exactly one. A chest at (10,10) on the second
	// floor of a tower is not the chest at (10,10) on the first, and without
	// this opening one would have emptied both.
	Floor int `json:"floor,omitempty"`
}

// POI is one visitable location on the overworld.
type POI struct {
	Pos   core.Point
	Kind  POIKind
	Name  string
	Tag   string // one-line description shown on approach
	Level int    // difficulty band
	Seed  int64  // regenerates the interior deterministically

	Discovered bool
	Visited    bool
	Cleared    bool

	// Used survives leaving and re-entering. Interiors are regenerated from
	// Seed rather than stored, so without this an emptied chest would refill
	// itself the moment you stepped outside.
	Used []UsedKey
}

// MarkUsed records that an interactable has been spent.
func (p *POI) MarkUsed(kind string, at core.Point, floor int) {
	k := UsedKey{Kind: kind, X: at.X, Y: at.Y, Floor: floor}
	for _, u := range p.Used {
		if u == k {
			return
		}
	}
	p.Used = append(p.Used, k)
}

// IsUsed reports whether an interactable has already been spent.
func (p *POI) IsUsed(kind string, at core.Point, floor int) bool {
	for _, u := range p.Used {
		if u.Kind == kind && u.X == at.X && u.Y == at.Y && u.Floor == floor {
			return true
		}
	}
	return false
}

// Map is a generated continent.
type Map struct {
	Seed     int64
	Tiles    []Terrain
	POIs     []*POI
	Start    core.Point // where the player begins, always inside the capital
	Explored []bool     // per-tile fog, for the parchment map
}

// At returns the terrain at a tile coordinate, treating out-of-bounds as ocean
// so callers never need to bounds-check before peeking at a neighbour.
func (m *Map) At(x, y int) Terrain {
	if x < 0 || y < 0 || x >= Width || y >= Height {
		return Ocean
	}
	return m.Tiles[y*Width+x]
}

func (m *Map) set(x, y int, t Terrain) {
	if x < 0 || y < 0 || x >= Width || y >= Height {
		return
	}
	m.Tiles[y*Width+x] = t
}

// Walkable reports whether the player can stand at x,y.
func (m *Map) Walkable(x, y int) bool { return m.At(x, y).Passable() }

// POIAt returns the point of interest standing on a tile, if any.
func (m *Map) POIAt(x, y int) *POI {
	for _, p := range m.POIs {
		if p.Pos.X == x && p.Pos.Y == y {
			return p
		}
	}
	return nil
}

// regionRadius is how far around a point counts as "here" when working out how
// dangerous the ground is.
const regionRadius = 18

// HomeRadius is how far the ground around the capital stays a place you can
// learn the game in. Inside it the overworld caps an encounter at the player's
// own level, which does not make it safe — it makes it predictable.
//
// Here rather than in the scene layer because it describes the map, and because
// the balance report needs it to say whether a long story's first leg sends a
// fresh character outside the one region that was tuned for them.
const HomeRadius = 14

// RegionLevel is how dangerous the country around a point is, read off the
// locations near it. One means nothing worse than the outskirts of the capital.
//
// It lives here rather than in the scene layer because two things need it and
// they must not disagree: the overworld, which blends it with the player's own
// level to pick an encounter, and the balance report, which uses it to check
// whether a long story's legs actually get harder as they get further out.
// A copy of this loop in cmd/balance would be a report measuring its own
// arithmetic rather than the game's, which is the one thing that report is not
// allowed to do.
func (m *Map) RegionLevel(at core.Point) int {
	level := 1
	for _, p := range m.POIs {
		if p.Pos.Manhattan(at) < regionRadius && p.Level > level {
			level = p.Level
		}
	}
	return level
}

// Reveal marks the tiles within radius of a point as explored.
func (m *Map) Reveal(at core.Point, radius int) {
	for y := at.Y - radius; y <= at.Y+radius; y++ {
		for x := at.X - radius; x <= at.X+radius; x++ {
			if x < 0 || y < 0 || x >= Width || y >= Height {
				continue
			}
			if (x-at.X)*(x-at.X)+(y-at.Y)*(y-at.Y) > radius*radius {
				continue
			}
			m.Explored[y*Width+x] = true
			if p := m.POIAt(x, y); p != nil {
				p.Discovered = true
			}
		}
	}
}

// IsExplored reports fog state for the parchment map.
func (m *Map) IsExplored(x, y int) bool {
	if x < 0 || y < 0 || x >= Width || y >= Height {
		return false
	}
	return m.Explored[y*Width+x]
}

// Namer supplies every piece of generated prose the map builders need. The
// content package implements it; world takes it as an interface so map
// generation stays independent of the writing, and so a test can generate a
// continent with stub text.
type Namer interface {
	PlaceName(g *core.RNG, kind string) string
	PlaceTag(g *core.RNG, kind string) string
	PersonName(g *core.RNG) string
	NPCLine(g *core.RNG) string
	SignText(g *core.RNG) string
	RecruitPitch(g *core.RNG, blood string) string
	// Oddity is the joke zone's own voice, keyed by what is speaking.
	Oddity(g *core.RNG, what string) string
}

// Generate builds a continent from a seed.
func Generate(seed int64, namer Namer) *Map {
	g := core.NewRNG(seed)
	m := &Map{
		Seed:     seed,
		Tiles:    make([]Terrain, Width*Height),
		Explored: make([]bool, Width*Height),
	}

	elev := noiseField(g.Fork("elevation", seed), 6, 0.52)
	moist := noiseField(g.Fork("moisture", seed), 4, 0.55)
	temp := noiseField(g.Fork("temperature", seed), 3, 0.60)

	for y := 0; y < Height; y++ {
		for x := 0; x < Width; x++ {
			i := y*Width + x
			// Radial falloff turns the noise into an island: the coast is
			// always the map edge, so the player can never walk off the world.
			e := elev[i] * falloff(x, y)
			// Bias temperature by latitude so the north is cold and the
			// south bakes, which gives the biomes a legible geography.
			lat := float64(y) / float64(Height)
			t := core.ClampF(temp[i]*0.55+lat*0.55, 0, 1)
			m.Tiles[i] = classify(e, moist[i], t)
		}
	}

	carveRivers(m, g.Fork("rivers", seed), elev)
	placePOIs(m, g.Fork("pois", seed), namer)
	layRoads(m)

	// Start in the capital, which placePOIs guarantees exists.
	for _, p := range m.POIs {
		if p.Kind == KindCapital {
			m.Start = p.Pos
			p.Discovered = true
			break
		}
	}
	m.Reveal(m.Start, 8)
	return m
}

// falloff returns a 0..1 multiplier that pushes elevation down near the edges,
// squared off slightly so the continent is blobby rather than perfectly round.
func falloff(x, y int) float64 {
	nx := float64(x)/float64(Width)*2 - 1
	ny := float64(y)/float64(Height)*2 - 1
	d := math.Max(math.Abs(nx), math.Abs(ny))*0.55 + math.Hypot(nx, ny)*0.45
	return core.ClampF(1.08-d*d*1.15, 0, 1)
}

// noiseField builds fractal value noise in [0,1] over the map, summing
// octaves of a bilinearly interpolated random lattice.
func noiseField(g *core.RNG, octaves int, persistence float64) []float64 {
	out := make([]float64, Width*Height)
	amp, total := 1.0, 0.0
	cells := 4

	for o := 0; o < octaves; o++ {
		lat := makeLattice(g, cells+1, cells+1)
		for y := 0; y < Height; y++ {
			for x := 0; x < Width; x++ {
				fx := float64(x) / float64(Width) * float64(cells)
				fy := float64(y) / float64(Height) * float64(cells)
				out[y*Width+x] += amp * sampleLattice(lat, cells+1, fx, fy)
			}
		}
		total += amp
		amp *= persistence
		cells *= 2
	}
	for i := range out {
		out[i] = core.ClampF(out[i]/total, 0, 1)
	}
	return out
}

func makeLattice(g *core.RNG, w, h int) []float64 {
	l := make([]float64, w*h)
	for i := range l {
		l[i] = g.Float()
	}
	return l
}

// sampleLattice does bilinear interpolation with a smoothstep easing, which is
// what keeps the coastline from looking like graph paper.
func sampleLattice(l []float64, stride int, fx, fy float64) float64 {
	x0, y0 := int(fx), int(fy)
	tx, ty := smooth(fx-float64(x0)), smooth(fy-float64(y0))
	at := func(x, y int) float64 {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= stride {
			x = stride - 1
		}
		if i := y*stride + x; i >= 0 && i < len(l) {
			return l[i]
		}
		return 0
	}
	a := at(x0, y0)*(1-tx) + at(x0+1, y0)*tx
	b := at(x0, y0+1)*(1-tx) + at(x0+1, y0+1)*tx
	return a*(1-ty) + b*ty
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

// carveRivers runs water downhill from high ground to the sea. Rivers are
// impassable, so they double as natural level gating: the bridge is wherever
// the road happens to cross.
func carveRivers(m *Map, g *core.RNG, elev []float64) {
	attempts := 0
	for made := 0; made < 7 && attempts < 400; attempts++ {
		x, y := g.Intn(Width), g.Intn(Height)
		if t := m.At(x, y); t != Mountain && t != Peak {
			continue
		}
		path := make([]core.Point, 0, 128)
		for step := 0; step < 300; step++ {
			cur := m.At(x, y)
			if cur == Ocean || cur == Shallows || cur == River {
				break
			}
			path = append(path, core.Point{X: x, Y: y})
			// Walk to the lowest neighbour, with a nudge so rivers meander.
			bestX, bestY, best := x, y, elev[y*Width+x]
			for _, d := range []core.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
				nx, ny := x+d.X, y+d.Y
				if nx < 0 || ny < 0 || nx >= Width || ny >= Height {
					continue
				}
				e := elev[ny*Width+nx] + (g.Float()-0.5)*0.03
				if e < best {
					bestX, bestY, best = nx, ny, e
				}
			}
			if bestX == x && bestY == y {
				break // landlocked pit; abandon this river
			}
			x, y = bestX, bestY
		}
		if len(path) < 12 {
			continue
		}
		for _, p := range path {
			if m.At(p.X, p.Y) == Peak {
				continue
			}
			m.set(p.X, p.Y, River)
		}
		made++
	}
}

// poiPlan is the target census for one continent.
var poiPlan = []struct {
	kind  POIKind
	count int
}{
	{KindCapital, 1},
	{KindTown, 4},
	{KindVillage, 7},
	{KindCastle, 2},
	{KindDungeon, 5},
	{KindCave, 6},
	{KindRuin, 5},
	{KindTower, 3},
	{KindShrine, 4},
	{KindCamp, 4},
	{KindOddity, 4},
}

// placePOIs scatters locations on suitable terrain, keeping them apart so the
// map does not clump, and assigns each a level band by distance from the
// capital — the classic "danger radiates outward" layout.
func placePOIs(m *Map, g *core.RNG, namer Namer) {
	taken := map[core.Point]bool{}
	minGap := 6

	suitable := func(k POIKind, t Terrain) bool {
		switch k {
		case KindCapital, KindTown, KindVillage:
			return t == Plains || t == Meadow || t == Beach || t == Forest
		case KindCastle, KindTower:
			return t == Hills || t == Plains || t == Meadow || t == Mountain
		case KindDungeon, KindCave:
			return t == Mountain || t == Hills || t == Deepwood || t == Wasteland
		case KindRuin:
			return t == Desert || t == Wasteland || t == Swamp || t == Deepwood || t == Plains
		case KindShrine:
			return t.Passable() && t != Beach
		case KindCamp:
			return t == Forest || t == Plains || t == Hills || t == Deepwood
		default:
			return t.Passable()
		}
	}

	farEnough := func(p core.Point) bool {
		for q := range taken {
			if p.Manhattan(q) < minGap {
				return false
			}
		}
		return true
	}

	// The capital goes first and takes the most central habitable spot, so
	// that "walk outward" always means "walk into trouble".
	var capital core.Point
	{
		best, bestScore := core.Point{X: Width / 2, Y: Height / 2}, math.MaxFloat64
		for tries := 0; tries < 6000; tries++ {
			x, y := g.Intn(Width), g.Intn(Height)
			if !suitable(KindCapital, m.At(x, y)) {
				continue
			}
			d := math.Hypot(float64(x-Width/2), float64(y-Height/2))
			if d < bestScore {
				best, bestScore = core.Point{X: x, Y: y}, d
			}
		}
		capital = best
	}

	add := func(kind POIKind, at core.Point) {
		p := &POI{
			Pos:  at,
			Kind: kind,
			Name: namer.PlaceName(g, string(kind)),
			Tag:  namer.PlaceTag(g, string(kind)),
			Seed: int64(g.Intn(1<<30)) ^ (int64(at.X)<<20 | int64(at.Y)),
		}
		m.POIs = append(m.POIs, p)
		taken[at] = true
	}

	add(KindCapital, capital)

	for _, plan := range poiPlan {
		if plan.kind == KindCapital {
			continue
		}
		placed := 0
		for tries := 0; tries < 20000 && placed < plan.count; tries++ {
			at := core.Point{X: g.Intn(Width), Y: g.Intn(Height)}
			if !suitable(plan.kind, m.At(at.X, at.Y)) || taken[at] || !farEnough(at) {
				continue
			}
			add(plan.kind, at)
			placed++
		}
	}

	gradeByDistance(m, capital)
}

// gradeByDistance is "danger radiates outward" turned into a level per location,
// and it runs after placement rather than during it because it needs to know how
// far the outward goes.
//
// It used to divide by half the diagonal of the map rectangle, and that is the
// bug this function exists to have fixed. The continent is a blob inside the
// rectangle, so the land never gets near the corners: measured over forty
// seeds, the farthest walkable tile from the capital is 49 to 72 tiles out
// against a denominator of 100. The level formula was therefore spending 59%
// of its range on the whole world and throwing the rest off the edge of the
// map. The highest location generated in forty maps — 1,778 of them — was
// level 9. Levels 10 through 14 did not exist anywhere, ever, and the content
// tables, the gear bands and the entire top half of the balance report were
// describing a place the player cannot be sent to.
//
// Worth being exact about why "make the world bigger" was the wrong question,
// since that is how this sat on the open list for months: the formula is a
// *fraction* of the denominator, so it is scale-invariant. Doubling Width and
// Height would have moved nothing at all. The variable was never the size, it
// was what the size is measured against.
//
// The denominator is now the map's own reach — the farthest location actually
// placed — so the ramp spans whatever continent the noise produced, and a small
// map gets a steep ramp rather than no endgame.
//
// And it spans the whole of it: thirteen steps, so the outermost location is
// level fourteen. Twelve was the first attempt, on the reasoning that the top
// two bands are what a dungeon's +2 is for — which reproduced the original bug
// two notches up, because a dungeon has to happen to be the farthest thing on
// the map to reach fourteen, and across ten seeds none was. A level-14
// character wants somewhere that is level 14 to be in; the overworld blends
// two parts the player's level with one part the region's, so nothing above a
// level-12 region can hand a level-14 player a level-14 fight at all. The
// dungeon bonus still matters everywhere it has room, which is everywhere but
// the rim.
func gradeByDistance(m *Map, capital core.Point) {
	reach := 0.0
	for _, p := range m.POIs {
		if d := math.Hypot(float64(p.Pos.X-capital.X), float64(p.Pos.Y-capital.Y)); d > reach {
			reach = d
		}
	}
	if reach <= 0 {
		reach = 1
	}
	for _, p := range m.POIs {
		d := math.Hypot(float64(p.Pos.X-capital.X), float64(p.Pos.Y-capital.Y))
		level := 1 + int(d/reach*13)
		switch p.Kind {
		case KindCapital:
			level = 1
		case KindDungeon:
			level += 2 // dungeons punch above their neighbourhood
		case KindVillage, KindCamp:
			level = core.Max(1, level-1)
		}
		p.Level = core.Clamp(level, 1, 14)
	}
}

// layRoads connects every settlement to the capital with a walkable road,
// bulldozing rivers into fords along the way. Roads are the safest terrain,
// so this also defines the low-risk travel network.
func layRoads(m *Map) {
	var capital *POI
	var settlements []*POI
	for _, p := range m.POIs {
		if p.Kind == KindCapital {
			capital = p
			continue
		}
		if p.Kind.Settlement() {
			settlements = append(settlements, p)
		}
	}
	if capital == nil {
		return
	}
	for _, s := range settlements {
		// An L-shaped path is crude but reads correctly on a tile map and
		// never fails to connect, which matters more than elegance here.
		x, y := s.Pos.X, s.Pos.Y
		for x != capital.Pos.X {
			if x < capital.Pos.X {
				x++
			} else {
				x--
			}
			pave(m, x, y)
		}
		for y != capital.Pos.Y {
			if y < capital.Pos.Y {
				y++
			} else {
				y--
			}
			pave(m, x, y)
		}
	}
}

func pave(m *Map, x, y int) {
	switch m.At(x, y) {
	case Ocean, Shallows, Peak:
		return // roads do not cross the sea or summit a mountain
	}
	// Do not pave over a location's own tile.
	if m.POIAt(x, y) != nil {
		return
	}
	m.set(x, y, Road)
}

// Describe returns the status line shown while standing on a tile.
func (m *Map) Describe(at core.Point) string {
	t := m.At(at.X, at.Y)
	if p := m.POIAt(at.X, at.Y); p != nil {
		return fmt.Sprintf("%s  (%s)", p.Name, p.Kind)
	}
	return t.Name()
}

// RollEncounter reports whether stepping onto a tile starts a fight, and which
// monster table to use.
func (m *Map) RollEncounter(g *core.RNG, at core.Point, level int, prowl float64) (string, bool) {
	t := m.At(at.X, at.Y)
	if !dangerRoll(g, t, level, prowl) {
		return "", false
	}
	return t.Biome(), true
}
