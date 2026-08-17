package world

import "github.com/slycrel/slycrel-rpg/internal/core"

// Terrain is one overworld cell type.
type Terrain uint8

// The overworld terrain set. Order matters only for debugging; look-ups go
// through the table below.
const (
	Ocean Terrain = iota
	Shallows
	Beach
	Plains
	Meadow
	Forest
	Deepwood
	Hills
	Mountain
	Peak
	Swamp
	Desert
	Wasteland
	Road
	River
)

// TerrainInfo describes how a terrain behaves and how it looks.
type TerrainInfo struct {
	Name string
	// Tile is the assetsys key used to draw it.
	Tile string
	// Passable is whether you can walk onto it at all.
	Passable bool
	// Cost is how many movement points entering costs; higher terrain slows
	// travel and, because encounter rolls happen per step, is proportionally
	// more dangerous to cross.
	Cost int
	// Danger is the per-step encounter chance in percent.
	Danger int
	// Biome names the monster table to draw from.
	Biome string
}

var terrainTable = [...]TerrainInfo{
	Ocean:     {"open sea", "tile/ocean", false, 0, 0, "coast"},
	Shallows:  {"shallows", "tile/shallows", false, 0, 0, "coast"},
	Beach:     {"sand", "tile/beach", true, 2, 3, "coast"},
	Plains:    {"plains", "tile/plains", true, 1, 5, "plains"},
	Meadow:    {"meadow", "tile/meadow", true, 1, 4, "plains"},
	Forest:    {"woods", "tile/forest", true, 2, 9, "forest"},
	Deepwood:  {"deep woods", "tile/deepwood", true, 3, 14, "forest"},
	Hills:     {"hills", "tile/hills", true, 2, 8, "hills"},
	Mountain:  {"mountains", "tile/mountain", true, 4, 12, "mountain"},
	Peak:      {"high peaks", "tile/peak", false, 0, 0, "mountain"},
	Swamp:     {"mire", "tile/swamp", true, 3, 16, "swamp"},
	Desert:    {"waste sands", "tile/desert", true, 3, 10, "desert"},
	Wasteland: {"scorched land", "tile/wasteland", true, 2, 18, "wasteland"},
	Road:      {"road", "tile/road", true, 1, 2, "plains"},
	River:     {"river", "tile/river", false, 0, 0, "swamp"},
}

// Info returns the descriptor for t.
func (t Terrain) Info() TerrainInfo {
	if int(t) >= len(terrainTable) {
		return terrainTable[Ocean]
	}
	return terrainTable[t]
}

// Passable reports whether the player can enter this terrain on foot.
func (t Terrain) Passable() bool { return t.Info().Passable }

// Name returns the display name.
func (t Terrain) Name() string { return t.Info().Name }

// Biome returns the monster table key.
func (t Terrain) Biome() string { return t.Info().Biome }

// classify turns an elevation/moisture/temperature sample into terrain. The
// thresholds are tuned so a default seed produces roughly one third water, a
// walkable coastal ring, and a mountainous spine you have to route around —
// which is what makes an overworld feel like a place rather than a texture.
func classify(elev, moist, temp float64) Terrain {
	switch {
	case elev < 0.30:
		return Ocean
	case elev < 0.36:
		return Shallows
	case elev < 0.40:
		return Beach
	case elev > 0.86:
		return Peak
	case elev > 0.74:
		return Mountain
	case elev > 0.63:
		if moist > 0.62 {
			return Deepwood
		}
		return Hills
	}

	// Lowlands: moisture and heat decide.
	switch {
	case temp > 0.70 && moist < 0.34:
		return Desert
	case moist > 0.74 && elev < 0.47:
		return Swamp
	case moist > 0.60:
		return Deepwood
	case moist > 0.46:
		return Forest
	case moist > 0.30:
		return Meadow
	default:
		return Plains
	}
}

// dangerRoll reports whether a step onto t triggers an encounter. Level is the
// party level: low-level characters get a short grace period so the first walk
// out of town is not immediately fatal.
//
// prowl scales it for what it is doing outside: above one after dark, below one
// when it is coming down hard enough that nothing else wants to be out either.
// Passing 1 is the old behaviour exactly.
func dangerRoll(g *core.RNG, t Terrain, level int, prowl float64) bool {
	d := t.Info().Danger
	if d <= 0 {
		return false
	}
	if level <= 2 {
		d = d * 2 / 3
	}
	// Rounded rather than truncated, or a 1-in-100 tile in the rain would drop
	// to zero and a corner of the map would stop being dangerous at all.
	d = int(float64(d)*prowl + 0.5)
	if d <= 0 {
		return false
	}
	return g.Intn(100) < d
}
