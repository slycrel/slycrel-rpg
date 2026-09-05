package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// helpScene is the list of what the keys do.
//
// It exists because the first playthrough found four separate things by not
// finding them: saving, the quest log, how to hand a quest in, and how to heal
// without paying an innkeeper. None of those were missing — the pause menu has
// Save, J opens the log, the log names who to take it back to, and the pack is
// full of drink. They were unreachable, because the status bar has room for
// about twenty characters and was spending them on two of the six keys.
//
// So the bar now advertises one key, this one, and this screen carries the rest.
type helpScene struct {
	under Scene
}

func newHelpScene(g *Game) *helpScene { return &helpScene{under: g.Top()} }

func (s *helpScene) Update(g *Game) error {
	if g.Back() || g.Accept() || ebiten.IsKeyPressed(ebiten.KeyH) && Cancel() {
		g.Pop()
	}
	return nil
}

// helpRow is a key and what it does. Grouped rather than alphabetical: the
// order is roughly what a new player needs first.
type helpRow struct{ key, what string }

var helpKeys = []helpRow{
	{"Arrows / WASD", "walk"},
	{"Z", "confirm; walking into a thing already does it"},
	{"X", "back out"},
	{"", ""},
	{"C or I", "character sheet, pack, techniques"},
	{"J", "journal: errands, stories, who to see"},
	{"T", "talk to the people you hired"},
	{"M", "map"},
	{"Esc", "pause: save, load, settings"},
	{"\\", "save a screenshot"},
}

// helpNotes are the things a keyboard layout cannot tell you.
//
// Kept to what a player cannot work out by pressing things, and kept to one
// line each: the panel holds five, and a note that wraps costs another note its
// place. Every one was either something a playthrough had to guess at or
// something added since that has no other way of announcing itself.
//
// One line means 59 characters, not 69. The font is basicfont.Face7x13 — a
// *seven pixel* fixed advance — so the 416-pixel wrap is 59 columns. Estimating
// it instead of dividing put every note below on two lines and the last one at
// y=274 on a 270-pixel screen, which is to say off it.
var helpNotes = []string{
	"Hand an errand back to whoever asked. J says who.",
	"Z in the journal aims the arrow in the corner.",
	"C heals: potions and techniques, one list.",
	"After dark things hit harder. A bed buys back the morning.",
	"T asks a companion about the thing they stopped saying.",
}

func (s *helpScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xF0})

	keysH, notesH := helpHeights()

	ui.TitledPanel(dst, "how this works", helpX, helpTop, render.ScreenW-2*helpX, keysH)
	y := float64(helpTop + helpInset)
	for _, r := range helpKeys {
		if r.key == "" {
			y += helpGap
			continue
		}
		render.Text(dst, r.key, helpX+12, y, render.ColGold)
		render.Text(dst, r.what, 140, y, render.ColInk)
		y += render.LineH
	}

	notesTop := float64(helpTop) + keysH + 6
	ui.TitledPanel(dst, "things nobody tells you", helpX, notesTop, render.ScreenW-2*helpX, notesH)
	y = notesTop + helpInset
	for _, n := range helpNotes {
		for _, ln := range render.Wrap(n, helpNoteW) {
			render.Text(dst, ln, helpX+12, y, render.ColInkDim)
			y += render.LineH
		}
		y += helpGap
	}

	render.TextCenter(dst, "X to close", render.ScreenW/2, notesTop+notesH+10, render.ColInkFaint)
}

// The help screen measures itself.
//
// Both panels used to be fixed heights that happened to fit the lists in them,
// which is the arrangement where adding one row is silently a bug: the tenth
// key ran six pixels past the bottom of its own frame and into the panel below.
// Nothing warns you, because a panel is a rectangle and text is text, and
// neither has an opinion about the other.
const (
	helpX     = 14
	helpTop   = 14
	helpInset = 12 // above the first row, and again below the last
	helpGap   = 4  // between groups of keys, and between notes
	// helpNoteW is the wrap. The font is basicfont.Face7x13 — a *seven pixel*
	// fixed advance — so this is 59 columns, and a note written to 69 comes out
	// on two lines and costs the note below it its place.
	helpNoteW = render.ScreenW - 64
)

// helpHeights is how tall each panel has to be for what is in it.
func helpHeights() (keys, notes float64) {
	keys = helpInset * 2
	for _, r := range helpKeys {
		if r.key == "" {
			keys += helpGap
			continue
		}
		keys += render.LineH
	}
	notes = helpInset*2 - helpGap
	for _, n := range helpNotes {
		notes += float64(len(render.Wrap(n, helpNoteW)))*render.LineH + helpGap
	}
	return keys, notes
}

// helpBottom is where the last thing on the screen is drawn, which is the one
// number the layout has to keep under render.ScreenH.
func helpBottom() float64 {
	keys, notes := helpHeights()
	return helpTop + keys + 6 + notes + 10
}
