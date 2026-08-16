// Package gamedata loads the JSON content tables off disk. Content lives in
// data/ as plain JSON rather than baked into the binary so that balance passes
// and new jokes are a text edit and a relaunch, not a recompile.
package gamedata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// Tables is every content table the game needs, loaded once at boot.
type Tables struct {
	// Monsters is keyed by biome ("forest", "swamp", "dungeon", ...), matching
	// the filenames under data/monsters/.
	Monsters map[string][]*model.MonsterDef
	// ByID indexes every monster regardless of biome.
	ByID map[string]*model.MonsterDef

	Weapons []model.Weapon
	Armors  []model.Armor
	Items   map[string]model.Item
	Spells  []model.Spell

	Text Text
}

// Text is the writing room: word banks the generators recombine at runtime.
type Text struct {
	// Name parts for people and places.
	GivenNames  []string `json:"givenNames"`
	Epithets    []string `json:"epithets"`
	PlacePrefix []string `json:"placePrefix"`
	PlaceSuffix []string `json:"placeSuffix"`
	PlaceOf     []string `json:"placeOf"`

	// Point-of-interest colour, keyed by POI kind.
	PoiTagline map[string][]string `json:"poiTagline"`
	PoiRumor   map[string][]string `json:"poiRumor"`

	// Combat and world chatter.
	HitFlavor   []string `json:"hitFlavor"`
	CritFlavor  []string `json:"critFlavor"`
	MissFlavor  []string `json:"missFlavor"`
	DeathFlavor []string `json:"deathFlavor"`
	LevelFlavor []string `json:"levelFlavor"`
	IdleFlavor  []string `json:"idleFlavor"`
	NpcLine     []string `json:"npcLine"`
	SignText    []string `json:"signText"`

	// Hirelings: the sales pitch, the handshake, the parting, and what happens
	// when one of them stops being upright.
	RecruitPitch []string `json:"recruitPitch"`
	RecruitJoin  []string `json:"recruitJoin"`
	RecruitLeave []string `json:"recruitLeave"`
	AllyDown     []string `json:"allyDown"`
	AllyUp       []string `json:"allyUp"`
	// BloodPitch is what a hireling who is visibly not entirely human opens
	// with, keyed by the model.MonsterKind string of their ancestry.
	BloodPitch map[string][]string `json:"bloodPitch"`
	// Rescue and Revived cover getting back up: carried into town by the
	// company, and stood up mid-fight by an item or a technique.
	Rescue  []string `json:"rescue"`
	Revived []string `json:"revived"`

	// Quest lines, keyed by quest kind then by part (ask / nag / thank).
	Quest map[string]map[string][]string `json:"quest"`
}

// FindRoot locates the game root — the folder holding data/ — by walking up
// from the working directory, then from the executable's own directory.
//
// The working directory is tried first so `go run ./cmd/slycrel` picks up the
// repository being edited rather than a stale build in $GOCACHE. The executable
// path is the fallback that makes a distributed build work: a double-clicked
// binary inherits a working directory of / or the user's home, which would
// otherwise fail even though data/ sits right beside it.
func FindRoot() (string, error) {
	var tried []string

	if dir, err := os.Getwd(); err == nil {
		if root, ok := walkUpForData(dir); ok {
			return root, nil
		}
		tried = append(tried, dir)
	}

	if exe, err := os.Executable(); err == nil {
		if exe, err := filepath.EvalSymlinks(exe); err == nil {
			dir := filepath.Dir(exe)
			if root, ok := walkUpForData(dir); ok {
				return root, nil
			}
			tried = append(tried, dir)
		}
	}

	return "", fmt.Errorf("could not find a data/ directory above any of %v", tried)
}

// walkUpForData climbs at most 8 levels from dir looking for a data/ folder.
func walkUpForData(dir string) (string, bool) {
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(filepath.Join(dir, "data")); err == nil && fi.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

// Load reads every table under root/data.
func Load(root string) (*Tables, error) {
	t := &Tables{
		Monsters: map[string][]*model.MonsterDef{},
		ByID:     map[string]*model.MonsterDef{},
		Items:    map[string]model.Item{},
	}
	dd := filepath.Join(root, "data")

	// Monsters: one file per biome.
	entries, err := os.ReadDir(filepath.Join(dd, "monsters"))
	if err != nil {
		return nil, fmt.Errorf("reading monsters: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		biome := strings.TrimSuffix(e.Name(), ".json")
		var defs []*model.MonsterDef
		if err := readJSON(filepath.Join(dd, "monsters", e.Name()), &defs); err != nil {
			return nil, err
		}
		t.Monsters[biome] = defs
		for _, d := range defs {
			t.ByID[d.ID] = d
		}
	}
	if len(t.ByID) == 0 {
		return nil, fmt.Errorf("no monsters found under %s", filepath.Join(dd, "monsters"))
	}

	if err := readJSON(filepath.Join(dd, "items", "weapons.json"), &t.Weapons); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "items", "armor.json"), &t.Armors); err != nil {
		return nil, err
	}
	var items []model.Item
	if err := readJSON(filepath.Join(dd, "items", "items.json"), &items); err != nil {
		return nil, err
	}
	for _, it := range items {
		t.Items[it.Name] = it
	}
	if err := readJSON(filepath.Join(dd, "items", "spells.json"), &t.Spells); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "text", "flavor.json"), &t.Text); err != nil {
		return nil, err
	}

	sort.Slice(t.Weapons, func(i, j int) bool { return t.Weapons[i].Cost < t.Weapons[j].Cost })
	sort.Slice(t.Armors, func(i, j int) bool { return t.Armors[i].Cost < t.Armors[j].Cost })
	return t, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
	}
	return nil
}

// Item looks up an item template by name.
func (t *Tables) Item(name string) (model.Item, bool) {
	it, ok := t.Items[name]
	return it, ok
}

// StarterWeapon returns the cheapest weapon, used to arm a new character.
func (t *Tables) StarterWeapon() model.Weapon {
	if len(t.Weapons) == 0 {
		return model.Weapon{Name: "Bare Hands", Strike: 1, Verb: "slap"}
	}
	return t.Weapons[0]
}

// StarterArmor returns the cheapest armor.
func (t *Tables) StarterArmor() model.Armor {
	if len(t.Armors) == 0 {
		return model.Armor{Name: "Rags", Verb: "flaps"}
	}
	return t.Armors[0]
}

// SpellsFor returns the spells a character currently knows.
func (t *Tables) SpellsFor(c *model.Character) []model.Spell {
	var out []model.Spell
	for _, s := range t.Spells {
		if s.Known(c) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost < out[j].Cost })
	return out
}

// StockFor returns the gear a shop of the given tier carries, widest at the
// capital and thinnest in a hamlet with one anvil and a grudge.
func (t *Tables) StockFor(tier int) ([]model.Weapon, []model.Armor) {
	var ws []model.Weapon
	for _, w := range t.Weapons {
		if w.Tier <= tier {
			ws = append(ws, w)
		}
	}
	var as []model.Armor
	for _, a := range t.Armors {
		if a.Tier <= tier {
			as = append(as, a)
		}
	}
	return ws, as
}

// GearTierFor is the gear band a character is expected to be carrying at a
// level. The shops stock by tier and tiers span roughly three levels each, so
// this is the "on curve" assumption: it is what the balance report measures
// against, and what a hireling turns up already wearing.
func GearTierFor(level int) int { return core.Clamp(1+(level-1)/3, 1, 5) }

// Equip fits a character with the best gear of their expected tier. Anyone
// arriving mid-game — a companion for hire, a simulated subject — is dressed
// through here, so there is one definition of what "level N and properly
// equipped" means rather than one per caller.
func (t *Tables) Equip(c *model.Character) {
	ws, as := t.StockFor(GearTierFor(c.Level))
	if len(ws) > 0 {
		c.Weapon = ws[len(ws)-1]
	}
	if len(as) > 0 {
		c.Armor = as[len(as)-1]
	}
}

// BiomeDrops lists the distinct items monsters of a biome can drop, which is
// what a fetch quest is allowed to ask for. Asking for something nothing
// nearby drops would be an errand that cannot be run.
func (t *Tables) BiomeDrops(biome string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range t.Monsters[biome] {
		for _, drop := range d.Loot {
			if seen[drop.Item] {
				continue
			}
			if _, ok := t.Items[drop.Item]; !ok {
				continue // never name an item the game does not have
			}
			seen[drop.Item] = true
			out = append(out, drop.Item)
		}
	}
	sort.Strings(out)
	return out
}

// BiomeMonsters lists a biome's creatures within reach of a level, so a cull
// quest names something the player will actually run into.
func (t *Tables) BiomeMonsters(biome string, level int) []*model.MonsterDef {
	var out []*model.MonsterDef
	for _, d := range t.Monsters[biome] {
		if core.Abs(d.Level-level) <= 3 {
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		out = append(out, t.Monsters[biome]...)
	}
	return out
}

// PickMonsters chooses a plausible encounter group for a biome at a level,
// preferring monsters near the target level and falling back to the whole
// biome roster when nothing matches.
func (t *Tables) PickMonsters(g *core.RNG, biome string, level, count int) []*model.Monster {
	pool := t.Monsters[biome]
	if len(pool) == 0 {
		pool = t.Monsters["plains"]
	}
	if len(pool) == 0 {
		for _, v := range t.Monsters {
			pool = v
			break
		}
	}
	if len(pool) == 0 {
		return nil
	}

	// Weight by closeness to the target level: a level-1 rat should stop
	// showing up once you are level 9, without ever being formally retired.
	weights := make([]int, len(pool))
	for i, d := range pool {
		diff := core.Abs(d.Level - level)
		w := 10 - diff*3
		if w < 1 {
			w = 1
		}
		weights[i] = w
	}

	out := make([]*model.Monster, 0, count)
	letters := map[string]int{}
	for i := 0; i < count; i++ {
		idx := g.Weighted(weights)
		if idx < 0 {
			idx = g.Intn(len(pool))
		}
		m := pool[idx].Spawn(g, level)
		letters[m.Def.ID]++
		out = append(out, m)
	}
	// Disambiguate duplicates in the target list: "Gutter Troll A / B".
	seen := map[string]int{}
	for _, m := range out {
		if letters[m.Def.ID] > 1 {
			m.Name = fmt.Sprintf("%s %c", m.Def.Name, 'A'+seen[m.Def.ID])
			seen[m.Def.ID]++
		}
	}
	return out
}
