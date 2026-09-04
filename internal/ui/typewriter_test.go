package ui_test

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/ui"
)

var speech = []string{"Take this to the", "lighthouse keeper."}

// The row count never changes, however little has arrived.
//
// This is the property the whole type exists for. The caller sizes its panel
// off the wrapped lines, so a Visible that returned only the rows reached would
// have every line slide upward as the next one started — which reads as the
// panel scrolling rather than as somebody speaking, and it is what a naive
// implementation does.
func TestTheShapeOfTheTextNeverChanges(t *testing.T) {
	w := ui.NewTypewriter(speech, 0.5)
	for i := 0; i < 200; i++ {
		if got := len(w.Visible()); got != len(speech) {
			t.Fatalf("after %d ticks the text is %d rows, the box was drawn for %d", i, got, len(speech))
		}
		w.Tick()
	}
}

// What is revealed is always a prefix of what was given, and it only ever
// grows. A cut that landed mid-way through a multi-byte rune would draw
// mojibake, and one that went backwards would read as a stutter.
func TestTheTextOnlyEverArrives(t *testing.T) {
	w := ui.NewTypewriter(speech, 0.37)
	full := strings.Join(speech, "")
	last := 0
	for i := 0; i < 400; i++ {
		got := strings.Join(w.Visible(), "")
		if !strings.HasPrefix(full, got) {
			t.Fatalf("tick %d shows %q, which is not the start of %q", i, got, full)
		}
		if len([]rune(got)) < last {
			t.Fatalf("tick %d went backwards, from %d characters to %d", i, last, len([]rune(got)))
		}
		last = len([]rune(got))
		w.Tick()
	}
	if !w.Done() {
		t.Error("the text never finished arriving")
	}
	if strings.Join(w.Visible(), "") != full {
		t.Error("the finished text is not the text it was given")
	}
}

// A rate under one character a tick still has to move, or the slowest setting
// is a box that never fills.
func TestASlowRateStillArrives(t *testing.T) {
	w := ui.NewTypewriter([]string{"abcdefghij"}, 0.25)
	for i := 0; i < 39; i++ {
		w.Tick()
	}
	if w.Done() {
		t.Error("ten characters at a quarter each finished in under forty ticks")
	}
	w.Tick()
	if !w.Done() {
		t.Error("ten characters at a quarter each did not finish in forty ticks")
	}
}

// Finish is what a key press does, and a player who has read the line must be
// able to get past it without waiting out an animation.
func TestFinishArrivesAtTheSameText(t *testing.T) {
	w := ui.NewTypewriter(speech, 0.5)
	w.Tick()
	w.Finish()
	if !w.Done() {
		t.Fatal("Finish left text still arriving")
	}
	full := ui.NewTypewriter(speech, 0)
	if strings.Join(w.Visible(), "|") != strings.Join(full.Visible(), "|") {
		t.Error("finishing early and never starting disagree about the text")
	}
}

// A rate of nothing means the effect is off, and off has to be correct on the
// first frame rather than after one — the alternative is a caller that must
// remember to special-case it, which is the branch this is here to remove.
func TestNoRateIsAlreadyFinished(t *testing.T) {
	w := ui.NewTypewriter(speech, 0)
	if !w.Done() {
		t.Error("a rate of zero is still typing")
	}
	if strings.Join(w.Visible(), " ") != strings.Join(speech, " ") {
		t.Error("a rate of zero is not showing the whole text")
	}
}

// Nothing to say is finished, not stuck. An empty body is a real case — a
// message box with only a menu in it — and a Done that never came would leave
// the menu permanently unreachable.
func TestNothingToSayIsAlreadySaid(t *testing.T) {
	for _, empty := range [][]string{nil, {}, {""}, {"", ""}} {
		if w := ui.NewTypewriter(empty, 1); !w.Done() {
			t.Errorf("%q is still arriving", empty)
		}
	}
}

// The transcript's own reveal, which is a different problem from a message
// box's: entries arrive while earlier ones may still be arriving.

// A new entry starts from nothing, however far the last one had got.
//
// The bound the whole design rests on is that only the newest entry is ever
// arriving — a round can queue seven messages, and two half-arrived sentences
// at once is either a queue nobody wrote or a transcript falling further
// behind every round. What that costs is a counter that has to be reset, and
// the reset is invisible unless the new entry is *shorter* than the progress
// carried into it: with a long line half revealed, a short one after it is
// already past its own length and arrives whole.
//
// Which is what the first version of this test failed to notice. It used two
// long lines, so a missing reset left the count below both of their lengths
// and every assertion passed with the reset deleted.
func TestANewEntryStartsFromNothing(t *testing.T) {
	l := ui.NewLog(10)
	l.Typing(1)
	l.Add("a sentence quite a lot longer than the one that follows it")
	for i := 0; i < 30; i++ {
		l.Tick()
	}
	if l.Settled() {
		t.Fatal("fifty-eight characters at one a tick finished in thirty")
	}
	l.Add("it bites.")
	if l.Settled() {
		t.Error("a short entry after a long one arrived already finished")
	}
	for i := 0; i < 9; i++ {
		l.Tick()
	}
	if !l.Settled() {
		t.Error("nine characters at one a tick did not arrive in nine ticks")
	}
}

// A log nobody asked to type is settled always, so a caller that waits on
// Settled — which the battle does, every frame — cannot deadlock by forgetting
// to set a rate.
func TestALogThatDoesNotTypeIsAlwaysSettled(t *testing.T) {
	l := ui.NewLog(10)
	if !l.Settled() {
		t.Error("an empty log is not settled")
	}
	l.Add("something happened")
	if !l.Settled() {
		t.Error("a log with no typing rate is waiting for something")
	}
	// And an empty log that does type is settled too, or the first frame of a
	// fight waits forever for a sentence nobody has written.
	e := ui.NewLog(10)
	e.Typing(1)
	if !e.Settled() {
		t.Error("an empty typing log is not settled")
	}
}

// The rate means characters a tick, including below one. A slow setting that
// silently ran at sixty a second would make the setting decorative.
func TestTheLogHonoursItsRate(t *testing.T) {
	for _, rate := range []float64{0.75, 1, 1.5} {
		l := ui.NewLog(10)
		l.Typing(rate)
		const n = 60
		l.Add("%s", strings.Repeat("x", n))
		ticks := 0
		for !l.Settled() && ticks < 1000 {
			l.Tick()
			ticks++
		}
		if want := int(float64(n)/rate + 0.999); ticks != want {
			t.Errorf("%d characters at %.2f a tick took %d ticks, expected %d", n, rate, ticks, want)
		}
	}
}
