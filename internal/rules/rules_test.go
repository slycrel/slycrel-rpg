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
		if rules.ChooseMonsterAction(g, healthy) == rules.MonFlee {
			t.Fatal("a monster at full health tried to run")
		}
	}
	dying := &model.Monster{Def: &model.MonsterDef{}, HP: 1, MaxHP: 100}
	fled := false
	for i := 0; i < 200; i++ {
		if rules.ChooseMonsterAction(g, dying) == rules.MonFlee {
			fled = true
			break
		}
	}
	if !fled {
		t.Error("a monster at 1%% health never once considered leaving")
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
