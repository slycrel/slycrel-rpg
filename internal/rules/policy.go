package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// What a technique is worth, in one currency, so that six kinds that do six
// unrelated things can be compared at all.
//
// The currency is **hit points over the rest of this fight** — either taken off
// the other side or not taken off you, which are the same thing to somebody
// deciding what to do with a round. A damage spell is worth what it lands. A
// poison is worth its ticks for as long as the fight has left to run. A stun is
// worth one round of that creature's output. A weaken is worth the difference
// it makes to every round after this one.
//
// Before this the policy could choose three kinds of technique out of nine, and
// the other six were not weighed and found wanting — they were never seen. Five
// of the Thief's nine, three of the Fighter's, four of the Mage's. Every number
// the report printed about a class was therefore a floor, under a heading that
// said "techniques used".
//
// **Every price here calls the arithmetic rather than describing it.** That is
// the rule this package has broken five times and each time in the same shape,
// so: a creature's output per round is incomingPerRound with one creature in
// the list, a tick is TickMean, a blow is techniqueWorth, a swing is
// freeSwingWorth. Nothing in this file knows a coefficient.

// roundsLeft is how much fight there is still to pay for, which is what turns a
// condition into a number.
//
// A poison over four rounds is worth four ticks in a fight with four rounds to
// go and one tick in a fight about to end, and the difference is most of what
// separates a good poison from a wasted round. It is the smaller of two races:
// how long they take to kill, and how long you last.
//
// Deliberately blunt. This is a policy, not a prediction, and the failure it
// has to avoid is not being a round out — it is being told that a fight with
// one round left has ten.
func roundsLeft(c *model.Character, living []*model.Monster, target *model.Monster) float64 {
	var theirHP int
	for _, m := range living {
		if !m.Dead {
			theirHP += m.HP
		}
	}
	mine := freeSwingWorth(c, target)
	if mine < 1 {
		mine = 1
	}
	toKill := float64(theirHP) / mine

	incoming := incomingPerRound(c, living)
	toDie := float64(c.MaxHP) // no threat: bounded by the round cap below
	if incoming > 0 {
		toDie = float64(c.HP) / float64(incoming)
	}

	n := toKill
	if toDie < n {
		n = toDie
	}
	// One at the floor — a round is happening whatever this says — and a
	// ceiling because a policy that believes in a fifty-round fight will pay
	// anything for a condition. Nothing in the report runs past about eight.
	return core.ClampF(n, 1, 8)
}

// alreadyOn reports whether a condition of this kind is already running, which
// is the difference between a technique and a wasted round.
//
// Apply takes the longer of two durations rather than stacking their power, so
// a second weaken on the same creature buys nothing at all unless the first is
// about to expire — and the ones that matter here last the whole fight.
func alreadyOn(list model.Effects, k model.EffectKind) bool {
	return Power(list, k) > 0
}

// techniqueValue prices any technique at all, in hit points over the rest of
// the fight, for a caster deciding what to spend a round on.
//
// Returns zero for anything that would buy nothing — a condition the target is
// already under, a heal on somebody at full, a revive with nobody down.
func techniqueValue(c *model.Character, s model.Spell, living []*model.Monster, target *model.Monster) float64 {
	if target == nil || !priced(s.Kind) {
		return 0
	}
	rounds := roundsLeft(c, living, target)

	// How many creatures it reaches. A technique over the field is worth its
	// effect once per creature standing, which is the whole reason the roster
	// has any: SHAPES exists because a composition is not a list, and this is
	// the one number in the policy that knows the difference.
	reach := 1.0
	if s.Target == model.TargetAll {
		n := 0
		for _, m := range living {
			if !m.Dead {
				n++
			}
		}
		reach = float64(core.Max(1, n))
	}

	switch s.Kind {
	case model.SpellDamage, model.SpellDrain, model.SpellPact:
		// Already priced, landed and net of what a pact costs the caster.
		return techniqueWorth(c, s, target) * reach

	case model.SpellPoison, model.SpellBurn:
		kind, dur := model.EffectPoison, float64(poisonRounds)
		if s.Kind == model.SpellBurn {
			kind, dur = model.EffectBurn, float64(burnRounds)
		}
		if alreadyOn(target.Active, kind) {
			return 0
		}
		// The ticks that will actually land: the condition's own duration, or
		// what is left of the fight, whichever runs out first.
		per := TickMean(model.Effect{Kind: kind, Power: s.Power})
		return per * core.MinF(dur, rounds) * reach

	case model.SpellWeaken, model.SpellSap:
		if alreadyOn(target.Active, model.EffectWeaken) {
			return 0
		}
		// What it takes off their swing, every round after this one — capped
		// at what they were going to hit for, since a creature cannot be
		// weakened past harmless.
		saved := core.MinF(float64(s.Power), float64(incomingPerRound(c, []*model.Monster{target})))
		v := saved * core.MaxF(0, rounds-1) * reach
		// A sap is two-sided: the caster wears the blessing, once per cast.
		if s.Kind == model.SpellSap {
			v += float64(s.Power) * core.MaxF(0, rounds-1)
		}
		return v

	case model.SpellStun:
		if alreadyOn(target.Active, model.EffectStun) {
			return 0
		}
		// One round of that creature, not of the room.
		return float64(incomingPerRound(c, []*model.Monster{target})) * reach

	case model.SpellHeal:
		// Only what would actually be restored. A heal on somebody near full
		// is a round spent on the overflow.
		return core.MinF(SpellPower(c, s), float64(c.MaxHP-c.HP))

	case model.SpellBless:
		if alreadyOn(c.Active, model.EffectBless) {
			return 0
		}
		// Added to every swing for the rest of the fight, which is the same
		// shape as a weaken and pointed the other way.
		return float64(s.Power) * core.MaxF(0, rounds-1)
	}
	return 0
}

// priced is every kind of technique the valuation above can put a number on,
// and it is the one list of them.
//
// Castable reads it too, so the report's UNREACHABLE block cannot fall out of
// step with the policy — which is exactly what happened the first time: the
// doors were opened and the block went on printing the old three for a run,
// because "what the policy can choose" was written down in two places.
//
// Revive is the one kind left outside, and it is left outside honestly rather
// than priced at zero and forgotten. Standing somebody up is worth a great
// deal and there is nobody to stand up: every fight in this report is one
// character, so a revive has no target that is not the caster and a caster who
// needs reviving has already lost. It becomes measurable when the report can
// fight a party, and not before.
func priced(k model.SpellKind) bool {
	switch k {
	case model.SpellDamage, model.SpellDrain, model.SpellPact,
		model.SpellPoison, model.SpellBurn,
		model.SpellWeaken, model.SpellSap,
		model.SpellStun, model.SpellHeal, model.SpellBless:
		return true
	}
	return false
}

// castKinds is every kind of technique a round can go on, in a stable order, so
// that counting what a fight actually did costs an array rather than a map.
//
// Four million fights go through the report and a map allocation in each of
// them is a measurable fraction of the run — which matters here more than it
// looks, because the whole reason for counting is that the report was only ever
// able to say whether a fight was won. What it was won *with* is the question a
// player asks, and it is the one "no playstyle beats auto-attacks" was really
// about.
var castKinds = []model.SpellKind{
	model.SpellDamage, model.SpellDrain, model.SpellPact,
	model.SpellPoison, model.SpellBurn,
	model.SpellWeaken, model.SpellSap, model.SpellStun,
	model.SpellHeal, model.SpellBless, model.SpellRevive,
}

// castKindCount sizes the counter array in FightResult.
const castKindCount = 11

// CastKinds is the order the per-kind counters are in.
func CastKinds() []model.SpellKind { return castKinds }

// castIndex is where a kind's counter lives, or -1 for a kind with none.
func castIndex(k model.SpellKind) int {
	for i, v := range castKinds {
		if v == k {
			return i
		}
	}
	return -1
}

// A guard on the counter array, since its size is a constant and its contents
// are a slice. They are declared apart because one is a type and the other is
// data, and the day they disagree the report indexes past the end of a fight.
func init() {
	if len(castKinds) != castKindCount {
		panic("rules: castKinds and castKindCount disagree")
	}
}
