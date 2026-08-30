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
		// A point or two rather than nothing-or-one, which is what it was.
		//
		// The old growth left a level-twelve Fighter with nine psyche against a
		// Haymaker costing eleven, so the technique at the top of their own
		// list was a row that could be read and never selected — and once
		// technique got a class surcharge, "never" stopped being an accident of
		// a bad roll and became the rule. A class whose best move is
		// permanently greyed out has a shorter menu than the one it is shown.
		c.MaxPsyche += g.Between(1, 2)
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
		c.MaxPsyche += g.Between(1, 2)
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
	// Magic is set when the blow was a bolt off a focus rather than a hit with
	// something heavy, so the transcript and the burst can say which happened.
	Magic bool
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
	if c.Casting() {
		// A rod's ordinary attack is a bolt, and it is free.
		//
		// This is the answer to the question a focus slot otherwise begs: a
		// caster holding a stick with strike 5 has a plain Attack worth
		// nothing, so every round they cannot afford a technique is a round
		// they are worse than a fighter with a table leg. Making the free
		// action itself magical means a Mage is casting all the time and
		// *paying* only for the big ones, which is what "magic is what a mage
		// does" has to mean if it is going to mean anything.
		//
		// It is resisted by whichever of the target's two defences is thinner.
		//
		// The first version went through the ward alone, which is the tidy
		// answer and the wrong one: ward outgrows armour at the top of the
		// monster table, so a level-thirteen Mage's free round against a dragon
		// landed for about three, and the class simply stopped having a free
		// action exactly where every other class's got better. Death at three
		// levels over went to 47% against a brief that allows 36.
		//
		// A bolt is a shove of raw force rather than a shaped working, so it
		// goes wherever the thing is thinnest. That keeps the rod a *floor* —
		// it is a small number in every matchup rather than a coin flip on one
		// — and it leaves the interesting choice intact, because a dagger still
		// carries strength behind it and still hits a dragon harder than the
		// rod does.
		sw.Magic = true
		sw.Damage = AfterWard(FocusBolt(g, c)+buffStr, core.Min(m.Defense, m.Ward))
		if sw.Crit {
			sw.Damage = sw.Damage*3/2 + 2
		}
		return sw
	}
	sw.Damage = PlayerDamage(g, c, m) + buffStr
	if sw.Crit {
		sw.Damage = sw.Damage*3/2 + 2
	}
	return sw
}

// FocusBolt rolls the free attack a wand or staff makes, before resistance.
//
// Deliberately flatter than a weapon swing: strength is not in it, so a Mage's
// output is a straight read of what they are holding and what level they are.
// That is the shape a caster is supposed to have — gear and study rather than
// arms — and it is why the focus ladder is the mage's whole shopping list.
func FocusBolt(g *core.RNG, c *model.Character) int {
	base := float64(c.Focus())*focusBite + float64(c.Level)*0.6
	return core.Max(1, g.Between(int(base*0.75), int(base*1.25)))
}

// focusBite is what a point of focus is worth in the free bolt. Tuned against
// the COMBAT table rather than picked: it is the value that leaves a Mage's
// unpaid round roughly where a Thief's swing is, which is the claim the whole
// slot rests on.
const focusBite = 1.15

// focusStudy is what the same point of focus is worth inside a *paid*
// technique, and it is deliberately half what the free bolt gets.
//
// The rod is the mage's whole shopping list, so it has to move both numbers or
// buying one is a decision about the cheap round only. But a spell already
// scales off the psyche pool and the level, so paying focus at full rate there
// compounds three growth terms into one build — which is exactly what the first
// draft did, and it put a level-eleven Mage at 94% on the stretch fights the
// Fighter was losing half of.
const focusStudy = 0.15

// psycheStudy and levelStudy are the two terms that grow on their own, and both
// came down when the caster's off arm opened up.
//
// A Mage with a talisman was winning the stretch fights at levels eleven to
// thirteen by fifteen to twenty points over both other classes, and the cause
// was not the barrier: it was that three of SpellPower's four terms grow with
// level, so the magnitude curve outruns a table whose defences grow linearly.
// Cutting the pool's coefficient rather than the pool itself was the second
// attempt — slowing psyche growth to everybody else's rate took the level
// thirteen Mage from too strong to dying in 43% of on-level-plus-two fights,
// which is a class that has stopped working rather than one that has been
// tuned. A coefficient moves the top of the curve and leaves the bottom, which
// is the half that was already too thin.
const (
	psycheStudy = 0.60
	levelStudy  = 0.60
)

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
	base := SpellPower(c, s)
	lo := int(base * 0.8)
	hi := int(base * 1.3)
	return core.Max(1, g.Between(lo, hi))
}

// SpellPower is the magnitude a technique lands for before the roll's spread.
//
// Three terms, and each is a different thing the player did. Power is the
// technique. Psyche is what they are — the pool a level-up grows and a charm
// widens, and the only term a Fighter's techniques have ever had. Focus is what
// they are *holding*, and it is the term that exists so a caster has something
// to spend money on: before it, a mage's shopping list was a sword they were
// bad at swinging and their magic got better only by levelling.
//
// It reads out here rather than inside SpellDamage because two callers need the
// number without the dice: the AI weighing a technique against a swing, and the
// character sheet, which cannot show a range it does not know the middle of.
func SpellPower(c *model.Character, s model.Spell) float64 {
	return float64(s.Power) + float64(c.MaxPsy())*psycheStudy +
		float64(c.Focus())*focusStudy + float64(c.Level)*levelStudy
}

// --- the two-sided techniques ---------------------------------------------

// pactShare is what a pact charges the caster, as a fraction of the technique's
// own power. A quarter is the value at which the report stops treating it as a
// straight upgrade: much less and it is simply the best attack in the list,
// much more and the simulator's policy never picks it, which is the same as it
// not existing.
const pactShare = 0.25

// PactCost is the weakness the caster wears for the rest of the fight, in the
// same units OffenseMod reads.
//
// It is derived from the technique rather than authored beside it for the
// reason the whole content layer follows: two numbers that must move together
// are one number and a rule. A pact whose power was raised in a balance pass
// and whose cost was not would silently become the free lunch the kind exists
// to not be.
func PactCost(s model.Spell) int { return core.Max(1, int(float64(s.Power)*pactShare)) }

// --- what a technique costs, and who it costs it ---------------------------

// psycheRate is how dearly each class buys the pool it spends.
//
// A Mage pays the number on the tin. Everybody else pays a surcharge, and the
// point of it is not the arithmetic — a Fighter's psyche pool was always small
// and their techniques always rationed — it is that the surcharge is *stated*.
// The pool used to be one bar with one price, so "why does the mage cast so
// much more than I do" was answered by two stat tables nobody puts side by
// side. Now the shop of techniques quotes a different figure to each of them
// and the reason is on the row.
//
// It is a multiplier rather than a second cost column in the data, because the
// spell table is authored per class already and a second number per row would
// be a second number to keep in step for no reader's benefit.
var psycheRate = map[model.Class]float64{
	model.ClassMage:    1.00,
	model.ClassThief:   1.15,
	model.ClassFighter: 1.30,
}

// PsycheRate is what a class multiplies a technique's listed cost by. Exposed
// so the interface can say *why* a figure is what it is rather than only what
// it is: a surcharge the game will not explain is a surcharge that reads as the
// numbers being wrong.
func PsycheRate(class model.Class) float64 {
	if r, ok := psycheRate[class]; ok {
		return r
	}
	return 1
}

// PsycheCost is what this character actually pays to cast this technique.
//
// Rounded up, and floored at one: a rate that could round a cost to nothing
// would hand somebody a free technique for being bad at techniques.
func PsycheCost(c *model.Character, s model.Spell) int {
	rate, ok := psycheRate[c.Class]
	if !ok {
		rate = 1
	}
	return core.Max(1, int(math.Ceil(float64(s.Cost)*rate)))
}

// --- getting your breath back ---------------------------------------------

// The share of what a fight actually *took* that comes back to anybody still
// standing at the end of it.
//
// A share of the spend rather than a share of the pool, and that is the whole
// of making this safe. A flat tenth of maximum hit points refunded per fight is
// larger than what a level-one fight costs, so the character heals faster than
// the world hurts them and endurance goes to infinity — which the first draft
// did, and the table said so: forty fights on one rest at level one and level
// five, against sixteen before. A share of the damage taken is a *discount* on
// the encounter. It can never be net positive, it scales with how bad the fight
// was without being told the level, and it moves endurance by a factor anybody
// can compute: one over one minus the share.
//
// Per fight rather than per round, which is the other thing this could have
// been. Regeneration inside a fight makes a long fight cheaper than a short
// one, which rewards turtling and quietly makes stalling the correct play. The
// encounter is the unit the overworld counts in, the unit ENDURANCE measures,
// and the unit a player is thinking in when they decide whether they can take
// one more before the inn.
//
// Psyche comes back at a higher share than blood on purpose. A caster out of
// psyche has no second option and stands there waving a stick; somebody at low
// hit points still has every option they had at full, and being made to weigh
// that is what a health bar is for.
const (
	restHPShare     = 0.20
	restPsycheShare = 0.40
)

// CatchBreath hands back a share of what the fight cost to somebody who walked
// away from it, whether they won or ran. Reports what came back.
//
// Running counts, and that is not generosity: a retreat already pays no
// experience, no coin and no drop, and charging it the full price of the fight
// as well is what makes "leave" the answer nobody picks.
func CatchBreath(c *model.Character, hpLost, psycheSpent int) (hp, psyche int) {
	if c == nil || c.HP <= 0 {
		return 0, 0
	}
	hp = core.Clamp(int(float64(hpLost)*restHPShare), 0, c.MaxHP-c.HP)
	psyche = core.Clamp(int(float64(psycheSpent)*restPsycheShare), 0, c.MaxPsyche-c.Psyche)
	c.HP += hp
	c.Psyche += psyche
	return hp, psyche
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
		if s.Kind != kind || PsycheCost(c, s) > c.Psyche || !s.Known(c) {
			continue
		}
		if !found || s.Power > best.Power {
			best, found = s, true
		}
	}
	return best, found
}

// SellRate is the fraction of an item's value a merchant will hand over.
// Merchants are not charities and the game should not pretend otherwise.
//
// It lives here rather than at the counter that uses it because the balance
// report has to price a haul: what a fight is worth is the coins plus what the
// drops fetch, and a second copy of this number in cmd/balance would be the
// arbiter quietly measuring a different shop from the one in the game.
const SellRate = 0.45

// SellPrice is what a merchant will hand over for something worth n new.
func SellPrice(n int) int {
	if p := int(float64(n) * SellRate); p > 1 {
		return p
	}
	return 1
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

// What a rescue costs: a share of the purse, up to a point.
//
// The share hurts proportionally at every level and can never be unpayable,
// which is why it is a share and not a flat sum — a run must not be able to end
// because the player could not afford to survive. But a percentage grows
// without limit, so the same rule that is a fair sting at level two is a
// confiscation at level twelve, and the fee is meant to be why hiring somebody
// is worth it rather than why dying is unthinkable.
//
// So it is a third of the purse, and never more than a good weapon's worth.
// Past the cap the cost of dying stops rising and the cost of *not* having
// hired anybody keeps going, which is the direction the pressure should point.
const (
	rescueShare = 30
	rescueCap   = 250
)

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
	return core.Min64(rescueCap, core.Max64(1, coins*rescueShare/100))
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
	// What CatchBreath handed back at the end. Reported rather than only
	// applied, because a section measuring the cost of a fight has to be able
	// to say what the fight refunded.
	HPBack     int
	PsycheBack int
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
	mons := make([]*model.Monster, 0, len(defs))
	for _, d := range defs {
		mons = append(mons, d.Spawn(g, level))
	}
	return SimulateGroup(g, c, mons, maxRounds, spells)
}

// SimulateGroup is SimulateFight against creatures that already exist.
//
// The split is what lets the report measure an *encounter* rather than a
// creature. A shape is a composition — more of them and smaller, one of them
// and enormous, one that stops steel beside one that stops magic — and none of
// that survives being flattened into "n definitions, all at level L", which is
// the only thing SimulateFight could ever say.
func SimulateGroup(g *core.RNG, c *model.Character, mons []*model.Monster, maxRounds int, spells []model.Spell) FightResult {
	// The character is spent, not copied: hit points and psyche carry out of
	// the fight so a run of encounters can be simulated on one rest. Callers
	// wanting an isolated fight pass a copy.
	sim := c

	var res FightResult
	// What the fight cost in psyche, which is half of what it refunds.
	spent := 0
	// Whatever is on the off arm goes up before the first blow. A talisman the
	// simulator could not see would be a slot the balance pass says is empty.
	Raise(sim)
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
				cost := PsycheCost(sim, s)
				sim.Psyche -= cost
				spent += cost
				switch s.Kind {
				case model.SpellHeal:
					sim.HP = core.Clamp(sim.HP+SpellDamage(g, sim, s), 0, sim.MaxHP)
				case model.SpellDrain:
					d := AfterWard(SpellDamage(g, sim, s), target.Ward)
					hurt(target, d)
					sim.HP = core.Clamp(sim.HP+d/2, 0, sim.MaxHP)
				case model.SpellSap:
					// Off them and onto you, once, however many it reached.
					for _, m := range sapTargets(s, living, target) {
						m.Active = Apply(m.Active, model.Effect{
							Kind: model.EffectWeaken, Power: s.Power, Rounds: model.Forever,
						})
					}
					sim.Active = Apply(sim.Active, model.Effect{
						Kind: model.EffectBless, Power: s.Power, Rounds: model.Forever,
					})
				case model.SpellPact:
					raw := SpellDamage(g, sim, s)
					for _, m := range sapTargets(s, living, target) {
						hurt(m, AfterWard(raw, m.Ward))
					}
					sim.Active = Apply(sim.Active, model.Effect{
						Kind: model.EffectWeaken, Power: PactCost(s), Rounds: model.Forever,
					})
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
			// Buffs and weakenings, which the simulator used to pass as zero.
			// That was a hole rather than a simplification: the battle screen
			// has always read them, so a blessing was worth something in the
			// game and nothing in the report — and it is the whole payload of
			// the two-sided techniques below, which would otherwise have been
			// measured as an attack that does nothing.
			sw := PlayerAttack(g, sim, target, OffenseMod(sim.Active), DexterityMod(sim.Active))
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
					// The other half of the same hole: a weakened monster hits
					// softer in the game and hit full strength in the report.
					dmg := core.Max(0, MonsterDamage(g, sim, m)+OffenseMod(m.Active))
					if punished {
						dmg = FeintPunish(dmg)
					}
					// The barrier eats what it can before anything reaches the
					// body, whatever the blow was made of.
					sim.Active, dmg, _ = Soak(sim.Active, dmg)
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
			d := t.Damage
			sim.Active, d, _ = Soak(sim.Active, d)
			sim.HP = core.Max(0, sim.HP-d)
			res.DamageTaken += d
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
	// Getting your breath back happens here rather than at the call site, so
	// the ENDURANCE table and the battle screen cannot disagree about how much
	// a fight actually costs. It is the single number this whole section is
	// about; a simulator that could not see it would be measuring a game where
	// every encounter is paid for in full and never repaid.
	res.HPBack, res.PsycheBack = CatchBreath(sim, res.DamageTaken, spent)
	res.HPLeft = sim.HP
	return res
}

// sapTargets is who a two-sided technique reaches: everything standing, or the
// one thing it was pointed at.
func sapTargets(s model.Spell, living []*model.Monster, target *model.Monster) []*model.Monster {
	if s.Target != model.TargetAll {
		return []*model.Monster{target}
	}
	out := make([]*model.Monster, 0, len(living))
	for _, m := range living {
		if !m.Dead {
			out = append(out, m)
		}
	}
	return out
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
		if PsycheCost(c, s) > c.Psyche || !s.Known(c) {
			continue
		}
		switch s.Kind {
		case model.SpellDamage, model.SpellDrain, model.SpellPact:
		default:
			continue
		}
		// A pact is weighed on what is left of it after the caster has paid.
		// Comparing raw power would make it the answer to every round in the
		// game, since paying for it later is free at the moment of choosing.
		if !found || attackWorth(s) > attackWorth(attack) {
			attack, found = s, true
		}
	}
	if !found {
		return model.Spell{}, false
	}
	if SpellPower(c, attack) <= freeSwingWorth(c) {
		return model.Spell{}, false
	}
	return attack, true
}

// attackWorth ranks two attacking techniques against each other: the magnitude,
// less whatever the caster is going to be wearing afterwards.
func attackWorth(s model.Spell) float64 {
	if s.Kind == model.SpellPact {
		return float64(s.Power - PactCost(s))
	}
	return float64(s.Power)
}

// freeSwingWorth is what the round costs nothing to spend: a swing, or the bolt
// off a rod. It is what a paid technique has to beat to be worth paying for.
//
// A caster's floor rises with their rod, which is the point — a level-one spark
// should stop being worth two psyche once the staff throws harder for free,
// exactly as a fighter's opening technique retires behind a better sword.
func freeSwingWorth(c *model.Character) float64 {
	if c.Casting() {
		return float64(c.Focus())*focusBite + float64(c.Level)*0.6
	}
	return float64(c.Str())/2 + float64(c.Strike())
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
	// A sap goes first or not at all: it pays out over the rest of the fight,
	// so a round spent on it late buys almost nothing, and casting a second one
	// would only stack a blessing the caster already has. Both conditions are
	// read off the board rather than remembered, which is what keeps this a
	// policy rather than a piece of state the simulator has to carry.
	if s, ok := worthSapping(c, spells, living); ok {
		return s, true
	}
	return bestAttack(c, spells)
}

// worthSapping picks a two-sided debuff to open with, when there is a fight
// left to spend it on.
func worthSapping(c *model.Character, spells []model.Spell, living []*model.Monster) (model.Spell, bool) {
	if Has(c.Active, model.EffectBless) {
		return model.Spell{}, false
	}
	var best model.Spell
	found := false
	for _, s := range spells {
		if s.Kind != model.SpellSap || PsycheCost(c, s) > c.Psyche || !s.Known(c) {
			continue
		}
		if !found || s.Power > best.Power {
			best, found = s, true
		}
	}
	if !found {
		return model.Spell{}, false
	}
	// Only while there is something to spend the blessing on. Against one thing
	// already most of the way down, the round is better spent finishing it —
	// which is the same judgement the feint makes and for the same reason.
	whole := 0
	for _, m := range living {
		if !m.Dead && m.HP*2 > m.MaxHP {
			whole++
		}
	}
	if whole == 0 {
		return model.Spell{}, false
	}
	return best, true
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
