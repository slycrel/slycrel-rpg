// Package sky is the time of day and what it is doing outside.
//
// Both are answers to the same question — what is it like out there right now —
// and both are read by the same three things: how far you can see, how likely
// something is to come at you, and how hard it will be when it does.
//
// The design is one idea. Night and weather pull in *opposite* directions, and
// that is what makes reading the sky worth doing:
//
//   - Night is dark and hunting. You see less and more things want you.
//   - Bad weather is dark and quiet. You see less and nothing wants to be out
//     in it either.
//
// So the dangerous night is the clear one, a storm is cover, and the two
// compose into four kinds of evening rather than one slider. A player can
// ignore all of it and sleep at an inn, which is exactly the shape asked for:
// ignorable, and progressively more expensive to keep ignoring.
//
// Nothing here is stored except the step count. Weather is derived from the
// world seed, the clock and the biome, the same way scenery is derived from
// position — so it is identical on every run of a seed, it changes as you walk
// from forest into mountain, and there is no weather state to serialise, drift,
// or forget to save.
package sky

import "github.com/slycrel/slycrel-rpg/internal/core"

// DayLength is how many steps a full day and night takes.
//
// Measured in steps rather than frames because steps are the thing the player
// spends. A clock that ran on wall time would advance while somebody read a
// shop menu, and standing still would be a way to wait out the night — which is
// a mechanic nobody asked for and the opposite of what the inn is for.
//
// 480 puts a night about eight minutes of steady walking apart at the pace the
// overworld actually moves, which is long enough to be a condition you travel
// under and short enough that a player who wants to see one does not have to
// plan for it.
const DayLength = 480

// The phases, and where each one starts. Day and night are the substantial
// ones; dawn and dusk are short because their job is to warn you.
const (
	dawnAt  = 0
	dayAt   = 60
	duskAt  = 300
	nightAt = 360
)

// Phase is where the sun is.
type Phase int

// In the order they happen, so a Phase can be compared for "later than".
const (
	Dawn Phase = iota
	Day
	Dusk
	Night
)

// Clock is the time of day. The zero value is the first dawn of a new run,
// which is where a character should be standing when they walk out of the gate.
type Clock struct {
	// Step counts forward forever rather than wrapping, so Days can be read off
	// it and a save carries how long the run has been going.
	Step int `json:"step"`
}

// Tick advances the clock. Negative or zero does nothing, so a caller can pass
// a cost straight through without checking it.
func (c *Clock) Tick(n int) {
	if n > 0 {
		c.Step += n
	}
}

// Days is how many full days the run has lasted, for the record rather than for
// any mechanic. Nothing gets harder because it is Tuesday.
func (c Clock) Days() int { return c.Step / DayLength }

// Of is the position within the current day, in steps.
func (c Clock) Of() int {
	n := c.Step % DayLength
	if n < 0 {
		n += DayLength
	}
	return n
}

// Phase is what it is doing right now.
func (c Clock) Phase() Phase {
	switch n := c.Of(); {
	case n >= nightAt:
		return Night
	case n >= duskAt:
		return Dusk
	case n >= dayAt:
		return Day
	default:
		return Dawn
	}
}

// WakeAt winds the clock forward to the next occurrence of a phase's start,
// which is what a night at an inn buys: the morning.
//
// Always forward. Winding back would let a player who slept at noon arrive at
// the previous dawn and gain a day, and more to the point it would make a bed
// a way to *undo* time rather than spend it.
func (c *Clock) WakeAt(p Phase) {
	want := startOf(p)
	ahead := want - c.Of()
	if ahead <= 0 {
		ahead += DayLength
	}
	c.Step += ahead
}

func startOf(p Phase) int {
	switch p {
	case Day:
		return dayAt
	case Dusk:
		return duskAt
	case Night:
		return nightAt
	}
	return dawnAt
}

// Name is what to call a phase on screen.
func (p Phase) Name() string {
	switch p {
	case Day:
		return "day"
	case Dusk:
		return "dusk"
	case Night:
		return "night"
	}
	return "dawn"
}

// Dark reports whether it is dark enough for the world to be tinted and for
// the things in it to behave differently.
func (p Phase) Dark() bool { return p == Dusk || p == Night }

// Sight is how far a step reveals, in tiles.
//
// The daylight value is the number the overworld used before there was a clock,
// so a run spent entirely in daylight plays exactly as it did.
func (p Phase) Sight() int {
	switch p {
	case Dusk:
		return 5
	case Night:
		return 3
	}
	return 6
}

// Prowl multiplies the chance of a step turning into a fight.
//
// This is the half of the design that costs something. Night is when more of
// what lives out there is awake and interested, and it is deliberately a
// multiplier on the existing terrain roll rather than a flat addition, so
// somewhere that was safe in daylight stays proportionally safer after dark:
// the road home does not become the swamp because the sun went down.
func (p Phase) Prowl() float64 {
	switch p {
	case Dusk:
		return 1.15
	case Night:
		return 1.35
	}
	return 1
}

// LevelShift is how much harder what turns up is.
//
// One level, at night only, and it is the single most load-bearing number in
// this package: the DANGER report already measures exactly what one level over
// costs, which is how this can be added without re-tuning anything. It is
// applied before the home region clamps the roll to the player's own level, so
// the ground around the capital stays the ground around the capital after dark.
func (p Phase) LevelShift() int {
	if p == Night {
		return 1
	}
	return 0
}

// Weather is what is falling out of the sky.
type Weather int

const (
	// Clear: nothing, and the default everywhere.
	Clear Weather = iota
	// Cloudy: cover and no consequences. It exists so that a changing sky is
	// not always a changing mechanic — a world where every visible difference
	// is also a modifier is a world that has to be played rather than lived in.
	Cloudy
	// Rain: the ordinary bad weather of everywhere that is not a mountain.
	Rain
	// Storm: rain with the volume up, and the strongest cover in the game.
	Storm
	// Snow: the mountains' version, and colder about it.
	Snow
)

// Name is what to call it on screen.
func (w Weather) Name() string {
	switch w {
	case Cloudy:
		return "overcast"
	case Rain:
		return "rain"
	case Storm:
		return "storm"
	case Snow:
		return "snow"
	}
	return "clear"
}

// Falling reports whether anything is actually coming down, which is what
// decides whether the screen gets an overlay.
func (w Weather) Falling() bool { return w == Rain || w == Storm || w == Snow }

// Sight is how many tiles of the phase's reveal radius the weather takes away.
func (w Weather) Sight() int {
	switch w {
	case Rain:
		return -1
	case Snow:
		return -2
	case Storm:
		return -2
	}
	return 0
}

// Prowl multiplies the chance of a step turning into a fight, and it multiplies
// it *down*.
//
// This is the half of the design that gives something back, and it is the whole
// reason weather is worth reading rather than just worth looking at. Nothing
// out there wants to be in a downpour either. A storm is the safest travelling
// in the game and the blindest, which makes it a genuine choice rather than
// atmosphere with a penalty attached — and it means the night to be afraid of
// is the clear one.
func (w Weather) Prowl() float64 {
	switch w {
	case Rain:
		return 0.80
	case Snow:
		return 0.75
	case Storm:
		return 0.65
	}
	return 1
}

// minSight is the floor on how far you can see once everything has taken its
// share. A stormy night is the darkest the game gets and it still shows you the
// ground you are standing on, because a reveal radius of nothing means a map
// that stops filling in and a player who cannot tell whether it is broken.
const minSight = 2

// Sight combines the two into the reveal radius for one step.
func Sight(p Phase, w Weather) int {
	return core.Max(minSight, p.Sight()+w.Sight())
}

// Prowl combines the two into a multiplier on the encounter roll.
func Prowl(p Phase, w Weather) float64 { return p.Prowl() * w.Prowl() }

// spellLength is how long one weather system holds, in steps.
//
// Shorter than a phase on purpose: weather that changed with the light would
// read as one system rather than two, and the point of them being separate is
// that a clear night and a wet night are different problems.
const spellLength = 90

// At is the weather over a biome at a moment.
//
// Derived rather than stored, from the seed, which spell of the run it is, and
// where you are standing — the same trick the scenery uses. That buys three
// things: a seed reproduces its weather exactly, walking from forest into
// mountain turns rain into snow without anything having to track a boundary,
// and there is no weather in the save file to get out of step with the world it
// belongs to.
//
// Biome decides what is even possible. Nowhere warm gets snow, nowhere cold
// gets rain, and the deep desert stays clear because a downpour there would be
// the most memorable thing in the run and it should not be free.
func At(seed int64, c Clock, biome string) Weather {
	// The golden-ratio constant, as the signed half of the 64-bit value: a
	// plain multiply by the spell number would leave neighbouring spells with
	// neighbouring seeds, and NewRNG is not obliged to scramble that for us.
	const mix = 0x1E3779B97F4A7C15
	spell := c.Step / spellLength
	g := core.NewRNG(seed ^ int64(spell)*mix)
	roll := g.Intn(100)

	switch biome {
	case "mountain", "tundra", "peak":
		switch {
		case roll < 52:
			return Clear
		case roll < 74:
			return Cloudy
		default:
			return Snow
		}
	case "desert", "wasteland":
		if roll < 82 {
			return Clear
		}
		return Cloudy
	case "swamp", "coast", "rainforest", "jungle":
		// Wet places are wet. This is the one biome group where a traveller
		// should expect to be rained on rather than surprised by it.
		switch {
		case roll < 38:
			return Clear
		case roll < 56:
			return Cloudy
		case roll < 86:
			return Rain
		default:
			return Storm
		}
	default:
		switch {
		case roll < 56:
			return Clear
		case roll < 76:
			return Cloudy
		case roll < 94:
			return Rain
		default:
			return Storm
		}
	}
}
