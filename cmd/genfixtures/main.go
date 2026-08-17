// Command genfixtures rewrites the save fixtures under saves/fixtures.
//
// Run it when world generation changes. A save records one entry per point of
// interest and the loader refuses a file whose count no longer matches, which
// is a deliberate canary — so the sequence is: notice the fixtures failing,
// confirm the world change was intended, then run this.
//
//	go run ./cmd/genfixtures
//
// It does not touch v1-solo.json or v2-company.json. Those files' whole job is
// to be old saves, and regenerating them at the current version would quietly
// delete the only evidence that the loader still reads the earlier formats.
// v1 is a run from before the party existed; v2 is a company from before the
// backstories did, which is the one that exercises threads being cast on the
// way in rather than at the moment somebody was hired.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// writerFor returns the real content writer.
//
// A stub namer will not do. Location names are drawn from the same generator
// that places the locations, so a namer which consumes no randomness produces a
// different continent — and a fixture generated against that continent is
// refused by the game as "the save predates a change to world generation".
func writerFor(t *gamedata.Tables) *content.Writer { return content.New(&t.Text) }

func main() {
	root, err := gamedata.FindRoot()
	must(err)
	tables, err := gamedata.Load(root)
	must(err)

	write(root, "solo-fresh", build(tables, 1994, 1, 0, "", func(f *save.File) {
		f.Summary = "a level 1 hero, alone, one step out of the capital"
	}))

	write(root, "company", build(tables, 1994, 6, 1, "", func(f *save.File) {
		f.Summary = "a level 6 hero and one hireling, mid-run"
		f.POIs[3].Discovered, f.POIs[3].Visited = true, true
		f.POIs[3].Used = []save.UsedEntity{{Kind: "chest", X: 12, Y: 9}}
	}))

	write(root, "full-company", build(tables, 20260816, 11, 2, model.KindDemon, func(f *save.File) {
		f.Summary = "a level 11 hero at the party cap, one of them part demon"
		// Affixed gear, so that a save carrying one is exercised: the suffix
		// hangs off a pointer, and a pointer is what silently comes back nil.
		for _, w := range tables.Weapons {
			if w.Tier == 4 && model.Affixable(w.Name) {
				f.Player.Weapon = w
				break
			}
		}
		if a, ok := tables.PickAffix(core.NewRNG(4), 4); ok {
			f.Player.Weapon.Affix = &a
		}
		// One backstory already underway, so the set holds a company that is
		// partway through something rather than three people who have only just
		// been introduced.
		if len(f.Threads) > 0 {
			f.Threads[0].At = 1
			f.Threads[0].Have = 2
		}
	}))

	write(root, "battered", build(tables, 1994, 8, 2, model.KindOoze, func(f *save.File) {
		f.Summary = "a level 8 company on its last legs, one of them down"
		f.Player.HP = 3
		f.Player.Psyche = 0
		f.Allies[0].HP = 0 // the rescue and revive paths start here
		f.Allies[1].HP = 4
	}))

	write(root, "inside", build(tables, 1994, 4, 1, "", func(f *save.File) {
		f.Summary = "a level 4 company standing inside a location"
		for i, p := range f.POIs {
			_ = p
			f.POIs[i].Discovered = true
		}
		f.Inside = &save.Inside{POI: settlement(tables, 1994), At: core.Point{X: 10, Y: 12}, Facing: 0}
	}))

	fmt.Println("wrote", len(names), "fixtures")
}

var names []string

// build assembles a run: a hero of the given level, that many hirelings, and a
// POI list matching the continent the seed generates.
func build(t *gamedata.Tables, seed int64, level, allies int, blood model.MonsterKind, tweak func(*save.File)) *save.File {
	g := core.NewRNG(seed)
	m := world.Generate(seed, writerFor(t))

	hero := rules.BuildCharacter(g, model.ClassFighter, level)
	hero.Name, hero.Epithet = "Bosk", "the Regrettable"
	hero.Coins = 250
	t.Equip(hero)
	if it, ok := t.Item("Small Beer"); ok {
		it.Count = 3
		hero.AddItem(it)
	}
	if it, ok := t.Item("Smelling Salts, Militant"); ok {
		it.Count = 1
		hero.AddItem(it)
	}

	f := &save.File{
		Seed:   seed,
		Player: hero,
		At:     m.Start,
		Facing: int(core.DirDown),
		POIs:   make([]save.POIState, len(m.POIs)),
		Fog:    save.PackFog(m.Explored),
	}
	for i := 0; i < allies; i++ {
		b := model.MonsterKind("")
		if i == 0 {
			b = blood
		}
		c := rules.Recruit(g, fmt.Sprintf("Hireling %d", i+1), model.AllClasses[i%len(model.AllClasses)], b, level)
		t.Equip(c)
		c.Sprite, c.Portrait = "hero/druid", "portrait/female/f_08"
		f.Allies = append(f.Allies, c)
	}
	// Something in the pack that is not a consumable, so the save format's
	// carried-equipment list is exercised by the fixture net rather than only
	// by whatever a playtest happens to pick up.
	if ws, as := t.StockFor(gamedata.GearTierFor(level)); len(ws) > 0 && len(as) > 0 {
		w, a := ws[0], as[0]
		hero.Carry(model.Carried{Weapon: &w})
		hero.Carry(model.Carried{Armor: &a})
	}

	// A backstory apiece. The game would cast these on load anyway, but a
	// fixture that carries them is a fixture that can be halfway through one,
	// which is the state worth having a starting point for.
	var log thread.Log
	for _, c := range f.Allies {
		if th, ok := thread.Cast(g, &t.Threads, m, t, c, m.Start, log.IDs()); ok {
			log.Add(th)
		}
	}
	f.Threads = log.Threads

	if q, ok := quest.Generate(g, m, t, writerFor(t), 0, "Person"); ok {
		f.Quests = append(f.Quests, q)
	}
	tweak(f)
	return f
}

// settlement returns the index of the first settlement on a seed's continent.
func settlement(t *gamedata.Tables, seed int64) int {
	for i, p := range world.Generate(seed, writerFor(t)).POIs {
		if p.Kind.Settlement() {
			return i
		}
	}
	return 0
}

func write(root, name string, f *save.File) {
	names = append(names, name)
	path := filepath.Join(root, "saves", "fixtures", name+".json")
	f.Version = save.Version
	f.Saved = "2026-08-16T12:00:00Z" // fixed, so a fixture is not churn in every diff
	data, err := json.MarshalIndent(f, "", "  ")
	must(err)
	must(os.WriteFile(path, append(data, '\n'), 0o644))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
