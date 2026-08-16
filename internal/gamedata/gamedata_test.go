package gamedata_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

func load(t *testing.T) *gamedata.Tables {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Fatalf("finding repo root: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	return tables
}

// TestLootReferencesExist catches the failure mode that is otherwise silent:
// a monster dropping an item name that no longer appears in items.json, which
// the loot roller quietly skips rather than reporting.
func TestLootReferencesExist(t *testing.T) {
	tables := load(t)
	for biome, defs := range tables.Monsters {
		for _, d := range defs {
			for _, drop := range d.Loot {
				if _, ok := tables.Item(drop.Item); !ok {
					t.Errorf("%s/%s drops %q, which is not in items.json", biome, d.ID, drop.Item)
				}
			}
		}
	}
}

func TestMonsterDefsAreSane(t *testing.T) {
	tables := load(t)
	ids := map[string]string{}
	for biome, defs := range tables.Monsters {
		for _, d := range defs {
			if prev, dup := ids[d.ID]; dup {
				t.Errorf("duplicate monster id %q in %s and %s", d.ID, prev, biome)
			}
			ids[d.ID] = biome

			if d.HP <= 0 || d.Offense <= 0 || d.Level <= 0 {
				t.Errorf("%s: nonsense stats hp=%d off=%d lvl=%d", d.ID, d.HP, d.Offense, d.Level)
			}
			if d.XP <= 0 {
				t.Errorf("%s: awards no experience", d.ID)
			}
			if len(d.AttackVerb) == 0 || len(d.AttackWith) == 0 {
				t.Errorf("%s: missing attack flavor, combat log will read badly", d.ID)
			}
			if d.Sprite == "" {
				t.Errorf("%s: no sprite key", d.ID)
			}
		}
	}
}

// TestEveryBiomeCanSpawn guards the encounter path: every terrain the player
// can walk on must resolve to a monster table that yields something.
func TestEveryBiomeCanSpawn(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(7)
	for _, biome := range []string{
		"plains", "forest", "hills", "mountain", "swamp",
		"desert", "wasteland", "coast", "dungeon",
	} {
		mons := tables.PickMonsters(g, biome, 5, 2)
		if len(mons) != 2 {
			t.Errorf("biome %q produced %d monsters, want 2", biome, len(mons))
		}
		for _, m := range mons {
			if m.HP <= 0 {
				t.Errorf("biome %q spawned %s with %d hp", biome, m.Name, m.HP)
			}
		}
	}
}

func TestSpellsCoverEveryClass(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(3)
	for _, class := range model.AllClasses {
		c := rules.NewCharacter(g, "Test", class)
		c.Level = 1
		if got := tables.SpellsFor(c); len(got) == 0 {
			t.Errorf("%s knows no techniques at level 1", class)
		}
		c.Level = 10
		if got := tables.SpellsFor(c); len(got) < 4 {
			t.Errorf("%s only knows %d techniques at level 10", class, len(got))
		}
	}
}

// stubNamer lets world generation run without the content package.
type stubNamer struct{}

func (stubNamer) PlaceName(*core.RNG, string) string { return "Placename" }
func (stubNamer) PlaceTag(*core.RNG, string) string  { return "tag" }
func (stubNamer) PersonName(*core.RNG) string        { return "Person" }
func (stubNamer) NPCLine(*core.RNG) string           { return "line" }
func (stubNamer) SignText(*core.RNG) string          { return "sign" }

// TestWorldGenerationIsHabitable runs a spread of seeds because the failure
// this guards against — an island so small or so waterlogged that the capital
// has nowhere to go — only shows up on unlucky noise.
func TestWorldGenerationIsHabitable(t *testing.T) {
	for _, seed := range []int64{1, 2, 42, 1994, 20260815, 999999} {
		m := world.Generate(seed, stubNamer{})

		if !m.Walkable(m.Start.X, m.Start.Y) {
			t.Errorf("seed %d: start tile is not walkable", seed)
		}
		if p := m.POIAt(m.Start.X, m.Start.Y); p == nil || p.Kind != world.KindCapital {
			t.Errorf("seed %d: start tile is not the capital", seed)
		}

		land := 0
		for y := 0; y < world.Height; y++ {
			for x := 0; x < world.Width; x++ {
				if m.Walkable(x, y) {
					land++
				}
			}
		}
		frac := float64(land) / float64(world.Width*world.Height)
		if frac < 0.15 || frac > 0.85 {
			t.Errorf("seed %d: %.0f%% of the map is walkable, want 15-85%%", seed, frac*100)
		}

		if len(m.POIs) < 25 {
			t.Errorf("seed %d: only %d locations placed", seed, len(m.POIs))
		}
		settlements := 0
		for _, p := range m.POIs {
			if p.Kind.Settlement() {
				settlements++
			}
			if !m.At(p.Pos.X, p.Pos.Y).Passable() {
				t.Errorf("seed %d: %s sits on impassable %s", seed, p.Name, m.At(p.Pos.X, p.Pos.Y).Name())
			}
		}
		if settlements < 8 {
			t.Errorf("seed %d: only %d settlements", seed, settlements)
		}
	}
}

// TestLocalMapsAreEnterable checks that every kind of location generates an
// interior the player can actually stand in and leave again.
func TestLocalMapsAreEnterable(t *testing.T) {
	m := world.Generate(1994, stubNamer{})
	seen := map[world.POIKind]bool{}
	for _, p := range m.POIs {
		l := world.BuildLocal(p, stubNamer{})
		if !l.At(l.Entry.X, l.Entry.Y).Info().Passable {
			t.Errorf("%s (%s): entry tile is solid", p.Name, p.Kind)
		}
		exits := 0
		for _, e := range l.Entities {
			if e.Kind == world.EExit {
				exits++
			}
		}
		if exits == 0 {
			t.Errorf("%s (%s): no way out", p.Name, p.Kind)
		}
		seen[p.Kind] = true
	}
	if len(seen) < 8 {
		t.Errorf("only exercised %d location kinds", len(seen))
	}
}

// TestSpentInteractablesSurviveLeaving pins the bug that persistence had to fix
// first: interiors are regenerated from their seed on every visit, so without
// the location remembering what has been used, an emptied chest refills itself
// the moment the player steps outside.
func TestSpentInteractablesSurviveLeaving(t *testing.T) {
	m := world.Generate(1994, stubNamer{})

	var poi *world.POI
	for _, p := range m.POIs {
		if p.Kind == world.KindDungeon {
			poi = p
			break
		}
	}
	if poi == nil {
		t.Skip("seed produced no dungeon")
	}

	first := world.BuildLocal(poi, stubNamer{})
	var spent []*world.Entity
	for _, e := range first.Entities {
		if e.Kind == world.EChest || e.Kind == world.EFoe {
			e.Used = true
			poi.MarkUsed(string(e.Kind), e.Pos)
			spent = append(spent, e)
		}
	}
	if len(spent) == 0 {
		t.Skip("dungeon had nothing to spend")
	}

	// Walk out and back in.
	again := world.BuildLocal(poi, stubNamer{})
	for _, e := range spent {
		found := false
		for _, r := range again.Entities {
			if r.Kind == e.Kind && r.Pos == e.Pos {
				found = true
				if !r.Used {
					t.Errorf("%s at %v came back unused after re-entering", r.Kind, r.Pos)
				}
			}
		}
		if !found {
			t.Errorf("%s at %v vanished entirely on re-entry", e.Kind, e.Pos)
		}
	}
}

// TestInteriorsAreDeterministic is the assumption persistence rests on: the
// same location must regenerate identically, or saved positions would point at
// the wrong things.
func TestInteriorsAreDeterministic(t *testing.T) {
	m := world.Generate(2026, stubNamer{})
	for _, p := range m.POIs[:core.Min(12, len(m.POIs))] {
		a := world.BuildLocal(p, stubNamer{})
		b := world.BuildLocal(p, stubNamer{})
		if a.W != b.W || a.H != b.H || a.Entry != b.Entry {
			t.Errorf("%s (%s) regenerated at a different size or entry", p.Name, p.Kind)
			continue
		}
		if len(a.Entities) != len(b.Entities) {
			t.Errorf("%s regenerated with %d entities then %d", p.Name, len(a.Entities), len(b.Entities))
			continue
		}
		for i := range a.Entities {
			if a.Entities[i].Kind != b.Entities[i].Kind || a.Entities[i].Pos != b.Entities[i].Pos {
				t.Errorf("%s entity %d moved between generations", p.Name, i)
				break
			}
		}
		for i := range a.Tiles {
			if a.Tiles[i] != b.Tiles[i] {
				t.Errorf("%s tiles differ between generations", p.Name)
				break
			}
		}
	}
}

// manifestKeys reads the committed asset manifest. The file is checked in even
// though the art it points at is not, which is what lets content reference
// icons by key and be verified without anyone holding the purchased bundle.
func manifestKeys(t *testing.T) map[string]bool {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Fatalf("finding repo root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "assets", "manifest.json"))
	if err != nil {
		t.Skipf("no asset manifest: %v", err)
	}
	var m struct {
		Entries []struct {
			Key string `json:"key"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing manifest: %v", err)
	}
	keys := map[string]bool{}
	for _, e := range m.Entries {
		keys[e.Key] = true
	}
	return keys
}

// TestIconsResolve is the check that catches a whole class of silent failure:
// an icon key that no manifest entry provides just leaves a blank space in a
// menu, which is easy to miss and impossible to attribute later. It already
// caught the loot pack shipping its whetstone icon as "whetstonel_x.png".
func TestIconsResolve(t *testing.T) {
	keys := manifestKeys(t)
	tables := load(t)

	check := func(what, name, icon string) {
		if icon == "" {
			t.Errorf("%s %q has no icon", what, name)
			return
		}
		if !keys[icon] {
			t.Errorf("%s %q wants icon %q, which the manifest does not provide", what, name, icon)
		}
	}
	for _, it := range tables.Items {
		check("item", it.Name, it.Icon)
	}
	for _, w := range tables.Weapons {
		check("weapon", w.Name, w.Icon)
	}
	for _, a := range tables.Armors {
		check("armor", a.Name, a.Icon)
	}
	for _, s := range tables.Spells {
		check("spell", s.Name, s.Icon)
	}
}

// TestItemIconsAreDistinct keeps the pack readable: two items sharing an icon
// makes the bag ambiguous at a glance, which is the only reason to have icons.
func TestItemIconsAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, it := range load(t).Items {
		if prev, dup := seen[it.Icon]; dup {
			t.Errorf("%q and %q share icon %q", prev, it.Name, it.Icon)
		}
		seen[it.Icon] = it.Name
	}
}
