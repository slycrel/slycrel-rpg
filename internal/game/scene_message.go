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
	portrait string
}

// Say pushes a plain message box over the current scene.
func (g *Game) Say(speaker, body string) {
	g.Push(&messageScene{
		under:   g.Top(),
		speaker: speaker,
		body:    render.Wrap(body, render.ScreenW-56),
	})
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
