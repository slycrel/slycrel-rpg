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
	{"Z", "talk, enter, confirm"},
	{"X", "back out"},
	{"", ""},
	{"C or I", "character sheet, pack, techniques"},
	{"J", "journal: errands, stories, who to see"},
	{"M", "map"},
	{"Esc", "pause: save, load, sound"},
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
	"After dark things hit harder. An inn buys the morning.",
	"Dying offers the moment before the fight. Take it.",
}

func (s *helpScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xF0})

	ui.TitledPanel(dst, "how this works", 14, 14, render.ScreenW-28, 118)
	y := 26.0
	for _, r := range helpKeys {
		if r.key == "" {
			y += 4
			continue
		}
		render.Text(dst, r.key, 26, y, render.ColGold)
		render.Text(dst, r.what, 140, y, render.ColInk)
		y += render.LineH
	}

	ui.TitledPanel(dst, "things nobody tells you", 14, 138, render.ScreenW-28, 100)
	y = 150
	for _, n := range helpNotes {
		for _, ln := range render.Wrap(n, render.ScreenW-64) {
			render.Text(dst, ln, 26, y, render.ColInkDim)
			y += render.LineH
		}
		y += 4
	}

	render.TextCenter(dst, "X to close", render.ScreenW/2, 248, render.ColInkFaint)
}
