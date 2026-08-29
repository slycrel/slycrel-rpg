package game

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
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

// A town may never show more marks than it can actually honour.
//
// The hashes behind "this person has an errand" and "this person has a story"
// are per-villager and fire for roughly half of them, but the code that grants
// either caps it at one per settlement. Marking everybody whose hash says yes
// would put three or four stars over a town that can produce exactly one
// errand and one story — which is not a hint, it is a lie with a shape, and the
// player finds out by walking into all of them, which is the problem the mark
// was added to solve.
func TestATownNeverPromisesMoreThanItCanGive(t *testing.T) {
	g := storyGame(t)

	towns := 0
	for idx, p := range g.World.POIs {
		if !p.Kind.Settlement() {
			continue
		}
		g.Local = world.BuildLocal(p, g.Write)
		g.LocalWalk.Place(g.Local.Entry)

		offers, people := 0, 0
		for _, e := range g.Local.Entities {
			if e.Kind == world.ENPC {
				people++
			}
			if g.attention(e, idx) == attentionOffer {
				offers++
			}
		}
		// One errand, one story, and at most one person waiting outside the
		// inn: three is the ceiling, and only a settlement with an inn reaches
		// it. Anything above that is the mark counting hashes rather than
		// counting what the town can hand over.
		if offers > 3 {
			t.Errorf("%s marks %d people with something on offer, out of %d; the cap is 3",
				p.Name, offers, people)
		}
		towns++
		if towns >= 8 {
			break
		}
	}
	g.Local = nil
	if towns == 0 {
		t.Skip("this continent generated no settlements")
	}
}

// An errand you are already carrying is not news.
//
// The mark is for finding what you have not seen. Putting one over the person
// whose job you are in the middle of would star the one villager in town whose
// news you have already heard — and, worse, it would look exactly like the mark
// that means "come and collect", which is the one that has to stay meaningful.
func TestAJobInProgressIsNotStarred(t *testing.T) {
	g := storyGame(t)
	idx := -1
	for i, p := range g.World.POIs {
		if p.Kind.Settlement() {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Skip("this continent generated no settlements")
	}
	g.Local = world.BuildLocal(g.World.POIs[idx], g.Write)
	g.LocalWalk.Place(g.Local.Entry)
	defer func() { g.Local = nil }()

	var giver *world.Entity
	for _, e := range g.Local.Entities {
		if e.Kind == world.ENPC {
			giver = e
			break
		}
	}
	if giver == nil {
		t.Skip("this town generated nobody to ask")
	}

	q, ok := quest.Generate(g.RNG, g.World, g.Data, g.Write, idx, giver.Name)
	if !ok {
		t.Skip("no errand could be generated for this town")
	}
	g.Quests.Add(q)

	if got := g.attention(giver, idx); got != attentionNone {
		t.Errorf("a job already taken is marked %v, want no mark at all", got)
	}
}

// Advice must never point at something the player cannot do.
//
// "Never offer a choice you are about to refuse" is the rule the shop counter
// and the hiring board already follow by greying out what you cannot afford,
// and it applies just as much to a stranger in the street. Being told to go and
// buy a bed by somebody who can see your purse is the same failure as a menu
// row that takes your keypress and says no — worse, because there is nothing
// greyed out to explain it.
//
// So the ordering in adviceKey is load-bearing, not cosmetic: money problems
// have to come before every suggestion that costs money.
func TestNobodyAdvisesSomethingYouCannotAfford(t *testing.T) {
	g := storyGame(t)

	for _, coins := range []int64{0, 1, 30, 60, 75, 120, 400, 5000, 100000} {
		for _, frac := range []float64{1.0, 0.5, 0.2} {
			g.Player.Coins = coins
			g.Player.HP = int(float64(g.Player.MaxHP) * frac)
			g.Quests = quest.Log{}
			g.Allies = nil

			key := g.adviceKey()
			bed := innCost(g.Player.Level, len(g.Party()))
			hire := rules.HireCost(core.Max(1, g.Player.Level), "", rules.Read(g.Player))

			switch key {
			case "hurt":
				if coins < bed {
					t.Errorf("%d coins, %.0f%% health: told to sleep, but a bed is %d",
						coins, frac*100, bed)
				}
			case "alone":
				if coins < hire {
					t.Errorf("%d coins: told to hire, but the going rate is %d", coins, hire)
				}
			case "flush":
				if coins < hire {
					t.Errorf("%d coins: called rich, but cannot afford a single hireling at %d",
						coins, hire)
				}
			}
			// And a broke player must always be told the one thing that costs
			// nothing, whatever else is wrong with the run.
			if coins < bed && key != "broke" {
				t.Errorf("%d coins is under the %d a bed costs, but the advice was %q",
					coins, bed, key)
			}
		}
	}
}

// Every advice key the game can produce has to have something written for it.
//
// The two halves live in different files — the conditions in Go, the lines in
// flavor.json — which is the arrangement that lets the writing be revised
// without touching code, and also the arrangement where a renamed key fails
// silently. A villager who has been selected to say something useful and then
// says nothing falls back to their own line, so nothing looks broken and the
// feature simply never fires.
func TestEveryAdviceKeyHasLinesBehindIt(t *testing.T) {
	g := storyGame(t)
	keys := map[string]bool{}
	for _, coins := range []int64{0, 60, 200, 900, 100000} {
		for _, frac := range []float64{1.0, 0.3} {
			for _, allies := range []int{0, 1} {
				g.Player.Coins = coins
				g.Player.HP = int(float64(g.Player.MaxHP) * frac)
				g.Allies = nil
				if allies > 0 {
					g.Allies = []*model.Character{rules.NewCharacter(g.RNG, "Someone", model.ClassThief)}
				}
				g.Quests = quest.Log{}
				if k := g.adviceKey(); k != "" {
					keys[k] = true
				}
			}
		}
	}
	if len(keys) == 0 {
		t.Fatal("no advice key fired for any state, so none of this is reachable")
	}
	for k := range keys {
		if len(g.Data.Text.Advice[k]) == 0 {
			t.Errorf("adviceKey can return %q, but flavor.json has no lines for it", k)
		}
	}
}

// Bulk selling must never touch something an errand is counting.
//
// A fetch quest names an item and SyncFetch recounts the bag afterwards, so a
// key that swept the pack would silently undo a job in progress: the log would
// go on claiming a number the pack no longer backs up, and the only clue would
// be a counter that had gone backwards for no stated reason. The single-row
// sell is still allowed to do it, because that is somebody deliberately selling
// a named thing they are looking at.
func TestSellingTheJunkKeepsWhatAnErrandWants(t *testing.T) {
	g := storyGame(t)
	s := &shopScene{e: &world.Entity{Name: "Blacksmith"}, tab: tabSell}

	junk := trinkets(t, g, 3)
	for _, it := range junk {
		it.Count = 4
		g.Player.AddItem(it)
	}
	// One of them is what somebody asked for.
	g.Quests.Add(&quest.Quest{
		ID: "test", Kind: quest.Fetch, State: quest.Active,
		Title: "Bring me four", Item: junk[1].Name, Need: 4,
	})
	g.Quests.SyncFetch(g.Player.Bag)

	before := g.Player.Coins
	s.sellJunk(g)

	left := map[string]int{}
	for _, it := range g.Player.Bag {
		left[it.Name] = it.Count
	}
	if left[junk[1].Name] != 4 {
		t.Errorf("the errand wanted 4 %s and the sweep left %d",
			junk[1].Name, left[junk[1].Name])
	}
	for _, n := range []string{junk[0].Name, junk[2].Name} {
		if left[n] != 0 {
			t.Errorf("%s is junk nobody asked for, and %d survived the sweep", n, left[n])
		}
	}
	if g.Player.Coins <= before {
		t.Errorf("the sweep sold two stacks and paid %d", g.Player.Coins-before)
	}
	// And the errand's counter still matches the pack it is counting.
	if q := g.Quests.Active()[0]; q.Have != 4 {
		t.Errorf("the errand reads %d/%d after the sweep, want 4", q.Have, q.Need)
	}
}

// Selling a row sells the pile, not one off the top of it.
func TestSellingARowSellsTheWholeStack(t *testing.T) {
	g := storyGame(t)
	s := &shopScene{e: &world.Entity{Name: "Blacksmith"}, tab: tabSell}

	it := trinkets(t, g, 1)[0]
	it.Count = 7
	g.Player.AddItem(it)
	g.Player.Coins = 0

	s.refresh(g)
	var row ui.MenuItem
	for _, mi := range s.menu.Items {
		if r, ok := mi.Data.(sellRow); ok && !r.gear && g.Player.Bag[r.idx].Name == it.Name {
			row = mi
		}
	}
	// What the row quotes is what the keypress has to pay.
	want := int64(sellPrice(it.Value) * 7)
	if row.Detail != fmt.Sprintf("%d", want) {
		t.Errorf("the row quotes %q for seven, want %d", row.Detail, want)
	}

	s.sell(g, row)
	for _, b := range g.Player.Bag {
		if b.Name == it.Name {
			t.Errorf("%d %s left in the pack after selling the stack", b.Count, it.Name)
		}
	}
	if g.Player.Coins != want {
		t.Errorf("paid %d for seven, want %d", g.Player.Coins, want)
	}
}

// trinkets pulls n distinct sellable-junk items out of the tables.
func trinkets(t *testing.T, g *Game, n int) []model.Item {
	t.Helper()
	var out []model.Item
	for _, it := range g.Data.Items {
		if it.Kind == model.ItemTrinket && it.Value > 1 {
			out = append(out, it)
		}
		if len(out) == n {
			return out
		}
	}
	t.Skipf("only %d sellable trinkets in the tables, need %d", len(out), n)
	return nil
}

// The sweep is a row, and it must not be reachable by a movement key.
//
// It was very nearly bound to S, which is the down key on WASD — so scrolling
// the sell list would have emptied the pack, and the player would have had no
// idea which press did it. This pins the row's existence and, by having no
// hotkey to test, the absence of the binding.
func TestTheSweepIsARowAndQuotesItsPrice(t *testing.T) {
	g := storyGame(t)
	s := &shopScene{e: &world.Entity{Name: "Blacksmith"}, tab: tabSell}

	// Nothing to sweep: the row must not be offered at all, rather than offered
	// and refused.
	s.refresh(g)
	if findSweep(s) != nil {
		t.Error("an empty pack still offers to sell everything in it")
	}

	junk := trinkets(t, g, 2)
	for _, it := range junk {
		it.Count = 5
		g.Player.AddItem(it)
	}
	s.refresh(g)

	row := findSweep(s)
	if row == nil {
		t.Fatal("a pack full of junk does not offer to sell it")
	}
	worth, pieces := s.junkWorth(g)
	if pieces != 10 {
		t.Errorf("the sweep covers %d pieces, want 10", pieces)
	}
	if row.Detail != fmt.Sprintf("%d", worth) {
		t.Errorf("the row quotes %q, and the sweep pays %d", row.Detail, worth)
	}

	before := g.Player.Coins
	s.sell(g, *row)
	if got := g.Player.Coins - before; got != int64(worth) {
		t.Errorf("the row quoted %d and paid %d", worth, got)
	}
}

func findSweep(s *shopScene) *ui.MenuItem {
	for i, mi := range s.menu.Items {
		if _, ok := mi.Data.(sellAll); ok {
			return &s.menu.Items[i]
		}
	}
	return nil
}

// Nobody in the foe field overlaps anybody else, and nobody hangs off it.
//
// Every mark on a creature is positioned from its slot — the portrait, the
// frame, the health meter, the condition pips, the damage number, the burst —
// so a grid that overlapped would not merely look crowded, it would attach the
// wrong number to the wrong monster at the exact moment the player is deciding
// which one to hit. Six is the ceiling: four is the most an ordinary encounter
// sends (party.MaxFoes) and a pack adds two on top of that.
func TestTheFoeFieldHoldsEverybodyWithoutOverlap(t *testing.T) {
	for n := 1; n <= party.MaxFoes+2; n++ {
		type box struct{ x, y, w, h float64 }
		var boxes []box
		for i := 0; i < n; i++ {
			x, top, w, h := monBox(i, n)
			// The block is the portrait plus the meter, pips and name under it.
			b := box{x, top, w, h + monBelow(n)}
			if b.x < foeFieldX-1 || b.x+b.w > foeFieldX+foeFieldW+1 {
				t.Errorf("%d foes: slot %d runs from x %.0f to %.0f, field is %.0f..%.0f",
					n, i, b.x, b.x+b.w, foeFieldX, foeFieldX+foeFieldW)
			}
			if b.y < foeFieldY-1 || b.y+b.h > foeFieldY+foeFieldH+1 {
				t.Errorf("%d foes: slot %d runs from y %.0f to %.0f, field is %.0f..%.0f",
					n, i, b.y, b.y+b.h, foeFieldY, foeFieldY+foeFieldH)
			}
			for j, o := range boxes {
				if b.x < o.x+o.w && o.x < b.x+b.w && b.y < o.y+o.h && o.y < b.y+b.h {
					t.Errorf("%d foes: slot %d overlaps slot %d", n, i, j)
				}
			}
			boxes = append(boxes, b)
		}
	}
}

// The bottom bar and the panel that overdraws it both have to fit the frame.
//
// The command panel deliberately sits *on* the transcript rather than beside
// it, so the thing to check is not that they miss each other but that the
// overlay stays inside the bar it is covering and leaves usable room beside it
// — a negative remainder there would wrap the transcript to nothing and print
// one character per line.
func TestTheBottomStripFits(t *testing.T) {
	if cmdPanelX < logPanelX || cmdPanelX+cmdPanelW > logPanelX+logPanelW {
		t.Errorf("the command panel runs %.0f..%.0f, outside the bar at %.0f..%.0f",
			cmdPanelX, cmdPanelX+cmdPanelW, logPanelX, logPanelX+logPanelW)
	}
	if barSideX+barSideW > logPanelX+logPanelW {
		t.Errorf("the space beside the command panel ends at %.0f, past the bar at %.0f",
			barSideX+barSideW, logPanelX+logPanelW)
	}
	// Enough to wrap a sentence into rather than a column of single letters.
	if barSideW < 120 {
		t.Errorf("only %.0f pixels beside the command panel; a transcript needs more", barSideW)
	}
	if logPanelX+logPanelW > render.ScreenW {
		t.Errorf("the transcript ends at %.0f, past a %d-pixel screen",
			logPanelX+logPanelW, render.ScreenW)
	}
	if battleBarY+battleBarH > render.ScreenH {
		t.Errorf("the bottom strip ends at %.0f, past a %d-pixel screen",
			battleBarY+battleBarH, render.ScreenH)
	}
	// And the field above it must not reach into it.
	if partyPanelY+partyPanelH > battleBarY || foeFieldY+foeFieldH > battleBarY {
		t.Error("the field overlaps the bottom strip")
	}
	if partyPanelX+partyPanelW > foeFieldX {
		t.Error("the company column overlaps the foe field")
	}
}

// The whole company fits in its column.
func TestTheCompanyColumnHoldsAFullParty(t *testing.T) {
	n := party.MaxSize
	for i := 0; i < n; i++ {
		ry := partyRowY(i, n)
		if ry < partyPanelY-1 || ry+partyRowH > partyPanelY+partyPanelH+1 {
			t.Errorf("member %d of %d sits at y %.0f..%.0f, panel is %.0f..%.0f",
				i, n, ry, ry+partyRowH, partyPanelY, partyPanelY+partyPanelH)
		}
	}
}

// A creature's name splits into what it is and what sort of one.
func TestAMonstersNameSplitsIntoSpeciesAndEpithet(t *testing.T) {
	for _, c := range []struct{ full, head, tail string }{
		{"Crab, Territorial", "Crab", "Territorial"},
		{"Mummy, Poorly Wrapped", "Mummy", "Poorly Wrapped"},
		// The group letter belongs to the head, or two of them label
		// identically at the moment the player is choosing between them.
		{"Crab, Territorial A", "Crab A", "Territorial"},
		{"Crab, Territorial B", "Crab B", "Territorial"},
		// A brute keeps its prefix, which is the part that says it is a brute.
		{"A Very Large Goblin, Middle Manager", "A Very Large Goblin", "Middle Manager"},
		// Names with no epithet are all head.
		{"Something That Was A Diver", "Something That Was A Diver", ""},
	} {
		head, tail := monsterName(c.full)
		if head != c.head || tail != c.tail {
			t.Errorf("%q split to (%q, %q), want (%q, %q)", c.full, head, tail, c.head, c.tail)
		}
	}
}

// Splitting a name never loses part of it.
//
// Half the roster is not written "Species, Epithet" — "Owl That Knows" and
// "Something That Was A Diver" are whole phrases and the phrase is the joke —
// so the split has to be lossless rather than clever: whatever it does not put
// in the head must come back in the tail, because between them they are what
// the target panel shows and that is the one place the player reads a
// creature's whole name.
func TestSplittingANameNeverLosesPartOfIt(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory to load: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	seen := map[string]bool{}
	for _, defs := range tables.Monsters {
		for _, m := range defs {
			if seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			for _, full := range []string{m.Name, m.Name + " A"} {
				head, tail := monsterName(full)
				rejoined := head
				if tail != "" {
					rejoined += " " + tail
				}
				// Compared without the comma, which is the separator being
				// consumed rather than a word being lost.
				for _, word := range strings.Fields(strings.ReplaceAll(full, ",", "")) {
					if !strings.Contains(rejoined, word) {
						t.Errorf("%q split to (%q, %q), which drops %q",
							full, head, tail, word)
					}
				}
			}
		}
	}
}
