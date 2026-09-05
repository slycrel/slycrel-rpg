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

	// Foes whose frames arrived as separate files and were stacked into strips
	// by `assetpipe foes`. Same 64x64 slicing as the sheets above, because
	// that is the point of stacking them.
	for _, f := range pngsIn(filepath.Join(genRoot, "foes")) {
		add("foe/"+strings.Replace(base(f), "_", "/", 1), f, 64, 64)
	}

	// Scenery that animates, stacked by `assetpipe foes`. The frame geometry is
	// spelled out rather than inferred: a strip on disk is just a tall image,
	// and nothing in the file says where one frame ends.
	for _, d := range []struct {
		key    string
		fw, fh int
	}{
		{"brazier", 19, 64},
	} {
		add("decor/"+d.key, filepath.Join(genRoot, "decor", d.key+".png"), d.fw, d.fh)
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

	// Weather. The rain and snow sheets are eight-frame animations laid out
	// horizontally, two tiles wide and eight high, which the loader slices
	// row-major into exactly the animation order.
	weather := filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"20.07b - Weather Effects", "packaged")
	for _, f := range []struct{ key, file string }{
		{"weather/rain_light", "weather effects, rain light anim 32x128.png"},
		{"weather/rain_heavy", "weather effects, rain heavy anim 32x128.png"},
		{"weather/snow_light", "weather effects, snow light anim 32x128.png"},
		{"weather/snow_heavy", "weather effects, snow heavy anim 32x128.png"},
	} {
		add(f.key, filepath.Join(weather, f.file), 32, 128)
	}

	// Combat effects. Every sheet in the pack is 64x384 — six 64x64 frames
	// stacked vertically — which the loader slices row-major into exactly the
	// animation order. Only the ones something actually plays are listed;
	// naming all 115 would put a hundred keys in the audit that nothing reads.
	vfx := filepath.Join(rawRoot, "pixelartrpgvfx",
		"Pixel Art RPG VFX - AfGameAssets - V3")
	for _, f := range []struct{ key, dir, file string }{
		{"vfx/slash_a", "Attack Slash", "Slash_attack_002"},
		{"vfx/slash_b", "Attack Slash", "Slash_attack_005"},
		{"vfx/slash_c", "Attack Slash", "Slash_attack_007"},
		{"vfx/fire", "Fire", "FireExplosion1"},
		{"vfx/burn", "Fire", "FireFlamme"},
		{"vfx/ice", "Ice", "IceSpike"},
		{"vfx/shock", "Electricity", "ElectricExplosion"},
		{"vfx/bolt", "Electricity", "ElectricLighting1"},
		{"vfx/holy", "Holy", "HolyBlessing"},
		{"vfx/wings", "Holy", "HolyWings"},
		{"vfx/cross", "Holy", "HolyCross"},
		{"vfx/void", "Void", "VoidExplosion1"},
		{"vfx/drain", "Void", "VoidBlackHole"},
		{"vfx/wind", "Wind", "WindGust"},
		{"vfx/poison", "Earth", "EarthGrow"},
		{"vfx/rock", "Earth", "EarthRock"},
		{"vfx/boom", "Explosion", "Explosion_03"},
	} {
		add(f.key, filepath.Join(vfx, f.dir, f.file+".png"), 64, 64)
	}

	// Interior clutter, at two scales.
	//
	// The 16px sheet is objects on a surface — bottles, bread, books — and the
	// 32px one is the furniture they sit on: tables, benches, beds, and the
	// mounted boar's head that makes a room a tavern rather than a room. Both
	// are needed and they are not interchangeable: a table drawn at sixteen
	// pixels is a stool, and a tankard drawn at thirty-two is a barrel.
	add("prop/cozy16", filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"19.04b - Cozy Furnishings", "packaged", "cozy furnishings 16x16.png"), 16, 16)
	add("prop/cozy32", filepath.Join(rawRoot, "manaseedpixelarttilesetcollection",
		"19.04b - Cozy Furnishings", "packaged", "cozy furnishings 32x32.png"), 32, 32)
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
	// Tier bands, written by `assetpipe bands`: the same icon re-shaded through
	// a quality ramp so a shop row says which of two coats is the better one
	// without the player reading the price. Filenames already carry the "_t3",
	// so the key is just the set and the file.
	for _, set := range []string{"loot", "rune", "ab", "garb", "arms"} {
		for _, f := range pngsIn(filepath.Join(bandRoot(), set)) {
			add("icon/band/"+set+"/"+base(f), f, 0, 0)
		}
	}

	// Garment icons, cut from the crafting sheet by `assetpipe garb`. Unlike
	// the sets above there is no original to fall back to — the source is one
	// sheet, and a cell of it is not a file.
	for _, set := range []string{"garb", "arms"} {
		for _, f := range pngsIn(filepath.Join(genRoot, "icons", set)) {
			add("icon/"+set+"/"+base(f), f, 0, 0)
		}
	}

	// Overworld creatures, cut by `assetpipe wild`, keyed by monster kind.
	for _, f := range pngsIn(filepath.Join(genRoot, "wild")) {
		add("wild/"+base(f), f, 0, 0)
	}

	// Overworld location markers, cut by `assetpipe poi`. Their own namespace
	// rather than icon/: these are drawn on the map at native size, not fitted
	// into a menu row's 16px box.
	for _, f := range pngsIn(filepath.Join(genRoot, "icons", "poi")) {
		add("poi/"+base(f), f, 0, 0)
	}

	// The loot pack misspells one 32px file as "whetstonel_x.png"; alias it so
	// content can refer to the thing by its name rather than the typo.
	for _, e := range entries {
		if e.Key == "icon/loot/whetstonel" {
			add("icon/loot/whetstone", e.File, 0, 0)
			break
		}
	}

	// The oddity's residents, from the same vendor family as the monster
	// portraits already in use and at exactly the same 256px — masked things,
	// helmeted things, and one broad-faced fellow who will not say what he is
	// doing here. They slot into the battle screen with no work at all, which
	// is the whole reason they were worth going back to the bundle for.
	//
	// Only the A and K families. M is ordinary humans in jackets, which is a
	// face this game already has seventy-six of.
	sci := filepath.Join(rawRoot, "sci-ficharactersicons",
		"Sci-Fi_Characters_Icons_png", "transparent")
	for _, f := range pngsIn(sci) {
		n := slug(base(f))
		if !strings.HasPrefix(n, "a_") && !strings.HasPrefix(n, "k_") {
			continue
		}
		add("mob/sci_"+strings.TrimSuffix(n, "_t"), f, 0, 0)
	}

	// The oddity's furniture.
	//
	// Two thirds of the bundle is sci-fi and cyberpunk that this game has no
	// use for — except as an over-the-top joke zone, which is exactly what an
	// oddity location is for. That has been written in the plan since the
	// asset budget was first counted and never cashed.
	//
	// Only the *furniture* is taken, and that is the joke working rather than a
	// limit: the people standing next to a vending machine are ordinary
	// villagers who treat it as a wall with a slot in it. Importing cyberpunk
	// characters as well would have somebody in the frame who is in on it, and
	// the rule this game's writing follows is that nobody ever is.
	//
	// Whole images rather than sheets, so footPadding anchors each one on its
	// own base and a sign four tiles tall stands on the ground instead of
	// hovering over it.
	odd := filepath.Join(rawRoot, "pixelartcyberpunkcity",
		"Pixel Art Cyberpunk City - The Game Assets Mine -", "PNG")
	for key, file := range map[string]string{
		"odd/vending1": "vending machine 01.png",
		"odd/vending2": "vending machine 02.png",
		"odd/vending3": "vending machine 03.png",
		"odd/vending4": "vending machine 04.png",
		"odd/sign1":    "sign 01.png",
		"odd/sign2":    "sign 09.png",
		"odd/sign3":    "sign 11.png",
		"odd/sign4":    "sign 15.png",
		"odd/sign5":    "sign 16.png",
		"odd/daub1":    "graffiti 01.png",
		"odd/daub2":    "graffiti 02.png",
		"odd/daub3":    "graffiti 03.png",
		"odd/daub4":    "graffiti 04.png",
		"odd/bin1":     "trash 01.png",
		"odd/bin2":     "trash 02.png",
		"odd/bin3":     "trash 03.png",
		"odd/bin4":     "trash 04.png",
		"odd/barrier1": "barrier 01.png",
		"odd/barrier2": "barrier 02.png",
		"odd/lanterns": "lanterns.png",
		"odd/car":      "car01.png",
		"odd/metro":    "metro entrance 01.png",
	} {
		add(key, filepath.Join(odd, file), 0, 0)
	}

	// The oddity's second supplier. The cyberpunk pack is a lit city at night;
	// this one is a roadside after everybody left, and the two overlap exactly
	// where the joke lives — a sofa, a stop sign, a parked car and a barrel are
	// the wrong century in a way a neon hoarding is not, because a villager has
	// no word for any of them either. Nothing scenic is taken: the pack's
	// streets, sand and grass are a whole second ground palette and
	// groundMaterials is deliberately one.
	waste := filepath.Join(rawRoot, "pixelartwasteland",
		"Pixel Art Waste Land -by Acasas-", "PNG")
	for key, file := range map[string]string{
		"odd/sofa":     "sofa.png",
		"odd/stopsign": "stop sign.png",
		"odd/sign6":    "sign 01.png",
		"odd/car2":     "car 02.png",
		"odd/barrel":   "barrell.png",
		"odd/bin5":     "trash 01.png",
	} {
		add(key, filepath.Join(waste, file), 0, 0)
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
