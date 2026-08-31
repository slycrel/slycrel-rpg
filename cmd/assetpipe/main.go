// Command assetpipe inventories and extracts the Humble "Complete RPG Creator
// Bundle" archives into assets-raw/ so the game can cherry-pick from them.
//
//	assetpipe build                run the whole derived-art pipeline, in order
//	assetpipe inventory            write docs/ASSET-INVENTORY.md from the zip indexes
//	assetpipe extract tier1        extract the packs listed in tier1 (see tiers below)
//	assetpipe extract <pack>...    extract named packs
//	assetpipe manifest             regenerate assets/manifest.json from what is extracted
//	assetpipe audio                regenerate assets/audio.json from the extracted sound packs
//	assetpipe props                rewrite prop sheets with translucent shadows
//	assetpipe icons                box-reduce every icon set to 16px
//	assetpipe garb                 cut armour icons from the crafting sheet
//	assetpipe arms                 cut weapon icons from the crafting sheet
//	assetpipe poi                  cut overworld location markers
//	assetpipe foes                 stack per-frame foe animations into strips
//	assetpipe wild                 cut overworld creatures, one per monster kind
//	assetpipe map                  regenerate the table in docs/ASSET-MAP.md
//	assetpipe bands                write tier recolours of the armour icons
//	assetpipe extract all          extract everything (the bundle is 16.7 GB; budget disk)
//	assetpipe find <substr>...     grep extracted filenames
//
// Nothing here touches the source bundle; it is treated as read-only.
package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// bundleSubdir is where the purchased bundle is expected to sit inside the
// user's home directory. Override the whole path with SLYCREL_BUNDLE.
const bundleSubdir = "Desktop/RPG Maker Stuff"

// defaultBundleRoot resolves the bundle location for the current user.
func defaultBundleRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return bundleSubdir
	}
	return filepath.Join(home, bundleSubdir)
}

// rawRoot is the extraction target, relative to the repo root. Gitignored.
const rawRoot = "assets-raw"

// tier1 is the subset needed for the current milestone: an Ultima-style
// overworld, zoom-in point-of-interest scenes, and a turn-based battle screen.
var tier1 = []string{
	// world + local scene tiles
	"manaseedpixelarttilesetcollection",
	"2dbuildingstown",
	"minicityassetpack",
	"minicityinteriorstilesets",
	"pixelartmedievalinteriors_windows",
	"pixelartmedievalinteriors2",
	"beowulfsrpgdungeontilesets",
	"pixelartgreentemple",
	"pixelartfarmlifeset",
	// overworld/local actors
	"pixelartrpgtopdowncharacters",
	"pixelartrpgnpc",
	"pixelartrpgtopdownenemies",
	"miniadventureheroeshumans",
	"miniadventureheroeselves",
	"minianimalsassetpack",
	// battle screen art
	"monstersavataricons_windows",
	"mobsavataricons_windows",
	"characteravataricons_windows",
	"pixelrpgdungeonsmonsters",
	"pixelartrpgvfx",
	// interface
	"guipro_fantasyrpg_gamedevmarket",
	"fantasynordicgui",
	"dialogueboxes_windows",
	"rpgandmmoui4",
	// icons
	"spellsandabilityicons_windows",
	"2dminimalskillicons",
	"beowulfsrpgmonsterloots",
	"magicrunespixelartassetpack",
	// audio
	"combatsoundsbundlecollection",
	"userinterfacesfxbundle",
	"ambiencesoundspack",
	"monstersoundsvolume1",
	"oldmagicianvoicepack",
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	root := os.Getenv("SLYCREL_BUNDLE")
	if root == "" {
		root = defaultBundleRoot()
	}

	switch os.Args[1] {
	case "build":
		must(buildAll())
	case "inventory":
		must(inventory(root))
	case "extract":
		if len(os.Args) < 3 {
			usage()
		}
		must(extract(root, os.Args[2:]))
	case "manifest":
		must(buildManifest())
	case "audio":
		must(buildAudioManifest())
	case "props":
		must(buildProps())
	case "icons":
		must(buildIcons())
	case "garb":
		must(buildGarb())
	case "arms":
		must(buildArms())
	case "poi":
		must(buildPOI())
	case "foes":
		must(buildFoes())
	case "wild":
		must(buildWild())
	case "map":
		must(buildAssetMap())
	case "bands":
		must(buildBands())
	case "find":
		if len(os.Args) < 3 {
			usage()
		}
		must(find(os.Args[2:]))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: assetpipe inventory | extract <tier1|all|pack...> | manifest | audio | props | icons | find <substr...>")
	os.Exit(2)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "assetpipe:", err)
		os.Exit(1)
	}
}

// pack is one zip in the bundle.
type pack struct {
	Name    string // basename without .zip
	Path    string // absolute path to the zip
	Group   string // bundle subfolder (Images/other/windows)
	Bytes   int64  // compressed size on disk
	Files   int    // entry count
	ByExt   map[string]int
	TopDirs []string
}

// scan reads every zip's central directory. It never decompresses anything,
// so a full scan of the ~2.3 GB bundle takes well under a second.
func scan(root string) ([]pack, error) {
	var packs []pack
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.EqualFold(filepath.Ext(path), ".zip") {
			return err
		}
		p := pack{
			Name:  strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			Path:  path,
			Group: filepath.Base(filepath.Dir(path)),
			Bytes: fi.Size(),
			ByExt: map[string]int{},
		}
		zr, zerr := zip.OpenReader(path)
		if zerr != nil {
			return nil // unreadable archive: report it as empty rather than aborting
		}
		defer zr.Close()
		tops := map[string]bool{}
		for _, f := range zr.File {
			if strings.HasPrefix(f.Name, "__MACOSX/") || strings.Contains(f.Name, "/._") {
				continue
			}
			if strings.HasSuffix(f.Name, "/") {
				continue
			}
			p.Files++
			ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.Name), "."))
			if ext != "" && !strings.Contains(ext, " ") {
				p.ByExt[ext]++
			}
			if i := strings.IndexByte(f.Name, '/'); i > 0 {
				tops[f.Name[:i]] = true
			}
		}
		for d := range tops {
			p.TopDirs = append(p.TopDirs, d)
		}
		sort.Strings(p.TopDirs)
		packs = append(packs, p)
		return nil
	})
	sort.Slice(packs, func(i, j int) bool { return packs[i].Name < packs[j].Name })
	return packs, err
}

func inventory(root string) error {
	packs, err := scan(root)
	if err != nil {
		return err
	}

	var b strings.Builder
	var totalBytes int64
	var totalFiles int
	globalExt := map[string]int{}
	for _, p := range packs {
		totalBytes += p.Bytes
		totalFiles += p.Files
		for e, n := range p.ByExt {
			globalExt[e] += n
		}
	}

	fmt.Fprintf(&b, "# Asset Inventory\n\n")
	fmt.Fprintf(&b, "Generated by `go run ./cmd/assetpipe inventory` against the purchased bundle,\n"+
		"which is treated as read-only and never committed. Point `SLYCREL_BUNDLE` at it if it\n"+
		"is not under `%s`.\n\n", bundleSubdir)
	fmt.Fprintf(&b, "**%d packs, %d files, %.1f GB compressed.**\n\n", len(packs), totalFiles, float64(totalBytes)/(1<<30))

	fmt.Fprintf(&b, "## File types across the whole bundle\n\n| ext | count | notes |\n|---|--:|---|\n")
	type kv struct {
		k string
		n int
	}
	var exts []kv
	for k, n := range globalExt {
		exts = append(exts, kv{k, n})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].n > exts[j].n })
	for _, e := range exts {
		if e.n < 5 {
			continue
		}
		fmt.Fprintf(&b, "| %s | %d | %s |\n", e.k, e.n, extNote(e.k))
	}

	fmt.Fprintf(&b, "\n## Packs\n\n| pack | group | files | size | contents | tier1 |\n|---|---|--:|--:|---|:-:|\n")
	inTier1 := map[string]bool{}
	for _, n := range tier1 {
		inTier1[n] = true
	}
	for _, p := range packs {
		mark := ""
		if inTier1[p.Name] {
			mark = "yes"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d | %s | %s | %s |\n",
			p.Name, p.Group, p.Files, human(p.Bytes), topExts(p.ByExt, 4), mark)
	}

	fmt.Fprintf(&b, "\n## Pack detail\n\n")
	for _, p := range packs {
		fmt.Fprintf(&b, "### `%s`\n\n- group: %s / files: %d / size: %s\n- types: %s\n",
			p.Name, p.Group, p.Files, human(p.Bytes), topExts(p.ByExt, 8))
		if len(p.TopDirs) > 0 {
			shown := p.TopDirs
			if len(shown) > 10 {
				shown = shown[:10]
			}
			fmt.Fprintf(&b, "- top level: %s\n", strings.Join(shown, ", "))
		}
		fmt.Fprintln(&b)
	}

	if err := os.MkdirAll("docs", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile("docs/ASSET-INVENTORY.md", []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote docs/ASSET-INVENTORY.md (%d packs, %d files, %.1f GB)\n",
		len(packs), totalFiles, float64(totalBytes)/(1<<30))
	return nil
}

func extNote(ext string) string {
	switch ext {
	case "png", "gif", "jpg", "jpeg":
		return "ready to use"
	case "ogg", "wav", "mp3":
		return "audio; ebiten wants ogg/vorbis or wav"
	case "tmx", "tsx":
		return "Tiled map/tileset - loadable directly"
	case "psd":
		return "needs `magick x.psd out.png` to flatten"
	case "unitypackage":
		return "tar.gz of a Unity project; assetpipe unpacks these"
	case "fla", "swf":
		return "Flash source; ignore, PNG siblings usually exist"
	case "pdn":
		return "paint.net source; ignore"
	case "tps":
		return "TexturePacker project; PNG+plist usually alongside"
	case "ttf", "otf":
		return "fonts"
	case "meta":
		return "Unity metadata; ignore"
	default:
		return ""
	}
}

func topExts(m map[string]int, n int) string {
	type kv struct {
		k string
		v int
	}
	var s []kv
	for k, v := range m {
		s = append(s, kv{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].v > s[j].v })
	if len(s) > n {
		s = s[:n]
	}
	var parts []string
	for _, e := range s {
		parts = append(parts, fmt.Sprintf("%d %s", e.v, e.k))
	}
	return strings.Join(parts, ", ")
}

func human(b int64) string {
	switch {
	case b > 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b > 1<<20:
		return fmt.Sprintf("%.0f MB", float64(b)/(1<<20))
	default:
		return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
	}
}

func extract(root string, args []string) error {
	packs, err := scan(root)
	if err != nil {
		return err
	}
	byName := map[string]pack{}
	for _, p := range packs {
		byName[p.Name] = p
	}

	var want []pack
	switch args[0] {
	case "all":
		want = packs
	case "tier1":
		for _, n := range tier1 {
			if p, ok := byName[n]; ok {
				want = append(want, p)
			} else {
				fmt.Fprintf(os.Stderr, "warning: tier1 pack %q not found in bundle\n", n)
			}
		}
	default:
		for _, n := range args {
			p, ok := byName[n]
			if !ok {
				return fmt.Errorf("unknown pack %q (see docs/ASSET-INVENTORY.md)", n)
			}
			want = append(want, p)
		}
	}

	for _, p := range want {
		dst := filepath.Join(rawRoot, p.Name)
		if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
			fmt.Printf("skip   %-45s (already extracted)\n", p.Name)
			continue
		}
		n, err := unzip(p.Path, dst)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error  %-45s %v\n", p.Name, err)
			continue
		}
		fmt.Printf("ok     %-45s %d files\n", p.Name, n)
	}
	return nil
}

// unzip extracts src into dst, skipping macOS resource forks. Nested zips and
// .unitypackage archives are left in place; unpack those on demand.
func unzip(src, dst string) (int, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return 0, err
	}
	defer zr.Close()

	count := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "__MACOSX/") || strings.Contains(f.Name, "/._") ||
			strings.HasSuffix(f.Name, ".DS_Store") {
			continue
		}
		// Reject entries that would escape dst (zip-slip).
		target := filepath.Join(dst, filepath.Clean("/"+f.Name))
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return count, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return count, err
		}
		if err := copyEntry(f, target); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func copyEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func find(substrs []string) error {
	lower := make([]string, len(substrs))
	for i, s := range substrs {
		lower[i] = strings.ToLower(s)
	}
	hits := 0
	err := filepath.Walk(rawRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		l := strings.ToLower(path)
		for _, s := range lower {
			if !strings.Contains(l, s) {
				return nil
			}
		}
		fmt.Println(path)
		hits++
		return nil
	})
	if hits == 0 {
		fmt.Fprintln(os.Stderr, "no matches (is the pack extracted?)")
	}
	return err
}
