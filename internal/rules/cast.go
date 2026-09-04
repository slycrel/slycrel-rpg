package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// How long a condition laid on a creature runs for.
//
// These were literals inside the battle screen's switch and nowhere else, which
// is why the simulator could not have agreed with them even if it had tried:
// there was nothing to agree with. A poison lasting four rounds is a balance
// number — it is most of what separates a poison from a smaller instant hit —
// and a balance number that lives in a draw call is a balance number nothing
// can price.
const (
	poisonRounds = 4
	burnRounds   = 3
	stunRounds   = 1
)

// Landed is what one technique did to one creature.
//
// It exists so that the battle screen and the simulator can stop being two
// implementations of the same rules. They were: `castOnFoes` resolved six kinds
// in a switch beside its log lines, `SimulateGroupAs` resolved four of them in a
// switch beside its counters, and the two agreed only because one person wrote
// both on the same afternoon. The four the simulator was missing are exactly the
// four the report has been unable to measure — a weaken, a stun, a poison and a
// burn were unreachable not because the policy would not pick them but because
// there was nothing in `internal/rules` that knew what picking one would do.
type Landed struct {
	// Damage is what should come off the creature's hit points, already past
	// its ward. It is returned rather than applied because the two callers
	// bookkeep a blow differently — the screen raises a floater, feeds the
	// talisman and collapses a stack; the simulator counts it — and neither of
	// those is a rule.
	Damage int
	// Drained is what the caster recovers from it, which only a drain returns.
	Drained int
}

// CastAtFoe resolves a technique against one creature.
//
// It applies the condition, if the technique leaves one, and returns the blow
// for the caller to land. The asymmetry is deliberate and is the whole shape of
// the split: a condition going onto a creature is the same act wherever it
// happens, and damage coming off one is not.
//
// The caller decides *who* — a technique over the whole field is this function
// once per creature, which is also how ward works out right, since a fireball
// that reaches three things is resisted three times rather than once.
func CastAtFoe(g *core.RNG, c *model.Character, s model.Spell, m *model.Monster) Landed {
	lay := func(k model.EffectKind, power, rounds int) {
		m.Active = Apply(m.Active, model.Effect{Kind: k, Power: power, Rounds: rounds})
	}
	switch s.Kind {
	case model.SpellDamage, model.SpellPact:
		return Landed{Damage: AfterWard(SpellDamage(g, c, s), m.Ward)}
	case model.SpellDrain:
		d := AfterWard(SpellDamage(g, c, s), m.Ward)
		return Landed{Damage: d, Drained: d / 2}
	case model.SpellWeaken, model.SpellSap:
		// A sap is half an exchange: this is the half that lands on the
		// creature, and the blessing the caster wears goes on once for the
		// whole cast rather than once per target. The caller owns that half,
		// because "once per cast" is not something a per-target function can
		// say.
		lay(model.EffectWeaken, s.Power, model.Forever)
	case model.SpellStun:
		lay(model.EffectStun, 1, stunRounds)
	case model.SpellPoison:
		lay(model.EffectPoison, s.Power, poisonRounds)
	case model.SpellBurn:
		lay(model.EffectBurn, s.Power, burnRounds)
	}
	return Landed{}
}

// CastOnCaster is the half of a two-sided technique that lands on whoever cast
// it, once for the whole cast however many creatures it reached.
//
// Once per cast rather than once per target is the design and not an
// optimisation: pointing a sap at three things would otherwise be three
// blessings, and a technique whose value scales with how outnumbered you are is
// the wrong shape for the one that exists to even a fight up.
func CastOnCaster(c *model.Character, s model.Spell) {
	switch s.Kind {
	case model.SpellSap:
		c.Active = Apply(c.Active, model.Effect{
			Kind: model.EffectBless, Power: s.Power, Rounds: model.Forever,
		})
	case model.SpellPact:
		c.Active = Apply(c.Active, model.Effect{
			Kind: model.EffectWeaken, Power: PactCost(s), Rounds: model.Forever,
		})
	}
}

// CastOnAlly resolves a technique aimed at somebody on your own side, and
// reports what it was worth so the caller can say so.
//
// Revive returns the hit points it stood them up with; a heal returns what it
// restored, which is not the roll — somebody two points from full takes two.
func CastOnAlly(g *core.RNG, c *model.Character, s model.Spell, target *model.Character) int {
	switch s.Kind {
	case model.SpellHeal:
		if !target.Alive() {
			return 0
		}
		return target.Heal(SpellDamage(g, c, s))
	case model.SpellRevive:
		if target.Alive() {
			return 0
		}
		// ReviveAmount, not a roll: standing up is a fraction of the target's
		// own maximum rather than of the caster's power, and the rule already
		// existed. Writing a second one here would have been this file's own
		// point missed on the first page of it.
		target.HP = ReviveAmount(target, s.Power)
		return target.HP
	case model.SpellBless:
		if !target.Alive() {
			return 0
		}
		target.Active = Apply(target.Active, model.Effect{
			Kind: model.EffectBless, Power: s.Power, Rounds: model.Forever,
		})
	}
	return 0
}
