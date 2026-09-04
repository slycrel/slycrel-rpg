// Package gamedata loads the JSON content tables off disk. Content lives in
// data/ as plain JSON rather than baked into the binary so that balance passes
// and new jokes are a text edit and a relaunch, not a recompile.
package gamedata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/saga"
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
	// Sidearms are the off-hand weapons, which are Weapons in every respect
	// except which arm they go on. A separate table rather than a flag on the
	// main one because the shop has to list them as their own shelf and the
	// thief has to be able to buy one without being asked which hand — a
	// parrying dagger is an off-arm item the way a shield is, not a sword that
	// happens to be small.
	Sidearms []model.Weapon
	Items    map[string]model.Item
	Spells   []model.Spell
	// Affixes are the suffixes a piece of gear can carry, and what they do.
	Affixes []model.Affix

	Text Text
	// Threads are the authored companion backstories, before anybody is cast
	// in one. They live beside the rest of the writing rather than in the
	// binary for the same reason everything else here does.
	Threads thread.Book
	// Sagas are the authored long stories: the spine that starts at the gate,
	// and the short arcs found out in the world.
	Sagas saga.Book
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
	// Advice is keyed by what is true about the run — see Game.adviceKey.
	Advice map[string][]string `json:"advice"`
	// LabelPlaceholder is what a kind of thing calls itself before you are
	// near enough to be told its name, keyed by world.EntityKind. A kind with
	// no entry shows its real name at any distance, which is what shops and
	// signs want: their name is how you navigate to them rather than a reward
	// for arriving.
	LabelPlaceholder map[string]string `json:"labelPlaceholder"`
	SignText         []string          `json:"signText"`

	// Hirelings: the sales pitch, the handshake, the parting, and what happens
	// when one of them stops being upright.
	RecruitPitch []string `json:"recruitPitch"`
	RecruitJoin  []string `json:"recruitJoin"`
	RecruitLeave []string `json:"recruitLeave"`
	AllyDown     []string `json:"allyDown"`
	AllyUp       []string `json:"allyUp"`
	// What a companion says about equipment handed to them, in three banks
	// because there are three things it can be: the thing they were saving
	// for, something better than they had, and something that is neither.
	GiftWanted []string `json:"giftWanted"`
	GiftBetter []string `json:"giftBetter"`
	GiftPlain  []string `json:"giftPlain"`
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

	// OddityVoice is the joke zone's own register, keyed by what is speaking:
	// a sign, a resident, a machine, a bin. It is a separate bank rather than
	// more SignText because the whole point of an oddity is that it does not
	// sound like the rest of the continent — and because nothing there may ever
	// be in on it, which is a rule about tone that a shared bank cannot hold.
	OddityVoice map[string][]string `json:"oddityVoice"`

	// Quest lines, keyed by quest kind then by part (ask / nag / thank).
	Quest map[string]map[string][]string `json:"quest"`

	// Where an errand sends you, keyed by biome. Each is a phrase naming the
	// country around a settlement — "the woods outside {P}" — because a fetch
	// or a cull happens in a region rather than at an address, and a quest that
	// cannot name an address was previously naming nothing at all.
	QuestWhere map[string][]string `json:"questWhere"`
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
	if err := readJSON(filepath.Join(dd, "items", "sidearms.json"), &t.Sidearms); err != nil {
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
	if err := readJSON(filepath.Join(dd, "text", "sagas.json"), &t.Sagas); err != nil {
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

// StarterKit is the cheapest real weapon and coat a class can actually use.
//
// Two rules, and both were paid for. Tier zero is skipped, because the
// tier-zero rows exist to give the *absence* of equipment a name — "Bare Hands"
// at strike 1, "Regrettable Rags" at defence 0 — and picking the cheapest thing
// in the table handed every new character exactly that. So the game opened by
// issuing nothing and calling it a loadout.
//
// And the class gate is honoured, because the moment weapons had lanes the
// cheapest row in the file became a table leg no Mage may pick up and a robe no
// Fighter would be issued. A starting kit somebody cannot equip is worse than
// no kit: it is a sheet showing gear next to a pack that refuses to take it
// off. The cheap end of each lane exists so that this can be a real answer for
// everybody — a Mage opens holding a humming stick and shops up to a switch,
// which is the same first morning a Fighter has with a table leg and a mace.
func (t *Tables) StarterKit(class model.Class) (model.Weapon, model.Armor) {
	// The cheap end of the lane they will be shopping in, which is a stronger
	// rule than "cheapest thing they are allowed". A Fighter is allowed to wear
	// a robe — nothing stops anybody wearing cloth — so "cheapest allowed" put
	// every class in the same eight-coin robe and the armour lanes vanished
	// from the one screen where the player meets them. Reading the lane off
	// what Equip buys at tier one means the starting kit and the first upgrade
	// are the same sentence, which is what a starting kit is for.
	curve := &model.Character{Level: 1, Class: class}
	t.EquipAs(curve, Archetypes[0])

	w := model.Weapon{Name: "Bare Hands", Strike: 1, Verb: "slap"}
	for _, c := range t.Weapons {
		if c.Tier < 1 || c.Kind != curve.Weapon.Kind || c.TwoHanded() != curve.Weapon.TwoHanded() {
			continue
		}
		if w.Name == "Bare Hands" || c.Cost < w.Cost {
			w = c
		}
	}
	a := model.Armor{Name: "Rags", Verb: "flaps"}
	for _, c := range t.Armors {
		if c.Tier < 1 || c.Kind != curve.Armor.Kind {
			continue
		}
		if a.Name == "Rags" || c.Cost < a.Cost {
			a = c
		}
	}
	return w, a
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

// StockForClass narrows a shelf to what one character could actually leave with.
//
// The shop still lists everything — a row nobody can take is greyed rather than
// hidden, because a mage should be able to see that plate exists and that it is
// not for them. This is for the callers who need the answer rather than the
// question: what to dress a simulated subject in, and what a chest should
// bother leaving on the floor.
func (t *Tables) StockForClass(tier int, class model.Class) ([]model.Weapon, []model.Armor) {
	ws, as := t.StockFor(tier)
	ws = slices.DeleteFunc(ws, func(w model.Weapon) bool { return !model.CanWield(class, w) })
	as = slices.DeleteFunc(as, func(a model.Armor) bool { return !model.CanWear(class, a) })
	return ws, as
}

// bestWeapon picks what a class would buy off a shelf.
//
// A mage reads the same shelf differently from everybody else, and that is the
// whole point of the focus slot: the number they are shopping for is Focus, and
// the strike on a rod is close enough to nothing that ranking by it would send
// them home with a dagger. One comparison with two orderings beats a second
// stock function that means "the caster's shelf".
func bestWeapon(ws []model.Weapon, class model.Class, hands int) (model.Weapon, bool) {
	var best model.Weapon
	found := false
	for _, w := range ws {
		switch {
		case hands == 1 && w.TwoHanded():
			continue
		case hands == 2 && !w.TwoHanded():
			continue
		}
		if !found || betterWeapon(w, best, class) {
			best, found = w, true
		}
	}
	return best, found
}

func betterWeapon(a, b model.Weapon, class model.Class) bool {
	if class == model.ClassMage && (a.Focus > 0 || b.Focus > 0) {
		if a.Focus != b.Focus {
			return a.Focus > b.Focus
		}
	}
	return a.Strike > b.Strike
}

// pickSidearm chooses what goes on the off arm: the lane the build asked for at
// the best band that stocks one, and the plainest thing the class can hold when
// the shelf has nothing in that lane.
//
// The fallback is the load-bearing half. A build asking for a silvered shield in
// a band that has none has to take the wall rather than nothing, for the same
// reason a Mage asked for a two-hander takes a rod: an archetype that arrives
// undressed measures the spec rather than the content, which this section has
// now got wrong twice in two different slots.
func pickSidearm(ss []model.Shield, class model.Class, want model.SidearmLane) model.Shield {
	pick := func(match func(model.Shield) bool) (model.Shield, bool) {
		var best model.Shield
		found := false
		for _, sh := range ss {
			if !model.CanHoldShield(class, sh) || !match(sh) {
				continue
			}
			if !found || sh.Tier > best.Tier ||
				(sh.Tier == best.Tier && betterSidearm(sh, best)) {
				best, found = sh, true
			}
		}
		return best, found
	}
	if s, ok := pick(func(sh model.Shield) bool { return sh.Lane() == want }); ok {
		return s
	}
	s, _ := pick(func(model.Shield) bool { return true })
	return s
}

// betterSidearm compares two off-arm items of the same band in their own unit.
func betterSidearm(a, b model.Shield) bool {
	if a.Barrier() {
		return a.Absorb > b.Absorb
	}
	return a.Defense > b.Defense
}

func bestArmor(as []model.Armor) (model.Armor, bool) {
	var best model.Armor
	found := false
	for _, a := range as {
		if !found || a.Defense > best.Defense {
			best, found = a, true
		}
	}
	return best, found
}

// SidearmsFor returns the shields and charms a shop of the given tier carries.
// OffHandFor is the off-hand weapons a shop of the given tier carries, narrowed
// to the ones this class may actually hold.
//
// Narrowed rather than listed-and-greyed, unlike the main shelves, because the
// answer is the same for every member of a class at every tier: two of the
// three can never hold one at all, and a shelf that is empty for them is a
// shelf that should not be drawn.
func (t *Tables) OffHandFor(tier int, class model.Class) []model.Weapon {
	var out []model.Weapon
	for _, w := range t.Sidearms {
		if w.Tier <= tier && model.CanHoldSidearm(class, w) {
			out = append(out, w)
		}
	}
	return out
}

// BestSidearm is the heaviest off-hand weapon on a shelf, which for this table
// is simply the dearest — they are one lane, so there is nothing to weigh.
func BestSidearm(ws []model.Weapon) (model.Weapon, bool) {
	var best model.Weapon
	found := false
	for _, w := range ws {
		if !found || w.Tier > best.Tier || (w.Tier == best.Tier && w.Strike > best.Strike) {
			best, found = w, true
		}
	}
	return best, found
}

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

	// Arm is which lane of off-arm item the build reaches for. It exists
	// because the sidearm slot stopped being a single ladder: a wall, a spiked
	// one and a silvered one sit in every band, and an archetype that always
	// took the wall would leave two thirds of the shelf unmeasured — which is
	// exactly the state the report was in when it said the two-hander beats
	// the shield everywhere. It beats *one* of the shields.
	Arm model.SidearmLane

	// OffHand puts a second weapon on the arm instead of a plank, for the one
	// class that may. False is a plank, which is what every build measured
	// before this existed took, so the zero value keeps every earlier number
	// meaning what it meant.
	OffHand bool

	// Hands is how the weapon arm is spent: 1 leaves the off arm free for a
	// shield, 2 commits both, 0 takes whatever is best in the band.
	//
	// It exists because the biggest numbers in the weapon table now need both
	// hands, so "best weapon of your tier" and "a weapon and a shield" stopped
	// being the same sentence. That is the trade the duelist was always meant
	// to be measuring and could not, back when nothing in the tables let a
	// build spend defence on offence.
	Hands int
}

// ArmByLevel is a lane that is not a lane. It means "whichever of the three is
// right at this level", and it is what the balanced build reaches for.
//
// It lives here rather than in model.SidearmLane because it is a balance
// decision rather than a property of an item: a shield still sells exactly one
// thing, and nothing on a shelf is ever "by level". Negative so it can never
// collide with a real lane however many get added.
const ArmByLevel = model.SidearmLane(-1)

// strikeFromLevel is where the wall stops being worth carrying, and it is
// measured rather than chosen — see the LANES section of cmd/balance, which
// exists to keep this number honest and which had to be rebuilt before it
// could.
//
// It was 6 and it was the ward lane, on the strength of a table that averaged
// three classes into one row and called a gap of one point a result. Neither
// half survived being measured properly. LANES now prints, in the same table,
// twenty rows where all three lanes dress the character identically — a
// Mage, who cannot hold a plank at any level; anybody at all below the level
// the balanced build affords an off arm — and the spread across those columns
// is sampling wobble by construction. It spreads by up to 4.3 points. The
// threshold that gated this constant was 1.0, so the old crossover was read
// off noise.
//
// Measured against a floor taken from those rows, per class, on two axes: the
// wall goes behind somewhere between level eight and level ten depending on the
// class and the axis, and eight costs either class nothing in between — the
// spiked lane is already ahead of the wall there, just not yet by more than the
// floor. So the constant does not want a class parameter, which was the open
// question and is this line's answer.
//
// It has been re-pinned twice in one sitting and neither time was a content
// change. Fixing freeSwingWorth — the policy had been comparing a paid
// technique against a bad guess at a swing, so a Fighter cast when it should
// have hit — moved the crossover two levels on its own, and two further fixes
// to the same policy moved it again. Which is the argument for the whole
// apparatus rather than against it: a constant nobody re-derives is a constant
// that is right about the game it was measured in, and this one is checked
// against the fights on every report run rather than trusted.
//
// The two lanes do not cost the same, and the earlier claim here that they came
// "within five per cent in every band" was wrong in four bands of five — the
// wall and the silvered shield run 8/22, 48/52, 110/125, 260/280, 620/650, so
// the tier-one pair differ by 175%. What makes the comparison honest is that a
// sidearm is a small part of a kit, and LANES now prints that as a number
// rather than asserting it: the widest gap between the three lanes' whole kits
// is 1.5% from level seven up, against lane differences reaching fourteen.
const strikeFromLevel = 8

// StrikeFromLevel is the crossover, for the report that has to print it beside
// the numbers that justify it.
func StrikeFromLevel() int { return strikeFromLevel }

// LaneForLevel is which off-arm lane a sensible person carries at this level.
//
// Two lanes, not three, and the ward lane is not one of them — which reverses
// what this function used to say and is worth stating plainly, because the
// reasoning that put the silvered shield here was sound and the premise under
// it was false.
//
// The premise was that the spiked lane trades guard for strike: an offensive
// choice, properly made by the duelist, and a baseline that made it would have
// an opinion about offence, which is the one thing a baseline must not have.
// That argument still holds wherever the premise does. At the top of the game
// the premise does not hold. Averaged over levels twelve to fourteen, a Fighter
// carrying the spiked shield into fights five levels over its head wins 60.2%
// of them and dies in 23.3%; the same Fighter carrying the wall wins 46.9% and
// dies in 32.9%, and carrying the silvered shield wins 54.9% and dies in 33.4%.
// The spiked lane is not trading anything there. It is the best defensive item
// on the arm as well as the best offensive one, and it gets there by killing
// rather than by escaping — it flees *less* than the wall does and still dies
// nine points less often. A design position whose premise has been measured
// away is not a position.
//
// **Those figures are from a particular evening's tree and that matters.** The
// first draft of this comment quoted 54.5 / 37.0 / 45.0 and death rates eight
// to eleven points lower, and every one of them was true when it was written —
// then three fixes to internal/rules landed the same night (the retreat policy
// misreading magical damage, the second swing re-entering the technique
// chooser, and the same policy clamping a mean where it wanted the mean of a
// clamp) and the whole table moved. The decision survived all three and the
// crossover has been re-pinned twice. The numbers did not survive, and a
// comment that quotes measurements without saying which game they were taken
// in will be quietly wrong the next time the rules move. If these disagree with
// a fresh report run, believe the run.
//
// The silvered shield is the casualty. It is now the best lane in one cell of
// the twenty-eight LANES measures, which is what a coin looks like, and it is
// the worst of the three on the death rate at the top of the game — fewest
// escapes, most deaths, because it pays three points of guard for fifteen of
// ward and the WARD section prices the whole ward slot at nought to three
// points. That is a content problem rather than a constant problem: a band of
// three where one is never the answer has stopped being a choice, exactly as
// the charm bands have. It is written down and not fixed here.
func LaneForLevel(level int) model.SidearmLane {
	if level >= strikeFromLevel {
		return model.ArmStrike
	}
	return model.ArmBlock
}

// Archetypes are the builds the balance report measures.
//
// Balanced is first on purpose: every other number in the report is measured
// against it, and a hireling is still dressed by it. Nothing here is a claim
// that the other two are viable — that is the question the report exists to
// answer.
//
// There was a fourth, "warden", which was balanced with the silvered shield
// instead of the wall. It has been retired because it won: at identical spend
// it beat balanced at every level from seven upward, by up to 11.2 points, and
// the right response to an archetype that dominates the baseline for free is
// not to keep it in the table, it is to stop the baseline making that mistake.
// (Most of that 11.2 was noise — the section that produced it called a gap of
// one point a result, and it took a per-class rebuild of LANES to find out. The
// warden's retirement was still the right call: it just was not beating the
// baseline for the reason it was credited with, and the lane it was retired in
// favour of turned out to be the wrong one too.)
// Balanced now takes the lane the level calls for, and the measurement that
// says which lane that is has its own section — LANES in cmd/balance — because
// a crossover that lives in a retired archetype is a number nobody can check.
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
		Hands:  1,
		// The off arm follows the level rather than sitting on the wall
		// forever. This used to be the zero value, which is ArmBlock, which
		// meant the one assumption the whole report is measured against — and
		// every hireling in the game, since Equip is what dresses them — kept
		// the plainest plank in the band into the levels where it is the worst
		// of the three, at identical spend. Which lane it follows to is
		// LaneForLevel's business and has changed once already.
		Arm: ArmByLevel,
	},
	{
		Name: "attrition",
		Note: "armour and off arm of your tier, weapon a band behind",
		// The wall. Fights take longer and are meant to; what this build buys
		// is the ability to still be standing at the end of one.
		Weapon: Slot{Back: 1},
		Charm:  Slot{Back: 1},
		Hands:  1,
		// The right arm for the level, like the baseline. This was the zero
		// value — ArmBlock, the plain shield — in the one build whose entire
		// identity is the off arm, at exactly the levels LANES says the plain
		// shield is the worst of the three. "The wall" is a description of a
		// playstyle, not a claim about which lane: a build that spends most on
		// the arm should spend it on the arm that is worth something.
		Arm: ArmByLevel,
	},
	{
		Name: "duelist",
		Note: "both hands on the weapon, so nothing on the off arm",
		// Not a glass cannon, though that is what it was drafted as. A glass
		// cannon trades defence for damage, and until weapons had lanes there
		// was nothing in the tables to trade *for*: no item outside the weapon
		// slot adds strike, so the most offensive build available was "best
		// weapon", which every build already had. That absence was a finding,
		// and the two-handed lane is the answer to it — this build is now the
		// trade it was always named for.
		//
		// The off arm is closed by the weapon rather than by fiat, which EquipAs
		// reads off the hand count. That matters since casters got a slot: a
		// Mage cannot hold a two-hander, so telling them to drop the off arm as
		// well made "duelist" mean "balanced, minus a talisman" for one class in
		// three — not a different build, a worse one, and the report averaged it
		// in and reported the duelist as winning nothing at all.
		//
		// The charm still comes a band behind, exactly as balanced's does. Best
		// in tier there as well would make this "balanced, plus a better charm,
		// plus a two-hander", an archetype that *overspends* — which measures
		// the spec just as surely as one that underspends, and did: it went to
		// winning seven levels out of seven. The trade this build exists to
		// measure is the two-hander against the arm it closes. Nothing else may
		// move.
		Charm: Slot{Back: 1},
		Hands: 2,
		// And the right arm for the level on the classes that cannot make this
		// build's trade at all. Only a Fighter may hold a two-hander, so a
		// Thief "duelist" is a one-hander with a free arm — and this field was
		// unset, which is ArmBlock, which is the wall, which LANES says is the
		// worst lane from level ten. Fourth instance of the same defect, found
		// by a reviewer inside the very row the equal-purse conclusion was read
		// off. A build that cannot close its arm should at least fill it well.
		Arm: ArmByLevel,
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

	// The hand count is a preference rather than a rule: a Mage asked for a
	// two-handed weapon owns none, and handing them nothing would measure the
	// spec rather than the content — which is the mistake the duelist's first
	// draft already made once, in a different slot.
	if tier := core.Max(1, a.Weapon.tierAt(base)); tier >= 1 {
		ws, _ := t.StockForClass(tier, c.Class)
		if w, ok := bestWeapon(ws, c.Class, a.Hands); ok {
			c.Weapon = w
		} else if w, ok := bestWeapon(ws, c.Class, 0); ok {
			c.Weapon = w
		}
	}
	if tier := core.Max(1, a.Armor.tierAt(base)); tier >= 1 {
		_, as := t.StockForClass(tier, c.Class)
		if ar, ok := bestArmor(as); ok {
			c.Armor = ar
		}
	}
	// The off arm needs a free hand and something the class may put on it —
	// a plank for two of them, a talisman for the third, or a second weapon
	// for the one that duels. One thing, whichever it is: putting a dagger on
	// the arm puts the shield down, which is the trade the slot exists to make
	// and the invariant TestTheOffArmHoldsOneThing holds.
	c.Shield = model.Shield{}
	c.Sidearm = model.Weapon{}
	if tier := a.Shield.tierAt(base); tier >= 1 && c.CanHold() {
		if a.OffHand {
			if w, ok := BestSidearm(t.OffHandFor(tier, c.Class)); ok {
				c.Sidearm = w
			}
		}
		if !c.Sidearm.Worn() {
			ss, _ := t.SidearmsFor(tier)
			arm := a.Arm
			if arm == ArmByLevel {
				arm = LaneForLevel(c.Level)
			}
			c.Shield = pickSidearm(ss, c.Class, arm)
		}
	}
	c.Charm = model.Charm{}
	if tier := a.Charm.tierAt(base); tier >= 1 {
		if _, cs := t.SidearmsFor(tier); len(cs) > 0 {
			c.Charm = BestCharm(cs)
		}
	}
}

// Charm weights: what a point in each column of a Bonus is worth to somebody
// deciding which charm to wear.
//
// These exist because the charm slot was picking `cs[len(cs)-1]` — the last
// row of the file — which is the off-arm bug again in the one slot the design
// says is *deliberately* unrankable. Every charm gives with one hand and takes
// with the other, so there is no better one, so any pick is as good as any
// other: that was the reasoning, and the arbiter disagrees with its premise.
// Measured on the stretch fights and on fights-per-rest, one charm wins its
// band on both axes for essentially every class, in three bands out of four —
// and the file order landed on the loser in three bands out of four. It cost a
// Thief at level eleven 12.5 points of win rate and a third of its endurance.
//
// The numbers are read off that measurement rather than reasoned out, and the
// CHARMS section of cmd/balance re-derives the ordering on every run and
// complains when this ranking and the fights disagree.
//
// What that check can and cannot do is worth being exact about, because the
// seven decimals here promise more than it delivers. It is five argmax
// comparisons against seven continuous weights, produced by the same simulator
// that fitted them, so it guards against content drift flipping a pick and
// cannot falsify a weight. charmPsyche at 0.2 and at 0.0 choose identically in
// every band. The top band is never measured at all — reportCharms reaches
// charm tier four and players shop tier five. And the ordering is one figure
// across three classes: the Mage measurably prefers The Quiet Stone at level
// eleven and loses 1.5 points to this ranking. What the measurement actually
// pins is "ward beats the combat stats beats psyche", not the decimals.
//
// Two of the weights are worth stating:
//
//   - Psyche is worth almost nothing here, and that is not an oversight about
//     endurance. Psyche is the currency of the *next* fight, so the obvious
//     objection is that a single-fight measure cannot see it — but the
//     endurance column says the same thing, for every class including the
//     Mage. Four points of pool does not buy a fight; six points of ward does.
//   - Strength beats dexterity slightly, which is the whole of why the
//     tier-one band has a winner at all: the two charms there are mirror
//     images of each other.
//
// **These are measurements now, not a fit.** They used to be read off five
// argmax comparisons — which charm the fights preferred in each band — and the
// comment below them was honest that seven continuous weights cannot be
// recovered from five discrete choices. EXCHANGE measures the thing directly:
// it nudges the balanced build a few points either way in one stat and reads
// what that buys on both of LANES' bands. These are its all-class, both-axis
// means over levels five, nine and thirteen, rounded, and re-deriving them is a
// report run rather than an argument.
//
// The hand-fitted weights had ward at 1.0 against a measured 0.15, dexterity at
// 1.0 against 0.19 and guard at 1.5 against 0.81 — which is why the balanced
// build kept reaching for the ward charm in every band that had one.
//
// They have been re-derived once already, and the reason is worth keeping,
// because the first measurement was wrong in a way that looked right. EXCHANGE
// began as a one-sided difference: it added K points and never removed any.
// These curves saturate — a Fighter at the top of the game wins 83% of the
// stretch fights — so adding six strike bought about 1.0 a point while removing
// six cost between 1.5 and 3.7 a point, and the rate depended on which
// direction the content happened to move. It is a central difference now, and
// the weights moved by up to a factor of two when it changed: guard from 0.53
// to 0.81, speed from 0.29 to 0.55.
//
// Which is also the standing caveat. A derivative through the operating point
// prices a point; it cannot price a swap of eleven, and LANES remains the
// instrument for whole items. When the two disagree, LANES is measuring the
// thing and this is measuring the neighbourhood of it.
//
// Still class-blind, and that is now a known cost rather than an oversight: a
// point of psyche is worth 0.68 to a Mage at level thirteen and -0.01 to a
// Fighter, so this single number under-serves the caster and over-serves the
// two who never spend it. It is the same shape as LaneForLevel's question and
// it wants the same answer — a measurement per class before a parameter — which
// EXCHANGE can now supply and this has not spent yet.
const (
	charmStrike = 0.98
	charmGuard  = 0.81
	charmStr    = 0.68
	charmSpeed  = 0.55
	charmPsyche = 0.23
	charmDex    = 0.19
	charmWard   = 0.15
)

// CharmValue scores what a charm's trade is worth, positive and negative
// columns together.
//
// It ranks charms, which the shop counter deliberately refuses to do — see
// TestTheShelfNeverGradesACharm. That is not a contradiction yet: the counter
// refuses because the *content* is supposed to make them incomparable, and
// this exists because the balanced build has to put something in the slot and
// picking by file order is not a decision. If the content is ever made to
// trade properly, the spread this returns across a band collapses toward zero
// and the pick stops mattering, which is the outcome to want.
func CharmValue(c model.Charm) float64 {
	b := c.Bonus
	if c.Affix != nil {
		b = b.Add(c.Affix.Bonus)
	}
	return charmWard*float64(b.Ward) +
		charmStrike*float64(b.Strike) +
		charmGuard*float64(b.Defense) +
		charmStr*float64(b.Strength) +
		charmDex*float64(b.Dexterity) +
		charmSpeed*float64(b.Speed) +
		charmPsyche*float64(b.Psyche)
}

// BestCharm is the one a sensible person would wear out of the shop, highest
// tier first so a band behind is still a band behind.
//
// Exported for the report, which has to name the same charm the game does. It
// was computing the charm slot's exchange rate off `cs[len(cs)-1].Bonus.Defense`
// — the last row of the file, on the one axis charms mostly do not carry — so
// the WHY table's charm column read 0, 0, 1, 2 and meant nothing at all. The
// arbiter had the same bug as the thing it was arbitrating.
func BestCharm(cs []model.Charm) model.Charm {
	var best model.Charm
	found := false
	for _, c := range cs {
		switch {
		case !found, c.Tier > best.Tier,
			c.Tier == best.Tier && CharmValue(c) > CharmValue(best):
			best, found = c, true
		}
	}
	return best
}

// GearCost totals what a character is wearing, which is what makes an archetype
// a trade rather than a preference.
//
// Every slot, and the off arm counts whichever thing is on it. The fifth slot
// was added without this being told, which had two consequences and neither
// announced itself: LANES printed a cost column claiming to be "what the swap
// does to the whole kit" while the dagger entered the kit for free — the real
// deltas were +12/0/+3 against a printed -8/-55/-275 — and EquipWithin, which
// fits a build to the purse balanced spends, could not see the dagger at all.
// The second is the worse one. It is exactly the "measuring a budget rather
// than a build" confound the spend column exists to prevent, lying dormant
// until an archetype set OffHand.
func GearCost(c *model.Character) int {
	n := c.Weapon.Cost + c.Armor.Cost
	if c.Shield.Worn() {
		n += c.Shield.Cost
	}
	if c.Sidearm.Worn() {
		n += c.Sidearm.Cost
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
//
// It is the flat version: count creatures, all at the level asked for. The
// controls in cmd/balance use it, because a measurement wants one variable, and
// so does anything that needs a creature rather than a fight — a quest naming
// something to go and kill. What the game actually throws at a player comes out
// of PickEncounter below, which is this with a shape on it.
func (t *Tables) PickMonsters(g *core.RNG, biome string, level, count int) []*model.Monster {
	pool := t.poolFor(biome, level)
	if len(pool) == 0 {
		return nil
	}
	return nameGroup(drawFrom(g, pool, level, count, nil))
}

// poolFor is the roster a biome can supply at a level, capped and floored.
func (t *Tables) poolFor(biome string, level int) []*model.MonsterDef {
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
		return capped
	}
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
	return lowest
}

// drawFrom spawns count creatures out of a pool, weighted towards the level
// asked for. bias, when given, multiplies a definition's weight — which is how
// a shape says "something with plating" or "something fast" without needing a
// second roster.
func drawFrom(g *core.RNG, pool []*model.MonsterDef, level, count int,
	bias func(*model.MonsterDef) int) []*model.Monster {
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
		if bias != nil {
			w *= core.Max(1, bias(d))
		}
		weights[i] = core.Max(1, w)
	}

	out := make([]*model.Monster, 0, count)
	for i := 0; i < count; i++ {
		idx := g.Weighted(weights)
		if idx < 0 {
			idx = g.Intn(len(pool))
		}
		out = append(out, pool[idx].Spawn(g, level))
	}
	return out
}

// nameGroup disambiguates the target list, and what needs disambiguating
// changed when creatures started stacking.
//
// It used to letter every duplicate: three wolves were Wolf A, Wolf B and Wolf
// C. A letter is a label on a *choice*, and identical creatures now share one
// slot with a count on it, so there is no longer a choice between them to label
// — "Wolf A" on a plate reading "Wolf x3" invites the player to work out which
// of the three the transcript means, and the answer is that it does not matter,
// which is a question the interface should not have asked.
//
// So the letter goes on the *kind*, not the body. Two ordinary wolves and one
// scaled-up wolf are Wolf A twice and Wolf B once — one letter per stack, and
// only when a name would otherwise appear twice on the field. A swarm of six
// identical wolves is six Wolves and one slot, with nothing to tell apart.
func nameGroup(out []*model.Monster) []*model.Monster {
	// kinds[def] is one representative per distinct kind sharing that name.
	kinds := map[string][]*model.Monster{}
	for _, m := range out {
		id := m.Def.ID
		found := false
		for _, k := range kinds[id] {
			if model.SameKind(k, m) {
				found = true
				break
			}
		}
		if !found {
			kinds[id] = append(kinds[id], m)
		}
	}
	for _, m := range out {
		ks := kinds[m.Def.ID]
		if len(ks) < 2 {
			continue
		}
		for i, k := range ks {
			if model.SameKind(k, m) {
				m.Name = fmt.Sprintf("%s %c", m.Def.Name, 'A'+i)
				break
			}
		}
	}
	return out
}
