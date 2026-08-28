package saga_test

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

func tables(t *testing.T) *gamedata.Tables {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Fatalf("finding repo root: %v", err)
	}
	tb, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	return tb
}

// stubNamer keeps world generation independent of the writing.
type stubNamer struct{}

func (stubNamer) PlaceName(*core.RNG, string) string    { return "Placename" }
func (stubNamer) PlaceTag(*core.RNG, string) string     { return "a place" }
func (stubNamer) PersonName(*core.RNG) string           { return "Somebody" }
func (stubNamer) NPCLine(*core.RNG) string              { return "..." }
func (stubNamer) SignText(*core.RNG) string             { return "..." }
func (stubNamer) RecruitPitch(*core.RNG, string) string { return "..." }
func (stubNamer) Oddity(*core.RNG, string) string       { return "..." }

// TestSpinesPointOutward is the pacing mechanism, and the whole reason a saga
// needs no level gate anywhere in it.
//
// Legs are dealt out in order of distance from where the story starts, so leg
// two is always further than leg one. The difficulty curve is a function of how
// far out a region is, which means the spine paces itself: a player who runs it
// at level three dies in the fourth region, having been given three
// increasingly obvious warnings. A generator that picked places at random would
// need a gate, and a gate is a thing that says no.
func TestSpinesPointOutward(t *testing.T) {
	tb := tables(t)
	var cast int

	for _, seed := range []int64{1, 7, 1994, 20260817} {
		w := world.Generate(seed, stubNamer{})
		g := core.NewRNG(seed)
		for _, sk := range append(tb.Sagas.Spines(), tb.Sagas.Arcs()...) {
			s, ok := saga.Cast(g, &tb.Sagas, w, tb, sk, w.Start, 4, nil)
			if !ok {
				continue
			}
			cast++
			last := -1
			for i, idx := range s.Places {
				d := w.POIs[idx].Pos.Manhattan(w.Start)
				if d <= last {
					t.Errorf("%s on seed %d: leg %d is %d out, leg %d was %d — "+
						"the spine doubles back and the difficulty curve stops pacing it",
						sk.ID, seed, i, d, i-1, last)
				}
				last = d
			}

			// And it has to actually span the continent, which is the property
			// this test was missing when it only checked the order.
			//
			// "Each leg is further than the last" is true of a cluster, and a
			// cluster is what the first staging produced: five legs at 6 to 17
			// tiles, all inside the eighteen-tile radius RegionLevel reads, so
			// the last leg was no rougher than the first. It passed this test
			// every time. cmd/balance found it; this is the assertion that
			// would have.
			span := 0
			for _, p := range w.POIs {
				if d := p.Pos.Manhattan(w.Start); d > span {
					span = d
				}
			}
			if span > 0 && last*10 < span*6 {
				t.Errorf("%s on seed %d ends %d tiles out on a continent that reaches %d — "+
					"the legs are bunched at the near end and distance is not buying difficulty",
					sk.ID, seed, last, span)
			}
		}
	}
	if cast == 0 {
		t.Fatal("four continents and not one saga could be staged on any of them")
	}
}

// TestSagasNameOnlyRealThings is the rule the quest generator and the thread
// caster both follow, restated: a story must never mention a place, a creature
// or an object this continent does not contain.
func TestSagasNameOnlyRealThings(t *testing.T) {
	tb := tables(t)
	seen := map[string]int{}

	for _, seed := range []int64{1, 7, 1994, 20260817} {
		w := world.Generate(seed, stubNamer{})
		g := core.NewRNG(seed)
		for _, sk := range append(tb.Sagas.Spines(), tb.Sagas.Arcs()...) {
			s, ok := saga.Cast(g, &tb.Sagas, w, tb, sk, w.Start, 4, nil)
			if !ok {
				continue
			}
			seen[sk.ID]++

			if len(s.Places) != len(sk.Legs) {
				t.Errorf("%s: %d legs, %d places", sk.ID, len(sk.Legs), len(s.Places))
				continue
			}
			for i, idx := range s.Places {
				if idx < 0 || idx >= len(w.POIs) {
					t.Errorf("%s leg %d points at location %d of %d", sk.ID, i, idx, len(w.POIs))
					continue
				}
				if got := w.POIs[idx].Name; got != s.PlaceNames[i] {
					t.Errorf("%s leg %d calls it %q, location %d is %q",
						sk.ID, i, s.PlaceNames[i], idx, got)
				}
				// A leg that asked for a sort of place has to have got one, or
				// the writing about a farm is attached to a village.
				switch sk.Legs[i].Place {
				case "settlement":
					if !w.POIs[idx].Kind.Settlement() {
						t.Errorf("%s leg %d wanted a settlement and got a %s",
							sk.ID, i, w.POIs[idx].Kind)
					}
				case "delve":
					switch w.POIs[idx].Kind {
					case world.KindDungeon, world.KindCave, world.KindRuin:
					default:
						t.Errorf("%s leg %d wanted somewhere to clear and got a %s",
							sk.ID, i, w.POIs[idx].Kind)
					}
				}
			}
			if s.MonsterID != "" {
				def, ok := tb.ByID[s.MonsterID]
				if !ok {
					t.Errorf("%s names monster %q, which does not exist", sk.ID, s.MonsterID)
				} else if def.Name != s.Roles["{X}"] {
					t.Errorf("%s calls the monster %q, it is called %q",
						sk.ID, s.Roles["{X}"], def.Name)
				}
			}
			if item := s.Roles["{I}"]; item != "" {
				if _, ok := tb.Item(item); !ok {
					t.Errorf("%s wants %q, which is not an item", sk.ID, item)
				}
			}

			// And every line the player can be shown comes out filled. Any
			// brace at all is the failure: casting reads its requirements out
			// of the writing, so a placeholder nothing filled is one the author
			// invented and nothing implements.
			for i := range sk.Legs {
				for _, line := range []string{sk.Legs[i].Text, sk.Legs[i].Note} {
					check(t, sk.ID, s.FillAt(line, i))
				}
			}
			check(t, sk.ID, s.FillAt(sk.Opening, 0))
			for _, e := range sk.Endings {
				check(t, sk.ID, s.FillAt(e.Text, len(sk.Legs)-1))
				check(t, sk.ID, e.Label)
			}
		}
	}

	// Every skeleton in the book should be reachable. One that never casts is
	// writing nobody will read, and the likeliest cause is a requirement no
	// world can satisfy.
	for _, sk := range tb.Sagas.Sagas {
		if seen[sk.ID] == 0 {
			t.Errorf("saga %q was never cast across four continents", sk.ID)
		}
	}
}

func check(t *testing.T, id, line string) {
	t.Helper()
	if i := strings.IndexByte(line, '{'); i >= 0 {
		t.Errorf("%s: an unfilled placeholder survived into %q", id, line[i:])
	}
}

// TestEverySagaHasAWayOut. A player must never be handed a long story whose
// ending they cannot afford, having walked across a continent for it.
func TestEverySagaHasAWayOut(t *testing.T) {
	for _, sk := range tables(t).Sagas.Sagas {
		if len(sk.Legs) < 3 {
			t.Errorf("%q has %d legs; that is an errand with a title", sk.ID, len(sk.Legs))
		}
		if len(sk.Endings) < 2 {
			t.Errorf("%q offers %d ending(s); a story with no choice in it is a cutscene",
				sk.ID, len(sk.Endings))
		}
		free := false
		for _, e := range sk.Endings {
			if e.Costs() == 0 {
				free = true
			}
		}
		if !free {
			t.Errorf("%q: every ending costs money, so a broke player is stuck holding it", sk.ID)
		}
	}
}

// TestNoSagaEndingDominatesAnother. Everything that gives must take: if one
// ending were better than another on every axis, the decision at the end of a
// five-town walk would be a formality with a menu in front of it.
func TestNoSagaEndingDominatesAnother(t *testing.T) {
	for _, sk := range tables(t).Sagas.Sagas {
		for i, a := range sk.Endings {
			for j, b := range sk.Endings {
				if i == j {
					continue
				}
				if dominates(a, b) {
					t.Errorf("%q: %q beats %q on every count, so nobody will pick the other",
						sk.ID, a.Label, b.Label)
				}
			}
		}
	}
}

// dominates reports whether a is at least as good as b everywhere and strictly
// better somewhere. Shame is a cost.
func dominates(a, b saga.Ending) bool {
	axes := [][2]int{
		{int(a.Coins), int(b.Coins)},
		{int(a.XP), int(b.XP)},
		{a.Fame, b.Fame},
		{-a.Shame, -b.Shame},
		{a.Honor, b.Honor},
	}
	better := false
	for _, ax := range axes {
		if ax[0] < ax[1] {
			return false
		}
		if ax[0] > ax[1] {
			better = true
		}
	}
	return better
}

// TestLegsFireInOrderAndOnlyOnce, and only for the thing they are waiting on.
func TestLegsFireInOrderAndOnlyOnce(t *testing.T) {
	b := &saga.Book{Sagas: []saga.Skeleton{{
		ID: "test", Title: "A Test", Opening: "it begins",
		Legs: []saga.Leg{
			{Trigger: saga.Reach, Text: "first", Note: "go to {P}"},
			{Trigger: saga.Hunt, Need: 3, Text: "second", Note: "put down three"},
			{Trigger: saga.Clear, Text: "third", Note: "empty {P}"},
		},
		Endings: []saga.Ending{{Label: "yes"}, {Label: "no", Shame: 1}},
	}}}
	s := &saga.Saga{
		Skeleton: "test", Title: "A Test", State: saga.Open,
		Places: []int{4, 9, 12}, PlaceNames: []string{"A", "B", "C"},
		MonsterID: "wolf",
	}
	l := &saga.Log{}
	l.Add(s)

	// The right trigger at the wrong place does nothing.
	if got := l.Advance(b, saga.Event{Kind: saga.Reach, POI: 9}); len(got) != 0 {
		t.Fatalf("arriving at the second leg's place advanced the first: %d fired", len(got))
	}
	// A trigger a later leg wants does nothing either.
	if got := l.Advance(b, saga.Event{Kind: saga.Hunt, Monster: "wolf", N: 3}); len(got) != 0 {
		t.Fatalf("a hunt fired while the saga was still waiting on a door: %d", len(got))
	}
	if got := l.Advance(b, saga.Event{Kind: saga.Reach, POI: 4}); len(got) != 1 {
		t.Fatalf("arriving at the first leg's place fired %d", len(got))
	}
	if s.At != 1 || s.Place() != 9 {
		t.Fatalf("after leg one the saga is at %d pointing at %d", s.At, s.Place())
	}

	// Counted legs take their events in bulk, and the wrong creature does not
	// count towards them.
	l.Advance(b, saga.Event{Kind: saga.Hunt, Monster: "rat", N: 5})
	if s.Have != 0 {
		t.Errorf("killing the wrong thing counted %d towards the hunt", s.Have)
	}
	l.Advance(b, saga.Event{Kind: saga.Hunt, Monster: "wolf", N: 2})
	if s.Have != 2 || s.At != 1 {
		t.Errorf("two of three left the saga at leg %d with %d", s.At, s.Have)
	}
	if got := l.Advance(b, saga.Event{Kind: saga.Hunt, Monster: "wolf", N: 1}); len(got) != 1 {
		t.Fatalf("the third kill fired %d legs", len(got))
	}

	fired := l.Advance(b, saga.Event{Kind: saga.Clear, POI: 12})
	if len(fired) != 1 || !fired[0].Last {
		t.Fatalf("the last leg fired %d, last=%v", len(fired), len(fired) > 0 && fired[0].Last)
	}
	if s.State != saga.Ready {
		t.Errorf("after the last leg the saga is %q, want ready", s.State)
	}
	// And nothing fires again afterwards.
	if got := l.Advance(b, saga.Event{Kind: saga.Clear, POI: 12}); len(got) != 0 {
		t.Errorf("a finished saga fired %d more legs", len(got))
	}
}

// TestASagaNeedsEnoughContinent. Cast reporting false is a supported answer —
// the same one the thread caster gives when there is no ruin to end at — and it
// has to be false rather than a saga with a leg pointing nowhere.
func TestASagaNeedsEnoughContinent(t *testing.T) {
	tb := tables(t)
	spines := tb.Sagas.Spines()
	if len(spines) == 0 {
		t.Skip("no spines authored")
	}
	empty := &world.Map{Seed: 1}
	if s, ok := saga.Cast(core.NewRNG(1), &tb.Sagas, empty, tb, spines[0],
		core.Point{X: 0, Y: 0}, 1, nil); ok {
		t.Errorf("a continent with no locations on it staged %q anyway, at %v", s.Title, s.Places)
	}
}

// TestEverySagaCanActuallyBeFinished plays each one through on real
// continents, firing exactly the events the game fires and nothing else.
//
// This is the test the whole feature rests on. A spine is five places and
// possibly several hours; a leg that can never come due does not announce
// itself, it just quietly becomes the reason a player's journal has a dead
// entry at the top of it forever. Everything else here checks that a saga is
// staged correctly — this checks that it ends.
func TestEverySagaCanActuallyBeFinished(t *testing.T) {
	tb := tables(t)

	for _, seed := range []int64{1, 7, 1994, 20260817} {
		w := world.Generate(seed, stubNamer{})
		g := core.NewRNG(seed)

		for _, sk := range tb.Sagas.Sagas {
			s, ok := saga.Cast(g, &tb.Sagas, w, tb, &sk, w.Start, 4, nil)
			if !ok {
				continue
			}
			l := &saga.Log{}
			l.Add(s)

			for step := 0; s.State == saga.Open; step++ {
				if step > len(sk.Legs)*2 {
					t.Fatalf("%s on seed %d: stuck on leg %d (%s) after %d events",
						sk.ID, seed, s.At, sk.Legs[s.At].Trigger, step)
				}
				leg := sk.Legs[s.At]
				// Exactly what the game sends, from the same three places:
				// walking through a door, emptying a location, killing a thing.
				switch leg.Trigger {
				case saga.Reach:
					l.Advance(&tb.Sagas, saga.Event{Kind: saga.Reach, POI: s.Place()})
				case saga.Clear:
					l.Advance(&tb.Sagas, saga.Event{Kind: saga.Clear, POI: s.Place()})
				case saga.Hunt:
					l.Advance(&tb.Sagas, saga.Event{
						Kind: saga.Hunt, Monster: s.MonsterID, N: 1,
					})
				default:
					t.Fatalf("%s leg %d waits on %q, which nothing in the game sends",
						sk.ID, s.At, leg.Trigger)
				}
			}

			if s.State != saga.Ready {
				t.Errorf("%s on seed %d finished as %q, want ready", sk.ID, seed, s.State)
			}
			if got := s.Options(&tb.Sagas); len(got) < 2 {
				t.Errorf("%s on seed %d offers %d endings at the end of it", sk.ID, seed, len(got))
			}
		}
	}
}

// TestAClearLegNeedsSomewhereToClear. A leg that waits on a location being
// emptied has to point at a location that *can* be emptied — a settlement never
// fires Clear, so a spine staged with one would stop dead partway through and
// look exactly like a bug in the event wiring.
func TestAClearLegNeedsSomewhereToClear(t *testing.T) {
	for _, sk := range tables(t).Sagas.Sagas {
		for i, l := range sk.Legs {
			if l.Trigger == saga.Clear && l.Place != "delve" {
				t.Errorf("%q leg %d waits to be cleared and asks for place %q; "+
					"only a delve is ever cleared", sk.ID, i, l.Place)
			}
		}
	}
}
