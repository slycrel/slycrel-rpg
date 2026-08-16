package model

// SpellKind is what a spell does when it lands.
type SpellKind string

const (
	SpellDamage SpellKind = "damage"
	SpellHeal   SpellKind = "heal"
	SpellDrain  SpellKind = "drain"  // damage, healing the caster for half
	SpellWeaken SpellKind = "weaken" // cuts the target's offense for the fight
	SpellStun   SpellKind = "stun"   // target loses its next turn
	SpellBless  SpellKind = "bless"  // raises an ally's offense for the fight
	SpellRevive SpellKind = "revive" // stands a fallen ally back up
	SpellPoison SpellKind = "poison" // damage over several rounds
	SpellBurn   SpellKind = "burn"   // more damage over fewer rounds
)

// Side is which half of the battlefield an effect is aimed at.
type Side int

const (
	SideFoes Side = iota
	SideParty
)

// Side reports who a kind of effect is for.
//
// It is derived from the kind rather than stored alongside it, because the two
// can never disagree: there is no such thing as a heal aimed at a monster or a
// stun aimed at your own party. A data file able to express one would sooner or
// later contain one.
func (k SpellKind) Side() Side {
	switch k {
	case SpellHeal, SpellBless, SpellRevive:
		return SideParty
	}
	return SideFoes
}

// SpellTarget is how many of that side an effect reaches.
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
	// Blood restricts a technique to people with that strain of ancestry.
	// These are the things a hireling brings that no hero can learn, which is
	// most of the reason to take a part-monster on in the first place.
	Blood MonsterKind `json:"blood,omitempty"`
	Icon  string      `json:"icon"`
	Cast  string      `json:"cast"` // flavor line, "%s" takes the caster name
}

// Known reports whether c has access to this spell.
func (s Spell) Known(c *Character) bool {
	if s.Blood != "" && s.Blood != c.Blood {
		return false
	}
	if s.Class != "" && s.Class != c.Class {
		return false
	}
	return c.Level >= s.Level
}
