package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

func withTalisman(pool int) *model.Character {
	return &model.Character{
		Class: model.ClassMage, Level: 10, MaxHP: 40, HP: 40,
		Shield: model.Shield{Name: "Ward-Knot", Kind: model.ShieldTalisman, Absorb: pool},
	}
}

// TestOnlyATalismanSiphons. The gate is the item rather than the class, because
// the talisman *is* the unit: a Mage holding nothing on that arm has no pool to
// refill, and nobody else can hold one at all.
func TestOnlyATalismanSiphons(t *testing.T) {
	bare := &model.Character{Class: model.ClassMage, Level: 10}
	if got := rules.Siphon(bare, 20); got != 0 {
		t.Errorf("a Mage with an empty arm siphoned %d", got)
	}
	plank := &model.Character{
		Class: model.ClassFighter, Level: 10,
		Shield: model.Shield{Name: "Buckler", Defense: 4},
	}
	if got := rules.Siphon(plank, 20); got != 0 {
		t.Errorf("a plank siphoned %d, and a plank is not a pool", got)
	}
	if got := rules.Siphon(nil, 20); got != 0 {
		t.Errorf("a nil character siphoned %d", got)
	}
	if got := rules.Siphon(withTalisman(14), 20); got <= 0 {
		t.Error("a talisman did not rebuild itself out of damage dealt")
	}
}

// TestTheSiphonHasACeiling.
//
// Without one, a long fight against many weak things ends with a Mage behind a
// wall bigger than its own hit points — which is a different class rather than
// a repaired one, and the repair was for a pool that runs out too early rather
// than a pool that is too small.
func TestTheSiphonHasACeiling(t *testing.T) {
	c := withTalisman(14)
	rules.Raise(c)
	for i := 0; i < 200; i++ {
		rules.Siphon(c, 50)
	}
	have := 0
	for _, e := range c.Active {
		if e.Kind == model.EffectBarrier {
			have += e.Power
		}
	}
	if have > 14*3 {
		t.Errorf("two hundred blows left a barrier of %d on a pool of 14", have)
	}
	if have <= 14 {
		t.Errorf("the barrier is %d, so the siphon never added anything", have)
	}
}

// TestTheSiphonStaysOnePool. Soak drains barriers in list order, so a siphon
// that appended a fresh effect per blow would leave a stack of small pools
// drained in an order nobody chose — and a hundred of them on the character's
// effect list, which the condition pips draw.
func TestTheSiphonStaysOnePool(t *testing.T) {
	c := withTalisman(14)
	rules.Raise(c)
	for i := 0; i < 20; i++ {
		rules.Siphon(c, 10)
	}
	n := 0
	for _, e := range c.Active {
		if e.Kind == model.EffectBarrier {
			n++
		}
	}
	if n != 1 {
		t.Errorf("twenty blows produced %d separate barriers, want one", n)
	}
}

// TestTheSiphonReportsWhatItGave. A pool that silently refills is a pool the
// player never learns they have, so the transcript needs the figure.
func TestTheSiphonReportsWhatItGave(t *testing.T) {
	c := withTalisman(20)
	rules.Raise(c)
	gain := rules.Siphon(c, 30)
	if gain <= 0 {
		t.Fatal("the siphon reported nothing")
	}
	have := 0
	for _, e := range c.Active {
		if e.Kind == model.EffectBarrier {
			have += e.Power
		}
	}
	if have != 20+gain {
		t.Errorf("reported %d back, but the pool went from 20 to %d", gain, have)
	}
}
