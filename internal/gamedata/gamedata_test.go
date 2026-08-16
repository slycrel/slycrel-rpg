package gamedata_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func (stubNamer) PlaceName(*core.RNG, string) string    { return "Placename" }
func (stubNamer) PlaceTag(*core.RNG, string) string     { return "tag" }
func (stubNamer) PersonName(*core.RNG) string           { return "Person" }
func (stubNamer) NPCLine(*core.RNG) string              { return "line" }
func (stubNamer) SignText(*core.RNG) string             { return "sign" }
func (stubNamer) RecruitPitch(*core.RNG, string) string { return "pitch" }

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

// --- balance ---------------------------------------------------------------
//
// These assert the shape of the difficulty curve rather than exact numbers, so
// content can be added freely but cannot silently break progression. Run
// `go run ./cmd/balance` for the full picture behind them.

// balanceBiome mirrors the harness: roughly where a character of this level is
// expected to be fighting.
func balanceBiome(level int) string {
	switch {
	case level <= 2:
		return "plains"
	case level <= 4:
		return "forest"
	case level <= 6:
		return "hills"
	case level <= 8:
		return "swamp"
	case level <= 10:
		return "dungeon"
	default:
		return "mountain"
	}
}

func geared(t *gamedata.Tables, c *model.Character) {
	tier := core.Clamp(1+(c.Level-1)/3, 1, 5)
	ws, as := t.StockFor(tier)
	if len(ws) > 0 {
		c.Weapon = ws[len(ws)-1]
	}
	if len(as) > 0 {
		c.Armor = as[len(as)-1]
	}
}

// winRate simulates isolated on-curve fights at an encounter level.
func winRate(g *core.RNG, tables *gamedata.Tables, class model.Class, level, encLevel, n int) float64 {
	wins := 0
	for i := 0; i < n; i++ {
		c := rules.BuildCharacter(g, class, level)
		geared(tables, c)
		mons := tables.PickMonsters(g, balanceBiome(level), encLevel, 1)
		if len(mons) == 0 {
			continue
		}
		fresh := *c
		if r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
			encLevel, 60, tables.SpellsFor(c)); r.Won {
			wins++
		}
	}
	return float64(wins) / float64(n)
}

// TestOnLevelFightsAreWinnable: a random encounter where you are supposed to be
// should almost always be survivable. Wandering is the risk, not walking.
func TestOnLevelFightsAreWinnable(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(11)
	for _, level := range []int{1, 3, 5, 7, 9, 11, 13} {
		for _, class := range model.AllClasses {
			got := winRate(g, tables, class, level, level, 250)
			if got < 0.85 {
				t.Errorf("level %d %s wins only %.0f%% of on-level fights; "+
					"the expected path should not be a coin flip", level, class, got*100)
			}
		}
	}
}

// TestDangerRadiatesOutward: the world places harder locations further from the
// capital, and that only means anything if over-level fights are actually worse.
func TestDangerRadiatesOutward(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(12)
	for _, level := range []int{5, 9, 12} {
		on := winRate(g, tables, model.ClassFighter, level, level, 250)
		over := winRate(g, tables, model.ClassFighter, level, level+3, 250)
		if over >= on {
			t.Errorf("level %d fighter wins %.0f%% on-level and %.0f%% three levels up; "+
				"straying is meant to cost something", level, on*100, over*100)
		}
	}
}

// TestEnduranceHoldsAcrossLevels: how many fights you get from one rest governs
// how far you can wander, and it should not collapse as the game goes on. It
// used to run twelve fights at level 1 and two by level 9.
func TestEnduranceHoldsAcrossLevels(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(13)
	for _, level := range []int{1, 5, 9, 13} {
		probe := rules.BuildCharacter(g, model.ClassFighter, level)
		geared(tables, probe)
		spells := tables.SpellsFor(probe)

		total := 0
		const runs = 60
		for i := 0; i < runs; i++ {
			sim := rules.BuildCharacter(g, model.ClassFighter, level)
			geared(tables, sim)
			survived := 0
			for survived < 40 {
				mons := tables.PickMonsters(g, balanceBiome(level), level, 1)
				if len(mons) == 0 {
					break
				}
				r := rules.SimulateFight(g, sim, []*model.MonsterDef{mons[0].Def}, level, 60, spells)
				if !r.Won || sim.HP <= 0 {
					break
				}
				survived++
			}
			total += survived
		}
		avg := float64(total) / runs
		if avg < 2.5 {
			t.Errorf("level %d fighter manages only %.1f fights per rest; "+
				"the overworld becomes a walk back to the inn", level, avg)
		}
		if avg > 20 {
			t.Errorf("level %d fighter manages %.1f fights per rest; "+
				"resting has stopped mattering", level, avg)
		}
	}
}

// A hireling advertises a trade and then walks over wearing a sprite. Those
// have to agree: somebody selling themselves as a mage turning up dressed as a
// swordsman reads as a bug, and the two are picked separately.
func TestRecruitsLookLikeTheTradeTheySell(t *testing.T) {
	seen := map[string]bool{}
	for _, seed := range []int64{1, 42, 1994, 20260816} {
		m := world.Generate(seed, stubNamer{})
		for _, poi := range m.POIs {
			for _, e := range world.BuildLocal(poi, stubNamer{}).Entities {
				if e.Kind != world.ERecruit {
					continue
				}
				seen[e.Class] = true
				if e.Look == "" || e.Sprite != e.Look+"/idle" {
					t.Errorf("seed %d, %s: recruit has look %q and sprite %q, which do not agree",
						seed, poi.Name, e.Look, e.Sprite)
				}
				if !strings.HasPrefix(e.Look, "hero/") {
					t.Errorf("seed %d, %s: recruit walks on %q, not a hero sheet", seed, poi.Name, e.Look)
				}
				if want, ok := looksFor[e.Class]; !ok {
					t.Errorf("seed %d, %s: recruit sells %q, which is not a class", seed, poi.Name, e.Class)
				} else if !want[e.Look] {
					t.Errorf("seed %d, %s: a %s recruit turned up as %q", seed, poi.Name, e.Class, e.Look)
				}
			}
		}
	}
	// If no settlement in four continents put anybody outside an inn, the test
	// above proved nothing and would keep proving nothing after a regression.
	if len(seen) == 0 {
		t.Fatal("no recruits were generated in any of the sampled worlds")
	}
}

var looksFor = map[string]map[string]bool{
	"Fighter": {"hero/fighter": true},
	"Thief":   {"hero/thief": true},
	"Mage":    {"hero/mage": true, "hero/druid": true},
}

// Every technique in the table has to be coherent about who it is for. The
// targeting code reads the side off the kind and the breadth off the target,
// and a row that disagrees with itself would be an effect nobody can aim.
func TestSpellTargetingIsCoherent(t *testing.T) {
	tables := load(t)
	for _, s := range tables.Spells {
		if s.ID == "" || s.Name == "" {
			t.Errorf("a technique has no id or name: %+v", s)
		}
		switch s.Target {
		case model.TargetOne, model.TargetAll, model.TargetSelf:
		default:
			t.Errorf("%s targets %q, which is not a target", s.ID, s.Target)
		}
		switch s.Kind {
		case model.SpellDamage, model.SpellDrain, model.SpellWeaken, model.SpellStun,
			model.SpellHeal, model.SpellBless, model.SpellRevive:
		default:
			t.Errorf("%s is of kind %q, which nothing implements", s.ID, s.Kind)
		}
		// Nothing pointed at the monsters may be self-targeted: the caster is
		// not on that side of the field, and the battle screen would have to
		// invent a meaning for it.
		if s.Kind.Side() == model.SideFoes && s.Target == model.TargetSelf {
			t.Errorf("%s is aimed at the enemy but targets the caster", s.ID)
		}
		// A revive has to be able to reach somebody other than the caster; a
		// dead mage cannot cast anything.
		if s.Kind == model.SpellRevive && s.Target == model.TargetSelf {
			t.Errorf("%s revives only the caster, who by definition cannot cast it", s.ID)
		}
		if s.Cost < 0 || s.Level < 1 {
			t.Errorf("%s costs %d at level %d", s.ID, s.Cost, s.Level)
		}
	}
}

// A lineage technique is the reason to hire somebody who is not entirely a
// person, so each strain has to have one, and it must name a real lineage.
func TestEveryLineageHasATechniqueNobodyElseCanLearn(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(5)

	byBlood := map[model.MonsterKind][]model.Spell{}
	for _, s := range tables.Spells {
		if s.Blood == "" {
			continue
		}
		if _, ok := model.LineageOf(s.Blood); !ok {
			t.Errorf("%s is gated on blood %q, which is not a lineage", s.ID, s.Blood)
			continue
		}
		if s.Class != "" {
			t.Errorf("%s is gated on both %s ancestry and the %s class, so almost nobody can learn it",
				s.ID, s.Blood, s.Class)
		}
		byBlood[s.Blood] = append(byBlood[s.Blood], s)
	}

	for _, l := range model.Lineages {
		if len(byBlood[l.Kind]) == 0 {
			t.Errorf("nothing in the technique table is unique to a part-%s hireling", l.Kind)
		}
	}

	// And a hero must never be able to learn one, whatever their class or level.
	for _, class := range model.AllClasses {
		hero := rules.BuildCharacter(g, class, 20)
		for _, s := range tables.SpellsFor(hero) {
			if s.Blood != "" {
				t.Errorf("a level 20 %s can learn %s, which is meant to need %s ancestry",
					class, s.ID, s.Blood)
			}
		}
	}

	// A hireling with the ancestry must actually get it in a usable timeframe.
	for _, l := range model.Lineages {
		mate := rules.Recruit(g, "Mate", model.ClassFighter, l.Kind, 5)
		found := false
		for _, s := range tables.SpellsFor(mate) {
			if s.Blood == l.Kind {
				found = true
			}
		}
		if !found {
			t.Errorf("a level 5 part-%s hireling knows none of their own techniques", l.Kind)
		}
	}
}

// Standing somebody back up needs something in the world that does it, or the
// rules for it are unreachable.
func TestSomethingInTheWorldRevives(t *testing.T) {
	tables := load(t)
	items := 0
	for _, it := range tables.Items {
		if it.Kind == model.ItemRevive {
			items++
			if it.Power <= 0 {
				t.Errorf("%s revives for %d", it.Name, it.Power)
			}
		}
	}
	if items == 0 {
		t.Error("no item in the game stands a fallen party member back up")
	}
	spells := 0
	for _, s := range tables.Spells {
		if s.Kind == model.SpellRevive {
			spells++
		}
	}
	if spells == 0 {
		t.Error("no technique in the game stands a fallen party member back up")
	}
}

// A part-monster hireling leads with their ancestry, because they have learned
// the alternative is having the conversation twice. Every lineage therefore
// needs its own pitch, or a part-ooze opens with the generic line and the whole
// point of the lineage never reaches the player.
func TestEveryLineageHasSomethingToSay(t *testing.T) {
	tables := load(t)
	if len(tables.Text.RecruitPitch) == 0 {
		t.Fatal("nobody has a hiring pitch at all")
	}
	for _, l := range model.Lineages {
		lines := tables.Text.BloodPitch[string(l.Kind)]
		if len(lines) == 0 {
			t.Errorf("a part-%s hireling has nothing of their own to say", l.Kind)
		}
		for _, ln := range lines {
			if ln == "" {
				t.Errorf("a part-%s pitch is empty", l.Kind)
			}
		}
	}
	// The lines the party's two new endings need.
	if len(tables.Text.Rescue) == 0 {
		t.Error("nothing describes the company carrying a fallen hero to town")
	}
	if len(tables.Text.Revived) == 0 {
		t.Error("nothing describes somebody being stood back up")
	}
	// {N} and {P} are substituted at runtime; a line missing its slot silently
	// drops the name of whoever did the carrying.
	for _, ln := range tables.Text.Rescue {
		if !strings.Contains(ln, "{N}") || !strings.Contains(ln, "{P}") {
			t.Errorf("rescue line is missing a substitution slot: %q", ln)
		}
	}
	for _, ln := range tables.Text.Revived {
		if !strings.Contains(ln, "{N}") {
			t.Errorf("revive line never names anybody: %q", ln)
		}
	}
}
