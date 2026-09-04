package rules

import "github.com/slycrel/slycrel-rpg/internal/model"

// ClampedMean exposes the retreat policy's damage estimate to the external test
// package. It is unexported in the game because nothing outside these rules has
// any business estimating damage — the point of the function is that the policy
// stops keeping its own copy of the arithmetic — and it is exposed here because
// the alternative is testing it only through a fight, where a bias of two or
// three points a monster hides inside a win rate. Which is how the bias it
// replaced survived.
func ClampedMean(lo, hi, guard float64) float64 { return clampedMean(lo, hi, guard) }

// BestAttackAgainst and FreeSwingAgainst expose the two halves of the gate that
// decides between a technique and a free swing.
//
// Exposed for the same reason ClampedMean is: the gate is a comparison of two
// numbers, and testing it only through a fight hides a wrong answer inside a
// win rate. That is exactly how it survived for the life of the project
// comparing raw magnitudes across two different defences — every fight it
// mispriced still ended in a win, at the levels anybody was looking at.
func BestAttackAgainst(c *model.Character, spells []model.Spell, target *model.Monster) (model.Spell, bool) {
	return bestAttack(c, spells, target)
}

func FreeSwingAgainst(c *model.Character, target *model.Monster) float64 {
	return freeSwingWorth(c, target)
}
