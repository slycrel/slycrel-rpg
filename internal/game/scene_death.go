package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/render"
)

// deathScene is the black the run ends on.
//
// It does nothing and takes no input. Its whole job is to be underneath the
// question, because the battle screen has already faded itself out by the time
// this appears and without something holding that black the world underneath
// would draw straight back in — the fade would be undone in the frame after it
// finished, which is worse than not fading at all.
type deathScene struct{}

func (s *deathScene) Update(g *Game) error { return nil }

func (s *deathScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0, 0, 0, 0xFF})
	// One line, high and faint, so the box below it is the thing being read.
	render.TextCenter(dst, "you died", render.ScreenW/2, 60, render.ColBlood)
}
