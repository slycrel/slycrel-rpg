package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// rate is how often a second swing lands, over enough rolls to be a rate.
func rate(class model.Class, speed int) float64 {
	g := core.NewRNG(31337)
	c := &model.Character{Class: class, Level: 10, Speed: speed}
	hits := 0
	const n = 20000
	for i := 0; i < n; i++ {
		if rules.ExtraSwing(g, c) {
			hits++
		}
	}
	return float64(hits) / n
}

// TestOnlyTheFighterSwingsTwice. The other two classes have their own actives —
// a counter off a dodge, a barrier that refills — and a third one here would be
// a global buff to damage wearing a class's name.
func TestOnlyTheFighterSwingsTwice(t *testing.T) {
	for _, class := range []model.Class{model.ClassThief, model.ClassMage} {
		if got := rate(class, 40); got != 0 {
			t.Errorf("%s swings twice %.1f%% of the time", class, got*100)
		}
	}
	if rate(model.ClassFighter, 40) <= 0 {
		t.Error("a very fast Fighter never swings twice")
	}
	g := core.NewRNG(1)
	if rules.ExtraSwing(g, nil) {
		t.Error("a nil character swung twice")
	}
}

// TestTheSecondSwingArrivesWithLevellingAndStaysRare.
//
// Two properties, and both are about magnitude rather than flavour. A weapon
// band is +5 strike; a second swing is an entire extra attack, worth more on
// the rounds it fires than every gear step in the game together — which is why
// it is the last thing in the class-identity scheme and the smallest number in
// it.
//
// It has to be absent early, because "as you levelled" is the whole of what the
// original's version meant and a level-one Fighter earning it would make it a
// class trait rather than a reward. And it has to stay rare at the top, because
// the report cannot re-price it the way it can re-price a charm.
func TestTheSecondSwingArrivesWithLevellingAndStaysRare(t *testing.T) {
	// A Fighter rolls 6-9 speed at level one and climbs about a point a level.
	if got := rate(model.ClassFighter, 8); got != 0 {
		t.Errorf("a level-one Fighter swings twice %.1f%% of the time", got*100)
	}
	mid := rate(model.ClassFighter, 17)
	if mid <= 0 {
		t.Error("a mid-game Fighter never swings twice")
	}
	top := rate(model.ClassFighter, 22)
	if top <= mid {
		t.Errorf("the rate does not grow with speed: %.1f%% at 17, %.1f%% at 22",
			mid*100, top*100)
	}
	if top > 0.20 {
		t.Errorf("a top-end Fighter swings twice %.1f%% of the time, which is a "+
			"second weapon nobody paid for", top*100)
	}
}
