package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Scenery. Blended ground stops the map reading as a grid, but it still reads
// as a painted surface until something stands up out of it. Decor is what turns
// "dark green terrain" into "a wood".
//
// Placement is a pure hash of tile position and world seed rather than stored
// data: the overworld is 19,200 cells, and a seed already has to reproduce the
// continent exactly, so scattering is derived the same way everything else is.

// decorSet is one scattering rule: pick a frame from a sheet, this often.
type decorSet struct {
	Sheet  string
	Frames []int
	Chance float64 // per tile, in [0,1]
}

// Frame indices are row-major within each sheet. summer16 is a 6x5 grid:
// row 0 stumps and mushrooms, row 1 small plants and sticks, row 2 water
// plants, row 3 rocks and boulders, row 4 ferns.
var terrainDecor = map[world.Terrain][]decorSet{
	// Woods read as scattered canopy over open ground; deep woods close in.
	// The gap between the two densities is what makes them distinguishable at
	// a glance, which matters because they carry different encounter rates.
	world.Forest: {
		{Sheet: "prop/summer32", Frames: []int{0, 0, 1}, Chance: 0.12},
		{Sheet: "prop/summer16", Frames: []int{24, 25, 6, 7}, Chance: 0.13},
	},
	world.Deepwood: {
		// Weighted towards leafy canopy. Stumps and dead branches are loud
		// orange shapes: right for a scorched waste, wrong for a living wood
		// where they read as fire damage.
		{Sheet: "prop/summer32", Frames: []int{0, 0, 0, 1, 1, 4}, Chance: 0.30},
		{Sheet: "prop/summer16", Frames: []int{24, 25, 3, 0}, Chance: 0.18},
	},
	world.Plains: {
		{Sheet: "prop/summer16", Frames: []int{4, 6, 7, 8}, Chance: 0.05},
	},
	world.Meadow: {
		{Sheet: "prop/summer16", Frames: []int{3, 4, 6, 7}, Chance: 0.09},
		{Sheet: "prop/summer1632", Frames: []int{1, 2, 3}, Chance: 0.05},
	},
	world.Hills: {
		{Sheet: "prop/summer16", Frames: []int{18, 19, 20}, Chance: 0.11},
	},
	world.Mountain: {
		{Sheet: "prop/summer32", Frames: []int{2}, Chance: 0.14},
		{Sheet: "prop/summer16", Frames: []int{18, 19, 20, 21, 22, 23}, Chance: 0.22},
	},
	world.Peak: {
		{Sheet: "prop/summer16", Frames: []int{18, 22}, Chance: 0.09},
	},
	world.Swamp: {
		{Sheet: "prop/summer16", Frames: []int{12, 13, 14, 16, 17}, Chance: 0.26},
		{Sheet: "prop/summer32", Frames: []int{6}, Chance: 0.07},
	},
	world.Wasteland: {
		{Sheet: "prop/summer16", Frames: []int{0, 5, 9, 10}, Chance: 0.15},
		{Sheet: "prop/summer32", Frames: []int{4, 5}, Chance: 0.07},
	},
	world.Desert: {
		{Sheet: "prop/desert16", Frames: []int{1, 2, 4}, Chance: 0.09},
	},
	world.Beach: {
		{Sheet: "prop/summer16", Frames: []int{18}, Chance: 0.04},
	},
	// Water gets lilies and reeds, which is most of what sells a stretch of
	// shallows as shallow rather than merely a lighter blue.
	world.Shallows: {
		{Sheet: "prop/summer16", Frames: []int{12, 13, 14, 15}, Chance: 0.09},
	},
	world.River: {
		{Sheet: "prop/summer16", Frames: []int{14, 15}, Chance: 0.07},
	},
}

// decorHash mixes a tile position and the world seed into a well-distributed
// 32-bit value. A plain (x*a + y*b) would band along diagonals at these
// densities; the avalanche steps are what keep scattering from looking combed.
func decorHash(x, y int, seed int64, salt uint32) uint32 {
	h := uint32(x)*0x27d4eb2d ^ uint32(y)*0x165667b1 ^ uint32(seed)*0x9e3779b1 ^ salt
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return h
}

// unitHash returns a deterministic value in [0,1) for a tile.
func unitHash(x, y int, seed int64, salt uint32) float64 {
	return float64(decorHash(x, y, seed, salt)) / 4294967296.0
}

// personHash returns a deterministic value in [0,1) for somebody, by name.
//
// **Keyed on who they are rather than on where they are standing**, which is
// the difference between a fact about a person and a fact about a square.
// Three things about a townsperson were decided by their position — whether
// they hold an errand, whether they have a story, and what they look like —
// and all three were stable only because nobody in a town ever moved. The
// moment one of them takes a step they become a different person with a
// different face, and the star over their head moves to whoever wandered onto
// the tile they left.
//
// The name is the identity that survives walking, and it is already what the
// portrait pools key on for exactly this reason.
func personHash(name string, seed int64, salt uint32) float64 {
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return unitHash(int(h&0xFFFF), int(h>>16), seed, salt)
}

// drawDecor scatters scenery over the visible overworld. Called after the
// ground pass so props are never overpainted by a later tile's terrain.
func (g *Game) drawDecor(dst *ebiten.Image, cam render.Camera, x0, y0, x1, y1 int) {
	ox, oy := cam.Offset()
	const ts = assetsys.TileSize

	// Props are anchored at the tile's bottom edge and stand upwards, so tiles
	// below the view can still reach into it. Widen the band accordingly.
	for ty := y0; ty <= y1+3; ty++ {
		for tx := x0 - 1; tx <= x1+1; tx++ {
			t := g.World.At(tx, ty)
			sets := terrainDecor[t]
			if len(sets) == 0 {
				continue
			}
			// A location's own tile stays clear so its marker reads cleanly.
			if g.World.POIAt(tx, ty) != nil {
				continue
			}
			for i, ds := range sets {
				salt := uint32(i)*0x9e37 + 0x51ed
				if unitHash(tx, ty, g.Seed, salt) >= ds.Chance {
					continue
				}
				sp := g.Assets.Get(ds.Sheet)
				if sp.Count() == 0 {
					continue
				}
				pick := ds.Frames[int(decorHash(tx, ty, g.Seed, salt+1))%len(ds.Frames)]
				img := sp.Frame(pick)
				if img == nil {
					continue
				}
				// Bottom-centre on the tile, nudged by a stable jitter so a run
				// of props does not line up on the tile grid.
				jx := unitHash(tx, ty, g.Seed, salt+2)*6 - 3
				jy := unitHash(tx, ty, g.Seed, salt+3)*4 - 2
				w, h := float64(sp.W), float64(sp.H)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(
					float64(tx*ts)+ts/2-w/2+jx+ox,
					float64(ty*ts)+ts-h+jy+oy,
				)
				dst.DrawImage(img, op)
			}
		}
	}
}

// --- interiors -----------------------------------------------------------

// Interior scatter. The same idea as the overworld, with two differences: the
// sets are chosen by tile and by what kind of place this is, since a stone
// floor means "cottage" in a village and "cave" under a mountain; and anything
// standing on a cell is skipped, so clutter never hides a chest or a merchant.
var (
	// Household clutter: pots, sacks, bread, books, bottles. Row 0 of the
	// cozy furnishings sheet, skipping the blank first cell.
	houseProps = decorSet{
		Sheet:  "prop/cozy16",
		Frames: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		Chance: 0.30,
	}
	// Cave floors: moss, loose rock, stalagmites.
	caveProps = decorSet{
		Sheet:  "prop/cave16",
		Frames: []int{0, 1, 2, 3, 4},
		Chance: 0.14,
	}
	// Whatever grows in a town's open ground.
	yardProps = decorSet{
		Sheet:  "prop/summer16",
		Frames: []int{4, 6, 7, 8, 24, 25},
		Chance: 0.07,
	}
	// A swept street stays mostly swept.
	streetProps = decorSet{
		Sheet:  "prop/summer16",
		Frames: []int{18, 19},
		Chance: 0.02,
	}
	// What is left on the floor of a room that came furnished: bottles, and a
	// sixth as many of them.
	//
	// Not nothing. A taproom with a swept floor reads as a showroom, and the
	// scatter is what has always said somebody lives here. But a room whose
	// tables were placed on purpose is already saying that, and the two
	// together buried the tables in bread.
	sweptProps = decorSet{
		Sheet:  "prop/cozy16",
		Frames: []int{1, 3, 4, 7},
		Chance: 0.05,
	}
)

// localDecorFor returns the scatter rules for an interior cell.
func localDecorFor(t world.LocalTile, settlement, furnished bool) []decorSet {
	switch t {
	case world.LFloor:
		// The same tile is a cottage floor in a village and a cave floor under
		// a mountain; only the location knows which.
		switch {
		case furnished:
			return []decorSet{sweptProps}
		case settlement:
			return []decorSet{houseProps}
		}
		return []decorSet{caveProps}
	case world.LGrass:
		return []decorSet{yardProps}
	case world.LCobble:
		return []decorSet{streetProps}
	}
	return nil
}

// drawLocalDecor scatters clutter through the current interior.
func (g *Game) drawLocalDecor(dst *ebiten.Image, cam render.Camera, x0, y0, x1, y1 int) {
	if g.Local == nil {
		return
	}
	l := g.Local
	settlement := l.POI.Kind.Settlement()
	ox, oy := cam.Offset()
	const ts = assetsys.TileSize

	// An interior's seed keeps its clutter stable across visits, and distinct
	// from every other building generated from the same tile layout.
	seed := l.POI.Seed

	for ty := y0; ty <= y1+2; ty++ {
		for tx := x0 - 1; tx <= x1+1; tx++ {
			sets := localDecorFor(l.At(tx, ty), settlement, l.Furnished)
			if len(sets) == 0 {
				continue
			}
			// Never clutter a cell something stands on, or the way in.
			if l.EntityAt(tx, ty) != nil || (tx == l.Entry.X && ty == l.Entry.Y) {
				continue
			}
			for i, ds := range sets {
				salt := uint32(i)*0x9e37 + 0x2f1d
				if unitHash(tx, ty, seed, salt) >= ds.Chance {
					continue
				}
				sp := g.Assets.Get(ds.Sheet)
				if sp.Count() == 0 {
					continue
				}
				pick := ds.Frames[int(decorHash(tx, ty, seed, salt+1))%len(ds.Frames)]
				img := sp.Frame(pick)
				if img == nil {
					continue
				}
				jx := unitHash(tx, ty, seed, salt+2)*4 - 2
				w, h := float64(sp.W), float64(sp.H)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(
					float64(tx*ts)+ts/2-w/2+jx+ox,
					float64(ty*ts)+ts-h+oy,
				)
				dst.DrawImage(img, op)
			}
		}
	}
}
