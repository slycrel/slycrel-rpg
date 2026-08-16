// Package tiles draws blended terrain.
//
// The problem it solves: a tile map that paints one flat texture per cell reads
// as a grid of squares. Real terrain wants soft, irregular boundaries — grass
// overhanging dirt, sand fringing water.
//
// The approach is quarter-tile (corner) autotiling. Each 16px tile is
// considered as four 8px quarters, and each quarter looks at the three cells
// touching that corner. Where a higher-priority material touches, it bleeds
// into the quarter through a mask. Corner-based blending needs no inside-corner
// artwork and handles three-way junctions, which edge-based systems cannot.
//
// The masks are generated rather than drawn, from a coverage function plus an
// ordered dither. That is deliberate: Mana Seed ships beautiful hand-drawn
// transitions, but each pack encodes them in its own undocumented permutation
// layout, and none of the TSX files carry wangset metadata to decode it.
// Generating the masks means one uniform system covering every terrain — the
// packs supply the textures, which is the part that actually carries the look.
package tiles

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
)

// Size is the tile edge in pixels; Quarter is half of it.
const (
	Size    = assetsys.TileSize
	Quarter = Size / 2
)

// Rolls is how many phase-shifted variants of each texture are generated.
// Without this every cell of a material is pixel-identical and the map shows
// an obvious 16px grid; four rolls break that up for almost nothing.
const Rolls = 4

// Corner positions, in the order the renderer walks them.
const (
	cornerTL = iota
	cornerTR
	cornerBL
	cornerBR
	numCorners
)

// Quarter shapes, chosen by which neighbours at a corner share the material.
const (
	shapeVert  = iota // only the vertical neighbour: a strip along top or bottom
	shapeHoriz        // only the horizontal neighbour: a strip along a side
	shapeBoth         // both: the quarter is nearly covered, concave at the far corner
	shapeDiag         // neither, but the diagonal: a small nub in the corner
	numShapes
)

// Material is one ground type: a texture and a layering priority. Higher
// priority draws over lower, so grass fringes dirt rather than the reverse.
type Material struct {
	Name    string
	Texture string // assetsys key for a flat, seamlessly tiling 16x16 swatch
	// Fallback is used when Texture is not in the manifest, which is the case
	// for anyone without the purchased art. It should name a key assetsys can
	// generate, so the game still renders coherent terrain with no bundle.
	Fallback string
	Priority int
}

// set holds one material's composited artwork, packed into a single image so
// the pieces are free sub-images rather than separate textures.
type set struct {
	atlas   *ebiten.Image
	base    [Rolls]*ebiten.Image
	corners [Rolls][numCorners][numShapes]*ebiten.Image
	prio    int
}

// Renderer draws terrain for a set of materials.
type Renderer struct {
	sets  map[string]*set
	prios map[string]int
	// fallback is used for a material with no texture, so an unmapped terrain
	// is a flat colour rather than a crash.
	fallback *ebiten.Image
}

// New composites every material's atlas from its texture swatch.
func New(reg *assetsys.Registry, mats []Material) *Renderer {
	r := &Renderer{
		sets:     map[string]*set{},
		prios:    map[string]int{},
		fallback: ebiten.NewImage(Size, Size),
	}
	r.fallback.Fill(color.RGBA{0x30, 0x28, 0x38, 0xFF})

	masks := buildMasks()
	for _, m := range mats {
		r.prios[m.Name] = m.Priority
		key := m.Texture
		if !reg.Has(key) {
			key = m.Fallback
		}
		sp := reg.Get(key)
		src := sp.Frame(0)
		// Generated placeholders for unknown keys are not tile-shaped; refuse
		// them rather than compositing terrain out of a magenta marker.
		if src == nil || sp.W != Size || sp.H != Size {
			continue
		}
		r.sets[m.Name] = compose(src, masks, m.Priority)
	}
	return r
}

// Priority returns a material's layer, and whether it is known.
func (r *Renderer) Priority(name string) (int, bool) {
	p, ok := r.prios[name]
	return p, ok
}

// Draw paints one tile at screen position (sx, sy). at reports the material
// name for any world cell; it is called for the tile and its eight neighbours.
func (r *Renderer) Draw(dst *ebiten.Image, sx, sy float64, tx, ty int, at func(x, y int) string) {
	name := at(tx, ty)
	s := r.sets[name]
	if s == nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(sx, sy)
		dst.DrawImage(r.fallback, op)
		return
	}
	roll := rollFor(tx, ty)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(sx, sy)
	dst.DrawImage(s.base[roll], op)

	// Every distinct higher-priority material in the neighbourhood gets a pass,
	// lowest first, so a water/sand/grass junction layers in the right order.
	for _, over := range r.overlays(tx, ty, s.prio, at) {
		os := r.sets[over]
		if os == nil {
			continue
		}
		covers := func(dx, dy int) bool {
			p, ok := r.prios[at(tx+dx, ty+dy)]
			return ok && p >= os.prio
		}
		for c := 0; c < numCorners; c++ {
			dx, dy := cornerDir(c)
			v, h, d := covers(0, dy), covers(dx, 0), covers(dx, dy)

			shape, ok := cornerShape(v, h, d)
			if !ok {
				continue
			}
			qx, qy := 0.0, 0.0
			if dx > 0 {
				qx = Quarter
			}
			if dy > 0 {
				qy = Quarter
			}
			cop := &ebiten.DrawImageOptions{}
			cop.GeoM.Translate(sx+qx, sy+qy)
			dst.DrawImage(os.corners[roll][c][shape], cop)
		}
	}
}

// overlays lists the distinct materials around a tile that outrank it,
// ascending. The slice is tiny and short-lived; terrain rarely has more than
// two materials meeting at a cell.
func (r *Renderer) overlays(tx, ty, own int, at func(x, y int) string) []string {
	var out []string
	seen := map[string]bool{}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			n := at(tx+dx, ty+dy)
			p, ok := r.prios[n]
			if !ok || p <= own || seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	// Insertion sort: at most a handful of entries, and it avoids dragging in
	// sort.Slice's closure allocation on a per-tile path.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && r.prios[out[j]] < r.prios[out[j-1]]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// cornerShape picks the quarter artwork for a corner from which of its three
// neighbours carry the overlaying material: v is the vertical neighbour, h the
// horizontal, d the diagonal. It reports false when the corner is untouched.
//
// The diagonal only matters when neither side does: if a side is present the
// side shapes already cover that part of the quarter, and layering the nub on
// top would darken the seam.
func cornerShape(v, h, d bool) (int, bool) {
	switch {
	case v && h:
		return shapeBoth, true
	case v:
		return shapeVert, true
	case h:
		return shapeHoriz, true
	case d:
		return shapeDiag, true
	}
	return 0, false
}

// cornerDir returns the neighbour direction a corner faces.
func cornerDir(c int) (dx, dy int) {
	switch c {
	case cornerTL:
		return -1, -1
	case cornerTR:
		return 1, -1
	case cornerBL:
		return -1, 1
	default:
		return 1, 1
	}
}

// rollFor picks a texture phase from the tile position. Multiplying by coprime
// odd numbers keeps neighbouring tiles from landing on the same roll in rows
// or columns, which is what a naive (x+y)%n would do.
func rollFor(tx, ty int) int {
	h := tx*7 + ty*13
	return ((h % Rolls) + Rolls) % Rolls
}

// blendMask multiplies the destination by the source's alpha — a
// "destination in" composite. Masking on the GPU this way avoids ReadPixels,
// which needs a live graphics driver and would make the whole package unusable
// before the game loop starts.
var blendMask = ebiten.Blend{
	BlendFactorSourceRGB:        ebiten.BlendFactorZero,
	BlendFactorSourceAlpha:      ebiten.BlendFactorZero,
	BlendFactorDestinationRGB:   ebiten.BlendFactorSourceAlpha,
	BlendFactorDestinationAlpha: ebiten.BlendFactorSourceAlpha,
	BlendOperationRGB:           ebiten.BlendOperationAdd,
	BlendOperationAlpha:         ebiten.BlendOperationAdd,
}

// Atlas layout. The base row holds one rolled 16px tile per roll; below it,
// each roll gets a block of shapes across by corners down, all at quarter size.
const (
	atlasCornerW = numShapes * Quarter  // 32
	atlasCornerH = numCorners * Quarter // 32
	atlasW       = Rolls * atlasCornerW // 128, wider than the base row needs
	atlasH       = Size + atlasCornerH  // 48
)

// compose builds a material's atlas: rolled base tiles along the top, and each
// roll's sixteen masked corner quarters beneath.
func compose(src *ebiten.Image, masks [Rolls][numCorners][numShapes]*ebiten.Image, prio int) *set {
	s := &set{prio: prio}
	atlas := ebiten.NewImage(atlasW, atlasH)

	for roll := 0; roll < Rolls; roll++ {
		rolled := rollTexture(src, roll)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(roll*Size), 0)
		atlas.DrawImage(rolled, op)

		for c := 0; c < numCorners; c++ {
			// Take the matching quarter of the texture, so the overlay stays in
			// phase with the base tile underneath it.
			qx, qy := 0, 0
			if c == cornerTR || c == cornerBR {
				qx = Quarter
			}
			if c == cornerBL || c == cornerBR {
				qy = Quarter
			}
			quarter := rolled.SubImage(
				image.Rect(qx, qy, qx+Quarter, qy+Quarter)).(*ebiten.Image)

			for sh := 0; sh < numShapes; sh++ {
				scratch := ebiten.NewImage(Quarter, Quarter)
				scratch.DrawImage(quarter, &ebiten.DrawImageOptions{})

				mop := &ebiten.DrawImageOptions{}
				mop.Blend = blendMask
				scratch.DrawImage(masks[roll][c][sh], mop)

				dop := &ebiten.DrawImageOptions{}
				dop.GeoM.Translate(float64(roll*atlasCornerW+sh*Quarter), float64(Size+c*Quarter))
				atlas.DrawImage(scratch, dop)
			}
		}
	}

	s.atlas = atlas
	for roll := 0; roll < Rolls; roll++ {
		s.base[roll] = atlas.SubImage(
			image.Rect(roll*Size, 0, roll*Size+Size, Size)).(*ebiten.Image)
		for c := 0; c < numCorners; c++ {
			for sh := 0; sh < numShapes; sh++ {
				x := roll*atlasCornerW + sh*Quarter
				y := Size + c*Quarter
				s.corners[roll][c][sh] = atlas.SubImage(
					image.Rect(x, y, x+Quarter, y+Quarter)).(*ebiten.Image)
			}
		}
	}
	return s
}

// rollTexture shifts a seamlessly tiling texture by a quarter-tile multiple,
// wrapping around. Because the source tiles, the result still tiles.
func rollTexture(src *ebiten.Image, roll int) *ebiten.Image {
	out := ebiten.NewImage(Size, Size)
	shift := roll * (Size / Rolls)
	for _, off := range [][2]int{{0, 0}, {-Size, 0}, {0, -Size}, {-Size, -Size}} {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(shift+off[0]), float64(shift+off[1]))
		out.DrawImage(src, op)
	}
	return out
}

// bayer is a 4x4 ordered dither matrix. Thresholding coverage against it gives
// the boundary a crisp scattered fringe instead of a straight line or a blur,
// which is what makes a generated mask read as pixel art.
var bayer = [4][4]float64{
	{0.0625, 0.5625, 0.1875, 0.6875},
	{0.8125, 0.3125, 0.9375, 0.4375},
	{0.2500, 0.7500, 0.1250, 0.6250},
	{1.0000, 0.5000, 0.8750, 0.3750},
}

// buildMasks generates every corner/shape mask, one set per roll. Shapes are
// authored for the top-left corner and mirrored, so all four corners agree.
//
// The dither is offset per roll. With a single mask set, every tile along a
// straight boundary carries an identical fringe and the edge reads as a dashed
// line — the one artifact that gives a generated mask away. Varying the phase
// with the same position hash that picks the texture roll breaks it up.
func buildMasks() [Rolls][numCorners][numShapes]*ebiten.Image {
	var out [Rolls][numCorners][numShapes]*ebiten.Image
	for roll := 0; roll < Rolls; roll++ {
		out[roll] = buildMaskSet(roll)
	}
	return out
}

func buildMaskSet(roll int) [numCorners][numShapes]*ebiten.Image {
	var out [numCorners][numShapes]*ebiten.Image
	for c := 0; c < numCorners; c++ {
		flipX := c == cornerTR || c == cornerBR
		flipY := c == cornerBL || c == cornerBR
		for sh := 0; sh < numShapes; sh++ {
			// White RGB carrying the coverage in alpha, so blendMask can use it
			// directly as a stencil.
			m := image.NewRGBA(image.Rect(0, 0, Quarter, Quarter))
			for y := 0; y < Quarter; y++ {
				for x := 0; x < Quarter; x++ {
					// Sample in canonical top-left space.
					cx, cy := x, y
					if flipX {
						cx = Quarter - 1 - x
					}
					if flipY {
						cy = Quarter - 1 - y
					}
					if coverage(sh, cx, cy) > bayer[(y+roll)&3][(x+roll*2)&3] {
						m.SetRGBA(x, y, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
					}
				}
			}
			out[c][sh] = ebiten.NewImageFromImage(m)
		}
	}
	return out
}

// coverage returns how strongly a shape covers a pixel of the canonical
// top-left quarter, in [0,1]. Values are fed through the dither, so the useful
// range is the soft band around 0.5 rather than the extremes.
func coverage(shape, x, y int) float64 {
	fx, fy := float64(x)+0.5, float64(y)+0.5
	switch shape {
	case shapeVert:
		// Material presses down from above.
		return band(3.0 - fy)
	case shapeHoriz:
		// Material presses in from the side.
		return band(3.0 - fx)
	case shapeBoth:
		// Nearly the whole quarter, left concave at the far corner so the
		// underlying material pokes through diagonally.
		r := math.Hypot(float64(Quarter)-fx, float64(Quarter)-fy)
		return band(r - 4.6)
	default: // shapeDiag
		// A small nub hugging the outer corner.
		return band(2.7 - math.Hypot(fx, fy))
	}
}

// band converts a signed distance in pixels into a coverage ramp about 1.8px
// wide. Wider reads as mush at this scale; narrower loses the dither entirely.
func band(d float64) float64 {
	v := d/1.8 + 0.5
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}
