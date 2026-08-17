package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// slotNames are the fixed save slots. Fixed slots rather than free-text names
// because typing a filename with a d-pad is nobody's idea of a good time.
var slotNames = []string{"1", "2", "3"}

// slotMode is whether the screen is writing or reading.
type slotMode int

const (
	slotSave slotMode = iota
	slotLoad
)

// slotScene is the save/load picker, used both from the pause menu and from
// the title screen's Continue.
type slotScene struct {
	under Scene
	mode  slotMode
	menu  ui.Menu
	note  string
}

func newSlotScene(g *Game, mode slotMode) *slotScene {
	s := &slotScene{under: g.Top(), mode: mode}
	s.refresh(g)
	return s
}

func (s *slotScene) refresh(g *Game) {
	existing := map[string]save.Slot{}
	for _, sl := range save.List(g.Root) {
		existing[sl.Name] = sl
	}

	items := make([]ui.MenuItem, 0, len(slotNames))
	for _, name := range slotNames {
		sl, ok := existing[name]
		label := "Slot " + name
		detail := "empty"
		if ok {
			label = sl.Summary
			detail = humanAge(sl.Saved)
		}
		items = append(items, ui.MenuItem{
			Label:    render.Trunc(label, 300),
			Detail:   detail,
			Disabled: s.mode == slotLoad && !ok,
			Data:     name,
		})
	}
	s.menu.Visible = 0
	s.menu.SetItems(items)
}

// humanAge renders a save's age the way a player thinks about it.
func humanAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2 Jan")
	}
}

func (s *slotScene) Update(g *Game) error {
	if g.Back() {
		g.Pop()
		return nil
	}
	g.MenuNav(&s.menu)
	if !g.Accept() {
		return nil
	}

	it, ok := s.menu.Selected()
	if !ok || it.Disabled {
		return nil
	}
	slot := it.Data.(string)

	if s.mode == slotSave {
		if err := g.SaveTo(slot); err != nil {
			s.note = "Could not save: " + err.Error()
			return nil
		}
		s.note = "Saved to slot " + slot + "."
		s.refresh(g)
		return nil
	}

	if err := g.LoadFrom(slot); err != nil {
		s.note = "Could not load: " + err.Error()
		return nil
	}
	// Restore has already rebuilt the stack, so this scene is gone with it.
	return nil
}

func (s *slotScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xFF})

	title := "save"
	if s.mode == slotLoad {
		title = "load"
	}
	ui.TitledPanel(dst, title, 30, 62, render.ScreenW-60, 116)
	s.menu.Draw(dst, 46, 78, render.ScreenW-92)

	if s.note != "" {
		for i, ln := range render.Wrap(s.note, render.ScreenW-100) {
			if i > 1 {
				break
			}
			render.Text(dst, ln, 46, 142+float64(i)*render.LineH, render.ColInkDim)
		}
	}
	render.TextCenter(dst, "Z to confirm - X to go back", render.ScreenW/2, 192, render.ColInkFaint)
}

// pauseScene is the in-game menu. Escape opens it from the overworld or from
// inside a location.
type pauseScene struct {
	under Scene
	menu  ui.Menu
}

func newPauseScene(g *Game) *pauseScene {
	p := &pauseScene{under: g.Top()}
	p.refresh(g)
	return p
}

// refresh rebuilds the rows so the sound line reflects the current setting.
func (p *pauseScene) refresh(g *Game) {
	idx := p.menu.Index
	p.menu.SetItems([]ui.MenuItem{
		{Label: "Resume"},
		{Label: "Sound", Detail: soundLabel(g)},
		{Label: "Save"},
		{Label: "Load"},
		{Label: "Abandon run", Detail: "to the title"},
	})
	p.menu.Index = idx
}

func soundLabel(g *Game) string {
	// A build with no audio device has no Bank at all, and the pause menu is
	// not the place to find that out.
	if g.Sound == nil {
		return "unavailable"
	}
	switch {
	case !g.Sound.Enabled() && g.Sound.Muted():
		return "off"
	case !g.Sound.Enabled():
		return "unavailable"
	default:
		return fmt.Sprintf("%d%%", int(g.Sound.Volume()*100+0.5))
	}
}

func (p *pauseScene) Update(g *Game) error {
	if g.Back() {
		g.Pop()
		return nil
	}
	g.MenuNav(&p.menu)
	if !g.Accept() {
		return nil
	}
	// Dispatch on the label rather than the row number.
	//
	// It was a switch on the index, and the Sound row had been inserted second
	// without the numbers moving: Sound opened the save picker, Save opened the
	// *load* picker — where every empty slot is disabled, so no slot could be
	// selected at all — Load offered to abandon the run, and Abandon run did
	// nothing. Saving was unreachable from the only menu that offers it. A
	// position is not a name and should never have been used as one.
	switch p.selected() {
	case "Resume":
		g.Pop()
	case "Sound":
		g.cycleSound()
		p.refresh(g)
	case "Save":
		g.Push(newSlotScene(g, slotSave))
	case "Load":
		g.Push(newSlotScene(g, slotLoad))
	case "Abandon run":
		g.Ask("", "Abandon this run and return to the title? Anything since your last save goes with it.",
			[]string{"Abandon it", "Keep playing"}, func(g *Game, choice int) {
				if choice != 0 {
					return
				}
				g.stack = nil
				g.Local = nil
				g.Push(newTitleScene(g))
			})
	}
	return nil
}

// selected returns the label of the highlighted row, which is what the pause
// menu acts on.
func (p *pauseScene) selected() string {
	it, ok := p.menu.Selected()
	if !ok {
		return ""
	}
	return it.Label
}

// cycleSound steps the master volume down in quarters and round to full,
// passing through silence. One row, one key, no submenu — it is a setting
// somebody adjusts once and then forgets.
func (g *Game) cycleSound() {
	if g.Sound == nil {
		return
	}
	v := g.Sound.Volume() - 0.25
	if g.Sound.Muted() {
		v = 1
	}
	if v <= 0.001 {
		g.Sound.SetMuted(true)
		return
	}
	g.Sound.SetMuted(false)
	g.Sound.SetVolume(v)
}

func (p *pauseScene) Draw(g *Game, dst *ebiten.Image) {
	if p.under != nil {
		p.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xD0})

	ui.TitledPanel(dst, "paused", render.ScreenW/2-108, 76, 216, 88)
	p.menu.Draw(dst, render.ScreenW/2-84, 88, 184)

	render.TextCenter(dst, g.summary(), render.ScreenW/2, 176, render.ColInkDim)
	render.TextCenter(dst, fmt.Sprintf("seed %d", g.Seed), render.ScreenW/2, 190, render.ColInkFaint)
	if p.menu.Index == 1 {
		render.TextCenter(dst, "left/right adjusts - Z mutes", render.ScreenW/2, 206, render.ColInkFaint)
	}
}
