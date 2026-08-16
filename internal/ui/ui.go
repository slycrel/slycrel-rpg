// Package ui draws the interface chrome: panels, meters, menus, and the
// scrolling message log. The panels are drawn procedurally rather than
// nine-sliced from art, which keeps them resolution-exact and means the game
// looks finished before any of the bundle's 4,488 GUI PNGs are wired in.
package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
)

// Panel draws a framed box: fill, a bright inner bevel, and clipped corners so
// it reads as tooled leather rather than a browser div.
func Panel(dst *ebiten.Image, x, y, w, h float64) {
	render.Rect(dst, x, y, w, h, render.ColPanel)
	render.Frame(dst, x, y, w, h, render.ColPanelEdge)
	render.Frame(dst, x+1, y+1, w-2, h-2, color.RGBA{0x3A, 0x2C, 0x1E, 0xFF})
	// Knock the corners out so the frame looks cut, not drawn.
	for _, p := range [][2]float64{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		render.Rect(dst, p[0], p[1], 1, 1, color.RGBA{0, 0, 0, 0xC0})
	}
}

// TitledPanel draws a panel with a heading burned into the top edge.
func TitledPanel(dst *ebiten.Image, title string, x, y, w, h float64) {
	Panel(dst, x, y, w, h)
	if title == "" {
		return
	}
	tw := render.TextW(title)
	render.Rect(dst, x+6, y, tw+8, 1, render.ColPanel)
	render.Text(dst, title, x+10, y-4, render.ColGold)
}

// Bar draws a labelled meter. frac is clamped to [0,1].
func Bar(dst *ebiten.Image, x, y, w, h, frac float64, fill color.Color) {
	frac = core.ClampF(frac, 0, 1)
	render.Rect(dst, x, y, w, h, color.RGBA{0x18, 0x12, 0x18, 0xFF})
	if frac > 0 {
		fw := float64(int(w-2)) * frac
		if fw < 1 {
			fw = 1
		}
		render.Rect(dst, x+1, y+1, fw, h-2, fill)
		// A lighter top row gives the fill a bit of dimension.
		render.Rect(dst, x+1, y+1, fw, 1, lighten(fill, 40))
	}
	render.Frame(dst, x, y, w, h, render.ColInkFaint)
}

func lighten(c color.Color, by uint8) color.Color {
	r, g, b, a := c.RGBA()
	add := func(v uint32) uint8 {
		n := uint32(v>>8) + uint32(by)
		if n > 255 {
			n = 255
		}
		return uint8(n)
	}
	return color.RGBA{add(r), add(g), add(b), uint8(a >> 8)}
}

// StatBars draws the standard HP / spell-point pair used everywhere.
func StatBars(dst *ebiten.Image, x, y, w float64, hp, maxHP, ps, maxPS int) {
	render.Text(dst, fmt.Sprintf("HP %d/%d", hp, maxHP), x, y, render.ColInk)
	Bar(dst, x, y+render.LineH-2, w, 5, frac(hp, maxHP), render.ColBlood)
	render.Text(dst, fmt.Sprintf("SP %d/%d", ps, maxPS), x, y+render.LineH+7, render.ColInk)
	Bar(dst, x, y+2*render.LineH+5, w, 5, frac(ps, maxPS), render.ColMagic)
}

func frac(a, b int) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// MenuItem is one selectable row. A disabled row is drawn dim and skipped by
// the cursor, which is how "Spell" greys out when you are out of psyche.
type MenuItem struct {
	Label    string
	Detail   string // right-aligned annotation: cost, count, price
	Disabled bool
	Data     any // caller payload, e.g. the item or spell being chosen
}

// Menu is a vertical cursor list with wraparound and a scrolling window.
type Menu struct {
	Items   []MenuItem
	Index   int
	Visible int // rows shown at once; 0 means all
	top     int
}

// SetItems replaces the contents, keeping the cursor in range and parked on an
// enabled row.
func (m *Menu) SetItems(items []MenuItem) {
	m.Items = items
	if m.Index >= len(items) {
		m.Index = core.Max(0, len(items)-1)
	}
	m.snapToEnabled(1)
}

// Move steps the cursor by delta, skipping disabled rows.
func (m *Menu) Move(delta int) {
	if len(m.Items) == 0 {
		return
	}
	m.Index = ((m.Index+delta)%len(m.Items) + len(m.Items)) % len(m.Items)
	m.snapToEnabled(delta)
	m.scrollToCursor()
}

// snapToEnabled walks in the given direction until it lands on a selectable
// row, giving up after a full lap if everything is disabled.
func (m *Menu) snapToEnabled(dir int) {
	if len(m.Items) == 0 {
		return
	}
	if dir == 0 {
		dir = 1
	}
	for i := 0; i < len(m.Items); i++ {
		if !m.Items[m.Index].Disabled {
			return
		}
		m.Index = ((m.Index+dir)%len(m.Items) + len(m.Items)) % len(m.Items)
	}
}

func (m *Menu) scrollToCursor() {
	if m.Visible <= 0 || len(m.Items) <= m.Visible {
		m.top = 0
		return
	}
	if m.Index < m.top {
		m.top = m.Index
	}
	if m.Index >= m.top+m.Visible {
		m.top = m.Index - m.Visible + 1
	}
	m.top = core.Clamp(m.top, 0, len(m.Items)-m.Visible)
}

// Selected returns the current item, or false when the menu is empty.
func (m *Menu) Selected() (MenuItem, bool) {
	if len(m.Items) == 0 {
		return MenuItem{}, false
	}
	return m.Items[m.Index], true
}

// Draw renders the menu at x,y within the given width.
func (m *Menu) Draw(dst *ebiten.Image, x, y, w float64) {
	n := len(m.Items)
	if m.Visible > 0 && m.Visible < n {
		n = m.Visible
	}
	// When the list scrolls, the arrows occupy the right edge, so the detail
	// column has to give up that space or the two draw on top of each other.
	scrolls := m.Visible > 0 && len(m.Items) > m.Visible
	detailRight := x + w - 6
	if scrolls {
		detailRight -= 10
	}
	for row := 0; row < n; row++ {
		i := m.top + row
		if i >= len(m.Items) {
			break
		}
		it := m.Items[i]
		ly := y + float64(row)*render.LineH

		col := render.ColInk
		switch {
		case it.Disabled:
			col = render.ColInkFaint
		case i == m.Index:
			col = render.ColGold
			render.Rect(dst, x-2, ly-1, w, render.LineH, render.ColSelect)
			render.Text(dst, ">", x-8, ly, render.ColGold)
		}
		render.Text(dst, it.Label, x, ly, col)

		// The detail column gets whatever the label leaves, minus a gap. Sizing
		// it as a fixed fraction of the row instead is what let "Abandon the
		// run" and "back to the title" draw through each other.
		if it.Detail != "" {
			avail := detailRight - (x + render.TextW(it.Label)) - 10
			if avail >= 20 {
				d := render.ColInkDim
				if it.Disabled {
					d = render.ColInkFaint
				}
				render.TextRight(dst, render.Trunc(it.Detail, avail), detailRight, ly, d)
			}
		}
	}
	// Scroll hints when the list runs past the window.
	if scrolls {
		if m.top > 0 {
			render.Text(dst, "^", x+w-4, y, render.ColInkDim)
		}
		if m.top+m.Visible < len(m.Items) {
			render.Text(dst, "v", x+w-4, y+float64(m.Visible-1)*render.LineH, render.ColInkDim)
		}
	}
}

// Height returns the pixel height the menu will occupy.
func (m *Menu) Height() float64 {
	n := len(m.Items)
	if m.Visible > 0 && m.Visible < n {
		n = m.Visible
	}
	return float64(n) * render.LineH
}

// Log is the rolling combat/event transcript.
type Log struct {
	lines []logLine
	max   int
}

type logLine struct {
	text string
	col  color.Color
}

// NewLog returns a log that keeps the most recent max lines.
func NewLog(max int) *Log { return &Log{max: max} }

// Add appends a line in the default ink colour.
func (l *Log) Add(format string, args ...any) { l.AddColor(render.ColInk, format, args...) }

// AddColor appends a coloured line. Long lines are wrapped to the standard
// message width before being stored so scrollback stays accurate.
func (l *Log) AddColor(c color.Color, format string, args ...any) {
	for _, s := range render.Wrap(fmt.Sprintf(format, args...), render.ScreenW-40) {
		l.lines = append(l.lines, logLine{s, c})
	}
	if len(l.lines) > l.max {
		l.lines = l.lines[len(l.lines)-l.max:]
	}
}

// Clear empties the log.
func (l *Log) Clear() { l.lines = nil }

// Draw renders the last n lines ending at the given baseline, oldest fading.
func (l *Log) Draw(dst *ebiten.Image, x, y float64, n int) {
	start := core.Max(0, len(l.lines)-n)
	shown := l.lines[start:]
	for i, ln := range shown {
		c := ln.col
		// Fade everything but the two most recent lines.
		if i < len(shown)-2 {
			c = render.ColInkDim
		}
		render.Text(dst, ln.text, x, y+float64(i)*render.LineH, c)
	}
}

// Lines returns how many lines the log currently holds.
func (l *Log) Lines() int { return len(l.lines) }
