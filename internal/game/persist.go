package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Snapshot captures the run as a save file. The continent itself is not
// captured — it regenerates from the seed — so this is only the parts the
// player has changed.
func (g *Game) Snapshot() *save.File {
	f := &save.File{
		Seed:       g.Seed,
		Player:     g.Player,
		Clock:      g.Clock,
		Sagas:      g.Sagas,
		Track:      save.TrackState(g.Track),
		LastSpell:  g.LastSpell,
		Allies:     g.Allies,
		At:         g.Walk.Tile,
		Facing:     int(g.Walk.Dir()),
		Fog:        save.PackFog(g.World.Explored),
		SinceFight: g.sinceFight,
		NextAmbush: g.nextAmbush,
		Summary:    g.summary(),
		Quests:     g.Quests.Quests,
		Threads:    g.Threads.Threads,
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
		// The location to record is the one the world knows about. A shop room
		// carries a POI of its own that is not in the world's list, so asking
		// the room would answer -1 and file the party as standing outside the
		// town — see the note below on where they are actually put back.
		here := g.Local.POI
		if g.inShop && g.townPOI != nil {
			here = g.townPOI
		}
		if idx := g.poiIndex(here); idx >= 0 {
			at := g.LocalWalk.Tile
			if g.inShop {
				// Standing in a shop is recorded as standing at its door.
				//
				// A save is a place you come back to rather than a photograph,
				// and the door is where walking out puts you — so this is the
				// same answer by a shorter route, and it costs the format
				// nothing. The alternative is a field naming which room of
				// which building, which every save written before today would
				// answer with a zero that means the first shop in the town.
				at = g.shopReturn
			}
			f.Inside = &save.Inside{
				POI:    idx,
				At:     at,
				Facing: int(g.LocalWalk.Dir()),
				Floor:  g.floor,
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
	company := ""
	if n := len(g.Allies); n > 0 {
		company = fmt.Sprintf(" +%d", n)
	}
	return fmt.Sprintf("%s %s - %s L%d%s - %s",
		g.Player.Name, g.Player.Epithet, g.Player.Class, g.Player.Level, company, where)
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
	g.Allies = f.Allies
	// A save written before the party existed carries no marching order and no
	// list; both are already zero, so nothing needs converting.
	g.World = world.Generate(f.Seed, g.Write)
	g.sinceFight = f.SinceFight
	g.nextAmbush = f.NextAmbush
	g.Clock = f.Clock
	g.Sagas, g.pendingLegs = f.Sagas, nil
	g.Track = Track(f.Track)
	g.LastSpell = f.LastSpell
	g.Quests = quest.Log{Quests: f.Quests}
	g.Threads = thread.Log{Threads: f.Threads}
	g.pendingBeats, g.remindEndings = nil, false

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

	g.Walk = core.NewWalker(9)
	g.Walk.Place(f.At)
	g.Walk.Face(core.Dir(f.Facing))
	g.follow, g.localFollow = nil, nil
	g.reformLines()

	// Rebuild the scene stack from the bottom so backing out of an interior
	// lands on the overworld, exactly as it would have during play.
	g.stack = nil
	g.Local = nil
	g.Push(newOverworldScene(g))

	if f.Inside != nil && f.Inside.POI >= 0 && f.Inside.POI < len(g.World.POIs) {
		poi := g.World.POIs[f.Inside.POI]
		g.floor = f.Inside.Floor
		g.inShop, g.townPOI = false, poi
		g.Local = world.BuildLocal(poi, g.Write, g.floor)
		g.LocalWalk = core.NewWalker(7)
		g.LocalWalk.Place(f.Inside.At)
		g.LocalWalk.Face(core.Dir(f.Inside.Facing))
		g.localFollow.Place(f.Inside.At)
		g.Push(newLocalScene(g))
	}

	g.Quests.SyncFetch(g.Player.Bag)
	// A save from before threads existed, or one whose continent had nothing to
	// stage a particular story in, gets its companions caught up here.
	g.ensureThreads()

	g.Log.Clear()
	g.Log.Add("Loaded: %s", f.Summary)
	// And a save from before there were long stories gets one, for exactly the
	// reason its companions get backstories a line earlier. After the log is
	// cleared, or the line it writes about where to go would be wiped by it.
	g.ensureSaga()
	return nil
}

// AutosaveSlot is where the game puts the run just before a fight starts.
//
// Dying is the one thing in the game that cannot be undone by playing on, and
// an encounter is rolled at you rather than chosen — so the run ending on a
// walk you did not know was dangerous is a bad way to lose an hour. This is the
// out: on death the player is offered the state from immediately before the
// fight, which is the same run minus one encounter that had not happened yet.
//
// It is deliberately a normal save in a normal slot. It shows up in the load
// menu like any other, it can be loaded on purpose, and there is no separate
// format to keep working.
const AutosaveSlot = "autosave"

// autosave records the run as it stands, for the death prompt to offer back.
//
// Written when the player is *safe* — a bed, an altar, the first morning — and
// not before every fight, which is where it used to go. Checkpointing each
// encounter meant a death cost one fight, which is barely a cost: the run
// carried on from a step the player had already taken. Checkpointing at rest
// means a death costs everything since the last time you stopped, which turns
// "should I pay for a bed" into a real question and gives the inn a job beyond
// hit points.
//
// Failures are swallowed on purpose. A full disk should not stop somebody
// sleeping; it should cost the safety net and nothing else.
func (g *Game) autosave() {
	if g.InDemo() || g.Player == nil || g.World == nil {
		return
	}
	_ = g.SaveTo(AutosaveSlot)
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

// heroID names the character this run belongs to, closely enough to tell two
// runs apart, and it is the same pair save.File uses for the same purpose:
// name and epithet, because the generator hands out the same first name again
// eventually and the pair almost never repeats.
//
// One function rather than the expression written out wherever it is needed.
// It was written out in two places — the death prompt and the slot screen —
// and a rule for "is this the same person" that exists twice is a rule that
// will one day disagree with itself about whose save it is looking at.
func (g *Game) heroID() string {
	if g.Player == nil {
		return ""
	}
	return g.Player.Name + " " + g.Player.Epithet
}
