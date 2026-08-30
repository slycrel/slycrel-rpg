package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/thread"
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
	// What the shown companion is putting money aside for. Worked out when it
	// can change rather than sixty times a second: asking the tables every
	// frame for an answer that only moves on a keypress is the same mistake
	// the load menu made with the save directory.
	//
	// Two things invalidate it and both are covered. Their gear changing ends
	// in refresh, and the subject changing is caught by wantFor — which is
	// belt and braces on purpose, because `who` is a plain field and the demo
	// tour already sets it directly without going through the paging keys.
	want    gamedata.Want
	saving  bool
	wantFor *model.Character
}

func newStatusScene(g *Game) *statusScene {
	s := &statusScene{under: g.Top()}
	s.refresh(g)
	return s
}

// noteWant caches what the shown member is saving for.
func (s *statusScene) noteWant(g *Game, p *model.Character) {
	s.wantFor = p
	s.want, s.saving = g.Data.Wants(p)
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
	s.noteWant(g, s.subject(g))

	// The panel is always the hero's pack, because that is the pack the player
	// manages. What a companion is carrying shows as a line on their own sheet:
	// two item lists on one screen would mean every key needing to say which
	// one it meant.
	items := make([]ui.MenuItem, 0, len(g.Player.Bag)+len(g.Player.Carried))
	for i, it := range g.Player.Bag {
		items = append(items, ui.MenuItem{
			Label: it.Name, Detail: fmt.Sprintf("x%d", it.Count), Icon: it.Icon,
			Data: packRow{idx: i},
		})
	}
	// Equipment being carried rather than worn, in the same list. This is where
	// gear gets put on: it is a thing you own, so the sheet is where you decide
	// what to do with it, and Z means "use this" for a potion and a sword alike.
	// Greyed for whoever is being looked at rather than hidden: a maul in the
	// pack is still a maul, and a player paging to the fighter should find it
	// waiting rather than wonder where it went. The detail column says the slot
	// when it fits and who it is for when it does not.
	for i, gear := range g.Player.Carried {
		detail := gear.Slot()
		usable, why := s.subject(g).CanUse(gear)
		if !usable {
			detail = why
		}
		items = append(items, ui.MenuItem{
			Label: gear.Titled(), Detail: detail, Icon: gear.Icon(),
			Disabled: !usable,
			Data:     packRow{idx: i, gear: true},
		})
	}
	// Techniques that do something with nobody swinging at you. There was no
	// way to reach these outside a fight at all, which meant the only way to
	// heal between one and the next was to pay an innkeeper — and the innkeeper
	// is expensive at exactly the level where the healing is most needed.
	//
	// Same list and same key: Z means "use this" for a bottle, a sword and a
	// prayer alike.
	for _, sp := range g.Data.SpellsFor(s.subject(g)) {
		if !castableOutOfCombat(sp.Kind) {
			continue
		}
		cost := rules.PsycheCost(s.subject(g), sp)
		items = append(items, ui.MenuItem{
			Label: sp.Name, Detail: fmt.Sprintf("%d SP", cost), Icon: sp.Icon,
			Disabled: cost > s.subject(g).Psyche,
			Data:     packRow{idx: 0, spell: &sp},
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
	// The key that opened this closes it. C and I both open the sheet — I is
	// the habit half the world has for an inventory — so both have to shut it,
	// or one of them is a door that only goes one way.
	if inpututil.IsKeyJustPressed(ebiten.KeyC) || inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.Sound.Play("ui/cancel")
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
				s.bag.Index = 0
				s.refresh(g)
			case core.DirRight:
				s.who = (s.who + 1) % n
				g.Sound.Play("ui/move")
				s.bag.Index = 0
				s.refresh(g)
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

	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		s.askSubject(g)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		s.releaseSubject(g)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		s.give(g)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		s.takeBack(g)
		return nil
	}

	if g.Accept() {
		it, ok := s.bag.Selected()
		if !ok || it.Disabled {
			return nil
		}
		row, ok := it.Data.(packRow)
		if !ok {
			return nil
		}
		switch {
		case row.spell != nil:
			s.castOutOfCombat(g, *row.spell)
		case row.gear:
			s.equipCarried(g, row.idx)
		default:
			s.useOutOfCombat(g, row.idx)
		}
	}
	return nil
}

// packRow says which list a row of the pack panel came from.
type packRow struct {
	idx   int
	gear  bool
	spell *model.Spell
}

// castableOutOfCombat reports whether a technique does anything with nobody
// trying to hit you. Attacks and conditions need a target that is not there;
// blessings are timed and would expire on the walk to the next fight.
func castableOutOfCombat(k model.SpellKind) bool {
	return k == model.SpellHeal || k == model.SpellRevive
}

// castOutOfCombat spends psyche to patch the party up between fights.
//
// The caster is whoever the sheet is showing; the target is whoever needs it
// most, because that is what a party actually does when the fighting stops and
// making the player page to the patient first would be bookkeeping rather than
// a decision.
func (s *statusScene) castOutOfCombat(g *Game, sp model.Spell) {
	caster := s.subject(g)
	cost := rules.PsycheCost(caster, sp)
	if cost > caster.Psyche {
		g.Sound.Play("ui/deny")
		return
	}

	switch sp.Kind {
	case model.SpellRevive:
		var down *model.Character
		for _, c := range g.Party() {
			if !c.Alive() {
				down = c
				break
			}
		}
		if down == nil {
			g.Sound.Play("ui/deny")
			g.Log.AddColor(render.ColInkDim, "Everybody is already upright.")
			return
		}
		caster.Psyche -= cost
		down.HP = rules.ReviveAmount(down, sp.Power)
		g.Sound.Play("fight/heal")
		g.Log.AddColor(render.ColHeal, "%s", g.Write.Revived(g.RNG, down.Name))

	default: // heal
		// Whoever is furthest from full, so the technique is never wasted on
		// somebody who did not need it.
		target := caster
		worst := caster.HPFrac()
		for _, c := range g.Party() {
			if c.Alive() && c.HPFrac() < worst {
				target, worst = c, c.HPFrac()
			}
		}
		if target.HP >= target.MaxHP {
			g.Sound.Play("ui/deny")
			g.Log.AddColor(render.ColInkDim, "Nobody is hurt enough to be worth the psyche.")
			return
		}
		caster.Psyche -= cost
		healed := target.Heal(rules.SpellDamage(g.RNG, caster, sp))
		g.Sound.Play("fight/heal")
		g.Log.AddColor(render.ColHeal, "%s casts %s. %s recovers %d.",
			caster.Name, sp.Name, target.Name, healed)
	}
	s.refresh(g)
}

// equipCarried puts a carried piece of equipment on, and whatever comes off
// goes back in the pack rather than anywhere else. Nothing is destroyed by
// changing your mind, which is the entire reason equipment is carried at all.
func (s *statusScene) equipCarried(g *Game, i int) {
	c := s.subject(g)
	if i < 0 || i >= len(g.Player.Carried) {
		return
	}
	gear := g.Player.Carried[i]
	// Belt and braces: the row is greyed already, but equipCarried is the one
	// door onto the body and a refusal that only lives in a menu is a refusal
	// that stops existing the next time somebody adds a second way in.
	if ok, why := c.CanUse(gear); !ok {
		g.Sound.Play("ui/deny")
		g.Log.AddColor(render.ColInkDim, "%s cannot use %s: %s.", c.Name, gear.Titled(), why)
		return
	}

	// Both asked before the gear lands, because both are questions about the
	// state it is about to replace: what they were putting money aside for,
	// and what they had in that slot already.
	want, saving := g.Data.Wants(c)
	dearer := gear.Cost() > wornCost(c, gear)

	// The hero's pack is the shared one, so equipping onto a companion moves
	// the item out of it and whatever they were wearing back into it.
	if c != g.Player {
		if _, ok := g.Player.DropCarried(i); !ok {
			return
		}
		c.Carry(gear)
		if !c.Equip(len(c.Carried) - 1) {
			return
		}
		// Anything that came off them belongs in the pack the player manages.
		for len(c.Carried) > 0 {
			off, _ := c.DropCarried(0)
			g.Player.Carry(off)
		}
	} else if !g.Player.Equip(i) {
		return
	}

	g.Sound.Play("world/equip")
	g.Log.AddColor(render.ColGold, "%s puts on %s.", c.Name, gear.Titled())
	if c != g.Player {
		s.opinion(g, c, gear, want, saving, dearer)
	}
	s.refresh(g)
}

// wornCost is what a character already has in the slot a piece would go into.
// Read before the swap rather than off whatever comes back out of it: what
// comes off lands in a pack that may already have things in it, and the piece
// at the front of that pack is not necessarily the one just displaced.
func wornCost(c *model.Character, gear model.Carried) int {
	switch {
	case gear.Weapon != nil:
		return c.Weapon.Cost
	case gear.Armor != nil:
		return c.Armor.Cost
	case gear.Shield != nil:
		return c.Shield.Cost
	case gear.Charm != nil:
		return c.Charm.Cost
	}
	return 0
}

// opinion is what a companion has to say about equipment handed to them, and
// where their money is going now that it has landed.
//
// The second half is the useful one. A gift is the one way the player can steer
// a companion's spending, so the line that matters is not "thank you" — it is
// which slot the cut is now aimed at, said the moment the previous answer
// stopped being true.
func (s *statusScene) opinion(g *Game, c *model.Character, gear model.Carried, want gamedata.Want, saving, dearer bool) {
	wanted := saving && want.Gear.Slot() == gear.Slot() && gear.Cost() >= want.Cost
	if said := g.Write.Gift(g.RNG, c.Name, wanted, dearer); said != "" {
		g.Log.AddColor(render.ColInkDim, "%s", said)
	}

	next, ok := g.Data.Wants(c)
	switch {
	case !ok && saving:
		g.Log.AddColor(render.ColInkDim, "%s has nothing left to save for.", c.Name)
	case ok && (!saving || next.Gear.Titled() != want.Gear.Titled()):
		g.Log.AddColor(render.ColInkDim, "The cut goes toward %s now.", next.Gear.Titled())
	}
}

// askSubject asks the shown companion where their own business stands.
//
// Everything a hireling had to say, they said at you on a schedule: beats fire
// as you travel and endings are put in towns. There was no way to ask. That is
// a strange gap in a game with nine authored backstories in it — the player is
// carrying somebody's story around and cannot enquire after it.
//
// It answers the question they are actually being asked, which changes with
// where the story is: a decision if one is waiting, otherwise what they are
// waiting on, and if they have no story at all then something about themselves.
func (s *statusScene) askSubject(g *Game) {
	c := s.subject(g)
	if c == g.Player || !c.Ally || g.Data == nil {
		return
	}
	t := g.Threads.For(&g.Data.Threads, c.Name)
	if t == nil {
		// No story cast — a continent with nothing to stage one in, or a
		// hireling taken on before there were any. They still get a line,
		// because "nothing" is a worse answer than anything.
		if l, ok := model.LineageOf(c.Blood); ok {
			g.Say(c.Name, capitalise(l.Tag)+". "+l.Note+
				"\n\nBeyond that they have nothing they want to get into.")
			return
		}
		g.Say(c.Name, "They have nothing they want to get into, and say so at "+
			"a length that rather undermines it.")
		return
	}
	if t.State == thread.Ready {
		g.offerThreadEnding(t, "")
		return
	}
	note := t.Note(&g.Data.Threads)
	if note == "" {
		g.Say(c.Name, "They would rather not, just now.")
		return
	}
	if p := t.Progress(&g.Data.Threads); p != "" {
		note += "  (" + p + ")"
	}
	g.Sound.Play("ui/page")
	g.Say(c.Name, t.Fill(t.Title)+"\n\n"+note)
}

// releaseSubject lets the shown companion go, with a confirmation, because
// a mistyped key should not cost you the fee you paid for them.
func (s *statusScene) releaseSubject(g *Game) {
	c := s.subject(g)
	if c == g.Player {
		return
	}
	// What walks off with them is said here rather than discovered afterwards.
	// It was always true — dismiss returns the pack and never the body — and it
	// was a footnote while everything a companion wore was issued to them for
	// free. It stopped being one the moment handing somebody your old sword
	// became the fast way to equip them.
	g.Ask("", fmt.Sprintf("Let %s go? The hiring fee does not come back, and neither do they. "+
		"What is in their pack you get back; what they are standing in walks out with them.", c.Name),
		[]string{"Let them go", "Keep them"}, func(g *Game, choice int) {
			if choice != 0 {
				return
			}
			g.dismiss(c)
			s.who = 0
		})
}

// give hands the selected item from the hero's pack to the companion on screen.
//
// Only ever in that direction, and only ever on a companion's sheet, so there
// is never a question of who is giving what to whom. Taking back is a separate
// key that empties their pack, which is coarse but equally unambiguous.
func (s *statusScene) give(g *Game) {
	who := s.subject(g)
	if who == g.Player {
		g.Sound.Play("ui/deny")
		g.Log.AddColor(render.ColInkDim, "Page to somebody to hand them things.")
		return
	}
	it, ok := s.bag.Selected()
	if !ok || it.Disabled {
		return
	}
	row, ok := it.Data.(packRow)
	if !ok {
		return
	}
	if row.gear {
		// Equipment is handed over by putting it on them, which is the same
		// key on the same row and does the same thing.
		g.Log.AddColor(render.ColInkDim, "Equipment is given by wearing it: Z puts it on whoever is shown.")
		return
	}
	idx := row.idx
	if g.Player.Bag[idx].Kind == model.ItemTrinket {
		g.Sound.Play("ui/deny")
		g.Log.AddColor(render.ColInkDim, "%s has no use for a %s and says so.",
			who.Name, g.Player.Bag[idx].Name)
		return
	}
	moved, taken := g.Player.TakeItem(idx)
	if !taken {
		return
	}
	who.AddItem(moved)
	g.Sound.Play("world/loot")
	g.Log.AddColor(render.ColGold, "%s takes the %s, and will drink it without asking.",
		who.Name, moved.Name)
	s.refresh(g)
}

// takeBack empties a companion's pack into the hero's.
func (s *statusScene) takeBack(g *Game) {
	who := s.subject(g)
	if who == g.Player || len(who.Bag) == 0 {
		return
	}
	n := g.reclaim(who)
	g.Sound.Play("world/loot")
	g.Log.AddColor(render.ColGold, "%s hands back %d %s, with a look.",
		who.Name, n, plural(n, "thing", "things"))
	s.refresh(g)
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
	case model.ItemCure:
		// Conditions do not survive a fight, so out here this is always a
		// wasted swallow. Refuse rather than quietly spend it.
		g.Sound.Play("ui/deny")
		g.Log.AddColor(render.ColInkDim, "%s is not suffering from anything a %s would fix.",
			c.Name, g.Player.Bag[idx].Name)
		return
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
	case model.ItemCamp:
		// The sheet closes first: what happens next is a night, possibly with
		// something walking into it, and both of those belong out in the world
		// rather than over an inventory panel.
		it, _ := g.Player.TakeItem(idx)
		g.Pop()
		g.makeCamp(it)
		return
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
	// 208 rather than 200: a hireling's sheet runs to four gear rows under six
	// stat rows, and the old height clipped the last one against the frame.
	// 220 rather than 208: an ally's sheet gained a line for their backstory.
	//
	// 232, and the footer moved to 254 with it, because 220 was not enough
	// either and a frame said so. Eight stat rows is reachable — a caster with
	// an ancestry, a story, something in their pack and now a purse — and the
	// charm row was printing through the bottom of the frame and into the hint
	// underneath it. Counted rather than eyeballed this time: text drawn at y
	// inks to y+12, the four gear rows are the last four slots, so the panel
	// has to end at least fourteen below the last of them.
	ui.TitledPanel(dst, title, 10, 16, 250, 232)

	ui.Slot(dst, 17, 23, 58, 58, nil)
	render.ScreenFit(dst, g.Assets.Get(portraitOf(p)), 0, 18, 24, 56, 56, nil)
	render.Text(dst, p.Name, 82, 26, render.ColGold)
	// What they are saving for goes under their name, where "in your employ"
	// used to sit saying the same thing on every sheet forever. A line that
	// never changes is a line the eye stops reading, and this is the one place
	// the standing charge on every haul can be seen doing something.
	//
	// The slot rather than the thing. Naming it came out as "saving for Staff
	// of the." — the portrait leaves this line 170 pixels and generated gear
	// names are longer than that — and the slot is the more useful half
	// anyway: what the player does with this is go to the pack and hand over
	// something of that kind. The price sits in the cut row underneath, which
	// is the other half of that decision, and the transcript names the actual
	// item when they buy it.
	if p != s.wantFor {
		s.noteWant(g, p)
	}
	subtitle := p.Epithet
	if p.Ally {
		subtitle = "in your employ"
		if s.saving {
			subtitle = "saving for " + slotWords(s.want.Gear)
		}
	}
	render.Text(dst, render.Trunc(subtitle, 168), 82, 26+render.LineH, render.ColInkDim)

	render.Text(dst, fmt.Sprintf("%s, level %d", p.Class, p.Level), 82, 26+2*render.LineH, render.ColInk)

	// Ancestry gets its own line rather than being appended to the trade: the
	// column beside the portrait is 166 pixels, and "Mage, level 1, part demon"
	// came out as "Mage, level 1, part de.".
	// The header's fourth line carries the purse for a hero and the ancestry for
	// a hireling — they are never both — which buys back a row further down for
	// the two new equipment slots.
	y := 92.0
	if l, ok := model.LineageOf(p.Blood); ok {
		render.Text(dst, l.Tag, 82, 26+3*render.LineH, render.ColGold)
		render.Text(dst, render.Trunc(l.Note, 230), 20, 80, render.ColInkFaint)
		y = 96
	} else if !p.Ally {
		render.Text(dst, fmt.Sprintf("%d coins", p.Coins), 82, 26+3*render.LineH, render.ColGold)
		// The line a lineage note would occupy on a companion's sheet. A hero
		// has no ancestry to describe, and what a standing means is worth more
		// than the two numbers that produce it.
		render.Text(dst, render.Trunc(rules.Read(p).Sheet(), 230), 20, 80, render.ColInkFaint)
		y = 96
	}
	next := rules.XPForLevel(p.Level + 1)
	// The three stats share a row so that four equipment slots fit underneath.
	// The numbers shown are the effective ones — what the dice actually use —
	// because a charm that raises strength and a sheet that reports the base
	// would disagree in front of the player.
	rows := [][2]string{
		{"Hit points", fmt.Sprintf("%d / %d", p.HP, p.MaxHP)},
		{"Psyche", fmt.Sprintf("%d / %d", p.Psyche, p.MaxPsy())},
		{"Str / Dex / Spd", fmt.Sprintf("%d / %d / %d", p.Str(), p.Dex(), p.Spd())},
		// Strike and guard are totals across all four slots, which is the only
		// place an affix's contribution can actually be read.
		{"Strike / Guard", fmt.Sprintf("%d / %d", p.Strike(), p.Defense())},
	}
	// Focus only appears on somebody holding something magic, because for
	// everybody else it is a permanent zero — and a stat that is always nothing
	// teaches the player that the row is decoration. On a caster it is the
	// number their whole shopping list is about.
	if p.Casting() {
		rows = append(rows, [2]string{"Focus / Ward", fmt.Sprintf("%d / %d", p.Focus(), p.Ward())})
	}
	// A companion has no purse of their own — what they have instead is a
	// standing claim on yours. Their standing in the world is nobody's concern
	// including theirs, so Fame and Faith are the hero's row alone.
	if p.Ally {
		// The cut and what it has come to, on one row, because either one alone
		// is half a fact: a percentage with nothing behind it is a tax, and a
		// purse with no rate attached does not say where it came from. The
		// panel has no spare rows — see the note on its height — and these two
		// numbers were always one sentence anyway.
		cut := fmt.Sprintf("%d%%", p.Cut)
		switch {
		case s.saving:
			cut = fmt.Sprintf("%d%%, %d of %d", p.Cut, p.Coins, s.want.Cost)
		case p.Coins > 0:
			cut = fmt.Sprintf("%d%%, %d saved", p.Cut, p.Coins)
		}
		rows = append(rows,
			[2]string{"Their cut", cut},
			// What they are carrying, because they drink it without asking and
			// the player is the one who has to decide whether that is enough.
			[2]string{"Carrying", carrying(p)})
		// And what they are dealing with, on the screen that is meant to answer
		// "who is this person". The journal knows, but the journal is the list
		// of things outstanding, which is a different question.
		if t := g.Threads.For(&g.Data.Threads, p.Name); t != nil {
			rows = append(rows, [2]string{"Story", t.Title})
		}
	} else {
		// Both ledgers, a row each. The top one is what the world reads and the
		// player cannot pay down; the bottom one is what the player banked and
		// gets to spend. Which corner the read numbers add up to is the faint
		// line under the portrait — it needs a sentence, not a column.
		rows = append(rows,
			[2]string{"Experience", fmt.Sprintf("%d / %d", p.TotalXP, next)},
			[2]string{"Fame / Renown", fmt.Sprintf("%d / %d", p.Fame, p.Renown)},
			[2]string{"Honor / Faith", fmt.Sprintf("%d / %d", p.Honor, p.Faith)})
	}
	for _, r := range rows {
		render.Text(dst, r[0], 20, y, render.ColInkDim)
		// Truncated to what the label leaves rather than right-aligned straight
		// through it: "The Favour They Cannot Ask For" is wider than the panel.
		avail := 250 - (20 + render.TextW(r[0]) + 8)
		render.TextRight(dst, render.Trunc(r[1], avail), 250, y, render.ColInk)
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
	// Names only. The ratings they add up to are the Strike / Guard row above,
	// so repeating them here only cost the width that made "Mace of Modest
	// Ambition (+5)" truncate to "Mace of Modest Ambition .".
	gearRow("Weapon", fitGear(p.Weapon.Name, p.Weapon.Affix, gearWidth("Weapon")))
	gearRow("Armour", fitGear(p.Armor.Name, p.Armor.Affix, gearWidth("Armour")))
	// The two new slots always show, empty or not. A slot the player does not
	// know exists is a slot they never go shopping for.
	if p.Shield.Worn() {
		gearRow("Shield", fitGear(p.Shield.Name, p.Shield.Affix, gearWidth("Shield")))
	} else {
		gearRow("Shield", "nothing on that arm")
	}
	if p.Charm.Worn() {
		gearRow("Charm", fitGear(p.Charm.Name, p.Charm.Affix, gearWidth("Charm")))
	} else {
		gearRow("Charm", "nothing worn")
	}

	// Pack, ending level with the sheet beside it. Two panels on one screen
	// stopping at different heights reads as one of them having gone wrong,
	// and the sheet's height is set by its worst case rather than by taste —
	// so this is the one that follows.
	ui.TitledPanel(dst, "pack", 268, 16, 202, 232)
	s.bag.Draw(dst, 282, 26, 184)

	if it, ok := s.bag.Selected(); ok && !it.Disabled {
		if row, ok := it.Data.(packRow); ok {
			// Purpose first, then the joke. The description is flavour and the
			// pack is where somebody goes to find out what a thing is for.
			var text string
			switch {
			case row.gear && row.idx < len(g.Player.Carried):
				gear := g.Player.Carried[row.idx]
				text = "Z to put it on. " + carriedDescribe(gear)
			case !row.gear && row.idx < len(p.Bag):
				text = p.Bag[row.idx].Desc
				if what := itemPurpose(p.Bag[row.idx]); what != "" {
					text = upper(what) + ". " + text
				}
			}
			lines := render.Wrap(text, 178)
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

	hint := "Z use or put on - C or X to close"
	if len(party) > 1 {
		hint = "Left / Right for the company - Z use or put on - C to close"
		if p.Ally {
			hint = "L/R - B ask - G give - T take - R let go - X close"
		}
	}
	render.TextCenter(dst, hint, render.ScreenW/2, 254, render.ColInkFaint)
}

// gearWidth is the room a gear row leaves for its value after the label.
func gearWidth(label string) float64 { return 250 - (20 + render.TextW(label) + 8) }

// fitGear renders a piece of equipment into the width available, cutting the
// base name rather than the suffix.
//
// "Flamberge 'The Apology' of the Last Word" is wider than the panel however it
// is laid out, so something has to go — and the suffix is the half that says
// what the thing does. Truncating normally would leave "Flamberge 'The Apo.",
// which is the half the player already knew.
func fitGear(base string, a *model.Affix, width float64) string {
	if a == nil || a.Suffix == "" {
		return render.Trunc(base, width)
	}
	suffix := " " + a.Suffix
	if render.TextW(base+suffix) <= width {
		return base + suffix
	}
	room := width - render.TextW(suffix)
	if room < render.TextW("Mmm.") {
		return render.Trunc(base+suffix, width) // nothing fits; cut it anywhere
	}
	return render.Trunc(base, room) + suffix
}

// carrying summarises a companion's own supplies for their sheet.
func carrying(c *model.Character) string {
	if len(c.Bag) == 0 {
		return "nothing"
	}
	n := 0
	for _, it := range c.Bag {
		n += it.Count
	}
	if len(c.Bag) == 1 {
		return fmt.Sprintf("%s x%d", c.Bag[0].Name, c.Bag[0].Count)
	}
	return fmt.Sprintf("%d things, %d kinds", n, len(c.Bag))
}
