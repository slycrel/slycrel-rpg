// Package render owns the drawing primitives: the fixed logical resolution,
// the scrolling camera, sprite blitting, and text. Everything the game draws
// goes through here so the pixel grid stays honest — positions are rounded to
// whole pixels before blitting, which is what keeps 16px art from shimmering.
package render

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"golang.org/x/image/font/basicfont"
)

// The logical framebuffer. The window is an integer multiple of this, so every
// game pixel maps to an exact square block of screen pixels.
const (
	ScreenW = 480
	ScreenH = 270
	Scale   = 3
)

// Font is the UI typeface. basicfont is a 7x13 bitmap face that ships with
// x/image, so the game has readable crisp text with no font files to curate.
// Swapping in one of the bundle's 37 TTFs later is a one-line change here.
var Font = text.NewGoXFace(basicfont.Face7x13)

// LineH is the baseline-to-baseline distance for Font.
const LineH = 12

// Palette holds the interface colours. Warm parchment on bruise-purple: it
// reads as "tavern lit by one bad candle", which is the whole aesthetic.
var (
	ColInk       = color.RGBA{0xF2, 0xE4, 0xC4, 0xFF}
	ColInkDim    = color.RGBA{0xA4, 0x96, 0x7E, 0xFF}
	ColInkFaint  = color.RGBA{0x6C, 0x62, 0x54, 0xFF}
	ColPanel     = color.RGBA{0x22, 0x1A, 0x2A, 0xE8}
	ColPanelEdge = color.RGBA{0x7A, 0x5E, 0x38, 0xFF}
	ColGold      = color.RGBA{0xE0, 0xB0, 0x4C, 0xFF}
	ColBlood     = color.RGBA{0xB0, 0x30, 0x38, 0xFF}
	ColHeal      = color.RGBA{0x58, 0xB0, 0x58, 0xFF}
	ColMagic     = color.RGBA{0x60, 0x88, 0xD0, 0xFF}
	ColShadow    = color.RGBA{0x00, 0x00, 0x00, 0xB0}
	ColSelect    = color.RGBA{0xE0, 0xB0, 0x4C, 0x40}
	// ColSelectInk is the label colour on a highlighted row. Gold-on-gold was
	// technically legible and practically not.
	ColSelectInk = color.RGBA{0x2A, 0x1C, 0x10, 0xFF}
)

// Camera converts world pixel coordinates to screen coordinates.
type Camera struct {
	X, Y float64 // top-left of the view, in world pixels
	// Bounds is the world size in pixels; the camera clamps inside it when
	// Clamp is true. Small maps that fit on screen simply centre instead.
	W, H  float64
	Clamp bool
	shake float64
	// wobble advances every tick so Offset can derive the shake displacement
	// from a cheap trig pair. A counter beats an RNG here: it needs no
	// initialisation, and the motion reads as a ring rather than static.
	wobble float64
}

// CenterOn points the camera at a world pixel position.
func (c *Camera) CenterOn(wx, wy float64) {
	c.X = wx - ScreenW/2
	c.Y = wy - ScreenH/2
	if !c.Clamp {
		return
	}
	if c.W <= ScreenW {
		c.X = (c.W - ScreenW) / 2
	} else {
		c.X = core.ClampF(c.X, 0, c.W-ScreenW)
	}
	if c.H <= ScreenH {
		c.Y = (c.H - ScreenH) / 2
	} else {
		c.Y = core.ClampF(c.Y, 0, c.H-ScreenH)
	}
}

// Shake adds screen trauma, decaying each Update. Used on big hits.
func (c *Camera) Shake(amount float64) {
	c.shake = core.ClampF(c.shake+amount, 0, 8)
}

// Update decays the shake. Call once per tick.
func (c *Camera) Update() {
	c.wobble += 1.7
	if c.shake > 0 {
		c.shake *= 0.86
		if c.shake < 0.15 {
			c.shake = 0
		}
	}
}

// Offset returns the current camera translation, shake included.
func (c *Camera) Offset() (float64, float64) {
	if c.shake == 0 {
		return -round(c.X), -round(c.Y)
	}
	dx := math.Sin(c.wobble) * c.shake
	dy := math.Cos(c.wobble*1.37) * c.shake
	return -round(c.X + dx), -round(c.Y + dy)
}

func round(f float64) float64 {
	if f < 0 {
		return float64(int(f - 0.5))
	}
	return float64(int(f + 0.5))
}

// Ctx bundles the destination image with the active camera.
type Ctx struct {
	Dst *ebiten.Image
	Cam Camera
}

// World draws a sprite frame at a world position, its bottom-centre anchored to
// (wx, wy). Anchoring at the feet is what makes a 64px character stand
// correctly on a 16px tile.
func (c *Ctx) World(sp *assetsys.Sprite, frame int, wx, wy float64, flip bool) {
	img := sp.Frame(frame)
	if img == nil {
		return
	}
	ox, oy := c.Cam.Offset()
	w, h := float64(sp.W), float64(sp.H)
	op := &ebiten.DrawImageOptions{}
	if flip {
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(w, 0)
	}
	// Anchored on the artwork's feet, not the bottom of the frame. See
	// assetsys.Sprite.Foot: a 64-pixel character box with a tile of empty
	// space under the boots draws the character a whole tile above the square
	// they are standing on, and everything the player judges a collision by is
	// that square.
	op.GeoM.Translate(round(wx-w/2)+ox, round(wy-h+float64(sp.Foot))+oy)
	c.Dst.DrawImage(img, op)
}

// Tile draws a tile-sized sprite at tile coordinates.
func (c *Ctx) Tile(sp *assetsys.Sprite, frame, tx, ty int) {
	img := sp.Frame(frame)
	if img == nil {
		return
	}
	ox, oy := c.Cam.Offset()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(tx*assetsys.TileSize)+ox, float64(ty*assetsys.TileSize)+oy)
	c.Dst.DrawImage(img, op)
}

// TileTinted draws a tile-sized sprite at tile coordinates under a colour
// multiplier. Structures reuse the ground textures this way rather than needing
// their own art: a wall is the stone swatch at half value.
func (c *Ctx) TileTinted(sp *assetsys.Sprite, frame, tx, ty int, tint color.Color) {
	img := sp.Frame(frame)
	if img == nil {
		return
	}
	ox, oy := c.Cam.Offset()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(tx*assetsys.TileSize)+ox, float64(ty*assetsys.TileSize)+oy)
	op.ColorScale.ScaleWithColor(tint)
	c.Dst.DrawImage(img, op)
}

// shadowImg is the soft blot drawn under anything that stands on a tile. Built
// once: it is the same ellipse every time and there are a lot of them.
var shadowImg = func() *ebiten.Image {
	const w, h = 14, 6
	img := ebiten.NewImage(w, h)
	cx, cy := float64(w-1)/2, float64(h-1)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := (float64(x)-cx)/cx, (float64(y)-cy)/cy
			if d := dx*dx + dy*dy; d <= 1 {
				a := uint8(90 * (1 - d*0.7))
				img.Set(x, y, color.RGBA{0, 0, 0, a})
			}
		}
	}
	return img
}()

// Shadow marks the tile something is standing on.
//
// Character art here is drawn into a 64-pixel box on a 16-pixel grid, so a
// person is about two tiles tall and their body covers whatever is behind them.
// Standing below a wall therefore reads as standing *on* the wall, and no
// amount of correcting the anchor fixes that — the feet are in the right place
// and simply cannot be seen. This puts a mark where the feet are.
func (c *Ctx) Shadow(wx, wy float64) {
	ox, oy := c.Cam.Offset()
	w, h := float64(shadowImg.Bounds().Dx()), float64(shadowImg.Bounds().Dy())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(round(wx-w/2)+ox, round(wy-h/2)+oy)
	c.Dst.DrawImage(shadowImg, op)
}

// Screen draws a sprite frame at raw screen coordinates, top-left anchored,
// optionally scaled. Used for portraits and interface art.
func Screen(dst *ebiten.Image, sp *assetsys.Sprite, frame int, x, y, scale float64) {
	img := sp.Frame(frame)
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(round(x), round(y))
	dst.DrawImage(img, op)
}

// ScreenFit draws a sprite scaled to fit inside a w*h box, centred, preserving
// aspect and snapping to an integer scale when it can (512px monster portraits
// dropped into a 96px slot look far better at exactly 1/4 than at 0.1875).
func ScreenFit(dst *ebiten.Image, sp *assetsys.Sprite, frame int, x, y, w, h float64, tint color.Color) {
	img := sp.Frame(frame)
	if img == nil {
		return
	}
	sw, sh := float64(sp.W), float64(sp.H)
	if sw == 0 || sh == 0 {
		return
	}
	s := w / sw
	if v := h / sh; v < s {
		s = v
	}
	// Prefer a clean 1/N reduction when one is close enough to the fit.
	for n := 1.0; n <= 8; n++ {
		if inv := 1 / n; inv <= s && inv > s*0.72 {
			s = inv
			break
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(round(x+(w-sw*s)/2), round(y+(h-sh*s)/2))
	if tint != nil {
		op.ColorScale.ScaleWithColor(tint)
	}
	dst.DrawImage(img, op)
}

// Rect fills an axis-aligned screen rectangle.
func Rect(dst *ebiten.Image, x, y, w, h float64, c color.Color) {
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), c, false)
}

// Frame strokes a one-pixel rectangle outline.
func Frame(dst *ebiten.Image, x, y, w, h float64, c color.Color) {
	Rect(dst, x, y, w, 1, c)
	Rect(dst, x, y+h-1, w, 1, c)
	Rect(dst, x, y, 1, h, c)
	Rect(dst, x+w-1, y, 1, h, c)
}

// fold rewrites the typography a writer naturally reaches for into the ASCII
// the face actually has.
//
// basicfont.Face7x13 covers Latin-1 and nothing else, so an em-dash or a curly
// quote arrives on screen as a replacement box. That is not hypothetical: the
// em-dash in the hit table rendered every landed blow as "Bosk slap the Wolf @
// 6". Folding here rather than in the content files means a line is safe
// wherever it came from, including one a generator assembled at runtime.
//
// Every entry widens or holds its width except the ellipsis, and measurement
// folds too, so what is measured is always what is drawn.
var fold = strings.NewReplacer(
	"—", "-", // em dash
	"–", "-", // en dash
	"…", "...", // ellipsis
	"“", `"`, "”", `"`, // curly double quotes
	"‘", "'", "’", "'", // curly single quotes
	" ", " ", // non-breaking space
)

// Text draws a single line with a hard drop shadow for legibility over art.
func Text(dst *ebiten.Image, s string, x, y float64, c color.Color) {
	s = fold.Replace(s)

	op := &text.DrawOptions{}
	op.GeoM.Translate(round(x)+1, round(y)+1)
	op.ColorScale.ScaleWithColor(ColShadow)
	text.Draw(dst, s, Font, op)

	op = &text.DrawOptions{}
	op.GeoM.Translate(round(x), round(y))
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, Font, op)
}

// TextW measures a string in pixels, as it will actually be drawn.
func TextW(s string) float64 {
	w, _ := text.Measure(fold.Replace(s), Font, LineH)
	return w
}

// TextCenter draws s centred on cx.
func TextCenter(dst *ebiten.Image, s string, cx, y float64, c color.Color) {
	Text(dst, s, cx-TextW(s)/2, y, c)
}

// TextRight draws s ending at rx.
func TextRight(dst *ebiten.Image, s string, rx, y float64, c color.Color) {
	Text(dst, s, rx-TextW(s), y, c)
}

// Trunc shortens s until it fits within width pixels, marking the cut with a
// trailing period so a clipped name still reads as deliberate.
func Trunc(s string, width float64) string {
	s = fold.Replace(s)
	if TextW(s) <= width {
		return s
	}
	// By runes, not bytes: a name carrying anything the fold left multi-byte
	// would otherwise be cut in half and drawn as mojibake.
	r := []rune(s)
	for len(r) > 1 {
		r = r[:len(r)-1]
		if cut := string(r) + "."; TextW(cut) <= width {
			return cut
		}
	}
	return string(r)
}

// Wrap breaks s into lines no wider than width pixels, splitting on spaces.
// Explicit newlines in s are honoured.
func Wrap(s string, width float64) []string {
	var out []string
	for _, para := range strings.Split(fold.Replace(s), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if TextW(line+" "+w) > width {
				out = append(out, line)
				line = w
				continue
			}
			line += " " + w
		}
		out = append(out, line)
	}
	return out
}
