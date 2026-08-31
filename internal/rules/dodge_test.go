package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// TestOnlyTheThiefDodges. The scheme is one unit of defence each — everybody
// blocks, the Mage additionally owns a pool, and this is the Thief's. Handing a
// share of it to everyone would make it a global buff to defence wearing a
// class's name, which reads as identity and measures as inflation.
func TestOnlyTheThiefDodges(t *testing.T) {
	for _, class := range model.AllClasses {
		c := &model.Character{Class: class, Level: 10, Speed: 20}
		got := rules.DodgeChance(c, 8)
		if class == model.ClassThief {
			if got <= 0 {
				t.Errorf("a Thief with a nine-point speed advantage dodges %.0f%%", got*100)
			}
			continue
		}
		if got != 0 {
			t.Errorf("%s dodges %.1f%%, and dodge is the Thief's unit", class, got*100)
		}
	}
	if rules.DodgeChance(nil, 8) != 0 {
		t.Error("a nil character dodges, which is a crash waiting for a caller")
	}
}

// TestDodgeStaysFlatAcrossTheLevels.
//
// The first version of this scaled hard with the speed gap — two points of
// chance per point of speed — and a Thief outpaces these creatures by three
// points at level three and seventeen by level eleven. So it was worth 16% at
// the bottom and pinned to a 30% cap at the top: a defence that grew fastest
// exactly where the class was already strongest, and did least where it was
// weakest. On the stretch fights that was +15 points of win rate at levels
// eleven and thirteen, which made the Thief the best survivor in the game by
// eighteen points over the Fighter.
//
// What is pinned here is the shape rather than the numbers: the rate must not
// vary much across the speed gaps the game actually produces, so the mechanic
// adds about the same everywhere instead of compounding into dominance.
func TestDodgeStaysFlatAcrossTheLevels(t *testing.T) {
	// The gaps measured off the real rosters: a Thief's speed against the
	// median creature near its level, from level three to thirteen.
	gaps := []int{3, 7, 8, 9, 17, 16}
	lo, hi := 1.0, 0.0
	for _, gap := range gaps {
		c := &model.Character{Class: model.ClassThief, Level: 10, Speed: 10 + gap}
		p := rules.DodgeChance(c, 10)
		if p < lo {
			lo = p
		}
		if p > hi {
			hi = p
		}
	}
	if hi-lo > 0.06 {
		t.Errorf("dodge runs from %.0f%% to %.0f%% across the game's speed gaps; "+
			"that is a defence that compounds with level rather than a unit",
			lo*100, hi*100)
	}
	if hi > 0.20 {
		t.Errorf("dodge reaches %.0f%%, which stops being a unit and becomes an "+
			"argument against wearing armour", hi*100)
	}
	if lo <= 0 {
		t.Error("a Thief with the smallest real speed advantage cannot dodge at all")
	}
}

// TestDodgeIsRolledNotAssumed. Dodged is the only door onto the mechanic and
// both the battle screen and SimulateFight go through it, so a character who
// cannot dodge must never consume a roll differently from one who can — that is
// what would make two identical seeds diverge by class.
func TestDodgeIsRolledNotAssumed(t *testing.T) {
	fighter := &model.Character{Class: model.ClassFighter, Level: 10, Speed: 30}
	for i := 0; i < 200; i++ {
		if rules.Dodged(nil, fighter, 1) {
			t.Fatal("a Fighter dodged")
		}
	}
}
