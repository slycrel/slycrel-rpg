package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Some packs ship an animation as one file per frame rather than as a strip.
//
// The `foe/` keys the game already loads are vertical strips of 64x64 frames,
// which the manifest slices row-major; `pixelartdungeonlevel4` instead ships
// "Golem front 01.png" through "Golem front 04.png" as four separate files.
// Stacking them here rather than teaching the loader a second layout keeps one
// idea of what an animation is on disk, and the manifest entry for a stacked
// strip is identical to one for a strip that came that way.
//
// Licence: Acasas, Tier B in ASSET-LICENSING.md, the permissive end.

// frameSet is one animation to assemble: the files that make it up, in order,
// and the key fragment to write it under.
type frameSet struct {
	key    string // "golem/walk"
	dir    []string
	prefix string // filenames starting with this, case-insensitively
}

// stackedFoes are the animations to build. The Golem is the first creature in
// the game with a back and a side of its own, which is why the naming follows
// the hero sheets — walk/up/side — rather than the flat idle/walk/attack the
// other foes carry.
var stackedFoes = []frameSet{
	{"golem/walk", golemDir, "golem front"},
	{"golem/up", golemDir, "golem back"},
	{"golem/side", golemDir, "golem side"},
	{"golem/attack", golemDir, "golem attack front"},
	{"golem/attack_up", golemDir, "golem attack back"},
	{"golem/attack_side", golemDir, "golem attack side"},
}

var golemDir = []string{
	"pixelartdungeonlevel4", "Pixel Art Dungeon Level 4-By Acasas-", "PNGs", "Golem",
}

// buildFoes stacks per-frame animations into strips under _generated/foes/.
func buildFoes() error {
	outDir := filepath.Join(genRoot, "foes")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	total := 0
	for _, fs := range stackedFoes {
		dir := filepath.Join(append([]string{rawRoot}, fs.dir...)...)
		var files []string
		for _, f := range pngsIn(dir) {
			if strings.HasPrefix(strings.ToLower(base(f)), fs.prefix) {
				files = append(files, f)
			}
		}
		if len(files) == 0 {
			fmt.Fprintf(os.Stderr, "  no frames for %s under %s; skipping\n", fs.key, dir)
			continue
		}
		// Filenames end in 01..04, so lexical order is frame order. Sorting
		// explicitly rather than trusting the directory read is what stops an
		// animation playing backwards on a filesystem that hands them over in
		// another order.
		sort.Strings(files)

		strip, err := stack(files)
		if err != nil {
			return fmt.Errorf("%s: %w", fs.key, err)
		}
		out := filepath.Join(outDir, strings.ReplaceAll(fs.key, "/", "_")+".png")
		if err := writePNG(out, strip); err != nil {
			return err
		}
		total++
	}
	if total == 0 {
		return fmt.Errorf("no frame sets built; run `assetpipe extract pixelartdungeonlevel4` first")
	}
	fmt.Printf("ok     foes   %3d animations stacked\n", total)
	return nil
}

// stack piles frames into one vertical strip. Every frame has to be the same
// size, because a strip is sliced by a fixed frame height and a short frame
// would shift every frame after it by the difference.
func stack(files []string) (*image.NRGBA, error) {
	var frames []image.Image
	w, h := 0, 0
	for _, f := range files {
		img, err := readPNG(f)
		if err != nil {
			return nil, err
		}
		b := img.Bounds()
		if w == 0 {
			w, h = b.Dx(), b.Dy()
		} else if b.Dx() != w || b.Dy() != h {
			return nil, fmt.Errorf("%s is %dx%d, expected %dx%d", base(f), b.Dx(), b.Dy(), w, h)
		}
		frames = append(frames, img)
	}

	out := image.NewNRGBA(image.Rect(0, 0, w, h*len(frames)))
	for i, img := range frames {
		draw.Draw(out, image.Rect(0, i*h, w, (i+1)*h), img, img.Bounds().Min, draw.Src)
	}
	return out, nil
}
