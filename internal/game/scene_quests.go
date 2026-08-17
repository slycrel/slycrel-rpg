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
	for _, q := range g.Quests.Active() {
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

	// The long story goes on top, above the errands, because it is the reason
	// to be here and everything else is something that came up.
	if running := g.Sagas.Running(); len(running) > 0 {
		var head []ui.MenuItem
		for _, sg := range running {
			detail := sg.Progress(&g.Data.Sagas)
			if detail == "" {
				detail = sg.PlaceName()
			}
			head = append(head, ui.MenuItem{
				Label: sg.Fill(sg.Title), Detail: detail, Data: sg,
			})
		}
		items = append(append([]ui.MenuItem{
			{Label: "the long way round", Header: true}}, head...), items...)
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
				Label: t.Title, Detail: t.Progress(&g.Data.Threads), Data: t,
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

	ui.TitledPanel(dst, "things outstanding", 14, 16, render.ScreenW-28, 118)
	s.menu.Draw(dst, 28, 28, render.ScreenW-56)

	ui.TitledPanel(dst, "", 14, 144, render.ScreenW-28, 96)
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

	render.TextCenter(dst, "X to close", render.ScreenW/2, 250, render.ColInkFaint)
}

// drawSaga fills the detail panel for a long story. Same shape as the other
// two — what, where, how far along — because from the player's side that is
// what all three are.
func (s *questScene) drawSaga(g *Game, dst *ebiten.Image, sg *saga.Saga) {
	y := 152.0
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
	y := 152.0
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

// poiName resolves a stored location index for display.
func (g *Game) poiName(idx int) string {
	if g.World == nil || idx < 0 || idx >= len(g.World.POIs) {
		return "somewhere"
	}
	return g.World.POIs[idx].Name
}
