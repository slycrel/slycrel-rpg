package gamedata_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// budgetAt is what the baseline costs, which is the purse every build shops
// with in ARCS.
// Per class: a Thief's on-curve kit is about a tenth cheaper than a Fighter's,
// so one purse for all three would hand the cheaper classes spare money and
// call the result a fact about their build.
func budgetAt(t *testing.T, tables *gamedata.Tables, class model.Class, level int) int {
	t.Helper()
	c := &model.Character{Level: level, Class: class}
	tables.EquipAs(c, gamedata.Archetypes[0])
	return gamedata.GearCost(c)
}

// TestNoBuildOutspendsThePurse is the whole point of fitting to a budget.
//
// ARCS compared a duelist carrying 2,600 coins of gear against a balanced
// build carrying 2,220 and called the difference a finding about two-handed
// weapons. It was a finding about 380 coins. A build measured while holding
// more gear than the baseline could afford is not being compared with
// anything.
func TestNoBuildOutspendsThePurse(t *testing.T) {
	tables := load(t)
	for level := 1; level <= 14; level++ {
		for _, a := range gamedata.Archetypes {
			for _, class := range model.AllClasses {
				budget := budgetAt(t, tables, class, level)
				c := &model.Character{Level: level, Class: class}
				tables.EquipWithin(c, a, budget)
				if got := gamedata.GearCost(c); got > budget {
					t.Errorf("level %d %s %s: spent %d of a %d purse",
						level, class, a.Name, got, budget)
				}
			}
		}
	}
}

// TestTheBaselineReproducesItselfOnItsOwnPurse.
//
// The budget is defined as what balanced costs, so fitting balanced to it must
// hand back exactly balanced — otherwise the purse is not the baseline's purse
// and every other build is being measured against a number with no owner.
func TestTheBaselineReproducesItselfOnItsOwnPurse(t *testing.T) {
	tables := load(t)
	for level := 1; level <= 14; level++ {
		for _, class := range model.AllClasses {
			budget := budgetAt(t, tables, class, level)
			plain := &model.Character{Level: level, Class: class}
			tables.EquipAs(plain, gamedata.Archetypes[0])

			fitted := &model.Character{Level: level, Class: class}
			tables.EquipWithin(fitted, gamedata.Archetypes[0], budget)

			if plain.Weapon.Titled() != fitted.Weapon.Titled() ||
				plain.Armor.Titled() != fitted.Armor.Titled() ||
				plain.Shield.Titled() != fitted.Shield.Titled() ||
				plain.Charm.Titled() != fitted.Charm.Titled() {
				t.Errorf("level %d %s: balanced on its own purse came back different\n"+
					"  plain:  %s / %s / %s / %s\n  fitted: %s / %s / %s / %s",
					level, class,
					plain.Weapon.Titled(), plain.Armor.Titled(),
					plain.Shield.Titled(), plain.Charm.Titled(),
					fitted.Weapon.Titled(), fitted.Armor.Titled(),
					fitted.Shield.Titled(), fitted.Charm.Titled())
			}
		}
	}
}

// TestFittingKeepsTheShape. A purse may take bands off a build; it may not turn
// one build into another. The duelist's off arm stays shut and the weapon is
// the last thing anybody gives up, because a build that sold its weapon to
// afford a charm has stopped being the thing under test.
func TestFittingKeepsTheShape(t *testing.T) {
	tables := load(t)
	duelist, ok := gamedata.ArchetypeNamed("duelist")
	if !ok {
		t.Skip("no duelist archetype to check")
	}
	for level := 4; level <= 14; level++ {
		budget := budgetAt(t, tables, model.ClassFighter, level)
		c := &model.Character{Level: level, Class: model.ClassFighter}
		tables.EquipWithin(c, duelist, budget)
		if c.Shield.Worn() {
			t.Errorf("level %d: the duelist fitted to a purse picked up %q",
				level, c.Shield.Name)
		}
		if !c.Weapon.TwoHanded() {
			t.Errorf("level %d: the duelist fitted to a purse is holding %q, one-handed",
				level, c.Weapon.Name)
		}
	}
}

// TestAnImpossiblePurseStillDresses. Everything that dresses somebody has to
// hand back a usable character: a build floors at tier one rather than walking
// into a fight bare-handed, which is the arithmetic error EquipAs already
// learned once.
func TestAnImpossiblePurseStillDresses(t *testing.T) {
	tables := load(t)
	for _, a := range gamedata.Archetypes {
		c := &model.Character{Level: 13, Class: model.ClassFighter}
		tables.EquipWithin(c, a, 1)
		if c.Weapon.Name == "" {
			t.Errorf("%s on a purse of 1 came out holding nothing", a.Name)
		}
		if c.Armor.Name == "" {
			t.Errorf("%s on a purse of 1 came out wearing nothing", a.Name)
		}
	}
}
