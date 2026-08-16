// Package core holds the small primitives everything else leans on: a seeded
// RNG that can be forked deterministically, grid directions, and integer math.
package core

import (
	"hash/fnv"
	"math"
	"math/rand"
)

// RNG wraps math/rand with helpers the game reaches for constantly. Every
// generator in the game takes an *RNG rather than using the global source, so a
// world seed reproduces the same continent, the same towns, and the same jokes.
type RNG struct {
	r *rand.Rand
}

// NewRNG returns a generator seeded with seed.
func NewRNG(seed int64) *RNG { return &RNG{r: rand.New(rand.NewSource(seed))} }

// Fork derives a child generator from a label. Two calls with the same label on
// the same parent seed produce identical streams, which is what lets a point of
// interest regenerate its own interior on demand without storing the map.
func (g *RNG) Fork(label string, salt int64) *RNG {
	h := fnv.New64a()
	_, _ = h.Write([]byte(label))
	return NewRNG(int64(h.Sum64()) ^ salt)
}

// Intn returns a value in [0,n). n <= 0 yields 0.
func (g *RNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return g.r.Intn(n)
}

// Between returns a value in [lo,hi] inclusive.
func (g *RNG) Between(lo, hi int) int {
	if lo >= hi {
		return lo
	}
	return lo + g.r.Intn(hi-lo+1)
}

// Float returns a value in [0,1).
func (g *RNG) Float() float64 { return g.r.Float64() }

// Chance reports whether a p-probability event fires, p in [0,1].
func (g *RNG) Chance(p float64) bool { return g.r.Float64() < p }

// Pick returns a random element of s, or the zero value if s is empty.
func Pick[T any](g *RNG, s []T) T {
	var zero T
	if len(s) == 0 {
		return zero
	}
	return s[g.Intn(len(s))]
}

// PickN returns up to n distinct random elements of s.
func PickN[T any](g *RNG, s []T, n int) []T {
	if n >= len(s) {
		out := append([]T(nil), s...)
		g.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
		return out
	}
	idx := g.r.Perm(len(s))[:n]
	out := make([]T, 0, n)
	for _, i := range idx {
		out = append(out, s[i])
	}
	return out
}

// Shuffle permutes n elements via swap.
func (g *RNG) Shuffle(n int, swap func(i, j int)) { g.r.Shuffle(n, swap) }

// Weighted picks an index from weights proportionally. Returns -1 if all
// weights are non-positive.
func (g *RNG) Weighted(weights []int) int {
	total := 0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total == 0 {
		return -1
	}
	roll := g.Intn(total)
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		roll -= w
		if roll < 0 {
			return i
		}
	}
	return len(weights) - 1
}

// Point is an integer grid coordinate.
type Point struct{ X, Y int }

// Add returns p offset by q.
func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

// Manhattan returns the grid distance between p and q.
func (p Point) Manhattan(q Point) int { return Abs(p.X-q.X) + Abs(p.Y-q.Y) }

// Dist returns the euclidean distance between p and q.
func (p Point) Dist(q Point) float64 {
	dx, dy := float64(p.X-q.X), float64(p.Y-q.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// Dir is one of the four cardinal facings, matching the row order used by the
// character sprite sheets in the bundle (down, left, right, up).
type Dir int

const (
	DirDown Dir = iota
	DirLeft
	DirRight
	DirUp
)

// Delta returns the grid step for d.
func (d Dir) Delta() Point {
	switch d {
	case DirLeft:
		return Point{-1, 0}
	case DirRight:
		return Point{1, 0}
	case DirUp:
		return Point{0, -1}
	default:
		return Point{0, 1}
	}
}

// String names the direction.
func (d Dir) String() string {
	return [...]string{"south", "west", "east", "north"}[d]
}

// Abs returns the absolute value of n.
func Abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Clamp constrains n to [lo,hi].
func Clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// ClampF constrains f to [lo,hi].
func ClampF(f, lo, hi float64) float64 {
	if f < lo {
		return lo
	}
	if f > hi {
		return hi
	}
	return f
}

// Min returns the smaller of a and b.
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of a and b.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Max64 returns the larger of a and b. Coins and experience are int64, and
// converting them down to compare would be the kind of narrowing that goes
// unnoticed until somebody has a lot of money.
func Max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
