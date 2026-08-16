package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// statusScene is the character sheet and pack, on one screen because flipping
// between two of them is how you lose track of what you own.
//
// With a party it is also the roster: left and right page through the company
// and redraw the same sheet for whoever is showing. One screen that changes
// subject beats a separate party screen listing everything twice.
type statusScene struct {
	under Scene
	bag   ui.Menu
	// who indexes into the party. The pack below stays the hero's whatever is
	// on show — companions do not have pockets of their own — but a potion is
	// spent on the member you are looking at.
	who int
}

func newStatusScene(g *Game) *statusScene {
	s := &statusScene{under: g.Top()}
	s.refresh(g)
	return s
}

// subject is the member whose sheet is on screen.
func (s *statusScene) subject(g *Game) *model.Character {
	party := g.Party()
	if len(party) == 0 {
		return g.Player
	}
	s.who = core.Clamp(s.who, 0, len(party)-1)
	return party[s.who]
}

func (s *statusScene) refresh(g *Game) {
	items := make([]ui.MenuItem, 0, len(g.Player.Bag))
	for i, it := range g.Player.Bag {
		items = append(items, ui.MenuItem{
			Label: it.Name, Detail: fmt.Sprintf("x%d", it.Count), Icon: it.Icon, Data: i,
		})
	}
	if len(items) == 0 {
		items = append(items, ui.MenuItem{Label: "(nothing but lint)", Disabled: true})
	}
	s.bag.Icons = g.Assets
	s.bag.Visible = 7
	s.bag.SetItems(items)
}

func (s *statusScene) Update(g *Game) error {
	if g.Back() {
		g.Pop()
		return nil
	}

	// Left and right page through the company; up and down stay with the bag.
	if d, ok := MenuDir(); ok {
		if n := len(g.Party()); n > 1 {
			switch d {
			case core.DirLeft:
				s.who = (s.who - 1 + n) % n
				g.Sound.Play("ui/move")
			case core.DirRight:
				s.who = (s.who + 1) % n
				g.Sound.Play("ui/move")
			}
		}
		switch d {
		case core.DirDown:
			s.bag.Move(1)
			g.Sound.Play("ui/move")
		case core.DirUp:
			s.bag.Move(-1)
			g.Sound.Play("ui/move")
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		s.releaseSubject(g)
		return nil
	}

	if g.Accept() {
		it, ok := s.bag.Selected()
		if !ok || it.Disabled {
			return nil
		}
		idx := it.Data.(int)
		s.useOutOfCombat(g, idx)
	}
	return nil
}

// releaseSubject lets the shown companion go, with a confirmation, because
// a mistyped key should not cost you the fee you paid for them.
func (s *statusScene) releaseSubject(g *Game) {
	c := s.subject(g)
	if c == g.Player {
		return
	}
	g.Ask("", fmt.Sprintf("Let %s go? The hiring fee does not come back, and neither do they.", c.Name),
		[]string{"Let them go", "Keep them"}, func(g *Game, choice int) {
			if choice != 0 {
				return
			}
			g.dismiss(c)
			s.who = 0
		})
}

// useOutOfCombat handles the item uses that make sense while walking around.
// The item comes out of the hero's pack and goes into whoever is on screen,
// which is how a companion gets patched up between fights.
func (s *statusScene) useOutOfCombat(g *Game, idx int) {
	if idx >= len(g.Player.Bag) {
		return
	}
	c := s.subject(g)
	switch g.Player.Bag[idx].Kind {
	case model.ItemHeal:
		it, _ := g.Player.TakeItem(idx)
		n := c.Heal(it.Power)
		g.Sound.Play("fight/heal")
		g.Log.AddColor(render.ColHeal, "%s: %d hit points back for %s.", it.Name, n, c.Name)
	case model.ItemPsyche:
		it, _ := g.Player.TakeItem(idx)
		before := c.Psyche
		c.Psyche = core.Clamp(c.Psyche+it.Power, 0, c.MaxPsyche)
		g.Log.AddColor(render.ColMagic, "%s: %d psyche back for %s.", it.Name, c.Psyche-before, c.Name)
	case model.ItemRevive:
		// The company picks each other up once the fighting stops, so out here
		// this is a spare rather than a necessity. Say so instead of spending it.
		if c.Alive() {
			g.Sound.Play("ui/deny")
			g.Log.AddColor(render.ColInkDim, "%s is standing up already. Save it.", c.Name)
			return
		}
		it, _ := g.Player.TakeItem(idx)
		c.HP = rules.ReviveAmount(c, it.Power)
		g.Sound.Play("fight/heal")
		g.Log.AddColor(render.ColHeal, "%s", g.Write.Revived(g.RNG, c.Name))
	default:
		g.Sound.Play("ui/deny")
		g.Log.AddColor(render.ColInkDim, "%s is for selling, not for drinking.", g.Player.Bag[idx].Name)
	}
	s.refresh(g)
}

func (s *statusScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xFF})

	p := s.subject(g)
	party := g.Party()

	title := "the person in question"
	if len(party) > 1 {
		title = fmt.Sprintf("the company (%d of %d)", s.who+1, len(party))
	}
	ui.TitledPanel(dst, title, 10, 16, 250, 200)

	render.ScreenFit(dst, g.Assets.Get(portraitOf(p)), 0, 18, 24, 56, 56, nil)
	render.Text(dst, p.Name, 82, 26, render.ColGold)
	subtitle := p.Epithet
	if p.Ally {
		subtitle = "in your employ"
	}
	render.Text(dst, subtitle, 82, 26+render.LineH, render.ColInkDim)

	render.Text(dst, fmt.Sprintf("%s, level %d", p.Class, p.Level), 82, 26+2*render.LineH, render.ColInk)

	// Ancestry gets its own line rather than being appended to the trade: the
	// column beside the portrait is 166 pixels, and "Mage, level 1, part demon"
	// came out as "Mage, level 1, part de.".
	y := 92.0
	if l, ok := model.LineageOf(p.Blood); ok {
		render.Text(dst, l.Tag, 82, 26+3*render.LineH, render.ColGold)
		// The note runs the full width of the panel, under the portrait, where
		// there is room for it. The numbers the lineage moved are already
		// folded invisibly into the stats, so this line is what says why.
		render.Text(dst, render.Trunc(l.Note, 230), 20, 80, render.ColInkFaint)
		y = 96
	}
	next := rules.XPForLevel(p.Level + 1)
	rows := [][2]string{
		{"Hit points", fmt.Sprintf("%d / %d", p.HP, p.MaxHP)},
		{"Psyche", fmt.Sprintf("%d / %d", p.Psyche, p.MaxPsyche)},
		{"Strength", fmt.Sprint(p.Strength)},
		{"Dexterity", fmt.Sprint(p.Dexterity)},
		{"Speed", fmt.Sprint(p.Speed)},
		{"Experience", fmt.Sprintf("%d / %d", p.TotalXP, next)},
	}
	// A companion has no purse of their own — what they have instead is a
	// standing claim on yours. Their standing in the world is nobody's concern
	// including theirs, so Fame and Faith are the hero's row alone; dropping it
	// is also what buys back the line the lineage note needs.
	if p.Ally {
		rows = append(rows, [2]string{"Their cut", fmt.Sprintf("%d%%", p.Cut)})
	} else {
		rows = append(rows,
			[2]string{"Coins", fmt.Sprint(p.Coins)},
			[2]string{"Fame / Faith", fmt.Sprintf("%d / %d", p.Fame, p.Faith)})
	}
	for _, r := range rows {
		render.Text(dst, r[0], 20, y, render.ColInkDim)
		render.TextRight(dst, r[1], 250, y, render.ColInk)
		y += render.LineH
	}

	// Gear names run long — "Blade of Escalating Poor Decisions" is wider than
	// the panel — so the value is truncated to whatever the label leaves rather
	// than being right-aligned straight through it.
	y += 4
	gearRow := func(label, value string) {
		render.Text(dst, label, 20, y, render.ColInkDim)
		avail := 250 - (20 + render.TextW(label) + 8)
		render.TextRight(dst, render.Trunc(value, avail), 250, y, render.ColGold)
		y += render.LineH
	}
	gearRow("Wielding", fmt.Sprintf("%s (+%d)", p.Weapon.Name, p.Weapon.Strike))
	gearRow("Wearing", fmt.Sprintf("%s (+%d)", p.Armor.Name, p.Armor.Defense))

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

	hint := "Z to use - X to close"
	if len(party) > 1 {
		hint = "Left / Right for the company - Z to use - X to close"
		if p.Ally {
			hint = "Left / Right for the company - Z to use - R to let go - X to close"
		}
	}
	render.TextCenter(dst, hint, render.ScreenW/2, 232, render.ColInkFaint)
}
