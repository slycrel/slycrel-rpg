package game

import "github.com/slycrel/slycrel-rpg/internal/core"

// How long a combat step holds.
//
// The numbers are ticks at 60 a second, so a step is half a second, three
// quarters, or one. What the setting is really adjusting is reading speed: a
// round of a three-strong company against four monsters queues seven messages,
// and the difference between the ends of this range is three seconds of
// waiting against seven.
//
// The old fixed value is paceFast, deliberately. It shipped for months and
// somebody who liked it should be able to keep it exactly, rather than getting
// a new number that is nearly the same — and the complaint was never that it
// was wrong, it was that it was the only one.
const (
	paceFast   = 30
	paceSteady = 45
	paceSlow   = 60
)

// paceTicks is the settable ladder, slowest last so that walking down the list
// walks down the speed.
var paceTicks = []int{paceFast, paceSteady, paceSlow}

// paceNames are what the settings row shows. Plain words: this is a row about
// how long to wait, and a joke in the detail column of a settings screen is a
// joke somebody has to decode before they can change the thing they came to
// change.
var paceNames = []string{"fast", "steady", "slow"}

// paceIndex is which rung the current pace sits on, defaulting to the middle
// for any value that is not on the ladder — which includes the zero a
// settings file written before this existed will supply.
func paceIndex(ticks int) int {
	for i, t := range paceTicks {
		if t == ticks {
			return i
		}
	}
	return 1
}

// applyPace sets the combat pace from a stored preference.
func applyPace(ticks int) { stepTicks = paceTicks[paceIndex(ticks)] }

// setPace moves the pace by one rung and reports the new tick count, clamped
// at both ends rather than wrapping. A wrapping setting is one you can
// overshoot into its opposite by holding a key a beat too long.
func setPace(step int) int {
	i := core.Clamp(paceIndex(stepTicks)+step, 0, len(paceTicks)-1)
	stepTicks = paceTicks[i]
	return stepTicks
}

// typeRate is how fast a message box fills, in characters a tick, derived from
// the same setting that decides how long a combat step holds.
//
// One setting rather than two, and derived rather than tabled, because both
// numbers are answering the same question: how fast does this game hand you
// something to read. A player who slowed combat down did it to keep up with
// the transcript, and would have to find a second row to slow the dialogue
// they are equally behind on.
//
// The middle rung is one character a tick — 60 a second, which is where the
// machines this is imitating sat — so the ladder runs 1.5, 1.0 and 0.75. A
// hundred-character line takes a second and a half at the top and just over
// two at the bottom.
// The tour gets the text all at once. -demo takes one frame per screen, and a
// screen photographed mid-sentence documents nothing — the quest offer came out
// as `"I need 3 Rank P` and stopped. The guard is here rather than at the call
// site so it travels to any new one, which is the same reason autosave's is
// inside autosave.
func (g *Game) typeRate() float64 {
	if g.InDemo() {
		return 0
	}
	return float64(paceSteady) / float64(stepTicks)
}

// paceName is the current setting as the settings row shows it.
func paceName() string { return paceNames[paceIndex(stepTicks)] }
