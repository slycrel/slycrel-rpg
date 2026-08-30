package game

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// restore puts the bindings back after a test has moved them. They are package
// state, which is what the input helpers have always been, so a test that
// rebinds Down and leaves has rebound Down for every test after it.
func restore(t *testing.T) {
	t.Helper()
	t.Cleanup(restoreBindings)
}

// TestTheKeyNameTableIsReal. The names are derived by walking Ebitengine's key
// enum rather than typed out, and the point of doing it that way is that a
// name can never be one the engine does not have. This is that claim: every
// key the game ships bound can be written down and read back as itself.
func TestTheKeyNameTableIsReal(t *testing.T) {
	for name, keys := range defaultKeys {
		for _, k := range keys {
			s := k.String()
			if s == "" {
				t.Errorf("%s is bound to key %d, which has no name", name, k)
				continue
			}
			if back, ok := keysByName[strings.ToLower(s)]; !ok || back != k {
				t.Errorf("%s: %q read back as %v, want %v", name, s, back, k)
			}
		}
	}
	if len(keyNames()) < 50 {
		t.Errorf("the key table has %d entries, which is too few to be the whole keyboard",
			len(keyNames()))
	}
}

// TestNoTwoActionsShareAKey out of the box. This is the invariant the whole
// rebinding screen is built to protect, and the defaults are the one binding
// nobody chose — so if they collide, every player starts with the bug.
func TestNoTwoActionsShareAKey(t *testing.T) {
	seen := map[ebiten.Key]string{}
	for _, a := range actions() {
		for _, k := range *a.keys {
			if other, ok := seen[k]; ok {
				t.Errorf("%s is both %s and %s", k.String(), other, a.name)
			}
			seen[k] = a.name
		}
	}
}

// TestABindingSurvivesTheFile. Bindings are stored as names and applied as
// keys, so the round trip is the feature.
func TestABindingSurvivesTheFile(t *testing.T) {
	restore(t)

	downKeys = []ebiten.Key{ebiten.KeyN}
	stored := storedBindings()
	if len(stored) != 1 || len(stored["Down"]) != 1 || stored["Down"][0] != "N" {
		t.Fatalf("stored as %v", stored)
	}

	restoreBindings()
	applyBindings(stored)
	if len(downKeys) != 1 || downKeys[0] != ebiten.KeyN {
		t.Errorf("came back as %v", downKeys)
	}
	// And everything untouched is still itself, which is the half that says
	// the file holds changes rather than a snapshot.
	if !sameKeys(upKeys, defaultKeys["Up"]) {
		t.Errorf("Up moved to %v without being asked", upKeys)
	}
}

// TestOnlyChangesAreWrittenDown. A player who changed one key should have one
// line in their settings, not a transcript of the keyboard — and it is what
// makes a future change to the default bindings reach the people who never
// overrode them.
func TestOnlyChangesAreWrittenDown(t *testing.T) {
	restore(t)
	restoreBindings()
	if got := storedBindings(); got != nil {
		t.Errorf("untouched bindings wrote %v, want nothing", got)
	}
}

// TestAnImpossibleBindingKeepsTheDefault. A settings file is not a place to
// lose the ability to walk: a key name from a different version of the engine,
// or an empty list, has to read as "they did not set that".
func TestAnImpossibleBindingKeepsTheDefault(t *testing.T) {
	restore(t)
	for _, stored := range []map[string][]string{
		{"Down": {"NoSuchKey"}},
		{"Down": {}},
		{"Down": nil},
		nil,
	} {
		downKeys = []ebiten.Key{ebiten.KeyN}
		applyBindings(stored)
		if !sameKeys(downKeys, defaultKeys["Down"]) {
			t.Errorf("%v left Down as %v, want the default %v",
				stored, downKeys, defaultKeys["Down"])
		}
	}
}

// TestTheCameraCannotBeUnbound. Dumping the framebuffer is how anything in
// this game gets looked at — it is in CLAUDE.md as the only way, since screen
// capture is blocked on the machine this is built on — so a player who binds
// Cancel to backslash would quietly disable the only camera.
func TestTheCameraCannotBeUnbound(t *testing.T) {
	for _, k := range []ebiten.Key{ebiten.KeyBackslash, ebiten.KeyF12, ebiten.KeyShift} {
		if ok, why := bindable(k); ok {
			t.Errorf("%s can be bound, and should not be", k.String())
		} else if why == "" {
			t.Errorf("%s was refused without saying why", k.String())
		}
	}
	// And an ordinary letter still can, or the guard is refusing everything.
	if ok, _ := bindable(ebiten.KeyN); !ok {
		t.Error("N cannot be bound, which would make the screen useless")
	}
}

// TestACollisionIsFoundBeforeItHappens. Two actions on one key is a menu that
// moves and chooses at once, and the way a player finds that out is by no
// longer being able to leave the screen where they did it.
func TestACollisionIsFoundBeforeItHappens(t *testing.T) {
	restore(t)
	if got := boundElsewhere(ebiten.KeyS, "Confirm"); got != "Down" {
		t.Errorf("S reads as belonging to %q, want Down", got)
	}
	// Its own action does not count as elsewhere, or nothing could be rebound
	// to a key it already has.
	if got := boundElsewhere(ebiten.KeyS, "Down"); got != "" {
		t.Errorf("S collides with %q when rebinding Down onto itself", got)
	}
}

// TestThePaceLadderIsWhatItSaysItIs. The default has to be the middle rung and
// the fast rung has to be the value that shipped, because somebody who liked
// the old speed should be able to keep it exactly rather than get a new number
// that is nearly the same.
func TestThePaceLadderIsWhatItSaysItIs(t *testing.T) {
	t.Cleanup(func() { stepTicks = paceSteady })

	if paceTicks[0] != 30 {
		t.Errorf("the fast rung is %d, and the pace that shipped for months was 30", paceTicks[0])
	}
	if len(paceTicks) != len(paceNames) {
		t.Fatalf("%d rungs and %d names", len(paceTicks), len(paceNames))
	}
	for i := 1; i < len(paceTicks); i++ {
		if paceTicks[i] <= paceTicks[i-1] {
			t.Errorf("rung %d (%d) is not slower than rung %d (%d)",
				i, paceTicks[i], i-1, paceTicks[i-1])
		}
	}
	// A settings file written before this existed says zero, which has to read
	// as the middle rather than as a combat step of no time at all.
	applyPace(0)
	if stepTicks != paceSteady {
		t.Errorf("an unset pace came out as %d, want the middle rung %d", stepTicks, paceSteady)
	}
	// And the ends clamp rather than wrapping, or holding a key overshoots the
	// setting into its opposite.
	applyPace(paceFast)
	setPace(-1)
	if stepTicks != paceFast {
		t.Errorf("stepping below the fast end gave %d", stepTicks)
	}
	applyPace(paceSlow)
	setPace(1)
	if stepTicks != paceSlow {
		t.Errorf("stepping past the slow end gave %d", stepTicks)
	}
}

// TestEverySettingsRowDoesSomething, which is the pause menu's lesson applied
// before it can be learned twice. This screen dispatches on a tag in Data
// rather than on a row number, and the failure mode of that is quieter: a row
// with no tag falls through the type switch and simply does nothing, with
// nothing on screen to say so.
func TestEverySettingsRowDoesSomething(t *testing.T) {
	restore(t)
	g := &Game{Log: ui.NewLog(4)}
	s := &settingsScene{}
	s.refresh(g)

	if len(s.menu.Items) == 0 {
		t.Fatal("the settings screen has no rows at all")
	}
	keyed := 0
	for i, it := range s.menu.Items {
		if it.Header {
			continue
		}
		switch it.Data.(type) {
		case paceRow, soundRow, defaultsRow:
		case keyRow:
			keyed++
		default:
			t.Errorf("row %d is %q, which nothing acts on", i, it.Label)
		}
	}
	if keyed != len(actions()) {
		t.Errorf("the screen offers %d key rows for %d actions", keyed, len(actions()))
	}
	// The cursor must not start on a heading, which is the one thing Select
	// refuses and a plain index assignment does not.
	if it, ok := s.menu.Selected(); !ok || it.Header {
		t.Errorf("the cursor opens on %+v", it)
	}
}
