package model

import "strings"

// Bonus is a bundle of stat modifiers.
//
// One shape serves both halves of the equipment system: what a shield or a
// charm is worth, and what an affix adds on top. Keeping them the same type is
// what lets a character's total be a sum rather than four special cases, and it
// means adding a stat later is one field rather than five.
type Bonus struct {
	Strike    int `json:"strike,omitempty"`
	Defense   int `json:"defense,omitempty"`
	Strength  int `json:"strength,omitempty"`
	Dexterity int `json:"dexterity,omitempty"`
	Speed     int `json:"speed,omitempty"`
	Psyche    int `json:"psyche,omitempty"`
	// Ward is protection from magic, and it is the only defensive stat that
	// cannot be got from body armour. Steel does not stop a curse, so a
	// character who wants to be hard to burn has to spend a slot on it — which
	// is the point: generalising costs something.
	Ward int `json:"ward,omitempty"`
}

// Add returns the two bundles combined.
func (b Bonus) Add(o Bonus) Bonus {
	return Bonus{
		Strike:    b.Strike + o.Strike,
		Defense:   b.Defense + o.Defense,
		Strength:  b.Strength + o.Strength,
		Dexterity: b.Dexterity + o.Dexterity,
		Speed:     b.Speed + o.Speed,
		Psyche:    b.Psyche + o.Psyche,
		Ward:      b.Ward + o.Ward,
	}
}

// Empty reports whether the bundle does nothing at all.
func (b Bonus) Empty() bool { return b == Bonus{} }

// Affix is a suffix and what it does.
//
// Affixes are authored rather than rolled from a range, and every one gives
// with one hand and takes with the other — the same rule the lineages follow.
// A table of pure upgrades would make "is it affixed" the only question worth
// asking about a piece of gear, and there would be nothing to weigh.
type Affix struct {
	Suffix string `json:"suffix"`
	Bonus  Bonus  `json:"bonus"`
	Tier   int    `json:"tier"` // the gear band it starts appearing on
}

// Shield is what goes on the other arm.
type Shield struct {
	Name    string `json:"name"`
	Defense int    `json:"defense"`
	Cost    int    `json:"cost"`
	Tier    int    `json:"tier"`
	Verb    string `json:"verb"` // "catches", "turns"
	Icon    string `json:"icon"`
	Affix   *Affix `json:"affix,omitempty"`
	Extra   *Bonus `json:"extra,omitempty"` // a few shields do more than block
}

// Charm is anything worn that is neither weapon, armour nor shield: a pendant,
// a knucklebone, a licence somebody forged.
type Charm struct {
	Name  string `json:"name"`
	Bonus Bonus  `json:"bonus"`
	Cost  int    `json:"cost"`
	Tier  int    `json:"tier"`
	Desc  string `json:"desc"`
	Icon  string `json:"icon"`
	Affix *Affix `json:"affix,omitempty"`
}

// Affixable reports whether a name can carry a suffix without reading badly.
//
// A good many base names already end in a flourish — "Mace of Modest Ambition",
// "Scale of the Overconfident" — and bolting a second one on produces "Runed
// Maul of the Last Word of the Last Word", which is funny exactly once. Gear
// that already has a joke in its name keeps it and never gets affixed.
func Affixable(name string) bool { return !strings.Contains(name, " of ") }

// Titled returns a piece of gear's full name, affix included.
func titled(name string, a *Affix) string {
	if a == nil || a.Suffix == "" {
		return name
	}
	return name + " " + a.Suffix
}

// Titled returns the weapon's name with its affix.
func (w Weapon) Titled() string { return titled(w.Name, w.Affix) }

// Titled returns the armour's name with its affix.
func (a Armor) Titled() string { return titled(a.Name, a.Affix) }

// Titled returns the shield's name with its affix.
func (s Shield) Titled() string { return titled(s.Name, s.Affix) }

// Titled returns the charm's name with its affix.
func (c Charm) Titled() string { return titled(c.Name, c.Affix) }

// Worn reports whether the slot holds anything.
func (s Shield) Worn() bool { return s.Name != "" }

// Worn reports whether the slot holds anything.
func (c Charm) Worn() bool { return c.Name != "" }

// Gear totals everything the character is wearing that is not already folded
// into the base weapon and armour ratings: the shield, the charm, and every
// affix across all four slots.
//
// The base Strike and Defense stay where they are. Reading them through here
// as well would mean two paths to the same number and, sooner or later, one of
// them being updated alone.
func (c *Character) Gear() Bonus {
	var b Bonus
	if c.Weapon.Affix != nil {
		b = b.Add(c.Weapon.Affix.Bonus)
	}
	if c.Armor.Affix != nil {
		b = b.Add(c.Armor.Affix.Bonus)
	}
	if c.Shield.Worn() {
		b = b.Add(Bonus{Defense: c.Shield.Defense})
		if c.Shield.Extra != nil {
			b = b.Add(*c.Shield.Extra)
		}
		if c.Shield.Affix != nil {
			b = b.Add(c.Shield.Affix.Bonus)
		}
	}
	if c.Charm.Worn() {
		b = b.Add(c.Charm.Bonus)
		if c.Charm.Affix != nil {
			b = b.Add(c.Charm.Affix.Bonus)
		}
	}
	return b
}

// The effective stats, which are what the rules must read. A charm that raises
// strength has to raise the damage roll, or it is decoration with a price tag.
// Each is floored at one so that a bad affix can be a real cost without ever
// producing a character who cannot act.

// Str returns strength including everything worn.
func (c *Character) Str() int { return atLeast(c.Strength + c.Gear().Strength) }

// Dex returns dexterity including everything worn.
func (c *Character) Dex() int { return atLeast(c.Dexterity + c.Gear().Dexterity) }

// Spd returns speed including everything worn.
func (c *Character) Spd() int { return atLeast(c.Speed + c.Gear().Speed) }

// Strike returns the weapon rating including everything worn.
func (c *Character) Strike() int { return max0(c.Weapon.Strike + c.Gear().Strike) }

// Defense returns the armour rating including the shield and everything worn.
func (c *Character) Defense() int { return max0(c.Armor.Defense + c.Gear().Defense) }

// Ward returns protection from magic, which comes only from what is worn.
//
// There is no base ward from class or level on purpose. Defense arrives free
// with any coat, so a character is never wholly unarmoured; ward is bought or
// it is absent, which makes "am I protected against the other kind of damage"
// a question the player answers with a slot rather than one the sheet answers
// for them.
func (c *Character) Ward() int { return max0(c.Gear().Ward) }

// MaxPsy returns the psyche pool as the spell maths should read it.
//
// Worn psyche raises how hard a technique lands without granting more casts:
// the pool itself is spent and refilled at rests, and quietly raising its
// ceiling mid-run would leave a character standing at less than full for no
// reason they could see. A charm makes what you cast hit harder; a bed is still
// what gets you more of them.
func (c *Character) MaxPsy() int { return max0(c.MaxPsyche + c.Gear().Psyche) }

func atLeast(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// Carried is a piece of equipment in the pack rather than on the body.
// Exactly one field is set.
//
// Gear used to have nowhere to be except worn. Buying a sword put the old one
// "in the bin" — a 96-coin spear evaporating to make room for a 240-coin glaive
// — and a sword found in a chest was a yes/no question answered on the spot,
// with no way to carry it to a shop and no way to change your mind. That is how
// the 1994 original did it because a BBS door had eight equipment slots and no
// inventory to speak of; there is no reason to inherit the limitation along
// with the maths.
type Carried struct {
	Weapon *Weapon `json:"weapon,omitempty"`
	Armor  *Armor  `json:"armor,omitempty"`
	Shield *Shield `json:"shield,omitempty"`
	Charm  *Charm  `json:"charm,omitempty"`
}

// Empty reports a slot holding nothing, which should not occur but is cheaper
// to check than to prove impossible.
func (c Carried) Empty() bool {
	return c.Weapon == nil && c.Armor == nil && c.Shield == nil && c.Charm == nil
}

// Titled is the full name of whatever is in here, affix included.
func (c Carried) Titled() string {
	switch {
	case c.Weapon != nil:
		return c.Weapon.Titled()
	case c.Armor != nil:
		return c.Armor.Titled()
	case c.Shield != nil:
		return c.Shield.Titled()
	case c.Charm != nil:
		return c.Charm.Titled()
	}
	return ""
}

// Slot names which part of a character this would go on.
func (c Carried) Slot() string {
	switch {
	case c.Weapon != nil:
		return "weapon"
	case c.Armor != nil:
		return "armour"
	case c.Shield != nil:
		return "shield"
	case c.Charm != nil:
		return "charm"
	}
	return ""
}

// Cost is what it was worth new, which is what a sale is reckoned from.
func (c Carried) Cost() int {
	switch {
	case c.Weapon != nil:
		return c.Weapon.Cost
	case c.Armor != nil:
		return c.Armor.Cost
	case c.Shield != nil:
		return c.Shield.Cost
	case c.Charm != nil:
		return c.Charm.Cost
	}
	return 0
}

// Icon is the manifest key for the thing, for the lists it appears in.
func (c Carried) Icon() string {
	switch {
	case c.Weapon != nil:
		return c.Weapon.Icon
	case c.Armor != nil:
		return c.Armor.Icon
	case c.Shield != nil:
		return c.Shield.Icon
	case c.Charm != nil:
		return c.Charm.Icon
	}
	return ""
}

// Carry puts a piece of equipment in the pack.
func (c *Character) Carry(g Carried) {
	if g.Empty() {
		return
	}
	c.Carried = append(c.Carried, g)
}

// Equip puts the carried item at i on the body and returns whatever came off,
// which goes back in the pack. Swapping rather than replacing is the whole
// point: nothing is destroyed by changing your mind.
func (c *Character) Equip(i int) bool {
	if i < 0 || i >= len(c.Carried) {
		return false
	}
	g := c.Carried[i]
	c.Carried = append(c.Carried[:i], c.Carried[i+1:]...)

	switch {
	case g.Weapon != nil:
		old := c.Weapon
		c.Weapon = *g.Weapon
		if old.Name != "" {
			c.Carry(Carried{Weapon: &old})
		}
	case g.Armor != nil:
		old := c.Armor
		c.Armor = *g.Armor
		if old.Name != "" {
			c.Carry(Carried{Armor: &old})
		}
	case g.Shield != nil:
		old := c.Shield
		c.Shield = *g.Shield
		if old.Worn() {
			c.Carry(Carried{Shield: &old})
		}
	case g.Charm != nil:
		old := c.Charm
		c.Charm = *g.Charm
		if old.Worn() {
			c.Carry(Carried{Charm: &old})
		}
	default:
		return false
	}
	return true
}

// DropCarried removes the item at i without equipping it, for a sale.
func (c *Character) DropCarried(i int) (Carried, bool) {
	if i < 0 || i >= len(c.Carried) {
		return Carried{}, false
	}
	g := c.Carried[i]
	c.Carried = append(c.Carried[:i], c.Carried[i+1:]...)
	return g, true
}
