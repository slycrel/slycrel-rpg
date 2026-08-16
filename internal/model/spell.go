package model

// SpellKind is what a spell does when it lands.
type SpellKind string

const (
	SpellDamage SpellKind = "damage"
	SpellHeal   SpellKind = "heal"
	SpellDrain  SpellKind = "drain"  // damage, healing the caster for half
	SpellWeaken SpellKind = "weaken" // cuts the target's offense for the fight
	SpellStun   SpellKind = "stun"   // target loses its next turn
)

// SpellTarget selects who a spell hits.
type SpellTarget string

const (
	TargetOne  SpellTarget = "one"
	TargetAll  SpellTarget = "all"
	TargetSelf SpellTarget = "self"
)

// Spell is a castable ability. Fighters and thieves get them too — they are
// just called "techniques" on screen and cost the same psyche pool, because a
// single resource is one fewer bar to explain.
type Spell struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Class  Class       `json:"class"` // empty means every class can learn it
	Level  int         `json:"level"` // character level required
	Cost   int         `json:"cost"`  // psyche spent
	Power  int         `json:"power"` // base magnitude before stat scaling
	Kind   SpellKind   `json:"kind"`
	Target SpellTarget `json:"target"`
	Icon   string      `json:"icon"`
	Cast   string      `json:"cast"` // flavor line, "%s" takes the caster name
}

// Known reports whether c has access to this spell.
func (s Spell) Known(c *Character) bool {
	if s.Class != "" && s.Class != c.Class {
		return false
	}
	return c.Level >= s.Level
}
