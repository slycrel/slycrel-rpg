// Package model holds the persistent data shapes: characters, monsters, gear,
// and loot. The stat vocabulary is inherited from the 1994 Slycrel character
// record (strength/dexterity/speed/psyche/fame/honor/faith) because the combat
// math in package rules is a direct port and expects those fields.
package model

import "github.com/slycrel/slycrel-rpg/internal/core"

// Class is a character occupation. The three originals survive; the stat
// growth curves in rules.LevelUp are keyed off these.
type Class string

const (
	ClassFighter Class = "Fighter"
	ClassThief   Class = "Thief"
	ClassMage    Class = "Mage"
)

// AllClasses is the character-creation roster.
var AllClasses = []Class{ClassFighter, ClassThief, ClassMage}

// Blurb is the one-line pitch shown on the class select screen.
func (c Class) Blurb() string {
	switch c {
	case ClassFighter:
		return "Hits things. Owns one book, has not opened it."
	case ClassThief:
		return "Faster than you. Already holding your coin purse."
	case ClassMage:
		return "Fragile, insufferable, occasionally sets the sky on fire."
	}
	return ""
}

// Character is a player-controlled adventurer.
type Character struct {
	Name    string `json:"name"`
	Class   Class  `json:"class"`
	Epithet string `json:"epithet"` // generated, e.g. "the Regrettable"

	Level     int   `json:"level"`
	TotalXP   int64 `json:"totalXP"`
	SpendXP   int64 `json:"spendXP"` // unspent XP, the old game's training currency
	HP        int   `json:"hp"`
	MaxHP     int   `json:"maxHP"`
	Psyche    int   `json:"psyche"` // spell points
	MaxPsyche int   `json:"maxPsyche"`

	Strength  int `json:"strength"`
	Dexterity int `json:"dexterity"`
	Speed     int `json:"speed"`

	Fame  int `json:"fame"`
	Honor int `json:"honor"`
	Faith int `json:"faith"`
	Shame int `json:"shame"` // new: accrued from the dumb choices, gates some content

	Coins int64 `json:"coins"`

	Weapon Weapon `json:"weapon"`
	Armor  Armor  `json:"armor"`
	Bag    []Item `json:"bag"`

	Sprite   string `json:"sprite"`   // asset key for the overworld/local sprite
	Portrait string `json:"portrait"` // asset key for the battle portrait
}

// Alive reports whether the character can still act.
func (c *Character) Alive() bool { return c.HP > 0 }

// HPFrac returns current HP as a fraction of max, clamped to [0,1].
func (c *Character) HPFrac() float64 {
	if c.MaxHP <= 0 {
		return 0
	}
	return core.ClampF(float64(c.HP)/float64(c.MaxHP), 0, 1)
}

// PsycheFrac returns current spell points as a fraction of max.
func (c *Character) PsycheFrac() float64 {
	if c.MaxPsyche <= 0 {
		return 0
	}
	return core.ClampF(float64(c.Psyche)/float64(c.MaxPsyche), 0, 1)
}

// Heal restores up to n hit points and returns the amount actually restored.
func (c *Character) Heal(n int) int {
	before := c.HP
	c.HP = core.Clamp(c.HP+n, 0, c.MaxHP)
	return c.HP - before
}

// AddItem puts it in the bag, stacking with an existing entry when possible.
func (c *Character) AddItem(it Item) {
	for i := range c.Bag {
		if c.Bag[i].Name == it.Name {
			c.Bag[i].Count += it.Count
			return
		}
	}
	if it.Count <= 0 {
		it.Count = 1
	}
	c.Bag = append(c.Bag, it)
}

// TakeItem removes one of the item at index i, dropping the stack when empty.
func (c *Character) TakeItem(i int) (Item, bool) {
	if i < 0 || i >= len(c.Bag) {
		return Item{}, false
	}
	it := c.Bag[i]
	c.Bag[i].Count--
	if c.Bag[i].Count <= 0 {
		c.Bag = append(c.Bag[:i], c.Bag[i+1:]...)
	}
	it.Count = 1
	return it, true
}

// Weapon is a wielded implement. Strike feeds the damage roll and Quality is
// the durability/refinement rating carried over from the original armory.
type Weapon struct {
	Name    string `json:"name"`
	Strike  int    `json:"strike"`
	Range   int    `json:"range"` // 0 = melee
	Cost    int    `json:"cost"`
	Quality int    `json:"quality"`
	Verb    string `json:"verb"` // "slash", "bash", "thwack"
	Tier    int    `json:"tier"` // shop stocking band
	Icon    string `json:"icon"`
}

// Armor is worn protection.
type Armor struct {
	Name    string `json:"name"`
	Defense int    `json:"defense"`
	Cost    int    `json:"cost"`
	Quality int    `json:"quality"`
	Verb    string `json:"verb"` // "absorbs", "deflects"
	Tier    int    `json:"tier"`
	Icon    string `json:"icon"`
}

// ItemKind sorts consumables from junk from quest goods.
type ItemKind string

const (
	ItemHeal    ItemKind = "heal"
	ItemPsyche  ItemKind = "psyche"
	ItemBuff    ItemKind = "buff"
	ItemTrinket ItemKind = "trinket" // sellable junk, mostly a joke delivery system
	ItemKey     ItemKind = "key"
)

// Item is a bag entry.
type Item struct {
	Name  string   `json:"name"`
	Kind  ItemKind `json:"kind"`
	Power int      `json:"power"` // HP restored, psyche restored, or stat delta
	Value int      `json:"value"` // base sale price
	Count int      `json:"count"`
	Desc  string   `json:"desc"`
	Icon  string   `json:"icon"`
}

// MonsterKind is a creature family. Damage types and some jokes key off it.
type MonsterKind string

const (
	KindBeast     MonsterKind = "beast"
	KindHumanoid  MonsterKind = "humanoid"
	KindUndead    MonsterKind = "undead"
	KindOoze      MonsterKind = "ooze"
	KindFey       MonsterKind = "fey"
	KindDemon     MonsterKind = "demon"
	KindConstruct MonsterKind = "construct"
	KindAberrant  MonsterKind = "aberrant"
)

// MonsterDef is the immutable template loaded from data/monsters.
type MonsterDef struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Kind    MonsterKind `json:"kind"`
	Level   int         `json:"level"`
	HP      int         `json:"hp"`
	Offense int         `json:"offense"`
	Defense int         `json:"defense"`
	Speed   int         `json:"speed"`
	XP      int         `json:"xp"`
	Coins   int         `json:"coins"`

	// Flavor drives the combat log. AttackWith/AttackVerb combine as
	// "The Gutter Troll <verb> you with its <with>".
	AttackWith []string `json:"attackWith"`
	AttackVerb []string `json:"attackVerb"`
	DefendWith string   `json:"defendWith"`
	Taunt      []string `json:"taunt"`
	Death      []string `json:"death"`

	Sprite string `json:"sprite"` // battle art key
	Loot   []Drop `json:"loot"`
}

// Drop is a weighted loot entry; Chance is a percentage in [0,100].
type Drop struct {
	Item   string `json:"item"`
	Chance int    `json:"chance"`
	Min    int    `json:"min"`
	Max    int    `json:"max"`
}

// Monster is a live combatant instantiated from a MonsterDef.
//
// It carries its own stats rather than reading the template, because a monster
// met deep in the world is a scaled-up version of the same creature. Reading
// Def directly is what left a biome's roster capping the difficulty of every
// encounter in it.
type Monster struct {
	Def   *MonsterDef
	Name  string // may be uniquified, e.g. "Gutter Troll B"
	HP    int
	MaxHP int
	Dead  bool

	Offense int
	Defense int
	Speed   int
	XP      int
	Coins   int
}

// HPFrac returns current HP as a fraction of max.
func (m *Monster) HPFrac() float64 {
	if m.MaxHP <= 0 {
		return 0
	}
	return core.ClampF(float64(m.HP)/float64(m.MaxHP), 0, 1)
}

// Spawn instantiates a monster from its template, scaled to the level of the
// encounter it is appearing in.
//
// Only hit points used to scale, which meant a level-3 goblin met at level 10
// was a punching bag that hit like a level-3 goblin. Every combat stat scales
// now, at rates that keep the shape of a fight: health grows fastest so fights
// lengthen, offense more slowly so they do not become lethal, defense slowest
// of all because it multiplies against every hit.
func (d *MonsterDef) Spawn(g *core.RNG, level int) *Monster {
	over := float64(core.Max(0, level-d.Level))
	scaled := func(base int, rate float64) int {
		return core.Max(0, int(float64(base)*(1+rate*over)))
	}

	hp := scaled(d.HP, 0.18)
	hp += g.Between(-hp/8, hp/8)
	if hp < 1 {
		hp = 1
	}
	return &Monster{
		Def: d, Name: d.Name, HP: hp, MaxHP: hp,
		Offense: scaled(d.Offense, 0.12),
		Defense: scaled(d.Defense, 0.09),
		Speed:   scaled(d.Speed, 0.05),
		// Rewards track the effort, or fighting a scaled-up creature would
		// pay the same as the easy version of it.
		XP:    scaled(d.XP, 0.28),
		Coins: scaled(d.Coins, 0.22),
	}
}
