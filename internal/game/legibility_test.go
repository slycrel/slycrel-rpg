package game

import (
	"testing"
	"time"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// A companion's story has to set the destination even when the map already
// knows the place.
//
// This is the bug the whole tracking half of the feature had: revealing is a
// one-time thing and following is not, so showThreadDestination checked
// Discovered and returned before it had done anything at all. A thread pointing
// at somewhere the player had already walked past produced no pin, no compass
// and no line in the log — which is indistinguishable from the story being
// broken, and is what it was reported as.
func TestACompanionsErrandIsFollowedEvenSomewhereYouHaveBeen(t *testing.T) {
	g := storyGame(t)

	// A destination the player has already found. This is the case that failed.
	const target = 3
	if target >= len(g.World.POIs) {
		t.Skip("this continent has too few locations to stage the case")
	}
	g.World.POIs[target].Discovered = true

	g.showThreadDestination(reachThread(t, g, target))

	if !g.Track.On {
		t.Fatal("nothing is being followed after a companion named a destination")
	}
	if g.Track.POI != target {
		t.Errorf("following location %d, want %d", g.Track.POI, target)
	}
	if _, ok := g.trackBearing(); !ok {
		t.Error("the destination is followed but the compass has no bearing for it")
	}
}

// And it must never take a destination the player chose on purpose.
//
// trackIfIdle rather than trackPOI. A backstory is the least urgent thing in
// the journal, and a companion remembering something mid-walk is not a reason
// to stop pointing at the errand somebody deliberately selected.
func TestACompanionsErrandDoesNotStealTheChosenDestination(t *testing.T) {
	g := storyGame(t)
	if len(g.World.POIs) < 5 {
		t.Skip("this continent has too few locations to stage the case")
	}

	g.trackPOI(1, "Somewhere Chosen")
	g.showThreadDestination(reachThread(t, g, 4))

	if g.Track.POI != 1 {
		t.Errorf("a backstory moved the tracker to %d; the player had chosen 1", g.Track.POI)
	}
}

// reachThread is a companion's story parked on the beat that waits for the
// player to arrive somewhere.
//
// Wound forward to the last beat rather than invented, because Awaiting reads
// the skeleton out of the book: a made-up id has no beats, answers "" to every
// question, and would make showThreadDestination return before reaching any of
// the code these tests are about — passing both of them for no reason at all.
func reachThread(t *testing.T, g *Game, place int) *thread.Thread {
	t.Helper()
	const skeleton = "undead-contract"
	sk, ok := g.Data.Threads.Get(skeleton)
	if !ok {
		t.Skipf("%s is no longer a thread skeleton", skeleton)
	}
	at := -1
	for i, b := range sk.Beats {
		if b.Trigger == thread.Reach {
			at = i
		}
	}
	if at < 0 {
		t.Skipf("%s no longer has a beat that waits on arriving somewhere", skeleton)
	}
	th := &thread.Thread{
		Skeleton: skeleton, Owner: "Someone", Title: "A Long Way Round",
		State: thread.Open, PlacePOI: place, HomePOI: -1, At: at,
	}
	if got := th.Awaiting(&g.Data.Threads); got != thread.Reach {
		t.Fatalf("the staged thread is waiting on %q, not on arriving anywhere", got)
	}
	return th
}

// storyGame is a run standing on its opening tile with content loaded.
func storyGame(t *testing.T) *Game {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory to load: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	const seed = 1994
	write := content.New(&tables.Text)
	g := &Game{
		Root: root, Data: tables, Write: write,
		RNG: core.NewRNG(seed), Seed: seed, Log: ui.NewLog(20),
		World: world.Generate(seed, write),
	}
	g.Player = rules.NewCharacter(g.RNG, "Bosk", model.ClassFighter)
	g.Walk.Place(g.World.Start)
	return g
}

// The corner map's window has to stay on the continent.
//
// Everything drawn into the panel is positioned relative to this origin — the
// player blip, the pins, the followed marker — so a window that hangs off the
// edge does not merely show a band of nothing, it puts every mark on the panel
// at the wrong place. The clamp is also what makes the coast look like a coast
// rather than the map running out.
func TestTheCornerMapNeverLooksOffTheEdgeOfTheWorld(t *testing.T) {
	for _, c := range []struct {
		name string
		at   core.Point
	}{
		{"the top-left corner", core.Point{X: 0, Y: 0}},
		{"the bottom-right corner", core.Point{X: world.Width - 1, Y: world.Height - 1}},
		{"the middle", core.Point{X: world.Width / 2, Y: world.Height / 2}},
		{"just inside the north edge", core.Point{X: 40, Y: 2}},
		{"just inside the east edge", core.Point{X: world.Width - 2, Y: 60}},
	} {
		x, y := miniWindow(c.at)
		if x < 0 || y < 0 || x+miniTiles > world.Width || y+miniTiles > world.Height {
			t.Errorf("%s: window (%d,%d)+%d runs off a %dx%d continent",
				c.name, x, y, miniTiles, world.Width, world.Height)
		}
		// And the player has to actually be inside the window they are the
		// centre of, or the blip is drawn outside its own panel.
		if c.at.X < x || c.at.X >= x+miniTiles || c.at.Y < y || c.at.Y >= y+miniTiles {
			t.Errorf("%s: player at %v is not inside window (%d,%d)+%d",
				c.name, c.at, x, y, miniTiles)
		}
	}
}

// The save list re-ages against the clock rather than against whenever the
// screen happened to open.
//
// The bug this pins was reported as "the timestamp is wrong, but right after
// you re-save". Both halves follow from one cause: the age was rendered once at
// refresh and then left, so it was correct only for the instant the screen
// appeared, and saving — the one action that refreshes — was the only thing
// that ever put it right.
func TestTheSaveListDoesNotFreezeItsAgeColumn(t *testing.T) {
	s := &slotScene{}
	s.menu.SetItems([]ui.MenuItem{{Label: "a run", Detail: "just now"}})
	s.when = []time.Time{time.Now().Add(-95 * time.Minute)}

	s.reage()

	if got := s.menu.Items[0].Detail; got != "1h ago" {
		t.Errorf("the age column reads %q, want %q", got, "1h ago")
	}
}

// Typing a seed has to produce the run that seed names.
//
// The whole point of putting the field on the screen is that "seed 1994" at the
// bottom of the title stops being a number the game shows you and will not take
// back. If typing it gave a different opening from launching with -seed 1994,
// the field would be worse than not having one: it would be a promise the game
// visibly makes and quietly breaks, and the only way to notice would be to
// compare two runs by hand.
//
// This is what makes setSeed reroll the person as well as the world. Everything
// on that screen is forked off the seed, so applying it to half of them would
// make the same number mean two different runs depending on which order the
// player pressed things in.
func TestTypingASeedGivesTheSameRunAsLaunchingWithIt(t *testing.T) {
	const want = 1994

	// The run you get from the command line.
	launched := newCreateScene(creationGame(t, want))

	// The run you get by typing it in on a screen that opened on another world.
	typed := creationGame(t, 20250829)
	edited := newCreateScene(typed)
	edited.seedText = "1994"
	edited.commitSeed(typed)

	if typed.Seed != want {
		t.Fatalf("the field committed seed %d, want %d", typed.Seed, want)
	}
	if edited.name != launched.name || edited.epithet != launched.epithet {
		t.Errorf("typed opens on %q %q, launched opens on %q %q",
			edited.name, edited.epithet, launched.name, launched.epithet)
	}
	if edited.faceIdx != launched.faceIdx || edited.lookIdx != launched.lookIdx {
		t.Errorf("typed opens on face %d look %d, launched on face %d look %d",
			edited.faceIdx, edited.lookIdx, launched.faceIdx, launched.lookIdx)
	}
	for _, class := range model.AllClasses {
		a, b := edited.rolled[class], launched.rolled[class]
		if a == nil || b == nil {
			t.Fatalf("no %s rolled on one of the two screens", class)
		}
		if a.MaxHP != b.MaxHP || a.Strength != b.Strength || a.Focus() != b.Focus() {
			t.Errorf("the %s throw differs: typed %d HP / %d str, launched %d HP / %d str",
				class, a.MaxHP, a.Strength, b.MaxHP, b.Strength)
		}
	}
}

// A field that says nothing must not be read as a seed.
//
// Zero is the sentinel -seed uses for "pick one from the clock", so committing
// an empty or half-deleted field would hand the player a continent at the exact
// moment they were most specifically asking for a particular one.
func TestClearingTheSeedFieldChangesNothing(t *testing.T) {
	g := creationGame(t, 4242)
	c := newCreateScene(g)
	for _, bad := range []string{"", "0", "banana"} {
		c.seedText = bad
		c.commitSeed(g)
		if g.Seed != 4242 {
			t.Errorf("a field reading %q moved the world to seed %d", bad, g.Seed)
		}
	}
}

// creationGame is a game sitting on the character screen for one seed.
func creationGame(t *testing.T, seed int64) *Game {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory to load: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	return &Game{
		Root: t.TempDir(), Data: tables, Write: content.New(&tables.Text),
		RNG: core.NewRNG(seed), Seed: seed, Log: ui.NewLog(4),
	}
}
