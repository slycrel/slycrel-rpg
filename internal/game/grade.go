package game

import (
	"fmt"
	"image/color"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
)

// Grading a number against what it could have been.
//
// Two screens ask the same question in different words. Character creation asks
// "is this a good roll for this class", where the answer depends entirely on
// the class — a Mage with eight Strength is a good Mage. A shop counter asks "is
// this better than the one I am holding", where the answer depends on what is
// equipped. Both come out as the same three colours, because to a player they
// are the same question: is this number good news.

// The thirds. A roll in the bottom third of its band is a bad one and the top
// third a good one; everything between is a roll, which is what most rolls are.
// Splitting at the halfway mark instead would call every result either good or
// bad and make the colour meaningless — nothing would ever just be fine.
const (
	poorBelow = 1.0 / 3
	goodAbove = 2.0 / 3
)

// gradeFrac colours a position within a band.
func gradeFrac(f float64) color.Color {
	switch {
	case f < poorBelow:
		return render.ColWorse
	case f > goodAbove:
		return render.ColBetter
	}
	return render.ColInk
}

// gradeDelta colours a difference against what is already held: better, worse,
// or the same. Used where there is no band, only a comparison.
func gradeDelta(d int) color.Color {
	switch {
	case d > 0:
		return render.ColBetter
	case d < 0:
		return render.ColWorse
	}
	return render.ColInk
}

// shelfVerdict is what a piece of gear on a counter is worth against the one
// the buyer is already wearing, as a suffix for the price column and a colour.
//
// Weapons, armour and shields compare on one number each, which is the number
// the slot exists for. Charms deliberately do not: every charm in the table
// gives with one hand and takes with the other, so there is no better one and
// marking one green would be the interface lying about a system built on
// purpose so that "did I get the good one" is not the only question. They come
// back with no verdict at all, which is the honest answer.
//
// The comparison is against what is *equipped*, affix included, because that is
// what buying this would replace.
func shelfVerdict(buyer *model.Character, data any) (string, color.Color) {
	if buyer == nil {
		return "", nil
	}
	// A row nobody at this counter could leave with says who could, and says it
	// where the comparison would have gone. There is no point grading a maul
	// against a mage's rod: the answer is not "worse", it is "not for you", and
	// the two read completely differently to somebody deciding what to save up
	// for.
	if ok, why := buyer.CanUse(carriedOf(data)); !ok {
		return why, render.ColInkFaint
	}

	d, ok := shelfDelta(buyer, data)
	if !ok {
		return "", nil
	}
	switch {
	case d > 0:
		return fmt.Sprintf("+%d", d), gradeDelta(d)
	case d < 0:
		return fmt.Sprintf("%d", d), gradeDelta(d)
	}
	return "=", render.ColInkDim
}

// shelfDelta is how much better a piece of gear is than the one already worn,
// on whichever number the slot is actually sold on, and whether the question
// even applies.
//
// Split out of shelfVerdict so the counter and the shelf answer it the same
// way. A second comparison written for the buy confirmation would be a second
// opinion about which of a rod's numbers matters, and the two would drift.
func shelfDelta(buyer *model.Character, data any) (int, bool) {
	if buyer == nil {
		return 0, false
	}
	var have, want int
	switch v := data.(type) {
	case model.Weapon:
		have, want = buyer.Weapon.Strike+affixOf(buyer.Weapon.Affix).Strike, v.Strike
		// A caster shops for focus, so that is the number the shelf grades.
		// Comparing a rod's strike against a rod's strike would rank the whole
		// caster ladder as identical junk, which is what it is at hitting
		// people with.
		if v.Kind == model.WeaponFocus || buyer.Casting() {
			have, want = buyer.Weapon.Focus, v.Focus
		}
	case model.Armor:
		have, want = buyer.Armor.Defense+affixOf(buyer.Armor.Affix).Defense, v.Defense
	case model.Shield:
		// Graded inside its own lane, on that lane's own number.
		//
		// The off arm is three shelves wearing one field name: a wall sells
		// guard, a silvered one sells ward, a spiked one sells strike, and a
		// talisman sells a pool. Ranking all four by Defense told a player
		// shopping for anti-magic that a fifty-two-coin shrine plate was worth
		// "+1" — which is true of the number it was measured on and useless
		// about the thing they were buying.
		//
		// An empty arm is a rating of nothing, so the first one of any lane
		// reads as the upgrade it is. A shield of a *different* lane counts as
		// nothing too: swapping the wall for the silver is a change of plan,
		// not a step up a ladder, and there is no honest single figure for it.
		lane := v.Lane()
		if buyer.Shield.Worn() && buyer.Shield.Lane() == lane {
			have = laneValue(buyer.Shield)
		}
		want = laneValue(v)
	case model.OffHand:
		// Off-hand weapons are one lane with a single number, so unlike the
		// charms they have an honest answer and should give it. Falling into
		// the charm default meant no verdict on the row and — since the
		// "wear it now?" prompt needs a positive delta — no offer to put on a
		// strictly better dagger than the one already in that hand.
		//
		// Against another off-hand weapon only. A plank in that hand is a
		// different plan rather than a rung below, the same reasoning that
		// stops a wall being graded against a silvered one.
		if buyer.Sidearm.Worn() {
			have = model.SidearmShare(buyer.Sidearm) + affixOf(buyer.Sidearm.Affix).Strike
		}
		want = model.SidearmShare(v.Weapon)
	default:
		// Charms deliberately have no answer: every one gives with one hand and
		// takes with the other, so there is no better one to point at.
		return 0, false
	}
	return want - have, true
}

// laneValue is the number an off-arm item is actually sold on.
func laneValue(s model.Shield) int {
	switch s.Lane() {
	case model.ArmBarrier:
		return s.Absorb
	case model.ArmWard:
		return s.Extra.Ward
	case model.ArmStrike:
		return s.Extra.Strike
	}
	return s.Defense + affixOf(s.Affix).Defense
}

// carriedOf boxes a shelf row as a piece of equipment, so one wielding rule
// serves the counter, the pack and the character sheet.
func carriedOf(data any) model.Carried {
	switch v := data.(type) {
	case model.Weapon:
		return model.Carried{Weapon: &v}
	case model.Armor:
		return model.Carried{Armor: &v}
	case model.Shield:
		return model.Carried{Shield: &v}
	case model.OffHand:
		w := v.Weapon
		return model.Carried{Sidearm: &w}
	case model.Charm:
		return model.Carried{Charm: &v}
	}
	return model.Carried{}
}

// affixOf is a nil-safe read of a suffix's bonus.
func affixOf(a *model.Affix) model.Bonus {
	if a == nil {
		return model.Bonus{}
	}
	return a.Bonus
}
