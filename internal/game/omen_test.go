package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The three marks have to be three marks.
//
// They are told apart by silhouette first and colour second, which is the right
// way round: colour on grass at dusk in the rain is not something to hang a
// decision on. So the shapes must differ from each other — and from the
// attention star, which is the fourth thing floating over somebody's head in
// this game and the one a player already knows.
func TestTheOmenMarksAreTellableApart(t *testing.T) {
	flat := func(rows []string) string {
		out := ""
		for _, r := range rows {
			out += r
		}
		return out
	}
	seen := map[string]string{flat(starGlyph): "the attention star"}
	for _, o := range []world.Omen{world.OmenHostile, world.OmenBoon, world.OmenMystery} {
		rows, col, ok := omenMark(o, 0)
		if !ok {
			t.Errorf("%s has no mark", o)
			continue
		}
		if len(rows) != 7 {
			t.Errorf("%s is %d rows; the marks are hand-set at seven", o, len(rows))
		}
		for _, r := range rows {
			if len(r) != 7 {
				t.Errorf("%s has a row %d wide; the marks are seven across", o, len(r))
			}
		}
		if col == nil {
			t.Errorf("%s has no colour", o)
		}
		if was, dup := seen[flat(rows)]; dup {
			t.Errorf("%s draws the same shape as %s", o, was)
		}
		seen[flat(rows)] = string(o)
	}
	// And nothing at all over something that is not an encounter.
	if _, _, ok := omenMark(world.Omen(""), 0); ok {
		t.Error("something with no omen still draws a mark")
	}
}

// A mark means the same thing wherever it is standing.
//
// The overworld rolls its omens in the game layer and interiors roll theirs in
// world, because an interior is generated from its location's seed and a foe
// whose omen was redrawn on every visit would be a marker that changed colour
// between two walks through the same room. Two callers, and the one thing they
// must agree about is how often each mark comes up — a green ring that is
// common in a field and rare underground is two markers wearing one shape.
func TestBothRollsUseOneTable(t *testing.T) {
	const n = 20000
	count := func(draw func(*core.RNG) world.Omen) map[world.Omen]int {
		got := map[world.Omen]int{}
		g := core.NewRNG(4)
		for i := 0; i < n; i++ {
			got[draw(g)]++
		}
		return got
	}
	over := count(rollOmen)
	// world's own roll, reached through the only door it has: generating an
	// interior would be indirect, so this asserts the shares instead.
	for _, o := range []world.Omen{world.OmenBoon, world.OmenMystery} {
		want := world.BoonShare
		if o == world.OmenMystery {
			want = world.MysteryShare
		}
		got := float64(over[o]) / n
		if got < want-0.02 || got > want+0.02 {
			t.Errorf("%s came up %.1f%% of the time, the table says %.1f%%",
				o, got*100, want*100)
		}
	}
	if over[world.OmenHostile] == 0 {
		t.Error("nothing hostile ever came up")
	}
}

// A mystery has to be worth declining.
//
// A coin flip that pays out half the time is a marker you always walk to, which
// makes it the same as no marker at all — the whole reason the third state
// exists is that a world where every mark is honest is a world where the good
// ones are simply collected. It has to be against the player.
func TestAMysteryIsAGamble(t *testing.T) {
	g := core.NewRNG(11)
	const n = 20000
	boons := 0
	for i := 0; i < n; i++ {
		switch resolveMystery(g) {
		case world.OmenBoon:
			boons++
		case world.OmenMystery:
			t.Fatal("a mystery resolved to another mystery, which never ends")
		}
	}
	if share := float64(boons) / n; share >= 0.5 {
		t.Errorf("a mystery pays out %.0f%% of the time, so there is no reason not to take one", share*100)
	}
}

// Ambushes underground come at a bounded distance apart.
//
// The old roll was six per cent a step after a grace of six, which is
// memoryless — and measured over a hundred and eighty thousand steps of walking
// real dungeons it averaged a fight every 21.3 steps while *fifteen per cent of
// the gaps were eight steps or fewer* and the longest was 132. A playthrough
// reported it as "every 5-8 steps, with the occasional long stretch", which is
// not a rate complaint. The mean was fine. The shape was wrong at both ends,
// and a mean is exactly the statistic that cannot see that.
//
// So the test is on the bounds rather than the average: nothing may arrive
// sooner than the floor and nothing may take longer than the ceiling.
func TestAmbushesKeepTheirDistance(t *testing.T) {
	g := &Game{RNG: core.NewRNG(9)}
	gaps := []int{}
	since := 0
	for step := 0; step < 200000; step++ {
		since++
		if g.ambushDue() {
			gaps = append(gaps, since)
			since = 0
		}
	}
	if len(gaps) < 1000 {
		t.Fatalf("only %d ambushes in 200k steps; that is not a sample", len(gaps))
	}
	// The first gap includes the lazy draw of the very first budget, so it can
	// legitimately run one over. Every other one is the budget itself.
	for i, v := range gaps[1:] {
		if v < ambushFloor {
			t.Fatalf("gap %d was %d steps, inside the floor of %d", i, v, ambushFloor)
		}
		if v > ambushCeil {
			t.Fatalf("gap %d was %d steps, past the ceiling of %d", i, v, ambushCeil)
		}
	}
	sum := 0
	for _, v := range gaps {
		sum += v
	}
	// And it must actually be further apart than the roll it replaced, which
	// averaged 21.3 steps and was the thing being complained about.
	if mean := float64(sum) / float64(len(gaps)); mean < 25 {
		t.Errorf("ambushes average %.1f steps apart; the roll this replaced managed 21.3", mean)
	}
}

// A save written before the budget existed carries a zero, and a zero must mean
// "draw one" rather than "now".
//
// This format is seed plus deltas, so a field added today is absent in every
// file written before today and unmarshals to nothing. A zero read as "the
// budget has run out" would ambush the player on their first step back into
// every dungeon in every old save.
func TestAnOldSaveIsNotAmbushedOnTheFirstStep(t *testing.T) {
	g := &Game{RNG: core.NewRNG(3)}
	if g.ambushDue() {
		t.Error("a save with no budget was ambushed on its first step underground")
	}
	if g.nextAmbush < ambushFloor {
		t.Errorf("the drawn budget is %d, inside the floor of %d", g.nextAmbush, ambushFloor)
	}
}
