package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifestEntry mirrors assetsys.Entry. It is duplicated rather than imported
// so the pipeline tool stays independent of the game packages.
type manifestEntry struct {
	Key    string `json:"key"`
	File   string `json:"file"`
	FrameW int    `json:"frameW,omitempty"`
	FrameH int    `json:"frameH,omitempty"`
	Rect   []int  `json:"rect,omitempty"`
}

// buildManifest walks the extracted packs and writes assets/manifest.json.
// Paths are recorded relative to the repo root, so the game reads straight out
// of assets-raw/ with no copying. A later curation pass can rewrite these to
// point at trimmed copies under assets/ without touching any game code.
func buildManifest() error {
	var entries []manifestEntry

	add := func(key, file string, fw, fh int) {
		if _, err := os.Stat(file); err != nil {
			return
		}
		entries = append(entries, manifestEntry{Key: key, File: file, FrameW: fw, FrameH: fh})
	}

	// Battle portraits: one PNG per creature, used whole.
	mobDir := filepath.Join(rawRoot, "mobsavataricons_windows", "mobsavataricons")
	for _, f := range pngsIn(mobDir) {
		add("mob/"+slug(base(f)), f, 0, 0)
	}
	bossDir := filepath.Join(rawRoot, "monstersavataricons_windows", "monstersavataricons")
	for _, f := range pngsIn(bossDir) {
		add("boss/"+slug(base(f)), f, 0, 0)
	}
	for _, sex := range []string{"male", "female"} {
		dir := filepath.Join(rawRoot, "characteravataricons_windows", "characteravataricons", sex)
		for _, f := range pngsIn(dir) {
			add("portrait/"+sex+"/"+slug(base(f)), f, 0, 0)
		}
	}

	// Overworld actors. These sheets are 64px-wide vertical strips of 64x64
	// frames; the loader slices them row-major, which for a one-column sheet
	// is exactly the animation order.
	charRoot := filepath.Join(rawRoot, "pixelartrpgtopdowncharacters",
		"Pixel Art Top-Down RPG Characters - AfGameAssets - V2 Walking 4 Directions")
	// The pack's file naming is inconsistent between characters, so each
	// hero lists its own filenames rather than assuming a pattern.
	heroes := []struct {
		key   string
		dir   string
		files map[string]string // anim -> filename
	}{
		{"fighter", "Viking", map[string]string{
			"idle": "VikingIdle.png", "down": "Viking_Walk_Down.png",
			"left": "Viking_Walk_Left.png", "right": "Viking_Walk_Right.png",
			"up": "Viking_Walk_Up.png", "attack": "Viking_Attack_01.png",
			"dead": "VikingDead.png",
		}},
		{"thief", "MagicRogue", map[string]string{
			"idle": "MagicRogue_Idle.png", "down": "MagicRogue_Walk_Down.png",
			"left": "MagicRogue_Walk_Left.png", "right": "MagicRogue_Walk_Right.png",
			"up": "MagicRogue_Walk_Up.png", "attack": "MagicRogue_Attack_01.png",
			"dead": "MagicRogue_Dead.png",
		}},
		{"mage", "BloodMage", map[string]string{
			"idle": "BloodMage_Idle.png", "down": "BloodMage_Walking_Down.png",
			"left": "BloodMage_Walking_Side_Left.png", "right": "BloodMage_Walking_Side_Right.png",
			"up": "BloodMage_Walking_Up.png", "attack": "BloodMage_Attack_01.png",
			"dead": "BloodMage_Dead.png",
		}},
		{"druid", "Druid", map[string]string{
			"idle": "Druid_Idle.png", "down": "Druid_Walk_Down.png",
			"left": "Druid_Walk_Left.png", "right": "Druid_Walk_Right.png",
			"up": "Druid_Walk_Up.png", "attack": "Druid_Attack_01.png",
			"dead": "Druid_Dead.png",
		}},
	}
	for _, h := range heroes {
		for anim, file := range h.files {
			add(fmt.Sprintf("hero/%s/%s", h.key, anim), filepath.Join(charRoot, h.dir, file), 64, 64)
		}
	}

	// Walking enemies for dungeon interiors.
	foeRoot := filepath.Join(rawRoot, "pixelartrpgtopdownenemies",
		"Pixel Art Top-Down RPG Enemies - AfGameAssets - V2 - Walking")
	for _, d := range subdirs(foeRoot) {
		name := slug(filepath.Base(d))
		for _, f := range pngsIn(d) {
			anim := strings.ToLower(base(f))
			if i := strings.Index(anim, "_"); i >= 0 {
				anim = anim[i+1:]
			}
			add(fmt.Sprintf("foe/%s/%s", name, slug(anim)), f, 64, 64)
		}
	}

	// Ground textures. Mana Seed's seasonal "wang tiles" sheets carry a legend
	// strip of six flat, seamlessly tiling textures at a fixed offset. Those
	// swatches are what the autotiler composites; its own corner masks handle
	// the blending, so none of the packs' bespoke permutation layouts need to
	// be decoded.
	const wangLegendY = 64
	seasons := map[string]string{
		"summer": filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
			"20.04c - Summer Forest", "packaged", "summer sheets", "summer forest wang tiles.png"),
		"autumn": filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
			"20.06a - Autumn Forest", "packaged", "autumn sheets", "autumn forest wang tiles.png"),
		"winter": filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
			"20.07a - Winter Forest", "packaged", "winter sheets", "winter forest wang tiles (snowy).png"),
		"spring": filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
			"20.05c - Spring Forest", "packaged", "spring sheets", "spring forest wang tiles.png"),
	}
	// Strip order within every legend: dirt, light grass, dark grass, stone,
	// shallow water, deep water.
	swatch := []string{"dirt", "grass", "darkgrass", "stone", "shallow", "deep"}
	for season, file := range seasons {
		for i, name := range swatch {
			key := fmt.Sprintf("ground/%s_%s", season, name)
			if _, err := os.Stat(file); err != nil {
				continue
			}
			entries = append(entries, manifestEntry{
				Key: key, File: file, FrameW: 16, FrameH: 16,
				Rect: []int{i * 16, wangLegendY, 16, 16},
			})
		}
	}

	// The seasonal legends have no true sand, so it comes from the desert pack.
	// This patch was picked by tiling candidates 4x4 and keeping the one with no
	// visible repeating motif; most of that sheet is decorated with dune ripples
	// that turn into an obvious grid when tiled.
	desert := filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"23.03a - Desert Sands", "packaged", "desert sheets", "desert sands v01.png")
	if _, err := os.Stat(desert); err == nil {
		entries = append(entries, manifestEntry{
			Key: "ground/sand", File: desert, FrameW: 16, FrameH: 16,
			Rect: []int{96, 0, 16, 16},
		})
	}

	// Scenery props. These sheets are plain grids of equal frames, so the
	// loader's row-major slicing gives stable indices to select from.
	summerSheets := filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"20.04c - Summer Forest", "packaged", "summer sheets")
	add("prop/summer16", filepath.Join(summerSheets, "summer 16x16.png"), 16, 16)
	// Prefer the de-shadowed copy from `assetpipe props`; fall back to the
	// original so the manifest is still usable before that step is run.
	summer32 := filepath.Join(genRoot, "props", "summer 32x32.png")
	if _, err := os.Stat(summer32); err != nil {
		summer32 = filepath.Join(summerSheets, "summer 32x32.png")
	}
	add("prop/summer32", summer32, 32, 32)
	add("prop/summer1632", filepath.Join(summerSheets, "summer 16x32.png"), 16, 32)
	add("prop/desert16", filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"23.03a - Desert Sands", "packaged", "desert sheets", "desert 16x16.png"), 16, 16)

	// Interior clutter.
	add("prop/cozy16", filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"19.04b - Cozy Furnishings", "packaged", "cozy furnishings 16x16.png"), 16, 16)
	add("prop/cave16", filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"19.10a - Muddy Cave", "packaged", "muddy cave sheets", "muddy cave 16x16 v1.png"), 16, 16)

	// Icons. Two pixel-art sets at 32px — loot and runes — which is exactly UI
	// scale, plus the painted ability set for weapons, where 128px divides
	// cleanly by four. Loot filenames carry the thing they depict
	// ("monloot_57_water_lilly_x"), so keys stay semantic rather than numeric.
	// Icons are read from the reduced copies written by `assetpipe icons`,
	// which box-averages them down to 16px. Falling back to the originals keeps
	// the manifest usable before that step has run, just blurrier on screen.
	for _, set := range []struct {
		prefix   string
		original []string
	}{
		{"loot", []string{"beowulfsrpgmonsterloots", "Beowulf_RPG_Monsters_Loot", "monster_loots_size_32x32"}},
		{"rune", []string{"magicrunespixelartassetpack", "Beowulf's_Magic_Runes", "runes_size_x_32x32"}},
		{"ab", []string{"spellsandabilityicons_windows", "png", "128x128"}},
	} {
		reduced := filepath.Join(genRoot, "icons", set.prefix)
		if files := pngsIn(reduced); len(files) > 0 {
			for _, f := range files {
				add("icon/"+set.prefix+"/"+base(f), f, 0, 0)
			}
			continue
		}
		for _, f := range pngsIn(filepath.Join(append([]string{rawRoot}, set.original...)...)) {
			add("icon/"+set.prefix+"/"+iconName(set.prefix, f), f, 0, 0)
		}
	}
	// The loot pack misspells one 32px file as "whetstonel_x.png"; alias it so
	// content can refer to the thing by its name rather than the typo.
	for _, e := range entries {
		if e.Key == "icon/loot/whetstonel" {
			add("icon/loot/whetstone", e.File, 0, 0)
			break
		}
	}

	// Townspeople.
	npcRoot := filepath.Join(rawRoot, "pixelartrpgnpc", "Pixel Art Top-Down RPG NPC - AfGameAssets - V1")
	for _, f := range pngsIn(npcRoot) {
		add("npc/"+slug(base(f)), f, 64, 64)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })

	if err := os.MkdirAll("assets", 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(struct {
		Entries []manifestEntry `json:"entries"`
	}{entries}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("assets", "manifest.json"), out, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote assets/manifest.json (%d keys)\n", len(entries))
	return nil
}

func pngsIn(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out
}

func subdirs(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

func base(path string) string {
	b := filepath.Base(path)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// slug normalises a filename into a stable lowercase asset key fragment.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "_", "-", "_", ".", "_").Replace(s)
	return s
}
