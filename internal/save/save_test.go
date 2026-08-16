package save_test

import (
	"os"
	"path/filepath"
	"testing"

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

func TestVersionMismatchIsRefused(t *testing.T) {
	root := t.TempDir()
	if err := save.Write(root, "1", sample()); err != nil {
		t.Fatal(err)
	}
	path := save.Path(root, "1")
	data, _ := os.ReadFile(path)
	// Bump the on-disk version to something this build does not know.
	bumped := []byte(replaceFirst(string(data), `"version": 1`, `"version": 99`))
	if err := os.WriteFile(path, bumped, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := save.Load(root, "1"); err == nil {
		t.Fatal("a v99 save loaded without complaint")
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
