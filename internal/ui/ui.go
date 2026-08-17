// Package ui draws the interface chrome: panels, slots, meters, menus, and the
// scrolling message log.
//
// All of it is drawn procedurally rather than nine-sliced from art, and after
// an audit of the bundle's 4,488 GUI PNGs that is now a decision rather than a
// placeholder. The kits in there are painted mobile and MMO interfaces at two
// to four times this game's scale: three-pixel outlines and soft drop shadows
// against a seven-by-thirteen bitmap font and a sixteen-pixel tile. The closest
// thing to a match in the whole bundle — GUI Pro's ItemFrame_01 — is a thin
// gold border with clipped corners, which is a description of Panel below.
//
// So the art pass went the other way: rather than importing chrome that would
// have to be scaled down into mush, the vocabulary already here was extended to
// the places that had none. See Slot and Cursor.
package ui

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
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

// slotEdge is the resting border of a Slot. Dimmer than a Panel's edge, since
// a slot sits inside a panel and a frame as bright as its container reads as
// two boxes rather than a thing in a box.
var slotEdge = color.RGBA{0x6A, 0x52, 0x38, 0xFF}

// Slot draws a frame around a picture, in the same vocabulary as Panel: a
// border with the corners knocked out, so it reads as cut rather than as a
// rectangle somebody stroked. No inner bevel — Panel has one because it is big
// enough to carry it, and at portrait size a second line is just a thicker
// border.
//
// It exists because the battle screen had three monster portraits floating on a
// black field with nothing to say where one ended and the next began, and the
// character sheet had a face with no edge to it. A frame is the one piece of
// interface art that survives being drawn at this size, because it is a line
// rather than an illustration — which is exactly why the borrowed kits could
// not help here and eleven lines of drawing code could.
//
// border of nil takes the resting colour, so the common case is one argument
// shorter and a caller only names a colour when it means something.
func Slot(dst *ebiten.Image, x, y, w, h float64, border color.Color) {
	if border == nil {
		border = slotEdge
	}
	render.Rect(dst, x, y, w, h, color.RGBA{0x14, 0x0E, 0x14, 0xC0})
	render.Frame(dst, x, y, w, h, border)
	for _, p := range [][2]float64{{x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
		render.Rect(dst, p[0], p[1], 1, 1, color.RGBA{0, 0, 0, 0xC0})
	}
}

// Cursor draws the selection pointer: a small solid triangle, centred on a row
// of text.
//
// It replaces a ">" set in the body font, which is the one piece of chrome in
// the game that was still a character pretending to be a shape. A glyph carries
// the font's own weight and spacing and its drop shadow, so it read as the
// first letter of the label rather than as a pointer at it — and it could not
// be coloured or moved independently of the text it was standing in for.
func Cursor(dst *ebiten.Image, x, y float64, c color.Color) {
	// Five rows, tapering, drawn from the vertical centre of the ink. Odd so
	// there is a single-pixel tip rather than a blunt two-pixel one.
	cy := y + render.TextInkTop + float64(render.TextInkH)/2
	for i := 0.0; i < 3; i++ {
		render.Rect(dst, x+i, cy-2+i, 1, 5-i*2, c)
	}
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
	Icon     string // asset key; blank rows simply have no icon
	Disabled bool
	// Header marks a row that names the group under it rather than being a
	// choice. It draws as a label with a rule rather than as a greyed-out
	// option, because a heading dressed as an unavailable choice is a heading
	// the player spends a moment trying to select.
	Header bool
	Data   any // caller payload, e.g. the item or spell being chosen
}

// IconSource resolves an icon key to a drawable image, or nil when the key has
// no real art behind it.
type IconSource interface {
	Icon(key string) *ebiten.Image
}

// IconSize is the drawn size of a menu icon. Sixteen is not arbitrary: the
// pixel-art icon sets are 32px and the painted ability set is 128px, so both
// land on an exact integer division and stay crisp.
const IconSize = 16

// Menu is a vertical cursor list with wraparound and a scrolling window.
type Menu struct {
	Items   []MenuItem
	Index   int
	Visible int // rows shown at once; 0 means all
	// Icons resolves MenuItem.Icon. Leave nil for a text-only menu.
	Icons IconSource
	top   int
}

// rowH is the height of one row: taller when the menu carries icons, since an
// icon is wider than the text leading.
func (m *Menu) rowH() float64 {
	if m.Icons != nil && m.hasIcons() {
		return IconSize + 2
	}
	return render.LineH
}

func (m *Menu) hasIcons() bool {
	for _, it := range m.Items {
		if it.Icon != "" {
			return true
		}
	}
	return false
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
		if !m.Items[m.Index].Disabled && !m.Items[m.Index].Header {
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
		ly := y + float64(row)*m.rowH()

		if it.Header {
			ty := ly + m.textOffset() + render.TextInkTop
			render.Text(dst, it.Label, x, ly+m.textOffset(), render.ColInkFaint)
			// A rule from the end of the label to the edge of the list, so the
			// heading reads as a divider rather than as a row you missed.
			if rx := x + render.TextW(it.Label) + 6; rx < x+w-4 {
				render.Rect(dst, rx, ty+float64(render.TextInkH)/2, x+w-4-rx, 1, render.ColInkFaint)
			}
			continue
		}

		col := render.ColInk
		switch {
		case it.Disabled:
			col = render.ColInkFaint
		case i == m.Index:
			col = render.ColSelectInk
			// The bar covers the glyph box, not the nominal row.
			//
			// Text puts ink two pixels below the y it is handed, so a bar from
			// ly-1 for LineH pixels stopped two rows short of the bottom of
			// every letter: the base of each glyph fell onto the dark panel
			// below the bar, dark on dark, and a selected row read as struck
			// through. Rows with icons were never affected, because they are
			// taller than the text and the bar covered it by accident — which
			// is why the shop looked right and the title screen did not.
			barY, barH := ly+render.TextInkTop-1, float64(render.TextInkH+2)
			if h := m.rowH(); h > render.LineH {
				barY, barH = ly-1, h
			}
			render.Rect(dst, x-2, barY, w, barH, render.ColSelect)
			// Clear of the label rather than tight against it. When this was a
			// ">" glyph it also had to clear its own drop shadow, which is one
			// of the reasons it is not a glyph any more.
			Cursor(dst, x-9, ly+m.textOffset(), render.ColGold)
		}
		// Icon first; the label indents past it so rows line up whether or not
		// a given entry has art.
		lx := x
		if m.Icons != nil && m.hasIcons() {
			lx += IconSize + 4
			if img := m.Icons.Icon(it.Icon); img != nil {
				render.ScreenFit(dst, &assetsys.Sprite{
					Frames: []*ebiten.Image{img},
					W:      img.Bounds().Dx(), H: img.Bounds().Dy(),
				}, 0, x, ly-2, IconSize, IconSize, iconTint(it.Disabled))
			}
		}
		// The selected row sits on a solid bar, so it wants no drop shadow.
		drawText, drawRight := render.Text, render.TextRight
		if i == m.Index && !it.Disabled {
			drawText, drawRight = render.TextFlat, render.TextFlatRight
		}
		drawText(dst, it.Label, lx, ly+m.textOffset(), col)

		// The detail column gets whatever the label leaves, minus a gap. Sizing
		// it as a fixed fraction of the row instead is what let "Abandon the
		// run" and "back to the title" draw through each other.
		if it.Detail != "" {
			avail := detailRight - (lx + render.TextW(it.Label)) - 10
			if avail >= 20 {
				d := render.ColInkDim
				if it.Disabled {
					d = render.ColInkFaint
				}
				drawRight(dst, render.Trunc(it.Detail, avail), detailRight, ly+m.textOffset(), d)
			}
		}
	}
	// Scroll hints when the list runs past the window.
	if scrolls {
		if m.top > 0 {
			render.Text(dst, "^", x+w-4, y, render.ColInkDim)
		}
		if m.top+m.Visible < len(m.Items) {
			render.Text(dst, "v", x+w-4, y+float64(m.Visible-1)*m.rowH(), render.ColInkDim)
		}
	}
}

// textOffset centres the label against a taller icon row.
func (m *Menu) textOffset() float64 {
	if h := m.rowH(); h > render.LineH {
		return (h - render.LineH) / 2
	}
	return 0
}

// iconTint dims the icon on an unavailable row so it matches the label.
func iconTint(disabled bool) color.Color {
	if disabled {
		return color.RGBA{0x80, 0x80, 0x80, 0xB0}
	}
	return nil
}

// Height returns the pixel height the menu will occupy.
func (m *Menu) Height() float64 {
	n := len(m.Items)
	if m.Visible > 0 && m.Visible < n {
		n = m.Visible
	}
	return float64(n) * m.rowH()
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
