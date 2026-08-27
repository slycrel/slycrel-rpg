package model

// EffectKind is a condition riding on a combatant.
//
// These replace what used to be four separate mechanisms bolted onto the battle
// screen — a per-monster weaken array, a per-monster stun flag, a per-member
// blessing map, and two loose integers for the hero's potions. They behaved
// differently for no reason anybody could have explained, and none of them
// could be given a duration without a fifth mechanism.
type EffectKind string

const (
	EffectPoison  EffectKind = "poison"  // damage at the end of each round
	EffectBurn    EffectKind = "burn"    // more damage, over fewer rounds
	EffectWeaken  EffectKind = "weaken"  // deals less
	EffectBless   EffectKind = "bless"   // deals more
	EffectQuicken EffectKind = "quicken" // harder to hit past
	EffectStun    EffectKind = "stun"    // loses its turn
	// EffectBarrier is damage that lands on something other than the body. It
	// is spent rather than timed: every point that comes off it is a point that
	// does not come off hit points, and when it is gone it is gone for the rest
	// of the fight.
	EffectBarrier EffectKind = "barrier"
)

// Forever marks an effect that lasts the rest of the fight rather than a set
// number of rounds. Nothing outlives a battle, so this is the longest there is.
const Forever = -1

// Harmful reports whether a kind is something done to a combatant rather than
// for one. It decides the colour a line is logged in and whether the effect is
// worth telling the player about when it lands on their own party.
func (k EffectKind) Harmful() bool {
	switch k {
	case EffectBless, EffectQuicken, EffectBarrier:
		return false
	}
	return true
}

// Verb is how the effect reads in the transcript when it ticks.
func (k EffectKind) Verb() string {
	switch k {
	case EffectPoison:
		return "the poison works on"
	case EffectBurn:
		return "the burning takes"
	}
	return ""
}

// Effect is one condition and how much longer it has.
type Effect struct {
	Kind   EffectKind
	Power  int
	Rounds int // Forever, or the number of rounds still to run
}

// Effects is a combatant's active conditions.
//
// It is deliberately not persisted: nothing can be saved mid-battle, and a
// condition that outlived the fight it was applied in would be a bug rather
// than a feature.
type Effects []Effect
