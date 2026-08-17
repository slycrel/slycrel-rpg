package game

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The tour plays its steps in list order and only checks that the clock has
// reached each one, so a step listed after a later tick fires the instant the
// one before it does — and its capture lands on the same frame as its
// neighbour's. This is exactly the shape of a bug that was in the script.
func TestDemoStepsAreInClockOrder(t *testing.T) {
	prev := -1
	for i, s := range buildDemoSteps() {
		if s.at < prev {
			t.Errorf("step %d fires at tick %d, before step %d's %d", i, s.at, i-1, prev)
		}
		prev = s.at
	}
}

// Every capture in the tour needs its own name, or two of them write to the
// same file and one screen silently goes unreviewed.
func TestDemoShotNamesAreUnique(t *testing.T) {
	seen := map[string]int{}
	for i, s := range buildDemoSteps() {
		if s.shot == "" {
			continue
		}
		if first, dup := seen[s.shot]; dup {
			t.Errorf("steps %d and %d both capture %q", first, i, s.shot)
		}
		seen[s.shot] = i
	}
}

// A run with a company in it has to survive being written out and read back,
// including the parts that are not stored: the marching order is rebuilt from
// where the hero is standing rather than saved, and that has to actually happen
// on load or the first draw indexes past the end of the line.
func TestPartySurvivesASaveAndLoad(t *testing.T) {
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
	for _, class := range []model.Class{model.ClassMage, model.ClassThief} {
		c := rules.Recruit(g.RNG, "Hire"+string(class), class, "", 4)
		tables.Equip(c)
		c.Sprite, c.Portrait = "hero/druid", "portrait/female/f_08"
		c.HP = c.MaxHP / 2 // mid-run, not freshly hired
		g.Allies = append(g.Allies, c)
	}
	g.Walk.Place(g.World.Start)
	g.reformLines()
	// Mid-evening, and deliberately not on a phase boundary: a clock that came
	// back rounded to the nearest dawn would still pass a coarser check.
	g.Clock.Step = sky.DayLength*3 + 391

	loaded := &Game{
		Root: root, Data: tables, Write: write,
		RNG: core.NewRNG(1), Log: ui.NewLog(20),
	}
	if err := loaded.Restore(g.Snapshot()); err != nil {
		t.Fatalf("restoring the run: %v", err)
	}

	// The sky is not stored, only the clock it is derived from — so this one
	// field is the whole of whether a save comes back to the same evening. An
	// absent clock reads as the first dawn of the run, which is a fair answer
	// for an old save and a silent bug for a new one.
	if loaded.Clock != g.Clock {
		t.Errorf("saved at step %d, came back at %d", g.Clock.Step, loaded.Clock.Step)
	}
	if loaded.Clock.Phase() != g.Clock.Phase() {
		t.Errorf("saved in the %s, came back in the %s",
			g.Clock.Phase().Name(), loaded.Clock.Phase().Name())
	}

	if len(loaded.Allies) != len(g.Allies) {
		t.Fatalf("saved %d companions, got back %d", len(g.Allies), len(loaded.Allies))
	}
	for i, want := range g.Allies {
		got := loaded.Allies[i]
		switch {
		case got.Name != want.Name:
			t.Errorf("companion %d came back as %q, want %q", i, got.Name, want.Name)
		case got.Level != want.Level || got.HP != want.HP || got.MaxHP != want.MaxHP:
			t.Errorf("companion %d came back at L%d %d/%d, want L%d %d/%d",
				i, got.Level, got.HP, got.MaxHP, want.Level, want.HP, want.MaxHP)
		case got.Cut != want.Cut || !got.Ally:
			t.Errorf("companion %d came back with cut %d and ally=%v", i, got.Cut, got.Ally)
		case got.Sprite != want.Sprite || got.Portrait != want.Portrait:
			t.Errorf("companion %d came back looking like %q/%q, want %q/%q",
				i, got.Sprite, got.Portrait, want.Sprite, want.Portrait)
		case got.Weapon.Name != want.Weapon.Name || got.Armor.Name != want.Armor.Name:
			t.Errorf("companion %d came back holding %q/%q, want %q/%q",
				i, got.Weapon.Name, got.Armor.Name, want.Weapon.Name, want.Armor.Name)
		}
	}

	// The line is not in the file; Restore has to rebuild it.
	if len(loaded.follow) != len(loaded.Allies) {
		t.Fatalf("restored %d companions but %d walkers", len(loaded.Allies), len(loaded.follow))
	}
	for i, w := range loaded.follow {
		if w.Tile != loaded.Walk.Tile {
			t.Errorf("companion %d re-formed at %v, want the hero's tile %v", i, w.Tile, loaded.Walk.Tile)
		}
	}
}

// Everyone who loses initiative acts after the monsters, so anybody can be dead
// by the time their queued step comes up. A corpse must not take its turn:
// before this was guarded, a hero killed during the monster phase still swung,
// drank potions and fled afterwards.
func TestTheDeadDoNotTakeTheirTurn(t *testing.T) {
	hero := &model.Character{Name: "Bosk", HP: 10, MaxHP: 10, Speed: 1}
	mate := &model.Character{Name: "Nessa", Ally: true, HP: 10, MaxHP: 10, Speed: 1}
	b := &battleScene{party: []*model.Character{hero, mate}}

	acted := 0
	// Build the round as runRound does, then kill everybody before draining it,
	// which is what a bad monster phase amounts to.
	for _, c := range b.party {
		member := c
		b.queue = append(b.queue, func(g *Game) {
			if !member.Alive() || b.result != 0 {
				return
			}
			acted++
		})
	}
	hero.HP, mate.HP = 0, 0
	for _, step := range b.queue {
		step(nil)
	}
	if acted != 0 {
		t.Fatalf("%d dead combatants took a turn", acted)
	}

	// And the guard must not stop the living from acting, or the fix would have
	// traded one bug for a worse one.
	acted = 0
	hero.HP = 5
	for _, step := range b.queue {
		step(nil)
	}
	if acted != 1 {
		t.Fatalf("%d of the party acted; only the one still standing should have", acted)
	}
}

// Supplies bought for a companion must come back when they are let go, or
// stocking anybody up is a bet rather than a purchase.
func TestDismissingReturnsTheirSupplies(t *testing.T) {
	hero := &model.Character{Name: "Bosk"}
	mate := &model.Character{Name: "Nessa", Ally: true, Bag: []model.Item{
		{Name: "Field Poultice", Kind: model.ItemHeal, Power: 20, Count: 2},
		{Name: "Small Beer", Kind: model.ItemHeal, Power: 8, Count: 1},
	}}
	g := &Game{Player: hero, Allies: []*model.Character{mate}}

	if n := g.reclaim(mate); n != 3 {
		t.Fatalf("reclaimed %d items, want 3", n)
	}
	if len(mate.Bag) != 0 {
		t.Errorf("the companion still carries %d stacks", len(mate.Bag))
	}
	if len(hero.Bag) != 2 {
		t.Fatalf("the hero got back %d stacks, want 2", len(hero.Bag))
	}
	var total int
	for _, it := range hero.Bag {
		total += it.Count
	}
	if total != 3 {
		t.Errorf("the hero got back %d items, want 3", total)
	}

	// And it stacks with what the hero already had rather than duplicating.
	mate.Bag = []model.Item{{Name: "Small Beer", Kind: model.ItemHeal, Power: 8, Count: 4}}
	g.reclaim(mate)
	for _, it := range hero.Bag {
		if it.Name == "Small Beer" && it.Count != 5 {
			t.Errorf("Small Beer came back as %d, want 5 stacked", it.Count)
		}
	}
	if len(hero.Bag) != 2 {
		t.Errorf("reclaiming created a duplicate stack: %d kinds", len(hero.Bag))
	}
}

// The run has to be recoverable from the moment before a fight, because that
// is the only thing standing between a bad encounter roll and an hour gone.
// Encounters are rolled at the player rather than chosen, so the step that
// killed them was not a decision they had a chance to evaluate.
func TestAutosaveCapturesTheRunAsItStoodBeforeTheFight(t *testing.T) {
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
	// A temporary root, so the suite never writes into the repository's own
	// saves directory.
	tmp := t.TempDir()
	g := &Game{
		Root: tmp, Data: tables, Write: write,
		RNG: core.NewRNG(seed), Seed: seed, Log: ui.NewLog(20),
		World: world.Generate(seed, write),
	}
	g.Player = rules.NewCharacter(g.RNG, "Bosk", model.ClassFighter)
	g.Player.HP = g.Player.MaxHP
	g.Walk.Place(g.World.Start)
	g.reformLines()

	g.autosave()

	f, err := save.Load(tmp, AutosaveSlot)
	if err != nil {
		t.Fatalf("nothing was written to the autosave slot: %v", err)
	}
	if f.Player == nil || f.Player.HP != g.Player.HP {
		t.Errorf("the autosave holds %v, the run held %d hit points", f.Player, g.Player.HP)
	}
	if f.At != g.Walk.Tile {
		t.Errorf("the autosave puts the player at %v, they were at %v", f.At, g.Walk.Tile)
	}

	// And it has to be a save like any other, so that loading it needs no
	// special path and cannot rot separately from the rest of the format.
	if err := g.Restore(f); err != nil {
		t.Errorf("the autosave would not load back: %v", err)
	}
}

// The tour writes no files. It runs on a machine that is only taking pictures,
// and an autosave firing during it would scribble over a real run.
func TestTheDemoDoesNotAutosave(t *testing.T) {
	tmp := t.TempDir()
	var tables gamedata.Tables
	g := &Game{Root: tmp, Log: ui.NewLog(20)}
	g.StartDemo()
	g.Player = &model.Character{Name: "Bosk", Level: 1, HP: 1, MaxHP: 1}
	g.World = world.Generate(1, content.New(&tables.Text))

	g.autosave()
	if _, err := save.Load(tmp, AutosaveSlot); err == nil {
		t.Error("the tour wrote an autosave")
	}
}

// The pause menu acts on the label of the highlighted row, not its position.
//
// It used to switch on the index, and the Sound row had been inserted second
// without the numbers moving — so Save opened the *load* picker, where every
// empty slot is disabled and no slot can be selected. Saving was unreachable
// from the only menu that offers it, and nothing said so. This asserts the two
// lists cannot drift apart again.
func TestEveryPauseRowHasSomethingBehindIt(t *testing.T) {
	g := &Game{Log: ui.NewLog(4)}
	p := &pauseScene{}
	p.refresh(g)

	// Every row the menu offers must be a label the dispatch knows.
	handled := map[string]bool{
		"Resume": true, "Sound": true, "Save": true, "Load": true, "Abandon run": true,
	}
	if len(p.menu.Items) != len(handled) {
		t.Errorf("the pause menu shows %d rows and the dispatch knows %d",
			len(p.menu.Items), len(handled))
	}
	for i, it := range p.menu.Items {
		if !handled[it.Label] {
			t.Errorf("row %d is %q, which nothing acts on", i, it.Label)
		}
		p.menu.Index = i
		if got := p.selected(); got != it.Label {
			t.Errorf("row %d reads back as %q, not %q", i, got, it.Label)
		}
	}
}

// A save picker has to offer its slots. The load picker greys out the empty
// ones on purpose; the save picker must not, or there is nowhere to save to on
// a fresh run — which is every run the first time.
func TestTheSavePickerOffersEmptySlots(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	g := &Game{Root: t.TempDir(), Log: ui.NewLog(4)}
	_ = root

	for _, mode := range []slotMode{slotSave, slotLoad} {
		s := &slotScene{mode: mode}
		s.refresh(g)
		if len(s.menu.Items) == 0 {
			t.Fatalf("mode %v offers no slots at all", mode)
		}
		enabled := 0
		for _, it := range s.menu.Items {
			if !it.Disabled {
				enabled++
			}
		}
		switch mode {
		case slotSave:
			if enabled != len(s.menu.Items) {
				t.Errorf("the save picker greys out %d of %d empty slots",
					len(s.menu.Items)-enabled, len(s.menu.Items))
			}
		case slotLoad:
			if enabled != 0 {
				t.Errorf("the load picker offers %d slots with nothing in them", enabled)
			}
		}
	}
}

// The creation screen hands the game the character it was showing.
//
// It used to show one and start another: the panel rolled a preview per class
// from its own forked generator, and startRun then rolled a fresh character
// from the main stream and gave the player that. Every number on that screen
// was a number from a throw nobody kept, which is a strange thing to let
// somebody choose a class on and an impossible thing to offer a reroll of.
func TestCreationHandsOverTheCharacterItShowed(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	g := &Game{
		Root: t.TempDir(), Data: tables, Write: content.New(&tables.Text),
		RNG: core.NewRNG(7), Seed: 7, Log: ui.NewLog(4),
	}
	c := newCreateScene(g)

	for _, class := range model.AllClasses {
		p := c.rolled[class]
		if p == nil {
			t.Fatalf("no character rolled for %s", class)
		}
		if p.Class != class {
			t.Errorf("the %s row is showing a %s", class, p.Class)
		}
		if p.MaxHP < 1 || p.MaxPsyche < 0 {
			t.Errorf("%s rolled %d hit points and %d psyche", class, p.MaxHP, p.MaxPsyche)
		}
	}

	// Rolling again has to actually move the numbers, or the control is a lie.
	before := *c.rolled[model.ClassFighter]
	moved := false
	for i := 0; i < 20 && !moved; i++ {
		c.rerollStats()
		after := c.rolled[model.ClassFighter]
		moved = after.MaxHP != before.MaxHP || after.Strength != before.Strength ||
			after.Dexterity != before.Dexterity || after.Speed != before.Speed ||
			after.MaxPsyche != before.MaxPsyche || after.Coins != before.Coins
	}
	if !moved {
		t.Error("twenty stat rerolls produced the same fighter every time")
	}

	// And so does the other side of the screen.
	name := c.name
	changed := false
	for i := 0; i < 20 && !changed; i++ {
		c.rerollName(g)
		changed = c.name != name || c.epithet != ""
	}
	if !changed {
		t.Error("rerolling the name never changed it")
	}
}

// Nothing in the class list may be an action. Left and right are the rerolls
// now, and a row that is not a class or the way out would be a third thing the
// cursor can land on with no stats to show for it.
func TestTheClassListIsOnlyClasses(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	g := &Game{
		Root: t.TempDir(), Data: tables, Write: content.New(&tables.Text),
		RNG: core.NewRNG(7), Seed: 7, Log: ui.NewLog(4),
	}
	c := newCreateScene(g)
	for i, it := range c.menu.Items {
		if it.Label == "Back" {
			continue
		}
		if _, ok := it.Data.(model.Class); !ok {
			t.Errorf("row %d is %q, which is neither a class nor the way out", i, it.Label)
		}
	}
}

// The face and the walk sheet the player picked have to survive being handed
// to the game.
//
// This is the only screen in Slycrel where art is chosen rather than issued,
// and everything downstream of it defaults: heroSpriteKey falls back to a sheet
// named after the class and portraitOf falls back to m_01, so a startRun that
// blanked either field would produce a hero who looked exactly like the one the
// player did not pick, with nothing on screen to say what happened.
func TestTheChosenFaceAndLookSurviveStartingTheRun(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	const seed = 4242
	write := content.New(&tables.Text)
	g := &Game{
		Root: t.TempDir(), Data: tables, Write: write,
		RNG: core.NewRNG(seed), Seed: seed, Log: ui.NewLog(20),
	}

	c := newCreateScene(g)
	// Somewhere other than the first entry of either list, so a bug that reset
	// to the default has something to be caught by.
	c.lookIdx, c.faceIdx = len(heroLooks)-1, len(c.faces)-1
	wantLook, wantFace := heroLooks[c.lookIdx].Key, c.face()

	p := c.dress(c.rolled[model.ClassMage])
	if p.Sprite != wantLook || p.Portrait != wantFace {
		t.Fatalf("dressing gave sprite %q and portrait %q, want %q and %q",
			p.Sprite, p.Portrait, wantLook, wantFace)
	}

	g.startRun(p, c.name, c.epithet)
	if g.Player.Sprite != wantLook {
		t.Errorf("the run began with sprite %q, want the chosen %q", g.Player.Sprite, wantLook)
	}
	if g.Player.Portrait != wantFace {
		t.Errorf("the run began with portrait %q, want the chosen %q", g.Player.Portrait, wantFace)
	}
	// And the two lookups that would otherwise quietly substitute a default.
	if got := heroSpriteKey(g.Player, core.DirDown, false); got != wantLook+"/idle" {
		t.Errorf("the overworld draws %q, want %q", got, wantLook+"/idle")
	}
	if got := portraitOf(g.Player); got != wantFace {
		t.Errorf("the battle screen draws %q, want %q", got, wantFace)
	}
}

// Every face the creation screen offers has to be art that exists, because the
// alternative is a player scrolling into a magenta box with their own name
// under it. heroFaces probes the registry rather than listing keys for exactly
// this reason; this is the assertion that the probe is doing anything.
func TestEveryOfferedFaceIsRealArt(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	g := &Game{
		Root: root, Data: tables, Write: content.New(&tables.Text),
		RNG: core.NewRNG(1), Seed: 1, Log: ui.NewLog(4),
		Assets: assetsys.New(root),
	}
	faces := g.heroFaces()
	if len(faces) < 20 {
		t.Fatalf("the creation screen offers %d faces; the portrait packs hold far more than that", len(faces))
	}
	for _, key := range faces {
		if !g.Assets.Has(key) {
			t.Errorf("the creation screen offers %q, which is not in the manifest", key)
		}
	}
	// And the walk sheets, which are listed rather than probed.
	for _, l := range heroLooks {
		if !g.Assets.Has(l.Key + "/idle") {
			t.Errorf("the creation screen offers the %s look, and %s/idle is missing", l.Name, l.Key)
		}
	}
}

// Somebody, somewhere, has to actually have a story.
//
// The gate on a resident's backstory is three conditions deep — this person is
// the storyteller, there is room under the cap, and the continent can stage one
// of the skeletons — and every one of them is a place the feature can silently
// switch itself off. A threshold typed with one fewer zero, or a set of
// skeletons that all need a ruin on a continent with none, and the writing is
// simply never read by anybody. Nothing else in the suite would notice.
func TestTownspeopleActuallyHaveStories(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}

	var settlements, tellers, cast int
	for _, seed := range []int64{1, 7, 1994, 20260816} {
		write := content.New(&tables.Text)
		g := &Game{
			Root: t.TempDir(), Data: tables, Write: write,
			RNG: core.NewRNG(seed), Seed: seed, Log: ui.NewLog(4),
			World: world.Generate(seed, write),
		}
		g.Player = rules.NewCharacter(g.RNG, "Bosk", model.ClassFighter)
		g.Player.Level = 4

		for idx, p := range g.World.POIs {
			if !p.Kind.Settlement() {
				continue
			}
			settlements++
			g.Local = world.BuildLocal(p, write)
			g.Walk.Place(p.Pos)
			for _, e := range g.Local.Entities {
				if e.Kind != world.ENPC {
					continue
				}
				if !g.hasStory(e) {
					continue
				}
				tellers++
				if g.residentThread(e, idx) != nil {
					cast++
				}
			}
			// Cleared between towns so the cap does not silently end the sweep
			// after the first two. What is being measured here is how often a
			// town *can* offer one, not how many the player may carry.
			g.Threads = thread.Log{}
		}
	}

	if settlements == 0 {
		t.Fatal("four continents produced no settlements at all")
	}
	if tellers == 0 {
		t.Fatalf("across %d settlements nobody was the sort of person with something going on", settlements)
	}
	if cast == 0 {
		t.Fatalf("%d townspeople had something going on and not one of them could be cast in a story", tellers)
	}
	// A rough floor rather than a rate. The point is that walking into a town
	// and finding somebody is an ordinary occurrence and not a lottery win.
	if got := float64(cast) / float64(settlements); got < 0.25 {
		t.Errorf("only %.0f%% of settlements have anybody with a story (%d of %d); "+
			"at that rate a player finishes a run without meeting one",
			got*100, cast, settlements)
	}
	t.Logf("%d settlements, %d storytellers, %d cast", settlements, tellers, cast)
}

// One story per town, and never more than the cap in total.
//
// A town is a place rather than a queue — the same rule the errand log follows
// — and the path that breaks it is the ordinary one: a player walks in and
// talks to everybody, which is exactly what the demo tour does.
func TestATownOffersOneStoryAtATime(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}

	for _, seed := range []int64{1, 7, 1994, 20260816} {
		write := content.New(&tables.Text)
		g := &Game{
			Root: t.TempDir(), Data: tables, Write: write,
			RNG: core.NewRNG(seed), Seed: seed, Log: ui.NewLog(4),
			World: world.Generate(seed, write),
		}
		g.Player = rules.NewCharacter(g.RNG, "Bosk", model.ClassFighter)

		// Walk into every settlement and talk to every single person in it,
		// which is what a thorough player does and what the tour does literally.
		for idx, p := range g.World.POIs {
			if !p.Kind.Settlement() {
				continue
			}
			g.Local = world.BuildLocal(p, write)
			g.Walk.Place(p.Pos)
			for _, e := range g.Local.Entities {
				if e.Kind == world.ENPC {
					g.residentThread(e, idx)
				}
			}

			here := 0
			for _, th := range g.Threads.Threads {
				if th.HomePOI == idx && th.IsResident(&tables.Threads) {
					here++
				}
			}
			if here > 1 {
				t.Errorf("seed %d: %s has %d people all midway through telling you something",
					seed, p.Name, here)
			}
		}

		if got := g.runningResidents(); got > residentCap {
			t.Errorf("seed %d: %d resident stories running at once, cap is %d", seed, got, residentCap)
		}
	}
}

// Every combat effect extracted into the manifest has to be played by
// something, and everything played has to be in the manifest.
//
// Both directions, because they fail differently and neither is visible in
// play. Art nothing plays is a key in the audit's count, a file in the manifest
// and a decision somebody made that never reached the screen — the same waste
// as a thread skeleton that can never be cast. A key nothing extracted is a
// magenta box in the middle of a fight.
//
// The audit catches the second on its own. This catches the first, and it
// caught five: fire, ice, bolt, void and rock were extracted for their
// distinctiveness and then not reached by the kind table, which had one
// sensible default per kind and no way to say "this technique is different".
func TestEveryCombatEffectIsPlayedBySomething(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	reg := assetsys.New(root)

	played := map[string]bool{}
	for _, k := range vfxKeys(tables.Spells) {
		played[k] = true
		if !reg.Has(k) {
			t.Errorf("something plays %q, which is not in the manifest", k)
		}
	}

	// The manifest is the source of what was extracted, so the unused set is
	// read off it rather than from a second list that could drift.
	for _, k := range reg.Keys() {
		if !strings.HasPrefix(k, "vfx/") {
			continue
		}
		if !played[k] {
			t.Errorf("%q was extracted and nothing ever plays it", k)
		}
	}
}

// TestTheCompassPointsTheRightWay.
//
// The classic failure here is the Y axis. Screen and map coordinates both grow
// *downward*, so a destination with a larger Y is to the south — get that
// backwards and the arrow is a perfect mirror of the truth, which is worse than
// no arrow at all because the player will trust it.
func TestTheCompassPointsTheRightWay(t *testing.T) {
	for _, c := range []struct {
		dx, dy int
		want   int
		name   string
	}{
		{0, -10, dirN, "straight up the map"},
		{0, 10, dirS, "straight down the map"},
		{10, 0, dirE, "right"},
		{-10, 0, dirW, "left"},
		{10, -10, dirNE, "up and right"},
		{10, 10, dirSE, "down and right"},
		{-10, 10, dirSW, "down and left"},
		{-10, -10, dirNW, "up and left"},
		// Barely off-axis stays on-axis. An arrow that goes diagonal the moment
		// a destination is one tile off true north has stopped saying anything.
		{1, -20, dirN, "a shade east of north"},
		{-2, 20, dirS, "a shade west of south"},
		{30, 3, dirE, "a shade north of east"},
	} {
		if got := bearing(c.dx, c.dy); got != c.want {
			t.Errorf("%s (%d,%d) points %d, want %d", c.name, c.dx, c.dy, got, c.want)
		}
	}
}

// Every compass glyph has to be the same size, or an arrow drawn in one
// direction sits a pixel off from the same arrow in another and the whole
// corner of the status bar jitters as the player walks around a destination.
func TestEveryCompassGlyphIsTheSameSize(t *testing.T) {
	for dir, glyph := range compassGlyphs {
		if len(glyph) != 7 {
			t.Errorf("direction %d is %d rows tall, want 7", dir, len(glyph))
			continue
		}
		ink := 0
		for row, line := range glyph {
			if len(line) != 7 {
				t.Errorf("direction %d row %d is %d wide, want 7", dir, row, len(line))
			}
			for _, ch := range line {
				switch ch {
				case '#':
					ink++
				case '.':
				default:
					t.Errorf("direction %d row %d has a %q in it", dir, row, ch)
				}
			}
		}
		if ink < 8 {
			t.Errorf("direction %d is %d pixels of ink; that is not an arrow", dir, ink)
		}
	}
}

// TestTheShelfNeverGradesACharm.
//
// Every charm in the table gives with one hand and takes with the other, which
// is a deliberate design rule with a test of its own. Marking one green would
// be the interface contradicting the content: there is no better charm, only a
// different one, and a colour that says otherwise turns "which trade do I want"
// back into "which is the good one".
func TestTheShelfNeverGradesACharm(t *testing.T) {
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no data directory: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	buyer := &model.Character{Name: "Bosk"}
	for _, ch := range tables.Charms {
		if verdict, _ := shelfVerdict(buyer, ch); verdict != "" {
			t.Errorf("the counter grades %q as %q", ch.Name, verdict)
		}
	}
}

// TestTheShelfComparesAgainstWhatIsWorn, affix and empty slot included.
func TestTheShelfComparesAgainstWhatIsWorn(t *testing.T) {
	buyer := &model.Character{
		Weapon: model.Weapon{Name: "Old", Strike: 5,
			Affix: &model.Affix{Bonus: model.Bonus{Strike: 2}}},
		Armor: model.Armor{Name: "Coat", Defense: 3},
	}

	// Seven effective strike, so a bare 8 is one better and a bare 7 is level.
	for _, c := range []struct {
		strike int
		want   string
	}{{9, "+2"}, {8, "+1"}, {7, "="}, {4, "-3"}} {
		got, _ := shelfVerdict(buyer, model.Weapon{Strike: c.strike})
		if got != c.want {
			t.Errorf("a strike-%d weapon against seven reads %q, want %q", c.strike, got, c.want)
		}
	}
	if got, _ := shelfVerdict(buyer, model.Armor{Defense: 3}); got != "=" {
		t.Errorf("the same coat reads %q, want =", got)
	}

	// An empty arm is nothing, so the first shield is the upgrade it is rather
	// than no change at all.
	if got, _ := shelfVerdict(buyer, model.Shield{Defense: 4}); got != "+4" {
		t.Errorf("the first shield onto an empty arm reads %q, want +4", got)
	}
	buyer.Shield = model.Shield{Name: "Board", Defense: 4}
	if got, _ := shelfVerdict(buyer, model.Shield{Defense: 4}); got != "=" {
		t.Errorf("the same shield reads %q, want =", got)
	}

	// And nothing at all for a buyer who is not there, since the shop builds
	// its rows before it has necessarily decided who is at the counter.
	if got, _ := shelfVerdict(nil, model.Weapon{Strike: 9}); got != "" {
		t.Errorf("a nil buyer got the verdict %q", got)
	}
}
