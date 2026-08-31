package gamedata

import "github.com/slycrel/slycrel-rpg/internal/model"

// What a companion does with the cut they have been taking.
//
// The cut used to be a subtraction and nothing else. A percentage came off
// every haul, left the purse and went nowhere at all; what it bought was a
// companion who re-armed for free on every level-up, which is the same
// arrangement with the arithmetic hidden. The money and the gear had no
// relationship to each other, so neither could be read against the other —
// a player could not tell whether the standing charge on everything they
// found was expensive, because nothing on the screen was its other half.
//
// So the cut is a purse now. It buys one piece at a time, at a counter that
// has to exist in the place they are standing in, and whatever comes off goes
// back to the person whose hauls paid for the replacement. A companion is as
// well equipped as you have made them rich, which is what a standing claim on
// every haul was always supposed to mean.

// Counter is where a piece of equipment is bought.
//
// A village has a smith and an apothecary; only a town runs to somebody who
// fits armour. A companion cannot spend money in a place with nowhere to spend
// it, and the split here is the same one the shop screen stocks its shelves
// from: the smith beats metal, so weapons and planks are his, and the armourer
// fits worn things, which is armour, charms and the caster's talisman.
type Counter int

const (
	CounterSmith Counter = iota
	CounterArmorer
)

// Want is a piece a companion is putting money aside for, and where they would
// have to be standing to buy it.
type Want struct {
	Gear model.Carried
	Cost int
	At   Counter
}

// Wants is the next thing a companion is saving for, or false if there is
// nothing they are behind on. It is what their sheet shows and what a gift is
// measured against.
func (t *Tables) Wants(c *model.Character) (Want, bool) {
	list := t.wants(c)
	if len(list) == 0 {
		return Want{}, false
	}
	return list[0], true
}

// wants lists every slot where what a companion is wearing is behind what the
// balanced build would give somebody of their level, in the order EquipAs
// fills them: the sword, then the coat, then the arm, then the charm.
//
// Equip is the target rather than a second opinion about one. That is the
// whole reason this is safe to add — "on curve" already has exactly one
// definition in the game, and a companion's kit is now a lagging indicator of
// it rather than a copy of the rule.
//
// The comparison is on price rather than on the number each slot exists for,
// and that is deliberate twice over. It is the only comparison that works in
// all four slots — a charm has no better, only dearer, which is the rule
// TestTheShelfNeverGradesACharm holds at the counter — and it is what somebody
// saving up actually compares. A mercenary reads a price tag.
func (t *Tables) wants(c *model.Character) []Want {
	if c == nil {
		return nil
	}
	// A copy, so asking what somebody wants never dresses them. Equip touches
	// the four worn slots and nothing else, so the shared pack and bag behind
	// this copy are not in play.
	probe := *c
	t.Equip(&probe)

	var out []Want
	behind := func(gear model.Carried, have int, at Counter) {
		if cost := gear.Cost(); cost > have {
			out = append(out, Want{Gear: gear, Cost: cost, At: at})
		}
	}

	w := probe.Weapon
	behind(model.Carried{Weapon: &w}, c.Weapon.Cost, CounterSmith)
	a := probe.Armor
	behind(model.Carried{Armor: &a}, c.Armor.Cost, CounterArmorer)
	if s := probe.Shield; s.Worn() {
		// A plank is beaten metal and a talisman is a worn thing, which is the
		// same line the two counters are already divided along.
		at := CounterSmith
		if s.Barrier() {
			at = CounterArmorer
		}
		// What they have on that arm, whichever thing it is. Reading
		// c.Shield.Cost alone made a companion holding a gifted off-hand
		// weapon look like somebody with an empty arm: they registered as
		// behind on a shield, bought one at the next smith out of their own
		// savings, and handed the gift back to the pack. Nothing was destroyed
		// and no invariant broke — the player's one way of steering a
		// companion's kit was simply undone by the recipient at the first
		// counter, quietly, which is worse than a bug that shows.
		behind(model.Carried{Shield: &s}, armCost(c), at)
	}
	if ch := probe.Charm; ch.Worn() {
		behind(model.Carried{Charm: &ch}, c.Charm.Cost, CounterArmorer)
	}
	return out
}

// armCost is what is already on the off arm, in the unit Wants compares in.
//
// One arm, one thing: a plank, a talisman, or the thief's second weapon. A
// companion is never sent shopping for an off-hand weapon — the shelf is
// narrowed to the class and the balanced build does not take one, so "on
// curve" for the arm is a shield — but somebody *holding* one is not behind,
// and this is what says so.
func armCost(c *model.Character) int {
	if c.Sidearm.Worn() {
		return c.Sidearm.Cost
	}
	return c.Shield.Cost
}

// Shop spends a companion's own savings on the best thing the town can sell
// them, one piece at a time, and reports what they bought and what came off.
//
// open says which counters this place actually has. It is asked rather than
// derived from the kind of settlement because the answer is already built:
// BuildLocal decides which shops a place gets, and a second copy of that rule
// here would be a companion coming out of a village holding armour nobody in
// it sells.
//
// They will take a lower want when the counter for a higher one is shut, which
// is a person spending what is in their hand rather than carrying it back out
// of a town — and since the list is in priority order, that only ever trades
// downward.
//
// They pay the sticker price rather than the hero's. A counter marks up the
// face it recognises, and nobody in the town has heard of the one carrying the
// bags.
func (t *Tables) Shop(c *model.Character, open func(Counter) bool) (bought, off []model.Carried) {
	if c == nil {
		return nil, nil
	}
	// Four slots, so four purchases is the most a single visit can be. Bounded
	// rather than looped until nothing changes: every buy sets a slot to
	// exactly what the curve asked for, so it stops being a want — but a
	// shopping trip that can only end by agreeing with itself is one table
	// edit away from never ending.
	for i := 0; i < 4; i++ {
		var buy Want
		found := false
		for _, w := range t.wants(c) {
			if int64(w.Cost) > c.Coins {
				continue
			}
			if open != nil && !open(w.At) {
				continue
			}
			buy, found = w, true
			break
		}
		if !found {
			return bought, off
		}

		// Nothing is destroyed by an upgrade: the piece goes in their pack,
		// onto them, and whatever it displaced comes back out as the
		// employer's. Anything already in their pack stays theirs.
		keep := len(c.Carried)
		c.Carry(buy.Gear)
		if !c.Equip(keep) {
			c.DropCarried(keep)
			return bought, off
		}
		c.Coins -= int64(buy.Cost)
		bought = append(bought, buy.Gear)
		for len(c.Carried) > keep {
			old, ok := c.DropCarried(keep)
			if !ok {
				break
			}
			off = append(off, old)
		}
	}
	return bought, off
}
