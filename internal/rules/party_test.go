package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// spellbook is a stand-in for the real technique table: a cheap attack, a heal
// that can be aimed at anybody, one nobody can afford, a blessing and a revive.
var spellbook = []model.Spell{
	{ID: "poke", Name: "Poke", Level: 1, Cost: 2, Power: 9, Kind: model.SpellDamage, Target: model.TargetOne},
	{ID: "patch", Name: "Patch", Level: 1, Cost: 3, Power: 20, Kind: model.SpellHeal, Target: model.TargetOne},
	{ID: "ruin", Name: "Ruin", Level: 1, Cost: 999, Power: 400, Kind: model.SpellDamage, Target: model.TargetOne},
	{ID: "cheer", Name: "Cheer", Level: 1, Cost: 2, Power: 3, Kind: model.SpellBless, Target: model.TargetOne},
	{ID: "raise", Name: "Raise", Level: 1, Cost: 4, Power: 30, Kind: model.SpellRevive, Target: model.TargetOne},
}

// selfOnly is the same book with a heal that cannot leave the caster, which is
// what Second Wind is and what every heal used to be.
var selfOnly = []model.Spell{
	{ID: "poke", Name: "Poke", Level: 1, Cost: 2, Power: 9, Kind: model.SpellDamage, Target: model.TargetOne},
	{ID: "wind", Name: "Wind", Level: 1, Cost: 3, Power: 20, Kind: model.SpellHeal, Target: model.TargetSelf},
}

// solo is the party a lone character is in.
func solo(c *model.Character) []*model.Character { return []*model.Character{c} }

func TestRecruitIsAnOrdinaryCharacterOfThatLevel(t *testing.T) {
	g := core.NewRNG(7)
	for _, level := range []int{1, 5, 12} {
		c := rules.Recruit(g, "Nessa", model.ClassThief, "", level)
		if c.Level != level {
			t.Errorf("hired at level %d, arrived at level %d", level, c.Level)
		}
		if !c.Ally {
			t.Errorf("level %d hireling is not marked as an ally", level)
		}
		if c.HP != c.MaxHP || c.HP <= 0 {
			t.Errorf("level %d hireling arrived at %d/%d hit points", level, c.HP, c.MaxHP)
		}
		// A hireling arrives with nothing but what they are wearing. The purse
		// and the training debt stay the hero's for good; the pack is theirs to
		// be given, but not before they have been hired.
		if c.Coins != 0 || len(c.Bag) != 0 || c.SpendXP != 0 {
			t.Errorf("level %d hireling turned up with %d coins, %d items and %d unspent XP",
				level, c.Coins, len(c.Bag), c.SpendXP)
		}
		if c.Cut <= 0 || c.Cut > 50 {
			t.Errorf("level %d hireling wants %d%% of the take, which is not a sane cut", level, c.Cut)
		}
	}
}

// A companion levelled to the hero's level must be worth roughly what the hero
// is, or "hire someone" is a worse purchase than any weapon at the same price.
func TestRecruitMatchesAHeroOfTheSameLevel(t *testing.T) {
	for _, class := range model.AllClasses {
		g := core.NewRNG(11)
		hero := rules.BuildCharacter(g, class, 8)
		mate := rules.Recruit(g, "Mate", class, "", 8)
		if mate.MaxHP < hero.MaxHP/2 || mate.Strength < hero.Strength/2 {
			t.Errorf("%s hireling (%d HP, %d str) is not in the same league as the hero (%d HP, %d str)",
				class, mate.MaxHP, mate.Strength, hero.MaxHP, hero.Strength)
		}
	}
}

// The whole of a companion's tactical brain. It must never pick something it
// cannot pay for — a companion that casts on credit would silently drive psyche
// negative and heal for free every round after that.
func TestCompanionNeverCastsWhatItCannotAfford(t *testing.T) {
	g := core.NewRNG(3)
	c := rules.BuildCharacter(g, model.ClassMage, 6)
	c.Weapon = model.Weapon{Name: "Stick", Strike: 1}

	for psyche := 0; psyche <= c.MaxPsyche; psyche++ {
		c.Psyche = psyche
		for hp := 1; hp <= c.MaxHP; hp++ {
			c.HP = hp
			move := rules.ChooseAllyMove(g, c, spellbook, solo(c))
			if move.Kind != rules.AllyCast {
				continue
			}
			if move.Spell.Cost > psyche {
				t.Fatalf("at %d psyche the companion chose %s, which costs %d",
					psyche, move.Spell.Name, move.Spell.Cost)
			}
		}
	}
}

// A companion with nothing worth casting has to actually swing. An AI that
// mostly guards would be a hireling you pay to stand behind you.
func TestCompanionMostlyAttacks(t *testing.T) {
	g := core.NewRNG(5)
	c := rules.BuildCharacter(g, model.ClassFighter, 5)
	c.Weapon = model.Weapon{Name: "Actual Sword", Strike: 6}
	c.Psyche = 0 // nothing affordable

	swings := 0
	const rounds = 400
	for i := 0; i < rounds; i++ {
		c.HP = c.MaxHP
		if rules.ChooseAllyMove(g, c, spellbook, solo(c)).Kind == rules.AllySwing {
			swings++
		}
	}
	if swings != rounds {
		t.Fatalf("a healthy companion with no psyche swung %d times out of %d; it should always swing",
			swings, rounds)
	}
}

// Badly hurt with a heal in reach, a companion has to use it, or the party
// medic is decorative.
func TestCompanionHealsItselfWhenBadlyHurt(t *testing.T) {
	g := core.NewRNG(9)
	c := rules.BuildCharacter(g, model.ClassMage, 6)
	c.Weapon = model.Weapon{Name: "Stick", Strike: 1}
	c.Psyche = c.MaxPsyche
	c.HP = 1 // as hurt as it gets while still upright

	for i := 0; i < 50; i++ {
		move := rules.ChooseAllyMove(g, c, selfOnly, solo(c))
		if move.Kind != rules.AllyCast || move.Spell.Kind != model.SpellHeal {
			t.Fatalf("at 1 hit point the companion chose %+v instead of healing", move)
		}
		if move.Ally != c {
			t.Fatalf("a self-only heal was aimed at %v instead of the caster", move.Ally)
		}
	}
}

func TestHireCostRisesWithLevel(t *testing.T) {
	prev := int64(-1)
	for level := 1; level <= 20; level++ {
		got := rules.HireCost(level, "", rules.Unknown)
		if got <= prev {
			t.Fatalf("a level %d hireling costs %d, which is not more than level %d's %d",
				level, got, level-1, prev)
		}
		prev = got
	}
	// A first hire has to be reachable but not immediate: a new character
	// starts with 45-95 coins, so this is a fight or two away, not free.
	if c := rules.HireCost(1, "", rules.Unknown); c < 50 || c > 120 {
		t.Errorf("the cheapest hireling costs %d, which is outside the intended first-hire band", c)
	}

	// And who they think you are moves the price in both directions, because a
	// mercenary is asking a different question from a shopkeeper.
	base := rules.HireCost(8, "", rules.Unknown)
	if rules.HireCost(8, "", rules.Celebrated) >= base {
		t.Error("a name to follow costs no less than an unknown one")
	}
	if rules.HireCost(8, "", rules.Notorious) <= base {
		t.Error("following somebody notorious costs no more than following a stranger")
	}
}

func TestSkimTakesTheStatedShare(t *testing.T) {
	if got := rules.Skim(200, 10); got != 20 {
		t.Errorf("a 10%% cut of 200 came to %d, want 20", got)
	}
	// Nothing to skim, and nobody skimming, both have to come out at zero
	// rather than at a negative that would pay the player for the privilege.
	if got := rules.Skim(0, 25); got != 0 {
		t.Errorf("a cut of nothing came to %d", got)
	}
	if got := rules.Skim(500, 0); got != 0 {
		t.Errorf("a zero cut came to %d", got)
	}
	if got := rules.Skim(-40, 20); got != 0 {
		t.Errorf("a cut of a negative haul came to %d", got)
	}
	// The party must never cost more than the haul it is taking from.
	for coins := int64(1); coins <= 1000; coins += 37 {
		var total int64
		for i := 0; i < 2; i++ {
			total += rules.Skim(coins, 18)
		}
		if total > coins {
			t.Fatalf("two companions skimmed %d from a haul of %d", total, coins)
		}
	}
}

// The point of the targeting rework: a medic patches whoever is worst off, not
// only themselves. Before this, every heal in the game was self-only and a
// party healer was decorative.
func TestCompanionHealsWhoeverIsWorstOff(t *testing.T) {
	g := core.NewRNG(21)
	medic := rules.BuildCharacter(g, model.ClassMage, 6)
	medic.Weapon = model.Weapon{Name: "Stick", Strike: 1}
	medic.Psyche = medic.MaxPsyche

	hero := rules.BuildCharacter(g, model.ClassFighter, 6)
	hero.HP = hero.MaxHP / 10 // on his last legs
	other := rules.BuildCharacter(g, model.ClassThief, 6)

	party := []*model.Character{hero, medic, other}
	move := rules.ChooseAllyMove(g, medic, spellbook, party)
	if move.Kind != rules.AllyCast || move.Spell.Kind != model.SpellHeal {
		t.Fatalf("with the hero nearly dead the medic chose %+v", move)
	}
	if move.Ally != hero {
		t.Fatalf("the heal was aimed at %q, not at the one who is nearly dead", move.Ally.Name)
	}
}

// Somebody on the floor outranks somebody merely bleeding: a revive returns a
// whole combatant to the round, which no heal can do.
func TestCompanionRevivesBeforeHealing(t *testing.T) {
	g := core.NewRNG(23)
	medic := rules.BuildCharacter(g, model.ClassMage, 8)
	medic.Weapon = model.Weapon{Name: "Stick", Strike: 1}
	medic.Psyche = medic.MaxPsyche

	fallen := rules.BuildCharacter(g, model.ClassFighter, 8)
	fallen.HP = 0
	bleeding := rules.BuildCharacter(g, model.ClassThief, 8)
	bleeding.HP = 1

	party := []*model.Character{fallen, bleeding, medic}
	move := rules.ChooseAllyMove(g, medic, spellbook, party)
	if move.Kind != rules.AllyCast || move.Spell.Kind != model.SpellRevive {
		t.Fatalf("with somebody down the medic chose %+v", move)
	}
	if move.Ally != fallen {
		t.Fatalf("the revive was aimed at %q, who is standing", move.Ally.Name)
	}
}

// A party-side technique must always name who it is for, or the battle screen
// has to guess and will guess wrong.
func TestPartySideCastsAlwaysNameATarget(t *testing.T) {
	g := core.NewRNG(29)
	for i := 0; i < 500; i++ {
		c := rules.BuildCharacter(g, model.ClassMage, 6)
		c.Weapon = model.Weapon{Name: "Stick", Strike: 1}
		c.Psyche = g.Between(0, c.MaxPsyche)
		c.HP = g.Between(1, c.MaxHP)
		mate := rules.BuildCharacter(g, model.ClassFighter, 6)
		mate.HP = g.Between(0, mate.MaxHP)

		move := rules.ChooseAllyMove(g, c, spellbook, []*model.Character{c, mate})
		if move.Kind != rules.AllyCast || move.Spell.Kind.Side() != model.SideParty {
			continue
		}
		if move.Ally == nil {
			t.Fatalf("%s was cast at nobody", move.Spell.Name)
		}
		// A revive aimed at somebody upright, or a heal at somebody down, would
		// both be a wasted turn and a confusing line in the transcript.
		if wantFallen := move.Spell.Kind == model.SpellRevive; move.Ally.Alive() == wantFallen {
			t.Fatalf("%s was aimed at %s, who is alive=%v", move.Spell.Name, move.Ally.Name, move.Ally.Alive())
		}
	}
}

// Triage comes first. A companion must not stop to hand out blessings while
// somebody is bleeding out.
func TestCompanionDoesNotBlessWhileSomeoneIsHurt(t *testing.T) {
	g := core.NewRNG(31)
	c := rules.BuildCharacter(g, model.ClassFighter, 6)
	c.Weapon = model.Weapon{Name: "Sword", Strike: 40} // so no attack spell beats it
	c.Psyche = c.MaxPsyche

	mate := rules.BuildCharacter(g, model.ClassThief, 6)
	mate.HP = mate.MaxHP / 4

	party := []*model.Character{c, mate}
	for i := 0; i < 300; i++ {
		if move := rules.ChooseAllyMove(g, c, spellbook, party); move.Spell.Kind == model.SpellBless {
			t.Fatalf("a companion blessed the party while %s was at %d/%d",
				mate.Name, mate.HP, mate.MaxHP)
		}
	}
}

func TestLineageGivesAndTakes(t *testing.T) {
	for _, l := range model.Lineages {
		if l.Tag == "" || l.Note == "" {
			t.Errorf("%s lineage has no description", l.Kind)
		}
		if l.Discount <= 0 || l.Discount >= 100 {
			t.Errorf("%s offers a %d%% discount, which is not a discount", l.Kind, l.Discount)
		}
		// Every lineage has to cost something, or it is a free upgrade and the
		// only decision left is "take whichever one is on offer".
		gives := l.HPPct > 0 || l.Strength > 0 || l.Dexterity > 0 || l.Speed > 0 || l.Psyche > 0
		takes := l.HPPct < 0 || l.Strength < 0 || l.Dexterity < 0 || l.Speed < 0 || l.Psyche < 0
		if !gives {
			t.Errorf("%s lineage gives nothing", l.Kind)
		}
		if !takes {
			t.Errorf("%s lineage takes nothing, so it is a straight upgrade", l.Kind)
		}
	}
}

// A lineage must shift the stat line and never produce an unusable character —
// a zero-hit-point hireling would be dead on arrival.
func TestRecruitingWithLineageStaysPlayable(t *testing.T) {
	for _, l := range model.Lineages {
		for _, level := range []int{1, 7, 15} {
			g := core.NewRNG(37)
			plain := rules.Recruit(g, "Plain", model.ClassFighter, "", level)
			g = core.NewRNG(37)
			mixed := rules.Recruit(g, "Mixed", model.ClassFighter, l.Kind, level)

			if mixed.Blood != l.Kind {
				t.Fatalf("%s hireling came back with blood %q", l.Kind, mixed.Blood)
			}
			if mixed.MaxHP < 1 || mixed.HP != mixed.MaxHP {
				t.Errorf("%s at level %d arrived at %d/%d hit points", l.Kind, level, mixed.HP, mixed.MaxHP)
			}
			if mixed.Strength < 1 || mixed.Dexterity < 1 || mixed.Speed < 1 {
				t.Errorf("%s at level %d has a non-positive stat: %+v", l.Kind, level, mixed)
			}
			if mixed.Psyche > mixed.MaxPsyche {
				t.Errorf("%s at level %d has %d of %d psyche", l.Kind, level, mixed.Psyche, mixed.MaxPsyche)
			}
			// Same seed, same rolls: the only difference should be the lineage.
			same := mixed.MaxHP == plain.MaxHP && mixed.Strength == plain.Strength &&
				mixed.Dexterity == plain.Dexterity && mixed.Speed == plain.Speed &&
				mixed.MaxPsyche == plain.MaxPsyche
			if same {
				t.Errorf("%s at level %d is statistically identical to an ordinary person", l.Kind, level)
			}
		}
	}
}

// Ancestry is why the cheap hireling on the corner is worth a look.
func TestLineageMakesAHirelingCheaper(t *testing.T) {
	for _, l := range model.Lineages {
		plain := rules.HireCost(8, "", rules.Unknown)
		mixed := rules.HireCost(8, l.Kind, rules.Unknown)
		if mixed >= plain {
			t.Errorf("a part-%s hireling costs %d against an ordinary %d", l.Kind, mixed, plain)
		}
		if mixed < 1 {
			t.Errorf("a part-%s hireling costs %d, which is free", l.Kind, mixed)
		}
	}
}

// The rescue must always be affordable: a run that cannot end because the
// player could not pay to survive is a run that has stopped being a game.
func TestRescueIsAlwaysAffordable(t *testing.T) {
	for _, coins := range []int64{0, 1, 7, 250, 1_000_000} {
		fee := rules.RescueFee(coins)
		if fee < 0 || fee > coins {
			t.Errorf("a rescue with %d coins in the purse cost %d", coins, fee)
		}
	}
	if fee := rules.RescueFee(0); fee != 0 {
		t.Errorf("a rescue with an empty purse cost %d", fee)
	}
	// It has to actually hurt when there is something to take.
	if fee := rules.RescueFee(500); fee < 100 {
		t.Errorf("a rescue with 500 coins cost only %d, which is not a consequence", fee)
	}
	// And it has to stop growing. A share of the purse is the right shape at
	// the bottom and the wrong one at the top: the same rule that stings at
	// level two confiscates at level twelve, and past the cap the cost of dying
	// should hold still while the cost of not having hired anybody keeps
	// rising.
	big := rules.RescueFee(1_000_000)
	if big != rules.RescueFee(50_000) {
		t.Errorf("a rescue cost %d with a million and %d with fifty thousand; "+
			"the fee is still climbing past the cap", big, rules.RescueFee(50_000))
	}
	if big > 250 {
		t.Errorf("the cap let a rescue cost %d", big)
	}
}

func TestReviveAlwaysStandsSomebodyUp(t *testing.T) {
	g := core.NewRNG(41)
	for _, level := range []int{1, 5, 12} {
		c := rules.BuildCharacter(g, model.ClassMage, level)
		for _, power := range []int{0, 1, 25, 60, 500} {
			got := rules.ReviveAmount(c, power)
			if got < 1 {
				t.Errorf("reviving a level %d character at power %d gave %d hit points", level, power, got)
			}
			if got > c.MaxHP {
				t.Errorf("reviving a level %d character at power %d gave %d of %d", level, power, got, c.MaxHP)
			}
		}
	}
}

// Nothing but a revive may raise somebody from zero. Healing used to be a plain
// clamp, which lifted anybody on zero straight back up — making every healing
// potion a resurrection and leaving the revive items with nothing to do.
func TestHealingCannotRaiseTheDead(t *testing.T) {
	g := core.NewRNG(43)
	c := rules.BuildCharacter(g, model.ClassFighter, 5)
	c.HP = 0

	if got := c.Heal(50); got != 0 {
		t.Errorf("healing a fallen character restored %d hit points", got)
	}
	if c.Alive() || c.HP != 0 {
		t.Errorf("healing a fallen character left them at %d hit points", c.HP)
	}

	// Standing them up is a different act, and it works.
	c.HP = rules.ReviveAmount(c, 25)
	if !c.Alive() {
		t.Fatalf("reviving left the character at %d hit points", c.HP)
	}
	// And healing the living still heals.
	c.HP = 1
	if got := c.Heal(5); got != 5 || c.HP != 6 {
		t.Errorf("healing a living character restored %d, leaving %d hit points", got, c.HP)
	}
}

// A companion drinks their own supplies rather than the hero's, and picks
// sensibly: the smallest bottle that covers the damage, so a party does not
// burn a Physician's Draught on a scratch.
func TestCompanionDrinksItsOwnSuppliesSensibly(t *testing.T) {
	g := core.NewRNG(51)
	c := rules.BuildCharacter(g, model.ClassFighter, 6)
	c.Weapon = model.Weapon{Name: "Sword", Strike: 40} // nothing worth casting
	c.Psyche = 0
	c.Bag = []model.Item{
		{Name: "Small Beer", Kind: model.ItemHeal, Power: 8, Count: 2},
		{Name: "Field Poultice", Kind: model.ItemHeal, Power: 20, Count: 1},
		{Name: "Physician's Draught", Kind: model.ItemHeal, Power: 45, Count: 1},
		{Name: "Rank Pelt", Kind: model.ItemTrinket, Count: 1},
	}
	party := []*model.Character{c}

	// A scratch: nothing should be drunk at all.
	c.HP = c.MaxHP - 2
	if move := rules.ChooseAllyMove(g, c, nil, party); move.Kind == rules.AllyUse {
		t.Errorf("a companion drank %q over a scratch", c.Bag[move.Item].Name)
	}

	// Badly hurt, and by roughly a Small Beer's worth: take the Small Beer
	// rather than the Poultice that would also cover it.
	c.MaxHP, c.HP = 12, 5 // 42% left, seven points missing
	move := rules.ChooseAllyMove(g, c, nil, party)
	if move.Kind != rules.AllyUse {
		t.Fatalf("badly hurt with a pack full of bottles, the companion chose %+v", move)
	}
	if got := c.Bag[move.Item].Name; got != "Small Beer" {
		t.Errorf("to heal 7 the companion reached for %q", got)
	}

	// Hurt worse than anything in the bag: take the biggest.
	c.MaxHP = 400
	c.HP = 10
	move = rules.ChooseAllyMove(g, c, nil, party)
	if move.Kind != rules.AllyUse {
		t.Fatalf("nearly dead, the companion chose %+v", move)
	}
	if got := c.Bag[move.Item].Name; got != "Physician's Draught" {
		t.Errorf("nearly dead the companion reached for %q, not the strongest", got)
	}
}

// A healing technique still beats a bottle: psyche refills at a rest and
// potions cost coin.
func TestCompanionPrefersATechniqueToABottle(t *testing.T) {
	g := core.NewRNG(53)
	c := rules.BuildCharacter(g, model.ClassMage, 6)
	c.Weapon = model.Weapon{Name: "Stick", Strike: 1}
	c.Psyche = c.MaxPsyche
	c.HP = 1
	c.Bag = []model.Item{{Name: "Small Beer", Kind: model.ItemHeal, Power: 8, Count: 1}}

	move := rules.ChooseAllyMove(g, c, selfOnly, []*model.Character{c})
	if move.Kind != rules.AllyCast || move.Spell.Kind != model.SpellHeal {
		t.Fatalf("with a heal known and psyche to spare, the companion chose %+v", move)
	}
}

// Nothing in the pack must ever be reached for as if it were a potion.
func TestCompanionNeverDrinksJunk(t *testing.T) {
	g := core.NewRNG(57)
	c := rules.BuildCharacter(g, model.ClassFighter, 4)
	c.Weapon = model.Weapon{Name: "Sword", Strike: 40}
	c.Psyche = 0
	c.HP = 1
	c.Bag = []model.Item{
		{Name: "Rank Pelt", Kind: model.ItemTrinket, Count: 3},
		{Name: "Bottled Nap", Kind: model.ItemPsyche, Power: 12, Count: 1},
		{Name: "Smelling Salts", Kind: model.ItemRevive, Power: 25, Count: 1},
	}
	for i := 0; i < 200; i++ {
		if move := rules.ChooseAllyMove(g, c, nil, []*model.Character{c}); move.Kind == rules.AllyUse {
			t.Fatalf("a companion with no healing drank %q", c.Bag[move.Item].Name)
		}
	}
}
