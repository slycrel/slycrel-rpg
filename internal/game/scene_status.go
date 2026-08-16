package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// statusScene is the character sheet and pack, on one screen because flipping
// between two of them is how you lose track of what you own.
type statusScene struct {
	under Scene
	bag   ui.Menu
}

func newStatusScene(g *Game) *statusScene {
	s := &statusScene{under: g.Top()}
	s.refresh(g)
	return s
}

func (s *statusScene) refresh(g *Game) {
	items := make([]ui.MenuItem, 0, len(g.Player.Bag))
	for i, it := range g.Player.Bag {
		items = append(items, ui.MenuItem{
			Label: it.Name, Detail: fmt.Sprintf("x%d", it.Count), Data: i,
		})
	}
	if len(items) == 0 {
		items = append(items, ui.MenuItem{Label: "(nothing but lint)", Disabled: true})
	}
	s.bag.Visible = 8
	s.bag.SetItems(items)
}

func (s *statusScene) Update(g *Game) error {
	if Cancel() {
		g.Pop()
		return nil
	}
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirDown:
			s.bag.Move(1)
		case core.DirUp:
			s.bag.Move(-1)
		}
	}
	if Confirm() {
		it, ok := s.bag.Selected()
		if !ok || it.Disabled {
			return nil
		}
		idx := it.Data.(int)
		s.useOutOfCombat(g, idx)
	}
	return nil
}

// useOutOfCombat handles the item uses that make sense while walking around.
func (s *statusScene) useOutOfCombat(g *Game, idx int) {
	if idx >= len(g.Player.Bag) {
		return
	}
	switch g.Player.Bag[idx].Kind {
	case model.ItemHeal:
		it, _ := g.Player.TakeItem(idx)
		n := g.Player.Heal(it.Power)
		g.Log.AddColor(render.ColHeal, "%s: %d hit points back.", it.Name, n)
	case model.ItemPsyche:
		it, _ := g.Player.TakeItem(idx)
		before := g.Player.Psyche
		g.Player.Psyche = core.Clamp(g.Player.Psyche+it.Power, 0, g.Player.MaxPsyche)
		g.Log.AddColor(render.ColMagic, "%s: %d psyche back.", it.Name, g.Player.Psyche-before)
	default:
		g.Log.AddColor(render.ColInkDim, "%s is for selling, not for drinking.", g.Player.Bag[idx].Name)
	}
	s.refresh(g)
}

func (s *statusScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xEE})

	p := g.Player
	ui.TitledPanel(dst, "the person in question", 10, 16, 250, 200)

	render.ScreenFit(dst, g.Assets.Get("portrait/male/m_01"), 0, 18, 24, 56, 56, nil)
	render.Text(dst, p.Name, 82, 26, render.ColGold)
	render.Text(dst, p.Epithet, 82, 26+render.LineH, render.ColInkDim)
	render.Text(dst, fmt.Sprintf("%s, level %d", p.Class, p.Level), 82, 26+2*render.LineH, render.ColInk)

	y := 92.0
	next := rules.XPForLevel(p.Level + 1)
	rows := [][2]string{
		{"Hit points", fmt.Sprintf("%d / %d", p.HP, p.MaxHP)},
		{"Psyche", fmt.Sprintf("%d / %d", p.Psyche, p.MaxPsyche)},
		{"Strength", fmt.Sprint(p.Strength)},
		{"Dexterity", fmt.Sprint(p.Dexterity)},
		{"Speed", fmt.Sprint(p.Speed)},
		{"Experience", fmt.Sprintf("%d / %d", p.TotalXP, next)},
		{"Coins", fmt.Sprint(p.Coins)},
		{"Fame / Faith", fmt.Sprintf("%d / %d", p.Fame, p.Faith)},
	}
	for _, r := range rows {
		render.Text(dst, r[0], 20, y, render.ColInkDim)
		render.TextRight(dst, r[1], 250, y, render.ColInk)
		y += render.LineH
	}

	y += 4
	render.Text(dst, "Wielding", 20, y, render.ColInkDim)
	render.TextRight(dst, fmt.Sprintf("%s (+%d)", p.Weapon.Name, p.Weapon.Strike), 250, y, render.ColGold)
	y += render.LineH
	render.Text(dst, "Wearing", 20, y, render.ColInkDim)
	render.TextRight(dst, fmt.Sprintf("%s (+%d)", p.Armor.Name, p.Armor.Defense), 250, y, render.ColGold)

	// Pack.
	ui.TitledPanel(dst, "pack", 268, 16, 202, 200)
	s.bag.Draw(dst, 282, 26, 184)

	if it, ok := s.bag.Selected(); ok && !it.Disabled {
		if idx, ok := it.Data.(int); ok && idx < len(p.Bag) {
			lines := render.Wrap(p.Bag[idx].Desc, 178)
			ty := 26 + s.bag.Height() + 10
			for i, ln := range lines {
				if i > 3 {
					break
				}
				render.Text(dst, ln, 282, ty, render.ColInkDim)
				ty += render.LineH
			}
		}
	}

	render.TextCenter(dst, "Z to use - X to close", render.ScreenW/2, 232, render.ColInkFaint)
}
