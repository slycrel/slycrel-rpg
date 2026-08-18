package save_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/save"
)

func sample() *save.File {
	return &save.File{
		Seed: 1994,
		Player: &model.Character{
			Name: "Bosk", Epithet: "the Regrettable", Class: model.ClassFighter,
			Level: 4, HP: 21, MaxHP: 30, Psyche: 2, MaxPsyche: 5,
			Strength: 14, Dexterity: 7, Speed: 9, Coins: 312,
			TotalXP: 900,
			Weapon:  model.Weapon{Name: "Actual Sword", Strike: 6, Verb: "slash"},
			Armor:   model.Armor{Name: "Boiled Leather", Defense: 3, Verb: "deflects"},
			Bag:     []model.Item{{Name: "Small Beer", Kind: model.ItemHeal, Power: 8, Count: 3}},
		},
		At:     core.Point{X: 42, Y: 61},
		Facing: int(core.DirLeft),
		POIs: []save.POIState{
			{Discovered: true, Visited: true},
			{Discovered: true, Cleared: true, Used: []save.UsedEntity{
				{Kind: "chest", X: 12, Y: 8},
				{Kind: "foe", X: 20, Y: 15},
			}},
		},
		SinceFight: 3,
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	root := t.TempDir()
	in := sample()
	in.Fog = save.PackFog([]bool{true, false, true, true, false, false, false, false, true})

	if err := save.Write(root, "1", in); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := save.Load(root, "1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if out.Seed != in.Seed {
		t.Errorf("seed %d, want %d", out.Seed, in.Seed)
	}
	if out.Player.Name != "Bosk" || out.Player.Level != 4 || out.Player.Coins != 312 {
		t.Errorf("character came back wrong: %+v", out.Player)
	}
	if out.Player.Weapon.Name != "Actual Sword" || out.Player.Armor.Defense != 3 {
		t.Errorf("equipment came back wrong: %+v %+v", out.Player.Weapon, out.Player.Armor)
	}
	if len(out.Player.Bag) != 1 || out.Player.Bag[0].Count != 3 {
		t.Errorf("bag came back wrong: %+v", out.Player.Bag)
	}
	if out.At != in.At || out.Facing != in.Facing {
		t.Errorf("position came back as %v facing %d", out.At, out.Facing)
	}
	if len(out.POIs) != 2 || !out.POIs[1].Cleared || len(out.POIs[1].Used) != 2 {
		t.Errorf("location state came back wrong: %+v", out.POIs)
	}
	if out.POIs[1].Used[0] != (save.UsedEntity{Kind: "chest", X: 12, Y: 8}) {
		t.Errorf("used entity came back as %+v", out.POIs[1].Used[0])
	}
	if out.Version != save.Version {
		t.Errorf("version stamped as %d", out.Version)
	}
	if out.Saved == "" {
		t.Error("no timestamp written")
	}
}

func TestFogRoundTrip(t *testing.T) {
	g := core.NewRNG(9)
	const n = 19200 // the real map size
	in := make([]bool, n)
	for i := range in {
		in[i] = g.Chance(0.4)
	}
	out := save.UnpackFog(save.PackFog(in), n)
	if len(out) != n {
		t.Fatalf("unpacked %d flags, want %d", len(out), n)
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("fog differs at tile %d", i)
		}
	}
	// The whole point of packing: it must be far smaller than the JSON array.
	if packed := len(save.PackFog(in)); packed > n/4 {
		t.Errorf("packed fog is %d bytes for %d tiles; packing is not working", packed, n)
	}
}

func TestCorruptFogDoesNotFailTheLoad(t *testing.T) {
	// Losing map progress is acceptable; refusing to load the character is not.
	out := save.UnpackFog("this is not base64 @@@@", 100)
	if len(out) != 100 {
		t.Fatalf("got %d flags, want 100", len(out))
	}
	for i, v := range out {
		if v {
			t.Fatalf("tile %d came back explored from garbage input", i)
		}
	}
}

// writeAtVersion writes the sample save and then rewrites its version stamp, so
// the test does not depend on the current one being any particular number.
func writeAtVersion(t *testing.T, root string, version int) {
	t.Helper()
	if err := save.Write(root, "1", sample()); err != nil {
		t.Fatal(err)
	}
	path := save.Path(root, "1")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stamped := replaceFirst(string(data),
		fmt.Sprintf(`"version": %d`, save.Version),
		fmt.Sprintf(`"version": %d`, version))
	if stamped == string(data) {
		t.Fatalf("could not find the version stamp to rewrite in %s", path)
	}
	if err := os.WriteFile(path, []byte(stamped), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFutureVersionIsRefused(t *testing.T) {
	root := t.TempDir()
	writeAtVersion(t, root, save.Version+97)

	if _, err := save.Load(root, "1"); err == nil {
		t.Fatal("a save from the future loaded without complaint")
	}
}

// A save older than the format floor is a layout this build would misread, so
// it must be refused as loudly as one from the future.
func TestPrehistoricVersionIsRefused(t *testing.T) {
	root := t.TempDir()
	writeAtVersion(t, root, 0)

	if _, err := save.Load(root, "1"); err == nil {
		t.Fatal("a v0 save loaded without complaint")
	}
}

// The party arrived in v2, and a v1 save is still a complete description of a
// run without one. Refusing those would have thrown away every save anyone had
// for no reason.
func TestPreviousVersionStillLoads(t *testing.T) {
	root := t.TempDir()
	writeAtVersion(t, root, 1)

	f, err := save.Load(root, "1")
	if err != nil {
		t.Fatalf("a v1 save should still load: %v", err)
	}
	if len(f.Allies) != 0 {
		t.Fatalf("a v1 save described %d companions; it cannot describe any", len(f.Allies))
	}
}

func TestListSkipsUnreadableSaves(t *testing.T) {
	root := t.TempDir()
	if err := save.Write(root, "1", sample()); err != nil {
		t.Fatal(err)
	}
	// A corrupt slot must not take the whole load menu down with it.
	if err := os.WriteFile(filepath.Join(root, save.Dir, "2.json"), []byte("{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := save.List(root)
	if len(got) != 1 || got[0].Name != "1" {
		t.Fatalf("listed %d slots, want just slot 1: %+v", len(got), got)
	}
	// The fixture carries no Summary, so the menu line must be derived from
	// the character rather than coming back blank.
	if got[0].Summary == "" {
		t.Error("slot has no summary for the load menu")
	}
	if !contains(got[0].Summary, "Bosk") {
		t.Errorf("derived summary %q does not name the character", got[0].Summary)
	}

	// An explicit summary must win over the derived one.
	f := sample()
	f.Summary = "Bosk the Regrettable - Fighter L4 - the mire"
	if err := save.Write(root, "3", f); err != nil {
		t.Fatal(err)
	}
	for _, sl := range save.List(root) {
		if sl.Name == "3" && sl.Summary != f.Summary {
			t.Errorf("slot 3 summary = %q, want the explicit one", sl.Summary)
		}
	}
}

func TestCleanMakesSafeFilenames(t *testing.T) {
	for in, want := range map[string]string{
		"1":            "1",
		"My Save":      "my-save",
		"../../etc/pw": "etcpw",
		"":             "save",
		"!!!":          "save",
	} {
		if got := save.Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func contains(s, sub string) bool { return indexOf(s, sub) >= 0 }

func replaceFirst(s, old, new string) string {
	i := indexOf(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// A hireling is most of what is worth saving about a run that has one: their
// ancestry decides which techniques they know and what their stat line means,
// so losing it on load would quietly turn a part-demon into an ordinary person
// with suspiciously good numbers.
func TestCompanionsRoundTripWholesale(t *testing.T) {
	root := t.TempDir()
	f := sample()
	f.Allies = []*model.Character{
		{
			Name: "Ilsabet Dun", Class: model.ClassMage, Ally: true, Cut: 12,
			Blood: model.KindDemon, Level: 6, HP: 3, MaxHP: 28, Psyche: 4, MaxPsyche: 14,
			Strength: 11, Dexterity: 5, Speed: 8,
			Sprite: "hero/druid", Portrait: "portrait/female/f_08",
			Weapon: model.Weapon{Name: "Actual Sword", Strike: 6},
			Armor:  model.Armor{Name: "Studded Leather", Defense: 4},
		},
		{
			Name: "Onager Flint", Class: model.ClassFighter, Ally: true, Cut: 9,
			Level: 6, HP: 40, MaxHP: 40,
		},
	}
	if err := save.Write(root, "1", f); err != nil {
		t.Fatal(err)
	}

	got, err := save.Load(root, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Allies) != len(f.Allies) {
		t.Fatalf("saved %d companions, read back %d", len(f.Allies), len(got.Allies))
	}
	for i := range f.Allies {
		// reflect.DeepEqual rather than == : Character carries a bag slice, and
		// comparing field by field is how a newly added field gets forgotten.
		if !reflect.DeepEqual(got.Allies[i], f.Allies[i]) {
			t.Errorf("companion %d came back different:\n got %+v\nwant %+v",
				i, *got.Allies[i], *f.Allies[i])
		}
	}
	// An ordinary hireling must not acquire an ancestry on the way through.
	if got.Allies[1].Blood != "" {
		t.Errorf("an ordinary hireling came back as part-%s", got.Allies[1].Blood)
	}
}

// TestLatestForRunFindsThisCharacterAndNoOther.
//
// What a death offers back. Reaching for the autosave slot by name was wrong in
// two ordinary ways: a player who saved by hand more recently than they last
// slept would be handed the older checkpoint, and the autosave outlives the run
// that wrote it, so a fresh character could be offered somebody else's.
func TestLatestForRunFindsThisCharacterAndNoOther(t *testing.T) {
	root := t.TempDir()

	write := func(slot string, seed int64, name, epithet string, ago time.Duration) {
		t.Helper()
		f := &save.File{
			Version: save.Version,
			Seed:    seed,
			Saved:   time.Now().Add(-ago).Format(time.RFC3339),
			Player:  &model.Character{Name: name, Epithet: epithet, Class: model.ClassFighter},
		}
		if err := save.Write(root, slot, f); err != nil {
			t.Fatalf("writing %s: %v", slot, err)
		}
	}

	const seed = 1994
	write("autosave", seed, "Bosk", "the Regrettable", 2*time.Hour)
	write("1", seed, "Bosk", "the Regrettable", 10*time.Minute) // the deliberate one
	write("2", seed, "Ilsabet", "the Unasked", time.Minute)     // somebody else, newer
	write("3", 77, "Bosk", "the Regrettable", time.Second)      // same name, other run

	got, ok := save.LatestForRun(root, seed, "Bosk the Regrettable")
	if !ok {
		t.Fatal("no save found for a character who has three")
	}
	if got.Name != "1" {
		t.Errorf("offered slot %q, want the hand-made save in slot 1 — "+
			"the autosave is older, and the newer files belong to other runs", got.Name)
	}

	// And a character with nothing of their own gets nothing, rather than the
	// newest file on disk.
	if _, ok := save.LatestForRun(root, seed, "Nobody at All"); ok {
		t.Error("a character with no saves was offered one anyway")
	}
	if _, ok := save.LatestForRun(root, 4242, "Bosk the Regrettable"); ok {
		t.Error("a run on a different seed was offered another run's save")
	}
}
