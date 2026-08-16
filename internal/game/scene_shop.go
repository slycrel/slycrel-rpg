package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// shopTab is which side of the counter the player is on.
type shopTab int

const (
	tabBuy shopTab = iota
	tabSell
)

// sellRate is the fraction of an item's value a merchant will pay. Merchants
// are not charities and the game should not pretend otherwise.
const sellRate = 0.45

// shopScene is buying and selling, one merchant at a time.
type shopScene struct {
	under Scene
	e     *world.Entity
	tab   shopTab
	menu  ui.Menu
	note  string
}

func newShopScene(g *Game, e *world.Entity) *shopScene {
	s := &shopScene{under: g.Top(), e: e}
	s.refresh(g)
	return s
}

// tier scales stock with the size of the settlement, so the capital is worth
// walking back to.
func (s *shopScene) tier(g *Game) int {
	switch g.Local.POI.Kind {
	case world.KindCapital:
		return 5
	case world.KindTown:
		return 3
	case world.KindCastle:
		return 4
	default:
		return 2
	}
}

func (s *shopScene) refresh(g *Game) {
	var items []ui.MenuItem
	if s.tab == tabBuy {
		weapons, armors := g.Data.StockFor(s.tier(g))
		switch s.e.Shop {
		case world.ShopSmith:
			for _, w := range weapons {
				if w.Cost == 0 {
					continue
				}
				items = append(items, ui.MenuItem{
					Label: w.Name, Detail: fmt.Sprintf("%d", w.Cost),
					Disabled: int64(w.Cost) > g.Player.Coins, Data: w,
				})
			}
		case world.ShopArmorer:
			for _, a := range armors {
				if a.Cost == 0 {
					continue
				}
				items = append(items, ui.MenuItem{
					Label: a.Name, Detail: fmt.Sprintf("%d", a.Cost),
					Disabled: int64(a.Cost) > g.Player.Coins, Data: a,
				})
			}
		default: // apothecary
			for _, name := range []string{
				"Small Beer", "Field Poultice", "Physician's Draught",
				"Bottled Nap", "Philosopher's Espresso", "Bitter Root", "Suspicious Pollen",
			} {
				it, ok := g.Data.Item(name)
				if !ok {
					continue
				}
				price := it.Value * 2
				items = append(items, ui.MenuItem{
					Label: it.Name, Detail: fmt.Sprintf("%d", price),
					Disabled: int64(price) > g.Player.Coins, Data: it,
				})
			}
		}
	} else {
		for i, it := range g.Player.Bag {
			price := int(float64(it.Value) * sellRate)
			if price < 1 {
				price = 1
			}
			items = append(items, ui.MenuItem{
				Label:  fmt.Sprintf("%s x%d", it.Name, it.Count),
				Detail: fmt.Sprintf("%d", price), Data: i,
			})
		}
	}
	if len(items) == 0 {
		items = append(items, ui.MenuItem{Label: "(nothing doing)", Disabled: true})
	}
	s.menu.Visible = 8
	s.menu.SetItems(items)
}

func (s *shopScene) Update(g *Game) error {
	if Cancel() {
		g.Pop()
		return nil
	}
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirDown:
			s.menu.Move(1)
		case core.DirUp:
			s.menu.Move(-1)
		case core.DirLeft, core.DirRight:
			if s.tab == tabBuy {
				s.tab = tabSell
			} else {
				s.tab = tabBuy
			}
			s.menu.Index = 0
			s.refresh(g)
		}
	}
	if Confirm() {
		it, ok := s.menu.Selected()
		if !ok || it.Disabled {
			return nil
		}
		if s.tab == tabBuy {
			s.buy(g, it)
		} else {
			s.sell(g, it)
		}
		s.refresh(g)
	}
	return nil
}

func (s *shopScene) buy(g *Game, it ui.MenuItem) {
	p := g.Player
	switch v := it.Data.(type) {
	case model.Weapon:
		p.Coins -= int64(v.Cost)
		old := p.Weapon
		p.Weapon = v
		s.note = fmt.Sprintf("You take the %s. The %s goes in the bin.", v.Name, old.Name)
	case model.Armor:
		p.Coins -= int64(v.Cost)
		old := p.Armor
		p.Armor = v
		s.note = fmt.Sprintf("You put on the %s. The %s is not missed.", v.Name, old.Name)
	case model.Item:
		v.Count = 1
		p.Coins -= int64(v.Value * 2)
		p.AddItem(v)
		s.note = fmt.Sprintf("One %s, wrapped in something.", v.Name)
	}
}

func (s *shopScene) sell(g *Game, mi ui.MenuItem) {
	idx, ok := mi.Data.(int)
	if !ok {
		return
	}
	it, taken := g.Player.TakeItem(idx)
	if !taken {
		return
	}
	price := int(float64(it.Value) * sellRate)
	if price < 1 {
		price = 1
	}
	g.Player.Coins += int64(price)
	s.note = fmt.Sprintf("%s, for %d. The shopkeeper does not meet your eye.", it.Name, price)
}

func (s *shopScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xEE})

	title := s.e.Name
	ui.TitledPanel(dst, title, 20, 18, render.ScreenW-40, 196)

	tabs := []string{"Buy", "Sell"}
	for i, t := range tabs {
		c := render.ColInkDim
		if shopTab(i) == s.tab {
			c = render.ColGold
		}
		render.Text(dst, t, 34+float64(i)*46, 28, c)
	}
	render.TextRight(dst, fmt.Sprintf("%d coins", g.Player.Coins), render.ScreenW-34, 28, render.ColGold)
	render.Rect(dst, 30, 44, render.ScreenW-60, 1, render.ColInkFaint)

	s.menu.Draw(dst, 40, 52, render.ScreenW-80)

	if s.note != "" {
		for i, ln := range render.Wrap(s.note, render.ScreenW-80) {
			if i > 1 {
				break
			}
			render.Text(dst, ln, 34, 176+float64(i)*render.LineH, render.ColInkDim)
		}
	}
	render.TextCenter(dst, "left/right switch - Z to deal - X to leave",
		render.ScreenW/2, 226, render.ColInkFaint)
}
