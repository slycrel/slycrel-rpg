package gamedata_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// TestTheCharmSlotIsChosenRatherThanTakenLast.
//
// The balanced build put `cs[len(cs)-1]` in the charm slot — the last row of
// charms.json — and the reasoning was a claim about the content: every charm
// gives with one hand and takes with the other, so there is no better one, so
// any pick is as good as any other. The arbiter disagrees with the premise. In
// three bands out of four one charm wins on win rate *and* on fights per rest,
// for every class, and the file order landed on the loser in three of four. It
// cost a Thief at level eleven 12.5 points of win rate and a third of its
// endurance.
//
// This does not pin which charm wins — that is the CHARMS section's job, and
// the answer has to be free to move when the tables do. It pins that the pick
// is a function of what the charm does, which is the property file order never
// had.
func TestTheCharmSlotIsChosenRatherThanTakenLast(t *testing.T) {
	tables := load(t)

	// Reordering the table must not change what anybody wears. This is the
	// whole defect, stated directly: a content edit that moves two lines in a
	// JSON file should not re-equip the game.
	for level := 1; level <= 14; level++ {
		before := &model.Character{Class: model.ClassFighter, Level: level}
		tables.Equip(before)

		flipped := *tables
		flipped.Charms = append([]model.Charm(nil), tables.Charms...)
		for i, j := 0, len(flipped.Charms)-1; i < j; i, j = i+1, j-1 {
			flipped.Charms[i], flipped.Charms[j] = flipped.Charms[j], flipped.Charms[i]
		}
		after := &model.Character{Class: model.ClassFighter, Level: level}
		flipped.Equip(after)

		if before.Charm.Name != after.Charm.Name {
			t.Errorf("level %d: reversing charms.json changed the charm worn from %q to %q",
				level, before.Charm.Name, after.Charm.Name)
		}
	}
}

// TestTheCharmWornIsTheBestOfItsBand. Whatever CharmValue believes, the build
// has to act on it: a scoring function the equipper does not consult is a
// comment with arithmetic in it.
func TestTheCharmWornIsTheBestOfItsBand(t *testing.T) {
	tables := load(t)
	for level := 1; level <= 14; level++ {
		worn := &model.Character{Class: model.ClassFighter, Level: level}
		tables.Equip(worn)
		if !worn.Charm.Worn() {
			continue
		}
		for _, c := range tables.Charms {
			if c.Tier != worn.Charm.Tier {
				continue
			}
			if gamedata.CharmValue(c) > gamedata.CharmValue(worn.Charm) {
				t.Errorf("level %d wears %q (%.1f) with %q (%.1f) on the same shelf",
					level, worn.Charm.Name, gamedata.CharmValue(worn.Charm),
					c.Name, gamedata.CharmValue(c))
			}
		}
	}
}

// TestEveryCharmStillTakesSomething.
//
// The rule the charm table is built on, restated against the scoring: a charm
// with nothing in its negative column is a pure upgrade, and a table of those
// makes "did I get the good one" the only question worth asking.
//
// Deliberately not a test that the *values* balance — CHARMS says they do not,
// and that is a finding to act on rather than a test to fail on every run. What
// this holds is the weaker, authored thing: everything gives and everything
// takes.
func TestEveryCharmStillTakesSomething(t *testing.T) {
	tables := load(t)
	for _, c := range tables.Charms {
		b := c.Bonus
		gives := b.Ward > 0 || b.Strike > 0 || b.Defense > 0 || b.Strength > 0 ||
			b.Dexterity > 0 || b.Speed > 0 || b.Psyche > 0
		takes := b.Ward < 0 || b.Strike < 0 || b.Defense < 0 || b.Strength < 0 ||
			b.Dexterity < 0 || b.Speed < 0 || b.Psyche < 0
		if !gives {
			t.Errorf("%q gives nothing", c.Name)
		}
		if !takes {
			t.Errorf("%q takes nothing, which makes it a pure upgrade", c.Name)
		}
	}
}
