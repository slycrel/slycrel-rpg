package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// questScene is the journal: the errands you agreed to, and whatever the people
// walking behind you have going on.
//
// They share one screen because they are the same question — what is
// outstanding — even though they are different mechanisms underneath. Two keys
// for two lists would have been two habits to build for one glance.
type questScene struct {
	under Scene
	menu  ui.Menu
}

func newQuestScene(g *Game) *questScene {
	s := &questScene{under: g.Top()}
	// Recount fetch quests on open, so the log never disagrees with the bag.
	g.Quests.SyncFetch(g.Player.Bag)
	s.refresh(g)
	return s
}

func (s *questScene) refresh(g *Game) {
	var items []ui.MenuItem

	// The long story goes on top, because it is the reason to be here and
	// everything under it is something that came up on the way.
	if running := g.Sagas.Running(); len(running) > 0 {
		items = append(items, ui.MenuItem{Label: "the long way round", Header: true})
		for _, sg := range running {
			// Where you are being sent, or how far through the leg you are when
			// the leg has a count. Which one matters depends on the leg, and
			// there is only one column.
			detail := sg.Progress(&g.Data.Sagas)
			if detail == "" {
				detail = sg.PlaceName()
			}
			items = append(items, ui.MenuItem{
				Label: sg.Fill(sg.Title), Detail: detail, Data: sg,
			})
		}
		// The errands need a heading of their own now that something sits above
		// them. Without it the first errand reads as part of the long story —
		// which is exactly how it looked on the frame this was checked against.
		items = append(items, ui.MenuItem{Label: "errands", Header: true})
	}

	first := len(items)
	for _, q := range g.Quests.Active() {
		detail := q.Progress()
		if q.Complete() {
			detail = "ready"
		}
		items = append(items, ui.MenuItem{Label: q.Title, Detail: detail, Data: q})
	}
	if len(items) == first {
		items = append(items, ui.MenuItem{
			Label: "(nobody has asked you for anything)", Disabled: true,
		})
	}

	// Backstories go underneath, behind headings, so one never looks like
	// something a stranger in a town asked for — and under two headings rather
	// than one, because the two kinds are found in different ways. A companion's
	// is with you; a resident's is at an address, and an address is the only
	// useful thing the journal can tell you about it.
	var company, residents []*thread.Thread
	for _, t := range g.Threads.Running() {
		if t.IsResident(&g.Data.Threads) {
			residents = append(residents, t)
		} else {
			company = append(company, t)
		}
	}
	if len(company) > 0 {
		items = append(items, ui.MenuItem{Label: "the company", Header: true})
		for _, t := range company {
			items = append(items, ui.MenuItem{
				Label: t.Title, Detail: g.threadDetail(t), Data: t,
			})
		}
	}
	if len(residents) > 0 {
		items = append(items, ui.MenuItem{Label: "people you have met", Header: true})
		for _, t := range residents {
			// Where they are, rather than how far through they are. A resident
			// only ever tells you the next piece when you are standing in front
			// of them, so the useful thing to know is which town that is.
			items = append(items, ui.MenuItem{
				Label: t.Fill(t.Title), Detail: g.residentJournalLine(t), Data: t,
			})
		}
	}

	s.menu.Visible = questRowsShown
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
	// Z follows whatever is selected. The journal is the only screen that knows
	// about all three kinds of outstanding thing at once, which makes it the
	// only place the choice can be offered without asking the player to hold
	// three lists in their head.
	if g.Accept() {
		it, ok := s.menu.Selected()
		if !ok || it.Disabled {
			return nil
		}
		idx, label, ok := g.destinationOf(it.Data)
		if !ok {
			g.Sound.Play("ui/deny")
			return nil
		}
		g.trackPOI(idx, label)
		g.Sound.Play("ui/page")
		g.Pop()
	}
	return nil
}

// The detail panel under the list, and where its first line of text sits.
//
// Named rather than typed in four places because the errand pane grew a line
// when it learned to say what to do next, and the three panes have to agree
// about where they start or two of them are laid out against a box that has
// moved. It also starts four pixels higher than it used to: the errand pane is
// the tallest of the three and its last row was inking through the bottom
// border, which is not something any test could see and not something a frame
// makes obvious either — it is two pixels of gold on a gold line.
const (
	detailX = 14.0
	// The list above ends at listY+listH and the pane starts below it.
	//
	// The pane took fourteen pixels off the list to get them, and the list had
	// them spare: seven rows at LineH from y+12 fill 96 of the 118 it used to
	// hold. The errand pane is the one that needed them — with a two-line
	// objective and a Where row it had nothing left for the giver's voice, and
	// dropped it silently, which is the failure this whole panel was being
	// rearranged to stop doing in the first place.
	// How many rows the list shows at once.
	questRowsShown = 7

	listY = 16.0
	listH = 104.0

	detailY   = listY + listH + 6
	detailH   = 118.0
	detailTop = detailY + 8
	// The lowest y a row may be drawn at and still have its ink inside the
	// panel. render.Text inks y+2 through y+12, and the border is one pixel.
	detailFloor = detailY + detailH - render.TextInkH - 3
)

func (s *questScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xFF})

	ui.TitledPanel(dst, "things outstanding", detailX, listY, render.ScreenW-2*detailX, listH)
	s.menu.Draw(dst, 28, 28, render.ScreenW-56)

	ui.TitledPanel(dst, "", detailX, detailY, render.ScreenW-2*detailX, detailH)
	if it, ok := s.menu.Selected(); ok && !it.Disabled {
		switch d := it.Data.(type) {
		case *saga.Saga:
			s.drawSaga(g, dst, d)
		case *thread.Thread:
			s.drawThread(g, dst, d)
		case *quest.Quest:
			s.drawQuest(g, dst, d)
		}
	}

	// Say that Z does something here. A key that follows the selected thing and
	// is not mentioned anywhere is a key nobody presses, and this screen is the
	// only place the choice can be made.
	hint := "X to close"
	if it, ok := s.menu.Selected(); ok && !it.Disabled {
		if _, _, has := g.destinationOf(it.Data); has {
			hint = "Z to follow it  -  X to close"
		}
	}
	render.TextCenter(dst, hint, render.ScreenW/2, 250, render.ColInkFaint)
}

// drawSaga fills the detail panel for a long story. Same shape as the other
// two — what, where, how far along — because from the player's side that is
// what all three are.
func (s *questScene) drawSaga(g *Game, dst *ebiten.Image, sg *saga.Saga) {
	y := detailTop
	for i, ln := range render.Wrap(sg.Note(&g.Data.Sagas), render.ScreenW-64) {
		if i > 2 {
			break
		}
		render.Text(dst, ln, 26, y, render.ColInk)
		y += render.LineH
	}
	y += 4

	if n := sg.PlaceName(); n != "" {
		render.Text(dst, "Next", 26, y, render.ColInkDim)
		render.TextRight(dst, n, render.ScreenW-26, y, render.ColInk)
		y += render.LineH
	}
	// How far through, in legs. A spine is five places long and the useful
	// thing to know is which one you are on, not how many creatures are left in
	// whichever of them happens to be a hunt.
	if sk, ok := g.Data.Sagas.Get(sg.Skeleton); ok {
		render.Text(dst, "Stage", 26, y, render.ColInkDim)
		render.TextRight(dst, fmt.Sprintf("%d of %d", sg.At+1, len(sk.Legs)),
			render.ScreenW-26, y, render.ColInk)
		y += render.LineH
	}
	if p := sg.Progress(&g.Data.Sagas); p != "" {
		render.Text(dst, "Progress", 26, y, render.ColInkDim)
		render.TextRight(dst, p, render.ScreenW-26, y, render.ColInk)
		y += render.LineH
	}

	render.Text(dst, "Pays", 26, y, render.ColInkDim)
	render.TextRight(dst, "depends what you decide", render.ScreenW-26, y, render.ColGold)
}

// drawThread fills the detail panel for a companion's backstory. It reads as
// the same shape as an errand — what, where, how far along — because from the
// player's side that is what it is.
func (s *questScene) drawThread(g *Game, dst *ebiten.Image, t *thread.Thread) {
	y := detailTop
	for i, ln := range render.Wrap(t.Note(&g.Data.Threads), render.ScreenW-64) {
		if i > 2 {
			break
		}
		render.Text(dst, ln, 26, y, render.ColInk)
		y += render.LineH
	}
	y += 4

	render.Text(dst, "Whose", 26, y, render.ColInkDim)
	render.TextRight(dst, t.Owner, render.ScreenW-26, y, render.ColInk)
	y += render.LineH

	if t.PlacePOI >= 0 {
		render.Text(dst, "Ends at", 26, y, render.ColInkDim)
		render.TextRight(dst, g.poiName(t.PlacePOI), render.ScreenW-26, y, render.ColInk)
		y += render.LineH
	}

	if p := t.Progress(&g.Data.Threads); p != "" {
		render.Text(dst, "Progress", 26, y, render.ColInkDim)
		render.TextRight(dst, p, render.ScreenW-26, y, render.ColInk)
		y += render.LineH
	}

	render.Text(dst, "Pays", 26, y, render.ColInkDim)
	render.TextRight(dst, "depends what you decide", render.ScreenW-26, y, render.ColGold)
}

func (s *questScene) drawQuest(g *Game, dst *ebiten.Image, q *quest.Quest) {
	y := detailTop

	// What to do next, in gold, above what anybody said about it.
	//
	// The journal used to open with the giver's nag and nothing else — a line
	// in character, which is a fine thing to have and is not an instruction.
	// A player coming back after a week got "Still 4 Chitin Scrap. The number
	// has not changed. I would have mentioned." and had to work out from it
	// what they were supposed to physically do, where, and how much of it was
	// left. The objective says that in one sentence and the nag stays
	// underneath, because the two are not redundant: one is what somebody said
	// to you and the other is what you wrote down afterwards.
	for i, ln := range render.Wrap(q.Objective(), render.ScreenW-64) {
		if i > 1 {
			break
		}
		render.Text(dst, ln, 26, y, render.ColGold)
		y += render.LineH
	}
	y += 2

	// What the giver says about it, dim and underneath, in whatever room is
	// left once the rows below have taken theirs.
	//
	// Budgeted rather than fixed, because the number of rows varies: an errand
	// pointing at a place has a Where row and one happening in a region does
	// not, and a fixed one-line allowance spent the spare line on nothing and
	// truncated the voice mid-word — "the fields outside Bastion of th." —
	// while a fixed two-line allowance pushed the reward through the bottom
	// border on the errands that do have the row.
	rows := 3 // asked by, progress, pays
	if q.TargetName != "" {
		rows++
	}
	budget := int((detailFloor - y - 4 - float64(rows)*render.LineH) / render.LineH)
	body := q.NagLine()
	if q.Complete() {
		body = q.Thank
	}
	for i, ln := range render.Wrap(body, render.ScreenW-64) {
		if i >= budget {
			break
		}
		render.Text(dst, ln, 26, y, render.ColInkDim)
		y += render.LineH
	}
	y += 4

	render.Text(dst, "Asked by", 26, y, render.ColInkDim)
	render.TextRight(dst, fmt.Sprintf("%s, %s", q.Giver, g.poiName(q.GiverPOI)),
		render.ScreenW-26, y, render.ColInk)
	y += render.LineH

	// Where, only for the errands that point at a place you can be sent to.
	//
	// A fetch and a cull happen in a region, and the objective above already
	// names it — putting it in a row underneath as well printed the same long
	// generated place name twice in a panel four inches wide. A delve and a
	// delivery point at a POI, which is a different fact: it is the thing Z
	// follows, and it belongs beside the other facts rather than only inside a
	// sentence.
	if q.TargetName != "" {
		render.Text(dst, "Where", 26, y, render.ColInkDim)
		render.TextRight(dst, render.Trunc(q.TargetName, render.ScreenW-110),
			render.ScreenW-26, y, render.ColInk)
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

// poiName resolves a stored location index for display.
func (g *Game) poiName(idx int) string {
	if g.World == nil || idx < 0 || idx >= len(g.World.POIs) {
		return "somewhere"
	}
	return g.World.POIs[idx].Name
}

// threadDetail is the journal's right-hand column for a companion's story.
//
// Thread.Progress deliberately answers only for the counted triggers — a fight
// tally is worth showing and a step count is not — which left every other beat
// with a blank column and a player looking at a title with nothing beside it.
// The two most common are the ones that most need saying: a story waiting for
// you to walk somewhere reads as a story that has stalled, when in fact it is
// the only kind with a specific answer.
//
// Distance rather than the place name, in tiles, for the same reason the
// tracker uses it: the name is already on the row you get by selecting this
// one, the compass is already pointing at it, and a name long enough to be
// interesting is long enough to eat the title it sits beside.
func (g *Game) threadDetail(t *thread.Thread) string {
	if p := t.Progress(&g.Data.Threads); p != "" {
		return p
	}
	switch t.Awaiting(&g.Data.Threads) {
	case thread.Reach:
		// Only for Reach. A beat that fires on walking into any town may still
		// have a {P} cast in its text, and answering it with a distance would
		// point confidently at somewhere the story is not asking you to go.
		if g.World != nil && t.PlacePOI >= 0 && t.PlacePOI < len(g.World.POIs) {
			d := g.World.POIs[t.PlacePOI].Pos.Manhattan(g.Walk.Tile)
			if d == 0 {
				return "here"
			}
			return fmt.Sprintf("%d away", d)
		}
	case thread.Town:
		return "any town"
	case thread.Travel:
		return "on the road"
	}
	return ""
}
