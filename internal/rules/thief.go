package rules

import "github.com/slycrel/slycrel-rpg/internal/model"

// What the thief gets instead of a way to heal itself.
//
// The class has no restorative technique and never will — its list is two
// drains and a pile of ways to hit something harder — so its entire recovery
// runs through the pack. That makes items the thief's real defensive stat, and
// a class whose defensive stat is bought at a counter should be better at the
// counter than everybody else. Both perks below are that, in the two places
// the pack gets filled: buying, and standing over what is left of a fight.
//
// Neither is a new attack. That is deliberate and it is Jeremy's framing: the
// interesting version of the thief is "the monster cannot hurt you if it is not
// there", not "the thief also hits hard". A discount on the thing keeping it
// alive and a habit of coming away from a bad fight with something in hand are
// both survival, priced in convenience rather than in damage.

// SleightOfHand is how many of an item somebody actually leaves the counter
// with, having paid for one.
//
// Jeremy's own suggestion, and it came out better than a price cut: "does the
// thief get 2 healing items instead of one when purchased because they steal
// one and buy one?" A percentage off the sticker is a number nobody notices.
// Walking out with two is a thing you can see happen.
//
// Restoratives only. A thief with a cheap supply of the thing that keeps it
// upright is a class working as designed; a thief with a cheap supply of
// everything on the shelf is a shoplifting simulator, and the trinkets are a
// joke delivery system that does not need a discount.
func SleightOfHand(c *model.Character, kind model.ItemKind) int {
	if c == nil || c.Class != model.ClassThief {
		return 1
	}
	switch kind {
	case model.ItemHeal, model.ItemRevive, model.ItemCure:
		return 2
	}
	return 1
}

// Pickpocket reports whether this character takes something off the creatures
// that ran rather than only off the ones that fell.
//
// It pairs with the change that made a routed monster worth anything at all.
// Everybody now gets the purse a runner drops and half the experience for
// having done most of the work; a thief also gets its drop table, on the
// grounds that the moment something turns to run past you is the moment it is
// least careful about what it is holding.
//
// The nice part is what it does to how the two classes read the same event. A
// fight that ends with the last thing bolting is a disappointment to everybody
// else and a payday to the thief, which is a more interesting way to
// compensate a class than adding a number to it.
func Pickpocket(c *model.Character) bool {
	return c != nil && c.Class == model.ClassThief
}
