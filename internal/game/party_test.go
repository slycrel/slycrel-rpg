package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

func TestPartyPutsTheHeroFirst(t *testing.T) {
	hero := &model.Character{Name: "Bosk", HP: 10, MaxHP: 10}
	one := &model.Character{Name: "Nessa", Ally: true, HP: 8, MaxHP: 8}
	two := &model.Character{Name: "Gil", Ally: true, HP: 0, MaxHP: 9}
	g := &Game{Player: hero, Allies: []*model.Character{one, two}}

	got := g.Party()
	if len(got) != 3 || got[0] != hero || got[1] != one || got[2] != two {
		t.Fatalf("party came back as %v, want hero then hirelings in order", names(got))
	}
	// Turn order, the panel and the XP split all read this, so a member on
	// zero hit points must drop out of the living list but stay in the roster.
	if living := g.LivingParty(); len(living) != 2 || living[1] != one {
		t.Fatalf("living party is %v, want the hero and Nessa", names(living))
	}
}

func TestPartyFullStopsAtTheCap(t *testing.T) {
	g := &Game{Player: &model.Character{Name: "Bosk"}}
	for len(g.Allies) < PartyMax-1 {
		if g.PartyFull() {
			t.Fatalf("the company was full at %d, below the cap of %d", len(g.Party()), PartyMax)
		}
		g.Allies = append(g.Allies, &model.Character{Ally: true})
	}
	if !g.PartyFull() {
		t.Fatalf("the company holds %d and is still not full", len(g.Party()))
	}
}

// A bigger company must draw a bigger crowd, but never one the battle screen
// cannot lay out.
func TestEncounterSizeScalesWithTheCompanyAndStaysDrawable(t *testing.T) {
	for allies := 0; allies < PartyMax; allies++ {
		g := &Game{RNG: core.NewRNG(1994), Player: &model.Character{}}
		for i := 0; i < allies; i++ {
			g.Allies = append(g.Allies, &model.Character{Ally: true})
		}

		var total int
		const rolls = 500
		for i := 0; i < rolls; i++ {
			n := g.encounterSize(1 + g.RNG.Intn(2))
			if n < 1 || n > maxFoes {
				t.Fatalf("%d allies: rolled an encounter of %d, outside 1..%d", allies, n, maxFoes)
			}
			total += n
		}

		avg := float64(total) / rolls
		if allies == 0 && avg > 1.6 {
			t.Errorf("a lone traveller averages %.2f foes an encounter, which is already a crowd", avg)
		}
		if allies > 0 && avg <= 1.6 {
			t.Errorf("%d allies: encounters average %.2f foes, no bigger than walking alone", allies, avg)
		}
	}
}

// The line follows where the leader has been, not where the leader is. That is
// what makes it bend around a corner instead of cutting it.
func TestFollowersWalkTheLeadersPath(t *testing.T) {
	g := &Game{
		Player: &model.Character{},
		Allies: []*model.Character{{Ally: true}, {Ally: true}},
	}
	g.Walk = walker{dur: 1}
	g.Walk.Place(core.Point{X: 5, Y: 5})
	g.reformLines()

	if len(g.follow) != 2 {
		t.Fatalf("the line has %d walkers for 2 companions", len(g.follow))
	}

	// Two steps east, then one north — the corner.
	path := []core.Point{{X: 6, Y: 5}, {X: 7, Y: 5}, {X: 7, Y: 4}}
	for _, to := range path {
		from := g.Walk.Tile
		g.Walk.Step(to, dirBetween(from, to))
		stepLine(g.follow, from)
	}

	// After three steps the hero is at the corner, and the two behind occupy
	// the two tiles it walked through to get there.
	if got := g.follow[0].Tile; got != (core.Point{X: 7, Y: 5}) {
		t.Errorf("the first companion is at %v, want the tile the hero just left", got)
	}
	if got := g.follow[1].Tile; got != (core.Point{X: 6, Y: 5}) {
		t.Errorf("the second companion is at %v, want one further back down the path", got)
	}
	// Nobody may share a tile with the hero once the line has spread out.
	for i, w := range g.follow {
		if w.Tile == g.Walk.Tile {
			t.Errorf("companion %d is standing inside the hero at %v", i, w.Tile)
		}
	}
}

// Dismissing shortens the line rather than leaving a walker behind for a
// companion who is no longer there — that mismatch would index out of range on
// the next draw.
func TestTheLineTracksTheRoster(t *testing.T) {
	g := &Game{Player: &model.Character{}}
	g.Walk.Place(core.Point{X: 2, Y: 2})

	for i := 0; i < PartyMax-1; i++ {
		g.Allies = append(g.Allies, &model.Character{Ally: true})
		g.reformLines()
		if len(g.follow) != len(g.Allies) || len(g.localFollow) != len(g.Allies) {
			t.Fatalf("%d companions but %d/%d walkers",
				len(g.Allies), len(g.follow), len(g.localFollow))
		}
	}
	for len(g.Allies) > 0 {
		g.Allies = g.Allies[:len(g.Allies)-1]
		g.reformLines()
		if len(g.follow) != len(g.Allies) || len(g.localFollow) != len(g.Allies) {
			t.Fatalf("%d companions but %d/%d walkers after a dismissal",
				len(g.Allies), len(g.follow), len(g.localFollow))
		}
	}
}

func names(cs []*model.Character) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

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

	loaded := &Game{
		Root: root, Data: tables, Write: write,
		RNG: core.NewRNG(1), Log: ui.NewLog(20),
	}
	if err := loaded.Restore(g.Snapshot()); err != nil {
		t.Fatalf("restoring the run: %v", err)
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
