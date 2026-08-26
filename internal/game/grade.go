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
		// An empty arm is a rating of nothing, so the first shield reads as the
		// upgrade it is rather than as no change.
		if buyer.Shield.Worn() {
			have = buyer.Shield.Defense + affixOf(buyer.Shield.Affix).Defense
		}
		want = v.Defense
	default:
		return "", nil
	}

	d := want - have
	switch {
	case d > 0:
		return fmt.Sprintf("+%d", d), gradeDelta(d)
	case d < 0:
		return fmt.Sprintf("%d", d), gradeDelta(d)
	}
	return "=", render.ColInkDim
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
