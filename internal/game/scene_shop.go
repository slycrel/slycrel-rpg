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
	// Greyed for what the buyer cannot use as well as for what they cannot
	// afford: it is the same courtesy the inn and the shrine already do, which
	// is never to offer a thing you are about to refuse. Tab turns the counter
	// to somebody else, and a maul greyed out for a mage is available the
	// moment the fighter steps up.
	usable, _ := s.buyer(g).CanUse(carriedOf(data))
	return ui.MenuItem{
		Label: name, Detail: detail, DetailTint: tint, Icon: icon,
		Disabled: int64(price) > g.Player.Coins || !usable, Data: data,
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
			// anybody else's. A talisman is not beaten metal and does not
			// belong on this counter — it goes to the armourer with the other
			// worn things, which is also the only shelf a Mage has any reason
			// to walk up to twice.
			for _, sh := range shields {
				if sh.Barrier() {
					continue
				}
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
			// So are talismans, which are a charm for the arm.
			for _, sh := range shields {
				if !sh.Barrier() {
					continue
				}
				items = append(items, s.gearRow(g, sh.Name, sh.Icon, sh.Cost, sh))
			}
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
				"Bedroll and Some Firewood", "Proper Camp Kit",
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
		// Everything the merchant will take that nothing is waiting on, as the
		// first row rather than as a hotkey.
		//
		// It was very nearly bound to S, which is the down key on WASD — so
		// scrolling this exact list would have emptied the pack. A row costs no
		// keybinding at all, quotes its price in the same column as every other
		// row, and greys itself out when there is nothing to do, which is the
		// rule the inn and the hiring board already follow. It is also the only
		// version of this that is discoverable without a footer telling you.
		if worth, pieces := s.junkWorth(g); pieces > 0 {
			items = append(items, ui.MenuItem{
				Label:  fmt.Sprintf("Everything you are not going to want (%d)", pieces),
				Detail: fmt.Sprintf("%d", worth),
				Data:   sellAll{},
			})
		}
		for i, it := range g.Player.Bag {
			// The price of the whole stack, because the whole stack is what
			// the keypress sells. Quoting the unit price beside an "x9" and
			// then handing over nine times it is the counter misquoting itself.
			items = append(items, ui.MenuItem{
				Label:  fmt.Sprintf("%s x%d", it.Name, it.Count),
				Detail: fmt.Sprintf("%d", sellPrice(it.Value)*core.Max(1, it.Count)),
				Icon:   it.Icon,
				Data:   sellRow{idx: i},
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
		// Whatever the last deal was, it stopped being the answer the moment
		// the cursor moved.
		//
		// The note and the description share one strip of screen, and the note
		// wins while it is set — so a note that was never cleared meant that
		// after a single purchase the shop stopped saying what anything was,
		// permanently, and went on describing the thing you had already bought.
		// The one line that told you what you were looking at was disabled by
		// looking at something.
		s.note = ""
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
	carry := func(gear model.Carried, cost int, data any) {
		pay(cost)
		p.Carry(gear)
		s.note = fmt.Sprintf("%s, into the pack. Put it on from the character sheet.",
			upper(gear.Titled()))
		g.Sound.Play("world/equip")
		s.offerToWear(g, p, gear, data)
	}

	switch v := it.Data.(type) {
	case model.Weapon:
		carry(model.Carried{Weapon: &v}, v.Cost, v)
	case model.Armor:
		carry(model.Carried{Armor: &v}, v.Cost, v)
	case model.Shield:
		carry(model.Carried{Shield: &v}, v.Cost, v)
	case model.Charm:
		carry(model.Carried{Charm: &v}, v.Cost, v)
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

// offerToWear asks, on the spot, about a piece that is plainly better than the
// one being worn.
//
// Buying does not equip — that rule stays, and it is there because equipping on
// purchase used to destroy whatever came off, which is how a 240-coin glaive
// silently ate a 96-coin spear. But the rule was written to stop gear being
// thrown away, not to make putting on a sword a trip to another screen: the
// honest reading of somebody buying a strictly better weapon is that they
// intend to hold it.
//
// So it is offered rather than done, and only when there is nothing to weigh
// up. A worse or equal piece is a real decision — matchups, weight, an affix
// you might be keeping for a reason — and a charm has no better at all by
// construction, so neither gets asked about. Whatever comes off goes into the
// pack the way it always did.
func (s *shopScene) offerToWear(g *Game, p *model.Character, gear model.Carried, data any) {
	if ok, _ := p.CanUse(gear); !ok {
		return
	}
	d, ok := shelfDelta(p, data)
	if !ok || d <= 0 {
		return
	}
	idx := len(p.Carried) - 1
	if idx < 0 {
		return
	}
	g.AskMenu(s.e.Name, fmt.Sprintf("%s, and it is %+d on what %s is holding. Wear it now?",
		upper(gear.Titled()), d, p.Name),
		[]ui.MenuItem{
			{Label: "Put it on", Detail: fmt.Sprintf("%+d", d)},
			{Label: "Just bag it"},
		},
		func(g *Game, choice int) {
			if choice != 0 {
				return
			}
			// Re-found by name rather than trusted by index: the box is modal,
			// but nothing here promises the pack has not been reordered by the
			// time somebody answers.
			if i := carriedIndex(p, gear); i >= 0 && p.Equip(i) {
				g.Sound.Play("world/equip")
				s.note = fmt.Sprintf("%s, on. What came off is in the pack.",
					upper(gear.Titled()))
			}
			s.refresh(g)
		})
}

// carriedIndex finds a piece in somebody's pack by what it is called.
func carriedIndex(p *model.Character, gear model.Carried) int {
	for i, c := range p.Carried {
		if c.Titled() == gear.Titled() {
			return i
		}
	}
	return -1
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

// sellAll tags the row that clears the junk out in one go.
type sellAll struct{}

// junkWorth is what the sweep would fetch, and how many objects it covers.
// Quoted on the row before it is pressed, because a row that does not say what
// it pays is a row nobody presses twice.
func (s *shopScene) junkWorth(g *Game) (worth int, pieces int) {
	wanted := questItems(g)
	for _, it := range g.Player.Bag {
		if it.Kind != model.ItemTrinket || wanted[it.Name] {
			continue
		}
		worth += sellPrice(it.Value) * core.Max(1, it.Count)
		pieces += core.Max(1, it.Count)
	}
	return worth, pieces
}

// questItems names everything an active errand is counting.
func questItems(g *Game) map[string]bool {
	out := map[string]bool{}
	for _, q := range g.Quests.Active() {
		if q.Item != "" {
			out[q.Item] = true
		}
	}
	return out
}

// sellPrice is what a merchant will hand over for something worth n new.
func sellPrice(n int) int {
	if p := int(float64(n) * sellRate); p > 1 {
		return p
	}
	return 1
}

func (s *shopScene) sell(g *Game, mi ui.MenuItem) {
	if _, all := mi.Data.(sellAll); all {
		s.sellJunk(g)
		return
	}
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
		// The whole stack, not one off the top of it.
		//
		// A row that says "Owl Pellet x24" and hands over one owl pellet is a
		// row that has to be pressed twenty-four times, and there is no
		// decision anywhere in the other twenty-three. Anything worth keeping
		// some of is worth keeping all of.
		it, taken := g.Player.TakeStack(row.idx)
		if !taken {
			return
		}
		name, price = it.Name, sellPrice(it.Value)*it.Count
		if it.Count > 1 {
			name = fmt.Sprintf("%s x%d", it.Name, it.Count)
		}
	}
	g.Player.Coins += int64(price)
	g.Sound.Play("world/coins")
	s.note = fmt.Sprintf("%s, for %d. The shopkeeper does not meet your eye.", name, price)
}

// sellJunk empties the pack of everything that is only worth money.
//
// Trinkets only. The kind is documented as sellable junk and that is exactly
// what it is used for, so it is the one category where "all of it" is a safe
// thing for a key to mean — a bulk action that could reach a restorative is a
// bulk action nobody will risk pressing.
//
// And never something an errand is counting. A fetch quest names an item and
// SyncFetch recounts the bag afterwards, so selling the lot would silently
// undo a job in progress and leave the log claiming a number the pack no longer
// backs up. The single-row sell can still do it, because that one is somebody
// deliberately selling a named thing.
func (s *shopScene) sellJunk(g *Game) {
	wanted := questItems(g)

	var kept []model.Item
	var coins int64
	pieces, stacks := 0, 0
	for _, it := range g.Player.Bag {
		if it.Kind != model.ItemTrinket || wanted[it.Name] {
			kept = append(kept, it)
			continue
		}
		coins += int64(sellPrice(it.Value) * it.Count)
		pieces += it.Count
		stacks++
	}
	if stacks == 0 {
		s.note = "Nothing in there but things you are going to want."
		g.Sound.Play("ui/deny")
		return
	}
	g.Player.Bag = kept
	g.Player.Coins += coins
	g.Quests.SyncFetch(g.Player.Bag)
	g.Sound.Play("world/coins")
	s.note = fmt.Sprintf("%d pieces of it, off your hands, for %d. Nobody counts them.",
		pieces, coins)
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
		out := fmt.Sprintf("Strike %d.", v.Strike)
		if v.Focus > 0 {
			// The important number on a rod, and it goes first, because the
			// strike beside it is what the thing is worth as a stick.
			out = fmt.Sprintf("Focus %d, strike %d. Attacks for free with magic.",
				v.Focus, v.Strike)
		}
		if v.TwoHanded() {
			out += " Both hands, so no shield."
		}
		if v.Extra != nil {
			out += " " + statWords(*v.Extra)
		}
		return strings.TrimSpace(out + " " + bonusWords(v.Affix))
	case model.Armor:
		out := fmt.Sprintf("Guard %d.", v.Defense)
		if v.Extra != nil {
			out += " " + statWords(*v.Extra)
		}
		return strings.TrimSpace(out + " " + bonusWords(v.Affix))
	case model.Shield:
		out := fmt.Sprintf("Guard %d on the off arm.", v.Defense)
		if v.Barrier() {
			// A talisman is measured in a different unit from a shield and it
			// has to say so, or "36" next to a plank's "6" reads as six times
			// the shield rather than a different mechanic.
			out = fmt.Sprintf("Soaks %d damage of any kind, once a fight.", v.Absorb)
		}
		if v.Extra != nil {
			out += " " + statWords(*v.Extra) + "."
		}
		if v.Desc != "" {
			out += "  " + v.Desc
		}
		return out
	case model.Charm:
		out := statWords(v.Bonus) + "."
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
				// Three lines. The numbers and the anecdote share this space,
				// and once the off arm had fourteen entries with something to
				// say for themselves, two lines was cutting the last clause off
				// half the shelf with no ellipsis to say so.
				for i, ln := range render.Wrap(d, render.ScreenW-80) {
					if i > 2 {
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
	// The footer says what this tab can do, not what the screen can do. Selling
	// has a key buying does not, and a buyer told about it has been given
	// something to forget before they can use it.
	hint := "left/right switch - Z to deal - X to leave"
	switch {
	case s.tab == tabSell:
		hint = "left/right switch - Z sells the whole stack - X to leave"
	case len(g.Party()) > 1:
		hint = "left/right switch - Tab: who for - Z to deal - X to leave"
	}
	render.TextCenter(dst, hint, render.ScreenW/2, 226, render.ColInkFaint)
}
