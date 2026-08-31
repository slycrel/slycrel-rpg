package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// messageScene is the modal box used for signs, dialogue, chests, and any
// "here is what just happened, press a key" moment. It draws the scene beneath
// it so the world stays visible behind the box.
type messageScene struct {
	under   Scene
	speaker string
	body    []string
	// choices, when non-empty, turns the box into a prompt. onChoose receives
	// the selected index; -1 means the player backed out.
	choices  []string
	menu     ui.Menu
	onChoose func(g *Game, choice int)
	// portrait and role turn the box into a conversation rather than a
	// notice. Empty for a sign, a chest, or the game telling you something:
	// those have no face, and drawing an empty frame beside them would be
	// worse than the plain box they get now.
	portrait string
	role     string
}

// Say pushes a plain message box over the current scene.
func (g *Game) Say(speaker, body string) {
	g.Push(&messageScene{
		under:   g.Top(),
		speaker: speaker,
		body:    render.Wrap(body, render.ScreenW-56),
	})
}

// SayAs is Say for a person: it carries a face and, when there is one, a word
// for what they are.
//
// The wrapping width is narrower than Say's because the portrait takes the left
// third of the panel. Getting that wrong is not subtle — the text runs under
// the frame and out the other side, since a panel does not clip what you draw
// in it.
func (g *Game) SayAs(name, role, face, body string) {
	g.Push(&messageScene{
		under:    g.Top(),
		speaker:  name,
		role:     role,
		portrait: face,
		body:     render.Wrap(body, talkTextW),
	})
}

// AskAs is AskMenu for a person, with the same face and the same width.
func (g *Game) AskAs(name, role, face, body string, items []ui.MenuItem, onChoose func(*Game, int)) {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	m := &messageScene{
		under:    g.Top(),
		speaker:  name,
		role:     role,
		portrait: face,
		body:     render.Wrap(body, talkTextW),
		choices:  labels,
		onChoose: onChoose,
	}
	m.menu.SetItems(items)
	g.Push(m)
}

// SayThen pushes a plain message box and runs then once it is dismissed.
//
// It exists so that one event can produce two boxes in the order they happened:
// a chest reports what was in it, and only then asks about the thing with a
// name on it. Pushing both at once would show them back to front.
func (g *Game) SayThen(speaker, body string, then func(*Game)) {
	g.Push(&messageScene{
		under:    g.Top(),
		speaker:  speaker,
		body:     render.Wrap(body, render.ScreenW-56),
		onChoose: func(g *Game, _ int) { then(g) },
	})
}

// Ask pushes a message box with choices. onChoose fires after the box closes.
func (g *Game) Ask(speaker, body string, choices []string, onChoose func(*Game, int)) {
	items := make([]ui.MenuItem, len(choices))
	for i, c := range choices {
		items[i] = ui.MenuItem{Label: c}
	}
	g.AskMenu(speaker, body, items, onChoose)
}

// AskMenu is Ask with the rows built by the caller, so a choice can carry a
// price in its detail column or be greyed out entirely.
//
// Ask only took strings, which meant a box could offer a thing and then refuse
// it: the ending of a backstory quoted a price, let you select it, and only
// then said you could not afford it. A menu that can say no in advance is the
// difference between a choice and a trick, and the same gap is why a section
// header in a list has to be a disabled row with dashes around it.
//
// onChoose receives the row index, which counts disabled rows too, so it lines
// up with what the caller passed in.
func (g *Game) AskMenu(speaker, body string, items []ui.MenuItem, onChoose func(*Game, int)) {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	m := &messageScene{
		under:    g.Top(),
		speaker:  speaker,
		body:     render.Wrap(body, render.ScreenW-56),
		choices:  labels,
		onChoose: onChoose,
	}
	m.menu.SetItems(items)
	g.Push(m)
}

func (m *messageScene) Update(g *Game) error {
	if len(m.choices) == 0 {
		// Anything at all closes a box that is only reporting. See Keystroke.
		if g.Dismiss() {
			g.Pop()
			if m.onChoose != nil {
				m.onChoose(g, 0)
			}
		}
		return nil
	}

	g.MenuNav(&m.menu)
	if g.Accept() {
		// A disabled row is an answer the caller has already said no to; taking
		// it would put the refusal after the decision, which is the thing
		// AskMenu exists to stop.
		if it, ok := m.menu.Selected(); ok && it.Disabled {
			g.Sound.Play("ui/deny")
			return nil
		}
		i := m.menu.Index
		g.Pop()
		if m.onChoose != nil {
			m.onChoose(g, i)
		}
	} else if g.Back() {
		g.Pop()
		if m.onChoose != nil {
			m.onChoose(g, -1)
		}
	}
	return nil
}

func (m *messageScene) Draw(g *Game, dst *ebiten.Image) {
	if m.under != nil {
		m.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, render.ColShadow)

	// A face means a person, and a person gets the big layout.
	if m.portrait != "" {
		m.drawTalk(g, dst)
		return
	}

	lines := len(m.body)
	h := float64(lines)*render.LineH + 26
	if len(m.choices) > 0 {
		h += m.menu.Height() + 6
	}
	if h < 54 {
		h = 54
	}
	y := render.ScreenH - h - 14
	ui.TitledPanel(dst, m.speaker, 16, y, render.ScreenW-32, h)

	ty := y + 10
	for _, ln := range m.body {
		render.Text(dst, ln, 26, ty, render.ColInk)
		ty += render.LineH
	}
	if len(m.choices) > 0 {
		m.menu.Draw(dst, 34, ty+4, render.ScreenW-80)
	} else if (g.Tick()/24)%2 == 0 {
		render.TextRight(dst, "v", render.ScreenW-28, y+h-16, render.ColGold)
	}
}

// The conversation layout.
//
// A person talking gets most of the screen, because the alternative — the
// bottom strip every message shared — gave a quest-giver exactly the same
// presence as a signpost. The panel stops just above the status bar rather than
// covering it: the strip says where you are and what you are carrying, and both
// are things you want while deciding whether to take a job.
const (
	talkX = 12
	talkY = 16
	talkW = render.ScreenW - 2*talkX
	// talkMaxH ends above the HUD rather than over it: the strip says where you
	// are and what you are carrying, and both are things you want in hand while
	// deciding whether to take a job.
	talkMaxH = render.ScreenH - hudH - talkY - 6

	// The portrait pane. 76 is as large as the frame goes without the text
	// column dropping below the width a sentence needs to not read as poetry.
	faceSize = 76
	facePad  = 12

	// Text starts to the right of the portrait and wraps well short of the
	// panel edge, since ui.Panel draws a border this must not run into.
	talkTextX = talkX + facePad + faceSize + 12
	talkTextW = talkW - (talkTextX - talkX) - 18

	// The caption's own column, which is wider than the portrait it sits under
	// and shorter than the text column it sits beside.
	//
	// Wrapping it to the frame was the obvious thing and the wrong one: at 76px
	// the word "apothecary," is 77 and overflows on its own, so a whole class of
	// caption could never fit however short the rest of it was. The left column
	// runs to where the body text starts and nothing else is drawn in it, so the
	// caption may have all of it.
	captionW  = talkTextX - talkX - facePad - 6
	roleLines = 3
)

// drawTalk renders the conversation layout: a face on the left, what they said
// on the right, and the choice underneath.
// talkHeight is the panel's height: enough for the face, enough for the words,
// and no more.
//
// Fixed height was tried first and looked worse than the strip it replaced. A
// two-line exchange in a panel sized for eight is a face at the top, a menu
// pinned to the bottom, and a hundred pixels of nothing between them — the
// screen was being used rather than filled. So it grows with the content and
// stops at the face's own height, which is the floor a conversation cannot go
// below anyway.
func (m *messageScene) talkHeight() float64 {
	text := float64(len(m.body))*render.LineH + 24
	if len(m.choices) > 0 {
		text += m.menu.Height() + 10
	}
	face := float64(faceSize) + 30
	if m.role != "" {
		n := len(render.Wrap(m.role, captionW))
		if n > roleLines {
			n = roleLines
		}
		face += float64(n) * render.LineH
	}
	h := text
	if face > h {
		h = face
	}
	if h > talkMaxH {
		h = talkMaxH
	}
	return h
}

func (m *messageScene) drawTalk(g *Game, dst *ebiten.Image) {
	h := m.talkHeight()
	ui.TitledPanel(dst, m.speaker, talkX, talkY, talkW, h)

	fx, fy := float64(talkX+facePad), float64(talkY+18)
	ui.Slot(dst, fx, fy, faceSize, faceSize, render.ColInkDim)
	if sp := g.Assets.Get(m.portrait); sp != nil {
		render.ScreenFit(dst, sp, 0, fx+2, fy+2, faceSize-4, faceSize-4, nil)
	}

	// What they are, under the face. A word rather than a sentence: the name is
	// already the title, and this is the thing the player is placing them by.
	//
	// Wrapped to the portrait's width over two lines rather than truncated to
	// one. "undead mage" is eleven characters against a box that fits ten, and
	// cutting it produced "undead ma." — which is not a shorter way of saying
	// it, it is a different thing that happens to start the same.
	ry := fy + faceSize + 4
	for i, ln := range render.Wrap(m.role, captionW) {
		if i >= roleLines {
			break
		}
		render.Text(dst, ln, fx, ry, render.ColInkDim)
		ry += render.LineH
	}

	ty := float64(talkY + 20)
	for _, ln := range m.body {
		render.Text(dst, ln, talkTextX, ty, render.ColInk)
		ty += render.LineH
	}

	// The choice sits under the text, not under the portrait, so the eye goes
	// down one column instead of crossing back.
	if len(m.choices) > 0 {
		m.menu.Draw(dst, talkTextX+8, ty+6, talkW-(talkTextX-talkX)-26)
	} else if (g.Tick()/24)%2 == 0 {
		render.TextRight(dst, "v", talkX+talkW-12, talkY+h-16, render.ColGold)
	}
}
