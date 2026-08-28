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
	// SpellSap takes the edge off them and puts it on you: the same magnitude
	// of weakness on the target and blessing on the caster. One technique, both
	// halves of the exchange, which is the shape a swing does not have.
	SpellSap SpellKind = "sap"
	// SpellPact is the other direction — it hits far harder than its band pays
	// for, and the caster wears the weakness for the rest of the fight. The
	// house rule that everything which gives must take, written as a technique
	// rather than as an item.
	SpellPact SpellKind = "pact"
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
	// VFX names the effect played where the technique lands, overriding the
	// default for its kind. Optional, and left empty for most: the kind is
	// right nearly always, and a per-spell key is one more art name to keep in
	// step with a manifest.
	VFX string `json:"vfx,omitempty"`
	// Cast is the flavour line, naming the caster as {A} — the same placeholder
	// the rest of the writing uses.
	//
	// It was documented as taking a "%s" and passed through fmt.Sprintf, which
	// no line in the table has ever contained: every technique cast since the
	// foundation commit printed its raw "{A}" followed by "%!(EXTRA string=Bosk)"
	// into the transcript. Nothing caught it because nothing reads the combat
	// log except a person looking at a frame, which is the failure mode this
	// project keeps finding and the reason -demo exists.
	Cast string `json:"cast"`
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
