package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// A swings-only run must actually swing. If it casts even once the band in
// SWINGS ONLY is not a control, and the difference it reports against the
// competent player is a difference between two things nobody named.
//
// Checked by breaking it: with the guard in strike() removed this fails on the
// Mage at the first level it has psyche for a spark.
func TestNeverCastReachesForNothing(t *testing.T) {
	for _, class := range model.AllClasses {
		for _, level := range []int{1, 5, 9, 13} {
			g := core.NewRNG(int64(level))
			c := rules.BuildCharacter(g, class, level)
			c.Weapon = model.Weapon{Name: "Actual Sword", Strike: 6}
			total := 0
			for i := 0; i < 200; i++ {
				fresh := *c
				r := rules.SimulateGroupAs(g, &fresh, []*model.Monster{
					{Def: &model.MonsterDef{Name: "Something"}, Name: "Something",
						HP: 200, MaxHP: 200, Offense: 4, Defense: 4, Ward: 4, Speed: 4},
				}, 40, spellbook, rules.Policy{NeverCast: true})
				total += r.Casts
			}
			if total != 0 {
				t.Errorf("%s at level %d cast %d times with NeverCast set", class, level, total)
			}
		}
	}
	// And the control is only a control if the thing it is controlling for
	// happens: a policy that never casts either way would pass the loop above
	// and measure nothing at all.
	g := core.NewRNG(4)
	c := rules.BuildCharacter(g, model.ClassMage, 9)
	c.Weapon = model.Weapon{Name: "Stick", Strike: 1}
	casts := 0
	for i := 0; i < 200; i++ {
		fresh := *c
		casts += rules.SimulateGroupAs(g, &fresh, []*model.Monster{
			{Def: &model.MonsterDef{Name: "Something"}, Name: "Something",
				HP: 200, MaxHP: 200, Offense: 4, Defense: 4, Ward: 4, Speed: 4},
		}, 40, spellbook, rules.Policy{}).Casts
	}
	if casts == 0 {
		t.Fatal("the competent policy cast nothing either, so NeverCast controls for nothing")
	}
}

// The gate decides between a technique and a free swing, and the two meet
// different defences: a swing meets Defense and a technique meets Ward. It has
// to be measured where the blow lands.
//
// The case is the one that was wrong in the game rather than a constructed
// one: a shelled, unwarded creature — the level-one coast crab is Defense 6,
// Ward 1 — against a character whose technique is smaller than their swing and
// lands for more of it. Comparing magnitudes refuses the better blow.
func TestTheGateMeasuresWhereTheBlowLands(t *testing.T) {
	g := core.NewRNG(17)
	c := rules.BuildCharacter(g, model.ClassThief, 1)
	c.Weapon = model.Weapon{Name: "Mace", Strike: 5}
	c.Psyche = c.MaxPsyche

	knife := []model.Spell{{
		ID: "poke", Name: "Poke", Level: 1, Cost: 1, Power: 6,
		Kind: model.SpellDamage, Target: model.TargetOne,
	}}

	// Behind a shell and nothing else, the technique is the better blow.
	shelled := &model.Monster{Name: "Crab", HP: 20, MaxHP: 20, Defense: 12, Ward: 0, Speed: 4}
	// Behind a ward and nothing else, the same two swap places.
	warded := &model.Monster{Name: "Wisp", HP: 20, MaxHP: 20, Defense: 0, Ward: 12, Speed: 4}

	if _, ok := rules.BestAttackAgainst(c, knife, shelled); !ok {
		t.Error("the knife was refused against a creature whose shell stops the sword")
	}
	if _, ok := rules.BestAttackAgainst(c, knife, warded); ok {
		t.Error("the knife was chosen against a creature whose ward stops the knife")
	}
	// Both branches must be reachable from one spell and one character, or the
	// test is asserting two unrelated facts about two setups.
}

// A pact charges the caster for the rest of the fight, and the gate has to
// price that or the policy casts one whose net worth is under a free swing.
//
// This is the defect that kept the swings-only Fighter ahead at level thirteen
// after the gate already measured landed damage: it was right about the first
// technique of the fight and blind to the second.
func TestAPactIsGatedOnWhatIsLeftOfIt(t *testing.T) {
	g := core.NewRNG(31)
	c := rules.BuildCharacter(g, model.ClassFighter, 10)
	c.Weapon = model.Weapon{Name: "Sabre", Strike: 21}
	c.Psyche = c.MaxPsyche

	target := &model.Monster{Name: "Something", HP: 90, MaxHP: 90, Defense: 14, Ward: 7, Speed: 8}
	swing := rules.FreeSwingAgainst(c, target)

	// A pact pitched so its raw magnitude clears the swing and its net does
	// not. Solving for it rather than guessing keeps the test honest if the
	// coefficients move.
	for power := 1; power < 400; power++ {
		p := model.Spell{
			ID: "pact", Name: "Pact", Level: 1, Cost: 1, Power: power,
			Kind: model.SpellPact, Target: model.TargetOne,
		}
		raw := rules.SpellPower(c, p) - float64(target.Ward)
		net := raw - float64(rules.PactCost(p))
		if raw > swing && net <= swing {
			if _, ok := rules.BestAttackAgainst(c, []model.Spell{p}, target); ok {
				t.Fatalf("a pact landing %.1f and leaving %.1f was chosen over a swing worth %.1f",
					raw, net, swing)
			}
			return
		}
	}
	t.Skip("no pact power separates the raw and net cases at these coefficients")
}
