package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// settingsScene is one screen for everything the player sets, rather than
// three places to look.
//
// It was three. Sound was a row on the pause menu, combat pace was a constant
// nobody could reach, and key bindings were a comment in game.go saying
// rebinding would be a single-file change whenever somebody wanted it. The
// argument for batching them is the one the first UX pass already made and
// proved: three settings in three menus is three places to look, and the
// fourth one lands somewhere else again.
//
// Reachable from the pause menu and from the title, because half of these are
// wanted before a run starts and the other half during one.
type settingsScene struct {
	under Scene
	menu  ui.Menu
	// binding is the action currently waiting for a key, or empty. While it is
	// set the screen stops reading its own navigation keys — the next press is
	// the answer to a question, whatever it happens to be bound to.
	binding string
	// note is what just happened: a refusal, or a confirmation. Cleared by the
	// next navigation rather than by the next action, which is the rule the
	// shop's description strip had to learn.
	note string
}

// settings row tags. Dispatch is on these rather than on the row number, which
// is the mistake the pause menu made once and is not going to make again.
type paceRow struct{}
type soundRow struct{}
type keyRow struct{ name string }
type defaultsRow struct{}

func newSettingsScene(g *Game) *settingsScene {
	s := &settingsScene{under: g.Top()}
	s.refresh(g)
	return s
}

func (s *settingsScene) refresh(g *Game) {
	idx := s.menu.Index
	items := []ui.MenuItem{
		{Label: "Play", Header: true},
		{Label: "Combat pace", Detail: paceName(), Data: paceRow{}},
		// Greyed when there is nothing to play rather than when it is merely
		// quiet: a run started with -mute can still be turned back up, and a
		// row that refuses to move because somebody chose silence is a row
		// arguing with them.
		{Label: "Sound", Detail: soundLabel(g), Data: soundRow{},
			Disabled: g.Sound == nil || !g.Sound.Available()},
		{Label: "Keys", Header: true},
	}
	for _, a := range actions() {
		detail := bindingLabel(*a.keys)
		if s.binding == a.name {
			detail = "press a key"
		}
		items = append(items, ui.MenuItem{Label: a.name, Detail: detail, Data: keyRow{a.name}})
	}
	items = append(items, ui.MenuItem{
		Label: "Restore the original keys", Detail: "all six", Data: defaultsRow{},
	})
	// Visible before SetItems, because scrolling is measured against it — and
	// the whole list fits, so nothing here ever scrolls.
	s.menu.Visible = len(items)
	s.menu.SetItems(items)
	// Select rather than assigning Index. Row zero is a header, so restoring a
	// remembered position by hand can park the cursor on a heading; Select
	// refuses that and leaves the cursor where a fresh menu would have put it.
	s.menu.Select(idx)
}

func (s *settingsScene) Update(g *Game) error {
	// Capture first and exclusively. Everything below reads navigation keys,
	// and the whole point of this mode is that the next press means "bind
	// this" rather than whatever it currently means.
	if s.binding != "" {
		s.capture(g)
		return nil
	}

	if g.Back() {
		g.savePrefs()
		g.Pop()
		return nil
	}

	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirUp, core.DirDown:
			// The note answers the row it was raised on, so it goes when the
			// cursor does.
			s.note = ""
			step := 1
			if d == core.DirUp {
				step = -1
			}
			s.menu.Move(step)
			g.Sound.Play("ui/move")
		case core.DirLeft, core.DirRight:
			step := 1
			if d == core.DirLeft {
				step = -1
			}
			s.adjust(g, step)
		}
		s.refresh(g)
		return nil
	}

	if !g.Accept() {
		return nil
	}
	it, ok := s.menu.Selected()
	if !ok {
		return nil
	}
	switch row := it.Data.(type) {
	case keyRow:
		s.binding = row.name
		s.note = "Press the key to use. X cancels."
	case defaultsRow:
		restoreBindings()
		s.commit(g)
		s.note = "Back to arrows, WASD and the vi keys."
	default:
		// Pace and sound are left-and-right rows. Confirm nudges them forward
		// rather than doing nothing, because a player who presses Z on a
		// setting meant to change it.
		s.adjust(g, 1)
	}
	s.refresh(g)
	return nil
}

// adjust moves whatever the cursor is on by one step.
func (s *settingsScene) adjust(g *Game, step int) {
	it, ok := s.menu.Selected()
	if !ok {
		return
	}
	s.note = ""
	switch it.Data.(type) {
	case paceRow:
		setPace(step)
		g.Sound.Play("ui/move")
	case soundRow:
		g.stepSound(step)
	default:
		return
	}
	s.commit(g)
}

// capture takes the next key pressed and puts it on the action being bound.
//
// One key replaces the whole list rather than joining it. A player rebinding
// Down is doing it because something else wants `S`, and an "add" that left
// the old binding in place would not have solved the problem they came here
// with — it would have made a second one.
func (s *settingsScene) capture(g *Game) {
	// Escape gets out whatever it is bound to, because a screen you enter by
	// pressing a key and leave by pressing the right key is a trap while the
	// right key is the thing you are in the middle of changing.
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.binding, s.note = "", "Left as it was."
		g.Sound.Play("ui/cancel")
		s.refresh(g)
		return
	}
	pressed := inpututil.AppendJustPressedKeys(nil)
	if len(pressed) == 0 {
		return
	}
	k := pressed[0]

	if ok, why := bindable(k); !ok {
		s.note = upper(why) + "."
		g.Sound.Play("ui/deny")
		return
	}
	if other := boundElsewhere(k, s.binding); other != "" {
		s.note = fmt.Sprintf("%s is already %s.", k.String(), other)
		g.Sound.Play("ui/deny")
		return
	}
	for _, a := range actions() {
		if a.name != s.binding {
			continue
		}
		*a.keys = []ebiten.Key{k}
		s.note = fmt.Sprintf("%s is %s now.", a.name, k.String())
	}
	s.binding = ""
	g.Sound.Play("ui/confirm")
	s.commit(g)
	s.refresh(g)
}

// commit copies the live settings into the preferences and writes them.
//
// Written on every change rather than on leaving the screen: this is the one
// screen a player might close by quitting the game, and a setting that only
// survives a polite exit is a setting that looks broken.
func (s *settingsScene) commit(g *Game) {
	if g.Prefs == nil {
		return
	}
	g.Prefs.Pace = stepTicks
	g.Prefs.Keys = storedBindings()
	g.savePrefs()
}

// stepSound moves the master volume by a tenth, with the bottom of the range
// being off rather than a silence that still calls itself five percent.
func (g *Game) stepSound(step int) {
	if g.Sound == nil {
		return
	}
	v := g.Sound.Volume()
	if g.Sound.Muted() {
		v = 0
	}
	v = core.ClampF(v+0.1*float64(step), 0, 1)
	if v < 0.05 {
		g.Sound.SetMuted(true)
		return
	}
	g.Sound.SetMuted(false)
	g.Sound.SetVolume(v)
	// Lift a -mute for this run too. Somebody who starts silent and then walks
	// into the settings screen and turns the volume up has said what they
	// want more recently than the command line did.
	g.Sound.Unsilence()
	// After setting it, not before: the click is the sample of what was just
	// chosen, which is the only way to set a volume without guessing.
	g.Sound.Play("ui/move")
}

func (s *settingsScene) Draw(g *Game, dst *ebiten.Image) {
	if s.under != nil {
		s.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x08, 0x10, 0xE8})

	// Sized to its rows rather than to a number somebody liked. The list has
	// no icons, so a row is one line of text, and the panel is the header, the
	// rows, and a margin the same as the one above them. A fixed height was
	// sixty pixels of empty box under the last row, which reads as a list that
	// has failed to load the rest of itself.
	const w, x, top, pad = 300.0, 90.0, 20.0, 12.0
	rows := float64(len(s.menu.Items)) * render.LineH
	ui.TitledPanel(dst, "settings", x, top, w, pad+rows+pad)
	s.menu.Draw(dst, x+14, top+pad, w-28)

	// The note and the hint share the strip under the panel, and the note wins
	// while there is one — the same arrangement as the shop's description line,
	// for the same reason: what just happened is more interesting than what the
	// keys do, right up until it stops being true.
	line, col := "Left / Right to change - Z to rebind - X to close", render.ColInkFaint
	if s.note != "" {
		line, col = s.note, render.ColGold
	}
	// Under the panel wherever the panel now ends, since its height moves with
	// the number of rebindable actions.
	render.TextCenter(dst, line, render.ScreenW/2,
		top+pad+rows+pad+10, col)
}
