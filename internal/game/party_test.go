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
