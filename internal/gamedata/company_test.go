package gamedata_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// anywhere is a town with every counter in it, for the tests that are about
// what somebody wants rather than about where they are standing.
func anywhere(gamedata.Counter) bool { return true }

// TestACompanionCatchesUpToTheCurveAndStopsThere is the whole claim the cut
// now makes: money buys the on-curve kit, and buys nothing beyond it.
//
// Both halves matter. A companion who never converges is a standing charge
// that buys nothing; one who keeps buying past the curve is a hireling
// out-equipping the balance report's own assumption about what "properly
// equipped" means, which would make every number measured against Equip a
// number about somebody poorer than the party actually is.
func TestACompanionCatchesUpToTheCurveAndStopsThere(t *testing.T) {
	tables := load(t)
	for _, class := range []model.Class{model.ClassFighter, model.ClassThief, model.ClassMage} {
		for _, level := range []int{1, 4, 7, 10, 13} {
			// Dressed for level one and walked to here without spending a coin,
			// which is exactly what a companion hired early and never taken to
			// a town looks like.
			c := &model.Character{Class: class, Level: 1, Ally: true}
			tables.Equip(c)
			c.Level = level
			c.Coins = 100000

			bought, _ := tables.Shop(c, anywhere)
			if level > 1 && len(bought) == 0 {
				t.Errorf("%s at level %d bought nothing with a fortune in hand", class, level)
			}

			curve := &model.Character{Class: class, Level: level}
			tables.Equip(curve)
			if c.Weapon.Name != curve.Weapon.Name || c.Armor.Name != curve.Armor.Name ||
				c.Shield.Name != curve.Shield.Name || c.Charm.Name != curve.Charm.Name {
				t.Errorf("%s at level %d shopped to %q/%q/%q/%q, curve is %q/%q/%q/%q",
					class, level,
					c.Weapon.Name, c.Armor.Name, c.Shield.Name, c.Charm.Name,
					curve.Weapon.Name, curve.Armor.Name, curve.Shield.Name, curve.Charm.Name)
			}
			if _, still := tables.Wants(c); still {
				t.Errorf("%s at level %d is on curve and still saving for something", class, level)
			}
			// A second trip to the same town with the same fortune has to be a
			// wasted walk, or a companion in a town spends until they are broke.
			if again, _ := tables.Shop(c, anywhere); len(again) > 0 {
				t.Errorf("%s at level %d went shopping twice", class, level)
			}
		}
	}
}

// TestNothingIsDestroyedByAnUpgrade holds the rule the whole equipment system
// is built on at the one door that was not going through Character.Equip: a
// companion buying a sword has to hand back the sword they had.
func TestNothingIsDestroyedByAnUpgrade(t *testing.T) {
	tables := load(t)
	c := &model.Character{Class: model.ClassFighter, Level: 1, Ally: true}
	tables.Equip(c)
	had := map[string]bool{c.Weapon.Name: true, c.Armor.Name: true}

	c.Level, c.Coins = 13, 100000
	_, off := tables.Shop(c, anywhere)
	for _, gear := range off {
		delete(had, gear.Titled())
	}
	if len(had) > 0 {
		t.Errorf("%d pieces went nowhere when they were replaced: %v", len(had), had)
	}
	// And nothing was quietly left in their pack: a companion's pack is for
	// what the player gave them, not for gear the shopping trip mislaid.
	if len(c.Carried) > 0 {
		t.Errorf("shopping left %d pieces in the companion's pack", len(c.Carried))
	}
}

// TestACompanionCannotBuyWhatTheTownDoesNotSell. A village has a smith and an
// apothecary and no armourer, and somebody walking out of one wearing a
// breastplate is the game naming something that was not there.
func TestACompanionCannotBuyWhatTheTownDoesNotSell(t *testing.T) {
	tables := load(t)
	c := &model.Character{Class: model.ClassFighter, Level: 1, Ally: true}
	tables.Equip(c)
	c.Level, c.Coins = 13, 100000
	was := c.Armor.Name

	smithOnly := func(ct gamedata.Counter) bool { return ct == gamedata.CounterSmith }
	bought, _ := tables.Shop(c, smithOnly)
	if len(bought) == 0 {
		t.Fatal("a smith on his own sold nothing at all")
	}
	if c.Armor.Name != was {
		t.Errorf("bought %q in a place with no armourer", c.Armor.Name)
	}
	for _, gear := range bought {
		if gear.Armor != nil || gear.Charm != nil {
			t.Errorf("the smith sold %s, which is the armourer's shelf", gear.Slot())
		}
	}
}

// TestSavingForIsAlwaysAffordableEventually. The sheet quotes a price against
// a purse, so a want nobody could ever reach would be a companion saving
// forever in front of a number that never moves.
func TestSavingForIsAlwaysAffordableEventually(t *testing.T) {
	tables := load(t)
	c := &model.Character{Class: model.ClassFighter, Level: 14, Ally: true}
	tables.Equip(c)
	c.Weapon, c.Armor, c.Shield, c.Charm = model.Weapon{}, model.Armor{}, model.Shield{}, model.Charm{}

	seen := map[string]bool{}
	for i := 0; i < 8; i++ {
		w, ok := tables.Wants(c)
		if !ok {
			break
		}
		if w.Cost <= 0 {
			t.Fatalf("saving for %q, which costs nothing", w.Gear.Titled())
		}
		if seen[w.Gear.Titled()] {
			t.Fatalf("still saving for %q after buying it", w.Gear.Titled())
		}
		seen[w.Gear.Titled()] = true
		c.Coins = int64(w.Cost)
		if bought, _ := tables.Shop(c, anywhere); len(bought) != 1 {
			t.Fatalf("exact money for %q bought %d things", w.Gear.Titled(), len(bought))
		}
	}
	if _, still := tables.Wants(c); still {
		t.Error("a stripped companion never finished re-equipping")
	}
}

// TestTheBaselineTakesTheLaneItsLevelCallsFor.
//
// The off arm was the one slot in Equip nobody had ever chosen: ArmBlock is
// the zero value of Archetype.Arm, so the balanced build carried the wall for
// the life of the report by default rather than by decision — and it cost 11.2
// points at level thirteen, at identical spend, to every hireling in the game.
//
// This holds the decision that replaced it. Not the crossover level, which is
// LANES' job to measure and move: what is pinned here is that the lane is a
// function of level at all, that it runs guard-then-ward rather than the other
// way round, and that the baseline never quietly acquires an opinion about
// offence.
func TestTheBaselineTakesTheLaneItsLevelCallsFor(t *testing.T) {
	tables := load(t)
	switched := 0
	for level := 1; level <= 14; level++ {
		c := &model.Character{Class: model.ClassFighter, Level: level}
		tables.Equip(c)
		if !c.Shield.Worn() {
			continue
		}
		lane := c.Shield.Lane()
		if lane == model.ArmStrike {
			t.Errorf("level %d: the baseline is carrying %q, which is the offensive lane",
				level, c.Shield.Name)
		}
		if want := gamedata.LaneForLevel(level); lane != want {
			t.Errorf("level %d: Equip gave lane %v (%q), LaneForLevel says %v",
				level, lane, c.Shield.Name, want)
		}
		if lane == model.ArmWard {
			if switched == 0 {
				switched = level
			}
		} else if switched != 0 {
			t.Errorf("level %d went back to the wall after switching at %d", level, switched)
		}
	}
	if switched == 0 {
		t.Error("the baseline never picks up the ward lane at any level")
	}
	if switched <= 1 || switched >= 14 {
		t.Errorf("the lane switches at level %d, which is not a crossover, it is a constant", switched)
	}
}
