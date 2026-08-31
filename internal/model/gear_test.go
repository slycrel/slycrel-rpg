package model_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
)

func base() *model.Character {
	return &model.Character{
		Name: "Bosk", Level: 5, HP: 30, MaxHP: 30, Psyche: 6, MaxPsyche: 6,
		Strength: 10, Dexterity: 7, Speed: 8,
		Weapon: model.Weapon{Name: "Actual Sword", Strike: 6},
		Armor:  model.Armor{Name: "Studded Leather", Defense: 4},
	}
}

// With nothing worn beyond weapon and armour, the effective stats have to be
// exactly the base ones. Everything else in the system is a delta on this.
func TestBareCharacterReadsItsOwnStats(t *testing.T) {
	c := base()
	if c.Str() != 10 || c.Dex() != 7 || c.Spd() != 8 {
		t.Errorf("bare stats came out %d/%d/%d, want 10/7/8", c.Str(), c.Dex(), c.Spd())
	}
	if c.Strike() != 6 || c.Defense() != 4 {
		t.Errorf("bare gear came out strike %d defence %d, want 6 and 4", c.Strike(), c.Defense())
	}
	if !c.Gear().Empty() {
		t.Errorf("a character wearing nothing extra has a bonus of %+v", c.Gear())
	}
}

func TestEverySlotContributes(t *testing.T) {
	c := base()
	c.Weapon.Affix = &model.Affix{Suffix: "of Poor Decisions", Bonus: model.Bonus{Strike: 3, Dexterity: -2}}
	c.Armor.Affix = &model.Affix{Suffix: "of the Damp", Bonus: model.Bonus{Defense: 2, Strength: -1}}
	c.Shield = model.Shield{Name: "Buckler", Defense: 2, Extra: &model.Bonus{Speed: -1}}
	c.Charm = model.Charm{Name: "Lucky Tooth", Bonus: model.Bonus{Dexterity: 1, Strength: -1}}

	// strength 10 -1 (damp) -1 (tooth) = 8
	if got := c.Str(); got != 8 {
		t.Errorf("strength came out %d, want 8", got)
	}
	// dexterity 7 -2 (decisions) +1 (tooth) = 6
	if got := c.Dex(); got != 6 {
		t.Errorf("dexterity came out %d, want 6", got)
	}
	// speed 8 -1 (buckler) = 7
	if got := c.Spd(); got != 7 {
		t.Errorf("speed came out %d, want 7", got)
	}
	// strike 6 +3 = 9
	if got := c.Strike(); got != 9 {
		t.Errorf("strike came out %d, want 9", got)
	}
	// defence 4 +2 (damp) +2 (buckler) = 8
	if got := c.Defense(); got != 8 {
		t.Errorf("defence came out %d, want 8", got)
	}
}

// A bad enough pile of affixes must never produce a character who cannot act.
// Stats floor at one and the two gear ratings at zero.
func TestRuinousGearStillLeavesAWorkingCharacter(t *testing.T) {
	c := base()
	awful := &model.Affix{Suffix: "of Total Collapse", Bonus: model.Bonus{
		Strike: -99, Defense: -99, Strength: -99, Dexterity: -99, Speed: -99,
	}}
	c.Weapon.Affix = awful
	c.Armor.Affix = awful
	c.Shield = model.Shield{Name: "Cursed Lid", Defense: -99}
	c.Charm = model.Charm{Name: "Bad Idea", Bonus: model.Bonus{Strength: -99}}

	if c.Str() < 1 || c.Dex() < 1 || c.Spd() < 1 {
		t.Errorf("ruinous gear produced stats %d/%d/%d", c.Str(), c.Dex(), c.Spd())
	}
	if c.Strike() < 0 || c.Defense() < 0 {
		t.Errorf("ruinous gear produced strike %d and defence %d", c.Strike(), c.Defense())
	}
}

// Worn psyche raises what a technique is worth without granting more casts, so
// the pool and its ceiling are deliberately different numbers.
func TestWornPsycheRaisesThePowerNotThePool(t *testing.T) {
	c := base()
	c.Charm = model.Charm{Name: "The Quiet Stone", Bonus: model.Bonus{Psyche: 4}}
	if got := c.MaxPsy(); got != 10 {
		t.Errorf("worn psyche gave a ceiling of %d, want 10", got)
	}
	if c.Psyche != 6 || c.MaxPsyche != 6 {
		t.Errorf("worn psyche changed the pool itself: %d/%d", c.Psyche, c.MaxPsyche)
	}
}

func TestTitledNamesTheAffix(t *testing.T) {
	c := base()
	if got := c.Weapon.Titled(); got != "Actual Sword" {
		t.Errorf("an unaffixed weapon is titled %q", got)
	}
	c.Weapon.Affix = &model.Affix{Suffix: "of the Damp"}
	if got := c.Weapon.Titled(); got != "Actual Sword of the Damp" {
		t.Errorf("an affixed weapon is titled %q", got)
	}
	// An affix with no suffix must not leave a trailing space.
	c.Weapon.Affix = &model.Affix{}
	if got := c.Weapon.Titled(); got != "Actual Sword" {
		t.Errorf("an empty affix produced %q", got)
	}
}

func TestEmptySlotsAreNotWorn(t *testing.T) {
	c := base()
	if c.Shield.Worn() || c.Charm.Worn() {
		t.Error("a bare character is wearing a shield or a charm")
	}
	c.Shield = model.Shield{Name: "Barrel Lid", Defense: 1}
	c.Charm = model.Charm{Name: "Lucky Tooth"}
	if !c.Shield.Worn() || !c.Charm.Worn() {
		t.Error("equipped slots do not report as worn")
	}
}

// A suffix must never land on a name that already ends in one. Half the weapon
// table is called "<thing> of <joke>", and bolting a second flourish on gives
// "Runed Maul of the Last Word of the Last Word".
func TestAffixableRejectsNamesThatAlreadyHaveAFlourish(t *testing.T) {
	cases := map[string]bool{
		"Notched Blade":                      true,
		"Flamberge 'The Apology'":            true,
		"Half-Plate, Dented Fondly":          true,
		"Mace of Modest Ambition":            false,
		"Scale of the Overconfident":         false,
		"Blade of Escalating Poor Decisions": false,
	}
	for name, want := range cases {
		if got := model.Affixable(name); got != want {
			t.Errorf("Affixable(%q) = %v, want %v", name, got, want)
		}
	}
	// "of" inside a word must not trip it.
	if !model.Affixable("Ofcourse Hammer") {
		t.Error(`a name containing "of" as part of a word was rejected`)
	}
}

// Equipping swaps rather than replaces. The old piece goes back in the pack,
// because nothing should be destroyed by changing your mind — which is the
// whole reason equipment is carried instead of being a yes/no question asked
// once at the moment it is found.
func TestEquippingPutsTheOldPieceBackInThePack(t *testing.T) {
	c := base()
	old := c.Weapon

	found := model.Weapon{Name: "Actual Sword", Strike: 9}
	c.Carry(model.Carried{Weapon: &found})
	if len(c.Carried) != 1 {
		t.Fatalf("carrying one sword left %d things in the pack", len(c.Carried))
	}

	if !c.Equip(0) {
		t.Fatal("equipping the sword failed")
	}
	if c.Weapon.Name != "Actual Sword" {
		t.Errorf("after equipping, the weapon is %q", c.Weapon.Name)
	}
	if len(c.Carried) != 1 || c.Carried[0].Weapon == nil ||
		c.Carried[0].Weapon.Name != old.Name {
		t.Errorf("the old %q did not come back to the pack; it holds %+v", old.Name, c.Carried)
	}

	// And swapping back is symmetrical, so a mistake costs one keypress.
	if !c.Equip(0) {
		t.Fatal("equipping the old weapon back failed")
	}
	if c.Weapon.Name != old.Name {
		t.Errorf("swapping back gave %q, want %q", c.Weapon.Name, old.Name)
	}
}

// An empty slot has nothing to put back, so equipping into one must not leave
// a nameless ghost in the pack.
func TestEquippingIntoAnEmptySlotLeavesNothingBehind(t *testing.T) {
	c := base()
	c.Shield = model.Shield{}

	sh := model.Shield{Name: "Barrel Lid", Defense: 1}
	c.Carry(model.Carried{Shield: &sh})
	if !c.Equip(0) {
		t.Fatal("equipping the shield failed")
	}
	if !c.Shield.Worn() {
		t.Error("the shield did not go on")
	}
	if len(c.Carried) != 0 {
		t.Errorf("an empty slot put %+v back in the pack", c.Carried)
	}
}

// Selling takes a piece out without wearing it.
func TestDroppingCarriedGearRemovesIt(t *testing.T) {
	c := base()
	a := model.Armor{Name: "Boiled Leather", Defense: 3}
	b := model.Armor{Name: "Studded Leather", Defense: 4}
	c.Carry(model.Carried{Armor: &a})
	c.Carry(model.Carried{Armor: &b})

	got, ok := c.DropCarried(0)
	if !ok || got.Titled() != "Boiled Leather" {
		t.Fatalf("dropping the first piece gave %q, %v", got.Titled(), ok)
	}
	if len(c.Carried) != 1 || c.Carried[0].Titled() != "Studded Leather" {
		t.Errorf("the pack now holds %+v", c.Carried)
	}
	if _, ok := c.DropCarried(5); ok {
		t.Error("dropping past the end of the pack succeeded")
	}
}

// A Shield's Extra is a pointer, so every copy of one shares the Bonus that
// came off the content table. Nudged is the only sanctioned way to change a
// shield, and this is the thing it has to be true of.
//
// The bug it is standing in for was not hypothetical: the balance report added
// six points of ward to price the slot, wrote through the pointer, and buffed
// every shield of that tier a little more on each of four thousand iterations
// until both sides of the comparison were saturated and ward measured as worth
// exactly nothing. Nothing failed, nothing warned, and the number looked
// plausible enough to be believed.
func TestNudgingAShieldLeavesTheOriginalAlone(t *testing.T) {
	shelf := model.Shield{Name: "Mirror-Backed Targe", Defense: 3,
		Extra: &model.Bonus{Ward: 7, Speed: -1}}
	worn := shelf

	got := worn.Nudged(model.Bonus{Ward: 6})

	if shelf.Extra.Ward != 7 {
		t.Errorf("the shelf's copy now carries %d ward; Nudged wrote through the pointer",
			shelf.Extra.Ward)
	}
	if got.Extra.Ward != 13 {
		t.Errorf("the nudged copy carries %d ward, want 13", got.Extra.Ward)
	}
	if got.Extra.Speed != -1 {
		t.Errorf("the nudged copy lost the original's %d speed", got.Extra.Speed)
	}
	if got.Extra == shelf.Extra {
		t.Error("the nudged copy still points at the shelf's own Bonus")
	}

	// And a shield that does nothing extra must come back holding only the
	// nudge, rather than nil-dereferencing on the way.
	plain := model.Shield{Name: "Barrel Lid", Defense: 1}
	if b := plain.Nudged(model.Bonus{Ward: 2}); b.Extra == nil || b.Extra.Ward != 2 {
		t.Errorf("nudging a plain shield gave %+v", b.Extra)
	}
	if plain.Extra != nil {
		t.Error("nudging a plain shield gave the original an Extra it never had")
	}
}
