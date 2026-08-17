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
	{"C", "character sheet and pack"},
	{"J", "quest log"},
	{"M", "map"},
	{"Esc", "pause: save, load, sound"},
}

// helpNotes are the things a keyboard layout cannot tell you, taken straight
// from what the first playthrough had to guess at.
var helpNotes = []string{
	"Hand a quest in by going back to whoever asked. The log (J) names them " +
		"and where they are.",
	"Heal out of the pack with C, not just by sleeping. Potions and drink " +
		"work walking around, not only in a fight.",
	"Dying offers you the moment before the fight. Taking it is not cheating; " +
		"it is the only thing an encounter roll cannot take from you.",
}

func (s *helpScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xF0})

	ui.TitledPanel(dst, "how this works", 14, 14, render.ScreenW-28, 108)
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

	ui.TitledPanel(dst, "things nobody tells you", 14, 128, render.ScreenW-28, 106)
	y = 140
	for _, n := range helpNotes {
		for _, ln := range render.Wrap(n, render.ScreenW-64) {
			render.Text(dst, ln, 26, y, render.ColInkDim)
			y += render.LineH
		}
		y += 4
	}

	render.TextCenter(dst, "X to close", render.ScreenW/2, 248, render.ColInkFaint)
}
