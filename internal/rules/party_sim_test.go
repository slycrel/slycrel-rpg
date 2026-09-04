package rules_test

import (
	"math"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// A company of one has to play the same game as the solo simulator.
//
// This is the check that makes a second loop safe to have at all. Every number
// in the balance report comes out of SimulateGroupAs, and SimulateParty is a
// second implementation of the same round — initiative, the monsters' turns,
// conditions biting at the end. Two implementations of one thing is the defect
// this package keeps finding, and the only defence against it here is that the
// party version reduces to the solo one when there is nobody else in the party.
//
// **Measured on rounds against a target dummy, not on win rate.** The first
// version of this compared win rates within five points and hit-point totals
// within a fifth, and it passed with the Fighter's second swing deleted from
// the party loop — an entire class mechanic missing, and the test did not
// notice, because a fight that is won either way is won either way. Rounds to
// kill is the sensitive quantity: it is damage output per round with nothing
// else in it, and the dummy cannot hit back, so the only thing that can move it
// is the rules.
func TestACompanyOfOnePlaysTheSoloGame(t *testing.T) {
	book := spellbook
	for _, class := range model.AllClasses {
		for _, level := range []int{1, 5, 9, 13} {
			const fights = 800
			var soloRounds, partyRounds int

			for f := 0; f < fights; f++ {
				g := core.NewRNG(int64(f))
				c := rules.BuildCharacter(g, class, level)
				c.Weapon = model.Weapon{Name: "Sword", Strike: 4 + level}
				// Above extraFloor at the top level, so the Fighter's second
				// swing is inside what this test can see rather than a
				// mechanic it silently never exercises — which is how the
				// first version of it passed with that swing deleted.
				if level >= 13 {
					c.Speed = 22
				}
				// A dummy with plenty of hit points and no attack: how long it
				// takes to fall is a clean reading of what the player does in a
				// round, and nothing about running away or dying can get in.
				dummy := func() []*model.Monster {
					return []*model.Monster{{
						Def: &model.MonsterDef{Name: "Dummy"}, Name: "Dummy",
						HP: 400, MaxHP: 400, Offense: 0, Defense: 2, Ward: 2, Speed: 1,
					}}
				}
				a := *c
				soloRounds += rules.SimulateGroup(core.NewRNG(int64(f)), &a, dummy(), 400, book).Rounds

				b := *c
				partyRounds += rules.SimulateParty(core.NewRNG(int64(f)), []*model.Character{&b},
					func(*model.Character) []model.Spell { return book }, dummy(), 400, rules.Policy{}).Rounds
			}

			so := float64(soloRounds) / fights
			pa := float64(partyRounds) / fights
			if so == 0 {
				t.Fatalf("%s at %d never finished a dummy", class, level)
			}
			// Three per cent. The two draw from the generator in a different
			// order and always will — the party version rolls initiative per
			// member off a list it built — so they cannot be identical; but a
			// missing swing, a missing technique or a mis-scaled blow all move
			// this by far more than sampling does.
			if d := math.Abs(so-pa) / so; d > 0.03 {
				t.Errorf("%s at %d: solo takes %.2f rounds to fell a dummy, a company of one takes %.2f (%.1f%% apart)",
					class, level, so, pa, d*100)
			}
		}
	}
}

// A companion has to actually fight, not merely stand there being hit.
//
// The direction is the whole claim, and the first version of this test could
// not tell the two apart: three characters against one creature beat one
// character even with the companions' turns deleted, because three bodies is
// three places for a claw to land and that alone wins the fight. It passed
// against a party of spectators.
//
// So the creature cannot hit back, and the measurement is rounds. An extra body
// is worth nothing at all against a dummy; an extra sword is worth a third off
// the clock. Nothing but the companions taking their turn can move this.
func TestACompanionSwingsRatherThanWatches(t *testing.T) {
	book := spellbook
	const fights = 400
	var oneRounds, threeRounds int
	for f := 0; f < fights; f++ {
		g := core.NewRNG(int64(f))
		hero := rules.BuildCharacter(g, model.ClassFighter, 8)
		hero.Weapon = model.Weapon{Name: "Sword", Strike: 10}
		hero.Psyche = 0 // swings only, so this measures swords and not spells
		dummy := func() []*model.Monster {
			return []*model.Monster{{
				Def: &model.MonsterDef{Name: "Dummy"}, Name: "Dummy",
				HP: 900, MaxHP: 900, Offense: 0, Defense: 6, Ward: 6, Speed: 1,
			}}
		}
		spells := func(*model.Character) []model.Spell { return book }

		a := *hero
		oneRounds += rules.SimulateParty(core.NewRNG(int64(f)), []*model.Character{&a},
			spells, dummy(), 400, rules.Policy{}).Rounds

		b, c1, c2 := *hero, *hero, *hero
		threeRounds += rules.SimulateParty(core.NewRNG(int64(f)), []*model.Character{&b, &c1, &c2},
			spells, dummy(), 400, rules.Policy{}).Rounds
	}
	one := float64(oneRounds) / fights
	three := float64(threeRounds) / fights
	if three >= one*0.5 {
		t.Errorf("one sword fells a dummy in %.1f rounds and three in %.1f; "+
			"three swords should be close to a third of the time, so the companions are not swinging",
			one, three)
	}
}
