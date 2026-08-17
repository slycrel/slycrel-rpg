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
// Band is the inclusive range a stat is rolled in at level one.
type Band struct{ Lo, Hi int }

// Frac is where a value sits in its band, from 0 at the floor to 1 at the top.
// A band with no spread in it reads as the middle, since there was nothing to
// be lucky about.
func (b Band) Frac(v int) float64 {
	if b.Hi <= b.Lo {
		return 0.5
	}
	return core.ClampF(float64(v-b.Lo)/float64(b.Hi-b.Lo), 0, 1)
}

// StatBands is one class's five rolls.
type StatBands struct{ HP, Str, Dex, Spd, Psy Band }

// startingBands is what each class rolls at level one, and it is the only copy.
//
// NewCharacter reads it and so does the creation screen, which is the whole
// reason it is a table rather than a switch: colouring a roll good or bad means
// knowing what the roll could have been, and a second copy of these numbers
// would be a second copy that drifts. A Mage with eight Strength is a *good*
// Mage roll, and nothing outside this table knows that.
var startingBands = map[model.Class]StatBands{
	model.ClassFighter: {
		HP: Band{18, 24}, Str: Band{9, 13}, Dex: Band{5, 9}, Spd: Band{6, 9}, Psy: Band{2, 4},
	},
	model.ClassThief: {
		HP: Band{14, 19}, Str: Band{6, 10}, Dex: Band{9, 13}, Spd: Band{9, 13}, Psy: Band{3, 6},
	},
	model.ClassMage: {
		HP: Band{11, 16}, Str: Band{4, 8}, Dex: Band{6, 10}, Spd: Band{6, 10}, Psy: Band{8, 12},
	},
}

// startingCushion is what everybody gets on top of the rolled hit points.
//
// Level one is where the hit point pool is smallest and the tools to protect it
// are fewest, so the same unlucky opening exchange that costs a level-five
// character a quarter of their health ends the run at level one. The flat ten is
// deliberately not a percentage: it is worth a great deal at the start and
// rounds to nothing by the time it stops being needed.
const startingCushion = 10

// StartingCoins is the band a new character's purse is rolled in.
var StartingCoins = Band{45, 95}

// StartingBands reports what a class rolls, as the numbers a player is shown —
// so the hit point band already has the cushion folded into it and a caller
// never has to know the cushion exists.
func StartingBands(class model.Class) StatBands {
	b := startingBands[class]
	b.HP.Lo += startingCushion
	b.HP.Hi += startingCushion
	return b
}

func NewCharacter(g *core.RNG, name string, class model.Class) *model.Character {
	c := &model.Character{
		Name:  name,
		Class: class,
		Level: 1,
		// Enough to walk into the first shop and leave with something.
		//
		// It was 15 to 40, against 66 for the best weapon and coat of the first
		// band — so a new character could not reach the loadout every section
		// of the balance report calls "on curve", and mostly could not afford
		// either half of it. Being under-equipped on the first morning is the
		// intended shape; being unable to do anything about it is not.
		Coins: int64(g.Between(45, 95)),
	}
	b := startingBands[class]
	c.MaxHP = g.Between(b.HP.Lo, b.HP.Hi)
	c.Strength = g.Between(b.Str.Lo, b.Str.Hi)
	c.Dexterity = g.Between(b.Dex.Lo, b.Dex.Hi)
	c.Speed = g.Between(b.Spd.Lo, b.Spd.Hi)
	c.MaxPsyche = g.Between(b.Psy.Lo, b.Psy.Hi)
	c.MaxHP += startingCushion
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
	str := float64(c.Str())
	strike := float64(c.Strike())

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
	miss := core.ClampF(0.06+float64(m.Speed-c.Dex()-buffDex)*0.012, 0.03, 0.32)
	if g.Chance(miss) {
		return Swing{Miss: true}
	}

	sw := Swing{Crit: g.Chance(0.07 + float64(c.Dex())/400)}
	sw.Damage = PlayerDamage(g, c, m) + buffStr
	if sw.Crit {
		sw.Damage = sw.Damage*3/2 + 2
	}
	return sw
}

// MonsterDamage rolls the damage a monster deals to a character.
// Original: DoDamage(false).
//
// A magical attacker is stopped by the target's ward rather than their armour,
// which is what makes ward worth a slot. Ignoring it entirely stays a viable
// choice — nothing early in the game attacks that way — but it is a bet that
// gets worse the further out you go.
func MonsterDamage(g *core.RNG, c *model.Character, m *model.Monster) int {
	lo, hi := float64(m.Offense)*0.35, float64(m.Offense)*1.35
	guard := c.Defense()
	if m.Def != nil && m.Def.Magic {
		// A magical attack is weaker before it is resisted, because it is
		// going to be resisted by a great deal less. Going through armour
		// instead of into it is worth something, and if it were worth nothing
		// extra as well then every magical attacker would simply be a stronger
		// one: against a player wearing no ward at all — which the game allows,
		// and which the on-curve build does — an unscaled magical hit landed
		// for about two and a half times what the same creature's fists would
		// have, and the choice to skip ward stopped being a risk and became a
		// wall. The player who does buy ward comes out ahead of where armour
		// would have left them, which is the trade being worth making.
		lo, hi = lo*magicBite, hi*magicBite
		guard = c.Ward()
	}
	return core.Max(0, g.Between(int(lo), int(hi))-guard)
}

// magicBite is how hard a magical attack hits before resistance, relative to
// the same creature swinging. Tuned against the DANGER section rather than
// picked: it is the value that keeps an unwarded character inside the brief at
// every level while still leaving ward clearly worth a charm slot.
const magicBite = 0.62

// SpellDamage rolls a spell's magnitude, scaled by the caster's psyche pool so
// mages keep pace without needing a separate spellpower stat.
//
// This is the raw magnitude and nothing is subtracted from it, because the same
// roll is what a heal restores. Damage spells go through AfterWard on the way
// to a target; healing must not.
func SpellDamage(g *core.RNG, c *model.Character, s model.Spell) int {
	base := float64(s.Power) + float64(c.MaxPsy())*0.6 + float64(c.Level)*0.8
	lo := int(base * 0.8)
	hi := int(base * 1.3)
	return core.Max(1, g.Between(lo, hi))
}

// AfterWard reduces a magical hit by the target's resistance to magic.
//
// It floors at one rather than zero, unlike armour against a sword. A warded
// creature is meant to be the wrong target for a caster, not an invulnerable
// one: a mage who has walked into a room full of warded things should be having
// a bad time, not an unwinnable one, and the answer is supposed to be "draw
// your sword" rather than "reload".
func AfterWard(raw, ward int) int { return core.Max(1, raw-ward) }

// --- the false retreat ---------------------------------------------------

// feintLevel is when a thief works out that a rout can be sold rather than run.
const feintLevel = 4

// feintBonus is what a blow lands for when the target has committed to a
// pursuit, as a multiple of an ordinary swing.
const feintBonus = 2.2

// FeintIsWorthIt reports whether a false retreat could plausibly turn the fight
// rather than merely decorate the loss.
//
// Measured against what the blow would actually land for, not against a
// fraction of the target's health. A fraction reads the same at every level and
// it is not the same: at level 13 a creature sitting at 65% is still forty hit
// points away from dying and a doubled swing does not close that, so the thief
// was spending its escape on a gesture — plus 12.9 points of death for plus 1.9
// of victory. Against something actually within reach the same trade runs about
// one death per win, which is a gamble worth being offered.
func FeintIsWorthIt(c *model.Character, m *model.Monster) bool {
	if m == nil || m.Dead {
		return false
	}
	// The late-game damage midpoint, less what the target's armour takes off
	// it, then scaled by the bonus. Leaving armour out of the estimate is why
	// this misfired worst against the plated things in the mountains:
	// PlayerDamage subtracts Defense before the multiplier touches anything, so
	// an estimate that ignored it called a creature reachable when three such
	// blows would not have finished it, and the thief spent its escape on a
	// gesture.
	reach := (float64(c.Str())*0.65 + float64(c.Strike()) - float64(m.Defense)) * feintBonus
	if reach <= 0 {
		return false
	}
	return float64(m.HP) <= reach*1.1
}

// feintPunish is what the target hits back for when it does not buy the act,
// which is what makes this a gamble and not simply a better attack.
const feintPunish = 1.6

// CanFeint reports whether a character has the false retreat.
//
// It is the thief's alone and always will be. Fleeing pays nothing at all — no
// experience, no coin, no drop, and the fight was still fought — so the class
// whose survival plan is leaving is the class whose plan is to come away with
// nothing. This is the way out of that, and it is a way out only they have.
func CanFeint(c *model.Character) bool {
	return c != nil && c.Class == model.ClassThief && c.Level >= feintLevel
}

// feintFloor is the worst odds a competent player would take this at.
//
// The chance is carried by dexterity, so a level-four thief is selling the act
// at about three in ten and eating a heavier hit for the other seven. Offering
// the move that early is fine; taking it that early is not, and a simulator
// that took every gamble on offer reported the trick as a straight downgrade —
// win rates fell five to eight points through the middle of the game. The move
// arrives when the thief does and becomes worth using when their hands do.
const feintFloor = 0.5

// FeintChance is the odds of selling the retreat.
//
// Deliberately below FleeChance for the same character against the same
// creature. A feint that worked as often as a real escape would not be a
// gamble, it would be the correct move every time and the flee button would be
// decoration. Dexterity carries it rather than speed: this is a lie told with
// footwork, not a race.
func FeintChance(c *model.Character, monsterSpeed int) float64 {
	base := 0.30 + float64(c.Dex()-monsterSpeed)*0.035
	return core.ClampF(base, 0.10, 0.70)
}

// FeintDamage rolls the blow that lands when the act is bought. The target has
// turned its back on somebody it believed to be running away.
func FeintDamage(g *core.RNG, c *model.Character, m *model.Monster) int {
	return core.Max(1, int(float64(PlayerDamage(g, c, m))*feintBonus))
}

// FeintPunish scales the answer a creature gives when it does not fall for it.
func FeintPunish(dmg int) int { return int(float64(dmg) * feintPunish) }

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

// fleeWhenAlone is how much of the usual bolting instinct survives being the
// last one standing.
//
// Something that runs while its friends are still swinging has somewhere to
// run to. The last one in the room does not, and more to the point a fight
// that ends because the only thing left in it walked off is a fight that ends
// in an anticlimax — which is what it felt like at levels one to three, where
// an encounter is one or two creatures and the whole thing could evaporate
// after the player had done all the work.
//
// Damped rather than forbidden. A cornered animal bolting is worth keeping;
// it just should not be the ordinary way a low-level fight finishes.
const fleeWhenAlone = 0.3

// ChooseMonsterAction picks a monster's move. Roughly the original's 50-sided
// table: mostly attack, sometimes turtle, and bolt when nearly dead.
//
// alone says whether this is the last one still in the fight, which is the only
// thing outside the monster itself that its nerve depends on.
func ChooseMonsterAction(g *core.RNG, m *model.Monster, alone bool) MonsterAction {
	bolt := 0.35
	if alone {
		bolt *= fleeWhenAlone
	}
	if m.HPFrac() < 0.15 && g.Chance(bolt) {
		return MonFlee
	}
	if roll := g.Between(1, 50); roll > 38 {
		return MonDefend
	}
	return MonAttack
}

// RoutedXP is what driving something off is worth next to putting it down.
//
// Not nothing, which is what it used to be. A monster only runs below fifteen
// percent health, so by the time it goes the player has done nearly all of the
// work and the fight ends with the reward withheld on a coin flip the player
// had no part in. Half the experience says you did most of it; the missing half
// says you did not finish it.
//
// Its coins come across in full and are handled at the call site rather than
// here, because that is not a discount — a creature that turns and runs is a
// creature that is not stopping to pick anything up.
func RoutedXP(full int64) int64 { return full / 2 }

// --- companions -----------------------------------------------------------

// AllyMoveKind is what a companion does with its turn.
type AllyMoveKind int

const (
	AllySwing AllyMoveKind = iota // attack with the weapon
	AllyCast                      // use the technique in AllyMove.Spell
	AllyGuard                     // brace, halving what lands this round
	AllyUse                       // drink something out of their own pack
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
	// Item indexes the companion's own bag, for AllyUse.
	Item int
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

	// No technique for it, but something in their own pack. A companion never
	// reaches into the hero's bag — what they drink is what they were given or
	// bought, which is the whole reason to supply one.
	if c.HPFrac() < healBelow {
		if i, ok := bestRestorative(c); ok {
			return AllyMove{Kind: AllyUse, Item: i, Ally: c}
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

// bestRestorative picks which of a companion's own healing items to drink: the
// smallest one that covers the damage taken, or the biggest they have if none
// does.
//
// Reaching for the strongest bottle every time is how a party burns a
// Physician's Draught on a scratch. Picking the smallest that does the job is
// what a person who paid for them would do.
func bestRestorative(c *model.Character) (int, bool) {
	missing := c.MaxHP - c.HP
	best, biggest := -1, -1
	for i, it := range c.Bag {
		if it.Kind != model.ItemHeal || it.Count <= 0 {
			continue
		}
		if biggest < 0 || it.Power > c.Bag[biggest].Power {
			biggest = i
		}
		if it.Power < missing {
			continue
		}
		if best < 0 || it.Power < c.Bag[best].Power {
			best = i
		}
	}
	if best >= 0 {
		return best, true
	}
	if biggest >= 0 {
		return biggest, true
	}
	return 0, false
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
		if c.Alive() && c.Str() > best.Str() {
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
func HireCost(level int, blood model.MonsterKind, s Standing) int64 {
	base := int64(60 + level*level*6)
	if l, ok := model.LineageOf(blood); ok {
		base -= base * int64(l.Discount) / 100
	}
	// What somebody asks to follow you depends on who they think you are, and
	// it is not the same question a shopkeeper is asking.
	base = int64(float64(base) * s.HireMultiplier())
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
	// A hireling arrives with nothing but what they are wearing. The purse and
	// the errands stay the hero's for good; the pack is theirs to be given.
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
	Won bool
	// Fled reports that the fight was abandoned rather than lost. It is not a
	// win — nothing was killed and nothing was paid out — but it is emphatically
	// not a death either, and a report that conflated the two was measuring a
	// game nobody plays.
	Fled        bool
	Rounds      int
	DamageDealt int
	DamageTaken int
	HPLeft      int
}

// Died reports the outcome that actually costs a run.
func (r FightResult) Died() bool { return !r.Won && !r.Fled }

// worstHPFrac is the health of the healthiest thing still standing, which is
// what decides whether a fight is nearly turned or merely lost.
func worstHPFrac(living []*model.Monster) float64 {
	var worst float64
	for _, m := range living {
		if f := m.HPFrac(); f > worst {
			worst = f
		}
	}
	return worst
}

// inTrouble reports that the fight is close enough to going wrong to be worth
// gambling on. Looser than wantsOut, which is the threshold for giving up.
func inTrouble(c *model.Character, living []*model.Monster) bool {
	if c.MaxHP <= 0 {
		return false
	}
	return c.HP*2 <= c.MaxHP && float64(c.HP) < float64(incomingPerRound(c, living))*3
}

// incomingPerRound estimates what is about to be taken off the player, which is
// the unit both the retreat and the gambit are decided in.
func incomingPerRound(c *model.Character, living []*model.Monster) int {
	total := 0
	for _, m := range living {
		if m.Dead {
			continue
		}
		guard := c.Defense()
		if m.Def != nil && m.Def.Magic {
			guard = c.Ward()
		}
		total += core.Max(1, int(float64(m.Offense)*0.85)-guard)
	}
	return total
}

// wantsOut decides whether the simulated player should be trying to leave.
//
// Deliberately late and conservative: badly hurt, losing on exchange, and out
// of anything that would turn it round. A policy that ran at the first scratch
// would report a game with no danger in it at all, and the point is to measure
// what a competent player survives rather than what a cautious one avoids.
func wantsOut(c *model.Character, living []*model.Monster) bool {
	if c.MaxHP <= 0 || len(living) == 0 {
		return false
	}

	// Measured in rounds left rather than as a fraction of health, because a
	// fraction is the wrong unit for the decision. A flat "below 22%" had the
	// thief waiting until 22% of a small pool, by which point one hit finished
	// it — so the class with by far the best escape odds per attempt got fewer
	// attempts than the fighter and escaped *less often*. Nobody plays that
	// way. You leave while leaving is still possible.
	incoming := incomingPerRound(c, living)
	if c.HP > incoming*3/2 {
		return false
	}

	// Still winning the race? Then finish it. Being close to death is only a
	// reason to leave if the thing opposite is not closer.
	//
	// Half rather than a third: at a third the simulator bailed out of fights
	// it was about to win, and a level 13 fighter's run of fights on one rest
	// fell to 2.3 before it broke the endurance floor. Retreating is supposed
	// to be the answer to losing, not to being hurt.
	return worstHPFrac(living) > 0.5
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
		playerFirst := Initiative(g, sim.Spd(), fastest)

		// Set when a false retreat is not bought: the creature that saw through
		// it hits harder for the round. Declared up here because monsterTurns
		// closes over it below.
		punished := false

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
			if s, ok := bestSpell(sim, spells, living); ok {
				sim.Psyche -= s.Cost
				switch s.Kind {
				case model.SpellHeal:
					sim.HP = core.Clamp(sim.HP+SpellDamage(g, sim, s), 0, sim.MaxHP)
				case model.SpellDrain:
					d := AfterWard(SpellDamage(g, sim, s), target.Ward)
					hurt(target, d)
					sim.HP = core.Clamp(sim.HP+d/2, 0, sim.MaxHP)
				default:
					raw := SpellDamage(g, sim, s)
					if s.Target == model.TargetAll {
						for _, m := range living {
							if !m.Dead {
								hurt(m, AfterWard(raw, m.Ward))
							}
						}
					} else {
						hurt(target, AfterWard(raw, target.Ward))
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
		// standing counts what is still in the fight, which is what decides
		// whether the one taking its turn has anybody left to run behind.
		standing := func() int {
			n := 0
			for _, m := range living {
				if !m.Dead {
					n++
				}
			}
			return n
		}
		monsterTurns := func() {
			for _, m := range living {
				if m.Dead || sim.HP <= 0 {
					continue
				}
				switch ChooseMonsterAction(g, m, standing() == 1) {
				case MonFlee:
					m.Dead = true // leaves the fight; the player is not chasing it
				case MonDefend:
					// no attack this turn
				default:
					dmg := MonsterDamage(g, sim, m)
					if punished {
						dmg = FeintPunish(dmg)
					}
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

		// Run, if running is what a person would do.
		//
		// Without this the simulator measured a game in which nobody ever walks
		// away, which flattered every class that can stand and trade and
		// libelled the one built to leave. The thief has the best speed in the
		// game and speed is what FleeChance reads, so its whole survival plan
		// was invisible: it was being scored on how well it dies in fights it
		// would never have finished.
		//
		// Trying to leave *is* the player's action for the round, and failing
		// to leave spends it. Skipping the monsters' turn as well made escape
		// free and eventually certain, which is a different lie from the one it
		// replaced.
		// Losing gives the player a third answer as well as fight and run: a
		// thief can sell the retreat rather than take it. It is modelled here
		// because it has to be — a move the simulator cannot see is one the
		// balance report lies about, and this one deliberately trades survival
		// for the chance at a fight that pays.
		// Three answers when it is going badly, not two.
		//
		// The feint is not an alternative to running, which was the first way
		// this was modelled and it barely fired: wantsOut only triggers while
		// the other thing is still above half health, and a false retreat is
		// only worth selling when it is nearly dead, so the two windows hardly
		// ever met. It is an alternative to *swinging* — the gambit you take
		// when you are nearly out of hit points and the thing opposite is
		// nearly out of them too, and you would rather end it this round than
		// find out who runs out first.
		act := strike
		switch {
		case wantsOut(sim, living):
			act = func() {
				if g.Chance(FleeChance(sim.Spd(), fastest)) {
					res.Fled = true
				}
			}
		case CanFeint(sim) && inTrouble(sim, living) && FeintIsWorthIt(sim, living[0]) &&
			FeintChance(sim, fastest) >= feintFloor:
			act = func() {
				target := living[0]
				if g.Chance(FeintChance(sim, fastest)) {
					hurt(target, FeintDamage(g, sim, target))
					return
				}
				punished = true
			}
		}

		if playerFirst {
			act()
			if res.Fled {
				break
			}
			monsterTurns()
		} else {
			monsterTurns()
			if sim.HP > 0 {
				act()
			}
			if res.Fled {
				break
			}
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
	weapon := float64(c.Str())/2 + float64(c.Strike())
	spell := float64(attack.Power) + float64(c.MaxPsy())*0.6 + float64(c.Level)*0.8
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
// bestSpell picks the technique to cast, healing when healing is what the
// moment calls for.
//
// The trigger is rounds of survival left, not a fraction of health — the same
// correction the retreat needed and for the same reason. "Below 30%" is 4 hit
// points to a level-one mage and 36 to a level-thirteen fighter: the mage was
// waiting until one blow finished it before considering a heal, which is why it
// came out the most fragile thing in the game at exactly the level where it has
// the fewest hit points and the most psyche to spend on not dying.
func bestSpell(c *model.Character, spells []model.Spell, living []*model.Monster) (model.Spell, bool) {
	if heal, ok := bestHeal(c, spells); ok {
		incoming := incomingPerRound(c, living)
		if c.HP <= incoming*2 && c.HP < c.MaxHP*3/4 {
			return heal, true
		}
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
