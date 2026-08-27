package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// Effects are the conditions riding on a combatant during a fight: poison and
// burning that tick, weakness and blessings that change what a blow is worth,
// and stuns that cost a turn.
//
// The arithmetic lives here with the rest of the maths, so it is pure and
// testable, and so the battle screen is left doing what it should be doing —
// deciding when things happen and saying so — rather than also being the only
// place that knows what "weakened" means.

// Apply adds a condition, folding it into a matching one already present rather
// than letting the list grow without limit.
//
// Two blessings stack in power and take the longer of the two durations; the
// alternative is either a list that grows every round somebody casts, or a
// second blessing silently doing nothing. Stacking power is also what makes
// two potions worth drinking.
func Apply(list model.Effects, e model.Effect) model.Effects {
	if e.Rounds == 0 {
		e.Rounds = model.Forever
	}
	for i := range list {
		if list[i].Kind != e.Kind {
			continue
		}
		list[i].Power += e.Power
		list[i].Rounds = longer(list[i].Rounds, e.Rounds)
		return list
	}
	return append(list, e)
}

// longer returns whichever duration outlasts the other, treating Forever as
// beating any finite count.
func longer(a, b int) int {
	if a == model.Forever || b == model.Forever {
		return model.Forever
	}
	return core.Max(a, b)
}

// Has reports whether a condition of that kind is in force.
func Has(list model.Effects, k model.EffectKind) bool {
	for _, e := range list {
		if e.Kind == k {
			return true
		}
	}
	return false
}

// Power totals the magnitude of every condition of a kind.
func Power(list model.Effects, k model.EffectKind) int {
	n := 0
	for _, e := range list {
		if e.Kind == k {
			n += e.Power
		}
	}
	return n
}

// Remove drops every condition of a kind, which is how a stun is spent.
func Remove(list model.Effects, k model.EffectKind) model.Effects {
	out := list[:0]
	for _, e := range list {
		if e.Kind != k {
			out = append(out, e)
		}
	}
	return out
}

// --- the barrier ----------------------------------------------------------

// Barrier is the pool a talisman puts up at the start of a fight, or nothing
// when the off arm holds a plank or is empty.
//
// It is raised once per fight rather than regenerated per round, and that is
// the whole difference between this and a point of armour. Armour shaves every
// blow forever and is therefore worth most in a long grind; a barrier stops a
// fixed amount of anything and then is gone, so it is worth most against the
// opening exchange. That is the shape a Mage actually needs: their pool is
// small enough that the fights they lose are the ones where the first two
// blows land, not the ones that go long.
func Barrier(c *model.Character) (model.Effect, bool) {
	if c == nil || !c.Shield.Barrier() {
		return model.Effect{}, false
	}
	n := c.Shield.Absorb
	if c.Shield.Affix != nil {
		// A suffix on a talisman moves the pool, since that is the only number
		// on it anybody is buying.
		n += c.Shield.Affix.Bonus.Ward
	}
	if n <= 0 {
		return model.Effect{}, false
	}
	return model.Effect{Kind: model.EffectBarrier, Power: n, Rounds: model.Forever}, true
}

// Raise puts a character's barrier up, if they have one. Called once as a fight
// begins, by the battle screen and the simulator alike.
func Raise(c *model.Character) int {
	e, ok := Barrier(c)
	if !ok {
		return 0
	}
	c.Active = Apply(c.Active, e)
	return e.Power
}

// Soak runs incoming damage through whatever barrier is standing, returning
// what is left to land on the body and how much the barrier took.
//
// The pool is spent rather than timed, so this decrements Power and drops the
// effect when it runs out. It is the one condition in the list that is consumed
// by being used, which is why it does not go through Apply on the way down:
// Apply *adds* power, and a barrier only ever loses it.
func Soak(list model.Effects, dmg int) (model.Effects, int, int) {
	if dmg <= 0 {
		return list, dmg, 0
	}
	taken := 0
	out := list[:0]
	for _, e := range list {
		if e.Kind == model.EffectBarrier && dmg > 0 {
			n := core.Min(e.Power, dmg)
			e.Power -= n
			dmg -= n
			taken += n
			if e.Power <= 0 {
				continue
			}
		}
		out = append(out, e)
	}
	return out, dmg, taken
}

// OffenseMod is what the active conditions add to or take off a blow. A
// blessing and a weakening cancel each other out, which is the intuitive
// reading and saves the battle screen from applying them at two separate
// points in the damage calculation.
func OffenseMod(list model.Effects) int {
	return Power(list, model.EffectBless) - Power(list, model.EffectWeaken)
}

// DexterityMod is what the active conditions add to the odds of landing a hit.
func DexterityMod(list model.Effects) int { return Power(list, model.EffectQuicken) }

// Tick is one condition's damage at the end of a round.
type Tick struct {
	Kind   model.EffectKind
	Damage int
}

// TickDamage rolls what the lingering conditions cost their host this round.
//
// Poison and burning are rolled rather than fixed so that a long fight is not
// arithmetic the player can do in their head, and floored at one so a condition
// is never a decorative line in the transcript that changes nothing.
func TickDamage(g *core.RNG, list model.Effects) []Tick {
	var out []Tick
	for _, e := range list {
		var d int
		switch e.Kind {
		case model.EffectPoison:
			d = g.Between(e.Power*3/4, e.Power*5/4)
		case model.EffectBurn:
			d = g.Between(e.Power, e.Power*3/2)
		default:
			continue
		}
		out = append(out, Tick{Kind: e.Kind, Damage: core.Max(1, d)})
	}
	return out
}

// Advance runs the clock down one round and returns the surviving conditions
// along with the kinds that have just run out.
//
// Anything marked Forever is untouched: a weakening lasts the fight, which is
// what the original behaviour was and what the flavour text implies.
func Advance(list model.Effects) (model.Effects, []model.EffectKind) {
	var expired []model.EffectKind
	out := list[:0]
	for _, e := range list {
		if e.Rounds == model.Forever {
			out = append(out, e)
			continue
		}
		e.Rounds--
		if e.Rounds <= 0 {
			expired = append(expired, e.Kind)
			continue
		}
		out = append(out, e)
	}
	return out, expired
}

// Cleanse strips every harmful condition and returns what it removed, so the
// screen can name what somebody has just stopped suffering from. Blessings are
// left alone: an antidote that also cancelled the encouragement you were given
// would be a trap rather than a cure.
func Cleanse(list model.Effects) (model.Effects, []model.EffectKind) {
	var removed []model.EffectKind
	out := list[:0]
	for _, e := range list {
		if e.Kind.Harmful() {
			removed = append(removed, e.Kind)
			continue
		}
		out = append(out, e)
	}
	return out, removed
}

// RollAffliction decides whether a monster's attack leaves something behind,
// and what.
func RollAffliction(g *core.RNG, a *model.Affliction) (model.Effect, bool) {
	if a == nil || a.Kind == "" || !g.Chance(float64(a.Chance)/100) {
		return model.Effect{}, false
	}
	rounds := a.Rounds
	if rounds == 0 {
		rounds = 3
	}
	return model.Effect{Kind: a.Kind, Power: core.Max(1, a.Power), Rounds: rounds}, true
}
