package render_test

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/render"
)

// The UI face covers Latin-1 and nothing else, so typography a writer naturally
// types has to be folded to ASCII before it reaches the screen. Left alone, the
// em-dash in the hit table drew every landed blow as "Bosk slap the Wolf @ 6".
func TestTypographyIsFoldedToWhatTheFontHas(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a — b", "a - b"},
		{"a – b", "a - b"},
		{"wait…", "wait..."},
		{"“quoted”", `"quoted"`},
		{"it’s", "it's"},
		{"hard space", "hard space"},
		{"plain ascii", "plain ascii"},
	}
	for _, c := range cases {
		// Wrap is the widest path a line of prose takes, and it folds first.
		got := strings.Join(render.Wrap(c.in, 10_000), "")
		if got != c.want {
			t.Errorf("Wrap(%q) = %q, want %q", c.in, got, c.want)
		}
		if got := render.Trunc(c.in, 10_000); got != c.want {
			t.Errorf("Trunc(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Truncation cuts by runes. Cutting by bytes leaves a half-encoded character at
// the end of the string, which draws as garbage rather than as a clipped name.
func TestTruncNeverCutsARuneInHalf(t *testing.T) {
	const name = "Ærik Ømsbjørn the Considerable"
	// From three glyphs up. Below that there is no answer that both fits and
	// says anything, and Trunc gives up and returns one character.
	for width := 3 * render.TextW("M"); width < render.TextW(name); width += 3 {
		got := render.Trunc(name, width)
		if !isValidUTF8(got) {
			t.Fatalf("Trunc(%q, %v) = %q, which is not valid UTF-8", name, width, got)
		}
		if w := render.TextW(got); w > width {
			t.Fatalf("Trunc(%q, %v) = %q, which measures %v", name, width, got, w)
		}
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

// A name that already fits has to come back untouched, or every label in the
// game would grow a trailing full stop.
func TestTruncLeavesAFittingStringAlone(t *testing.T) {
	const s = "Bosk II"
	if got := render.Trunc(s, render.TextW(s)); got != s {
		t.Errorf("Trunc(%q) at its own width = %q", s, got)
	}
}
