package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// TestSleightOfHandIsRestorativesOnly. A thief with a cheap supply of the thing
// keeping it upright is the class working as designed. A thief with a cheap
// supply of everything on the shelf is a shoplifting simulator, and the buffs
// are where that would show up first — they are the cheapest thing on the
// counter with a Power to divide by.
func TestSleightOfHandIsRestorativesOnly(t *testing.T) {
	thief := &model.Character{Class: model.ClassThief}
	for _, k := range []model.ItemKind{model.ItemHeal, model.ItemRevive, model.ItemCure} {
		if n := rules.SleightOfHand(thief, k); n != 2 {
			t.Errorf("a thief buying one %s left with %d, want 2", k, n)
		}
	}
	for _, k := range []model.ItemKind{model.ItemPsyche, model.ItemBuff, model.ItemTrinket, model.ItemKey} {
		if n := rules.SleightOfHand(thief, k); n != 1 {
			t.Errorf("a thief buying one %s left with %d; the perk is sustain, not shopping", k, n)
		}
	}
}

// TestOnlyTheThiefPocketsAnything. Nobody else has to buy their way out of not
// having a heal, so nobody else gets the discount for it.
func TestOnlyTheThiefPocketsAnything(t *testing.T) {
	for _, class := range []model.Class{model.ClassFighter, model.ClassMage} {
		c := &model.Character{Class: class}
		if n := rules.SleightOfHand(c, model.ItemHeal); n != 1 {
			t.Errorf("a %s left the counter with %d of one purchase", class, n)
		}
		if rules.Pickpocket(c) {
			t.Errorf("a %s is working the ones that ran", class)
		}
	}
	if !rules.Pickpocket(&model.Character{Class: model.ClassThief}) {
		t.Error("the thief is not working the ones that ran, which is its half of a routed fight")
	}
}

// TestNothingBreaksOnANilBuyer. Both of these are read off whoever is at the
// counter or holding the purse, and both are reachable from a UI that builds
// its rows before it has decided who that is.
func TestNothingBreaksOnANilBuyer(t *testing.T) {
	if n := rules.SleightOfHand(nil, model.ItemHeal); n != 1 {
		t.Errorf("a nil buyer left with %d", n)
	}
	if rules.Pickpocket(nil) {
		t.Error("a nil character is picking pockets")
	}
}
