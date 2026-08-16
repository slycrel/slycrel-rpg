package game

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
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
	if Cancel() {
		g.Pop()
		return nil
	}
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirDown:
			s.menu.Move(1)
		case core.DirUp:
			s.menu.Move(-1)
		}
	}
	if !Confirm() {
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
	p.menu.SetItems([]ui.MenuItem{
		{Label: "Resume"},
		{Label: "Save"},
		{Label: "Load"},
		{Label: "Abandon the run", Detail: "back to the title"},
	})
	return p
}

func (p *pauseScene) Update(g *Game) error {
	if Cancel() {
		g.Pop()
		return nil
	}
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirDown:
			p.menu.Move(1)
		case core.DirUp:
			p.menu.Move(-1)
		}
	}
	if !Confirm() {
		return nil
	}
	switch p.menu.Index {
	case 0:
		g.Pop()
	case 1:
		g.Push(newSlotScene(g, slotSave))
	case 2:
		g.Push(newSlotScene(g, slotLoad))
	case 3:
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

func (p *pauseScene) Draw(g *Game, dst *ebiten.Image) {
	if p.under != nil {
		p.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xD0})

	ui.TitledPanel(dst, "paused", render.ScreenW/2-108, 82, 216, 76)
	p.menu.Draw(dst, render.ScreenW/2-84, 94, 184)

	render.TextCenter(dst, g.summary(), render.ScreenW/2, 172, render.ColInkDim)
	render.TextCenter(dst, fmt.Sprintf("seed %d", g.Seed), render.ScreenW/2, 188, render.ColInkFaint)
}
