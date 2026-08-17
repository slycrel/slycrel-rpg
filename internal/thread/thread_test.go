package thread_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// A stub namer, deliberately: these assert structural properties of a cast
// thread, not the identity of the continent it was cast on. Anything comparing
// a save against its world needs the real writer instead.
type stubNamer struct{}

func (stubNamer) PlaceName(*core.RNG, string) string    { return "Placename" }
func (stubNamer) PlaceTag(*core.RNG, string) string     { return "tag" }
func (stubNamer) PersonName(*core.RNG) string           { return "Person" }
func (stubNamer) NPCLine(*core.RNG) string              { return "line" }
func (stubNamer) SignText(*core.RNG) string             { return "sign" }
func (stubNamer) RecruitPitch(*core.RNG, string) string { return "pitch" }

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

func hireling(name string, blood model.MonsterKind, level int) *model.Character {
	return &model.Character{Name: name, Level: level, Ally: true, Blood: blood, Cut: 12}
}

// TestCastThreadsNameOnlyRealThings is the property the whole package exists
// for, and it is the quest generator's rule restated: a backstory must never
// mention a place, a creature or an object this continent does not contain.
func TestCastThreadsNameOnlyRealThings(t *testing.T) {
	tb := tables(t)
	seen := map[string]int{}

	for _, seed := range []int64{1, 7, 1994, 20260816} {
		w := world.Generate(seed, stubNamer{})
		g := core.NewRNG(seed)

		// Both entry points, because they draw from different halves of the
		// book: Cast skips the residents' stories and CastResident is the only
		// thing that reaches them. Checking one would leave the other's writing
		// entirely unexercised, which is exactly the state four new skeletons
		// arrived in.
		var cast []*thread.Thread
		owners := map[*thread.Thread]string{}
		for _, l := range append([]model.Lineage{{}}, model.Lineages...) {
			for try := 0; try < 8; try++ {
				c := hireling("Bosk", l.Kind, 4)
				if th, ok := thread.Cast(g, &tb.Threads, w, tb, c, w.Start, nil); ok {
					cast = append(cast, th)
					owners[th] = c.Name
				}
			}
		}
		for home := range w.POIs {
			for try := 0; try < 4; try++ {
				const who = "Marta"
				if th, ok := thread.CastResident(g, &tb.Threads, w, tb, who, home, 4, w.Start, nil); ok {
					cast = append(cast, th)
					owners[th] = who
					if th.HomePOI != home {
						t.Errorf("%s: cast for location %d, lives at %d", th.Skeleton, home, th.HomePOI)
					}
					// The meeting is the first beat and it fires on casting, so
					// a resident always walks out of Cast already holding
					// something to say. Without it the opening conversation is
					// a journal note from somebody who has not said hello.
					if th.Owed == "" {
						t.Errorf("%s: a freshly cast resident has nothing to open with", th.Skeleton)
					}
				}
			}
		}

		for _, th := range cast {
			seen[th.Skeleton]++

			if th.Owner != owners[th] {
				t.Errorf("%s: thread belongs to %q, was cast for %q", th.Skeleton, th.Owner, owners[th])
			}
			if th.State != thread.Open {
				t.Errorf("%s: a freshly cast thread is already %q", th.Skeleton, th.State)
			}
			if th.PlacePOI >= 0 {
				if th.PlacePOI >= len(w.POIs) {
					t.Errorf("%s: points at location %d of %d", th.Skeleton, th.PlacePOI, len(w.POIs))
					continue
				}
				if got := w.POIs[th.PlacePOI].Name; got != th.Roles["{P}"] {
					t.Errorf("%s: names the place %q, location %d is %q",
						th.Skeleton, th.Roles["{P}"], th.PlacePOI, got)
				}
			}
			if th.MonsterID != "" {
				def, ok := tb.ByID[th.MonsterID]
				if !ok {
					t.Errorf("%s: names monster %q, which does not exist", th.Skeleton, th.MonsterID)
				} else if def.Name != th.Roles["{X}"] {
					t.Errorf("%s: calls the monster %q, it is called %q",
						th.Skeleton, th.Roles["{X}"], def.Name)
				}
			}
			if item := th.Roles["{I}"]; item != "" {
				if _, ok := tb.Item(item); !ok {
					t.Errorf("%s: wants %q, which is not an item", th.Skeleton, item)
				}
			}

			// And every line the player can be shown must come out filled.
			// Any brace at all is the failure: casting reads its
			// requirements out of the writing, so a placeholder nothing
			// filled is one the author invented and nothing implements,
			// and it reaches the player as literal braces mid-sentence.
			for _, line := range allText(&tb.Threads, th) {
				if i := strings.IndexByte(line, '{'); i >= 0 {
					t.Errorf("%s: an unfilled placeholder survived into %q", th.Skeleton, line[i:])
				}
			}
		}
	}

	// Every skeleton in the book should be reachable. One that never casts is
	// writing nobody will ever read, and the likeliest cause is a requirement
	// no world can satisfy.
	for _, s := range tb.Threads.Threads {
		if seen[s.ID] == 0 {
			t.Errorf("skeleton %q was never cast across four continents", s.ID)
		}
	}
}

// allText is every line a cast thread can put in front of the player.
func allText(b *thread.Book, t *thread.Thread) []string {
	out := []string{t.Fill(t.Title)}
	s, ok := b.Get(t.Skeleton)
	if !ok {
		return out
	}
	for i := range s.Beats {
		t.At = i
		out = append(out, t.Fill(s.Beats[i].Text), t.Note(b))
	}
	t.At = 0
	for _, e := range t.Options(b) {
		out = append(out, e.Label, e.Text)
	}
	return out
}

// A companion of three is three different stories. Casting the same skeleton
// twice would give a company two people with the same past, which is a worse
// failure than a companion with no past at all.
func TestCastDoesNotRepeatAStoryTheCompanyIsAlreadyTelling(t *testing.T) {
	tb := tables(t)
	w := world.Generate(1994, stubNamer{})
	g := core.NewRNG(1994)

	var taken []string
	for i := 0; i < 3; i++ {
		c := hireling(fmt.Sprintf("Hireling %d", i), "", 5)
		th, ok := thread.Cast(g, &tb.Threads, w, tb, c, w.Start, taken)
		if !ok {
			break
		}
		for _, id := range taken {
			if id == th.Skeleton {
				t.Fatalf("cast %q again with it already in use", id)
			}
		}
		taken = append(taken, th.Skeleton)
	}
	if len(taken) < 2 {
		t.Fatalf("only %d thread(s) cast; the test cannot see the collision it is for", len(taken))
	}
}

// A hireling with a lineage gets the story written for that lineage. It is the
// reason the ancestry is on the sheet at all: "there is an arrangement" is a
// promise, and casting a part-demon in the generic debt story breaks it.
func TestALineageGetsItsOwnStoryWhileThereIsOneLeft(t *testing.T) {
	tb := tables(t)
	w := world.Generate(1994, stubNamer{})

	for _, l := range model.Lineages {
		want := ""
		for _, s := range tb.Threads.Threads {
			if s.Blood == string(l.Kind) {
				want = s.ID
			}
		}
		if want == "" {
			t.Errorf("nothing is written for a %s", l.Tag)
			continue
		}
		g := core.NewRNG(int64(len(l.Kind)))
		c := hireling("Bosk", l.Kind, 6)
		th, ok := thread.Cast(g, &tb.Threads, w, tb, c, w.Start, nil)
		if !ok {
			t.Errorf("%s: nothing could be cast at all", l.Tag)
			continue
		}
		if th.Skeleton != want {
			t.Errorf("%s was cast in %q, not their own %q", l.Tag, th.Skeleton, want)
		}

		// With their own story already running, they fall through to the
		// general pool rather than coming away with nothing.
		th2, ok := thread.Cast(g, &tb.Threads, w, tb, c, w.Start, []string{want})
		if !ok {
			t.Errorf("%s: a second one got no story once %q was taken", l.Tag, want)
			continue
		}
		if th2.Skeleton == want {
			t.Errorf("%s: %q was cast twice despite being listed as taken", l.Tag, want)
		}
	}
}

// TestEveryThreadCanBeFinishedByABrokePlayer. Endings may cost money, and a
// player who has just paid a rescue fee may have none. Without a free way out,
// a companion could stand there asking a question that cannot be answered for
// the rest of the run.
func TestEveryThreadCanBeFinishedByABrokePlayer(t *testing.T) {
	for _, s := range tables(t).Threads.Threads {
		free := false
		for _, e := range s.Endings {
			if e.Costs() == 0 {
				free = true
			}
		}
		if !free {
			t.Errorf("every ending of %q costs money", s.ID)
		}
	}
}

// A generated name will not take an article or a plural. "Owl That Knows" is a
// creature and an item is "Suspicious Pollen", so "a {X}" becomes "a Owl That
// Knows" and "{X}s" becomes something nobody wrote. Both got into the shipped
// writing on the first pass and both were caught by reading the filled text
// rather than the templates, which is the argument for this test existing.
func TestWritingNeverPutsAnArticleOrAPluralOnAGeneratedName(t *testing.T) {
	bad := []string{"a {X}", "an {X}", "a {I}", "an {I}", "a {P}", "an {P}",
		"{X}s", "{I}s", "{P}s", "{N}s"}
	for _, s := range tables(t).Threads.Threads {
		for _, line := range append(beatText(s), endingText(s)...) {
			for _, b := range bad {
				if strings.Contains(line, b) {
					t.Errorf("%q: %q will not read as English once it is cast: %q", s.ID, b, line)
				}
			}
		}
	}
}

func beatText(s thread.Skeleton) []string {
	out := []string{s.Title}
	for _, b := range s.Beats {
		out = append(out, b.Text, b.Note)
	}
	return out
}

func endingText(s thread.Skeleton) []string {
	var out []string
	for _, e := range s.Endings {
		out = append(out, e.Label, e.Text)
	}
	return out
}

// TestNoEndingDominatesAnother. Everything that gives must take: if one ending
// were better than another on every axis, the choice at the end of a thread
// would be a formality with a menu in front of it.
func TestNoEndingDominatesAnother(t *testing.T) {
	for _, s := range tables(t).Threads.Threads {
		if len(s.Endings) < 2 {
			t.Errorf("%q offers %d ending(s); a thread with no choice in it is a cutscene",
				s.ID, len(s.Endings))
			continue
		}
		for i, a := range s.Endings {
			for j, b := range s.Endings {
				if i == j {
					continue
				}
				if dominates(a, b) {
					t.Errorf("%q: %q beats %q on every count, so nobody will ever pick the other",
						s.ID, a.Label, b.Label)
				}
			}
		}
	}
}

// TestHonorIsNotJustTheMoneyAxisRenamed.
//
// Honour is worth having as a separate number only if it sometimes disagrees
// with the coins. If every ending that paid nothing were honourable and every
// ending that paid were not, the sheet would carry a fourth number that says
// exactly what the third one already said, and the choice at the end of a
// thread would be no wider than it was before.
//
// So: at least one thread has to put honour on both of its endings, and at
// least one has to put it on neither. Those are the two shapes that prove the
// axis is authored per thread rather than derived from the payout — a thread
// where standing by somebody is not the question, and a thread where both ways
// of standing by them are.
func TestHonorIsNotJustTheMoneyAxisRenamed(t *testing.T) {
	var bothHonourable, neitherHonourable int
	for _, s := range tables(t).Threads.Threads {
		with := 0
		for _, e := range s.Endings {
			if e.Honor > 0 {
				with++
			}
		}
		switch {
		case with == len(s.Endings):
			bothHonourable++
		case with == 0:
			neitherHonourable++
		}
	}
	if bothHonourable == 0 {
		t.Error("no thread has honour on every ending: without one, honour is " +
			"only ever the name of the option that pays nothing")
	}
	// Not an error on its own — it is possible to author a good table without
	// this shape — but worth saying out loud, because losing it is how the axis
	// quietly collapses back into the payout.
	if neitherHonourable == 0 {
		t.Log("every thread turns on honour; nothing is left that is purely a question of money")
	}
}

// TestNoThreadForcesADishonourableEnding. Everything that gives must take, but
// a player must never be handed a story whose every exit costs them something
// they were trying to keep.
func TestNoThreadForcesADishonourableEnding(t *testing.T) {
	for _, s := range tables(t).Threads.Threads {
		clean := false
		for _, e := range s.Endings {
			if e.Honor >= 0 {
				clean = true
				break
			}
		}
		if !clean {
			t.Errorf("%q: every ending costs honour, so taking the thread was the mistake", s.ID)
		}
	}
}

// dominates reports whether a is at least as good as b everywhere and strictly
// better somewhere. A lower cut is better; shame is a cost.
func dominates(a, b thread.Ending) bool {
	axes := [][2]int{
		{int(a.Coins), int(b.Coins)},
		{int(a.XP), int(b.XP)},
		{-a.Cut, -b.Cut},
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

// --- advancing ------------------------------------------------------------

// book is a two-beat thread with a counted first beat, small enough to reason
// about and independent of whatever the shipped writing happens to say.
func book() *thread.Book {
	return &thread.Book{Threads: []thread.Skeleton{{
		ID: "test", Title: "A Test", Place: "delve",
		Beats: []thread.Beat{
			{Trigger: thread.Travel, Need: 3, Text: "one", Note: "walk"},
			{Trigger: thread.Reach, Text: "two", Note: "arrive"},
		},
		Endings: []thread.Ending{{Label: "yes"}, {Label: "no", Coins: 10, Shame: 1}},
	}}}
}

func running(owner string) *thread.Thread {
	return &thread.Thread{
		Skeleton: "test", Owner: owner, Title: "A Test", State: thread.Open,
		PlacePOI: 5, Roles: map[string]string{"{N}": owner},
	}
}

func TestBeatsFireInOrderAndOnlyOnce(t *testing.T) {
	b, l := book(), &thread.Log{}
	th := running("Bosk")
	l.Add(th)

	// A trigger the current beat is not waiting for does nothing, even when it
	// is one a later beat wants.
	if got := l.Advance(b, thread.Event{Kind: thread.Reach, POI: 5}); len(got) != 0 {
		t.Fatalf("arriving fired %d beat(s) while the thread was still counting steps", len(got))
	}

	if got := th.Awaiting(b); got != thread.Travel {
		t.Errorf("a thread on its first beat is waiting on %q", got)
	}

	for i := 0; i < 2; i++ {
		if got := l.Advance(b, thread.Event{Kind: thread.Travel, N: 1}); len(got) != 0 {
			t.Fatalf("step %d of 3 fired the beat early", i+1)
		}
	}
	fired := l.Advance(b, thread.Event{Kind: thread.Travel, N: 1})
	if len(fired) != 1 || fired[0].Text != "one" {
		t.Fatalf("the third step produced %+v", fired)
	}
	if fired[0].Last {
		t.Error("the first of two beats reported itself as the last")
	}

	// The game reveals a thread's destination the moment it starts waiting to
	// be taken there, so this has to flip on the beat before the arrival and
	// not on the arrival itself.
	if got := th.Awaiting(b); got != thread.Reach {
		t.Errorf("a thread one beat from its destination is waiting on %q", got)
	}

	// Walking further must not re-fire a beat that is behind you.
	for i := 0; i < 5; i++ {
		if got := l.Advance(b, thread.Event{Kind: thread.Travel, N: 1}); len(got) != 0 {
			t.Fatal("a beat fired twice")
		}
	}

	if got := l.Advance(b, thread.Event{Kind: thread.Reach, POI: 4}); len(got) != 0 {
		t.Fatal("arriving somewhere else fired the arrival beat")
	}
	fired = l.Advance(b, thread.Event{Kind: thread.Reach, POI: 5})
	if len(fired) != 1 || !fired[0].Last {
		t.Fatalf("arriving at the cast place produced %+v", fired)
	}
	if th.State != thread.Ready {
		t.Errorf("a thread out of beats is %q, expected %q", th.State, thread.Ready)
	}
	if got := l.Advance(b, thread.Event{Kind: thread.Reach, POI: 5}); len(got) != 0 {
		t.Error("a thread waiting on its ending is still advancing")
	}
	if got := th.Awaiting(b); got != "" {
		t.Errorf("a thread out of beats is still waiting on %q", got)
	}
}

// A counted beat has to accept a single event worth several, or a caller that
// batches becomes a caller that silently loses progress.
// The journal shows a counter for what the player is deliberately doing and
// keeps quiet about steps. "0 / 70" beside a story makes the beat that exists
// only to let some road go by look like the demanding one.
func TestTheJournalCountsFightsAndKillsButNotSteps(t *testing.T) {
	b := &thread.Book{Threads: []thread.Skeleton{{
		ID: "test", Title: "A Test",
		Beats: []thread.Beat{
			{Trigger: thread.Travel, Need: 70, Note: "walk with them"},
			{Trigger: thread.Kills, Need: 3, Note: "put three down"},
			{Trigger: thread.Reach, Note: "arrive"},
		},
		Endings: []thread.Ending{{Label: "yes"}, {Label: "no", Shame: 1}},
	}}}
	th := running("Bosk")
	th.Have = 2

	for _, c := range []struct {
		at   int
		want string
	}{
		{0, ""},      // travel: the note carries it
		{1, "2 / 3"}, // kills: the player is doing this on purpose
		{2, ""},      // reach: no halfway
	} {
		th.At = c.at
		if got := th.Progress(b); got != c.want {
			t.Errorf("beat %d shows progress %q, want %q", c.at, got, c.want)
		}
		// Whatever the counter does, the note always says what to do next.
		if th.Note(b) == "" {
			t.Errorf("beat %d has no journal note", c.at)
		}
	}
}

func TestCountedBeatsTakeTheirEventsInBulk(t *testing.T) {
	b, l := book(), &thread.Log{}
	l.Add(running("Bosk"))
	if got := l.Advance(b, thread.Event{Kind: thread.Travel, N: 3}); len(got) != 1 {
		t.Fatalf("three steps at once fired %d beats", len(got))
	}
}

// A departing companion takes their story with them. Left behind, it would sit
// in the journal waiting on a beat that nobody can trigger any more.
func TestDismissingSomebodyTakesTheirThread(t *testing.T) {
	l := &thread.Log{}
	l.Add(running("Bosk"))
	l.Add(running("Ilsabet"))

	l.Drop(book(), "Bosk")
	if l.For(book(), "Bosk") != nil {
		t.Error("a dismissed companion's thread is still in the log")
	}
	if l.For(book(), "Ilsabet") == nil {
		t.Error("dismissing one companion dropped another's thread")
	}
	if got := len(l.Running()); got != 1 {
		t.Errorf("%d threads still running, expected 1", got)
	}
}

func TestResolvingClosesAThreadAndRecordsWhichWay(t *testing.T) {
	b := book()
	th := running("Bosk")
	opts := th.Options(b)
	if len(opts) != 2 {
		t.Fatalf("the test book offers %d endings", len(opts))
	}
	th.Resolve(opts[1])
	if th.State != thread.Closed {
		t.Errorf("a resolved thread is %q", th.State)
	}
	if th.Ended != "no" {
		t.Errorf("a resolved thread recorded %q as the choice taken", th.Ended)
	}
	l := &thread.Log{}
	l.Add(th)
	if got := l.Running(); len(got) != 0 {
		t.Error("a closed thread is still listed as running")
	}
	if got := l.Advance(b, thread.Event{Kind: thread.Travel, N: 99}); len(got) != 0 {
		t.Error("a closed thread advanced")
	}
}

// A thread whose skeleton has been deleted from the data must go quiet rather
// than panic. Saves outlive the writing they were cast from.
func TestAThreadWithNoSkeletonLeftIsHarmless(t *testing.T) {
	b, l := book(), &thread.Log{}
	th := running("Bosk")
	th.Skeleton = "deleted-in-a-later-build"
	l.Add(th)

	if got := l.Advance(b, thread.Event{Kind: thread.Travel, N: 10}); len(got) != 0 {
		t.Error("an orphaned thread fired a beat")
	}
	if th.Note(b) != "" || th.Progress(b) != "" || th.Options(b) != nil {
		t.Error("an orphaned thread still has something to say for itself")
	}
}

// --- residents ------------------------------------------------------------

// TestResidentEndingsNeverAdjustACut. Cut is a companion's standing claim on
// the purse, and a resident does not have one — resolveThread is handed a nil
// owner for theirs and has nothing to apply it to. An authored cut on one of
// these would be a trade-off the player is promised and never paid.
func TestResidentEndingsNeverAdjustACut(t *testing.T) {
	for _, s := range tables(t).Threads.Threads {
		if !s.Resident {
			continue
		}
		for _, e := range s.Endings {
			if e.Cut != 0 {
				t.Errorf("%q: ending %q moves a cut by %d, and nobody is taking one",
					s.ID, e.Label, e.Cut)
			}
		}
	}
}

// TestResidentsWaitRatherThanWalk. A person who stays put cannot be counting
// the steps you took without them, and Town — which fires on walking into *any*
// settlement — would come due in a town they are not standing in. Return is
// theirs: it is Town narrowed to the one place they actually are.
func TestResidentsWaitRatherThanWalk(t *testing.T) {
	for _, s := range tables(t).Threads.Threads {
		if !s.Resident {
			continue
		}
		if len(s.Beats) < 2 {
			t.Errorf("%q: %d beat(s). The first is the meeting, so one beat is a story "+
				"that is over before the player can go anywhere", s.ID, len(s.Beats))
		}
		for i, b := range s.Beats {
			switch b.Trigger {
			case thread.Travel, thread.Town:
				t.Errorf("%q beat %d waits on %q, which is a companion's trigger: "+
					"they were not walking with you", s.ID, i, b.Trigger)
			}
		}
	}
	// And the mirror: nobody else may use Return, which compares against a home
	// that a companion's thread does not have.
	for _, s := range tables(t).Threads.Threads {
		if s.Resident {
			continue
		}
		for i, b := range s.Beats {
			if b.Trigger == thread.Return {
				t.Errorf("%q beat %d waits on %q, and has no address to wait at", s.ID, i, b.Trigger)
			}
		}
	}
}

// TestAResidentTellsOneThingPerVisit is the pacing, and it is the whole reason
// a resident's beats park in Owed instead of firing where they happen.
//
// The player goes away, things happen, they come back. If beats kept advancing
// while one was already owed, a long absence would empty the entire story into
// a single conversation — which is the shape of a cutscene, not a serial.
func TestAResidentTellsOneThingPerVisit(t *testing.T) {
	b := &thread.Book{Threads: []thread.Skeleton{{
		ID: "res", Resident: true, Title: "A Test",
		Beats: []thread.Beat{
			{Trigger: thread.Return, Text: "hello", Note: "come back"},
			{Trigger: thread.Fights, Need: 1, Text: "second", Note: "go and fight"},
			{Trigger: thread.Fights, Need: 1, Text: "third", Note: "go and fight again"},
		},
		Endings: []thread.Ending{{Label: "yes"}, {Label: "no", Shame: 1}},
	}}}
	th := &thread.Thread{
		Skeleton: "res", Owner: "Marta", Title: "A Test", State: thread.Open,
		HomePOI: 3, PlacePOI: -1, At: 1, Owed: "hello",
	}
	l := &thread.Log{}
	l.Add(th)

	// Nothing a resident does is ever returned for the caller to say out loud.
	for i := 0; i < 5; i++ {
		if got := l.Advance(b, thread.Event{Kind: thread.Fights, N: 1}); len(got) != 0 {
			t.Fatalf("a resident's beat was fired at the player from %d towns away", len(got))
		}
	}
	// And five fights while holding an installment advanced nothing.
	if th.At != 1 || th.Owed != "hello" {
		t.Errorf("after five fights the thread is at beat %d owing %q; want 1 and the opening line",
			th.At, th.Owed)
	}

	// Collect what they were holding, and the next one can come due — but only
	// one of it, however many events arrive.
	if got := th.Say(); got != "hello" {
		t.Fatalf("they said %q, want the opening line", got)
	}
	l.Advance(b, thread.Event{Kind: thread.Fights, N: 1})
	l.Advance(b, thread.Event{Kind: thread.Fights, N: 1})
	if th.Owed != "second" || th.At != 2 {
		t.Errorf("two fights later they are holding %q at beat %d, want the second line at beat 2",
			th.Owed, th.At)
	}
}

// TestAResidentIsNotDroppedWithASharedName. Names are made unique inside a
// company and not across a continent, so a hireling called Marta and a
// shopkeeper called Marta are two people. Letting the hireling go must not take
// the shopkeeper's story with them.
func TestAResidentIsNotDroppedWithASharedName(t *testing.T) {
	b := &thread.Book{Threads: []thread.Skeleton{
		{ID: "test", Title: "A Companion Story", Beats: []thread.Beat{{Trigger: thread.Travel, Need: 1}},
			Endings: []thread.Ending{{Label: "a"}, {Label: "b", Shame: 1}}},
		{ID: "res", Resident: true, Title: "A Resident Story", Beats: []thread.Beat{{Trigger: thread.Return}},
			Endings: []thread.Ending{{Label: "a"}, {Label: "b", Shame: 1}}},
	}}
	l := &thread.Log{}
	l.Add(&thread.Thread{Skeleton: "test", Owner: "Marta", State: thread.Open, HomePOI: -1, PlacePOI: -1})
	l.Add(&thread.Thread{Skeleton: "res", Owner: "Marta", State: thread.Open, HomePOI: 2, PlacePOI: -1})

	l.Drop(b, "Marta")
	if l.For(b, "Marta") != nil {
		t.Error("the companion's thread survived being let go")
	}
	if l.ForResident(b, 2, "Marta") == nil {
		t.Error("letting a hireling go took a shopkeeper's story with it")
	}
}
