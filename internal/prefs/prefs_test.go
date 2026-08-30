package prefs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/prefs"
)

// TestAnOlderSettingsFileStillReads is the whole reason this package took over
// the file rather than opening a new one beside it.
//
// saves/settings.json has existed since sound landed and holds two keys. A
// player upgrading has one of those files, and the fields added since have to
// read as "they never set that" rather than as zero — which for pace would be
// a combat step of no ticks at all.
func TestAnOlderSettingsFileStillReads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "saves"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"muted": false, "volume": 0.45}`
	if err := os.WriteFile(prefs.Path(root), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	p := prefs.Load(root)
	if p.Volume != 0.45 || p.Muted {
		t.Errorf("audio came back as muted=%v volume=%v", p.Muted, p.Volume)
	}
	if p.Pace != 0 {
		t.Errorf("pace read as %d from a file that does not mention it", p.Pace)
	}
	if p.Keys != nil {
		t.Errorf("keys read as %v from a file that does not mention them", p.Keys)
	}
}

// A file that is missing, empty or corrupt has to produce a playable game
// rather than a silent one, because the volume is the one setting whose zero
// value is indistinguishable from the thing going wrong.
func TestNothingOnDiskIsStillAudible(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"missing", ""},
		{"empty", ``},
		{"corrupt", `{"volume": `},
		{"nonsense volume", `{"volume": 40}`},
		{"zero volume", `{"volume": 0}`},
	} {
		root := t.TempDir()
		if tc.body != "" || tc.name != "missing" {
			if err := os.MkdirAll(filepath.Join(root, "saves"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(prefs.Path(root), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := prefs.Load(root).Volume; got != prefs.DefaultVolume {
			t.Errorf("%s: volume came back as %v, want the default %v",
				tc.name, got, prefs.DefaultVolume)
		}
	}
}

// A round trip has to survive, or a setting is something the player does twice.
func TestSettingsSurviveWriting(t *testing.T) {
	root := t.TempDir()
	want := &prefs.Prefs{
		Muted: true, Volume: 0.3, Pace: 60,
		Keys: map[string][]string{"Down": {"S"}},
	}
	want.Save(root)

	got := prefs.Load(root)
	if got.Muted != want.Muted || got.Volume != want.Volume || got.Pace != want.Pace {
		t.Errorf("came back as %+v, wrote %+v", got, want)
	}
	if len(got.Keys["Down"]) != 1 || got.Keys["Down"][0] != "S" {
		t.Errorf("keys came back as %v", got.Keys)
	}
}

// Saving into a directory that does not exist yet is the first run, which is
// every run once.
func TestTheFirstSaveMakesItsOwnDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not", "there", "yet")
	(&prefs.Prefs{Volume: 0.5}).Save(root)
	if _, err := os.Stat(prefs.Path(root)); err != nil {
		t.Errorf("nothing was written: %v", err)
	}
}
