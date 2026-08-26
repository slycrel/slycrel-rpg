package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

func TestXPCurveIsMonotonic(t *testing.T) {
	prev := int64(-1)
	for lvl := 1; lvl <= 30; lvl++ {
		got := rules.XPForLevel(lvl)
		if got <= prev {
			t.Fatalf("level %d needs %d XP, which is not more than level %d's %d", lvl, got, lvl-1, prev)
		}
		prev = got
	}
	// The original's cubic curve should make level 10 a real commitment but
	// not a wall; these bounds catch an accidental sign or exponent change.
	if got := rules.XPForLevel(10); got < 2000 || got > 20000 {
		t.Errorf("level 10 costs %d XP, which is outside the playable band", got)
	}
}

func TestPendingLevelsMatchesCurve(t *testing.T) {
	c := &model.Character{Level: 1}
	c.TotalXP = rules.XPForLevel(4)
	if n := rules.PendingLevels(c); n != 3 {
		t.Errorf("with exactly level-4 XP, banked levels = %d, want 3", n)
	}
	c.TotalXP = rules.XPForLevel(2) - 1
	if n := rules.PendingLevels(c); n != 0 {
		t.Errorf("one XP short of level 2, banked levels = %d, want 0", n)
	}
}

func TestDamageIsNeverNegative(t *testing.T) {
	g := core.NewRNG(11)
	def := &model.MonsterDef{Name: "Wall", HP: 10, Offense: 1, Defense: 500, Speed: 1, Level: 1}
	m := def.Spawn(g, 1)

	// A weak character against an absurdly armoured monster, and vice versa:
	// both directions must clamp at zero rather than healing the target.
	weak := rules.NewCharacter(g, "Weak", model.ClassMage)
	weak.Level = 9 // push past the low-level mercy floor
	for i := 0; i < 500; i++ {
		if d := rules.PlayerDamage(g, weak, m); d < 0 {
			t.Fatalf("player dealt %d damage", d)
		}
	}
	tank := rules.NewCharacter(g, "Tank", model.ClassFighter)
	tank.Armor = model.Armor{Name: "Slab", Defense: 500}
	for i := 0; i < 500; i++ {
		if d := rules.MonsterDamage(g, tank, m); d < 0 {
			t.Fatalf("monster dealt %d damage", d)
		}
	}
}

func TestLevelUpImprovesTheCharacter(t *testing.T) {
	g := core.NewRNG(5)
	for _, class := range model.AllClasses {
		c := rules.NewCharacter(g, "Subject", class)
		before := *c
		rules.LevelUp(g, c)

		if c.Level != before.Level+1 {
			t.Errorf("%s: level went %d -> %d", class, before.Level, c.Level)
		}
		if c.MaxHP <= before.MaxHP {
			t.Errorf("%s: max HP did not grow (%d -> %d)", class, before.MaxHP, c.MaxHP)
		}
		if c.HP != c.MaxHP {
			t.Errorf("%s: level up left HP at %d/%d instead of full", class, c.HP, c.MaxHP)
		}
		if c.Strength < before.Strength || c.Dexterity < before.Dexterity || c.Speed < before.Speed {
			t.Errorf("%s: a stat went backwards on level up", class)
		}
	}
}

func TestFleeChanceStaysPlayable(t *testing.T) {
	// However lopsided the speed gap, fleeing must never be certain or
	// impossible; a guaranteed escape trivialises the world map and a
	// guaranteed failure makes a bad encounter a death sentence.
	for _, tc := range [][2]int{{1, 99}, {99, 1}, {10, 10}} {
		p := rules.FleeChance(tc[0], tc[1])
		if p <= 0 || p >= 1 {
			t.Errorf("speeds %v gave flee chance %.2f", tc, p)
		}
	}
	if slow, fast := rules.FleeChance(5, 20), rules.FleeChance(20, 5); slow >= fast {
		t.Errorf("being slower (%.2f) should not beat being faster (%.2f)", slow, fast)
	}
}

func TestDispositionCoversTheGrid(t *testing.T) {
	seen := map[rules.Disposition]bool{}
	for _, p := range []float64{0.9, 0.5, 0.1} {
		for _, m := range []float64{0.9, 0.5, 0.1} {
			seen[rules.GetDisposition(p, m)] = true
		}
	}
	if len(seen) != 9 {
		t.Errorf("the 3x3 HP grid produced %d distinct dispositions, want 9", len(seen))
	}
}

func TestMonstersFleeOnlyWhenNearlyDead(t *testing.T) {
	g := core.NewRNG(2)
	healthy := &model.Monster{Def: &model.MonsterDef{}, HP: 100, MaxHP: 100}
	for i := 0; i < 2000; i++ {
		for _, alone := range []bool{false, true} {
			if rules.ChooseMonsterAction(g, healthy, alone) == rules.MonFlee {
				t.Fatal("a monster at full health tried to run")
			}
		}
	}
	dying := &model.Monster{Def: &model.MonsterDef{}, HP: 1, MaxHP: 100}
	fled := false
	for i := 0; i < 200; i++ {
		if rules.ChooseMonsterAction(g, dying, false) == rules.MonFlee {
			fled = true
			break
		}
	}
	if !fled {
		t.Error("a monster at 1%% health never once considered leaving")
	}
}

// TestTheLastOneStandingHoldsItsNerve. A fight that ends because the only thing
// left in it walked off is a fight that ends in an anticlimax, and at levels
// one to three — where an encounter is one or two creatures — that was most of
// what "the monsters keep running away" meant.
//
// Damped, not forbidden: a cornered animal bolting is worth keeping. So this
// asserts the shape (rarer when alone, still possible) rather than a rate.
func TestTheLastOneStandingHoldsItsNerve(t *testing.T) {
	const tries = 4000
	count := func(alone bool) int {
		g := core.NewRNG(7)
		n := 0
		for i := 0; i < tries; i++ {
			dying := &model.Monster{Def: &model.MonsterDef{}, HP: 1, MaxHP: 100}
			if rules.ChooseMonsterAction(g, dying, alone) == rules.MonFlee {
				n++
			}
		}
		return n
	}
	withFriends, lastOne := count(false), count(true)
	if lastOne >= withFriends {
		t.Errorf("the last one standing ran %d times in %d and one with friends ran %d: "+
			"something with nowhere to run to should try it less often",
			lastOne, tries, withFriends)
	}
	if lastOne == 0 {
		t.Error("the last one standing never ran at all; this is meant to be damped, not forbidden")
	}
}

// TestDamageHasNoCliff is the direct regression for the bug the balance
// simulator found: the original switched damage formulas outright at level 5,
// and a fighter's output fell by a third between two fights. The formulas are
// still both there, but the crossing is now spread across a band.
func TestDamageHasNoCliff(t *testing.T) {
	g := core.NewRNG(4)
	def := &model.MonsterDef{Name: "Dummy", HP: 1000, Offense: 1, Defense: 0, Speed: 1, Level: 1}
	m := def.Spawn(g, 1)

	// Real characters, levelled the way the game levels them, carrying the
	// same weapon throughout. Holding the weapon fixed is the point: damage
	// must not fall as you level just because you have not been shopping.
	avg := make([]float64, 15)
	for level := 1; level <= 14; level++ {
		var total, n int
		for c := 0; c < 60; c++ {
			ch := rules.BuildCharacter(g, model.ClassFighter, level)
			ch.Weapon = model.Weapon{Name: "Test", Strike: 6}
			for i := 0; i < 200; i++ {
				total += rules.PlayerDamage(g, ch, m)
				n++
			}
		}
		avg[level] = float64(total) / float64(n)
	}

	for level := 2; level <= 14; level++ {
		prev, cur := avg[level-1], avg[level]
		if cur < prev*0.9 {
			t.Errorf("damage falls %.1f%% from level %d (%.1f) to %d (%.1f); "+
				"that is a cliff, not a curve",
				(1-cur/prev)*100, level-1, prev, level, cur)
		}
	}
	// And it should still be going up overall, not merely not falling.
	if avg[14] <= avg[4] {
		t.Errorf("damage at level 14 (%.1f) is no better than at level 4 (%.1f)", avg[14], avg[4])
	}
}

// TestSpawnScalesEveryCombatStat guards the other half of that fix: only hit
// points used to scale with the encounter, so a low-level monster met deep in
// the world was a punching bag that still hit like a low-level monster.
func TestSpawnScalesEveryCombatStat(t *testing.T) {
	g := core.NewRNG(8)
	def := &model.MonsterDef{
		Name: "Scaler", Level: 2, HP: 40, Offense: 10, Defense: 6, Speed: 8, XP: 30, Coins: 12,
	}
	base := def.Spawn(g, 2)
	deep := def.Spawn(g, 12)

	for _, c := range []struct {
		name      string
		low, high int
	}{
		{"max hp", base.MaxHP, deep.MaxHP},
		{"offense", base.Offense, deep.Offense},
		{"defense", base.Defense, deep.Defense},
		{"xp", base.XP, deep.XP},
		{"coins", base.Coins, deep.Coins},
	} {
		if c.high <= c.low {
			t.Errorf("%s does not scale with encounter level: %d at level 2, %d at level 12",
				c.name, c.low, c.high)
		}
	}
	// Health must outpace offense, or deep encounters become lethal rather
	// than merely longer.
	hpGrowth := float64(deep.MaxHP) / float64(base.MaxHP)
	atkGrowth := float64(deep.Offense) / float64(base.Offense)
	if atkGrowth >= hpGrowth {
		t.Errorf("offense grows %.2fx against health's %.2fx; scaled monsters will one-shot",
			atkGrowth, hpGrowth)
	}
}

// The simulator has to be willing to walk away, or every number it produces
// describes a game in which nobody ever does.
//
// This was not a small error. FleeChance reads speed, the thief has the best
// speed in the game, and the simulator never called it — so the thief's entire
// survival plan was invisible and it was being scored on how well it dies in
// fights it would never have finished. It came out the most fragile class at
// nearly every level, which was an artefact of the measurement rather than a
// fact about the class: with running restored it stops being the outlier.
func TestTheSimulatedPlayerRunsFromALostFight(t *testing.T) {
	g := core.NewRNG(17)
	// Something far too big, met by someone far too small.
	boss := &model.MonsterDef{
		Name: "Much Too Large", Level: 12, HP: 400, Offense: 40, Defense: 6, Speed: 6,
	}

	outcomes := func(class model.Class) (fled, died int) {
		for i := 0; i < 400; i++ {
			c := rules.BuildCharacter(g, class, 3)
			c.Weapon = model.Weapon{Name: "Test", Strike: 4}
			r := rules.SimulateFight(g, c, []*model.MonsterDef{boss}, 12, 60, nil)
			switch {
			case r.Fled:
				fled++
			case r.Died():
				died++
			}
		}
		return
	}

	fled, died := outcomes(model.ClassFighter)
	if fled == 0 {
		t.Fatalf("a hopeless fight ended in %d deaths and no escapes at all", died)
	}

	// And speed has to be worth something: the class built to leave should
	// leave more often than the class built to stand there.
	thiefFled, _ := outcomes(model.ClassThief)
	fighterFled, _ := outcomes(model.ClassFighter)
	if thiefFled <= fighterFled {
		t.Errorf("the thief escaped %d times and the fighter %d; speed is buying nothing",
			thiefFled, fighterFled)
	}
}

// A fight walked away from is not a win and not a death, and the three have to
// stay distinguishable — a report that counted an escape as a death is exactly
// what libelled the thief.
func TestFledIsNeitherWonNorDied(t *testing.T) {
	for _, r := range []rules.FightResult{
		{Won: true},
		{Fled: true},
		{},
	} {
		n := 0
		for _, b := range []bool{r.Won, r.Fled, r.Died()} {
			if b {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%+v reports %d of won/fled/died; the three are meant to partition", r, n)
		}
	}
}

// The false retreat is the thief's answer to a retreat paying nothing. It has
// to stay a gamble: sold less often than a real escape would succeed, and
// punished when it is not bought. If it were as reliable as running it would
// simply be the correct move every time and the flee button would be decoration.
func TestTheFalseRetreatIsAGambleAndNotAnUpgrade(t *testing.T) {
	g := core.NewRNG(41)
	for _, level := range []int{4, 8, 13} {
		c := rules.BuildCharacter(g, model.ClassThief, level)
		for _, monSpeed := range []int{6, 12, 18} {
			feint := rules.FeintChance(c, monSpeed)
			flee := rules.FleeChance(c.Spd(), monSpeed)
			if feint <= 0 || feint >= 1 {
				t.Errorf("feint chance came out %.2f", feint)
			}
			// Against something you could outrun, running has to be the better
			// bet. Against something faster than you it does not: FleeChance
			// floors out down there and lying to a thing you cannot escape is
			// exactly when a lie is worth telling. That niche is the point of
			// the move, so the rule is asserted where it means something.
			if monSpeed > c.Spd() {
				continue
			}
			if feint >= flee {
				t.Errorf("level %d thief (speed %d) against speed %d: feint %.2f, "+
					"flee %.2f; a lie that works as often as running is not a gamble",
					level, c.Spd(), monSpeed, feint, flee)
			}
		}
	}

	// And failing has to hurt more than not trying.
	if rules.FeintPunish(10) <= 10 {
		t.Errorf("a retreat nobody bought costs %d against a normal %d", rules.FeintPunish(10), 10)
	}
}

// Only the thief, and only once they have grown into it.
func TestOnlyTheThiefCanSellARetreat(t *testing.T) {
	g := core.NewRNG(42)
	for _, class := range model.AllClasses {
		for _, level := range []int{1, 3, 4, 10} {
			c := rules.BuildCharacter(g, class, level)
			want := class == model.ClassThief && level >= 4
			if got := rules.CanFeint(c); got != want {
				t.Errorf("%s level %d: CanFeint=%v, want %v", class, level, got, want)
			}
		}
	}
	if rules.CanFeint(nil) {
		t.Error("a nil character can feint")
	}
}

// The decision to try it has to read the target's armour, not just its health.
// PlayerDamage subtracts Defense before the feint bonus multiplies anything, so
// an estimate that ignored armour called the heavily plated things reachable
// when three such blows would not have finished them.
func TestAFeintIsOnlyWorthItAgainstSomethingActuallyReachable(t *testing.T) {
	g := core.NewRNG(43)
	c := rules.BuildCharacter(g, model.ClassThief, 10)
	c.Weapon = model.Weapon{Name: "Test", Strike: 12}

	soft := &model.Monster{HP: 10, Defense: 0}
	if !rules.FeintIsWorthIt(c, soft) {
		t.Error("something on its last legs and unarmoured is not worth a gamble")
	}
	tough := &model.Monster{HP: 10, Defense: 500}
	if rules.FeintIsWorthIt(c, tough) {
		t.Error("armour that eats the whole blow still reads as reachable")
	}
	far := &model.Monster{HP: 100000, Defense: 0}
	if rules.FeintIsWorthIt(c, far) {
		t.Error("something unkillable reads as one blow away")
	}
	if rules.FeintIsWorthIt(c, &model.Monster{HP: 1, Dead: true}) {
		t.Error("a corpse is worth feinting at")
	}
}

// Reputation is two numbers because the corners where they disagree are the
// point. One bar can only say "more" or "less", and a world reacting to a
// single number reacts the same way to a hero and to a loudmouth.
func TestTheTwoAxesComeApart(t *testing.T) {
	for _, c := range []struct {
		name         string
		fame, renown int
		shame        int
		want         rules.Standing
	}{
		{"a nobody", 0, 0, 0, rules.Unknown},
		{"deeds travel, face does not", 9, 0, 0, rules.Rumoured},
		{"face travels, deeds do not", 0, 9, 0, rules.Recognised},
		{"both", 9, 9, 0, rules.Celebrated},
		{"the wrong half travels", 2, 9, 6, rules.Notorious},
		// Shame is weighed against the deeds, not against a fixed number: a
		// scoundrel with a long record of finished errands is a complicated
		// person rather than simply a villain.
		{"more good than bad", 12, 9, 4, rules.Celebrated},
	} {
		ch := &model.Character{Fame: c.fame, Renown: c.renown, Shame: c.shame}
		if got := rules.Read(ch); got != c.want {
			t.Errorf("%s (fame %d, renown %d, shame %d) reads as %q, want %q",
				c.name, c.fame, c.renown, c.shame, got.Name(), c.want.Name())
		}
	}
	if got := rules.Read(nil); got != rules.Unknown {
		t.Errorf("a nil character reads as %q", got.Name())
	}
}

// The shopkeeper marks up the person they recognise, and recognising somebody
// is not thinking well of them. So the legend nobody has placed pays the
// sticker price, which is the whole benefit of that corner.
func TestBeingUnplaceableIsWorthSomethingAtTheCounter(t *testing.T) {
	if p := rules.Rumoured.PriceMultiplier(); p != 1 {
		t.Errorf("an unplaced legend is charged %.2f of the sticker", p)
	}
	if rules.Recognised.PriceMultiplier() <= 1 {
		t.Error("a well-known face is not charged more than a stranger")
	}
	if rules.Notorious.PriceMultiplier() <= rules.Celebrated.PriceMultiplier() {
		t.Error("being disliked costs no more at the counter than being liked")
	}

	// And the mercenary asks the opposite question, so a standing that costs
	// at one counter can pay at the other.
	if rules.Celebrated.HireMultiplier() >= 1 {
		t.Error("nobody will take a cut to have been there")
	}
	if rules.Notorious.HireMultiplier() <= 1 {
		t.Error("there is no hazard pay for following a villain")
	}
}

// TestStartingBandsMatchWhatIsActuallyRolled.
//
// The bands were hoisted out of a switch so the creation screen could colour a
// roll against what it could have been, and the moment there are two readers of
// one table the question becomes whether the table still describes the roller.
// It does not describe it by construction — StartingBands folds in the hit
// point cushion, which NewCharacter adds separately — so this is the seam.
func TestStartingBandsMatchWhatIsActuallyRolled(t *testing.T) {
	g := core.NewRNG(1994)
	for _, class := range model.AllClasses {
		b := rules.StartingBands(class)
		lo := map[string]int{}
		hi := map[string]int{}
		for i := 0; i < 4000; i++ {
			c := rules.NewCharacter(g, "Subject", class)
			for _, s := range []struct {
				name string
				got  int
				band rules.Band
			}{
				{"hit points", c.MaxHP, b.HP},
				{"strength", c.Strength, b.Str},
				{"dexterity", c.Dexterity, b.Dex},
				{"speed", c.Speed, b.Spd},
				{"psyche", c.MaxPsyche, b.Psy},
			} {
				if s.got < s.band.Lo || s.got > s.band.Hi {
					t.Fatalf("%s rolled %s %d, outside the declared band %d-%d",
						class, s.name, s.got, s.band.Lo, s.band.Hi)
				}
				if n, ok := lo[s.name]; !ok || s.got < n {
					lo[s.name] = s.got
				}
				if n, ok := hi[s.name]; !ok || s.got > n {
					hi[s.name] = s.got
				}
			}
			if c.Coins < int64(rules.StartingCoins.Lo) || c.Coins > int64(rules.StartingCoins.Hi) {
				t.Fatalf("%s rolled %d coins, outside %d-%d",
					class, c.Coins, rules.StartingCoins.Lo, rules.StartingCoins.Hi)
			}
		}
		// And the band is not wider than the roll. A band with slack in it
		// would mean the top of it is unreachable, so a perfect roll could
		// never come out green — which is the failure nobody would notice,
		// because the colour would simply be rarer than intended.
		for _, s := range []struct {
			name string
			band rules.Band
		}{
			{"hit points", b.HP}, {"strength", b.Str}, {"dexterity", b.Dex},
			{"speed", b.Spd}, {"psyche", b.Psy},
		} {
			if lo[s.name] != s.band.Lo || hi[s.name] != s.band.Hi {
				t.Errorf("%s %s ranges %d-%d over 4000 rolls, band claims %d-%d",
					class, s.name, lo[s.name], hi[s.name], s.band.Lo, s.band.Hi)
			}
		}
	}
}

// TestBandFracSpansItsRange, including the degenerate case. A band with no
// spread has nothing to be lucky about and has to read as middling, or a stat
// that cannot vary would be permanently coloured.
func TestBandFracSpansItsRange(t *testing.T) {
	b := rules.Band{Lo: 4, Hi: 8}
	if got := b.Frac(4); got != 0 {
		t.Errorf("the floor of a band is %v, want 0", got)
	}
	if got := b.Frac(8); got != 1 {
		t.Errorf("the top of a band is %v, want 1", got)
	}
	if got := b.Frac(6); got != 0.5 {
		t.Errorf("the middle of a band is %v, want 0.5", got)
	}
	// Out of range clamps rather than running past the ends, since a lineage
	// or a charm can push a stat outside what was rolled.
	if got := b.Frac(99); got != 1 {
		t.Errorf("above the band reads %v, want 1", got)
	}
	if got := b.Frac(-99); got != 0 {
		t.Errorf("below the band reads %v, want 0", got)
	}
	if got := (rules.Band{Lo: 3, Hi: 3}).Frac(3); got != 0.5 {
		t.Errorf("a band with no spread reads %v, want the middle", got)
	}
}

// --- the focus slot and getting your breath back ---------------------------

// The load-bearing property of CatchBreath: it is a discount on a fight, never
// a profit from one. A refund larger than what the fight cost means the world
// cannot wear a character down, and endurance runs away to infinity.
//
// That is not hypothetical. The first draft handed back a flat tenth of the
// maximum pools, which at level one is more than a level-one fight takes, and
// ENDURANCE went from sixteen fights on one rest to forty — the cap the harness
// stops counting at.
func TestCatchingYourBreathIsADiscountAndNeverAProfit(t *testing.T) {
	g := core.NewRNG(5)
	for i := 0; i < 400; i++ {
		c := rules.BuildCharacter(g, model.ClassFighter, 1+g.Intn(14))
		lost := g.Between(1, c.MaxHP)
		spent := g.Between(0, c.MaxPsyche)
		c.HP = core.Max(1, c.MaxHP-lost)
		c.Psyche = core.Max(0, c.MaxPsyche-spent)
		hp, ps := rules.CatchBreath(c, lost, spent)
		if hp > lost || ps > spent {
			t.Fatalf("a fight costing %d/%d handed back %d/%d", lost, spent, hp, ps)
		}
		if c.HP > c.MaxHP || c.Psyche > c.MaxPsyche {
			t.Fatalf("catching breath overfilled: %d/%d hp, %d/%d psyche",
				c.HP, c.MaxHP, c.Psyche, c.MaxPsyche)
		}
	}
}

// Nobody gets their breath back off the floor. The rescue and the reload are
// what a defeat is for; a corpse quietly topping itself up would make both of
// them unreachable.
func TestTheFallenGetNothingBack(t *testing.T) {
	g := core.NewRNG(6)
	c := rules.BuildCharacter(g, model.ClassMage, 7)
	c.HP, c.Psyche = 0, 0
	if hp, ps := rules.CatchBreath(c, 50, 12); hp != 0 || ps != 0 {
		t.Fatalf("somebody on the floor got %d hp and %d psyche back", hp, ps)
	}
}

// Technique costs what the class pays, and the surcharge can never round a
// price down to nothing — a rate that did would hand the worst caster in the
// game a free move for being bad at casting.
func TestTechniqueIsDearerForTheClassesThatAreNotMadeOfIt(t *testing.T) {
	g := core.NewRNG(8)
	mage := rules.BuildCharacter(g, model.ClassMage, 5)
	fighter := rules.BuildCharacter(g, model.ClassFighter, 5)
	thief := rules.BuildCharacter(g, model.ClassThief, 5)

	for _, cost := range []int{1, 2, 4, 8, 12} {
		s := model.Spell{Cost: cost}
		m, f, th := rules.PsycheCost(mage, s), rules.PsycheCost(fighter, s), rules.PsycheCost(thief, s)
		if m != cost {
			t.Errorf("a mage pays %d for a technique listed at %d; the list is their price", m, cost)
		}
		if f < th || th < m {
			t.Errorf("cost %d: mage %d, thief %d, fighter %d — the order is meant to be "+
				"mage cheapest and fighter dearest", cost, m, th, f)
		}
		if m < 1 || th < 1 || f < 1 {
			t.Errorf("cost %d rounded somebody's technique to free: %d/%d/%d", cost, m, th, f)
		}
	}
}

// The claim the whole focus slot rests on: a caster out of psyche still has
// something to do, and what it is worth is a read of the rod they bought.
//
// Without this a Mage's empty round is a poke with a stick worth strike five,
// which is worse than a Fighter with a table leg — and the class would spend
// every round it could not pay for being a bad fighter.
func TestACasterOutOfPsycheStillHasSomethingToThrow(t *testing.T) {
	g := core.NewRNG(9)
	def := &model.MonsterDef{Name: "Dummy", Level: 5, HP: 200, Offense: 1, Defense: 4, Ward: 4, Speed: 1}

	weak := rules.BuildCharacter(g, model.ClassMage, 5)
	weak.Weapon = model.Weapon{Name: "Twig", Kind: model.WeaponFocus, Strike: 2, Focus: 4}
	strong := rules.BuildCharacter(g, model.ClassMage, 5)
	strong.Weapon = model.Weapon{Name: "Rod", Kind: model.WeaponFocus, Strike: 2, Focus: 16}

	avg := func(c *model.Character) float64 {
		total := 0
		const n = 4000
		for i := 0; i < n; i++ {
			total += rules.PlayerAttack(g, c, def.Spawn(g, 5), 0, 0).Damage
		}
		return float64(total) / n
	}
	lo, hi := avg(weak), avg(strong)
	if lo < 1 {
		t.Errorf("a caster's free round lands for %.1f, which is nothing at all", lo)
	}
	if hi <= lo {
		t.Errorf("a better rod throws %.1f against the cheap one's %.1f; the focus ladder "+
			"is the mage's whole shopping list and it has to buy something", hi, lo)
	}
}
