package ui

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/render"
)

// TestALogEntryIsStoredWhole.
//
// The log used to wrap every message into rows at a fixed width and store
// those, which quietly turned it from a list of things that happened into a
// list of *rows*. Everything downstream inherited that. The walking-around
// screen draws one row and got the newest one — the tail of a sentence, shown
// as though it were the sentence: "Somebody will hear about it." in place of
// the line explaining why. And DrawWrapped's promise that an entry goes in
// whole or not at all was operating on rows, so it was not the promise it said
// it was either.
//
// So this is the invariant the fix rests on: one call in, one entry out,
// however long it is.
func TestALogEntryIsStoredWhole(t *testing.T) {
	l := NewLog(10)
	long := "They were in the middle of something. Somebody will hear about it, " +
		"and it will be repeated with additions."
	if render.TextW(long) <= render.ScreenW {
		t.Fatalf("the test's own line is too short to exercise wrapping: %.0f px",
			render.TextW(long))
	}
	l.Add("%s", long)

	if len(l.lines) != 1 {
		t.Fatalf("one message became %d entries", len(l.lines))
	}
	if l.lines[0].text != long {
		t.Errorf("stored %q, added %q", l.lines[0].text, long)
	}
}

// TestTheLogKeepsTheNewestEntries. max is a count of entries now rather than of
// rows, so a run of long messages cannot push short ones out faster than a run
// of short ones.
func TestTheLogKeepsTheNewestEntries(t *testing.T) {
	l := NewLog(3)
	for _, s := range []string{"first", "second", strings.Repeat("long ", 40), "fourth"} {
		l.Add("%s", s)
	}
	if len(l.lines) != 3 {
		t.Fatalf("a log capped at 3 holds %d", len(l.lines))
	}
	if l.lines[0].text != "second" {
		t.Errorf("the oldest survivor is %q, want %q", l.lines[0].text, "second")
	}
	if l.lines[2].text != "fourth" {
		t.Errorf("the newest is %q, want %q", l.lines[2].text, "fourth")
	}
}
