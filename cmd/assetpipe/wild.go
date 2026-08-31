package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
)

// Creatures for the overworld, one per monster kind.
//
// The overworld has never drawn a creature: an encounter was a dice roll in
// tall grass that cut straight to the battle screen. Giving the roll something
// the player can see coming needs a sprite at the scale the overworld draws
// people at, and that turned out to be the gap. `mob/` is painted portraits
// reduced from 128px — a battle-screen bust, not a thing that stands in a
// field. `foe/` is 64x64 interior walkers, which is the right scale but the
// wrong roster: five shapes, chosen to lurk in corridors.
//
// `pixelartrogue-likerpg` has sixteen creature rows drawn at 13-25px, against
// the 27x33 the hero's own art occupies inside its 64px box. Measured by
// standing them side by side on one ground line, which is the only way to
// settle a scale question.
//
// Keyed by `model.MonsterKind` rather than by species, because that is what the
// silhouette is for. A player who sees a skeleton knows the thing in the grass
// is undead before the transcript says so, and kind is the one property of a
// monster that reads at a glance and is worth knowing in advance. It also makes
// the lookup a string join and lets the audit check all eight.
//
// Licence: AFGameAssets / Pixogen, the Tier C reading in ASSET-LICENSING.md,
// same creator as three packs already shipping.

var wildSheet = []string{
	"pixelartrogue-likerpg", "Pixel Art Dungeon Crawler - AFGameAssets - V2",
	"PixelArtDungeonCrawler2.png",
}

// wildCut is one creature: the kind it stands for and where its standing pose
// sits on the sheet.
//
// The sheet lays each creature out as a row of poses — standing, walking,
// prone, striking — and only the first is taken. A wanderer that animated
// would be nicer and is not what decides whether the feature works; a wanderer
// that is legible at a glance is.
//
// Rectangles came from walking the sheet for opaque islands and grouping them
// into rows, not from eye-measuring a screenshot. Four rows are skipped: they
// are the pack's own player character in various kit, and a wanderer that looks
// like an adventurer is a wanderer the player will try to talk to.
type wildCut struct {
	kind string
	x, y int
	w, h int
	note string
}

var wildCuts = []wildCut{
	{"beast", 5, 201, 21, 17, "a crab, because a beast should read as an animal and not as a wolf-shaped man"},
	{"humanoid", 3, 101, 21, 21, "short, armed, and walking upright"},
	{"undead", 7, 35, 16, 23, "a skeleton holding a sword"},
	{"ooze", 5, 174, 20, 12, "a slime: the one silhouette nobody mistakes"},
	{"fey", 7, 325, 16, 21, "a small green thing that is probably up to something"},
	{"demon", 8, 69, 15, 21, "red, horned, and making no secret of it"},
	{"construct", 5, 423, 24, 19, "a boulder with intent"},
	{"aberrant", 8, 138, 13, 14, "a jellyfish, for the things with no better word"},
}

// buildWild cuts the overworld creatures.
func buildWild() error {
	sheetPath := filepath.Join(append([]string{rawRoot}, wildSheet...)...)
	sheet, err := readPNG(sheetPath)
	if err != nil {
		return fmt.Errorf("%w (run `assetpipe extract pixelartrogue-likerpg` first)", err)
	}

	outDir := filepath.Join(genRoot, "wild")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	for _, c := range wildCuts {
		r := image.Rect(c.x, c.y, c.x+c.w, c.y+c.h).Add(sheet.Bounds().Min)
		if !r.In(sheet.Bounds()) {
			return fmt.Errorf("%s: cut %v is outside the sheet %v", c.kind, r, sheet.Bounds())
		}
		// Exact size, no padding, for the same reason the location markers are
		// cut that way: Sprite.Foot counts transparent rows below the art, so a
		// creature in a padded box hovers above the grass it is standing on.
		dst := image.NewNRGBA(image.Rect(0, 0, c.w, c.h))
		draw.Draw(dst, dst.Bounds(), sheet, r.Min, draw.Src)
		if err := writePNG(filepath.Join(outDir, c.kind+".png"), dst); err != nil {
			return err
		}
	}
	fmt.Printf("ok     wild   %3d creatures, one per monster kind\n", len(wildCuts))
	return nil
}
