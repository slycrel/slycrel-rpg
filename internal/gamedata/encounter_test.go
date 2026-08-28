package gamedata_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// A shape has to actually be the shape it says it is. The name reaches the
// player — the transcript opens with it — so a "pack" that turned out to be one
// creature would be the interface lying about the fight in the one line the
// player reads before choosing.
func TestAShapeIsTheShapeItClaims(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(77)
	biomes := []string{"plains", "forest", "swamp", "mountain", "dungeon", "oddity"}

	seen := map[gamedata.Shape]int{}
	for i := 0; i < 4000; i++ {
		biome := biomes[i%len(biomes)]
		level := 1 + g.Intn(14)
		enc := tables.PickEncounter(g, biome, level, 1+g.Intn(2))
		if len(enc.Monsters) == 0 {
			t.Fatalf("%s at level %d produced an empty encounter", biome, level)
		}
		seen[enc.Shape]++

		switch enc.Shape {
		case gamedata.ShapeBrute:
			if len(enc.Monsters) != 1 {
				t.Fatalf("a brute arrived with %d in it", len(enc.Monsters))
			}
		case gamedata.ShapePack:
			if len(enc.Monsters) < 3 {
				t.Fatalf("a pack of %d is a mixed fight with a different name",
					len(enc.Monsters))
			}
		case gamedata.ShapeEscort:
			if !enc.Monsters[0].Def.Magic {
				t.Fatalf("an escort is escorting %q, which does not attack with magic",
					enc.Monsters[0].Def.Name)
			}
			if len(enc.Monsters) < 2 {
				t.Fatal("an escort with nobody in front of it is a lone caster")
			}
		case gamedata.ShapeMismatch:
			if len(enc.Monsters) != 2 {
				t.Fatalf("a mismatch of %d", len(enc.Monsters))
			}
			a, b := enc.Monsters[0].Def, enc.Monsters[1].Def
			if a.Defense <= b.Defense || b.Ward <= a.Ward {
				t.Fatalf("%q (def %d ward %d) and %q (def %d ward %d) are not two "+
					"different problems", a.Name, a.Defense, a.Ward,
					b.Name, b.Defense, b.Ward)
			}
		}

		// The promise an encounter level makes holds whatever the shape is: no
		// member is drawn from more than a band above it. A shape that reached
		// higher up the roster would be the accidental overshoot poolFor exists
		// to stop, wearing a description.
		//
		// Except where the biome has nothing that low, which is the floor case
		// poolFor documents: a dungeon holds nothing under level three and
		// something still has to be sent. That is the roster being short, not
		// the shape overreaching.
		floor := 1 << 30
		for _, d := range tables.Monsters[biome] {
			if d.Level < floor {
				floor = d.Level
			}
		}
		ceiling := level + 1
		if floor > ceiling {
			ceiling = floor
		}
		for _, m := range enc.Monsters {
			if m.Def.Level > ceiling {
				t.Fatalf("%s at level %d in %s sent %q, a level-%d creature, "+
					"against a roster that starts at %d",
					enc.Shape, level, biome, m.Def.Name, m.Def.Level, floor)
			}
		}
	}

	// And every shape has to be reachable somewhere, or it is code nobody will
	// ever meet.
	for _, s := range []gamedata.Shape{gamedata.ShapeMixed, gamedata.ShapePack,
		gamedata.ShapeBrute, gamedata.ShapeEscort, gamedata.ShapeMismatch} {
		if seen[s] == 0 {
			t.Errorf("no roster in the game ever produces a %s", s)
		}
	}
	// Mixed stays the plurality: shapes are the thing you notice, and a game
	// where every fight is a special composition has no ordinary fight to
	// notice them against.
	if seen[gamedata.ShapeMixed]*2 < 4000 {
		t.Errorf("only %d of 4000 encounters were ordinary; shapes have stopped "+
			"being texture and become the norm", seen[gamedata.ShapeMixed])
	}
}

// An escort needs something that attacks with magic, and nothing does below the
// level the answer goes on sale. That rule is written once, in the monster
// table, and the shape reads it off the roster rather than repeating the number
// — so content and composition cannot drift apart.
func TestNothingEscortsBeforeMagicExists(t *testing.T) {
	tables := load(t)
	g := core.NewRNG(78)

	first := 1 << 30
	for _, defs := range tables.Monsters {
		for _, d := range defs {
			if d.Magic && d.Level < first {
				first = d.Level
			}
		}
	}
	if first == 1<<30 {
		t.Skip("nothing in the game attacks with magic")
	}

	for _, biome := range []string{"plains", "forest", "swamp", "oddity"} {
		for level := 1; level < first-1; level++ {
			for i := 0; i < 200; i++ {
				if enc := tables.PickEncounter(g, biome, level, 2); enc.Shape == gamedata.ShapeEscort {
					t.Fatalf("%s at level %d fielded an escort, but the first magical "+
						"attacker in the game is level %d", biome, level, first)
				}
			}
		}
	}
}

// The same seed has to produce the same fight. A shape rolled off the shared
// generator is fine — encounters are rolled from it already — but a shape that
// consumed a different amount of randomness depending on what it chose would
// make a seed stop reproducing the run, which is the one thing the whole
// generation model rests on.
func TestAnEncounterIsReproducible(t *testing.T) {
	tables := load(t)
	one := tables.PickEncounter(core.NewRNG(4242), "forest", 6, 2)
	two := tables.PickEncounter(core.NewRNG(4242), "forest", 6, 2)
	if one.Shape != two.Shape || len(one.Monsters) != len(two.Monsters) {
		t.Fatalf("same seed gave %s of %d and %s of %d",
			one.Shape, len(one.Monsters), two.Shape, len(two.Monsters))
	}
	for i := range one.Monsters {
		if one.Monsters[i].Name != two.Monsters[i].Name ||
			one.Monsters[i].MaxHP != two.Monsters[i].MaxHP {
			t.Fatalf("same seed gave %q at %d and %q at %d",
				one.Monsters[i].Name, one.Monsters[i].MaxHP,
				two.Monsters[i].Name, two.Monsters[i].MaxHP)
		}
	}
}

// A shape that only says "3 of them" has not told the player anything the
// portraits do not. The transcript's opening line is the whole interface for
// this feature.
func TestEveryShapeSaysWhatItIs(t *testing.T) {
	for _, s := range []gamedata.Shape{gamedata.ShapePack, gamedata.ShapeBrute,
		gamedata.ShapeEscort, gamedata.ShapeMismatch} {
		e := gamedata.Encounter{Shape: s, Monsters: []*model.Monster{{}, {}}}
		if e.Line() == "" {
			t.Errorf("a %s opens the transcript with nothing to distinguish it", s)
		}
	}
}
