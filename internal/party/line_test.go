package party_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/party"
)

// The line follows where the leader has been, not where the leader is. That is
// what makes it bend around a corner instead of cutting it.
func TestFollowersWalkTheLeadersPath(t *testing.T) {
	leader := core.NewWalker(1)
	leader.Place(core.Point{X: 5, Y: 5})
	line := party.Fit(nil, 2, leader.Tile, 1)

	if len(line) != 2 {
		t.Fatalf("the line has %d walkers for 2 companions", len(line))
	}

	// Two steps east, then one north — the corner.
	for _, to := range []core.Point{{X: 6, Y: 5}, {X: 7, Y: 5}, {X: 7, Y: 4}} {
		from := leader.Tile
		leader.Step(to, core.DirBetween(from, to))
		line.Step(from)
	}

	// After three steps the leader is at the corner, and the two behind occupy
	// the two tiles it walked through to get there.
	if got := line[0].Tile; got != (core.Point{X: 7, Y: 5}) {
		t.Errorf("the first companion is at %v, want the tile the leader just left", got)
	}
	if got := line[1].Tile; got != (core.Point{X: 6, Y: 5}) {
		t.Errorf("the second companion is at %v, want one further back down the path", got)
	}
	for i := range line {
		if line[i].Tile == leader.Tile {
			t.Errorf("companion %d is standing inside the leader at %v", i, line[i].Tile)
		}
	}
}

// Dismissing shortens the line rather than leaving a walker behind for a
// companion who is no longer there — that mismatch would index out of range on
// the next draw.
func TestTheLineTracksTheRoster(t *testing.T) {
	at := core.Point{X: 2, Y: 2}
	var line party.Line
	for n := 0; n <= party.MaxSize; n++ {
		line = party.Fit(line, n, at, 7)
		if len(line) != n {
			t.Fatalf("fitting to %d companions produced %d walkers", n, len(line))
		}
	}
	for n := party.MaxSize; n >= 0; n-- {
		line = party.Fit(line, n, at, 7)
		if len(line) != n {
			t.Fatalf("shrinking to %d companions produced %d walkers", n, len(line))
		}
	}
}

// A follower joining mid-run must appear on the tile it is following rather
// than at the origin, or a new hireling walks in from the corner of the map.
func TestNewFollowersStartWhereTheyAreTold(t *testing.T) {
	at := core.Point{X: 40, Y: 17}
	line := party.Fit(nil, 2, at, 7)
	for i := range line {
		if line[i].Tile != at {
			t.Errorf("companion %d formed up at %v, want %v", i, line[i].Tile, at)
		}
		if line[i].Moving() {
			t.Errorf("companion %d formed up mid-step", i)
		}
	}

	// Place moves the whole line, for entering a location or loading a save.
	somewhere := core.Point{X: 3, Y: 9}
	line.Place(somewhere)
	for i := range line {
		if line[i].Tile != somewhere {
			t.Errorf("companion %d is at %v after Place(%v)", i, line[i].Tile, somewhere)
		}
	}
}

// Advance runs the tween; Settle finishes it. A scripted capture depends on the
// second, because a frame grabbed mid-step is a frame that depends on timing.
func TestAdvanceAndSettle(t *testing.T) {
	line := party.Fit(nil, 1, core.Point{}, 4)
	line.Step(core.Point{X: 1})
	if !line[0].Moving() {
		t.Fatal("the follower did not start moving")
	}
	line.Advance()
	if !line[0].Moving() {
		t.Error("one tick of a four-tick step finished it")
	}
	line.Settle()
	if line[0].Moving() {
		t.Error("the follower is still moving after Settle")
	}
}
