package ui

// Typewriter reveals already-wrapped text one character at a time, which is
// how a game of this shape has always delivered a sentence somebody said.
//
// It takes *wrapped* lines rather than a paragraph, and that is the whole of
// why it is a type rather than a counter held by each caller. Wrapping has to
// happen against the full text or the box changes shape as it fills: a line
// that has not arrived yet still decides how tall the panel is, and a panel
// that grows while you read it is worse than no effect at all. So the caller
// wraps once, sizes its box off the result, and hands the finished lines here
// to be uncovered.
type Typewriter struct {
	// The finished text, by rendered row, kept as runes because the count
	// this advances is a count of characters and the fold leaves some of them
	// multi-byte. Advancing by bytes would reveal half of one.
	rows [][]rune
	// total is every character across every row. shown is how many of them
	// have arrived.
	total, shown int
	// rate is characters per tick, and acc carries the fraction between ticks
	// so a rate below one still moves.
	rate, acc float64
}

// NewTypewriter starts revealing the given wrapped lines at the given rate in
// characters per tick.
//
// A rate of zero or less means the text is already finished, which is the
// honest way to express "this player has turned the effect off" — the caller
// then needs no branch, and every question it asks (is it done, what is
// visible) answers correctly on the first frame.
func NewTypewriter(lines []string, rate float64) *Typewriter {
	t := &Typewriter{rate: rate}
	for _, ln := range lines {
		r := []rune(ln)
		t.rows = append(t.rows, r)
		t.total += len(r)
	}
	if rate <= 0 {
		t.shown = t.total
	}
	return t
}

// Tick advances the reveal by one frame's worth.
func (t *Typewriter) Tick() {
	if t.shown >= t.total {
		return
	}
	t.acc += t.rate
	for t.acc >= 1 && t.shown < t.total {
		t.acc--
		t.shown++
	}
}

// Finish reveals the rest at once. This is what a key press does while text is
// still arriving, and it is not optional politeness: a player who has read the
// line already must be able to get past it without waiting out an animation,
// or the effect becomes a tax on rereading anything.
func (t *Typewriter) Finish() { t.shown = t.total }

// Done reports that everything has arrived.
func (t *Typewriter) Done() bool { return t.shown >= t.total }

// Visible is what to draw: the same rows, cut at the character reached.
func (t *Typewriter) Visible() []string { return revealRows(t.rows, t.shown) }

// Reveal cuts already-wrapped rows at the nth character across all of them,
// keeping the row count.
//
// Free-standing because the transcript needs the same cut without owning a
// Typewriter: it wraps at draw time, against a width it is told, so it cannot
// hold rows the way a message box can — but the count of characters it has
// revealed is a property of the sentence and survives being rewrapped.
func Reveal(rows []string, n int) []string {
	rr := make([][]rune, len(rows))
	for i, ln := range rows {
		rr[i] = []rune(ln)
	}
	return revealRows(rr, n)
}

// revealRows is the cut itself.
//
// Rows past the front come back as empty strings rather than being dropped, so
// the caller's y advances by a line for each one and the text stays put as it
// fills. Dropping them would slide every line upward on arrival, which reads as
// the panel scrolling rather than as somebody speaking.
func revealRows(rows [][]rune, n int) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		switch {
		case n >= len(r):
			out[i] = string(r)
			n -= len(r)
		case n > 0:
			out[i] = string(r[:n])
			n = 0
		}
	}
	return out
}

// Len is how many characters the whole text runs to, which a caller counting
// its own progress needs in order to know when to stop.
func (t *Typewriter) Len() int { return t.total }
