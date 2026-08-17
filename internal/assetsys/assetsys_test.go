package assetsys

import (
	"image"
	"image/color"
	"testing"
)

// Character art is drawn into a box much larger than a tile — the hero sheets
// are 64x64 on a 16-pixel grid — and the feet do not reach the bottom of it.
// Anchoring a sprite by its frame therefore floats the character above the tile
// it occupies, which is what "the hit box for the walls is off by one" was.
func TestSpritesKnowWhereTheirFeetAre(t *testing.T) {
	const w, h = 64, 64
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// A figure occupying rows 16..47, leaving exactly one tile of padding below.
	for y := 16; y < 48; y++ {
		for x := 24; x < 40; x++ {
			img.Set(x, y, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		}
	}

	if got := footPadding(img, w, h); got != 16 {
		t.Errorf("foot padding measured %d, the art ends 16 rows above the frame", got)
	}
}

// Art that fills its frame needs no correction, and must not get one.
func TestAFullFrameSpriteIsNotShifted(t *testing.T) {
	const w, h = 16, 16
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{0xFF, 0xFF, 0xFF, 0xFF})
		}
	}
	if got := footPadding(img, w, h); got != 0 {
		t.Errorf("a sprite with no padding reports %d rows of it", got)
	}
}

// And an entirely transparent frame must not report the whole height, or a
// placeholder would drag everything drawn with it off the bottom of the world.
func TestAnEmptyFrameReportsNoPadding(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	if got := footPadding(img, 16, 16); got != 0 {
		t.Errorf("an empty frame reports %d rows of padding", got)
	}
}
