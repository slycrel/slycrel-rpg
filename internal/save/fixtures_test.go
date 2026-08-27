package save_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// The fixtures under saves/fixtures are hand-curated runs, each parked in a
// state that is otherwise a chore to reach: a full company, somebody on the
// floor, a party standing inside a location, and a file written before the
// party existed at all.
//
// They are a regression net that needs no display. A save is the world seed
// plus what the player changed, so loading one asserts that the seed still
// generates the same continent, that the content it names still exists, and
// that the format has not quietly stopped being readable — the three ways a
// save silently rots. They double as playtest starting points via -load.

// writerFor returns the real content writer for regenerating a continent.
//
// A stub namer will not do, and this is the whole reason the fixtures earn
// their keep: location names are drawn from the same generator that places the
// locations, so a namer consuming no randomness produces a *different*
// continent. The first fixture generated that way was refused by the game with
// "the save predates a change to world generation" — from a stub, not a change.
// Anything checking a save against its world has to regenerate the world the
// game would.
func writerFor(t *testing.T) *content.Writer {
	t.Helper()
	return content.New(&tablesFor(t).Text)
}

func tablesFor(t *testing.T) *gamedata.Tables {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no repository root: %v", err)
	}
	tables, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	return tables
}

func fixtures(t *testing.T) map[string]*save.File {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Skipf("no repository root to read fixtures from: %v", err)
	}
	dir := filepath.Join(root, "saves", "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	out := map[string]*save.File{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		f, err := save.Read(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Errorf("%s no longer loads: %v", e.Name(), err)
			continue
		}
		out[strings.TrimSuffix(e.Name(), ".json")] = f
	}
	if len(out) == 0 {
		t.Fatal("no fixtures found; the regression net is empty")
	}
	return out
}

// The continent is not stored — it is regenerated from the seed — so a change
// to world generation silently invalidates every save anybody has. The loader
// refuses a file whose location count no longer matches, and this is what
// notices before a player does.
func TestFixturesStillMatchTheWorldTheyWereSavedIn(t *testing.T) {
	for name, f := range fixtures(t) {
		m := world.Generate(f.Seed, writerFor(t))
		if len(f.POIs) != len(m.POIs) {
			t.Errorf("%s: saved %d locations, seed %d now generates %d — "+
				"world generation changed under the fixtures",
				name, len(f.POIs), f.Seed, len(m.POIs))
		}
		if !m.Walkable(f.At.X, f.At.Y) {
			t.Errorf("%s: the player is standing at %v, which is no longer walkable", name, f.At)
		}
		if f.Inside != nil && (f.Inside.POI < 0 || f.Inside.POI >= len(m.POIs)) {
			t.Errorf("%s: saved inside location %d, of %d", name, f.Inside.POI, len(m.POIs))
		}
		// The fog is a packed bitset sized to the map; a resize would silently
		// shift every explored tile.
		if got := len(save.UnpackFog(f.Fog, len(m.Explored))); got != len(m.Explored) {
			t.Errorf("%s: fog unpacked to %d tiles, want %d", name, got, len(m.Explored))
		}
	}
}

// A save names content by string. Renaming an item or a monster orphans every
// file that mentions it, and nothing else in the suite would notice.
func TestFixturesOnlyNameContentThatExists(t *testing.T) {
	tables := tablesFor(t)
	for name, f := range fixtures(t) {
		for _, c := range append([]*model.Character{f.Player}, f.Allies...) {
			for _, it := range c.Bag {
				if _, ok := tables.Item(it.Name); !ok {
					t.Errorf("%s: %s is carrying %q, which no longer exists", name, c.Name, it.Name)
				}
			}
		}
		for _, q := range f.Quests {
			if q == nil {
				t.Errorf("%s: a nil quest survived the round trip", name)
				continue
			}
			if q.Item != "" {
				if _, ok := tables.Item(q.Item); !ok {
					t.Errorf("%s: a quest asks for %q, which no longer exists", name, q.Item)
				}
			}
			if q.MonsterID != "" {
				if _, ok := tables.ByID[q.MonsterID]; !ok {
					t.Errorf("%s: a quest names monster %q, which no longer exists", name, q.MonsterID)
				}
			}
			// Every quest points at somewhere; an index past the end of the
			// location list is how a quest ends up naming nothing at all.
			if q.GiverPOI < 0 || q.TargetPOI < 0 {
				t.Errorf("%s: a quest points at location %d / %d", name, q.GiverPOI, q.TargetPOI)
			}
		}
		for _, th := range f.Threads {
			if th == nil {
				t.Errorf("%s: a nil thread survived the round trip", name)
				continue
			}
			if _, ok := tables.Threads.Get(th.Skeleton); !ok {
				t.Errorf("%s: a thread was cast from skeleton %q, which no longer exists",
					name, th.Skeleton)
			}
			if th.MonsterID != "" {
				if _, ok := tables.ByID[th.MonsterID]; !ok {
					t.Errorf("%s: a thread names monster %q, which no longer exists",
						name, th.MonsterID)
				}
			}
			// A thread is keyed to its owner by name, so a thread whose owner
			// is not in the company is one that can never advance and never be
			// resolved: it would sit in the journal for the rest of the run.
			owned := false
			for _, c := range f.Allies {
				if c.Name == th.Owner {
					owned = true
				}
			}
			if !owned {
				t.Errorf("%s: %q belongs to %q, who is not in the company",
					name, th.Title, th.Owner)
			}
		}
		for _, sg := range f.Sagas.Sagas {
			if sg == nil {
				t.Errorf("%s: a nil saga survived the round trip", name)
				continue
			}
			sk, ok := tables.Sagas.Get(sg.Skeleton)
			if !ok {
				t.Errorf("%s: a saga was cast from skeleton %q, which no longer exists",
					name, sg.Skeleton)
				continue
			}
			if sg.MonsterID != "" {
				if _, ok := tables.ByID[sg.MonsterID]; !ok {
					t.Errorf("%s: a saga names monster %q, which no longer exists",
						name, sg.MonsterID)
				}
			}
			// One place per leg, or a leg points at nothing and the story stops
			// where it stands.
			if len(sg.Places) != len(sk.Legs) || len(sg.PlaceNames) != len(sk.Legs) {
				t.Errorf("%s: %q has %d legs but %d places and %d names",
					name, sg.Title, len(sk.Legs), len(sg.Places), len(sg.PlaceNames))
			}
			// A leg counter past the end is the saga equivalent of a thread
			// whose owner has left: nothing can ever advance it, and it sits at
			// the top of the journal for the rest of the run.
			if sg.At < 0 || (sg.State == "open" && sg.At >= len(sk.Legs)) {
				t.Errorf("%s: %q is open at leg %d of %d",
					name, sg.Title, sg.At, len(sk.Legs))
			}
		}
	}
}

// The invariants a run must satisfy however it got into that state. These are
// the things that would let a corrupt or hand-edited save through the loader
// and produce nonsense several screens later.
func TestFixturesHoldTheRunInvariants(t *testing.T) {
	for name, f := range fixtures(t) {
		p := f.Player
		if p == nil {
			t.Errorf("%s has no character", name)
			continue
		}
		if p.Level < 1 || p.MaxHP < 1 || p.HP > p.MaxHP || p.Psyche > p.MaxPsyche {
			t.Errorf("%s: hero is L%d at %d/%d hit points and %d/%d psyche",
				name, p.Level, p.HP, p.MaxHP, p.Psyche, p.MaxPsyche)
		}
		if p.Ally {
			t.Errorf("%s: the hero is marked as a hireling", name)
		}

		if len(f.Allies)+1 > party.MaxSize {
			t.Errorf("%s: %d in the company, over the cap of %d", name, len(f.Allies)+1, party.MaxSize)
		}
		for _, c := range f.Allies {
			switch {
			case !c.Ally:
				t.Errorf("%s: %s is in the company but not marked a hireling", name, c.Name)
			case c.Coins != 0:
				// The purse stays the hero's. A companion holding coin means
				// money that vanishes the moment they are let go — supplies do
				// not, because dismissal hands those back.
				t.Errorf("%s: %s is carrying %d coins", name, c.Name, c.Coins)
			case c.HP > c.MaxHP || c.Psyche > c.MaxPsyche:
				t.Errorf("%s: %s is at %d/%d hit points", name, c.Name, c.HP, c.MaxHP)
			case c.Blood != "":
				if _, ok := model.LineageOf(c.Blood); !ok {
					t.Errorf("%s: %s is part %q, which is not a lineage", name, c.Name, c.Blood)
				}
			}
			// Conditions are fight-scoped and must never reach a file.
			if len(c.Active) != 0 {
				t.Errorf("%s: %s carries %d conditions out of a save", name, c.Name, len(c.Active))
			}
		}
		if len(p.Active) != 0 {
			t.Errorf("%s: the hero carries %d conditions out of a save", name, len(p.Active))
		}

		// Everybody in the file is wearing something their own class could put
		// on. A fixture in a state the game cannot produce is worse than no
		// fixture, because it is a starting point for a playtest of a game
		// nobody is playing — and this caught a real one: the affixed-weapon
		// fixture took the first affixable tier-four row in the table and handed
		// it over, which since weapons have lanes is a dagger going to a Fighter
		// and a two-hander going to somebody already holding a shield.
		for _, c := range append([]*model.Character{p}, f.Allies...) {
			if !model.CanWield(c.Class, c.Weapon) {
				t.Errorf("%s: %s is a %s holding %q", name, c.Name, c.Class, c.Weapon.Name)
			}
			if !model.CanWear(c.Class, c.Armor) {
				t.Errorf("%s: %s is a %s wearing %q", name, c.Name, c.Class, c.Armor.Name)
			}
			if c.Shield.Worn() {
				if !model.CanHoldShield(c.Class, c.Shield) {
					t.Errorf("%s: %s is a %s holding %q", name, c.Name, c.Class, c.Shield.Name)
				}
				if c.Weapon.TwoHanded() {
					t.Errorf("%s: %s has %q on the arm %q is already using",
						name, c.Name, c.Shield.Name, c.Weapon.Name)
				}
			}
			for _, g := range c.Carried {
				if ok, why := c.CanUse(g); !ok {
					t.Errorf("%s: %s is carrying %q, which they can never put on (%s)",
						name, c.Name, g.Titled(), why)
				}
			}
		}
	}
}

// The fixtures are only a net if they actually cover the awkward states. This
// asserts the set has not been quietly reduced to five copies of "level one,
// standing in a field".
func TestFixturesCoverTheStatesWorthCovering(t *testing.T) {
	all := fixtures(t)

	var haveOld, haveFull, haveFallen, haveInside, haveSolo, haveLineage bool
	var haveCaster bool
	var haveAffix, haveSidearms, haveThreadUnderway, haveCompanyNoThreads bool
	var haveCarried, haveClock, haveSagaUnderway, haveTrack, haveLastSpell bool
	for _, f := range all {
		// The four fields the format grew after the backstories. None of them
		// was covered by any fixture until somebody went looking, which is the
		// shape of hole this test exists to refuse: the net was complete for
		// every field that existed when it was written, and silent about every
		// field added since.
		if f.Clock.Step > 0 {
			haveClock = true
		}
		if f.Track.On {
			haveTrack = true
		}
		if f.LastSpell != "" {
			haveLastSpell = true
		}
		for _, sg := range f.Sagas.Sagas {
			// Partway through, for the same reason a thread has to be:
			// everything interesting about a long story is in the middle of it,
			// and a freshly cast one exercises almost none of the fields.
			if sg != nil && sg.At > 0 {
				haveSagaUnderway = true
			}
		}
		// Equipment in the pack rather than on the body is newer than most of
		// the format, and it is the half a round trip can silently drop.
		if len(f.Player.Carried) > 0 {
			haveCarried = true
		}
		// A company from before backstories existed, which is what makes the
		// loader cast them on the way in rather than only at the hiring.
		if len(f.Allies) > 0 && len(f.Threads) == 0 {
			haveCompanyNoThreads = true
		}
		for _, th := range f.Threads {
			// Partway through, rather than freshly cast. Everything interesting
			// about a thread is in the middle of it.
			if th != nil && th.At > 0 {
				haveThreadUnderway = true
			}
		}
		// An affix hangs off a pointer, and a pointer is what comes back nil.
		if f.Player.Weapon.Affix != nil || f.Player.Armor.Affix != nil {
			haveAffix = true
		}
		if f.Player.Shield.Worn() && f.Player.Charm.Worn() {
			haveSidearms = true
		}
		if f.Version < save.Version {
			haveOld = true
		}
		if len(f.Allies)+1 == party.MaxSize {
			haveFull = true
		}
		if len(f.Allies) == 0 {
			haveSolo = true
		}
		if f.Inside != nil {
			haveInside = true
		}
		for _, c := range f.Allies {
			if !c.Alive() {
				haveFallen = true
			}
			if c.Blood != "" {
				haveLineage = true
			}
		}
		// The caster half of the equipment tables — a focus weapon, cloth, and
		// a talisman on the off arm — is three fields the save format grew and
		// a whole class the net had never stood on. Every fixture was a Fighter
		// until somebody went looking, which is the same hole this test was
		// written to refuse in the first place.
		if f.Player.Casting() && f.Player.Shield.Barrier() {
			haveCaster = true
		}
	}

	for _, c := range []struct {
		got  bool
		want string
	}{
		{haveOld, "a save in an older format"},
		{haveSolo, "a run with nobody hired"},
		{haveFull, "a company at the party cap"},
		{haveFallen, "a companion on the floor"},
		{haveLineage, "a part-monster hireling"},
		{haveInside, "a party standing inside a location"},
		{haveSidearms, "a character with a shield and a charm"},
		{haveAffix, "a piece of gear carrying an affix"},
		{haveThreadUnderway, "a companion partway through their backstory"},
		{haveCompanyNoThreads, "a company saved before backstories existed"},
		{haveCarried, "equipment carried rather than worn"},
		{haveClock, "a run saved at a time of day other than the first dawn"},
		{haveSagaUnderway, "a long story partway through"},
		{haveTrack, "a followed destination"},
		{haveLastSpell, "a remembered technique"},
		{haveCaster, "a caster with a rod and a talisman"},
	} {
		if !c.got {
			t.Errorf("no fixture covers %s", c.want)
		}
	}
}
