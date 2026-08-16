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
