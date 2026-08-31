package rules

// ClampedMean exposes the retreat policy's damage estimate to the external test
// package. It is unexported in the game because nothing outside these rules has
// any business estimating damage — the point of the function is that the policy
// stops keeping its own copy of the arithmetic — and it is exposed here because
// the alternative is testing it only through a fight, where a bias of two or
// three points a monster hides inside a win rate. Which is how the bias it
// replaced survived.
func ClampedMean(lo, hi, guard float64) float64 { return clampedMean(lo, hi, guard) }
