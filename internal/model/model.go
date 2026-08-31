// Package model holds the persistent data shapes: characters, monsters, gear,
// and loot. The stat vocabulary is inherited from the 1994 Slycrel character
// record (strength/dexterity/speed/psyche/fame/honor/faith) because the combat
// math in package rules is a direct port and expects those fields.
package model

import (
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
)

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

	// Fame is what the deeds are worth; Renown is how well the face is known.
	// Two numbers rather than one because they come apart, and the corners
	// where they disagree are the interesting ones — see rules.Read.
	Fame   int `json:"fame"`
	Renown int `json:"renown"`
	Honor  int `json:"honor"`
	Faith  int `json:"faith"`
	Shame  int `json:"shame"` // the other end of Fame: deeds that travelled badly

	Coins int64 `json:"coins"`

	Weapon Weapon `json:"weapon"`
	Armor  Armor  `json:"armor"`
	Shield Shield `json:"shield,omitempty"`
	Charm  Charm  `json:"charm,omitempty"`
	// Sidearm is a second weapon on the off arm, which is the thief's answer
	// to a slot the other two classes fill with a plank or a talisman.
	//
	// It shares the off arm with Shield and the two are mutually exclusive:
	// EquipAs and Character.Equip both put one down to pick the other up, and
	// TestTheOffArmHoldsOneThing says so. Absent in every save written before
	// it existed, which unmarshals to a weapon with no name, which Worn reads
	// as an empty hand — the honest answer, and the safe one.
	Sidearm Weapon `json:"sidearm,omitempty"`
	Bag     []Item `json:"bag"`
	// Carried is equipment in the pack rather than on the body: bought, found,
	// or taken off. See Carried in gear.go for why it exists.
	Carried []Carried `json:"carried,omitempty"`

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

// TakeStack removes an entire stack from the bag and returns it with its count
// intact, where TakeItem peels one off the top.
//
// Both exist because both are wanted: using a potion is one potion, and selling
// a pile of owl pellets is the pile.
func (c *Character) TakeStack(i int) (Item, bool) {
	if i < 0 || i >= len(c.Bag) {
		return Item{}, false
	}
	it := c.Bag[i]
	c.Bag = append(c.Bag[:i], c.Bag[i+1:]...)
	if it.Count < 1 {
		it.Count = 1
	}
	return it, true
}

// WeaponKind is what sort of implement something is, which is the whole of who
// is allowed to carry it.
//
// The empty string means "anyone", and that is not a placeholder: it is what
// every weapon in every save written before classes had lanes unmarshals to,
// and it is the only answer that leaves an old character still holding what
// they were holding. Bare Hands is authored that way on purpose too.
type WeaponKind string

const (
	WeaponAny     WeaponKind = ""
	WeaponDagger  WeaponKind = "dagger"  // short, quick, and never the biggest number
	WeaponBlade   WeaponKind = "blade"   // swords and axes
	WeaponBlunt   WeaponKind = "blunt"   // maces, hammers, a table leg
	WeaponPolearm WeaponKind = "polearm" // reach, and both hands
	WeaponFocus   WeaponKind = "focus"   // wands and staves: a spell's weapon
)

// Weapon is a wielded implement. Strike feeds the damage roll and Quality is
// the durability/refinement rating carried over from the original armory.
type Weapon struct {
	Name    string     `json:"name"`
	Kind    WeaponKind `json:"kind,omitempty"`
	Strike  int        `json:"strike"`
	Range   int        `json:"range"` // 0 = melee
	Cost    int        `json:"cost"`
	Quality int        `json:"quality"`
	Verb    string     `json:"verb"` // "slash", "bash", "thwack"
	Tier    int        `json:"tier"` // shop stocking band
	Icon    string     `json:"icon"`
	Affix   *Affix     `json:"affix,omitempty"`
	// Hands is how many are needed. Zero reads as one, which is the answer
	// every weapon in every old save gives and the right one for all of them.
	Hands int `json:"hands,omitempty"`
	// Focus is spell strike: what a wand or staff is *for*. It feeds the
	// magnitude of everything cast and the free bolt a focus attacks with, and
	// it is zero on anything with an edge, which is the trade — a caster's
	// weapon is a bad thing to hit somebody with.
	Focus int `json:"focus,omitempty"`
	// Extra is what a weapon does besides land: a dagger's dexterity, mostly.
	Extra *Bonus `json:"extra,omitempty"`
}

// TwoHanded reports whether the weapon occupies the shield arm as well.
func (w Weapon) TwoHanded() bool { return w.Hands >= 2 }

// Worn reports whether this is a weapon at all, as opposed to an empty slot.
func (w Weapon) Worn() bool { return w.Name != "" }

// OffHand is a weapon on a shelf that is for the other arm.
//
// It is a wrapper rather than a flag because the only thing that needs to tell
// the difference is a type switch — the shop deciding which slot a purchase
// goes into. Everything else treats it as the Weapon it embeds, which it is.
// A flag would have to be checked at every one of those places and would be
// forgotten at one of them.
type OffHand struct{ Weapon }

// SidearmShare is how much of an off-hand weapon's strike reaches the blow.
//
// Half, and the half is the whole design. The point of putting a weapon on the
// off arm rather than a shield is that it is priced on the *weapon* table,
// which steps about five a band where the sidearm table steps two — that is
// the one legitimate way past TestShieldsStaySecondaryToArmour, since a dagger
// is not a shield and the rule that holds shields under half the body armour
// of their band has nothing to say about it.
//
// Full strike would make it strictly the best thing on the arm at every level
// and turn the slot back into a ladder. Half puts a tier-4 sidearm at about
// what the spiked shield of the same band is worth, which is where a choice
// lives: EXCHANGE prices strike near a point of win rate each at the top of
// the game, the spiked lane carries six of it, and half of a tier-4 dagger is
// in the same neighbourhood.
//
// It is a function rather than a constant beside the field because the halving
// has to happen in exactly one place. Two copies is how a stat ends up counted
// once on the character sheet and twice in the damage roll.
func SidearmShare(w Weapon) int {
	if !w.Worn() {
		return 0
	}
	return w.Strike / 2
}

// ArmorKind is how heavy a coat is, which is the whole of who can move in it.
// The empty string means "anyone", for the same reason WeaponKind's does.
type ArmorKind string

const (
	ArmorAny   ArmorKind = ""
	ArmorCloth ArmorKind = "cloth"
	ArmorLight ArmorKind = "light"
	ArmorHeavy ArmorKind = "heavy"
)

// Armor is worn protection.
type Armor struct {
	Name    string    `json:"name"`
	Kind    ArmorKind `json:"kind,omitempty"`
	Defense int       `json:"defense"`
	Cost    int       `json:"cost"`
	Quality int       `json:"quality"`
	Verb    string    `json:"verb"` // "absorbs", "deflects"
	Tier    int       `json:"tier"`
	Icon    string    `json:"icon"`
	Affix   *Affix    `json:"affix,omitempty"`
	// Extra is what a coat does besides stop things. Robes carry ward, which
	// is the whole reason a caster is not simply worse off for being unable to
	// wear plate.
	Extra *Bonus `json:"extra,omitempty"`
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
	// ItemCamp is a night in the field: half of both pools back, for the whole
	// company, without walking anywhere. It is the answer to a level at the top
	// of the game costing eleven round trips to an inn.
	ItemCamp ItemKind = "camp"
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

// SplitName divides a creature's name into what it is and what sort of one.
//
// Every name in the tables is written one of two ways. "Crab, Territorial" is a
// species and a characterisation, and the characterisation is the joke; "Owl
// That Knows" is a whole phrase and the phrase is the joke. The comma is the
// only thing that tells them apart, so a name with no comma is all species and
// no epithet, which is the honest answer for the second kind.
//
// The group letter stays with the species. Two crabs are "Crab A" and "Crab B",
// and a head that dropped the letter would label them identically at the exact
// moment the player is choosing between them.
//
// It lives here rather than beside the battle screen that draws it because the
// writing needs it too: prose says the species, the portrait says both.
func SplitName(full string) (head, tail string) {
	letter := ""
	if n := len(full); n > 2 && full[n-2] == ' ' && full[n-1] >= 'A' && full[n-1] <= 'Z' {
		letter, full = " "+full[n-1:], full[:n-2]
	}
	if i := strings.Index(full, ", "); i >= 0 {
		return full[:i] + letter, full[i+2:]
	}
	return full + letter, ""
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
	// Dead means out of the fight, which is not the same as killed: something
	// that ran is Dead with hit points left. Fled says which, because the two
	// are worth different amounts and reading it off the hit points alone was
	// what made a runner worth nothing at all.
	Dead bool
	Fled bool

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

// Short is what a creature is called in prose: the species and its group
// letter, without the epithet.
//
// The transcript says "Wolf B bites Bosk" because that is what a person
// watching would say. It used to say "Wolf, Deeply Unimpressed B bites Bosk",
// and a taunt naming the same creature twice came out as "Wolf, Deeply
// Unimpressed has watched you fight before. Wolf, Deeply Unimpressed was not
// impressed then either." The epithet is not lost — it is under the portrait,
// permanently, which is where a joke can be read rather than scrolled past.
func (m *Monster) Short() string {
	head, _ := SplitName(m.Name)
	return head
}

// SameKind reports whether two live creatures are the same thing, scaled the
// same way — which is what makes them interchangeable to somebody choosing a
// target, and therefore what makes them stack.
//
// Hit points are deliberately not part of it. Spawn jitters HP by an eighth, so
// two wolves rolled from one definition at one level are never quite equal
// there and would never stack if they had to be. Everything else Spawn and the
// encounter shapes touch — offence, guard, ward, speed — is deterministic given
// the definition, the level and the scaling, so it is the part that says
// whether two creatures are the same *kind* of thing rather than two rolls of
// one.
//
// That distinction is doing real work. A pack scales every body it draws, so a
// pack of six wolves is six of one kind. An escort scales its caster and leaves
// its guards alone, so a magical wolf leading two ordinary ones is two kinds
// wearing one name — and they must not stack, because the player choosing
// between them is choosing between different creatures.
func SameKind(a, b *Monster) bool {
	if a == nil || b == nil || a.Def == nil || b.Def == nil {
		return false
	}
	return a.Def.ID == b.Def.ID &&
		a.Offense == b.Offense && a.Defense == b.Defense &&
		a.Ward == b.Ward && a.Speed == b.Speed
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
