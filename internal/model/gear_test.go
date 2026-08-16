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
