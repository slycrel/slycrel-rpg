package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Marking the people who have something for you.
//
// A settlement draws eight or ten identical villagers and exactly one of them
// is holding the errand. Which one is decided by a hash of where they are
// standing, which is stable and invisible, so the only way to find them was to
// walk into all of them — and after the friction pass made walking into things
// the way you use them, that is a town you clear by bumping every person in it.
//
// The mark is a star rather than the exclamation point every game since 1998
// has used. Same job, and it does not read as somebody else's icon.

// attentionKind is what somebody has, which decides the colour.
type attentionKind int

const (
	attentionNone attentionKind = iota
	// attentionOffer is something available if you want it: an errand nobody
	// has taken, a story nobody has started, somebody waiting to be hired.
	attentionOffer
	// attentionOwed is something already yours, waiting on you specifically —
	// a finished errand to hand in, or an installment somebody has been holding
	// since you left. Gold, and worth crossing a street for.
	attentionOwed
)

// attention reports what the person under the cursor is holding.
//
// Every branch is a lookup or a hash and nothing here may have side effects:
// this runs for every visible entity on every frame. residentThread in
// particular must never be called from it — that one *casts* a story and writes
// a line to the log, which from a draw call would mean a town narrating itself
// at sixty lines a second.
func (g *Game) attention(e *world.Entity, poiIdx int) attentionKind {
	if e.Used || poiIdx < 0 || g.Local == nil || g.Player == nil {
		return attentionNone
	}
	switch e.Kind {
	case world.ERecruit:
		// Somebody sitting at the end of an inn's bar waiting to be paid is the
		// clearest case of all: they are there *for* this.
		return attentionOffer
	case world.ENPC:
	default:
		return attentionNone
	}

	// An errand this person gave you, either finished or not.
	if q := g.Quests.From(poiIdx, e.Name); q != nil {
		if q.Complete() {
			return attentionOwed
		}
		// Deliberately nothing. A job you are in the middle of is a job you
		// already know about, and marking it would put a star over the one
		// person in town whose news you have heard.
		return attentionNone
	}

	// A story this person is in the middle of telling.
	if t := g.Threads.ForResident(&g.Data.Threads, poiIdx, e.Name); t != nil {
		if t.Say() != "" || t.State == thread.Ready {
			return attentionOwed
		}
		return attentionNone
	}

	// Something on offer. Both of these are capped at one per settlement by the
	// code that grants them, so the mark is capped the same way — see
	// firstWith. Marking every villager whose hash says yes would put three
	// stars in a town that can only produce one errand, which is not a hint,
	// it is a lie with a shape.
	if g.Quests.CountActive() < maxActiveQuests && !g.Quests.HasFrom(poiIdx) &&
		g.firstWith(poiIdx, (*Game).wantsToAsk) == e {
		return attentionOffer
	}
	if g.runningResidents() < residentCap && !g.hasResidentIn(poiIdx) &&
		g.firstWith(poiIdx, storyTeller) == e {
		return attentionOffer
	}
	return attentionNone
}

// firstWith returns the first person in the interior for whom want is true.
//
// "First" is by position in the entity list, which is generated from the
// location's seed and is therefore the same on every visit — the same property
// the hashes themselves rely on. Any stable tiebreak would do; this one is free.
func (g *Game) firstWith(poiIdx int, want func(*Game, *world.Entity, int) bool) *world.Entity {
	for _, e := range g.Local.Entities {
		if e.Kind == world.ENPC && !e.Used && want(g, e, poiIdx) {
			return e
		}
	}
	return nil
}

// storyTeller adapts hasStory to the shape firstWith wants. hasStory does not
// take a location index because a person's story is cast from where they stand.
func storyTeller(g *Game, e *world.Entity, _ int) bool { return g.hasStory(e) }

// hasResidentIn reports whether this settlement already has a story running,
// which is the same one-per-town rule residentThread enforces.
func (g *Game) hasResidentIn(poiIdx int) bool {
	for _, t := range g.Threads.Threads {
		if t.HomePOI == poiIdx && t.IsResident(&g.Data.Threads) {
			return true
		}
	}
	return false
}

// starGlyph is a five-pointed star, hand-set for the same reason the compass
// arrowheads are: at seven pixels a rasterised star is a smudge, and drawing it
// by hand is quicker than getting a rasteriser to agree it is a star.
var starGlyph = []string{
	"...#...",
	"..###..",
	"#######",
	".#####.",
	"..###..",
	".##.##.",
	"##...##",
}

// starBob is the two-pixel hover, in frames. Movement is most of what makes a
// seven-pixel mark findable over a street of moving villagers — a still glyph
// this size reads as part of whatever it is standing in front of.
var starBob = []float64{0, -1, -2, -1}

// drawAttention paints the mark over somebody's head.
//
// Not while their own label is naming them. A tag sits in exactly this space —
// a one-line plate occupies the sixteen pixels above the tile, which is where a
// mark over somebody's head has to go — and the two drawn together is the star
// buried under the name.
//
// Which is the right way round rather than a compromise. The mark exists to be
// read from across a street; the name appears when you are close enough to be
// told it. So the star is what you get *until* the label can tell you more.
//
// It stands its ground when a label is up, and moves out of the way instead.
//
// The first version stood down, on the grounds that a tag occupies exactly the
// sixteen pixels above a head and two marks for one fact is one mark too many.
// That is true of the *fact* and wrong about the mark: a star is what the eye
// catches while crossing a street, and having it wink out at four tiles means
// the thing you were walking towards stops being marked the moment you commit
// to walking towards it. The name is not a replacement for the star, it is the
// answer to a different question.
//
// So when a tag is up the star is lifted clear of the plate rather than
// suppressed by it, which is what the geometry was really asking for.
func (g *Game) drawAttention(ctx *render.Ctx, e *world.Entity, kind attentionKind, poiIdx int) {
	if kind == attentionNone {
		return
	}
	// Gold, both of them, because gold is the one colour on this palette that
	// nothing in the world is.
	//
	// It went pale blue first on the grounds that gold already meant "yours"
	// and the two states wanted telling apart — and pale blue on grass is a
	// mark you find by looking for it, which is the one thing a mark must not
	// be. The states are told apart by brightness and by movement instead: what
	// is merely on offer sits still in plain gold, and what is waiting on you
	// pulses to something near white.
	col := color.RGBA{0xE0, 0xB0, 0x4C, 0xFF}
	if kind == attentionOwed && (g.Tick()/20)%2 == 0 {
		col = color.RGBA{0xFF, 0xF0, 0xC0, 0xFF}
	}

	const ts = assetsys.TileSize
	ox, oy := ctx.Cam.Offset()
	// Above the artwork, not above the tile.
	//
	// Character art is drawn into a generous box and anchored on its feet, so
	// how far a person's head is above the square they are standing on depends
	// entirely on which sheet they were drawn from — the townsfolk are about a
	// tile, the winged hirelings better than two. A fixed offset put the star
	// on the tall ones' faces. Sprite.Head is where the ink actually starts,
	// measured off the image, which is the same answer Foot gives at the other
	// end for the same reason.
	top := float64(e.Pos.Y*ts + ts) // the tile's floor, where a sprite stands
	if sp := g.Assets.Get(e.Sprite); sp != nil && e.Sprite != "" {
		top -= float64(sp.H - sp.Head)
	} else {
		top -= ts
	}
	x := float64(e.Pos.X*ts) + ox + (ts-7)/2
	y := top + oy - 5

	// Clear of the tag, when there is one. A plate hangs off the top of the
	// tile rather than off the top of the artwork, so a short sprite's star sits
	// squarely inside it and a tall one's is already well above — which is why
	// this is a ceiling rather than an offset.
	if g.labelShowing(e, poiIdx) {
		plateTop := float64(e.Pos.Y*ts) + oy - tagPlateH
		y = core.MinF(y, plateTop-9)
	}
	y += starBob[(g.Tick()/8)%len(starBob)]

	// A hard shadow down and to the right, then the star over it.
	//
	// Offset rather than a ring, and offset both ways rather than straight
	// down: a one-pixel outline all round a seven-pixel glyph eats the glyph,
	// and a shadow directly underneath reads as a thicker star rather than as
	// depth. Two pixels of dark along the bottom-right is what lifts it off
	// whatever it happens to be floating over, which at this size is usually a
	// hedge.
	drawGlyph(ctx.Dst, starGlyph, x+2, y+2, color.RGBA{0x10, 0x0C, 0x14, 0xD8})
	drawGlyph(ctx.Dst, starGlyph, x, y, col)
}

// drawGlyph paints a hand-set bitmap at x,y. The compass has its own copy of
// this loop because its glyphs are indexed by direction; this one takes the
// rows directly, which is all a single-shape glyph needs.
func drawGlyph(dst *ebiten.Image, rows []string, x, y float64, c color.Color) {
	for row, line := range rows {
		for col, ch := range line {
			if ch == '#' {
				render.Rect(dst, x+float64(col), y+float64(row), 1, 1, c)
			}
		}
	}
}
