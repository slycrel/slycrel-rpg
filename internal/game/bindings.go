package game

import (
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// Rebinding, which the input helpers in game.go were written to allow and
// nothing had yet asked for.
//
// The awkward fact this exists to fix is written down in CLAUDE.md: `S` is the
// down key, and so are `J`, `A`, `D`, `H`, `K` and `L`. WASD and the vi keys
// between them claim most of the alphabet's convenient letters, so every new
// hotkey is a negotiation with a table nobody chose. A player who never uses
// vi keys is carrying six of those collisions for nothing.

// action is one thing the player can do, in the order the settings screen
// lists it. The string is both the label and the key it is stored under, so a
// binding written to disk today is readable by a person and by a version of
// this game with the rows in a different order.
type action struct {
	name string
	keys *[]ebiten.Key
	// what the action is for, since "cancel" and "back" are the same idea and
	// only one of them is the word on the row.
	note string
}

// actions is every rebindable input. Deliberately not every key in the game:
// the letters that open screens (C, M, B, G, T, R) are single-purpose and
// rebinding them is a bigger table for a smaller problem. These six are the
// ones that collide.
func actions() []action {
	return []action{
		{"Up", &upKeys, "move and menu up"},
		{"Down", &downKeys, "move and menu down"},
		{"Left", &leftKeys, "move and menu left"},
		{"Right", &rightKeys, "move and menu right"},
		{"Confirm", &confirmKeys, "talk, enter, choose"},
		{"Cancel", &cancelKeys, "back out, close, pause"},
	}
}

// defaultKeys is what the bindings were before anybody touched them, captured
// once at startup rather than written down twice. A second copy of the table
// is a second copy to disagree with the first.
var defaultKeys = map[string][]ebiten.Key{}

func init() {
	for _, a := range actions() {
		defaultKeys[a.name] = append([]ebiten.Key(nil), *a.keys...)
	}
}

// keysByName is every key Ebitengine can report, indexed by the name it prints
// itself. Built by walking the enum rather than typed out, because a
// hand-written table of a hundred and forty keys is a hundred and forty
// chances to name one that does not exist — and Key.String returns "" for the
// undefined ones, which is the loop's own filter.
var keysByName = func() map[string]ebiten.Key {
	m := map[string]ebiten.Key{}
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if n := k.String(); n != "" {
			m[strings.ToLower(n)] = k
		}
	}
	return m
}()

// applyBindings replaces the built-in bindings with whatever the preferences
// hold. Anything absent, empty, or naming a key this engine does not have
// keeps its default: a settings file is not a place to lose the ability to
// walk.
func applyBindings(stored map[string][]string) {
	for _, a := range actions() {
		*a.keys = append([]ebiten.Key(nil), defaultKeys[a.name]...)
		names, ok := stored[a.name]
		if !ok || len(names) == 0 {
			continue
		}
		var keys []ebiten.Key
		for _, n := range names {
			if k, ok := keysByName[strings.ToLower(n)]; ok {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			*a.keys = keys
		}
	}
}

// storedBindings is the current bindings in the form the preferences file
// keeps them, with anything still at its default left out. A player who
// changed one key gets one line in their settings rather than a transcript of
// every key in the game.
func storedBindings() map[string][]string {
	out := map[string][]string{}
	for _, a := range actions() {
		if sameKeys(*a.keys, defaultKeys[a.name]) {
			continue
		}
		var names []string
		for _, k := range *a.keys {
			names = append(names, k.String())
		}
		out[a.name] = names
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sameKeys(a, b []ebiten.Key) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// bindingLabel is an action's keys as the settings row shows them.
func bindingLabel(keys []ebiten.Key) string {
	if len(keys) == 0 {
		return "nothing"
	}
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, k.String())
	}
	return strings.Join(names, " ")
}

// boundElsewhere reports which other action already answers to this key.
//
// Two actions on one key is not a preference, it is a menu that moves and
// chooses at the same time — and the way you find that out is by no longer
// being able to leave the screen where you did it. Refused in advance instead.
func boundElsewhere(k ebiten.Key, except string) string {
	for _, a := range actions() {
		if a.name == except {
			continue
		}
		for _, have := range *a.keys {
			if have == k {
				return a.name
			}
		}
	}
	return ""
}

// bindable reports whether a key can be put on an action at all, and says why
// not when it cannot.
//
// The screenshot key is the load-bearing refusal, the same one notAKeystroke
// makes: dumping the framebuffer is how anything in this game gets looked at,
// and a player who binds Cancel to backslash has quietly disabled the only
// camera. Modifiers are refused because they are held rather than pressed, and
// a movement key you have to hold is a movement key you cannot combine with
// anything.
func bindable(k ebiten.Key) (bool, string) {
	switch {
	case k.String() == "":
		return false, "this keyboard does not have a name for that"
	case notAKeystroke[k]:
		return false, "that key is spoken for"
	}
	return true, ""
}

// restoreBindings puts every action back to the table the game shipped with.
// It is a row on the settings screen rather than a buried command because it
// is the way out of a keyboard somebody has made unusable, and a way out you
// have to already know about is not one.
func restoreBindings() {
	for _, a := range actions() {
		*a.keys = append([]ebiten.Key(nil), defaultKeys[a.name]...)
	}
}

// keyNames lists every bindable key name, for the tests that check the table
// is real rather than for anything the game draws.
func keyNames() []string {
	out := make([]string, 0, len(keysByName))
	for n := range keysByName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
