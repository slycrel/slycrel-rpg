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

// Damage blend band. The original switched formulas outright at level 5, and
// simulation showed exactly what that does: a fighter's output fell from 14.6
// damage a round to 8.7 overnight, and a mage's win rate went from 98% to 66%.
//
// Both formulas are kept — they give the game its shape, wide and
// strength-dominated early, flatter and gear-dependent later — but the crossing
// is spread over these levels instead of happening between two fights.
const (
	blendFrom = 4
	blendTo   = 8
)

// damageBlend is how far a level is across the crossing, in [0,1].
func damageBlend(level int) float64 {
	switch {
	case level <= blendFrom:
		return 0
	case level >= blendTo:
		return 1
	default:
		return float64(level-blendFrom) / float64(blendTo-blendFrom)
	}
}

// PlayerDamage rolls the damage a character deals to a monster.
// Original: DoDamage(true) in Sly_TextCombatUnit.cp.
//
// The two formulas' bounds are interpolated and then rolled once, rather than
// rolling each and averaging: averaging two rolls would quietly halve the
// spread mid-band and make those levels feel oddly consistent.
func PlayerDamage(g *core.RNG, c *model.Character, m *model.Monster) int {
	str := float64(c.Strength)
	strike := float64(c.Weapon.Strike)

	// Early: swingy and driven by raw strength.
	//
	// Toned down from the original's (str*0.75 .. str/0.75) spread. That was
	// generous enough that levels one to four were a walkover — 99% win rates
	// and fights over in under two rounds — and it forced the late formula to
	// be enormous just to avoid a drop at the crossing. Pulling the early
	// numbers down fixes both ends at once.
	earlyLo := (str*0.55 + strike) * 1.25
	earlyHi := (str*0.95 + strike) * 1.35
	// Late: flatter, so armour and weapon quality carry the difference.
	//
	// The original halved strength here. Simulation showed why that cannot
	// stand on its own: with a fixed weapon a character's damage fell 46%
	// between levels 4 and 8 purely from the formula, and the only reason the
	// game looked balanced was that the simulated player bought a new weapon
	// every three levels. That made shopping mandatory to stay level. A larger
	// coefficient keeps the flatter late-game shape while letting gear be an
	// improvement rather than a tax.
	lateLo := str*0.65 + strike
	lateHi := (str*0.65 + strike) * 1.25

	t := damageBlend(c.Level)
	lo := int(math.Round(earlyLo + (lateLo-earlyLo)*t))
	hi := int(math.Round(earlyHi + (lateHi-earlyHi)*t))

	dmg := g.Between(lo, hi) - m.Defense

	// The original's mercy floor, retired as the late formula takes over: past
	// the band a well-armoured monster is supposed to be able to shrug a hit.
	if t < 1 && dmg < 2 {
		dmg = g.Intn(3)
	}
	return core.Max(0, dmg)
}

// Swing is the outcome of one attack, resolved in full.
type Swing struct {
	Miss   bool
	Crit   bool
	Damage int
}

// PlayerAttack resolves a character's attack, hit roll and all. Buffs are the
// temporary bonuses an item may have granted for the fight.
//
// This lives here rather than in the battle screen so the balance simulator
// exercises exactly the arithmetic the game plays. A simulator with its own
// copy of the hit roll measures a game nobody is playing.
func PlayerAttack(g *core.RNG, c *model.Character, m *model.Monster, buffStr, buffDex int) Swing {
	// Miss chance from the speed/dexterity gap, floored and capped so neither
	// side ever becomes untouchable.
	miss := core.ClampF(0.06+float64(m.Speed-c.Dexterity-buffDex)*0.012, 0.03, 0.32)
	if g.Chance(miss) {
		return Swing{Miss: true}
	}

	sw := Swing{Crit: g.Chance(0.07 + float64(c.Dexterity)/400)}
	sw.Damage = PlayerDamage(g, c, m) + buffStr
	if sw.Crit {
		sw.Damage = sw.Damage*3/2 + 2
	}
	return sw
}

// MonsterDamage rolls the damage a monster deals to a character.
// Original: DoDamage(false).
func MonsterDamage(g *core.RNG, c *model.Character, m *model.Monster) int {
	lo := int(float64(m.Offense) * 0.35)
	hi := int(float64(m.Offense) * 1.35)
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
		total += int64(m.XP)
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
		c := m.Coins
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

// --- companions -----------------------------------------------------------

// AllyMoveKind is what a companion does with its turn.
type AllyMoveKind int

const (
	AllySwing AllyMoveKind = iota // attack with the weapon
	AllyCast                      // use the technique in AllyMove.Spell
	AllyGuard                     // brace, halving what lands this round
)

// AllyMove is a companion's decision for one turn.
//
// Companions are not commanded — the player drives the hero and the hirelings
// have their own opinions about the rest of it — so this is the whole of their
// tactical brain.
type AllyMove struct {
	Kind  AllyMoveKind
	Spell model.Spell // set only when Kind is AllyCast
	// Ally is who a party-side technique is aimed at. It is nil for anything
	// pointed at the monsters, and for a self-targeted technique it is the
	// caster, so the battle screen never has to work out who was meant.
	Ally *model.Character
}

// ChooseAllyMove picks a companion's action for the round.
//
// The order of business is triage first: someone on the floor, then someone
// about to be, then hurting whatever is causing it. That ordering is the whole
// of the difference between a companion and a second sword — it is why a party
// medic is worth a slot even though healing does no damage.
//
// The attack half runs the same policy the balance simulator plays, so a
// companion's contribution can be reasoned about from numbers that already
// exist rather than guessed at.
func ChooseAllyMove(g *core.RNG, c *model.Character, spells []model.Spell, party []*model.Character) AllyMove {
	// Somebody is down and this one can stand them up.
	if s, ok := affordable(c, spells, model.SpellRevive); ok {
		if target := mostBroken(party, false); target != nil {
			return AllyMove{Kind: AllyCast, Spell: s, Ally: target}
		}
	}

	// Somebody is failing. Heal the worst of them, which may well be the
	// caster: a medic who patches everyone but themselves falls over first.
	if s, ok := bestHeal(c, spells); ok {
		if target := healTarget(c, s, party); target != nil && target.HPFrac() < healBelow {
			return AllyMove{Kind: AllyCast, Spell: s, Ally: target}
		}
	}

	// Nothing urgent. Hit something, unless a technique beats the weapon.
	if s, ok := bestAttack(c, spells); ok {
		return AllyMove{Kind: AllyCast, Spell: s}
	}

	// Fresh, safe, and nothing worth casting at the enemy: this is when a
	// blessing is worth the psyche rather than a round wasted while people are
	// bleeding. Occasionally, so a companion is not permanently mid-speech.
	if s, ok := affordable(c, spells, model.SpellBless); ok && partyIsHealthy(party) && g.Chance(0.3) {
		return AllyMove{Kind: AllyCast, Spell: s, Ally: blessTarget(c, s, party)}
	}

	// Badly hurt with nothing left to cast: sometimes cover up rather than
	// trade blows it cannot afford. Rarely enough that a companion never
	// becomes a wall that just stands there.
	if c.HPFrac() < 0.25 && g.Chance(0.35) {
		return AllyMove{Kind: AllyGuard}
	}
	return AllyMove{Kind: AllySwing}
}

// healBelow is how hurt somebody has to be before healing them beats acting.
// Above it, a heal is psyche spent to top up a scratch.
const healBelow = 0.45

// healTarget picks who a heal is aimed at: the worst-off member it can reach.
// A self-only technique can only ever reach one person.
func healTarget(caster *model.Character, s model.Spell, party []*model.Character) *model.Character {
	if s.Target == model.TargetSelf {
		return caster
	}
	return mostBroken(party, true)
}

// blessTarget picks who a blessing lands on. Self-only goes to the caster;
// anything else goes to whoever hits hardest, since a blessing is offense.
func blessTarget(caster *model.Character, s model.Spell, party []*model.Character) *model.Character {
	if s.Target == model.TargetSelf || s.Target == model.TargetAll {
		return caster
	}
	best := caster
	for _, c := range party {
		if c.Alive() && c.Strength > best.Strength {
			best = c
		}
	}
	return best
}

// mostBroken returns the party member in the worst state. With standing set it
// looks for the lowest fraction among those still up; with it clear it looks
// for anybody who is down.
func mostBroken(party []*model.Character, standing bool) *model.Character {
	var worst *model.Character
	for _, c := range party {
		if c.Alive() != standing {
			continue
		}
		if worst == nil || c.HPFrac() < worst.HPFrac() {
			worst = c
		}
	}
	return worst
}

// partyIsHealthy reports whether everyone is upright and in one piece, which is
// when there is room in the round for something other than triage.
func partyIsHealthy(party []*model.Character) bool {
	for _, c := range party {
		if !c.Alive() || c.HPFrac() < 0.8 {
			return false
		}
	}
	return len(party) > 0
}

// affordable returns the strongest known technique of a kind that the caster
// can currently pay for.
func affordable(c *model.Character, spells []model.Spell, kind model.SpellKind) (model.Spell, bool) {
	var best model.Spell
	found := false
	for _, s := range spells {
		if s.Kind != kind || s.Cost > c.Psyche || !s.Known(c) {
			continue
		}
		if !found || s.Power > best.Power {
			best, found = s, true
		}
	}
	return best, found
}

// HireCost is the fee a companion of the given level and ancestry asks up
// front. It grows with the square of level so that a late hire is a considered
// purchase rather than pocket change, and lands in the same band as a weapon of
// the tier the hireling arrives carrying.
//
// A lineage takes a percentage off, because nobody else is bidding for someone
// who is visibly part troll. That discount is the reward for the trade-offs in
// the lineage table, and it is why the cheap hireling on the corner is the one
// with the interesting abilities.
func HireCost(level int, blood model.MonsterKind) int64 {
	base := int64(60 + level*level*6)
	if l, ok := model.LineageOf(blood); ok {
		base -= base * int64(l.Discount) / 100
	}
	return core.Max64(1, base)
}

// Skim is the share of a coin haul a companion takes off the top.
func Skim(coins int64, cut int) int64 {
	if cut <= 0 || coins <= 0 {
		return 0
	}
	return coins * int64(cut) / 100
}

// Recruit rolls a companion of the given class and ancestry, levelled to
// match. Gear is the caller's business — it comes out of the content tables,
// which this package deliberately cannot see.
func Recruit(g *core.RNG, name string, class model.Class, blood model.MonsterKind, level int) *model.Character {
	c := BuildCharacter(g, class, level)
	c.Name = name
	c.Ally = true
	c.Cut = g.Between(8, 18)
	c.Blood = blood
	ApplyLineage(c)
	// Rested and ready. ApplyLineage only ever clamps downward — it has no way
	// to know whether the character it is adjusting was at full — so a lineage
	// that raises the maximum would otherwise hand over somebody who arrives
	// already short of the hit points they are being paid for.
	c.HP, c.Psyche = c.MaxHP, c.MaxPsyche
	// The pack, the purse and the errands stay with the hero.
	c.Coins, c.Bag, c.SpendXP = 0, nil, 0
	return c
}

// ApplyLineage folds a character's ancestry into their stat line.
//
// It runs once, after levelling, rather than being re-applied per level: the
// deltas are a description of the person, not of their training, so a part-ooze
// is stout at level one and proportionally as stout at level fifteen. Hit
// points shift by percentage for that reason and the rest are flat, because the
// stats themselves only creep upward a point or two a level.
func ApplyLineage(c *model.Character) {
	l, ok := model.LineageOf(c.Blood)
	if !ok {
		return
	}
	c.MaxHP = core.Max(1, c.MaxHP+c.MaxHP*l.HPPct/100)
	c.Strength = core.Max(1, c.Strength+l.Strength)
	c.Dexterity = core.Max(1, c.Dexterity+l.Dexterity)
	c.Speed = core.Max(1, c.Speed+l.Speed)
	c.MaxPsyche = core.Max(0, c.MaxPsyche+l.Psyche)
	c.HP = core.Clamp(c.HP, 0, c.MaxHP)
	c.Psyche = core.Clamp(c.Psyche, 0, c.MaxPsyche)
}

// ReviveAmount is how much health standing somebody back up gives them.
//
// Power is the item's or technique's rating, read as a percentage of maximum,
// and the floor of one exists so that a revive is never the no-op of standing
// somebody up dead. It is deliberately a fraction rather than a full heal:
// getting back up is the expensive part, and being fit to fight afterwards is a
// separate purchase.
func ReviveAmount(c *model.Character, power int) int {
	if power <= 0 {
		power = 25
	}
	return core.Clamp(c.MaxHP*power/100, 1, c.MaxHP)
}

// RescueFee is what the hirelings take for carrying a dead employer to the
// nearest town and paying somebody to argue with the situation.
//
// It is a share of the purse rather than a flat sum, so it hurts proportionally
// at every level and can never be unpayable — a run must not be able to end
// because the player could not afford to survive. What it costs when the purse
// is empty is time and a point of Shame, which is the correct currency.
func RescueFee(coins int64) int64 {
	if coins <= 0 {
		return 0
	}
	return core.Max64(1, coins*40/100)
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

// --- simulation -----------------------------------------------------------

// FightResult is the outcome of a simulated encounter.
type FightResult struct {
	Won         bool
	Rounds      int
	DamageDealt int
	DamageTaken int
	HPLeft      int
}

// SimulateFight plays an encounter to the end, mutating nothing the caller owns.
//
// The policy is competent rather than optimal: heal when badly hurt, cast the
// best affordable attack spell while the psyche lasts, swing otherwise. No
// potions — a player who brings supplies should be doing better than this, not
// rescued by it. Passing no spells measures the pure weapon floor.
//
// Casting matters to the measurement, not just to realism: a mage judged on
// swinging a stick looks broken when it is only being played wrong.
func SimulateFight(g *core.RNG, c *model.Character, defs []*model.MonsterDef, level, maxRounds int, spells []model.Spell) FightResult {
	// The character is spent, not copied: hit points and psyche carry out of
	// the fight so a run of encounters can be simulated on one rest. Callers
	// wanting an isolated fight pass a copy.
	sim := c

	mons := make([]*model.Monster, 0, len(defs))
	for _, d := range defs {
		mons = append(mons, d.Spawn(g, level))
	}

	var res FightResult
	for res.Rounds = 1; res.Rounds <= maxRounds; res.Rounds++ {
		living := livingMonsters(mons)
		if len(living) == 0 {
			res.Won = true
			break
		}

		fastest := 0
		for _, m := range living {
			if m.Speed > fastest {
				fastest = m.Speed
			}
		}
		playerFirst := Initiative(g, sim.Speed, fastest)

		hurt := func(target *model.Monster, dmg int) {
			target.HP = core.Max(0, target.HP-dmg)
			res.DamageDealt += dmg
			if target.HP == 0 {
				target.Dead = true
			}
		}
		strike := func() {
			target := living[0]
			if target.Dead {
				return
			}
			if s, ok := bestSpell(sim, spells); ok {
				sim.Psyche -= s.Cost
				switch s.Kind {
				case model.SpellHeal:
					sim.HP = core.Clamp(sim.HP+SpellDamage(g, sim, s), 0, sim.MaxHP)
				case model.SpellDrain:
					d := SpellDamage(g, sim, s)
					hurt(target, d)
					sim.HP = core.Clamp(sim.HP+d/2, 0, sim.MaxHP)
				default:
					d := SpellDamage(g, sim, s)
					if s.Target == model.TargetAll {
						for _, m := range living {
							if !m.Dead {
								hurt(m, d)
							}
						}
					} else {
						hurt(target, d)
					}
				}
				return
			}
			sw := PlayerAttack(g, sim, target, 0, 0)
			if sw.Miss {
				return
			}
			hurt(target, sw.Damage)
		}
		monsterTurns := func() {
			for _, m := range living {
				if m.Dead || sim.HP <= 0 {
					continue
				}
				switch ChooseMonsterAction(g, m) {
				case MonFlee:
					m.Dead = true // leaves the fight; earns the player nothing
				case MonDefend:
					// no attack this turn
				default:
					dmg := MonsterDamage(g, sim, m)
					sim.HP = core.Max(0, sim.HP-dmg)
					res.DamageTaken += dmg
					// Some creatures leave a condition behind, which is worth
					// more than their damage roll suggests. The simulator has
					// to catch that or the report would rate a venomous spider
					// by its bite alone and call the swamp safer than it is.
					if e, ok := RollAffliction(g, m.Def.Inflicts); ok && sim.HP > 0 {
						sim.Active = Apply(sim.Active, e)
					}
				}
			}
		}

		if playerFirst {
			strike()
			monsterTurns()
		} else {
			monsterTurns()
			strike()
		}

		// Conditions bite at the end of the round on both sides, exactly as
		// the battle screen resolves them.
		for _, m := range living {
			if m.Dead {
				continue
			}
			for _, t := range TickDamage(g, m.Active) {
				hurt(m, t.Damage)
			}
			m.Active, _ = Advance(m.Active)
		}
		for _, t := range TickDamage(g, sim.Active) {
			sim.HP = core.Max(0, sim.HP-t.Damage)
			res.DamageTaken += t.Damage
		}
		sim.Active, _ = Advance(sim.Active)

		if sim.HP <= 0 {
			break
		}
	}
	// Nothing survives the fight it was applied in.
	sim.Active = nil

	if len(livingMonsters(mons)) == 0 && sim.HP > 0 {
		res.Won = true
	}
	res.HPLeft = sim.HP
	return res
}

// bestHeal returns the strongest affordable technique that restores hit points.
func bestHeal(c *model.Character, spells []model.Spell) (model.Spell, bool) {
	return affordable(c, spells, model.SpellHeal)
}

// bestAttack returns the strongest affordable technique worth casting at the
// enemy, or false when swinging the weapon would do at least as well.
//
// That last condition is what keeps a fighter's technique a finisher rather
// than a replacement for the sword, and it is why a low-level attack spell
// quietly retires once the weapon outgrows it instead of being cast forever.
func bestAttack(c *model.Character, spells []model.Spell) (model.Spell, bool) {
	var attack model.Spell
	found := false
	for _, s := range spells {
		if s.Cost > c.Psyche || !s.Known(c) {
			continue
		}
		if s.Kind != model.SpellDamage && s.Kind != model.SpellDrain {
			continue
		}
		if !found || s.Power > attack.Power {
			attack, found = s, true
		}
	}
	if !found {
		return model.Spell{}, false
	}
	weapon := float64(c.Strength)/2 + float64(c.Weapon.Strike)
	spell := float64(attack.Power) + float64(c.MaxPsyche)*0.6 + float64(c.Level)*0.8
	if spell <= weapon {
		return model.Spell{}, false
	}
	return attack, true
}

// bestSpell picks what a competent solo player would cast this round: a heal
// when badly hurt, otherwise the strongest attack worth the psyche.
//
// This is the simulator's policy. It stays deliberately solo — the balance
// report measures one character against an encounter, and a heal it casts is
// necessarily on itself — while ChooseAllyMove layers party triage on top of
// the same two helpers. One set of "what is worth casting" rules, two callers
// with different amounts of company.
func bestSpell(c *model.Character, spells []model.Spell) (model.Spell, bool) {
	if heal, ok := bestHeal(c, spells); ok && c.HPFrac() < 0.3 {
		return heal, true
	}
	return bestAttack(c, spells)
}

func livingMonsters(mons []*model.Monster) []*model.Monster {
	out := mons[:0:0]
	for _, m := range mons {
		if !m.Dead {
			out = append(out, m)
		}
	}
	return out
}

// BuildCharacter rolls a character and levels it to the given level, which is
// how the game produces one; there is no other path to a level-N adventurer.
func BuildCharacter(g *core.RNG, class model.Class, level int) *model.Character {
	c := NewCharacter(g, "Subject", class)
	for c.Level < level {
		LevelUp(g, c)
	}
	return c
}
