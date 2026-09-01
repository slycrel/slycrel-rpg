package world

import "github.com/slycrel/slycrel-rpg/internal/core"

// A Wanderer is an encounter the player can see coming.
//
// The overworld roll has always fired straight into the battle screen: a step
// into tall grass, a hidden percentage, and a cut to combat. That gives the
// player no information and one lever, which is walking somewhere else. This
// does not change the roll, the rate, or the tables — the same `dangerRoll`
// fires exactly as often — it changes what the roll *produces*. Instead of a
// fight, it puts a creature a few tiles off, and the fight happens when the two
// of you meet. The encounter it will become is decided at the moment it appears,
// so what you see is what you get.
//
// Deliberately not saved. A wanderer is weather, not furniture: the save format
// is seed plus deltas, and a creature that persisted would have to be a delta,
// which would mean every file written before today loads with none of them and
// every file written after carries a list of animals. Walk away and it is gone;
// come back and the grass rolls you a new one.
type Wanderer struct {
	Pos core.Point
	// Kind is a model.MonsterKind, held as a string for the same reason
	// Entity.Class is: world generation must not drag the character model in.
	// It selects the sprite and nothing else.
	Kind string
	// Seen is set once it has noticed you, and never cleared. A creature that
	// forgot about you every time you stepped out of its radius would jitter on
	// the boundary instead of committing.
	Seen bool
	// Life counts down the steps it will keep looking before it loses interest.
	Life int
}

const (
	// WanderSpawnMin and Max are how far off it appears: close enough to be
	// obviously about you, far enough that turning round is a real option.
	WanderSpawnMin = 4
	WanderSpawnMax = 6
	// WanderNotice is how near you have to be before it comes at you.
	WanderNotice = 4
	// WanderGiveUp is the distance at which it stops being your problem. Larger
	// than the reveal radius at noon, so a creature does not wink out inside
	// the circle you can see.
	WanderGiveUp = 14
	// WanderLife is how many of its own steps it takes before losing interest.
	WanderLife = 90
	// WanderLean is how often a drifting creature drifts *toward* you rather
	// than in a direction off the compass.
	//
	// Not a beeline, which the paragraph on Step explains is worse than a slow
	// creature — something that walks straight at you from across the map is
	// not avoidable, only postponed. A lean is the difference between milling
	// about and milling about in your general direction, and it is what turns a
	// spawn into a meeting often enough to be worth the roll that made it.
	//
	// Measured, and the measurement is why this is 0.55 and not higher. Across
	// a sweep from a pure random walk to nearly-always-leaning, the share of
	// spawns that reach a walking player only moves from 42% to 51% — the lean
	// is worth about six points of it and no more. What actually decided how
	// often anything happens was the *cap* on how many may be out at once; see
	// wanderCap in the overworld scene. This is kept because it is the
	// difference between milling about and milling about your way, which is
	// what makes a creature read as coming for you, not because it is the lever.
	WanderLean = 0.55
)

// SpawnWanderer places a creature on open ground a few tiles from `near`.
//
// It refuses to stand on a location, because a marker and a monster on one
// square is a square the player cannot read, and refuses the player's own tile
// for the obvious reason. Returns nil when nowhere in the ring works, which
// happens on a beach or a spit of land — and a roll that finds nowhere to put
// its creature is simply a roll that does not become a fight. That is the one
// place the visible model gives up a fight the invisible one would have had.
func (m *Map) SpawnWanderer(g *core.RNG, near core.Point, kind string) *Wanderer {
	for try := 0; try < 24; try++ {
		d := WanderSpawnMin + g.Intn(WanderSpawnMax-WanderSpawnMin+1)
		// A ring rather than a box: sample an offset and reject the ones that
		// land too close on the diagonal.
		dx := g.Intn(2*d+1) - d
		dy := g.Intn(2*d+1) - d
		p := core.Point{X: near.X + dx, Y: near.Y + dy}
		if chebyshev(p, near) < WanderSpawnMin {
			continue
		}
		if p == near || !m.Walkable(p.X, p.Y) || m.POIAt(p.X, p.Y) != nil {
			continue
		}
		return &Wanderer{Pos: p, Kind: kind, Life: WanderLife}
	}
	return nil
}

// Step moves the wanderer one tile, and reports whether it is still around.
//
// Outside its notice radius it drifts at random, which is the same rule the
// interior foes use and for the same reason: something that beelines from
// across the map is not avoidable, it is only slow. Inside the radius it closes
// on the target and keeps closing, because an encounter you can shake off by
// stepping sideways once is not an encounter.
func (w *Wanderer) Step(g *core.RNG, m *Map, toward core.Point) bool {
	w.Life--
	if w.Life <= 0 || chebyshev(w.Pos, toward) > WanderGiveUp {
		return false
	}

	if chebyshev(w.Pos, toward) <= WanderNotice {
		w.Seen = true
	}

	if !w.Seen {
		// Drifting is lazy on purpose: most ticks it does nothing, so a
		// creature in the middle distance reads as milling about rather than
		// pacing.
		if !g.Chance(0.35) {
			return true
		}
		// And it leans your way. A pure random walk is a creature that mostly
		// leaves: it has WanderGiveUp tiles of rope and a random walk spends
		// them, so the encounter the grass rolled for turns into nothing at
		// all far more often than it turns into a fight.
		if g.Chance(WanderLean) {
			w.tryMove(m, w.Pos.Add(w.lean(g, toward).Delta()))
			return true
		}
		d := core.Dir(g.Intn(4))
		w.tryMove(m, w.Pos.Add(d.Delta()))
		return true
	}

	// Close on the larger axis first, falling back to the other when the step
	// is blocked, which is enough to get around a lake without pathfinding.
	step := core.Point{X: w.Pos.X, Y: w.Pos.Y}
	dx, dy := sign(toward.X-w.Pos.X), sign(toward.Y-w.Pos.Y)
	if abs(toward.X-w.Pos.X) >= abs(toward.Y-w.Pos.Y) {
		step.X += dx
		if !w.tryMove(m, step) && dy != 0 {
			w.tryMove(m, core.Point{X: w.Pos.X, Y: w.Pos.Y + dy})
		}
	} else {
		step.Y += dy
		if !w.tryMove(m, step) && dx != 0 {
			w.tryMove(m, core.Point{X: w.Pos.X + dx, Y: w.Pos.Y})
		}
	}
	return true
}

// lean is a direction that closes the distance, choosing the axis it is
// furthest away on so a creature crosses the gap rather than circling it.
//
// One axis at a time and never diagonally, because the drift is meant to read
// as an animal going about its business that happens to be coming this way.
func (w *Wanderer) lean(g *core.RNG, toward core.Point) core.Dir {
	dx, dy := toward.X-w.Pos.X, toward.Y-w.Pos.Y
	// On a tie, either axis — otherwise a creature exactly diagonal would
	// always pick the same one and walk a staircase.
	useX := abs(dx) > abs(dy) || (abs(dx) == abs(dy) && g.Chance(0.5))
	if useX && dx != 0 {
		if dx > 0 {
			return core.DirRight
		}
		return core.DirLeft
	}
	if dy > 0 {
		return core.DirDown
	}
	if dy < 0 {
		return core.DirUp
	}
	return core.Dir(g.Intn(4))
}

// tryMove takes the step if the ground allows it, and reports whether it did.
// A location is refused for the same reason it is refused at spawn.
func (w *Wanderer) tryMove(m *Map, to core.Point) bool {
	if !m.Walkable(to.X, to.Y) || m.POIAt(to.X, to.Y) != nil {
		return false
	}
	w.Pos = to
	return true
}

// chebyshev is distance in king moves, which is the right metric here because
// the creature moves in eight directions and the player reads the map as a
// grid: a thing four tiles away diagonally looks four tiles away.
func chebyshev(a, b core.Point) int {
	dx, dy := abs(a.X-b.X), abs(a.Y-b.Y)
	if dx > dy {
		return dx
	}
	return dy
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}
	return 0
}
