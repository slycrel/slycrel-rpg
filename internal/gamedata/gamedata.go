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
	"github.com/slycrel/slycrel-rpg/internal/thread"
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
	Shields []model.Shield
	Charms  []model.Charm
	Items   map[string]model.Item
	Spells  []model.Spell
	// Affixes are the suffixes a piece of gear can carry, and what they do.
	Affixes []model.Affix

	Text Text
	// Threads are the authored companion backstories, before anybody is cast
	// in one. They live beside the rest of the writing rather than in the
	// binary for the same reason everything else here does.
	Threads thread.Book
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
	// Afflicted is what a condition landing reads like, keyed by the
	// model.EffectKind string.
	Afflicted map[string][]string `json:"afflicted"`

	// StandingLine is what a townsperson opens with, keyed by how they read
	// you — see rules.Standing. Absent for "nobody", who gets the ordinary
	// NpcLine, because having no reputation is not a reaction.
	StandingLine map[string][]string `json:"standingLine"`

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
	if err := readJSON(filepath.Join(dd, "items", "shields.json"), &t.Shields); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "items", "charms.json"), &t.Charms); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "items", "affixes.json"), &t.Affixes); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "items", "spells.json"), &t.Spells); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "text", "flavor.json"), &t.Text); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dd, "text", "threads.json"), &t.Threads); err != nil {
		return nil, err
	}

	sort.Slice(t.Weapons, func(i, j int) bool { return t.Weapons[i].Cost < t.Weapons[j].Cost })
	sort.Slice(t.Armors, func(i, j int) bool { return t.Armors[i].Cost < t.Armors[j].Cost })
	sort.Slice(t.Shields, func(i, j int) bool { return t.Shields[i].Cost < t.Shields[j].Cost })
	sort.Slice(t.Charms, func(i, j int) bool { return t.Charms[i].Cost < t.Charms[j].Cost })
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

// StarterWeapon returns the cheapest real weapon, used to arm a new character.
//
// Tier zero is skipped, and that is the whole point of this function. The
// tier-zero rows exist to give the *absence* of equipment a name — "Bare Hands"
// at strike 1, "Regrettable Rags" at defence 0 — and picking the cheapest thing
// in the table handed every new character exactly that. So the game opened by
// issuing nothing and calling it a loadout: three points of damage a swing, no
// armour at all, and 15 to 40 coins against the 66 it costs to reach what every
// section of the balance report calls being on curve.
func (t *Tables) StarterWeapon() model.Weapon {
	for _, w := range t.Weapons {
		if w.Tier >= 1 {
			return w
		}
	}
	return model.Weapon{Name: "Bare Hands", Strike: 1, Verb: "slap"}
}

// StarterArmor returns the cheapest real armour. Tier zero is skipped for the
// same reason as StarterWeapon.
func (t *Tables) StarterArmor() model.Armor {
	for _, a := range t.Armors {
		if a.Tier >= 1 {
			return a
		}
	}
	return model.Armor{Name: "Rags", Verb: "flaps"}
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

// SidearmsFor returns the shields and charms a shop of the given tier carries.
func (t *Tables) SidearmsFor(tier int) ([]model.Shield, []model.Charm) {
	var ss []model.Shield
	for _, s := range t.Shields {
		if s.Tier <= tier {
			ss = append(ss, s)
		}
	}
	var cs []model.Charm
	for _, c := range t.Charms {
		if c.Tier <= tier {
			cs = append(cs, c)
		}
	}
	return ss, cs
}

// PickAffix chooses a suffix appropriate to a gear band, or reports false when
// the band has none. Affixes are authored with their own tier so that a level
// two hand-axe cannot turn up "of Consequences".
func (t *Tables) PickAffix(g *core.RNG, tier int) (model.Affix, bool) {
	var pool []model.Affix
	for _, a := range t.Affixes {
		if a.Tier <= tier {
			pool = append(pool, a)
		}
	}
	if len(pool) == 0 {
		return model.Affix{}, false
	}
	return core.Pick(g, pool), true
}

// GearTierFor is the gear band a character is expected to be carrying at a
// level. The shops stock by tier and tiers span roughly three levels each, so
// this is the "on curve" assumption: it is what the balance report measures
// against, and what a hireling turns up already wearing.
func GearTierFor(level int) int { return core.Clamp(1+(level-1)/3, 1, 5) }

// Slot is how one equipment slot is filled relative to the character's expected
// gear tier.
//
// Back is how many bands behind that tier the slot buys, which is the only
// currency an archetype has: everything is paid for out of the same purse, so
// a slot bought at full tier is a slot elsewhere bought a band down. A slot
// whose effective tier falls below one is left empty, which is what keeps a
// level-one character from turning up with a barrel lid they could not afford.
type Slot struct {
	Back int
	Skip bool // this build does not use the slot at all
}

func (s Slot) tierAt(base int) int {
	if s.Skip {
		return 0
	}
	return base - s.Back
}

// Archetype is one way of being correctly levelled.
//
// There used to be exactly one, written into Equip, and that made "on curve"
// and "the way we expect you to play" the same sentence. These are the same
// assumption made three times, so the balance report can ask whether each is
// playable rather than only whether the one is balanced.
//
// They are deliberately not costed against each other beyond the band offsets.
// A real accounting would need shop prices per slot per tier and a model of
// what a player has saved by level N, and the point of this pass is to find out
// whether the content supports more than one shape at all before any of that is
// worth building.
type Archetype struct {
	Name string
	Note string

	Weapon, Armor, Shield, Charm Slot
}

// Archetypes are the builds the balance report measures.
//
// Balanced is the original assumption, unchanged and first on purpose: every
// other number in the report is still measured against it, and a hireling is
// still dressed by it. Nothing here is a claim that the other two are viable —
// that is the question the report exists to answer.
var Archetypes = []Archetype{
	{
		Name: "balanced",
		Note: "best weapon and armour of your tier, sidearms a band behind",
		// Best-in-tier across all four slots is not a character anybody can
		// afford: the shield and the charm are what you buy with whatever the
		// sword and the coat left over, and a report assuming otherwise would
		// be measuring a richer player than exists and calling the game easier
		// than it is.
		Shield: Slot{Back: 1},
		Charm:  Slot{Back: 1},
	},
	{
		Name: "attrition",
		Note: "armour and shield of your tier, weapon a band behind",
		// The wall. Fights take longer and are meant to; what this build buys
		// is the ability to still be standing at the end of one.
		Weapon: Slot{Back: 1},
		Charm:  Slot{Back: 1},
	},
	{
		Name: "duelist",
		Note: "nothing on the off arm, everything else best in tier",
		// Not a glass cannon, though that is what it was drafted as. A glass
		// cannon trades defence for damage, and there is nothing in the game to
		// trade *for*: no item outside the weapon slot adds strike, so the most
		// offensive build the tables permit is "best weapon available", which
		// every build already has. That absence is itself a finding and it is
		// why this is named for what it does rather than what it was meant to.
		//
		// The first draft of this gave up an armour band *and* the shield to
		// buy one charm band, which is not a trade, and it lost by twenty-two
		// points because it was a worse character rather than a different one.
		// See the cost column: an archetype that underspends is measuring the
		// spec, not the content.
		Shield: Slot{Skip: true},
	},
}

// ArchetypeNamed finds a build by name.
func ArchetypeNamed(name string) (Archetype, bool) {
	for _, a := range Archetypes {
		if a.Name == name {
			return a, true
		}
	}
	return Archetype{}, false
}

// Equip fits a character with the best gear of their expected tier. Anyone
// arriving mid-game — a companion for hire, a simulated subject — is dressed
// through here, so there is one definition of what "level N and properly
// equipped" means rather than one per caller.
//
// It is the balanced archetype and always will be. Everything in the game that
// dresses somebody goes through this, and the moment it started meaning
// something else the balance report and the hiring board would be describing
// different games.
func (t *Tables) Equip(c *model.Character) { t.EquipAs(c, Archetypes[0]) }

// EquipAs fits a character out according to a named build.
//
// The two main slots floor at tier one and the two sidearms do not, which is
// the difference between owning no shield and owning no sword. A build that
// buys a band behind is buying a worse weapon, never no weapon — without the
// floor, attrition walked into levels one to three bare-handed and the report
// dutifully measured it losing, which reads as a finding about the build rather
// than the arithmetic error it was.
func (t *Tables) EquipAs(c *model.Character, a Archetype) {
	base := GearTierFor(c.Level)

	if tier := core.Max(1, a.Weapon.tierAt(base)); tier >= 1 {
		if ws, _ := t.StockFor(tier); len(ws) > 0 {
			c.Weapon = ws[len(ws)-1]
		}
	}
	if tier := core.Max(1, a.Armor.tierAt(base)); tier >= 1 {
		if _, as := t.StockFor(tier); len(as) > 0 {
			c.Armor = as[len(as)-1]
		}
	}
	if tier := a.Shield.tierAt(base); tier >= 1 {
		if ss, _ := t.SidearmsFor(tier); len(ss) > 0 {
			c.Shield = ss[len(ss)-1]
		}
	}
	if tier := a.Charm.tierAt(base); tier >= 1 {
		if _, cs := t.SidearmsFor(tier); len(cs) > 0 {
			c.Charm = cs[len(cs)-1]
		}
	}
}

// GearCost totals what a character is wearing, which is what makes an archetype
// a trade rather than a preference.
func GearCost(c *model.Character) int {
	n := c.Weapon.Cost + c.Armor.Cost
	if c.Shield.Worn() {
		n += c.Shield.Cost
	}
	if c.Charm.Worn() {
		n += c.Charm.Cost
	}
	return n
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

	// Nothing more than a band above what was asked for.
	//
	// model.Spawn scales a creature up to the encounter level but never down,
	// so a definition picked from above it arrives at its own full strength and
	// the encounter is harder than its level says. That was happening a lot:
	// between a fifth and a half of all rolls came out over-level, by as much
	// as five at the worst, which is a level-six creature meeting a level-one
	// character with all of its own numbers intact. A death there is not a
	// consequence of anything the player chose — they were where they were
	// supposed to be — and an encounter level that does not predict the fight
	// makes every other difficulty guarantee in the game unfalsifiable.
	//
	// Capping the pick rather than scaling the creature down keeps a monster
	// being itself: you meet the dragon when you are in dragon country, not as
	// a diminished dragon in the meadows. One band of overshoot is left in on
	// purpose, because a fight slightly above expectation is a good surprise.
	const overshoot = 1
	capped := pool[:0:0]
	for _, d := range pool {
		if d.Level <= level+overshoot {
			capped = append(capped, d)
		}
	}
	if len(capped) > 0 {
		pool = capped
	} else {
		// Asking below the bottom of a roster — a dungeon has nothing under
		// level three, and something has to be sent. The floor of the biome is
		// the least wrong answer; falling back to the whole roster would let a
		// request for level one draw the deepest thing in the place.
		floor := pool[0].Level
		for _, d := range pool {
			if d.Level < floor {
				floor = d.Level
			}
		}
		lowest := pool[:0:0]
		for _, d := range pool {
			if d.Level == floor {
				lowest = append(lowest, d)
			}
		}
		pool = lowest
	}

	// Weight by closeness to the target level: a level-1 rat should stop
	// showing up once you are level 9, without ever being formally retired.
	//
	// The first three bands keep their original 10 : 7 : 4 ratio, scaled up so
	// the tail has room underneath. Past that the weight decays instead of
	// clamping to a floor, which is the whole point of this shape: a flat floor
	// stops ranking, and once the target is more than three levels above the
	// entire roster *everything* sits on the floor and the pick goes uniform.
	// That is exactly the case at the top of the game — an encounter five
	// levels over a level-13 hero was picking a level-5 wolf as often as a
	// level-14 dragon, then scaling the wolf up thirteen levels into a creature
	// with a dragon's hit points and a wolf's everything else.
	weights := make([]int, len(pool))
	for i, d := range pool {
		diff := core.Abs(d.Level - level)
		w := (10 - diff*3) * 1000
		if w < 1000 {
			w = 4000 / (1 + (diff-2)*(diff-2))
		}
		weights[i] = core.Max(1, w)
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
