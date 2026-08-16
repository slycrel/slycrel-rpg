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
	Shield Shield `json:"shield,omitempty"`
	Charm  Charm  `json:"charm,omitempty"`
	Bag    []Item `json:"bag"`

	// Ally marks a hired companion rather than the player's own hero.
	//
	// A companion is an ordinary Character in every other respect — same rolls,
	// same growth curve, same spell list — because a hireling that used a
	// reduced stat block would need its own balance pass, and there is no
	// reason for one. What differs is ownership: the pack, the coin and the
	// quest log belong to the hero, and a companion is never commanded.
	Ally bool `json:"ally,omitempty"`
	// Cut is the percentage of every coin haul the companion takes off the top,
	// which is the standing price of not swinging the sword yourself.
	Cut int `json:"cut,omitempty"`
	// Blood is a strain of non-human ancestry. Empty means an ordinary person,
	// which most people are. It shifts the stat line and unlocks a technique no
	// hero can learn, and it is the reason the hireling was going cheap.
	Blood MonsterKind `json:"blood,omitempty"`

	Sprite   string `json:"sprite"`   // asset key for the overworld/local sprite
	Portrait string `json:"portrait"` // asset key for the battle portrait

	// Active is what the character is currently suffering or enjoying. It is
	// fight-scoped and never written to a save: a battle cannot be saved from,
	// and a poison that survived reloading would be a bug.
	Active Effects `json:"-"`
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
//
// Healing does not raise the dead. Left as a plain clamp, it lifted anybody on
// zero straight back up, which made every healing potion a resurrection and
// left the revive items and Reknit with nothing to do. Standing somebody up is
// a different act with its own cost, and it sets hit points directly.
func (c *Character) Heal(n int) int {
	if !c.Alive() {
		return 0
	}
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
	Affix   *Affix `json:"affix,omitempty"`
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
	Affix   *Affix `json:"affix,omitempty"`
}

// ItemKind sorts consumables from junk from quest goods.
type ItemKind string

const (
	ItemHeal    ItemKind = "heal"
	ItemPsyche  ItemKind = "psyche"
	ItemBuff    ItemKind = "buff"
	ItemRevive  ItemKind = "revive"  // stands a fallen party member back up
	ItemCure    ItemKind = "cure"    // strips the conditions somebody picked up
	ItemTrinket ItemKind = "trinket" // sellable junk, mostly a joke delivery system
	ItemKey     ItemKind = "key"
)

// UsedOnSomeone reports whether the item is applied to a person, and therefore
// needs somebody chosen before it does anything. A trinket is waved at the
// problem and a key is turned in a lock; neither picks a target.
func (k ItemKind) UsedOnSomeone() bool {
	switch k {
	case ItemHeal, ItemPsyche, ItemBuff, ItemRevive, ItemCure:
		return true
	}
	return false
}

// WantsTheFallen reports whether the item is for someone who is down rather
// than someone who is standing, which is the opposite target list.
func (k ItemKind) WantsTheFallen() bool { return k == ItemRevive }

// Item is a bag entry.
type Item struct {
	Name  string   `json:"name"`
	Kind  ItemKind `json:"kind"`
	Power int      `json:"power"` // HP restored, psyche restored, or stat delta
	Value int      `json:"value"` // base sale price
	Count int      `json:"count"`
	Desc  string   `json:"desc"`
	Icon  string   `json:"icon"`
	// Effect is what a buff item leaves behind, and for how long. The battle
	// screen used to tell the two buff items apart by comparing against the
	// literal string "Suspicious Pollen", which meant renaming a potion
	// silently changed what it did.
	Effect EffectKind `json:"effect,omitempty"`
	Rounds int        `json:"rounds,omitempty"`
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

	// Ward is armour against magic, and Defense is armour against everything
	// else. Keeping them separate is the whole matchup axis: a plated knight
	// stops swords and nothing else, a bound spirit is the other way round, and
	// which one you are looking at decides whether you should be swinging or
	// casting.
	//
	// Magic used to be untaxed — SpellDamage subtracted nothing at all — so a
	// caster's output overtook a fighter's at level 7 and nearly tripled it by
	// level 11. This is the missing side of that.
	Ward int `json:"ward"`
	// Magic reports that this creature's attacks are magical, so they are
	// reduced by the target's Ward instead of their Defense. A player in full
	// plate is not protected from a curse by the plate.
	Magic bool `json:"magic,omitempty"`
	XP    int  `json:"xp"`
	Coins int  `json:"coins"`

	// Flavor drives the combat log. AttackWith/AttackVerb combine as
	// "The Gutter Troll <verb> you with its <with>".
	AttackWith []string `json:"attackWith"`
	AttackVerb []string `json:"attackVerb"`
	DefendWith string   `json:"defendWith"`
	Taunt      []string `json:"taunt"`
	Death      []string `json:"death"`

	// Inflicts is what a landed hit from this creature leaves behind: a spider
	// leaves venom, a salamander leaves you on fire. Nil for the majority that
	// simply hit you.
	Inflicts *Affliction `json:"inflicts,omitempty"`

	Sprite string `json:"sprite"` // battle art key
	Loot   []Drop `json:"loot"`
}

// Affliction is a condition a monster's attack can apply, and how often.
type Affliction struct {
	Kind   EffectKind `json:"kind"`
	Power  int        `json:"power"`
	Rounds int        `json:"rounds"`
	Chance int        `json:"chance"` // percentage, 0-100
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
	Ward    int
	Speed   int
	XP      int
	Coins   int

	// Active is what the monster is currently suffering. Monsters are spawned
	// fresh for every encounter and never stored, so this needs no exclusion.
	Active Effects
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
		// Ward scales with defense, being the same idea pointed elsewhere.
		Ward:  scaled(d.Ward, 0.09),
		Speed: scaled(d.Speed, 0.05),
		// Rewards track the effort, or fighting a scaled-up creature would
		// pay the same as the easy version of it.
		XP:    scaled(d.XP, 0.28),
		Coins: scaled(d.Coins, 0.22),
	}
}
