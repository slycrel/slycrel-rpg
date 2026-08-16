package game

import (
	"github.com/slycrel/slycrel-rpg/internal/tiles"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Ground materials. Priority decides what fringes what when two meet: water
// sits under sand, sand under soil, soil under grass, and rock and snow on top
// of everything, which is the order these things layer in the actual world.
//
// Textures come from the seasonal Mana Seed legend strips, so a single palette
// covers every biome and the whole map reads as one place rather than four
// tilesets bolted together.
var groundMaterials = []tiles.Material{
	{Name: "deep", Texture: "ground/summer_deep", Fallback: "tile/ocean", Priority: 0},
	{Name: "shallow", Texture: "ground/summer_shallow", Fallback: "tile/shallows", Priority: 1},
	{Name: "sand", Texture: "ground/sand", Fallback: "tile/beach", Priority: 2},
	{Name: "ash", Texture: "ground/winter_dirt", Fallback: "tile/wasteland", Priority: 3},
	{Name: "dirt", Texture: "ground/summer_dirt", Fallback: "tile/hills", Priority: 4},
	{Name: "grass", Texture: "ground/summer_grass", Fallback: "tile/plains", Priority: 5},
	{Name: "swamp", Texture: "ground/autumn_darkgrass", Fallback: "tile/swamp", Priority: 6},
	{Name: "wood", Texture: "ground/summer_darkgrass", Fallback: "tile/forest", Priority: 7},
	{Name: "stone", Texture: "ground/summer_stone", Fallback: "tile/mountain", Priority: 8},
	{Name: "snow", Texture: "ground/winter_grass", Fallback: "tile/peak", Priority: 9},

	// A road is laid on top of the land, so it outranks everything it crosses.
	// Sharing dirt's priority instead let grass close over it from both sides
	// and a one-tile track all but vanished.
	{Name: "road", Texture: "ground/summer_dirt", Fallback: "tile/road", Priority: 10},
}

// terrainMaterial maps overworld terrain to a ground material. Several
// terrains deliberately share one: plains and meadow differ in encounter rate
// and flavour, not in what the ground looks like underfoot.
var terrainMaterial = map[world.Terrain]string{
	world.Ocean:     "deep",
	world.Shallows:  "shallow",
	world.River:     "shallow",
	world.Beach:     "sand",
	world.Desert:    "sand",
	world.Plains:    "grass",
	world.Meadow:    "grass",
	world.Road:      "road",
	world.Hills:     "dirt",
	world.Forest:    "wood",
	world.Deepwood:  "wood",
	world.Swamp:     "swamp",
	world.Wasteland: "ash",
	world.Mountain:  "stone",
	world.Peak:      "snow",
}

// localMaterial maps an interior tile to a ground material. Walls and roofs
// deliberately map to nothing: they are structures standing on the map, not
// terrain, and blending ground into them would soften edges that should read
// as solid.
var localMaterial = map[world.LocalTile]string{
	world.LFloor:  "dirt",
	world.LGrass:  "grass",
	world.LCobble: "stone",
	world.LWater:  "shallow",
	world.LDoor:   "road",
	world.LStair:  "stone",
}

// localMaterialAt reports the ground material for an interior cell.
func (g *Game) localMaterialAt(x, y int) string {
	if g.Local == nil {
		return ""
	}
	return localMaterial[g.Local.At(x, y)]
}

// materialAt reports the ground material for an overworld cell. Out of bounds
// reads as open sea, which is what world.At already returns.
func (g *Game) materialAt(x, y int) string {
	return terrainMaterial[g.World.At(x, y)]
}

// ground returns the terrain renderer, composing it on first use. It is built
// lazily because compositing draws to images, and the first Draw is the
// earliest point the graphics driver is guaranteed to be running.
func (g *Game) ground() *tiles.Renderer {
	if g.tiles == nil {
		g.tiles = tiles.New(g.Assets, groundMaterials)
	}
	return g.tiles
}
