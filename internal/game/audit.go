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
	for _, class := range []string{"fighter", "thief", "mage"} {
		for _, anim := range []string{"idle", "up", "down", "left", "right"} {
			check("hero/"+class+"/"+anim, "player sprite")
		}
	}
	check("portrait/male/m_01", "battle portrait")

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
