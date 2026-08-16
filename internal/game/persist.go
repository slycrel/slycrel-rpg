package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Snapshot captures the run as a save file. The continent itself is not
// captured — it regenerates from the seed — so this is only the parts the
// player has changed.
func (g *Game) Snapshot() *save.File {
	f := &save.File{
		Seed:       g.Seed,
		Player:     g.Player,
		At:         g.Walk.Tile,
		Facing:     int(g.Walk.Dir()),
		Fog:        save.PackFog(g.World.Explored),
		SinceFight: g.sinceFight,
		Summary:    g.summary(),
		Quests:     g.Quests.Quests,
	}

	f.POIs = make([]save.POIState, len(g.World.POIs))
	for i, p := range g.World.POIs {
		st := save.POIState{
			Discovered: p.Discovered,
			Visited:    p.Visited,
			Cleared:    p.Cleared,
		}
		for _, u := range p.Used {
			st.Used = append(st.Used, save.UsedEntity{Kind: u.Kind, X: u.X, Y: u.Y})
		}
		f.POIs[i] = st
	}

	if g.Local != nil {
		if idx := g.poiIndex(g.Local.POI); idx >= 0 {
			f.Inside = &save.Inside{
				POI:    idx,
				At:     g.LocalWalk.Tile,
				Facing: int(g.LocalWalk.Dir()),
			}
		}
	}
	return f
}

// summary is the one-liner shown in the load menu.
func (g *Game) summary() string {
	where := g.World.Describe(g.Walk.Tile)
	if g.Local != nil {
		where = g.Local.POI.Name
	}
	return fmt.Sprintf("%s %s - %s L%d - %s",
		g.Player.Name, g.Player.Epithet, g.Player.Class, g.Player.Level, where)
}

func (g *Game) poiIndex(target *world.POI) int {
	for i, p := range g.World.POIs {
		if p == target {
			return i
		}
	}
	return -1
}

// Restore rebuilds a run from a save and leaves the scene stack pointing at
// wherever the player was standing.
func (g *Game) Restore(f *save.File) error {
	if f.Player == nil {
		return fmt.Errorf("save contains no character")
	}

	g.Seed = f.Seed
	g.RNG = core.NewRNG(f.Seed)
	g.Player = f.Player
	g.World = world.Generate(f.Seed, g.Write)
	g.sinceFight = f.SinceFight
	g.Quests = quest.Log{Quests: f.Quests}

	// The location list is generated from the seed, so it is stable — but a
	// save written by a build with a different world generator would not line
	// up. Refuse rather than silently applying flags to the wrong places.
	if len(f.POIs) != len(g.World.POIs) {
		return fmt.Errorf("save has %d locations, this world generates %d; "+
			"the save predates a change to world generation",
			len(f.POIs), len(g.World.POIs))
	}
	for i, st := range f.POIs {
		p := g.World.POIs[i]
		p.Discovered = st.Discovered
		p.Visited = st.Visited
		p.Cleared = st.Cleared
		p.Used = nil
		for _, u := range st.Used {
			p.Used = append(p.Used, world.UsedKey{Kind: u.Kind, X: u.X, Y: u.Y})
		}
	}

	if fog := save.UnpackFog(f.Fog, len(g.World.Explored)); len(fog) == len(g.World.Explored) {
		g.World.Explored = fog
	}

	g.Walk = walker{dur: 9}
	g.Walk.Place(f.At)
	g.Walk.dir = core.Dir(f.Facing)

	// Rebuild the scene stack from the bottom so backing out of an interior
	// lands on the overworld, exactly as it would have during play.
	g.stack = nil
	g.Local = nil
	g.Push(newOverworldScene(g))

	if f.Inside != nil && f.Inside.POI >= 0 && f.Inside.POI < len(g.World.POIs) {
		poi := g.World.POIs[f.Inside.POI]
		g.Local = world.BuildLocal(poi, g.Write)
		g.LocalWalk = walker{dur: 7}
		g.LocalWalk.Place(f.Inside.At)
		g.LocalWalk.dir = core.Dir(f.Inside.Facing)
		g.Push(newLocalScene(g))
	}

	g.Quests.SyncFetch(g.Player.Bag)

	g.Log.Clear()
	g.Log.Add("Loaded: %s", f.Summary)
	return nil
}

// SaveTo writes the current run to a named slot.
func (g *Game) SaveTo(slot string) error {
	if g.Player == nil || g.World == nil {
		return fmt.Errorf("nothing to save yet")
	}
	return save.Write(g.Root, slot, g.Snapshot())
}

// LoadFrom reads a named slot and restores it.
func (g *Game) LoadFrom(slot string) error {
	f, err := save.Load(g.Root, slot)
	if err != nil {
		return err
	}
	return g.Restore(f)
}

// LoadPath reads a save from an explicit path, for the -load flag.
func (g *Game) LoadPath(path string) error {
	f, err := save.Read(path)
	if err != nil {
		return err
	}
	return g.Restore(f)
}
