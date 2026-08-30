// Package prefs is what the player set, as opposed to what the character did.
//
// It exists because there were about to be three of these. The audio bank had
// been quietly owning saves/settings.json since sound landed — reading it,
// writing it, and knowing where the saves directory is, none of which is
// anything to do with playing a sound. Adding combat pace and key bindings
// beside it would have meant either a second file describing the same thing or
// a second writer on the same one, and two writers on one file is one of them
// losing.
//
// So the file has an owner now, and it is not a subsystem. Nothing here
// imports Ebitengine: a binding is stored as the *name* of a key rather than
// its number, which is the only form that survives a version bump of the
// engine and the only form a person can read in a text editor.
package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DefaultVolume is where the master volume starts, and where it returns to if
// the file says something impossible.
const DefaultVolume = 0.7

// Prefs is the whole of what a player can set.
//
// Every field's zero value is "they never touched it", which is the only
// answer a file written before that field existed can give. Volume is the one
// exception and it predates this package: a zero there is indistinguishable
// from silence deliberately chosen, so Muted carries that and a zero volume is
// read as unset.
type Prefs struct {
	Muted  bool    `json:"muted"`
	Volume float64 `json:"volume"`

	// Pace is how many ticks a combat step holds before the next one plays.
	// Zero means unset, which is the middle setting rather than instant.
	Pace int `json:"pace,omitempty"`

	// Keys maps an action to the key names bound to it. Absent, or an action
	// absent from it, means the built-in binding for that action — so a file
	// written before rebinding existed rebinds nothing, and a player who only
	// changed one thing has one line in their settings.
	Keys map[string][]string `json:"keys,omitempty"`
}

// Path is where the preferences live: beside the saves rather than in them,
// because they describe the player's desk and not their character.
func Path(root string) string { return filepath.Join(root, "saves", "settings.json") }

// Load reads the preferences, falling back to the defaults for anything the
// file does not say. A missing or malformed file is not an error worth
// reporting to somebody who wanted to play a game.
func Load(root string) *Prefs {
	p := &Prefs{Volume: DefaultVolume}
	data, err := os.ReadFile(Path(root))
	if err != nil {
		return p
	}
	if err := json.Unmarshal(data, p); err != nil {
		return &Prefs{Volume: DefaultVolume}
	}
	if p.Volume <= 0 || p.Volume > 1 {
		p.Volume = DefaultVolume
	}
	return p
}

// Save writes the preferences back. Errors are swallowed for the same reason
// Load's are: a read-only disk should cost somebody their volume setting, not
// their evening.
func (p *Prefs) Save(root string) {
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(Path(root), data, 0o644)
}
