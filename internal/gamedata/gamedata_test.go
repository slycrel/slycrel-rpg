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

// geared dresses a subject the way the game would. It goes through Equip rather
// than reaching into the shelf itself, which it used to do: since weapons have
// lanes, "the last row of the tier" is a rod a fighter cannot hold and a maul a
// mage cannot lift, and a test that dressed everybody out of the same drawer
// would be measuring a character nobody can build.
func geared(t *gamedata.Tables, c *model.Character) { t.Equip(c) }

// winRate simulates isolated on-curve fights at an encounter level, in the
// region where that level of thing actually lives.
//
// The biome comes off encLevel rather than off the character's level, which is
// the same correction cmd/balance had to make and for the same reason: eight of
// fourteen levels cannot supply a fight three bands over their own doorstep, so
// asking the local biome for one gets whatever it has, which is a fight the
// player has already outgrown. Straying means going somewhere rougher, and the
// probe has to go there too or it is measuring the walk rather than the risk.
func winRate(g *core.RNG, tables *gamedata.Tables, class model.Class, level, encLevel, n int) float64 {
	wins := 0
	for i := 0; i < n; i++ {
		c := rules.BuildCharacter(g, class, level)
		geared(tables, c)
		mons := tables.PickMonsters(g, balanceBiome(encLevel), encLevel, 1)
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
			model.SpellPoison, model.SpellBurn, model.SpellSap, model.SpellPact,
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

// Afflictions are what make two monsters with the same stat line different to
// meet. Every one has to name a real condition and be rollable, and they have
// to stay a minority: if most things poison you, poison is just damage with
// extra words.
func TestMonsterAfflictionsAreSaneAndRare(t *testing.T) {
	tables := load(t)
	known := map[model.EffectKind]bool{
		model.EffectPoison: true, model.EffectBurn: true,
		model.EffectWeaken: true, model.EffectStun: true,
	}

	total, afflicting := 0, 0
	kinds := map[model.EffectKind]int{}
	for _, d := range tables.ByID {
		total++
		a := d.Inflicts
		if a == nil {
			continue
		}
		afflicting++
		kinds[a.Kind]++
		if !known[a.Kind] {
			t.Errorf("%s inflicts %q, which is not a condition a monster should apply", d.ID, a.Kind)
		}
		if a.Chance <= 0 || a.Chance > 100 {
			t.Errorf("%s inflicts %s with a %d%% chance", d.ID, a.Kind, a.Chance)
		}
		if a.Power < 1 {
			t.Errorf("%s inflicts %s at power %d", d.ID, a.Kind, a.Power)
		}
		// A ticking condition needs a duration; a permanent poison would run
		// for the whole fight and read as a second health bar.
		if a.Kind == model.EffectPoison || a.Kind == model.EffectBurn {
			if a.Rounds < 1 {
				t.Errorf("%s inflicts %s for %d rounds", d.ID, a.Kind, a.Rounds)
			}
		}
	}

	if afflicting == 0 {
		t.Fatal("nothing in the game inflicts a condition")
	}
	if share := float64(afflicting) / float64(total); share > 0.4 {
		t.Errorf("%d of %d monsters inflict a condition (%.0f%%), which is no longer a minority",
			afflicting, total, share*100)
	}
	// The lines that name it landing have to exist for every kind in use.
	for k := range kinds {
		if len(tables.Text.Afflicted[string(k)]) == 0 {
			t.Errorf("monsters inflict %s and nothing describes it happening", k)
		}
	}
}

// A condition the player can inflict but never shed would be a one-way ratchet.
func TestConditionsHaveACounter(t *testing.T) {
	tables := load(t)
	cures := 0
	for _, it := range tables.Items {
		if it.Kind == model.ItemCure {
			cures++
		}
	}
	if cures == 0 {
		t.Error("nothing in the game clears a condition")
	}
}

// Affixes follow the same rule as lineages: every one gives with one hand and
// takes with the other. A table of pure upgrades would make "is it affixed" the
// only question worth asking about a piece of gear, and a find in a chest would
// never be a decision.
func TestEveryAffixGivesAndTakes(t *testing.T) {
	tables := load(t)
	if len(tables.Affixes) == 0 {
		t.Fatal("no affixes at all")
	}
	seen := map[string]bool{}
	for _, a := range tables.Affixes {
		if a.Suffix == "" {
			t.Errorf("an affix has no suffix: %+v", a)
			continue
		}
		if seen[a.Suffix] {
			t.Errorf("%q appears twice", a.Suffix)
		}
		seen[a.Suffix] = true
		if !strings.HasPrefix(a.Suffix, "of ") {
			t.Errorf("%q does not read as a suffix", a.Suffix)
		}
		if a.Tier < 1 || a.Tier > 5 {
			t.Errorf("%q is tier %d, outside the gear bands", a.Suffix, a.Tier)
		}

		var gives, takes bool
		for _, n := range []int{a.Bonus.Strike, a.Bonus.Defense, a.Bonus.Strength,
			a.Bonus.Dexterity, a.Bonus.Speed, a.Bonus.Psyche} {
			if n > 0 {
				gives = true
			}
			if n < 0 {
				takes = true
			}
		}
		if !gives {
			t.Errorf("%q gives nothing", a.Suffix)
		}
		if !takes {
			t.Errorf("%q takes nothing, so it is a straight upgrade", a.Suffix)
		}
	}
	// The first gear band needs something, or a low chest can never produce a
	// find at all.
	if _, ok := tables.PickAffix(core.NewRNG(1), 1); !ok {
		t.Error("no affix is available in the lowest gear band")
	}
}

// A suffix must never turn up on gear too cheap to deserve it.
func TestPickAffixRespectsTheBand(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(7)
	for tier := 1; tier <= 5; tier++ {
		for i := 0; i < 300; i++ {
			a, ok := tables.PickAffix(g, tier)
			if !ok {
				t.Fatalf("tier %d produced no affix", tier)
			}
			if a.Tier > tier {
				t.Fatalf("tier %d produced %q, which is tier %d", tier, a.Suffix, a.Tier)
			}
		}
	}
}

// Shields and charms have to be worth their price and readable in a shop list.
func TestSidearmsAreSaneAndAffordableInOrder(t *testing.T) {
	tables := load(t)
	if len(tables.Shields) == 0 || len(tables.Charms) == 0 {
		t.Fatal("the new slots have nothing to put in them")
	}
	keys := manifestKeys(t)

	for _, s := range tables.Shields {
		if s.Name == "" || s.Verb == "" {
			t.Errorf("a shield is missing a name or a verb: %+v", s)
		}
		if s.Defense < 1 {
			t.Errorf("%q blocks %d", s.Name, s.Defense)
		}
		if s.Tier < 1 || s.Tier > 5 || s.Cost < 1 {
			t.Errorf("%q is tier %d at %d coins", s.Name, s.Tier, s.Cost)
		}
		if !keys[s.Icon] {
			t.Errorf("%q wants icon %q, which the manifest does not provide", s.Name, s.Icon)
		}
	}

	for _, c := range tables.Charms {
		if c.Name == "" || c.Desc == "" {
			t.Errorf("a charm is missing a name or a description: %+v", c)
		}
		if c.Bonus.Empty() {
			t.Errorf("%q does nothing", c.Name)
		}
		if c.Tier < 1 || c.Tier > 5 || c.Cost < 1 {
			t.Errorf("%q is tier %d at %d coins", c.Name, c.Tier, c.Cost)
		}
		if !keys[c.Icon] {
			t.Errorf("%q wants icon %q, which the manifest does not provide", c.Name, c.Icon)
		}
		// A charm is a trade, like an affix and like a lineage.
		var gives, takes bool
		for _, n := range []int{c.Bonus.Strike, c.Bonus.Defense, c.Bonus.Strength,
			c.Bonus.Dexterity, c.Bonus.Speed, c.Bonus.Psyche, c.Bonus.Ward} {
			if n > 0 {
				gives = true
			}
			if n < 0 {
				takes = true
			}
		}
		if !gives || !takes {
			t.Errorf("%q gives=%v takes=%v; a charm should be a trade", c.Name, gives, takes)
		}
	}
}

// A shield must never be worth more than the body armour of its own band, or
// the slot stops being a sidearm and starts being the point.
func TestShieldsStaySecondaryToArmour(t *testing.T) {
	tables := load(t)
	bestArmour := map[int]int{}
	for _, a := range tables.Armors {
		if a.Defense > bestArmour[a.Tier] {
			bestArmour[a.Tier] = a.Defense
		}
	}
	for _, s := range tables.Shields {
		if best := bestArmour[s.Tier]; best > 0 && s.Defense*2 > best {
			t.Errorf("%q blocks %d against the %d of tier %d body armour, which is not secondary",
				s.Name, s.Defense, best, s.Tier)
		}
	}
}

// --- archetypes -----------------------------------------------------------

// Equip is what dresses a hireling, a save fixture and every simulated subject
// in the balance report, and it has to keep meaning exactly what it meant
// before archetypes existed. If it ever drifts to some other build, the report
// and the hiring board start describing different games and nothing says so.
func TestEquipIsStillTheBalancedArchetype(t *testing.T) {
	tables := load(t)

	if got := gamedata.Archetypes[0].Name; got != "balanced" {
		t.Fatalf("the first archetype is %q; Equip delegates to index 0 and expects balanced", got)
	}

	for level := 1; level <= 14; level++ {
		viaEquip := &model.Character{Level: level}
		tables.Equip(viaEquip)

		viaArchetype := &model.Character{Level: level}
		tables.EquipAs(viaArchetype, gamedata.Archetypes[0])

		// Slot by slot rather than whole-struct: a Character carries a bag, and
		// a slice makes the struct incomparable.
		if viaEquip.Weapon.Titled() != viaArchetype.Weapon.Titled() ||
			viaEquip.Armor.Titled() != viaArchetype.Armor.Titled() ||
			viaEquip.Shield.Titled() != viaArchetype.Shield.Titled() ||
			viaEquip.Charm.Titled() != viaArchetype.Charm.Titled() {
			t.Errorf("level %d: Equip and EquipAs(balanced) dressed two different characters:\n"+
				"  Equip:   %s / %s / %s / %s\n  EquipAs: %s / %s / %s / %s", level,
				viaEquip.Weapon.Titled(), viaEquip.Armor.Titled(),
				viaEquip.Shield.Titled(), viaEquip.Charm.Titled(),
				viaArchetype.Weapon.Titled(), viaArchetype.Armor.Titled(),
				viaArchetype.Shield.Titled(), viaArchetype.Charm.Titled())
		}

		// And the assumption itself: main gear at tier, sidearms a band behind,
		// nothing on the sidearms in the first band.
		tier := gamedata.GearTierFor(level)
		if viaEquip.Weapon.Tier > tier || viaEquip.Armor.Tier > tier {
			t.Errorf("level %d: balanced bought above its tier %d", level, tier)
		}
		if tier < 2 && (viaEquip.Shield.Worn() || viaEquip.Charm.Worn()) {
			t.Errorf("level %d: a first-band character turned up with sidearms they could not afford", level)
		}
		if viaEquip.Shield.Worn() && viaEquip.Shield.Tier >= tier {
			t.Errorf("level %d: the shield is tier %d, not a band behind %d",
				level, viaEquip.Shield.Tier, tier)
		}
	}
}

// Whatever else an archetype trades away, it is still a person going into a
// fight. A build with an empty weapon or armour slot is a spec error, and it
// would show up in the report as a mysteriously terrible playstyle rather than
// as the mistake it is — which is exactly how the first draft of the duelist
// lost by twenty-two points.
func TestEveryArchetypeArrivesDressed(t *testing.T) {
	tables := load(t)
	for _, a := range gamedata.Archetypes {
		for level := 1; level <= 14; level++ {
			c := &model.Character{Level: level}
			tables.EquipAs(c, a)
			if c.Weapon.Name == "" {
				t.Errorf("%s at level %d has nothing to swing", a.Name, level)
			}
			if c.Armor.Name == "" {
				t.Errorf("%s at level %d has nothing on", a.Name, level)
			}
			if a.Shield.Skip && c.Shield.Worn() {
				t.Errorf("%s at level %d is carrying a shield it does not use", a.Name, level)
			}
		}
	}
}

// The top of each lane in each gear band is what Equip buys for the class that
// fights in that lane, so those numbers are the ladder every archetype trades
// bands along. Each lane has to climb evenly.
//
// They did not, before there were lanes: the weapon tops ran 5, 7, 12, 17, 21 —
// a +2 step into tier 2 and +5 into tier 3 — which meant "a band behind on the
// weapon" cost two and a half times as much at one tier as at the next, and any
// build paying in weapon bands lurched in and out of viability by level.
//
// The test is per lane now because there is no longer one ladder. A Fighter's
// two-handed top, a Thief's dagger and a Mage's focus climb separately, and a
// step that is even overall while one lane stalls is exactly the failure this
// is here to catch — it would read as a class quietly falling off the curve at
// one tier, which is the hardest kind of imbalance to see from play.
//
// Armour is deliberately not asserted here. Its steps are also uneven and
// nothing in the report currently blames them for anything, and pinning a rule
// that has not earned itself is how a table ends up shaped by its tests rather
// than by play.
func TestWeaponBandsStepEvenly(t *testing.T) {
	tables := load(t)

	lanes := map[string]func(model.Weapon) int{
		"two-handed": func(w model.Weapon) int {
			if !w.TwoHanded() {
				return 0
			}
			return w.Strike
		},
		"one-handed": func(w model.Weapon) int {
			if w.TwoHanded() || w.Kind == model.WeaponDagger || w.Kind == model.WeaponFocus {
				return 0
			}
			return w.Strike
		},
		"dagger": func(w model.Weapon) int {
			if w.Kind != model.WeaponDagger {
				return 0
			}
			return w.Strike
		},
		"focus": func(w model.Weapon) int {
			if w.Kind != model.WeaponFocus {
				return 0
			}
			return w.Focus
		},
	}

	for lane, rate := range lanes {
		var tops []int
		for tier := 1; tier <= 5; tier++ {
			ws, _ := tables.StockFor(tier)
			best := 0
			for _, w := range ws {
				if v := rate(w); v > best {
					best = v
				}
			}
			if best == 0 {
				t.Errorf("the %s lane has nothing at tier %d, so a class that "+
					"fights in it has no band to buy there", lane, tier)
				break
			}
			tops = append(tops, best)
		}
		if len(tops) < 5 {
			continue
		}

		var steps []int
		for i := 1; i < len(tops); i++ {
			steps = append(steps, tops[i]-tops[i-1])
		}
		lo, hi := steps[0], steps[0]
		for _, s := range steps {
			lo, hi = core.Min(lo, s), core.Max(hi, s)
		}
		if hi-lo > 1 {
			t.Errorf("%s band steps are %v across tops %v; the widest is %d and the "+
				"narrowest %d, so a band behind means something different at every tier",
				lane, steps, tops, hi, lo)
		}
		if lo < 1 {
			t.Errorf("%s band steps are %v; a band that buys nothing is not a band", lane, steps)
		}
	}
}

// An encounter's level is a promise about the fight, and model.Spawn scales a
// creature up to that level but never down — so a definition picked from above
// it arrives at its own full strength and the promise is broken. A death there
// is not the consequence of anything the player chose.
//
// One band of overshoot is deliberate; more than that was happening between a
// fifth and a half of the time, by as much as five levels.
func TestEncountersDoNotArriveAboveTheLevelTheyPromise(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(21)
	for _, biome := range []string{"plains", "forest", "hills", "swamp", "dungeon", "mountain"} {
		// The one legitimate exception: asking below the bottom of a roster.
		// A dungeon has nothing under level three and something has to be
		// sent, so the floor of the biome is allowed however far over it is.
		floor := 1 << 30
		for _, d := range tables.Monsters[biome] {
			if d.Level < floor {
				floor = d.Level
			}
		}
		for level := 1; level <= 18; level++ {
			for i := 0; i < 300; i++ {
				ms := tables.PickMonsters(g, biome, level, 1)
				if len(ms) == 0 {
					continue
				}
				got := ms[0].Def.Level
				if got <= level+1 || (level < floor && got == floor) {
					continue
				}
				t.Fatalf("%s at level %d rolled %s, which is level %d (%+d); "+
					"the biome floor is %d",
					biome, level, ms[0].Def.Name, got, got-level, floor)
			}
		}
	}
}

// Weighting by closeness has to keep ranking all the way out, not clamp to a
// floor. Once every creature in a roster sits on the floor the pick goes
// uniform, which is precisely the case at the top of the game: an encounter
// five levels over a level-13 hero was drawing a level-5 wolf as often as a
// level-14 dragon, then scaling the wolf up thirteen levels.
func TestTheNearestMonsterStaysLikeliestEvenFarOutOfRange(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(22)

	// A target well above everything mountain contains.
	const level = 18
	counts := map[int]int{}
	for i := 0; i < 4000; i++ {
		ms := tables.PickMonsters(g, "mountain", level, 1)
		if len(ms) == 0 {
			continue
		}
		counts[ms[0].Def.Level]++
	}

	var lowest, highest int
	for lv := range counts {
		if lowest == 0 || lv < lowest {
			lowest = lv
		}
		if lv > highest {
			highest = lv
		}
	}
	if highest == lowest {
		t.Skip("the roster has only one level in it; nothing to rank")
	}
	if counts[highest] <= counts[lowest] {
		t.Errorf("asking for level %d drew the level-%d creature %d times and the "+
			"level-%d one %d times; closeness has stopped ranking",
			level, highest, counts[highest], lowest, counts[lowest])
	}
}

// Ward is a slot the player is allowed to ignore, and that is only a fair offer
// if the threat it answers arrives after the answer is purchasable. A creature
// that goes through armour instead of into it turning up before any shop sells
// resistance is the un-fun surprise the difficulty brief rules out: there was
// no choice to make, only an outcome.
func TestMagicAttackersArriveAfterTheAnswerIsForSale(t *testing.T) {
	tables := load(t)

	// The level at which the first anti-magic charm is on a shelf.
	firstWard := 1 << 30
	for _, c := range tables.Charms {
		if c.Bonus.Ward <= 0 {
			continue
		}
		// A tier is three levels; a tier-3 charm is stocked from level 7.
		if lv := (c.Tier-1)*3 + 1; lv < firstWard {
			firstWard = lv
		}
	}
	if firstWard == 1<<30 {
		t.Fatal("nothing in the game grants ward, so nothing can answer a magical attacker")
	}

	// A player straying three levels is still making a choice; being ambushed
	// by a mechanic they could not have prepared for is not.
	const stray = 3
	for _, defs := range tables.Monsters {
		for _, d := range defs {
			if !d.Magic {
				continue
			}
			if d.Level-stray < firstWard {
				t.Errorf("%s attacks with magic at level %d, but the first ward charm "+
					"is not sold until level %d — a level %d character can meet it "+
					"with no way to have answered",
					d.Name, d.Level, firstWard, d.Level-stray)
			}
		}
	}
}

// Ward has to scale with the encounter like every other combat stat, or a
// warded creature met deep in the world stops being warded.
func TestSpawnScalesWard(t *testing.T) {
	g := core.NewRNG(31)
	def := &model.MonsterDef{Name: "Test", Level: 4, HP: 40, Offense: 10, Defense: 5, Ward: 8, Speed: 8}
	near := def.Spawn(g, 4)
	far := def.Spawn(g, 12)
	if near.Ward != def.Ward {
		t.Errorf("met at its own level, ward is %d and should be %d", near.Ward, def.Ward)
	}
	if far.Ward <= near.Ward {
		t.Errorf("met eight levels up, ward is %d against %d at home; it did not scale",
			far.Ward, near.Ward)
	}
}

// Every standing a town can read has something to say about it, except the one
// that should not: "nobody" gets no lines on purpose, because having no
// reputation is not a reaction and a townsperson remarking on your lack of one
// would be one.
func TestEveryStandingHasSomethingSaidAboutIt(t *testing.T) {
	tables := load(t)
	for _, s := range []rules.Standing{
		rules.Rumoured, rules.Celebrated, rules.Recognised, rules.Notorious,
	} {
		lines := tables.Text.StandingLine[s.Key()]
		if len(lines) < 2 {
			t.Errorf("%q has %d things said about it; one line becomes a catchphrase",
				s.Name(), len(lines))
		}
		for _, ln := range lines {
			if ln == "" {
				t.Errorf("%q has an empty line", s.Name())
			}
		}
	}
	if got := tables.Text.StandingLine[rules.Unknown.Key()]; len(got) > 0 {
		t.Errorf("being a nobody has %d lines written for it; it should have none", len(got))
	}
}

// --- class lanes -----------------------------------------------------------

// Equip is what dresses a hireling, a fixture and every simulated subject, and
// it is now the only thing that knows a class cannot hold everything. If it
// ever hands somebody a piece their own character sheet would refuse, the
// balance report is measuring a character the player cannot build — which is
// precisely the failure the archetype work already found once, in the slot
// where an underspending build measured the spec rather than the content.
func TestNobodyIsDressedInGearTheyCannotUse(t *testing.T) {
	tables := load(t)
	for _, class := range []model.Class{model.ClassFighter, model.ClassThief, model.ClassMage} {
		for level := 1; level <= 14; level++ {
			for _, a := range gamedata.Archetypes {
				c := &model.Character{Level: level, Class: class}
				tables.EquipAs(c, a)
				if !model.CanWield(class, c.Weapon) {
					t.Errorf("%s %s at level %d is holding %q, which that class cannot",
						class, a.Name, level, c.Weapon.Name)
				}
				if !model.CanWear(class, c.Armor) {
					t.Errorf("%s %s at level %d is wearing %q, which that class cannot",
						class, a.Name, level, c.Armor.Name)
				}
				if c.Shield.Worn() && !c.CanHold() {
					t.Errorf("%s %s at level %d has a shield on an arm that is not free",
						class, a.Name, level)
				}
			}
		}
	}
}

// Every class needs a ladder of its own to climb, at every band a shop stocks.
// A lane that runs out two tiers early is a class that quietly stops improving
// halfway through the game, and it would read from play as "the numbers got
// hard around level nine" rather than as the content gap it is.
func TestEveryClassCanKeepBuyingUpgrades(t *testing.T) {
	tables := load(t)
	for _, class := range []model.Class{model.ClassFighter, model.ClassThief, model.ClassMage} {
		var lastW, lastA string
		for tier := 1; tier <= 5; tier++ {
			c := &model.Character{Level: (tier-1)*3 + 1, Class: class}
			tables.Equip(c)
			if c.Weapon.Name == "" || c.Armor.Name == "" {
				t.Fatalf("%s has nothing to buy at tier %d", class, tier)
			}
			if tier > 1 && c.Weapon.Name == lastW && c.Armor.Name == lastA {
				t.Errorf("%s buys nothing new at tier %d: still %q and %q",
					class, tier, lastW, lastA)
			}
			lastW, lastA = c.Weapon.Name, c.Armor.Name
		}
	}
}

// A caster's shopping list is the focus ladder, so a Mage has to actually end
// up holding a focus. Ranking their shelf by strike would send them home with a
// dagger every time — the rod is deliberately terrible at hitting people, which
// is the whole trade — and the free bolt would never fire.
func TestAMageIsSoldSomethingToCastWith(t *testing.T) {
	tables := load(t)
	for level := 1; level <= 14; level++ {
		c := &model.Character{Level: level, Class: model.ClassMage}
		tables.Equip(c)
		if !c.Casting() {
			t.Errorf("level %d mage is on-curve holding %q, which casts nothing",
				level, c.Weapon.Name)
		}
	}
}

// The kit a new character is handed has to be a kit they can put on.
//
// This broke the moment weapons had lanes: the starter was "the cheapest row in
// the file", the cheapest row is a table leg, and a Mage may not hold one. A
// character sheet showing gear beside a pack that refuses to equip it is worse
// than starting with nothing, because nothing at least explains itself.
func TestTheStartingKitFitsTheClassItIsIssuedTo(t *testing.T) {
	tables := load(t)
	for _, class := range []model.Class{model.ClassFighter, model.ClassThief, model.ClassMage} {
		w, a := tables.StarterKit(class)
		if !model.CanWield(class, w) {
			t.Errorf("a new %s opens holding %q, which that class cannot use", class, w.Name)
		}
		if !model.CanWear(class, a) {
			t.Errorf("a new %s opens wearing %q, which that class cannot use", class, a.Name)
		}
		if w.Tier < 1 || a.Tier < 1 {
			t.Errorf("a new %s opens with tier-zero kit (%q / %q), which is the name "+
				"this game gives to owning nothing", class, w.Name, a.Name)
		}
		// And below the curve, or the first shop has nothing to sell them.
		curve := &model.Character{Level: 1, Class: class}
		tables.Equip(curve)
		if w.Cost >= curve.Weapon.Cost && a.Cost >= curve.Armor.Cost {
			t.Errorf("a new %s opens already on curve; the first morning is supposed to "+
				"be a shopping trip", class)
		}
	}
}

// A technique with two sides has to have both. A sap that weakened nobody or a
// pact that cost the caster nothing would be an ordinary technique with a
// misleading name, and the house rule the whole content layer follows is that
// everything which gives must take.
func TestTheTwoSidedTechniquesAreActuallyTwoSided(t *testing.T) {
	saps, pacts := 0, 0
	for _, s := range load(t).Spells {
		switch s.Kind {
		case model.SpellSap:
			saps++
			if s.Power < 1 {
				t.Errorf("%s saps nothing, so it takes from nobody and gives to nobody", s.ID)
			}
		case model.SpellPact:
			pacts++
			if rules.PactCost(s) < 1 {
				t.Errorf("%s is a pact that costs the caster nothing", s.ID)
			}
			if rules.PactCost(s) >= s.Power {
				t.Errorf("%s costs the caster %d for %d of effect, which is not a bargain "+
					"anybody would take", s.ID, rules.PactCost(s), s.Power)
			}
		}
	}
	if saps == 0 || pacts == 0 {
		t.Errorf("the table holds %d saps and %d pacts; the pair only means something "+
			"if both directions exist", saps, pacts)
	}
}

// Each class's list has to be more than three flavours of "hit it". A technique
// that only ever means "a bigger swing this round" is a number, and the reason
// to spend psyche on one rather than swinging is that it does something a swing
// cannot: land on everything, linger past the round, or move a stat.
func TestNoClassIsJustThreeKindsOfDamage(t *testing.T) {
	tables := load(t)
	for _, class := range []model.Class{model.ClassFighter, model.ClassThief, model.ClassMage} {
		plain, lasting := 0, 0
		for _, s := range tables.Spells {
			if s.Class != class {
				continue
			}
			switch s.Kind {
			case model.SpellDamage, model.SpellDrain, model.SpellHeal:
				if s.Target != model.TargetAll {
					plain++
				} else {
					lasting++
				}
			default:
				lasting++
			}
		}
		if lasting < plain {
			t.Errorf("%s has %d techniques that are a better swing and only %d that do "+
				"something a swing cannot", class, plain, lasting)
		}
	}
}
