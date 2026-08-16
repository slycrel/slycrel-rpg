// Package rules is the game's arithmetic: damage, initiative, levelling, and
// loot. The core formulas are a direct port of the 1994 Pascal / late-90s C++
// Slycrel (via the Go recreation in ../new-slycrel), which is why the damage
// curve has that specific "low level feels swingy, high level feels flat"
// shape. Keeping the port intact means the balance is already playtested by
// several hundred BBS users, thirty years ago.
//
// Everything here is pure: no I/O, no globals, RNG passed in. That makes the
// balance testable and the world reproducible from a seed.
package rules

import (
	"math"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// BaseXP is the experience needed to reach level 2.
const BaseXP = 50

// LevelDifficulty scales the level curve. The original exposed this as a
// sysop knob; 4 is the value the old game shipped with.
const LevelDifficulty = 4

// XPForLevel returns the total experience required to reach the given level.
// Original: NextLevelUp() — f(x) = difficulty*x^3 + f(x-1), f(1) = BaseXP.
func XPForLevel(level int) int64 {
	if level <= 1 {
		return BaseXP
	}
	total := int64(BaseXP)
	for l := 2; l <= level; l++ {
		total += int64(LevelDifficulty) * int64(l) * int64(l) * int64(l)
	}
	return total
}

// NewCharacter rolls a starting adventurer of the given class.
func NewCharacter(g *core.RNG, name string, class model.Class) *model.Character {
	c := &model.Character{
		Name:  name,
		Class: class,
		Level: 1,
		Coins: int64(g.Between(15, 40)),
	}
	switch class {
	case model.ClassFighter:
		c.MaxHP = g.Between(18, 24)
		c.Strength = g.Between(9, 13)
		c.Dexterity = g.Between(5, 9)
		c.Speed = g.Between(6, 9)
		c.MaxPsyche = g.Between(2, 4)
	case model.ClassThief:
		c.MaxHP = g.Between(14, 19)
		c.Strength = g.Between(6, 10)
		c.Dexterity = g.Between(9, 13)
		c.Speed = g.Between(9, 13)
		c.MaxPsyche = g.Between(3, 6)
	case model.ClassMage:
		c.MaxHP = g.Between(11, 16)
		c.Strength = g.Between(4, 8)
		c.Dexterity = g.Between(6, 10)
		c.Speed = g.Between(6, 10)
		c.MaxPsyche = g.Between(8, 12)
	}
	c.HP = c.MaxHP
	c.Psyche = c.MaxPsyche
	c.Honor = g.Between(0, 3)
	c.Faith = g.Between(0, 3)
	return c
}

// LevelUp advances one level, applying the original per-class growth rolls.
// Original: GiveNewLevel() in Sly_CommonGuildUnit.cp.
func LevelUp(g *core.RNG, c *model.Character) {
	c.Level++
	switch c.Class {
	case model.ClassFighter:
		c.MaxHP += g.Between(4, 5+c.Level)
		c.Speed++
		c.Strength += g.Between(1, 2)
		c.Dexterity += g.Between(0, 1)
		c.MaxPsyche += g.Between(0, 1)
	case model.ClassMage:
		c.MaxHP += g.Between(2, 3+c.Level)
		c.Speed++
		c.Strength += g.Between(0, 1)
		c.Dexterity += g.Between(0, 1)
		c.MaxPsyche += g.Between(1, 3)
	case model.ClassThief:
		c.MaxHP += g.Between(3, 4+c.Level)
		c.Speed += g.Between(1, 2)
		c.Strength += g.Between(1, 2)
		c.Dexterity += g.Between(1, 2)
		c.MaxPsyche += g.Between(0, 2)
	}
	c.Fame++
	c.HP = c.MaxHP
	c.Psyche = c.MaxPsyche
}

// PendingLevels reports how many level-ups the character has banked.
func PendingLevels(c *model.Character) int {
	n := 0
	for c.TotalXP >= XPForLevel(c.Level+1+n) {
		n++
		if n > 50 { // guard against a runaway XP award
			break
		}
	}
	return n
}

// PlayerDamage rolls the damage a character deals to a monster.
// Original: DoDamage(true) in Sly_TextCombatUnit.cp. The two-branch shape is
// deliberate — below level 5 the spread is wide and strength-dominated, above
// it the curve flattens so gear and monster defense start to matter.
func PlayerDamage(g *core.RNG, c *model.Character, m *model.Monster) int {
	str := float64(c.Strength)
	strike := float64(c.Weapon.Strike)

	var dmg int
	if c.Level <= 4 {
		lo := int(math.Round((str*0.75 + strike) * 1.25))
		hi := int(math.Round((str/0.75 + strike) * 1.35))
		dmg = g.Between(lo, hi) - m.Def.Defense
		if dmg < 2 {
			dmg = g.Intn(3) // the original's mercy floor: 0-2
		}
	} else {
		lo := int(math.Round(str/2 + strike))
		hi := int(math.Round((str/2 + strike) * 1.25))
		dmg = g.Between(lo, hi) - m.Def.Defense
	}
	return core.Max(0, dmg)
}

// MonsterDamage rolls the damage a monster deals to a character.
// Original: DoDamage(false).
func MonsterDamage(g *core.RNG, c *model.Character, m *model.Monster) int {
	lo := int(float64(m.Def.Offense) * 0.35)
	hi := int(float64(m.Def.Offense) * 1.35)
	return core.Max(0, g.Between(lo, hi)-c.Armor.Defense)
}

// SpellDamage rolls a spell's magnitude, scaled by the caster's psyche pool so
// mages keep pace without needing a separate spellpower stat.
func SpellDamage(g *core.RNG, c *model.Character, s model.Spell) int {
	base := float64(s.Power) + float64(c.MaxPsyche)*0.6 + float64(c.Level)*0.8
	lo := int(base * 0.8)
	hi := int(base * 1.3)
	return core.Max(1, g.Between(lo, hi))
}

// Initiative reports whether the player acts before the monster.
// Original: RollInitiative() — a speed difference plus a d4 fudge.
func Initiative(g *core.RNG, playerSpeed, monsterSpeed int) bool {
	return playerSpeed-monsterSpeed+g.Intn(4) >= 0
}

// Defending halves incoming damage, matching the original's Block modifier.
func Defending(dmg int) int { return dmg / 2 }

// FleeChance returns the probability of escaping, from the speed gap. Slower
// parties can still get away; it just costs them.
func FleeChance(playerSpeed, monsterSpeed int) float64 {
	base := 0.45 + float64(playerSpeed-monsterSpeed)*0.05
	return core.ClampF(base, 0.10, 0.92)
}

// XPAward totals the experience for a defeated group, with a small bonus for
// fighting outnumbered.
func XPAward(monsters []*model.Monster) int64 {
	var total int64
	for _, m := range monsters {
		total += int64(m.Def.XP)
	}
	if len(monsters) > 1 {
		total += total * int64(len(monsters)-1) / 10
	}
	return total
}

// CoinAward rolls the gold dropped by a defeated group.
func CoinAward(g *core.RNG, monsters []*model.Monster) int64 {
	var total int64
	for _, m := range monsters {
		c := m.Def.Coins
		total += int64(g.Between(c/2, c+c/2))
	}
	return total
}

// RollLoot resolves a monster's drop table into item ids and counts.
func RollLoot(g *core.RNG, defs []model.Drop) map[string]int {
	out := map[string]int{}
	for _, d := range defs {
		if g.Intn(100) >= d.Chance {
			continue
		}
		lo, hi := d.Min, d.Max
		if lo <= 0 {
			lo = 1
		}
		if hi < lo {
			hi = lo
		}
		out[d.Item] += g.Between(lo, hi)
	}
	return out
}

// MonsterAction is what a monster decides to do on its turn.
type MonsterAction int

const (
	MonAttack MonsterAction = iota
	MonDefend
	MonFlee
)

// ChooseMonsterAction picks a monster's move. Roughly the original's 50-sided
// table: mostly attack, sometimes turtle, and bolt when nearly dead.
func ChooseMonsterAction(g *core.RNG, m *model.Monster) MonsterAction {
	if m.HPFrac() < 0.15 && g.Chance(0.35) {
		return MonFlee
	}
	if roll := g.Between(1, 50); roll > 38 {
		return MonDefend
	}
	return MonAttack
}

// Disposition is the nine-way read on how a fight is going, used to select
// flavor text. Ported from GetMainCombatText.
type Disposition int

// The nine states, named for (monster, player) condition.
const (
	DispBothStrong Disposition = iota
	DispMonWeakYouStrong
	DispMonStrongYouWeak
	DispBothHurt
	DispBothWeak
	DispMonHurtYouStrong
	DispMonStrongYouHurt
	DispMonWeakYouHurt
	DispMonHurtYouWeak
)

// GetDisposition classifies the fight from both sides' HP fractions.
func GetDisposition(playerFrac, monsterFrac float64) Disposition {
	band := func(f float64) int {
		switch {
		case f >= 0.70:
			return 2 // strong
		case f >= 0.35:
			return 1 // hurt
		default:
			return 0 // weak
		}
	}
	p, m := band(playerFrac), band(monsterFrac)
	switch {
	case p == 2 && m == 2:
		return DispBothStrong
	case p == 2 && m == 1:
		return DispMonHurtYouStrong
	case p == 2 && m == 0:
		return DispMonWeakYouStrong
	case p == 1 && m == 2:
		return DispMonStrongYouHurt
	case p == 1 && m == 1:
		return DispBothHurt
	case p == 1 && m == 0:
		return DispMonWeakYouHurt
	case p == 0 && m == 2:
		return DispMonStrongYouWeak
	case p == 0 && m == 1:
		return DispMonHurtYouWeak
	default:
		return DispBothWeak
	}
}
