package save_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The fixtures under saves/fixtures are hand-curated runs, each parked in a
// state that is otherwise a chore to reach: a full company, somebody on the
// floor, a party standing inside a location, and a file written before the
// party existed at all.
//
// They are a regression net that needs no display. A save is the world seed
// plus what the player changed, so loading one asserts that the seed still
// generates the same continent, that the content it names still exists, and
// that the format has not quietly stopped being readable — the three ways a
// save silently rots. They double as playtest starting points via -load.

// writerFor returns the real content writer for regenerating a continent.
//
// A stub namer will not do, and this is the whole reason the fixtures earn
// their keep: location names are drawn from the same generator that places the
// locations, so a namer consuming no randomness produces a *different*
// continent. The first fixture generated that way was refused by the game with
// "the save predates a change to world generation" — from a stub, not a change.
// Anything checking a save against its world has to regenerate the world the
// game would.
func writerFor(t *testing.T) *content.Writer {
	t.Helper()
	return content.New(&tablesFor(t).Text)
}

func tablesFor(t *testing.T) *gamedata.Tables {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no repository root: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	return tables
}

func fixtures(t *testing.T) map[string]*save.File {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no repository root to read fixtures from: %v", err)
	}
	dir := filepath.Join(root, "saves", "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	out := map[string]*save.File{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		f, err := save.Read(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Errorf("%s no longer loads: %v", e.Name(), err)
			continue
		}
		out[strings.TrimSuffix(e.Name(), ".json")] = f
	}
	if len(out) == 0 {
		t.Fatal("no fixtures found; the regression net is empty")
	}
	return out
}

// The continent is not stored — it is regenerated from the seed — so a change
// to world generation silently invalidates every save anybody has. The loader
// refuses a file whose location count no longer matches, and this is what
// notices before a player does.
func TestFixturesStillMatchTheWorldTheyWereSavedIn(t *testing.T) {
	for name, f := range fixtures(t) {
		m := world.Generate(f.Seed, writerFor(t))
		if len(f.POIs) != len(m.POIs) {
			t.Errorf("%s: saved %d locations, seed %d now generates %d — "+
				"world generation changed under the fixtures",
				name, len(f.POIs), f.Seed, len(m.POIs))
		}
		if !m.Walkable(f.At.X, f.At.Y) {
			t.Errorf("%s: the player is standing at %v, which is no longer walkable", name, f.At)
		}
		if f.Inside != nil && (f.Inside.POI < 0 || f.Inside.POI >= len(m.POIs)) {
			t.Errorf("%s: saved inside location %d, of %d", name, f.Inside.POI, len(m.POIs))
		}
		// The fog is a packed bitset sized to the map; a resize would silently
		// shift every explored tile.
		if got := len(save.UnpackFog(f.Fog, len(m.Explored))); got != len(m.Explored) {
			t.Errorf("%s: fog unpacked to %d tiles, want %d", name, got, len(m.Explored))
		}
	}
}

// A save names content by string. Renaming an item or a monster orphans every
// file that mentions it, and nothing else in the suite would notice.
func TestFixturesOnlyNameContentThatExists(t *testing.T) {
	tables := tablesFor(t)
	for name, f := range fixtures(t) {
		for _, c := range append([]*model.Character{f.Player}, f.Allies...) {
			for _, it := range c.Bag {
				if _, ok := tables.Item(it.Name); !ok {
					t.Errorf("%s: %s is carrying %q, which no longer exists", name, c.Name, it.Name)
				}
			}
		}
		for _, q := range f.Quests {
			if q == nil {
				t.Errorf("%s: a nil quest survived the round trip", name)
				continue
			}
			if q.Item != "" {
				if _, ok := tables.Item(q.Item); !ok {
					t.Errorf("%s: a quest asks for %q, which no longer exists", name, q.Item)
				}
			}
			if q.MonsterID != "" {
				if _, ok := tables.ByID[q.MonsterID]; !ok {
					t.Errorf("%s: a quest names monster %q, which no longer exists", name, q.MonsterID)
				}
			}
			// Every quest points at somewhere; an index past the end of the
			// location list is how a quest ends up naming nothing at all.
			if q.GiverPOI < 0 || q.TargetPOI < 0 {
				t.Errorf("%s: a quest points at location %d / %d", name, q.GiverPOI, q.TargetPOI)
			}
		}
	}
}

// The invariants a run must satisfy however it got into that state. These are
// the things that would let a corrupt or hand-edited save through the loader
// and produce nonsense several screens later.
func TestFixturesHoldTheRunInvariants(t *testing.T) {
	for name, f := range fixtures(t) {
		p := f.Player
		if p == nil {
			t.Errorf("%s has no character", name)
			continue
		}
		if p.Level < 1 || p.MaxHP < 1 || p.HP > p.MaxHP || p.Psyche > p.MaxPsyche {
			t.Errorf("%s: hero is L%d at %d/%d hit points and %d/%d psyche",
				name, p.Level, p.HP, p.MaxHP, p.Psyche, p.MaxPsyche)
		}
		if p.Ally {
			t.Errorf("%s: the hero is marked as a hireling", name)
		}

		if len(f.Allies)+1 > party.MaxSize {
			t.Errorf("%s: %d in the company, over the cap of %d", name, len(f.Allies)+1, party.MaxSize)
		}
		for _, c := range f.Allies {
			switch {
			case !c.Ally:
				t.Errorf("%s: %s is in the company but not marked a hireling", name, c.Name)
			case c.Coins != 0 || len(c.Bag) != 0:
				// The purse and the pack are the hero's. A companion holding
				// either means money that vanishes the moment they are let go.
				t.Errorf("%s: %s carries %d coins and %d items", name, c.Name, c.Coins, len(c.Bag))
			case c.HP > c.MaxHP || c.Psyche > c.MaxPsyche:
				t.Errorf("%s: %s is at %d/%d hit points", name, c.Name, c.HP, c.MaxHP)
			case c.Blood != "":
				if _, ok := model.LineageOf(c.Blood); !ok {
					t.Errorf("%s: %s is part %q, which is not a lineage", name, c.Name, c.Blood)
				}
			}
			// Conditions are fight-scoped and must never reach a file.
			if len(c.Active) != 0 {
				t.Errorf("%s: %s carries %d conditions out of a save", name, c.Name, len(c.Active))
			}
		}
		if len(p.Active) != 0 {
			t.Errorf("%s: the hero carries %d conditions out of a save", name, len(p.Active))
		}
	}
}

// The fixtures are only a net if they actually cover the awkward states. This
// asserts the set has not been quietly reduced to five copies of "level one,
// standing in a field".
func TestFixturesCoverTheStatesWorthCovering(t *testing.T) {
	all := fixtures(t)

	var haveOld, haveFull, haveFallen, haveInside, haveSolo, haveLineage bool
	for _, f := range all {
		if f.Version < save.Version {
			haveOld = true
		}
		if len(f.Allies)+1 == party.MaxSize {
			haveFull = true
		}
		if len(f.Allies) == 0 {
			haveSolo = true
		}
		if f.Inside != nil {
			haveInside = true
		}
		for _, c := range f.Allies {
			if !c.Alive() {
				haveFallen = true
			}
			if c.Blood != "" {
				haveLineage = true
			}
		}
	}

	for _, c := range []struct {
		got  bool
		want string
	}{
		{haveOld, "a save in an older format"},
		{haveSolo, "a run with nobody hired"},
		{haveFull, "a company at the party cap"},
		{haveFallen, "a companion on the floor"},
		{haveLineage, "a part-monster hireling"},
		{haveInside, "a party standing inside a location"},
	} {
		if !c.got {
			t.Errorf("no fixture covers %s", c.want)
		}
	}
}
