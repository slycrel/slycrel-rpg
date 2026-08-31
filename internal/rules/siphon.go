package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// The mage's half of the class-identity scheme: a barrier that refills.
//
// It was proposed as flavour — something the class earns by attacking rather
// than another number — and CROWDS gave it a better brief than that. The Mage
// is the class left behind in fourteen of the level-and-size cells where a
// group fight is winnable at all: at three creatures on level it posts 4%, 12%,
// 15%, 4%, 29% and 3% across the levels against a Fighter's 18, 83, 53, 41, 94
// and 34.
//
// That is the design's own prediction arriving as a measurement rather than a
// surprise. A barrier is a pool spent once, which is the worst possible unit
// against a crowd: it is gone after the first blow and the next five arrive
// against nothing. Armour comes off *every* blow and so is worth more the more
// of them there are; a dodge is worth the same share whatever the count. The
// three units were chosen to diverge and this is where they diverge hardest.
//
// So the fix is not a bigger pool, which would only move the point at which it
// runs out. It is a pool that comes back — and tying that to damage dealt is
// what makes it the Mage's own loop rather than a second helping of armour: the
// class that answers a crowd by killing into it faster.
//
// Deliberately *not* healing. The Mage already has the deepest psyche pool and
// the only healing techniques in the game; lifesteal would hand more sustain to
// the class that has the most of it, and would be a fourth answer to "do not
// die" rather than a repair of the one it owns.

const (
	// siphonShare is how much of a blow's damage comes back as barrier.
	//
	// Measured against the gap it exists to close rather than chosen. One-on-one
	// it does nothing at any share — the duel is already a hundred per cent for
	// everybody, which is why the curve is set there and why this could never
	// have been priced there. Against three on level it takes the Mage from
	// 16.7%, 18.8%, 7.5% and 3.5% across the levels to 40.5%, 43.2%, 15.2% and
	// 14.3%: roughly triple, and still behind the Fighter's 83.7, 53.7, 45.0 and
	// 31.5 at every one of them. At 0.80 it reaches parity at level thirteen,
	// which is further than the fragile class should go.
	siphonShare = 0.35
	// siphonCap is the most the pool may be carrying at once, as a multiple of
	// what the talisman put up. Without a ceiling a long fight against many
	// weak things would end with a Mage behind a wall bigger than its own hit
	// points, which is a different class rather than a repaired one.
	siphonCap = 2.0
)

// CanSiphon reports whether dealing damage rebuilds the barrier. The talisman
// is the gate rather than the class, because the talisman *is* the unit: a Mage
// holding a dagger and nothing on its arm has no pool to refill, and anybody
// else cannot hold one at all.
func CanSiphon(c *model.Character) bool {
	return c != nil && c.Shield.Barrier()
}

// Siphon turns damage dealt into barrier, up to the ceiling, and reports how
// much came back so the transcript can say so. A pool that silently refills is
// a pool the player never learns they have.
func Siphon(c *model.Character, dealt int) int {
	if !CanSiphon(c) || dealt <= 0 {
		return 0
	}
	base, ok := Barrier(c)
	if !ok || base.Power <= 0 {
		return 0
	}
	ceiling := int(float64(base.Power) * siphonCap)

	have := 0
	for _, e := range c.Active {
		if e.Kind == model.EffectBarrier {
			have += e.Power
		}
	}
	room := ceiling - have
	if room <= 0 {
		return 0
	}
	gain := core.Min(room, core.Max(1, int(float64(dealt)*siphonShare)))

	// Into an existing barrier where there is one, so the pool stays a single
	// number rather than a stack of small ones that Soak would drain in an
	// order nobody chose.
	for i := range c.Active {
		if c.Active[i].Kind == model.EffectBarrier {
			c.Active[i].Power += gain
			return gain
		}
	}
	c.Active = append(c.Active, model.Effect{
		Kind: model.EffectBarrier, Power: gain, Rounds: model.Forever,
	})
	return gain
}
