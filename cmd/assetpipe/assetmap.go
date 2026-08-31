package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The asset map answers "what art does the game actually load, and where does
// each piece come from" — which ASSET-INVENTORY.md cannot, because it
// catalogues the purchased bundle rather than the manifest, and which
// ASSET-LICENSING.md only answers for the packs that happened to be wired in
// when it was written.
//
// The table is generated because the alternative is a hand-counted list that is
// wrong within two commits. The prose around it is not: a decision needs a
// person to have made it. So this rewrites only what sits between the markers
// and leaves everything else in the file alone.

const (
	mapDoc      = "docs/ASSET-MAP.md"
	mapBegin    = "<!-- BEGIN generated: go run ./cmd/assetpipe map -->"
	mapEnd      = "<!-- END generated -->"
	mapSkeleton = `# Asset Map

What the game loads, where it comes from, and what is still missing.

` + mapBegin + `
` + mapEnd + `
`
)

// buildAssetMap regenerates the table in docs/ASSET-MAP.md from the manifest.
func buildAssetMap() error {
	f, err := os.Open(filepath.Join("assets", "manifest.json"))
	if err != nil {
		return fmt.Errorf("%w (run `assetpipe manifest` first)", err)
	}
	defer f.Close()

	var doc struct {
		Entries []manifestEntry `json:"entries"`
	}
	if err := json.NewDecoder(f).Decode(&doc); err != nil {
		return err
	}

	// Group by key namespace, and inside each by the pack the file came from.
	type group struct {
		total int
		packs map[string]int
	}
	groups := map[string]*group{}
	for _, e := range doc.Entries {
		ns, _, _ := strings.Cut(e.Key, "/")
		g := groups[ns]
		if g == nil {
			g = &group{packs: map[string]int{}}
			groups[ns] = g
		}
		g.total++
		g.packs[packOf(e.File)]++
	}

	names := make([]string, 0, len(groups))
	for ns := range groups {
		names = append(names, ns)
	}
	sort.Slice(names, func(i, j int) bool {
		if a, b := groups[names[i]].total, groups[names[j]].total; a != b {
			return a > b
		}
		return names[i] < names[j]
	})

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", mapBegin)
	fmt.Fprintf(&b, "**%d keys across %d namespaces.**\n\n", len(doc.Entries), len(groups))
	fmt.Fprintf(&b, "| namespace | keys | sources |\n|---|--:|---|\n")
	for _, ns := range names {
		g := groups[ns]
		packs := make([]string, 0, len(g.packs))
		for p := range g.packs {
			packs = append(packs, p)
		}
		sort.Slice(packs, func(i, j int) bool {
			if a, b := g.packs[packs[i]], g.packs[packs[j]]; a != b {
				return a > b
			}
			return packs[i] < packs[j]
		})
		var cells []string
		for _, p := range packs {
			cells = append(cells, fmt.Sprintf("`%s` %d", p, g.packs[p]))
		}
		fmt.Fprintf(&b, "| `%s/` | %d | %s |\n", ns, g.total, strings.Join(cells, ", "))
	}
	fmt.Fprintf(&b, "\n%s", mapEnd)

	return replaceBetween(mapDoc, mapBegin, mapEnd, b.String())
}

// packOf names the pack a manifest path came from. Pipeline output is reported
// as the step that wrote it rather than as one undifferentiated `_generated`,
// since "which of these did a program make, and which one" is the question the
// table exists to answer.
func packOf(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	if len(parts) < 2 {
		return file
	}
	if parts[1] != "_generated" {
		return parts[1]
	}
	if len(parts) > 2 {
		return "_generated/" + parts[2]
	}
	return "_generated"
}

// replaceBetween rewrites the region between two markers, creating the file
// from a skeleton if it is not there yet. Anything outside the markers is
// carried through untouched — that is where the reasoning lives.
func replaceBetween(path, begin, end, body string) error {
	old, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		old = []byte(mapSkeleton)
	} else if err != nil {
		return err
	}

	s := string(old)
	i := strings.Index(s, begin)
	j := strings.Index(s, end)
	if i < 0 || j < i {
		return fmt.Errorf("%s: markers missing or out of order; expected %q then %q", path, begin, end)
	}
	out := s[:i] + body + s[j+len(end):]
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Printf("ok     map    %s\n", path)
	return nil
}
