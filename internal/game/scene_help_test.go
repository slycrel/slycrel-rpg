package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/render"
)

// The help screen fits on the screen.
//
// It is the one panel in the game that grows every time anything is added
// anywhere else, and it grew past its own frame the first time it did: both
// panels were fixed heights that happened to suit the lists in them, so the
// tenth key row ran six pixels into the panel below it and nothing said so.
// Nothing can say so — a panel is a rectangle and text is text.
func TestTheHelpScreenFitsOnTheScreen(t *testing.T) {
	if got := helpBottom(); got > render.ScreenH {
		t.Errorf("the help screen ends at %.0f on a %d-pixel screen: %d keys and "+
			"%d notes is more than it holds", got, render.ScreenH,
			len(helpKeys), len(helpNotes))
	}

	// And every note is one line, which is the rule the panel is sized by.
	for _, n := range helpNotes {
		if got := len(render.Wrap(n, helpNoteW)); got != 1 {
			t.Errorf("%q wraps to %d lines; a note is one line and costs the "+
				"note below it its place otherwise", n, got)
		}
	}
}
