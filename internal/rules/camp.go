package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// Camping: the answer to the walk back.
//
// The measurement that produced this is a division nobody had done, because it
// lives between two tables. ENDURANCE says how many fights one rest buys and
// PROGRESSION says how many fights a level costs; the quotient is how many
// round trips to an inn a level takes, and it runs from a tenth of one at level
// one to eleven and a half at level fourteen. Eleven times the same walk, for
// one level. Nothing in the report was watching it, because no single section
// could see it.
//
// The lever is the trip rather than the fights. Most of that hundredfold is
// endurance collapsing — eighteen fights a rest down to two and a half — and a
// game that gets more attritional as it goes is a legitimate arc; a game that
// makes you walk back eleven times to enjoy it is not.

// What a camp hands back, as a share of the pools. Half rather than all,
// because the inn has to keep a reason to exist and "full" is the whole of what
// it sells beyond the two things below.
const (
	campHPShare     = 0.5
	campPsycheShare = 0.5
)

// CampSteps is how long making camp takes, in the steps the clock counts in.
// A sixteenth of a day: long enough that camping through a night takes several
// and the weather has moved on by morning, short enough that it is not a way to
// skip to dawn — which is the inn's job and stays the inn's job.
const CampSteps = 30

// MakeCamp restores a share of both pools to one member of the company.
// Reports what came back.
//
// Deliberately not a rest: it does not fill the pools, it does not move the
// clock to dawn, and the caller does not write a checkpoint. An inn buys all
// three, which is why one still costs money at the top of the game. What a camp
// buys is the walk.
func MakeCamp(c *model.Character, share float64) (hp, psyche int) {
	if c == nil || !c.Alive() {
		return 0, 0
	}
	hp = core.Clamp(int(float64(c.MaxHP)*campHPShare*share), 0, c.MaxHP-c.HP)
	psyche = core.Clamp(int(float64(c.MaxPsyche)*campPsycheShare*share), 0, c.MaxPsyche-c.Psyche)
	c.HP += hp
	c.Psyche += psyche
	return hp, psyche
}

// campBase is the chance of being found while asleep somewhere with no door on
// it, before the light and the ground are taken into account.
const campBase = 0.18

// CampDisturbed reports whether something walks in on the camp.
//
// prowl is the sky's own multiplier — the one the encounter roll already reads,
// so a clear night is the dangerous one here for exactly the reason it is
// dangerous out on the road. danger is how rough the ground is, on the same
// scale the region levels use, and indoors doubles it: a dungeon is somewhere
// with things already living in it, and lying down in one is a decision rather
// than a rest.
//
// Capped well short of certain. A camp that fails half the time is a camp
// nobody packs, and the point of the item is to be the thing you reach for
// rather than the gamble you avoid.
// quality is what the kit itself is worth, as percentage points off the odds:
// zero for a bedroll and a blanket, more for canvas and a stake for the fire.
// It is the only difference between the two things on the shelf, and it is the
// right one — what you are buying when you pay three times as much for a night
// outdoors is not a better night, it is a smaller chance of it going wrong.
func CampChance(prowl float64, danger int, indoors bool, quality int) float64 {
	odds := campBase * prowl * (1 + float64(core.Max(0, danger))*0.06)
	if indoors {
		odds *= 2
	}
	odds *= 1 - core.ClampF(float64(quality)/100, 0, 0.8)
	return core.ClampF(odds, 0.02, 0.45)
}

// CampDisturbed rolls it.
func CampDisturbed(g *core.RNG, prowl float64, danger int, indoors bool, quality int) bool {
	return g.Chance(CampChance(prowl, danger, indoors, quality))
}

// DisturbedShare is how much of the rest survives being woken up. Not nothing:
// the fight that interrupts a camp is the cost, and losing the whole night on
// top of it would make the roll feel like a punishment for having tried.
const DisturbedShare = 0.4
