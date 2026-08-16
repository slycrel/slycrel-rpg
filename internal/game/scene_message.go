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

// Ask pushes a message box with choices. onChoose fires after the box closes.
func (g *Game) Ask(speaker, body string, choices []string, onChoose func(*Game, int)) {
	m := &messageScene{
		under:    g.Top(),
		speaker:  speaker,
		body:     render.Wrap(body, render.ScreenW-56),
		choices:  choices,
		onChoose: onChoose,
	}
	items := make([]ui.MenuItem, len(choices))
	for i, c := range choices {
		items[i] = ui.MenuItem{Label: c}
	}
	m.menu.SetItems(items)
	g.Push(m)
}

func (m *messageScene) Update(g *Game) error {
	if len(m.choices) == 0 {
		if g.Accept() || Cancel() {
			g.Pop()
			if m.onChoose != nil {
				m.onChoose(g, 0)
			}
		}
		return nil
	}

	g.MenuNav(&m.menu)
	if g.Accept() {
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
