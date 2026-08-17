package sky_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/sky"
)

// TestTheDayCoversItself. Four phases, no gaps, no overlaps, and dawn at zero
// so a new character walks out of the gate in the morning.
func TestTheDayCoversItself(t *testing.T) {
	seen := map[sky.Phase]int{}
	for step := 0; step < sky.DayLength; step++ {
		seen[sky.Clock{Step: step}.Phase()]++
	}
	for _, p := range []sky.Phase{sky.Dawn, sky.Day, sky.Dusk, sky.Night} {
		if seen[p] == 0 {
			t.Errorf("%s never happens", p.Name())
		}
	}
	if got := (sky.Clock{}).Phase(); got != sky.Dawn {
		t.Errorf("a new run begins at %s, want dawn", got.Name())
	}
	// Day and night are the substantial ones. Dawn and dusk are warnings, not
	// conditions to travel under.
	if seen[sky.Day] <= seen[sky.Dawn] || seen[sky.Night] <= seen[sky.Dusk] {
		t.Errorf("phase lengths are dawn %d, day %d, dusk %d, night %d; "+
			"the transitions should be the short ones",
			seen[sky.Dawn], seen[sky.Day], seen[sky.Dusk], seen[sky.Night])
	}
}

// TestSleepingAlwaysCostsTime. A bed buys the morning, and it has to buy it
// forwards: winding back would let somebody who slept at noon arrive at the
// previous dawn, which turns a night at an inn into a way to undo a day.
func TestSleepingAlwaysCostsTime(t *testing.T) {
	for step := 0; step < DayLengthTwice(); step++ {
		c := sky.Clock{Step: step}
		before := c
		c.WakeAt(sky.Dawn)
		if c.Step <= before.Step {
			t.Fatalf("sleeping at step %d moved the clock to %d", before.Step, c.Step)
		}
		if c.Step-before.Step > sky.DayLength {
			t.Fatalf("sleeping at step %d cost %d steps, more than a whole day",
				before.Step, c.Step-before.Step)
		}
		if got := c.Phase(); got != sky.Dawn {
			t.Fatalf("sleeping at step %d woke up in the %s", before.Step, got.Name())
		}
	}
}

// DayLengthTwice keeps the loop above honest about wrapping.
func DayLengthTwice() int { return sky.DayLength * 2 }

// TestNightHuntsAndWeatherHides is the whole design in one assertion.
//
// The two pull in opposite directions, and that is what makes reading the sky
// worth doing rather than being a slider from good to bad. If both ever pushed
// the same way there would be no reason to look up: the answer would always be
// "wait for a clear noon".
func TestNightHuntsAndWeatherHides(t *testing.T) {
	if sky.Night.Prowl() <= sky.Day.Prowl() {
		t.Error("more things are not out at night, which is most of what night is for")
	}
	if sky.Night.Sight() >= sky.Day.Sight() {
		t.Error("you can see as far at night as at noon")
	}
	for _, w := range []sky.Weather{sky.Rain, sky.Storm, sky.Snow} {
		if w.Prowl() >= 1 {
			t.Errorf("%s does not keep anything indoors; then it is a penalty with no give in it", w.Name())
		}
		if w.Sight() >= 0 {
			t.Errorf("%s costs nothing to see through; then it is a gift with no take in it", w.Name())
		}
	}
	// Clear and overcast are the two that mean nothing, deliberately: a world
	// where every visible difference is also a modifier is a world that has to
	// be played rather than lived in.
	for _, w := range []sky.Weather{sky.Clear, sky.Cloudy} {
		if w.Prowl() != 1 || w.Sight() != 0 {
			t.Errorf("%s has a mechanic attached to it", w.Name())
		}
	}

	// And the composed claim, which is the interesting one: the night to be
	// afraid of is the clear one.
	clear := sky.Prowl(sky.Night, sky.Clear)
	stormy := sky.Prowl(sky.Night, sky.Storm)
	if stormy >= clear {
		t.Errorf("a stormy night (%.2f) is no safer than a clear one (%.2f); "+
			"then a storm is only ever bad news and there is nothing to read", stormy, clear)
	}
	// A storm should not make the night safer than an ordinary day, or the
	// correct play becomes travelling exclusively in bad weather after dark.
	if stormy < 0.8*sky.Prowl(sky.Day, sky.Clear) {
		t.Errorf("a stormy night is %.2f against a clear day's %.2f; cover this good "+
			"makes the small hours the best time to be out",
			stormy, sky.Prowl(sky.Day, sky.Clear))
	}
}

// TestYouCanAlwaysSeeTheGroundYouAreOn. A reveal radius of nothing is a map
// that stops filling in, which a player reads as broken rather than as dark.
func TestYouCanAlwaysSeeTheGroundYouAreOn(t *testing.T) {
	for _, p := range []sky.Phase{sky.Dawn, sky.Day, sky.Dusk, sky.Night} {
		for _, w := range []sky.Weather{sky.Clear, sky.Cloudy, sky.Rain, sky.Storm, sky.Snow} {
			if got := sky.Sight(p, w); got < 2 {
				t.Errorf("%s in %s reveals %d tiles", p.Name(), w.Name(), got)
			}
		}
	}
}

// TestWeatherIsDerivedNotRolled. Nothing about the sky is stored, so the same
// seed at the same moment in the same place has to give the same answer every
// time — otherwise a save reloaded under a downpour comes back to sunshine, and
// the reveal radius the player was planning around changes under them.
func TestWeatherIsDerivedNotRolled(t *testing.T) {
	for _, seed := range []int64{1, 7, 1994} {
		for _, step := range []int{0, 91, 480, 1207} {
			c := sky.Clock{Step: step}
			for _, biome := range []string{"forest", "mountain", "swamp", "desert"} {
				first := sky.At(seed, c, biome)
				for i := 0; i < 8; i++ {
					if got := sky.At(seed, c, biome); got != first {
						t.Fatalf("seed %d step %d in %s gave %s then %s",
							seed, step, biome, first.Name(), got.Name())
					}
				}
			}
		}
	}
}

// TestTheSkyActuallyChanges, in both senses: over a run, and between biomes.
//
// Every gate here is a place the feature can quietly switch itself off. A spell
// length longer than a run, or a table that never leaves its first case, and
// the weather art is drawn once in testing and never again by anybody.
func TestTheSkyActuallyChanges(t *testing.T) {
	const seed = 1994
	over := map[sky.Weather]int{}
	for step := 0; step < sky.DayLength*20; step += 30 {
		over[sky.At(seed, sky.Clock{Step: step}, "forest")]++
	}
	if len(over) < 3 {
		t.Errorf("twenty days of forest weather produced %d kinds: %v", len(over), over)
	}

	// Cold places get snow and never rain; warm places the reverse. Nothing
	// else in the suite would notice a table that had them the wrong way round.
	var sawSnow, sawRain bool
	for step := 0; step < sky.DayLength*20; step += 30 {
		c := sky.Clock{Step: step}
		switch sky.At(seed, c, "mountain") {
		case sky.Snow:
			sawSnow = true
		case sky.Rain, sky.Storm:
			t.Fatalf("it is raining on a mountain at step %d", step)
		}
		switch sky.At(seed, c, "swamp") {
		case sky.Rain, sky.Storm:
			sawRain = true
		case sky.Snow:
			t.Fatalf("it is snowing in a swamp at step %d", step)
		}
	}
	if !sawSnow {
		t.Error("twenty days and it never snowed on a mountain")
	}
	if !sawRain {
		t.Error("twenty days and it never rained in a swamp")
	}
}
