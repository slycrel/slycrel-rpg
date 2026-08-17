package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// TestPenanceIsNotANoOp is the whole of why Confess takes Renown rather than
// Fame, and it is here because the wrong version passes every other test in
// this package.
//
// Read weighs Shame against Fame. An implementation that lifted both together
// would leave the character standing in exactly the corner they started in — a
// sacrament that changes two numbers and nothing else. Taking Renown instead
// means the deeds survive and the face stops being known, which is a thing the
// player can actually feel.
func TestPenanceIsNotANoOp(t *testing.T) {
	c := &model.Character{Fame: 2, Renown: 7, Shame: 5, Faith: 3}
	if got := rules.Read(c); got != rules.Notorious {
		t.Fatalf("setup: wanted a notorious character, got %v", got)
	}
	if n := rules.Confess(c); n != 3 {
		t.Fatalf("confession lifted %d, want 3", n)
	}
	if got := rules.Read(c); got == rules.Notorious {
		t.Errorf("confessing left the character notorious (fame %d, renown %d, shame %d): "+
			"penance that does not move the standing is a button that does nothing",
			c.Fame, c.Renown, c.Shame)
	}
	if c.Fame != 2 {
		t.Errorf("the deeds were taken too (fame %d, want 2): penance sells anonymity, not the past", c.Fame)
	}
}

// TestPenanceCostsTheThingItIsWorth. Everything that gives must take, so a
// confession has to be paid for in the currency of being known.
//
// The guarantee is one for one — a point of renown per point of shame — and
// not that the standing always changes band. Read is a step function, so a
// character far above the threshold can absorb a small scandal quietly, which
// is the correct outcome and not a hole: what they are spending is the margin
// they earned. The band claim is made below, from the edge of it, where the
// margin has run out.
func TestPenanceCostsTheThingItIsWorth(t *testing.T) {
	c := &model.Character{Fame: 9, Renown: 8, Shame: 2, Faith: 4}
	rules.Confess(c)
	if c.Renown != 6 || c.Faith != 2 {
		t.Errorf("lifting 2 shame left renown %d and faith %d, want 6 and 2: "+
			"penance is paid one for one in both", c.Renown, c.Faith)
	}
}

// TestPenanceCanCostTheStanding, from the edge of the band where it bites.
func TestPenanceCanCostTheStanding(t *testing.T) {
	c := &model.Character{Fame: 9, Renown: 6, Shame: 2, Faith: 4}
	if got := rules.Read(c); got != rules.Celebrated {
		t.Fatalf("setup: wanted a celebrated character, got %v", got)
	}
	rules.Confess(c)
	if got := rules.Read(c); got == rules.Celebrated {
		t.Error("a character with no margin left scrubbed their shame and stayed celebrated; " +
			"the way out of notoriety has to close the door to celebrity behind it")
	}
}

// TestPenanceUndoesBeingCarriedHome. Not a coincidence worth leaving
// undocumented: a rescue costs a point of shame and a point of renown — being
// carried through the gate is public — and a confession lifts exactly one of
// each. The shrine is the thing that unhappens the walk of shame, and it is
// the reason both numbers move together rather than shame moving alone.
func TestPenanceUndoesBeingCarriedHome(t *testing.T) {
	// One point of faith banked and nothing outstanding, so the confession has
	// exactly the one rescue to undo and cannot reach past it.
	before := model.Character{Fame: 4, Renown: 3, Shame: 0, Faith: 1}

	carried := before
	carried.Shame++ // what rescueToTown does, in the order it does it
	carried.Renown++

	if n := rules.Confess(&carried); n != 1 {
		t.Fatalf("confession lifted %d, want 1", n)
	}
	if carried.Shame != before.Shame || carried.Renown != before.Renown || carried.Fame != before.Fame {
		t.Errorf("after a rescue and a confession: fame %d, renown %d, shame %d; "+
			"want the %d, %d, %d it started with",
			carried.Fame, carried.Renown, carried.Shame,
			before.Fame, before.Renown, before.Shame)
	}
}

// TestConfessNeverRunsAnythingNegative. Reachable from a menu drawn a frame
// ago, so it clamps rather than trusting what the caller believed.
func TestConfessNeverRunsAnythingNegative(t *testing.T) {
	for _, c := range []*model.Character{
		{},                              // nothing at all
		{Faith: 9, Shame: 1, Renown: 0}, // more faith than there is anything to spend it on
		{Faith: 9, Shame: 9, Renown: 1}, // more shame than renown to pay with
		{Faith: 0, Shame: 4, Renown: 4}, // the plate is empty
		{Faith: 2, Shame: 2, Fame: 1},   // and no renown at all
	} {
		before := *c
		rules.Confess(c)
		if c.Faith < 0 || c.Shame < 0 || c.Renown < 0 {
			t.Errorf("confessing %+v left faith %d, shame %d, renown %d",
				before, c.Faith, c.Shame, c.Renown)
		}
	}
}

// TestPenanceNeverLaundersAWholeRun. Altars are one-shot but scattered, and a
// player holding a lot of faith should still have to walk to more than one of
// them to scrub a long run's worth of shame.
func TestPenanceNeverLaundersAWholeRun(t *testing.T) {
	if n := rules.Penance(40, 40); n > 3 {
		t.Errorf("one altar lifted %d points of shame; that is a whole run at one stop", n)
	}
	if n := rules.Penance(1, 40); n != 1 {
		t.Errorf("a point of faith lifted %d points of shame, want 1", n)
	}
	if n := rules.Penance(40, 0); n != 0 {
		t.Errorf("confessing with nothing to confess lifted %d", n)
	}
}

// TestHonorMovesTheCutAndStaysInBand. Honour is a lever on a percentage of
// every haul for the rest of the run, so what matters is that it moves the
// number at all and that it can never move it somewhere absurd.
func TestHonorMovesTheCutAndStaysInBand(t *testing.T) {
	const rolled = 13 // the middle of Recruit's 8-18 band

	if rules.AskingCut(rolled, 8) >= rolled {
		t.Error("an honourable employer is not asked for less")
	}
	if rules.AskingCut(rolled, -8) <= rolled {
		t.Error("an employer who leaves people mid-story is not asked for more")
	}
	if got := rules.AskingCut(rolled, 0); got != rolled {
		t.Errorf("a character with no history of either paid %d, want the rolled %d", got, rolled)
	}

	// The band holds at every honour anybody could reach, in both directions
	// and from either end of the roll.
	for honor := -60; honor <= 60; honor++ {
		for _, r := range []int{8, 13, 18} {
			got := rules.AskingCut(r, honor)
			if got < 3 || got > 30 {
				t.Fatalf("honour %d turned a rolled %d%% cut into %d%%", honor, r, got)
			}
		}
	}
}
