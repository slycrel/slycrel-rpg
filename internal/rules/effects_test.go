package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// Applying the same condition twice must fold rather than append, or a fight
// where somebody is poisoned six times carries six entries that each tick
// separately — which is both a runaway list and six times the damage.
func TestApplyFoldsMatchingConditions(t *testing.T) {
	var list model.Effects
	list = rules.Apply(list, model.Effect{Kind: model.EffectPoison, Power: 3, Rounds: 2})
	list = rules.Apply(list, model.Effect{Kind: model.EffectPoison, Power: 4, Rounds: 5})
	list = rules.Apply(list, model.Effect{Kind: model.EffectBurn, Power: 2, Rounds: 3})

	if len(list) != 2 {
		t.Fatalf("two kinds applied three times produced %d entries: %+v", len(list), list)
	}
	if got := rules.Power(list, model.EffectPoison); got != 7 {
		t.Errorf("stacked poison came to power %d, want 7", got)
	}
	// The longer of the two durations wins; refreshing must not shorten it.
	if list[0].Rounds != 5 {
		t.Errorf("stacked poison runs for %d rounds, want the longer 5", list[0].Rounds)
	}
}

// Forever outlasts anything finite, in either order.
func TestForeverOutlastsAnyCount(t *testing.T) {
	for _, order := range [][2]int{{model.Forever, 3}, {3, model.Forever}} {
		var list model.Effects
		list = rules.Apply(list, model.Effect{Kind: model.EffectWeaken, Power: 1, Rounds: order[0]})
		list = rules.Apply(list, model.Effect{Kind: model.EffectWeaken, Power: 1, Rounds: order[1]})
		if list[0].Rounds != model.Forever {
			t.Errorf("applying %v left a duration of %d, want Forever", order, list[0].Rounds)
		}
	}
	// And Advance must never expire it.
	list := model.Effects{{Kind: model.EffectWeaken, Power: 2, Rounds: model.Forever}}
	for i := 0; i < 50; i++ {
		var expired []model.EffectKind
		list, expired = rules.Advance(list)
		if len(expired) != 0 || len(list) != 1 {
			t.Fatalf("a permanent condition expired on round %d", i)
		}
	}
}

func TestAdvanceRunsTheClockDownAndReports(t *testing.T) {
	list := model.Effects{
		{Kind: model.EffectPoison, Power: 3, Rounds: 2},
		{Kind: model.EffectBurn, Power: 5, Rounds: 1},
	}
	list, expired := rules.Advance(list)
	if len(expired) != 1 || expired[0] != model.EffectBurn {
		t.Fatalf("after one round the expired list was %v, want just burn", expired)
	}
	if len(list) != 1 || list[0].Kind != model.EffectPoison || list[0].Rounds != 1 {
		t.Fatalf("poison came out as %+v, want one round left", list)
	}

	list, expired = rules.Advance(list)
	if len(list) != 0 || len(expired) != 1 || expired[0] != model.EffectPoison {
		t.Fatalf("after the second round: list %+v, expired %v", list, expired)
	}
}

// A blessing and a weakening are added at one point in the damage calculation,
// so they have to cancel rather than each being applied somewhere different.
func TestOffenseModNets(t *testing.T) {
	list := model.Effects{
		{Kind: model.EffectBless, Power: 4, Rounds: model.Forever},
		{Kind: model.EffectWeaken, Power: 6, Rounds: model.Forever},
	}
	if got := rules.OffenseMod(list); got != -2 {
		t.Errorf("a +4 blessing against a -6 weakening netted %d, want -2", got)
	}
	if got := rules.OffenseMod(nil); got != 0 {
		t.Errorf("nothing at all netted %d", got)
	}
}

// Ticking damage must never be free — a condition that rolls zero is a line in
// the transcript that changes nothing, which reads as a bug.
func TestTickDamageIsNeverFree(t *testing.T) {
	g := core.NewRNG(17)
	for power := 1; power <= 20; power++ {
		list := model.Effects{
			{Kind: model.EffectPoison, Power: power, Rounds: 3},
			{Kind: model.EffectBurn, Power: power, Rounds: 3},
		}
		for i := 0; i < 100; i++ {
			ticks := rules.TickDamage(g, list)
			if len(ticks) != 2 {
				t.Fatalf("power %d produced %d ticks, want one each for poison and burn", power, len(ticks))
			}
			for _, tk := range ticks {
				if tk.Damage < 1 {
					t.Fatalf("%s at power %d ticked for %d", tk.Kind, power, tk.Damage)
				}
			}
		}
	}
	// Conditions that do not tick must produce nothing rather than a zero.
	quiet := model.Effects{
		{Kind: model.EffectWeaken, Power: 3, Rounds: 2},
		{Kind: model.EffectBless, Power: 3, Rounds: 2},
		{Kind: model.EffectStun, Power: 1, Rounds: 1},
	}
	if ticks := rules.TickDamage(g, quiet); len(ticks) != 0 {
		t.Errorf("non-ticking conditions produced %d ticks", len(ticks))
	}
}

// Burning is the shorter, sharper one: for the same rating it has to hit
// harder than poison, or there is no reason for both to exist.
func TestBurningHitsHarderThanPoison(t *testing.T) {
	g := core.NewRNG(19)
	const power, rolls = 8, 4000
	var poison, burn int
	for i := 0; i < rolls; i++ {
		poison += rules.TickDamage(g, model.Effects{{Kind: model.EffectPoison, Power: power, Rounds: 9}})[0].Damage
		burn += rules.TickDamage(g, model.Effects{{Kind: model.EffectBurn, Power: power, Rounds: 9}})[0].Damage
	}
	if burn <= poison {
		t.Errorf("over %d rolls burning did %d and poison %d; burning should hit harder", rolls, burn, poison)
	}
}

// A cure takes the harm and leaves the help. An antidote that also cancelled
// your own blessing would be a trap.
func TestCleanseTakesTheHarmAndLeavesTheHelp(t *testing.T) {
	list := model.Effects{
		{Kind: model.EffectPoison, Power: 3, Rounds: 3},
		{Kind: model.EffectBless, Power: 4, Rounds: model.Forever},
		{Kind: model.EffectBurn, Power: 5, Rounds: 2},
		{Kind: model.EffectQuicken, Power: 2, Rounds: model.Forever},
	}
	list, removed := rules.Cleanse(list)
	if len(removed) != 2 {
		t.Fatalf("cleansing removed %v, want poison and burn", removed)
	}
	if len(list) != 2 {
		t.Fatalf("cleansing left %+v, want the blessing and the quickening", list)
	}
	for _, e := range list {
		if e.Kind.Harmful() {
			t.Errorf("cleansing left %s behind", e.Kind)
		}
	}
	if got := rules.OffenseMod(list); got != 4 {
		t.Errorf("the surviving blessing is worth %d, want 4", got)
	}
}

func TestRollAfflictionRespectsItsChance(t *testing.T) {
	g := core.NewRNG(23)
	if _, ok := rules.RollAffliction(g, nil); ok {
		t.Error("a monster with no affliction inflicted one")
	}
	never := &model.Affliction{Kind: model.EffectPoison, Power: 3, Chance: 0}
	for i := 0; i < 200; i++ {
		if _, ok := rules.RollAffliction(g, never); ok {
			t.Fatal("a zero-chance affliction landed")
		}
	}
	always := &model.Affliction{Kind: model.EffectPoison, Power: 3, Chance: 100}
	for i := 0; i < 200; i++ {
		e, ok := rules.RollAffliction(g, always)
		if !ok {
			t.Fatal("a certain affliction failed to land")
		}
		if e.Kind != model.EffectPoison || e.Power < 1 || e.Rounds < 1 {
			t.Fatalf("a certain affliction produced %+v", e)
		}
	}
	// An affliction with no duration given must not be permanent by accident.
	e, ok := rules.RollAffliction(g, &model.Affliction{
		Kind: model.EffectPoison, Power: 2, Chance: 100,
	})
	if !ok || e.Rounds == model.Forever || e.Rounds < 1 {
		t.Fatalf("an affliction with no stated duration came out as %+v", e)
	}
}

// Stunning is spent by the turn it costs, which is what Remove is for.
func TestRemoveClearsOneKind(t *testing.T) {
	list := model.Effects{
		{Kind: model.EffectStun, Power: 1, Rounds: 1},
		{Kind: model.EffectPoison, Power: 3, Rounds: 3},
	}
	if !rules.Has(list, model.EffectStun) {
		t.Fatal("the stun is not in force")
	}
	list = rules.Remove(list, model.EffectStun)
	if rules.Has(list, model.EffectStun) {
		t.Error("the stun survived being spent")
	}
	if !rules.Has(list, model.EffectPoison) {
		t.Error("removing the stun also took the poison")
	}
}

// --- the barrier ----------------------------------------------------------

// A barrier is spent, not timed, and everything it takes is damage that never
// reaches the body. Both halves matter: a pool that decremented but still let
// the blow through would be decoration, and one that never ran out would make
// the class it belongs to unkillable.
func TestABarrierIsSpentRatherThanWornOut(t *testing.T) {
	list := rules.Apply(nil, model.Effect{
		Kind: model.EffectBarrier, Power: 10, Rounds: model.Forever,
	})

	list, through, took := rules.Soak(list, 4)
	if through != 0 || took != 4 {
		t.Fatalf("a 4-point blow against a 10-point barrier: %d through, %d taken", through, took)
	}
	if got := rules.Power(list, model.EffectBarrier); got != 6 {
		t.Fatalf("barrier is at %d after taking 4 of 10", got)
	}

	// The blow that empties it: the pool takes what it has and the rest lands.
	list, through, took = rules.Soak(list, 15)
	if through != 9 || took != 6 {
		t.Fatalf("a 15-point blow against a 6-point barrier: %d through, %d taken", through, took)
	}
	if rules.Has(list, model.EffectBarrier) {
		t.Errorf("a spent barrier is still standing; it is a pool, not a duration")
	}

	// And nothing is soaked once it is gone.
	if _, through, took = rules.Soak(list, 20); through != 20 || took != 0 {
		t.Errorf("a gone barrier soaked %d of 20", took)
	}
}

// Advancing the round must not tick a barrier away. It is the one condition in
// the list with no clock on it: it lasts until something spends it, which is
// what makes it a shield rather than a blessing.
func TestABarrierDoesNotExpireOnItsOwn(t *testing.T) {
	list := rules.Apply(nil, model.Effect{
		Kind: model.EffectBarrier, Power: 12, Rounds: model.Forever,
	})
	for i := 0; i < 20; i++ {
		list, _ = rules.Advance(list)
	}
	if got := rules.Power(list, model.EffectBarrier); got != 12 {
		t.Errorf("after twenty rounds the barrier is at %d, not 12", got)
	}
}

// Only what is on the off arm raises one, and only the classes that may hold
// one have anything to raise.
func TestOnlyATalismanPutsUpABarrier(t *testing.T) {
	talisman := model.Shield{Name: "Sigil", Kind: model.ShieldTalisman, Absorb: 20}
	plank := model.Shield{Name: "Barrel Lid", Defense: 2}

	mage := &model.Character{Class: model.ClassMage, Shield: talisman}
	if n := rules.Raise(mage); n != 20 {
		t.Errorf("a mage holding a talisman raised %d", n)
	}
	fighter := &model.Character{Class: model.ClassFighter, Shield: plank}
	if n := rules.Raise(fighter); n != 0 {
		t.Errorf("a plank raised a barrier of %d", n)
	}
	bare := &model.Character{Class: model.ClassMage}
	if n := rules.Raise(bare); n != 0 {
		t.Errorf("an empty arm raised a barrier of %d", n)
	}

	// And the gate: nobody but a caster may hold the thing in the first place.
	if model.CanHoldShield(model.ClassFighter, talisman) ||
		model.CanHoldShield(model.ClassThief, talisman) {
		t.Error("a talisman is holdable by somebody who does not need one")
	}
	if model.CanHoldShield(model.ClassMage, plank) {
		t.Error("a mage is holding a plank, which is the hand they cast with")
	}
}
