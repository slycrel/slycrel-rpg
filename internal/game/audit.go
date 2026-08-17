package game

import (
	"fmt"
	"io"
	"sort"
)

// Audit cross-checks the content tables against the asset manifest without
// opening a window. It answers the question that actually matters during
// curation: "which monsters are going to show up as a magenta box?"
func (g *Game) Audit(w io.Writer) error {
	var missing []string
	seen := map[string]bool{}

	check := func(key, owner string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		if !g.Assets.Has(key) {
			missing = append(missing, fmt.Sprintf("%-28s (%s)", key, owner))
		}
	}

	for biome, defs := range g.Data.Monsters {
		for _, d := range defs {
			check(d.Sprite, biome+"/"+d.ID)
		}
	}
	// Every sheet the creation screen offers, not the three the classes used to
	// be issued. A look the player can pick and the audit does not check is a
	// look that gets shipped as a magenta box.
	for _, l := range heroLooks {
		for _, anim := range []string{"idle", "up", "down", "left", "right"} {
			check(l.Key+"/"+anim, "player sprite")
		}
	}
	// The face roster is probed against the registry as it is built, so these
	// cannot be missing by construction — checking them anyway is what makes
	// the count in the summary honest about how much art the hero alone needs.
	for _, key := range g.heroFaces() {
		check(key, "hero portrait")
	}
	// Combat effects, enumerated from the same tables the game plays them out
	// of, so a key can never be in one and not the other.
	for _, key := range vfxKeys(g.Data.Spells) {
		check(key, "combat effect")
	}
	// Weather. Four sheets, and a missing one is a screen full of magenta
	// rather than a quiet fallback, because they are drawn over everything.
	for _, sheets := range fallSheets {
		for _, f := range sheets {
			check(f.key, "weather")
		}
	}
	// Unconditionally, and not merely because the roster usually contains it.
	// This is what portraitOf substitutes for anybody with no face of their own,
	// so if the probe above ever stops finding it the roster silently shortens
	// and the fallback silently breaks — which is the one combination that
	// would put a magenta box on a screen with nothing to explain it.
	check(defaultPortrait, "fallback portrait")

	total := 0
	for _, defs := range g.Data.Monsters {
		total += len(defs)
	}
	fmt.Fprintf(w, "content: %d monsters across %d biomes, %d weapons, %d armors, %d items, %d spells\n",
		total, len(g.Data.Monsters), len(g.Data.Weapons), len(g.Data.Armors), len(g.Data.Items), len(g.Data.Spells))
	fmt.Fprintf(w, "assets:  %d keys in manifest, %d checked\n", g.Assets.Count(), len(seen))

	sfx, files := g.Sound.Inventory()
	soundErrs := g.Sound.Verify()
	fmt.Fprintf(w, "sound:   %d cues, %d files, %d decode failures\n", sfx, files, len(soundErrs))
	for _, e := range soundErrs {
		fmt.Fprintf(w, "  %v\n", e)
	}

	if len(missing) == 0 {
		fmt.Fprintln(w, "all referenced art resolves.")
		return nil
	}
	sort.Strings(missing)
	fmt.Fprintf(w, "\n%d keys will render as placeholders:\n", len(missing))
	for _, m := range missing {
		fmt.Fprintf(w, "  %s\n", m)
	}
	return nil
}
