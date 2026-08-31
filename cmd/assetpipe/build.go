package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The derived-art pipeline, in dependency order.
//
// It is a list rather than a paragraph because the order is real and getting it
// wrong is silent. `bands` reads what `icons`, `garb` and `arms` write, and it
// reads the gear tables to learn which pictures to band; `manifest` enumerates
// every directory the steps above produce; `map` reads the manifest back. Run
// `bands` before `arms` and it bands last run's weapons. Run `manifest` before
// `bands` and the new keys are simply absent, the game falls back, the audit
// says "all referenced art resolves" because the content still names what it
// named — and nothing anywhere reports a problem.
//
// So the sequence lives here, and `assetpipe build` is the only thing anyone
// should need to run after touching art or the gear tables.
var pipeline = []step{
	{
		name: "props", run: buildProps,
		writes: "props/", from: "manaseedpixelarttilesetcollection",
		what: "prop sheets with the baked purple shadows made translucent",
	},
	{
		name: "icons", run: buildIcons,
		writes: "icons/loot/, icons/rune/, icons/ab/",
		from:   "beowulfsrpgmonsterloots, magicrunespixelartassetpack, spellsandabilityicons_windows",
		what:   "every icon set box-reduced to 16px",
	},
	{
		name: "garb", run: buildGarb,
		writes: "icons/garb/", from: "pixelartminingcrafting",
		what: "ten garments cut from the paper-doll sheet's rows 2-3",
	},
	{
		name: "arms", run: buildArms,
		writes: "icons/arms/", from: "pixelartminingcrafting",
		what: "twenty-four weapons cut from the same sheet's columns 5-7",
	},
	{
		name: "poi", run: buildPOI,
		writes: "icons/poi/", from: "pixelartrogue-likerpg",
		what: "nine overworld location markers, at native size",
	},
	{
		name: "foes", run: buildFoes,
		writes: "foes/, decor/", from: "pixelartdungeonlevel4",
		what: "per-frame animations stacked into vertical strips: the golem, and a brazier",
	},
	{
		name: "wild", run: buildWild,
		writes: "wild/", from: "pixelartrogue-likerpg",
		what: "one overworld creature per monster kind",
	},
	{
		name: "bands", run: buildBands,
		writes: "bands/", from: "icons/ and the gear tables",
		what: "a six-rung tier recolour of every icon armour and weapons name",
	},
	{
		name: "manifest", run: buildManifest,
		writes: "assets/manifest.json", from: "everything above, plus the raw packs",
		what: "the key-to-file table the game loads at runtime",
	},
	{
		name: "audio", run: buildAudioManifest,
		writes: "assets/audio.json", from: "the five sound packs",
		what: "the cue-to-files table the mixer loads",
	},
	{
		name: "map", run: buildAssetMap,
		writes: mapDoc, from: "assets/manifest.json",
		what: "the generated table in the asset map",
	},
}

type step struct {
	name   string
	run    func() error
	writes string
	from   string
	what   string
}

// buildAll runs the whole pipeline and records what wrote what.
func buildAll() error {
	for _, s := range pipeline {
		if err := s.run(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return writeProvenance()
}

// writeProvenance drops a note in the generated tree saying which step produced
// which directory and out of what.
//
// The asset map infers this from the directory layout, which holds only as long
// as every step owns a directory and nobody ever looks at a stray PNG and
// wonders where it came from. Everything under here is a modified copy of
// purchased art; being able to answer "what made this, and from which pack" is
// worth ten lines.
//
// No timestamp on purpose. This file is rewritten on every build and a clock in
// it would make every run differ from the last for no reason.
func writeProvenance() error {
	var b strings.Builder
	b.WriteString("Pipeline-derived art. Every file under this directory was written by\n")
	b.WriteString("`go run ./cmd/assetpipe build` out of the purchased bundle, and none of it\n")
	b.WriteString("is in version control. Delete the whole tree and rebuild it at any time.\n\n")
	b.WriteString("Steps run in this order; the order matters (see cmd/assetpipe/build.go).\n\n")
	for _, s := range pipeline {
		fmt.Fprintf(&b, "%-9s -> %s\n", s.name, s.writes)
		fmt.Fprintf(&b, "%-9s    %s\n", "", s.what)
		fmt.Fprintf(&b, "%-9s    from: %s\n\n", "", s.from)
	}

	if err := os.MkdirAll(genRoot, 0o755); err != nil {
		return err
	}
	out := filepath.Join(genRoot, "PROVENANCE.txt")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("ok     build  %d steps -> %s\n", len(pipeline), out)
	return nil
}
