package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// The fighter's active: sometimes the round comes round twice.
//
// From the original, and Jeremy's: past a certain speed an attack action
// occasionally landed twice. It is the fighter's half of the scheme because the
// other two classes' actives are conditional on being attacked — a counter
// needs a dodge, a siphon needs a blow to answer — and the fighter should not
// have to wait for permission. It is also the right shape for the class the
// tropes want brainless: there is nothing to decide, it simply happens, and
// what it rewards is the stat the class grows slowest.
//
// **This is the largest single thing in the scheme and it is deliberately the
// smallest number.** A weapon band is +5 strike; an extra swing is a second
// whole attack, worth more on the rounds it fires than every gear step in the
// game together. The arcs work in this project spent itself getting three
// builds within seven points of each other at equal spend, and you can re-price
// a charm but you cannot re-price "sometimes twice".
//
// So it reads Speed above a floor rather than scaling from zero, and the floor
// is set where a Fighter arrives around the middle of the game: the reward is
// for having levelled rather than for the class one picked, which is what "as
// you levelled" meant.

const (
	// extraFloor is the speed at which a second swing becomes possible at all.
	// A Fighter rolls 6-9 at level one and climbs about a point a level, so
	// this lands in the middle of a run rather than at either end.
	extraFloor = 14
	// extraStep is what each point of speed past the floor is worth, and
	// extraCap is where it stops. Both small: see above.
	extraStep = 0.015
	extraCap  = 0.15
)

// ExtraSwing reports whether a character's speed earns them a second action
// this round.
//
// Fighter only. The other two classes have their own actives and a third one
// here would be a global buff to damage wearing a class's name.
func ExtraSwing(g *core.RNG, c *model.Character) bool {
	if c == nil || c.Class != model.ClassFighter {
		return false
	}
	over := c.Spd() - extraFloor
	if over <= 0 {
		return false
	}
	return g.Chance(core.ClampF(float64(over)*extraStep, 0, extraCap))
}
