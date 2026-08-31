// Command pixelsmith drafts a 16x16 icon with a local language model, in the
// style of the icons the game already has.
//
//	pixelsmith gen -name spear -n 12          draft candidates, write a review sheet
//	pixelsmith adopt -name spear -pick 5      keep one, as data/art/spear.txt
//
// Why a language model and not an image model. The target is sixteen pixels
// square on a six-colour palette. A diffusion model does not draw that; it
// draws a thousand pixels of something pixel-art-*shaped* and leaves you to
// reduce it, which is the exact failure the ability icons already demonstrate —
// a painted icon squeezed into a 16px box is a smudge. A language model can
// emit the grid itself: the right size, the right palette, no resampling, and
// every pixel inspectable before it is accepted.
//
// The style comes from the icons already in the pack, handed over as few-shot
// examples in the same grid notation. That is what "seeded" means here.
//
// Nothing about this runs in the asset pipeline. A model is not deterministic
// and `assetpipe build` must stay byte-reproducible, so the output of this tool
// is a *grid of palette indices* committed under data/art/, and the pipeline
// renders it. The committed file therefore holds no purchased pixels at all —
// only which of six palette slots each pixel uses — and the palette itself is
// read from the extracted pack at build time.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/slycrel/slycrel-rpg/internal/pixelpal"
)

const (
	// Size is the icon grid. Everything the game draws in a menu row is fitted
	// into this, and the existing weapon cuts are exactly this.
	Size = 16
	// artDir is where an adopted grid lives. Committed: it is authored content,
	// like the writing in data/text.
	artDir = "data/art"
	// seedDir is where the style examples come from.
	seedDir = "assets-raw/_generated/icons/arms"
)

// palette is loaded once, from the same rule assetpipe renders with. Never
// hardcode it here: the two tools disagreeing about which colour a slot is
// produces a valid grid that renders as the wrong picture, and nothing catches
// it. See internal/pixelpal.
var palette []pixelpal.Colour

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	pal, err := pixelpal.Load(seedDir)
	must(err)
	palette = pal

	switch os.Args[1] {
	case "gen":
		must(gen(os.Args[2:]))
	case "adopt":
		must(adopt(os.Args[2:]))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pixelsmith gen -name <n> [-n 12] [-model m] [-seeds a,b,c]")
	fmt.Fprintln(os.Stderr, "       pixelsmith adopt -name <n> -pick <i>")
	os.Exit(2)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "pixelsmith:", err)
		os.Exit(1)
	}
}

// --- drafting ---------------------------------------------------------------

func gen(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	name := fs.String("name", "", "what to draw, e.g. spear")
	desc := fs.String("desc", "", "a sentence describing it; defaults to the name")
	n := fs.Int("n", 12, "how many candidates to draft")
	model := fs.String("model", "gpt-oss:20b", "ollama model")
	seeds := fs.String("seeds", "sword1,axe1,dagger1,hammer1,staff3", "icons to measure the house style from")
	head := fs.Int("head", 0, "if set, ask only for a head this many cells square and draw the haft in code")
	fs.Parse(args)
	if *name == "" {
		return fmt.Errorf("-name is required")
	}
	what := *desc
	if what == "" {
		what = *name
	}

	prompt, err := buildPrompt(strings.Split(*seeds, ","), *name, what, *head)
	if err != nil {
		return err
	}

	out := filepath.Join("shots", "pixelsmith-"+*name)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	var kept []grid
	var heads [][][]byte
	for i := 0; len(kept) < *n && i < *n*8; i++ {
		body, err := ask(*model, prompt, 0.9)
		if err != nil {
			return err
		}
		var g grid
		var hd [][]byte
		var ok bool
		if *head > 0 {
			g, hd, ok = hafted(body, *head)
			// Judge the point before accepting it. Anything that is not
			// tapering, connected and taller than wide is an axe or a knob,
			// and looking at forty of those is how the last pass went.
			if ok && headScore(hd) <= 0 {
				ok = false
			}
		} else {
			g, ok = parseGrid(body)
		}
		if !ok {
			continue // a reply that is not a grid is simply discarded
		}
		heads = append(heads, hd)
		kept = append(kept, g)
		if err := os.WriteFile(filepath.Join(out, fmt.Sprintf("%02d.txt", len(kept))),
			[]byte(g.String()), 0o644); err != nil {
			return err
		}
		fmt.Printf("  candidate %02d\n", len(kept))
	}
	if len(kept) == 0 {
		return fmt.Errorf("no usable grids came back from %s", *model)
	}

	// Best-first, so the eye starts where the machine thinks it should. In head
	// mode that means the head's own score: the haft is identical on every
	// candidate and drowns out the only part that differs.
	// Sorted through an index, because kept and heads are parallel and sorting
	// one of them alone silently pairs every candidate with somebody else's
	// score.
	idx := make([]int, len(kept))
	for i := range idx {
		idx[i] = i
	}
	rank := func(i int) float64 { return score(kept[i]) }
	if *head > 0 {
		rank = func(i int) float64 { return headScore(heads[i]) }
	}
	sort.SliceStable(idx, func(a, b int) bool { return rank(idx[a]) > rank(idx[b]) })
	sorted := make([]grid, len(kept))
	for i, j := range idx {
		sorted[i] = kept[j]
	}
	kept = sorted
	for i, g := range kept {
		os.WriteFile(filepath.Join(out, fmt.Sprintf("%02d.txt", i+1)), []byte(g.String()), 0o644)
	}

	sheet := filepath.Join(out, "sheet.png")
	if err := writeSheet(sheet, kept); err != nil {
		return err
	}
	fmt.Printf("ok  %d candidates -> %s\n", len(kept), out)
	fmt.Printf("    review %s, then: pixelsmith adopt -name %s -pick <n>\n", sheet, *name)
	return nil
}

// buildPrompt assembles the few-shot request.
//
// The examples carry their own names, because "this is what a sword looks like
// here" is most of the instruction — the palette can be listed but the house
// habits (a weapon lies on the diagonal, the haft runs to the lower left, the
// dark tone outlines the lit side) are only legible by example.
func buildPrompt(seeds []string, name, desc string, head int) (string, error) {
	var b strings.Builder
	b.WriteString("You draw 16x16 pixel-art icons for a game, as a grid of characters.\n\n")
	b.WriteString("Rules, all of them absolute:\n")
	b.WriteString("- Output exactly 16 lines of exactly 16 characters. Nothing else. No prose, no code fence.\n")
	b.WriteString("- '.' is transparent. The digits are palette slots:\n")
	for i, p := range palette {
		fmt.Fprintf(&b, "    %d = %s (%s)\n", i+1, pixelpal.Hex(p.RGBA), p.Role)
	}
	b.WriteString("- Use 2 to outline the object against the transparency.\n")
	b.WriteString("- The object lies on a diagonal: the handle end at the lower left, the business end at the upper right.\n")
	b.WriteString("- Leave the bottom-right and top-left corners empty, as the examples do.\n\n")
	// Style is transferred as measurements, not as example grids.
	//
	// Showing the grids was tried first and it does not teach style, it invites
	// copying: qwen3 returned sword1 back almost cell for cell, twice, and
	// devstral spent a whole batch redrawing staff3 with the head moved. Both
	// are useless as a spear and the second is worse than useless — a near-copy
	// of a purchased sprite is the purchased sprite, which is exactly what a
	// grid committed to a public repository must not be.
	//
	// So the seeds are measured and described. The numbers below are read off
	// the real icons every run, so they cannot drift from the set they claim to
	// describe.
	st, err := measure(seeds)
	if err != nil {
		return "", err
	}
	b.WriteString("The set you are joining, measured:\n")
	fmt.Fprintf(&b, "- an icon uses between %d and %d coloured cells; the rest is '.'\n", st.minInk, st.maxInk)
	fmt.Fprintf(&b, "- the object runs corner to corner on the diagonal, occupying roughly %d rows and %d columns\n", st.rows, st.cols)
	fmt.Fprintf(&b, "- slot 2 is the outline: %d%% of the cells that touch transparency are slot 2\n", st.outlinePct)
	fmt.Fprintf(&b, "- slot 1 does most of the body: %d%% of all coloured cells\n", st.bodyPct)
	fmt.Fprintf(&b, "- slot 3 highlights, sparingly: %d%%\n", st.hiPct)
	b.WriteString("- the handle is one cell wide; only the working end is wider\n")
	b.WriteString("- nothing is symmetrical and nothing is centred\n\n")
	b.WriteString("Do not copy any existing icon. Draw the object described.\n\n")

	if head > 0 {
		// The decomposed ask.
		//
		// A 16x16 weapon is mostly a straight line, and a straight line is the
		// one part of this a program draws better than a model — perfectly
		// even, on the exact diagonal the set uses, every time. Asking for the
		// whole icon spends the model's very limited spatial budget on the easy
		// nine tenths and leaves nothing for the head, which is the only part
		// that carries the meaning. So: the tool lays the haft, and the model
		// is asked for a small shape to put on the end of it.
		// One worked example, generated here rather than taken from the pack.
		//
		// This is the middle ground the earlier passes missed. Showing real
		// icons makes the model copy them, and showing nothing leaves it with
		// no idea what the notation looks like in practice. A plain geometric
		// taper is nobody's art — it is a triangle — so it can be shown freely,
		// and it carries the two things prose kept failing to convey: that the
		// rows get narrower going up, and that slot 2 wraps the outside.
		b.WriteString("The notation, on a plain tapering shape. This is a geometric\n")
		b.WriteString("primitive and not an icon: vary it, do not reproduce it.\n\n")
		b.WriteString(taperExample(head))
		b.WriteString("\n")
		fmt.Fprintf(&b, "Draw ONLY the head of: %s\n", desc)
		fmt.Fprintf(&b, "Output exactly %d lines of exactly %d characters, not 16.\n", head, head)
		b.WriteString("The haft is drawn separately and will join at your bottom-left corner,\n")
		b.WriteString("so put the widest part low and the point at the top right.\n")
		return b.String(), nil
	}

	fmt.Fprintf(&b, "Now draw: %s\n", desc)
	fmt.Fprintf(&b, "Reply with the 16 lines for %q and nothing else.\n", name)
	return b.String(), nil
}

// taperExample draws a plain narrowing shape in the grid notation: outline in
// slot 2, body in slot 1, one highlight. Constructed, not sampled.
func taperExample(n int) string {
	rows := make([]string, n)
	for y := range rows {
		row := make([]byte, n)
		for i := range row {
			row[i] = '.'
		}
		rows[y] = string(row)
	}
	put := func(x, y int, c byte) {
		if x < 0 || y < 0 || x >= n || y >= n {
			return
		}
		r := []byte(rows[y])
		r[x] = c
		rows[y] = string(r)
	}
	// A taper along the diagonal, not a vertical triangle.
	//
	// This is the orientation the whole set is drawn in, and getting it wrong
	// in the example teaches the wrong thing: an axis-aligned leaf pasted on a
	// diagonal haft reads as a flag on a pole. The blade widens towards the
	// lower left, where the haft arrives, and closes to a single cell at the
	// upper-right tip.
	for i := 0; i < n; i++ {
		x, y := i, n-1-i // the tip end is top-right
		w := i / 2       // wider as it comes back down the haft
		put(x, y, '1')
		put(x+1, y, '2')
		for k := 1; k <= w; k++ {
			put(x-k, y, '1')
			put(x, y+k, '2')
		}
		if i > 1 && i < n-1 {
			put(x-1, y, '3')
		}
	}
	return strings.Join(rows, "\n") + "\n"
}

// hafted takes a small head from the model and puts it on a haft this draws.
//
// The haft is the set's own geometry: a single cell of slot 1 stepping one
// right and one up, from near the bottom-left corner, with slot 2 shadowing
// beneath it — which is what every hafted weapon in the pack does and what a
// model reproduces badly.
func hafted(reply string, n int) (grid, [][]byte, bool) {
	var head [][]byte
	for _, raw := range strings.Split(reply, "\n") {
		t := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), "`"))
		if t == "" {
			continue
		}
		ok := true
		for _, r := range t {
			if r != '.' && (r < '1' || r > '6') {
				ok = false
				break
			}
		}
		if !ok {
			head = nil
			continue
		}
		for len(t) < n {
			t += "."
		}
		head = append(head, []byte(t[:n]))
		if len(head) == n {
			break
		}
	}
	if len(head) < n {
		return grid{}, nil, false
	}

	var g grid
	for i := range g {
		g[i] = strings.Repeat(".", Size)
	}
	set := func(x, y int, c byte) {
		if x < 0 || y < 0 || x >= Size || y >= Size {
			return
		}
		row := []byte(g[y])
		row[x] = c
		g[y] = string(row)
	}
	// The head sits in the top-right corner, one cell in from each edge.
	hx, hy := Size-n-1, 1
	empty := true
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if c := head[y][x]; c != '.' {
				set(hx+x, hy+y, c)
				empty = false
			}
		}
	}

	// The haft runs down-left from wherever the head's ink actually starts.
	//
	// Not from the corner of the head's box: the model was told the join would
	// be at its bottom-left and it does not reliably draw there, so a haft
	// aimed at the corner leaves a stick and a separate lump. Instead find the
	// lowest, then leftmost, cell the model actually inked, and start beneath
	// it. That makes the join hold whatever the model draws, which is the only
	// way this works when it is right about the shape and careless about where
	// on the page it put it.
	jx, jy := -1, -1
	for y := Size - 1; y >= 0 && jy < 0; y-- {
		for x := 0; x < Size; x++ {
			if g[y][x] != '.' {
				jx, jy = x, y
				break
			}
		}
	}
	if jx < 0 {
		return grid{}, nil, false
	}
	// The haft's own shading is copied from the set's geometry, not invented:
	// mace1 and pick1 both lay a slot-1 cell with a slot-2 cell to its RIGHT on
	// the same row, stepping down-left. The first version put the shadow
	// underneath instead, which reads as a thinner, flatter stick than anything
	// else on the shelf — the sort of thing that looks fine alone and wrong in
	// a row of eight.
	for x, y := jx-1, jy+1; x >= 0 && y <= Size-1; x, y = x-1, y+1 {
		set(x, y, '1')
		set(x+1, y, '2')
		empty = false
	}
	return g, head, !empty
}

// stats are the house habits, in numbers, read off the icons themselves.
type stats struct{ minInk, maxInk, rows, cols, outlinePct, bodyPct, hiPct int }

func measure(seeds []string) (stats, error) {
	var st stats
	st.minInk = 1 << 30
	var body, hi, ink, edge, edge2, rows, cols, n int
	for _, s := range seeds {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		g, err := loadPNGGrid(filepath.Join(seedDir, s+".png"))
		if err != nil {
			return st, err
		}
		n++
		var c, minR, maxR, minC, maxC = 0, Size, -1, Size, -1
		for y := 0; y < Size; y++ {
			for x := 0; x < Size; x++ {
				ch := g[y][x]
				if ch == '.' {
					continue
				}
				c++
				if y < minR {
					minR = y
				}
				if y > maxR {
					maxR = y
				}
				if x < minC {
					minC = x
				}
				if x > maxC {
					maxC = x
				}
				if ch == '1' {
					body++
				}
				if ch == '3' {
					hi++
				}
				// A cell touching transparency is on the silhouette's edge.
				if x == 0 || y == 0 || x == Size-1 || y == Size-1 ||
					g[y][x-1] == '.' || g[y][x+1] == '.' || g[y-1][x] == '.' || g[y+1][x] == '.' {
					edge++
					if ch == '2' {
						edge2++
					}
				}
			}
		}
		ink += c
		if c < st.minInk {
			st.minInk = c
		}
		if c > st.maxInk {
			st.maxInk = c
		}
		rows += maxR - minR + 1
		cols += maxC - minC + 1
	}
	if n == 0 || ink == 0 {
		return st, fmt.Errorf("no seeds to measure")
	}
	st.rows, st.cols = rows/n, cols/n
	st.bodyPct = body * 100 / ink
	st.hiPct = hi * 100 / ink
	if edge > 0 {
		st.outlinePct = edge2 * 100 / edge
	}
	return st, nil
}

// --- the model --------------------------------------------------------------

func ask(model, prompt string, temp float64) (string, error) {
	req, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		// Thinking models spend their budget reasoning before they draw, and a
		// budget that runs out mid-grid produces a reply this discards. Off
		// where the model supports it, generous where it does not.
		"think": false,
		"options": map[string]any{
			"temperature": temp,
			"num_predict": 4000,
		},
	})
	c := &http.Client{Timeout: 5 * time.Minute}
	resp, err := c.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(req))
	if err != nil {
		return "", fmt.Errorf("ollama: %w (is it running?)", err)
	}
	defer resp.Body.Close()
	var out struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("ollama: %s", out.Error)
	}
	return out.Response, nil
}

// --- grids ------------------------------------------------------------------

type grid [Size]string

func (g grid) String() string { return strings.Join(g[:], "\n") + "\n" }

// parseGrid pulls the first 16x16 block of grid characters out of a reply.
//
// Models pad, apologise, fence the block, and occasionally number the rows, so
// this scans for a run of sixteen usable lines rather than trusting the whole
// response. A line short of sixteen cells is padded with transparency and a
// long one is cut: a nearly-right icon is worth looking at, and every candidate
// gets looked at before anything is adopted.
func parseGrid(s string) (grid, bool) {
	var lines []string
	for _, raw := range strings.Split(s, "\n") {
		t := strings.TrimSpace(raw)
		t = strings.Trim(t, "`")
		if t == "" {
			continue
		}
		ok := true
		for _, r := range t {
			if r != '.' && (r < '1' || r > '6') {
				ok = false
				break
			}
		}
		if !ok {
			lines = nil // a non-grid line breaks the run
			continue
		}
		lines = append(lines, t)
		if len(lines) == Size {
			break
		}
	}
	if len(lines) < Size {
		return grid{}, false
	}
	var g grid
	empty := true
	for i, l := range lines {
		for len(l) < Size {
			l += "."
		}
		g[i] = l[:Size]
		if strings.Trim(g[i], ".") != "" {
			empty = false
		}
	}
	return g, !empty
}

func loadPNGGrid(path string) (grid, error) {
	f, err := os.Open(path)
	if err != nil {
		return grid{}, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return grid{}, err
	}
	index := map[color.NRGBA]int{}
	for i, p := range palette {
		index[p.RGBA] = i + 1
	}
	var out grid
	bb := img.Bounds()
	for y := 0; y < Size; y++ {
		row := make([]byte, Size)
		for x := 0; x < Size; x++ {
			row[x] = '.'
			if x >= bb.Dx() || y >= bb.Dy() {
				continue
			}
			c := color.NRGBAModel.Convert(img.At(bb.Min.X+x, bb.Min.Y+y)).(color.NRGBA)
			if c.A == 0 {
				continue
			}
			c.A = 255
			if i, ok := index[c]; ok {
				row[x] = byte('0' + i)
			}
		}
		out[y] = string(row)
	}
	return out, nil
}

// Render turns a grid into an image, which is also what assetpipe does.
func (g grid) Render() *image.NRGBA {
	rgb := make([]color.NRGBA, len(palette)+1)
	for i, p := range palette {
		rgb[i+1] = p.RGBA
	}
	img := image.NewNRGBA(image.Rect(0, 0, Size, Size))
	for y := 0; y < Size; y++ {
		for x := 0; x < len(g[y]) && x < Size; x++ {
			c := g[y][x]
			if c < '1' || c > '6' {
				continue
			}
			img.SetNRGBA(x, y, rgb[c-'0'])
		}
	}
	return img
}

// writeSheet lays the candidates out for a look, because the only test that
// matters here is whether it reads as the thing at the size it is drawn.
func writeSheet(path string, gs []grid) error {
	const cell, zoom = 20, 8
	cols := 6
	rows := (len(gs) + cols - 1) / cols
	sheet := image.NewNRGBA(image.Rect(0, 0, cols*cell*zoom, rows*cell*zoom))
	for i := range sheet.Pix {
		sheet.Pix[i] = [...]uint8{0x20, 0x20, 0x24, 0xFF}[i%4]
	}
	for i, g := range gs {
		img := g.Render()
		ox, oy := (i%cols)*cell*zoom, (i/cols)*cell*zoom
		for y := 0; y < Size*zoom; y++ {
			for x := 0; x < Size*zoom; x++ {
				c := img.NRGBAAt(x/zoom, y/zoom)
				if c.A == 0 {
					continue
				}
				sheet.SetNRGBA(ox+x+zoom, oy+y+zoom, c)
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, sheet)
}

// --- adopting ---------------------------------------------------------------

func adopt(args []string) error {
	fs := flag.NewFlagSet("adopt", flag.ExitOnError)
	name := fs.String("name", "", "which draft set")
	pick := fs.Int("pick", 0, "which candidate number")
	fs.Parse(args)
	if *name == "" || *pick <= 0 {
		return fmt.Errorf("-name and -pick are both required")
	}
	src := filepath.Join("shots", "pixelsmith-"+*name, fmt.Sprintf("%02d.txt", *pick))
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(artDir, *name+".txt")
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("ok  %s -> %s\n", src, dst)
	return nil
}

// --- scoring ----------------------------------------------------------------

// score rates how much a candidate looks like a hafted weapon, so the review
// sheet can be ordered best-first.
//
// This is not taste and cannot be. It measures the three things every icon in
// the set has and most model output does not: the ink is one connected piece,
// it runs on the diagonal rather than sprawling, and there is more of it at one
// end than the other. A model asked for a spear will happily return a constellation
// of unconnected specks that no amount of looking makes into a weapon, and the
// point of this is to put those last rather than to pick a winner.
func score(g grid) float64 {
	var on [][2]int
	for y := 0; y < Size; y++ {
		for x := 0; x < len(g[y]) && x < Size; x++ {
			if c := g[y][x]; c >= '1' && c <= '6' {
				on = append(on, [2]int{x, y})
			}
		}
	}
	// Too little is a speck, too much is a blob. The cut weapons run 20-60.
	if len(on) < 14 || len(on) > 80 {
		return 0
	}

	// One piece. A weapon is a single object; anything in two parts is either a
	// collage or noise.
	seen := map[[2]int]bool{on[0]: true}
	queue := [][2]int{on[0]}
	for len(queue) > 0 {
		p := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				n := [2]int{p[0] + dx, p[1] + dy}
				if seen[n] || n[0] < 0 || n[1] < 0 || n[0] >= Size || n[1] >= Size {
					continue
				}
				if c := g[n[1]][n[0]]; c >= '1' && c <= '6' {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
	}
	connected := float64(len(seen)) / float64(len(on))

	// On the diagonal, and running the long way. Measured as how well the ink
	// tracks the line y = -x: the set draws every weapon lower-left to upper-right.
	var sum, sumSq float64
	for _, p := range on {
		d := float64(p[0] + p[1])
		sum += d
		sumSq += d * d
	}
	n := float64(len(on))
	spread := sumSq/n - (sum/n)*(sum/n) // variance along the wrong axis
	tight := 1 / (1 + spread/8)

	// Heavier at one end: a haft plus a head, not a uniform stick.
	var near, far int
	for _, p := range on {
		if p[0]+(Size-1-p[1]) > Size {
			far++
		} else {
			near++
		}
	}
	bias := 0.0
	if near+far > 0 {
		r := float64(far) / float64(near+far)
		bias = 1 - 2*absF(r-0.5) // best when the ink is genuinely split
	}

	return connected*0.5 + tight*0.3 + bias*0.2
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// headScore rates a head block on its own, before it is put on a haft.
//
// The whole-icon score cannot see this: attach any blob to a code-drawn haft
// and it scores well, because the haft is perfect and dominates every measure.
// Twenty-four candidates in a row passed that test and half of them were axes.
// So the head is judged separately, against the four things that make a point a
// point rather than a blade or a knob.
func headScore(h [][]byte) float64 {
	n := len(h)
	var on [][2]int
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if h[y][x] != '.' {
				on = append(on, [2]int{x, y})
			}
		}
	}
	// A blade, not a speck and not a slab.
	if len(on) < 5 || len(on) > 18 {
		return 0
	}

	// One piece.
	seen := map[[2]int]bool{on[0]: true}
	q := [][2]int{on[0]}
	for len(q) > 0 {
		p := q[len(q)-1]
		q = q[:len(q)-1]
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				m := [2]int{p[0] + dx, p[1] + dy}
				if seen[m] || m[0] < 0 || m[1] < 0 || m[0] >= n || m[1] >= n || h[m[1]][m[0]] == '.' {
					continue
				}
				seen[m] = true
				q = append(q, m)
			}
		}
	}
	if len(seen) != len(on) {
		return 0
	}

	// Everything below is measured along the anti-diagonal, because that is the
	// axis this set draws on. Judging a diagonal blade by its bounding box says
	// it is as wide as it is tall, which is true and useless — it was also what
	// made an earlier version of this reject every candidate the moment the
	// prompt started asking for a diagonal point.
	band := map[int]int{}
	lo, hi := 1<<30, -(1 << 30)
	for _, p := range on {
		d := p[0] - p[1] // grows towards the upper-right tip
		band[d]++
		if d < lo {
			lo = d
		}
		if d > hi {
			hi = d
		}
	}
	span := hi - lo + 1
	if span < 3 {
		return 0 // a blob, with no length along the blade
	}

	// A tip: the far end is one or two cells.
	if band[hi] > 2 {
		return 0
	}

	// Widening back towards the haft: more of the blade sits in the half
	// nearest the handle than in the half nearest the point.
	mid := lo + span/2
	var nearHaft, nearTip int
	for d, c := range band {
		if d < mid {
			nearHaft += c
		} else {
			nearTip += c
		}
	}
	if nearHaft <= nearTip {
		return 0
	}

	taper := float64(nearHaft-nearTip) / float64(len(on))
	length := minF(float64(span)/float64(n), 1)
	mass := minF(float64(len(on))/12, 1)
	return 0.4*taper + 0.35*length + 0.25*mass
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
