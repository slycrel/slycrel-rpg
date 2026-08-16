package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// questScene is the errand list: what you agreed to, how far along it is, and
// where it has to go back to.
type questScene struct {
	under Scene
	menu  ui.Menu
	list  []*quest.Quest
}

func newQuestScene(g *Game) *questScene {
	s := &questScene{under: g.Top()}
	// Recount fetch quests on open, so the log never disagrees with the bag.
	g.Quests.SyncFetch(g.Player.Bag)
	s.refresh(g)
	return s
}

func (s *questScene) refresh(g *Game) {
	s.list = g.Quests.Active()
	items := make([]ui.MenuItem, 0, len(s.list))
	for _, q := range s.list {
		detail := q.Progress()
		if q.Complete() {
			detail = "ready"
		}
		items = append(items, ui.MenuItem{Label: q.Title, Detail: detail, Data: q})
	}
	if len(items) == 0 {
		items = append(items, ui.MenuItem{
			Label: "(nobody has asked you for anything)", Disabled: true,
		})
	}
	s.menu.Visible = 7
	s.menu.SetItems(items)
}

func (s *questScene) Update(g *Game) error {
	if g.Back() {
		g.Pop()
		return nil
	}
	if ebiten.IsKeyPressed(ebiten.KeyJ) && Cancel() {
		g.Pop()
		return nil
	}
	g.MenuNav(&s.menu)
	return nil
}

func (s *questScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xFF})

	ui.TitledPanel(dst, "things you agreed to", 14, 16, render.ScreenW-28, 118)
	s.menu.Draw(dst, 28, 28, render.ScreenW-56)

	ui.TitledPanel(dst, "", 14, 144, render.ScreenW-28, 96)
	if it, ok := s.menu.Selected(); ok && !it.Disabled {
		q := it.Data.(*quest.Quest)
		y := 152.0

		body := q.Nag
		if q.Complete() {
			body = q.Thank
		}
		for i, ln := range render.Wrap(body, render.ScreenW-64) {
			if i > 2 {
				break
			}
			render.Text(dst, ln, 26, y, render.ColInk)
			y += render.LineH
		}
		y += 4

		render.Text(dst, "Asked by", 26, y, render.ColInkDim)
		render.TextRight(dst, fmt.Sprintf("%s, %s", q.Giver, g.poiName(q.GiverPOI)),
			render.ScreenW-26, y, render.ColInk)
		y += render.LineH

		if q.TargetName != "" {
			render.Text(dst, "Where", 26, y, render.ColInkDim)
			render.TextRight(dst, q.TargetName, render.ScreenW-26, y, render.ColInk)
			y += render.LineH
		}
		render.Text(dst, "Progress", 26, y, render.ColInkDim)
		col := render.ColInk
		if q.Complete() {
			col = render.ColGold
		}
		render.TextRight(dst, q.Progress(), render.ScreenW-26, y, col)
		y += render.LineH

		render.Text(dst, "Pays", 26, y, render.ColInkDim)
		render.TextRight(dst, fmt.Sprintf("%d coins, %d experience", q.RewardCoins, q.RewardXP),
			render.ScreenW-26, y, render.ColGold)
	}

	render.TextCenter(dst, "X to close", render.ScreenW/2, 250, render.ColInkFaint)
}

// poiName resolves a stored location index for display.
func (g *Game) poiName(idx int) string {
	if g.World == nil || idx < 0 || idx >= len(g.World.POIs) {
		return "somewhere"
	}
	return g.World.POIs[idx].Name
}
