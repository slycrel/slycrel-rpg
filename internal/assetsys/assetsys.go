// Package assetsys loads game art. It is deliberately forgiving: every lookup
// falls back to a generated placeholder, so the game runs end-to-end before a
// single file has been curated out of the 16.7 GB source bundle.
//
// Art arrives in three ways:
//
//	Whole images    a PNG used as-is (battle portraits, backdrops)
//	Frame strips    a PNG that is a vertical or horizontal run of equal frames
//	                (the top-down character sheets are 64px-wide vertical runs)
//	Generated       procedural pixel-art tiles, used until real tiles are wired
//
// Keys are stable strings like "hero/viking/walk_down" or "tile/forest". The
// manifest at assets/manifest.json maps keys to files; anything missing from it
// is generated instead.
package assetsys

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
)

// TileSize is the edge length of one world tile in pixels, re-exported from
// core so that art code can talk about the grid without importing it twice.
const TileSize = core.TileSize

// Entry is one manifest record.
type Entry struct {
	Key    string `json:"key"`
	File   string `json:"file"`   // relative to the assets/ root
	FrameW int    `json:"frameW"` // 0 = whole image
	FrameH int    `json:"frameH"`
	// Rect optionally crops the source before slicing: [x, y, w, h].
	Rect []int `json:"rect,omitempty"`
}

// Manifest is the on-disk asset map.
type Manifest struct {
	Entries []Entry `json:"entries"`
}

// Sprite is a loaded image plus its frame layout.
type Sprite struct {
	Frames []*ebiten.Image
	W, H   int
	// Foot is how many transparent rows sit below the artwork inside the
	// frame. Character art is drawn into a generous box — the hero sheets are
	// 64x64 on a 16-pixel grid — and the feet do not reach the bottom of it, so
	// anchoring a sprite by its frame puts the character above the tile it is
	// standing on. Every hero in the game was drawing a full tile high, which
	// read in play as the walls of a building having their hit box off by one.
	Foot int
	// Head is the mirror of Foot: how many transparent rows sit above the
	// artwork. Anything that wants to put something over a character's head —
	// a mark, a bubble — has to know where the head actually is, and in a
	// 64-pixel box holding a 30-pixel villager that is nowhere near the top of
	// the frame.
	Head int
}

// Frame returns frame i, wrapping so animation callers never index out of
// range. A sprite with no frames returns nil, which Draw treats as a no-op.
func (s *Sprite) Frame(i int) *ebiten.Image {
	if s == nil || len(s.Frames) == 0 {
		return nil
	}
	return s.Frames[((i%len(s.Frames))+len(s.Frames))%len(s.Frames)]
}

// Count returns the number of frames.
func (s *Sprite) Count() int {
	if s == nil {
		return 0
	}
	return len(s.Frames)
}

// Registry owns every loaded sprite.
type Registry struct {
	root  string
	mu    sync.RWMutex
	cache map[string]*Sprite
	man   map[string]Entry
	// missing records keys that fell back to a placeholder, so `slycrel
	// -assets` can report exactly what still needs curating.
	missing map[string]bool
}

// New opens a registry rooted at the repository root. It reads
// assets/manifest.json and resolves every entry's File relative to root, which
// lets the manifest point either at curated copies under assets/ or straight
// into the extracted bundle under assets-raw/. A missing or malformed manifest
// is not fatal; everything simply generates.
func New(root string) *Registry {
	r := &Registry{
		root:    root,
		cache:   map[string]*Sprite{},
		man:     map[string]Entry{},
		missing: map[string]bool{},
	}
	data, err := os.ReadFile(filepath.Join(root, "assets", "manifest.json"))
	if err != nil {
		return r
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "assetsys: manifest.json is malformed (%v); generating all art\n", err)
		return r
	}
	for _, e := range m.Entries {
		r.man[e.Key] = e
	}
	return r
}

// Get returns the sprite for key, loading it on first use. It never returns
// nil: unknown keys produce a generated placeholder derived from the key, so
// the same key always looks the same across runs.
func (r *Registry) Get(key string) *Sprite {
	r.mu.RLock()
	s, ok := r.cache[key]
	r.mu.RUnlock()
	if ok {
		return s
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.cache[key]; ok { // another goroutine won the race
		return s
	}

	s = r.load(key)
	if s == nil {
		s = generate(key)
		r.missing[key] = true
	}
	r.cache[key] = s
	return s
}

// Icon returns the image for a key, but only when it resolves to real art.
// Unknown keys return nil rather than the magenta placeholder: a missing icon
// should leave a gap in a menu, not a marker sitting next to every row.
func (r *Registry) Icon(key string) *ebiten.Image {
	if key == "" || !r.Has(key) {
		return nil
	}
	return r.Get(key).Frame(0)
}

// Has reports whether a key is declared in the manifest and its file exists.
// Used by the audit pass to find art that will silently fall back.
func (r *Registry) Has(key string) bool {
	r.mu.RLock()
	e, ok := r.man[key]
	root := r.root
	r.mu.RUnlock()
	if !ok {
		return false
	}
	_, err := os.Stat(filepath.Join(root, e.File))
	return err == nil
}

// Keys lists every key the manifest declares, sorted.
//
// For the audit and the tests that check coverage in the other direction: not
// "is everything the game names present", which Has answers, but "is everything
// present actually named by anything", which is how art gets extracted, counted
// and never drawn.
func (r *Registry) Keys() []string {
	r.mu.RLock()
	out := make([]string, 0, len(r.man))
	for k := range r.man {
		out = append(out, k)
	}
	r.mu.RUnlock()
	sort.Strings(out)
	return out
}

// Count returns how many keys the manifest declares.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.man)
}

// Missing lists the keys that are currently rendering as placeholders.
func (r *Registry) Missing() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.missing))
	for k := range r.missing {
		out = append(out, k)
	}
	return out
}

func (r *Registry) load(key string) *Sprite {
	e, ok := r.man[key]
	if !ok {
		return nil
	}
	f, err := os.Open(filepath.Join(r.root, e.File))
	if err != nil {
		return nil
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil
	}

	img := ebiten.NewImageFromImage(src)
	if len(e.Rect) == 4 {
		sub := img.SubImage(image.Rect(e.Rect[0], e.Rect[1], e.Rect[0]+e.Rect[2], e.Rect[1]+e.Rect[3]))
		img = ebiten.NewImageFromImage(sub)
	}

	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	fw, fh := e.FrameW, e.FrameH
	if fw <= 0 {
		fw = w
	}
	if fh <= 0 {
		fh = h
	}
	if fw > w {
		fw = w
	}
	if fh > h {
		fh = h
	}

	sp := &Sprite{W: fw, H: fh}
	// Row-major slicing covers vertical strips (cols==1), horizontal strips
	// (rows==1), and full grids with one code path.
	for y := 0; y+fh <= h; y += fh {
		for x := 0; x+fw <= w; x += fw {
			sub := img.SubImage(image.Rect(x, y, x+fw, y+fh)).(*ebiten.Image)
			sp.Frames = append(sp.Frames, sub)
		}
	}
	if len(sp.Frames) == 0 {
		return nil
	}
	sp.Foot = footPadding(src, fw, fh)
	sp.Head = headPadding(src, fw, fh)
	return sp
}

// footPadding counts the transparent rows below the artwork in the first frame.
//
// Measured off the decoded source rather than the uploaded texture, because
// ebiten refuses ReadPixels before the game loop starts and asset loading is
// not guaranteed to happen inside it — the audit command loads every key
// without ever opening a window. Measured at all, rather than declared in the
// manifest, because the packs this game draws from pad differently from each
// other and sometimes within themselves, and a number somebody has to remember
// to write down is a number that will be wrong for the next sheet.
func footPadding(src image.Image, w, h int) int {
	if src == nil || w <= 0 || h <= 0 {
		return 0
	}
	b := src.Bounds()
	if w > b.Dx() {
		w = b.Dx()
	}
	if h > b.Dy() {
		h = b.Dy()
	}
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			if _, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA(); a > 4096 {
				return h - 1 - y
			}
		}
	}
	return 0
}

// headPadding counts the empty rows above the artwork, the same way
// footPadding counts the ones below.
func headPadding(src image.Image, w, h int) int {
	if src == nil || w <= 0 || h <= 0 {
		return 0
	}
	b := src.Bounds()
	if w > b.Dx() {
		w = b.Dx()
	}
	if h > b.Dy() {
		h = b.Dy()
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if _, _, _, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA(); a > 4096 {
				return y
			}
		}
	}
	return 0
}

// generate builds a deterministic placeholder from the key. Tile keys get a
// dithered pixel-art fill in a terrain-appropriate palette; everything else
// gets a labelled block so it is obvious on screen what is still missing.
func generate(key string) *Sprite {
	if pal, ok := tilePalettes[key]; ok {
		return &Sprite{Frames: []*ebiten.Image{ditherTile(key, pal)}, W: TileSize, H: TileSize}
	}
	// Unknown key: a magenta-ish marker at sprite scale.
	const w, h = 24, 32
	img := ebiten.NewImage(w, h)
	img.Fill(color.RGBA{0xC0, 0x2A, 0x8A, 0xFF})
	inner := ebiten.NewImage(w-4, h-4)
	inner.Fill(color.RGBA{0x2A, 0x1A, 0x28, 0xFF})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(2, 2)
	img.DrawImage(inner, op)
	return &Sprite{Frames: []*ebiten.Image{img}, W: w, H: h}
}

// palette is the small colour ramp a generated tile is dithered from.
type palette struct {
	base, light, dark, accent color.RGBA
	// accentRate is how often the accent speck appears, in [0,1].
	accentRate float64
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 0xFF} }

// tilePalettes gives every terrain a hand-picked ramp so the generated
// overworld reads correctly (and honestly looks fine) before real tiles land.
var tilePalettes = map[string]palette{
	"tile/ocean":      {rgb(30, 58, 110), rgb(44, 80, 142), rgb(21, 42, 84), rgb(96, 150, 200), 0.04},
	"tile/shallows":   {rgb(52, 108, 158), rgb(78, 142, 190), rgb(38, 84, 130), rgb(140, 196, 224), 0.07},
	"tile/beach":      {rgb(214, 194, 138), rgb(232, 216, 168), rgb(188, 166, 112), rgb(160, 140, 96), 0.05},
	"tile/plains":     {rgb(104, 148, 74), rgb(126, 172, 90), rgb(84, 124, 60), rgb(150, 186, 104), 0.10},
	"tile/meadow":     {rgb(124, 168, 84), rgb(148, 190, 100), rgb(100, 140, 68), rgb(206, 200, 96), 0.08},
	"tile/forest":     {rgb(58, 102, 60), rgb(74, 122, 72), rgb(42, 80, 48), rgb(96, 142, 84), 0.16},
	"tile/deepwood":   {rgb(38, 74, 48), rgb(50, 92, 58), rgb(26, 54, 36), rgb(64, 106, 66), 0.20},
	"tile/hills":      {rgb(126, 122, 84), rgb(150, 144, 100), rgb(102, 100, 68), rgb(94, 86, 62), 0.14},
	"tile/mountain":   {rgb(122, 118, 122), rgb(158, 154, 158), rgb(88, 86, 92), rgb(196, 196, 200), 0.18},
	"tile/peak":       {rgb(196, 200, 208), rgb(228, 232, 238), rgb(150, 154, 164), rgb(255, 255, 255), 0.22},
	"tile/swamp":      {rgb(76, 92, 62), rgb(94, 110, 74), rgb(56, 70, 48), rgb(112, 126, 66), 0.18},
	"tile/desert":     {rgb(206, 178, 118), rgb(226, 202, 144), rgb(178, 150, 96), rgb(150, 124, 80), 0.06},
	"tile/wasteland":  {rgb(120, 96, 84), rgb(142, 116, 100), rgb(96, 76, 68), rgb(78, 58, 54), 0.14},
	"tile/road":       {rgb(168, 148, 116), rgb(190, 172, 138), rgb(142, 124, 96), rgb(120, 104, 82), 0.10},
	"tile/river":      {rgb(60, 116, 168), rgb(88, 152, 200), rgb(44, 92, 140), rgb(150, 200, 230), 0.10},
	"tile/floor":      {rgb(92, 82, 76), rgb(110, 100, 92), rgb(72, 64, 60), rgb(126, 114, 104), 0.10},
	"tile/wall":       {rgb(58, 52, 52), rgb(74, 66, 66), rgb(40, 36, 38), rgb(88, 78, 76), 0.08},
	"tile/cobble":     {rgb(132, 126, 118), rgb(154, 148, 138), rgb(108, 102, 96), rgb(90, 86, 82), 0.12},
	"tile/grassfloor": {rgb(112, 152, 78), rgb(134, 176, 94), rgb(92, 128, 64), rgb(158, 190, 108), 0.10},
	"tile/roof":       {rgb(146, 74, 58), rgb(172, 92, 70), rgb(114, 56, 46), rgb(92, 44, 38), 0.12},
	"tile/void":       {rgb(14, 12, 18), rgb(22, 20, 28), rgb(8, 7, 11), rgb(30, 26, 38), 0.05},
}

// ditherTile paints a 16x16 tile with a stable per-pixel scatter. The RNG is
// seeded from the key so a given terrain always renders identically.
func ditherTile(key string, p palette) *ebiten.Image {
	img := ebiten.NewImage(TileSize, TileSize)
	img.Fill(p.base)
	g := core.NewRNG(0).Fork(key, 0x5CE11)
	px := ebiten.NewImage(1, 1)
	px.Fill(color.White)
	for y := 0; y < TileSize; y++ {
		for x := 0; x < TileSize; x++ {
			var c color.RGBA
			switch {
			case g.Chance(p.accentRate):
				c = p.accent
			case g.Chance(0.14):
				c = p.light
			case g.Chance(0.16):
				c = p.dark
			default:
				continue
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			op.ColorScale.ScaleWithColor(c)
			img.DrawImage(px, op)
		}
	}
	return img
}
