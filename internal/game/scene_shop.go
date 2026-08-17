package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
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
	// who is the party member being outfitted. The coin is always the hero's;
	// what it buys need not be. Selling is always out of the hero's pack, since
	// that is where anything worth money ends up.
	who int
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

// gearRow builds one row on a gear counter: name, price, and what it is worth
// against the one already being worn.
func (s *shopScene) gearRow(g *Game, name, icon string, cost int, data any) ui.MenuItem {
	price := g.askingPrice(cost)
	detail := fmt.Sprintf("%d", price)
	var tint color.Color
	if verdict, c := shelfVerdict(s.buyer(g), data); verdict != "" {
		detail = fmt.Sprintf("%s  %s", detail, verdict)
		tint = c
	}
	return ui.MenuItem{
		Label: name, Detail: detail, DetailTint: tint, Icon: icon,
		Disabled: int64(price) > g.Player.Coins, Data: data,
	}
}

func (s *shopScene) refresh(g *Game) {
	var items []ui.MenuItem
	if s.tab == tabBuy {
		tier := s.tier(g)
		weapons, armors := g.Data.StockFor(tier)
		shields, charms := g.Data.SidearmsFor(tier)
		switch s.e.Shop {
		case world.ShopSmith:
			for _, w := range weapons {
				if w.Cost == 0 {
					continue
				}
				items = append(items, s.gearRow(g, w.Name, w.Icon, w.Cost, w))
			}
			// The smith beats metal. Shields are beaten metal, and putting them
			// here is what stops the armourer's list running twice as long as
			// anybody else's.
			for _, sh := range shields {
				items = append(items, s.gearRow(g, sh.Name, sh.Icon, sh.Cost, sh))
			}
		case world.ShopArmorer:
			for _, a := range armors {
				if a.Cost == 0 {
					continue
				}
				items = append(items, s.gearRow(g, a.Name, a.Icon, a.Cost, a))
			}
			// Charms are worn, and the armourer is the one who fits worn things.
			for _, ch := range charms {
				items = append(items, s.gearRow(g, ch.Name, ch.Icon, ch.Cost, ch))
			}
		default: // apothecary
			// The revive stock is the town's answer to somebody going down, and
			// the reason the physician's counter is worth walking back to.
			for _, name := range []string{
				"Small Beer", "Field Poultice", "Physician's Draught",
				"Bottled Nap", "Philosopher's Espresso", "Bitter Root", "Suspicious Pollen",
				"Smelling Salts, Militant", "Still-Warm Heart",
				"Damp Compress", "Broad Antidote",
			} {
				it, ok := g.Data.Item(name)
				if !ok {
					continue
				}
				price := g.askingPrice(it.Value * 2)
				// Quoted before the purchase, not discovered after it. The
				// thief's two-for-one is the class's answer to having no way to
				// heal itself, and a player who never notices it is a player
				// who thinks the class is simply worse.
				detail := fmt.Sprintf("%d", price)
				if rules.SleightOfHand(s.buyer(g), it.Kind) > 1 {
					detail = fmt.Sprintf("%d for two", price)
				}
				items = append(items, ui.MenuItem{
					Label: it.Name, Detail: detail, Icon: it.Icon,
					Disabled: int64(price) > g.Player.Coins, Data: it,
				})
			}
		}
	} else {
		for i, it := range g.Player.Bag {
			items = append(items, ui.MenuItem{
				Label:  fmt.Sprintf("%s x%d", it.Name, it.Count),
				Detail: fmt.Sprintf("%d", sellPrice(it.Value)), Icon: it.Icon,
				Data: sellRow{idx: i},
			})
		}
		// Equipment you are not wearing is worth money like anything else. It
		// had nowhere to be sold from because it had nowhere to be.
		for i, gear := range g.Player.Carried {
			items = append(items, ui.MenuItem{
				Label:  gear.Titled(),
				Detail: fmt.Sprintf("%d", sellPrice(gear.Cost())), Icon: gear.Icon(),
				Data: sellRow{idx: i, gear: true},
			})
		}
	}
	if len(items) == 0 {
		items = append(items, ui.MenuItem{Label: "(nothing doing)", Disabled: true})
	}
	s.menu.Icons = g.Assets
	s.menu.Visible = 7
	s.menu.SetItems(items)
}

// buyer is the party member currently being outfitted.
func (s *shopScene) buyer(g *Game) *model.Character {
	party := g.Party()
	s.who = core.Clamp(s.who, 0, len(party)-1)
	return party[s.who]
}

func (s *shopScene) Update(g *Game) error {
	if g.Back() {
		g.Pop()
		return nil
	}
	// Tab moves the counter along to the next member of the company. It is a
	// separate key rather than another tab because left and right already mean
	// buy and sell, and overloading them would make outfitting a hireling
	// something you discovered by accident.
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) && len(g.Party()) > 1 {
		s.who = (s.who + 1) % len(g.Party())
		s.menu.Index = 0
		g.Sound.Play("ui/page")
		s.note = fmt.Sprintf("The counter turns to %s.", s.buyer(g).Name)
		s.refresh(g)
		return nil
	}
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirDown:
			s.menu.Move(1)
			g.Sound.Play("ui/move")
		case core.DirUp:
			s.menu.Move(-1)
			g.Sound.Play("ui/move")
		case core.DirLeft, core.DirRight:
			g.Sound.Play("ui/page")
			if s.tab == tabBuy {
				s.tab = tabSell
			} else {
				s.tab = tabBuy
			}
			s.menu.Index = 0
			s.refresh(g)
		}
	}
	if g.Accept() {
		it, ok := s.menu.Selected()
		if !ok || it.Disabled {
			g.Sound.Play("ui/deny")
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
	// The coin is the hero's; the gear goes on whoever is at the counter.
	p := s.buyer(g)
	pay := func(n int) { g.Player.Coins -= int64(g.askingPrice(n)) }
	// Bought gear goes in the pack, not straight onto the body.
	//
	// It used to be worn on the spot and whatever came off was described as
	// going "in the bin" — which is exactly what happened to it. Buying a
	// 240-coin glaive silently destroyed the 96-coin spear you were holding.
	// Equipment is a thing you own now, so buying it is buying it and wearing
	// it is a separate decision made on the character sheet, the same as every
	// other thing you can carry.
	carry := func(gear model.Carried, cost int) {
		pay(cost)
		p.Carry(gear)
		s.note = fmt.Sprintf("%s, into the pack. Put it on from the character sheet.",
			upper(gear.Titled()))
		g.Sound.Play("world/equip")
	}

	switch v := it.Data.(type) {
	case model.Weapon:
		carry(model.Carried{Weapon: &v}, v.Cost)
	case model.Armor:
		carry(model.Carried{Armor: &v}, v.Cost)
	case model.Shield:
		carry(model.Carried{Shield: &v}, v.Cost)
	case model.Charm:
		carry(model.Carried{Charm: &v}, v.Cost)
	case model.Item:
		// A thief pays for one restorative and leaves with two. Said out loud
		// rather than left as a quiet extra in the pack, because a perk the
		// game will not talk about is a perk nobody knows they have.
		n := rules.SleightOfHand(p, v.Kind)
		v.Count = n
		pay(v.Value * 2)
		p.AddItem(v)
		switch {
		case n > 1 && p == g.Player:
			s.note = fmt.Sprintf("One %s, wrapped in something. And one that was not wrapped in anything.", v.Name)
		case n > 1:
			s.note = fmt.Sprintf("Two %s for %s, of which one was paid for.", v.Name, p.Name)
		case p == g.Player:
			s.note = fmt.Sprintf("One %s, wrapped in something.", v.Name)
		default:
			// Supplies bought for a companion go in their pack, and they drink
			// them without asking. That is the point of buying them.
			s.note = fmt.Sprintf("One %s, handed straight to %s.", v.Name, p.Name)
		}
		g.Sound.Play("world/buy")
	}
}

// askingPrice is what this particular customer is charged for something worth
// n on the shelf.
//
// It reads how well the face is known rather than what the deeds are worth,
// which is the whole reason those are two numbers. A counter marks up the
// person it recognises, and recognising somebody is not the same as thinking
// well of them — so the legend nobody has placed pays the sticker price and
// the celebrity pays for the privilege of being one.
func (g *Game) askingPrice(n int) int {
	p := int(float64(n) * rules.Read(g.Player).PriceMultiplier())
	if p < 1 {
		p = 1
	}
	return p
}

// sellRow identifies what a row of the sell list refers to: an index into the
// bag, or into the equipment being carried.
type sellRow struct {
	idx  int
	gear bool
}

// sellPrice is what a merchant will hand over for something worth n new.
func sellPrice(n int) int {
	if p := int(float64(n) * sellRate); p > 1 {
		return p
	}
	return 1
}

func (s *shopScene) sell(g *Game, mi ui.MenuItem) {
	row, ok := mi.Data.(sellRow)
	if !ok {
		return
	}
	var name string
	var price int
	if row.gear {
		gear, taken := g.Player.DropCarried(row.idx)
		if !taken {
			return
		}
		name, price = gear.Titled(), sellPrice(gear.Cost())
	} else {
		it, taken := g.Player.TakeItem(row.idx)
		if !taken {
			return
		}
		name, price = it.Name, sellPrice(it.Value)
	}
	g.Player.Coins += int64(price)
	g.Sound.Play("world/coins")
	s.note = fmt.Sprintf("%s, for %d. The shopkeeper does not meet your eye.", name, price)
}

// carriedDescribe says what a carried piece of equipment is worth wearing for,
// reusing the shop's wording so the same sword reads the same on both screens.
func carriedDescribe(c model.Carried) string {
	switch {
	case c.Weapon != nil:
		return shopDescribe(*c.Weapon)
	case c.Armor != nil:
		return shopDescribe(*c.Armor)
	case c.Shield != nil:
		return shopDescribe(*c.Shield)
	case c.Charm != nil:
		return shopDescribe(*c.Charm)
	}
	return ""
}

// shopDescribe renders one line of what a piece of stock is for.
//
// Numbers first, then the flavour: "what does it do" is the question being
// asked at a counter, and the joke in the name has already been read by the
// time anybody looks down here.
func shopDescribe(data any) string {
	switch v := data.(type) {
	case model.Weapon:
		return fmt.Sprintf("Strike %d. %s", v.Strike, bonusWords(v.Affix))
	case model.Armor:
		return fmt.Sprintf("Guard %d. %s", v.Defense, bonusWords(v.Affix))
	case model.Shield:
		out := fmt.Sprintf("Guard %d on the off arm.", v.Defense)
		if v.Extra != nil {
			out += " " + statWords(*v.Extra)
		}
		return out
	case model.Charm:
		out := statWords(v.Bonus)
		if v.Desc != "" {
			out += "  " + v.Desc
		}
		return out
	case model.Item:
		if what := itemPurpose(v); what != "" {
			return upper(what) + ". " + v.Desc
		}
		return v.Desc
	}
	return ""
}

func bonusWords(a *model.Affix) string {
	if a == nil {
		return ""
	}
	return statWords(a.Bonus)
}

// statWords turns a bundle of modifiers into something readable, keeping the
// signs so a trade reads as a trade.
func statWords(b model.Bonus) string {
	parts := make([]string, 0, 7)
	for _, f := range []struct {
		n string
		v int
	}{
		{"strike", b.Strike}, {"guard", b.Defense}, {"str", b.Strength},
		{"dex", b.Dexterity}, {"spd", b.Speed}, {"psyche", b.Psyche},
		{"ward", b.Ward},
	} {
		if f.v != 0 {
			parts = append(parts, fmt.Sprintf("%+d %s", f.v, f.n))
		}
	}
	return strings.Join(parts, ", ")
}

func (s *shopScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xFF})

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

	// Who the counter is serving. Only shown once there is somebody else it
	// could be — a solo run should not be told about a control it cannot use.
	if buyer := s.buyer(g); len(g.Party()) > 1 {
		render.TextCenter(dst, render.Trunc("for "+buyer.Name, 150), render.ScreenW/2, 28, render.ColInk)
	}
	render.Rect(dst, 30, 44, render.ScreenW-60, 1, render.ColInkFaint)

	s.menu.Draw(dst, 40, 52, render.ScreenW-80)

	// What the highlighted thing actually is, live as the cursor moves.
	//
	// The list was a column of names and a column of prices, so the only way to
	// find out what any of it did was to buy it and read the sheet afterwards.
	// A shop that cannot answer "what is this" is a shop you buy the cheapest
	// thing in.
	if s.note == "" {
		if it, ok := s.menu.Selected(); ok {
			if d := shopDescribe(it.Data); d != "" {
				for i, ln := range render.Wrap(d, render.ScreenW-80) {
					if i > 1 {
						break
					}
					render.Text(dst, ln, 34, 176+float64(i)*render.LineH, render.ColInk)
				}
			}
		}
	}

	if s.note != "" {
		for i, ln := range render.Wrap(s.note, render.ScreenW-80) {
			if i > 1 {
				break
			}
			render.Text(dst, ln, 34, 176+float64(i)*render.LineH, render.ColInkDim)
		}
	}
	hint := "left/right switch - Z to deal - X to leave"
	if len(g.Party()) > 1 {
		hint = "left/right switch - Tab: who for - Z to deal - X to leave"
	}
	render.TextCenter(dst, hint, render.ScreenW/2, 226, render.ColInkFaint)
}
