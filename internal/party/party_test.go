package party_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
)

func TestMembersPutTheHeroFirst(t *testing.T) {
	hero := &model.Character{Name: "Bosk", HP: 10, MaxHP: 10}
	one := &model.Character{Name: "Nessa", Ally: true, HP: 8, MaxHP: 8}
	two := &model.Character{Name: "Gil", Ally: true, HP: 0, MaxHP: 9}

	got := party.Members(hero, []*model.Character{one, two})
	if len(got) != 3 || got[0] != hero || got[1] != one || got[2] != two {
		t.Fatalf("company came back as %v, want hero then hirelings in order", names(got))
	}
	// Turn order, the panel and the experience split all read this, so a member
	// on zero hit points must drop out of the living list but stay in the roster.
	if living := party.Living(got); len(living) != 2 || living[1] != one {
		t.Fatalf("living company is %v, want the hero and Nessa", names(living))
	}
	// A run with no character yet must not produce a one-entry slice of nil.
	if got := party.Members(nil, nil); len(got) != 0 {
		t.Errorf("an empty company came back with %d members", len(got))
	}
}

func TestFullStopsAtTheCap(t *testing.T) {
	for allies := 0; allies < MaxAllies; allies++ {
		if party.Full(allies) {
			t.Fatalf("a company of %d hirelings was full, below the cap of %d", allies, party.MaxSize)
		}
	}
	if !party.Full(MaxAllies) {
		t.Fatalf("a company of %d hirelings is still not full", MaxAllies)
	}
}

// MaxAllies is how many hirelings fit beside the hero.
const MaxAllies = party.MaxSize - 1

// A bigger company must draw a bigger crowd, but never one the battle screen
// cannot lay out.
func TestEncounterSizeScalesAndStaysDrawable(t *testing.T) {
	for allies := 0; allies <= MaxAllies; allies++ {
		g := core.NewRNG(1994)

		var total int
		const rolls = 500
		for i := 0; i < rolls; i++ {
			n := party.EncounterSize(g, 1+g.Intn(2), allies)
			if n < 1 || n > party.MaxFoes {
				t.Fatalf("%d allies: rolled an encounter of %d, outside 1..%d", allies, n, party.MaxFoes)
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

func TestRestPutsEveryoneBackToFull(t *testing.T) {
	members := []*model.Character{
		{Name: "Bosk", HP: 2, MaxHP: 30, Psyche: 0, MaxPsyche: 5},
		{Name: "Nessa", HP: 0, MaxHP: 18, Psyche: 3, MaxPsyche: 12},
	}
	party.Rest(members)
	for _, c := range members {
		if c.HP != c.MaxHP || c.Psyche != c.MaxPsyche {
			t.Errorf("%s rested to %d/%d hit points and %d/%d psyche",
				c.Name, c.HP, c.MaxHP, c.Psyche, c.MaxPsyche)
		}
	}
}

// Two people answering to the same name make the panel and the transcript
// unreadable, and the name pool is shallow enough that it happens.
func TestUniqueNameDisambiguates(t *testing.T) {
	members := []*model.Character{{Name: "Bosk"}}
	if got := party.UniqueName(members, "Nessa"); got != "Nessa" {
		t.Errorf("an unused name came back as %q", got)
	}

	// Fill the company with collisions and check each one gets its own name.
	seen := map[string]bool{"Bosk": true}
	for i := 0; i < 4; i++ {
		got := party.UniqueName(members, "Bosk")
		if seen[got] {
			t.Fatalf("round %d produced %q, which is already taken", i, got)
		}
		seen[got] = true
		members = append(members, &model.Character{Name: got})
	}
	// The suffix has to stay short: the panel gives a name eighty pixels, and
	// the whole reason for a numeral is that "the Lesser" got truncated.
	for name := range seen {
		if len(name) > len("Bosk")+4 {
			t.Errorf("%q is too long to fit the party panel", name)
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
