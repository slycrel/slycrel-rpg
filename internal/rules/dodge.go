package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// The thief's unit of defence.
//
// Every class blocks — armour is flat subtraction and everybody wears some —
// and the Mage additionally owns a pool, the talisman's Absorb, spent once
// against a burst. The Thief owned nothing of its own: it wore lighter armour
// than the Fighter and answered damage with the same arithmetic. Dodge is its
// unit, and it is deliberately a different *shape* rather than a bigger number:
//
//	block   flat off every hit    worth most against many small ones
//	absorb  a pool spent once     worth most against a single burst
//	dodge   a chance of nothing    indifferent to how big the hit was
//
// Those three curve apart as monster Offense grows rather than staying
// parallel, which is the test of whether a difference is real or a rename.
//
// It reads Speed, which until now did nothing defensive at all — initiative and
// the flee roll were its whole job — and which the Thief rolls highest by a
// clear margin (9-13 against a Fighter's 6-9). Dexterity was the alternative
// and is already spoken for: it carries the player's own miss chance, the crit
// roll and the feint, and the feint's note is explicit that footwork of the
// deceptive kind is dexterity. Getting out of the way is not a lie, it is a
// race.
//
// What it is measured against is endurance rather than death rate, and that
// was the data's choice rather than a guess. The Thief is not worse at
// surviving a fight — at level thirteen three over it dies less often than the
// Fighter — it is worse at surviving an afternoon: 5.9 fights to a rest against
// the Fighter's 9.3 at level seven. Light armour takes more chip, and the class
// has no healing technique to answer it with. A chance to take nothing at all
// is chip damage's opposite.
const (
	// dodgeBase is the chance before any speed advantage, and dodgeStep is what
	// one point of speed over the attacker is worth.
	//
	// Measured rather than chosen, and the first attempt at measuring it was
	// itself wrong: sweeping dodgeBase moved almost nothing, because the speed
	// term dominates it — a Thief outpaces these creatures by eight to sixteen
	// points, so "base 0%" was already a 16% dodge at level seven and "base
	// 20%" was the cap. Sweeping the finished probability instead, from nothing
	// to full, is what actually priced it.
	dodgeBase = 0.08
	dodgeStep = 0.004
	// The ends. A floor above zero so a thief outsped by something is still a
	// thief; a ceiling because a defence that reaches half of everything stops
	// being a unit and becomes an argument for never wearing armour.
	dodgeFloor = 0.04
	dodgeCap   = 0.12
)

// DodgeChance is the probability that an incoming blow finds nobody there.
//
// Thief only. The scheme is one unit each and this is the Thief's; giving a
// share of it to everybody would make it a global buff to defence wearing a
// class's name, which is the kind of change that reads as identity and
// measures as inflation.
func DodgeChance(c *model.Character, monsterSpeed int) float64 {
	if c == nil || c.Class != model.ClassThief {
		return 0
	}
	base := dodgeBase + float64(c.Spd()-monsterSpeed)*dodgeStep
	return core.ClampF(base, dodgeFloor, dodgeCap)
}

// Dodged rolls it. Separate from MonsterDamage so the caller can say so: a hit
// that lands for nothing and a hit that never arrived are the same number and
// completely different sentences, and the transcript is where a defensive unit
// becomes something the player knows they have.
func Dodged(g *core.RNG, c *model.Character, monsterSpeed int) bool {
	if p := DodgeChance(c, monsterSpeed); p > 0 {
		return g.Chance(p)
	}
	return false
}
