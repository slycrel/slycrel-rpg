// Package game wires everything together: the scene stack, shared state, and
// the Ebitengine entry points. Scenes are a stack rather than a state enum so
// that transient screens (a shop, a battle, the parchment map) can push
// themselves over whatever was underneath and pop back without that screen
// needing to know who called it.
package game

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Scene is one screen of the game.
type Scene interface {
	Update(g *Game) error
	Draw(g *Game, dst *ebiten.Image)
}

// Game is the shared state every scene reads and the Ebitengine driver.
type Game struct {
	Root   string
	Assets *assetsys.Registry
	Data   *gamedata.Tables
	Write  *content.Writer
	RNG    *core.RNG
	Log    *ui.Log

	// Run state, populated once a character exists.
	Player *model.Character
	World  *world.Map
	Seed   int64

	// Where the player is on the overworld, and inside a location.
	Walk      walker
	Local     *world.LocalMap
	LocalWalk walker

	// Steps since the last encounter, so fights cannot chain immediately.
	sinceFight int

	stack []Scene
	tick  int
	quit  bool

	// Scripted capture mode; nil in normal play.
	demo        *demoScript
	pendingShot string
}

// New builds a game with content loaded but no character yet.
func New(root string, seed int64) (*Game, error) {
	tables, err := gamedata.Load(root)
	if err != nil {
		return nil, err
	}
	g := &Game{
		Root:   root,
		Assets: assetsys.New(root),
		Data:   tables,
		Write:  content.New(&tables.Text),
		RNG:    core.NewRNG(seed),
		Seed:   seed,
		Log:    ui.NewLog(200),
	}
	g.Push(newTitleScene(g))
	return g, nil
}

// Push puts a scene on top of the stack.
func (g *Game) Push(s Scene) { g.stack = append(g.stack, s) }

// Pop removes the top scene. Popping the last scene quits.
func (g *Game) Pop() {
	if len(g.stack) > 0 {
		g.stack = g.stack[:len(g.stack)-1]
	}
	if len(g.stack) == 0 {
		g.quit = true
	}
}

// Replace swaps the top scene for another. It substitutes in place rather than
// popping and pushing: Pop treats an emptied stack as "the player quit", and a
// one-scene stack (the title screen) is exactly when Replace is used most.
func (g *Game) Replace(s Scene) {
	if len(g.stack) == 0 {
		g.Push(s)
		return
	}
	g.stack[len(g.stack)-1] = s
}

// Top returns the active scene, or nil.
func (g *Game) Top() Scene {
	if len(g.stack) == 0 {
		return nil
	}
	return g.stack[len(g.stack)-1]
}

// Tick is the frame counter, used for sprite animation and blinking cursors.
func (g *Game) Tick() int { return g.tick }

// Quit asks the game to exit after this frame.
func (g *Game) Quit() { g.quit = true }

// KeyLog turns on a stderr trace of every key the engine reports. It exists
// because "the game is not responding to input" has two very different causes
// — a binding that is wrong, and events that never arrive — and only a trace
// tells them apart.
var KeyLog bool

// Update advances the top scene.
func (g *Game) Update() error {
	g.tick++
	if KeyLog {
		for _, k := range inpututil.AppendJustPressedKeys(nil) {
			fmt.Fprintf(os.Stderr, "key: %v\n", k)
		}
	}
	// Screenshot request. inpututil is only meaningful inside Update, so the
	// key is latched here and the frame is written in Draw, where a finished
	// framebuffer actually exists.
	if inpututil.IsKeyJustPressed(ebiten.KeyF12) || inpututil.IsKeyJustPressed(ebiten.KeyBackslash) {
		g.pendingShot = fmt.Sprintf("slycrel-%d-%06d", g.Seed, g.tick)
	}
	if g.quit {
		return ebiten.Termination
	}
	if g.demo != nil {
		g.updateDemo()
	}
	if s := g.Top(); s != nil {
		return s.Update(g)
	}
	return ebiten.Termination
}

// Draw renders the top scene. Scenes that want the world visible behind them
// draw it themselves; the stack does not composite automatically, because a
// battle wants a dimmed backdrop rather than a live one.
func (g *Game) Draw(dst *ebiten.Image) {
	if s := g.Top(); s != nil {
		s.Draw(g, dst)
	}
	if g.pendingShot != "" {
		g.saveFrame(dst, g.pendingShot)
		g.pendingShot = ""
	}
}

// saveFrame dumps the logical framebuffer to shots/ at native resolution.
// Grabbing the frame from inside Draw is the only place the pixels are
// guaranteed to be complete, and it keeps the capture free of window chrome
// and scaling.
func (g *Game) saveFrame(dst *ebiten.Image, stem string) {
	b := dst.Bounds()
	img := image.NewRGBA(b)
	dst.ReadPixels(img.Pix)

	dir := filepath.Join(g.Root, "shots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	name := filepath.Join(dir, stem+".png")
	f, err := os.Create(name)
	if err != nil {
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return
	}
	g.Log.Add("Screenshot saved to shots/%s", filepath.Base(name))
	fmt.Fprintf(os.Stderr, "wrote %s\n", name)
}

// drawStatusBar paints the bottom strip shared by the overworld and interiors.
// Three fixed rows inside hudH pixels: identity, meters, and one line of log.
// Keeping it in one place is what stops the two scenes drifting apart.
func (g *Game) drawStatusBar(dst *ebiten.Image, place, hint string) {
	y := float64(render.ScreenH - hudH)
	ui.Panel(dst, 0, y, render.ScreenW, hudH)
	p := g.Player

	render.Text(dst, fmt.Sprintf("%s  L%d", p.Name, p.Level), 8, y+5, render.ColInk)
	render.TextRight(dst, render.Trunc(place, 268), render.ScreenW-8, y+5, render.ColGold)

	ui.Bar(dst, 8, y+18, 88, 6, p.HPFrac(), render.ColBlood)
	render.Text(dst, fmt.Sprintf("%d/%d", p.HP, p.MaxHP), 100, y+17, render.ColInkDim)
	ui.Bar(dst, 152, y+18, 56, 6, p.PsycheFrac(), render.ColMagic)
	render.Text(dst, fmt.Sprintf("%d SP", p.Psyche), 212, y+17, render.ColInkDim)
	render.Text(dst, fmt.Sprintf("%d coins", p.Coins), 262, y+17, render.ColGold)
	render.TextRight(dst, hint, render.ScreenW-8, y+17, render.ColInkFaint)

	g.Log.Draw(dst, 8, y+30, 1)
}

// Layout fixes the logical resolution; the window scales by whole multiples.
func (g *Game) Layout(int, int) (int, int) { return render.ScreenW, render.ScreenH }

// walker is a grid-stepping actor with smooth interpolation between tiles.
// Movement is tile-locked (it is that kind of game) but drawn continuously, so
// it reads as walking rather than teleporting.
type walker struct {
	Tile core.Point
	prev core.Point
	t    float64 // progress from prev to Tile, in [0,1]
	dir  core.Dir
	// dur is how many ticks a step takes.
	dur float64
}

// Place teleports the walker with no animation.
func (w *walker) Place(p core.Point) {
	w.Tile, w.prev, w.t = p, p, 1
}

// Moving reports whether a step is in progress.
func (w *walker) Moving() bool { return w.t < 1 }

// Step begins a move to p, facing d.
func (w *walker) Step(p core.Point, d core.Dir) {
	w.prev = w.Tile
	w.Tile = p
	w.dir = d
	w.t = 0
}

// Advance progresses the interpolation one tick.
func (w *walker) Advance() {
	if w.dur <= 0 {
		w.dur = 8
	}
	if w.t < 1 {
		w.t += 1 / w.dur
		if w.t > 1 {
			w.t = 1
		}
	}
}

// Pixel returns the walker's interpolated position in world pixels, centred on
// its tile horizontally and at the tile's bottom edge vertically, which is the
// anchor render.Ctx.World expects.
func (w *walker) Pixel() (float64, float64) {
	const ts = assetsys.TileSize
	fx := float64(w.prev.X) + (float64(w.Tile.X)-float64(w.prev.X))*w.t
	fy := float64(w.prev.Y) + (float64(w.Tile.Y)-float64(w.prev.Y))*w.t
	return fx*ts + ts/2, fy*ts + ts
}

// Dir returns the current facing.
func (w *walker) Dir() core.Dir { return w.dir }

// Input helpers. Everything routes through these so rebinding later is a
// single-file change.

var (
	upKeys      = []ebiten.Key{ebiten.KeyUp, ebiten.KeyW, ebiten.KeyK}
	downKeys    = []ebiten.Key{ebiten.KeyDown, ebiten.KeyS, ebiten.KeyJ}
	leftKeys    = []ebiten.Key{ebiten.KeyLeft, ebiten.KeyA, ebiten.KeyH}
	rightKeys   = []ebiten.Key{ebiten.KeyRight, ebiten.KeyD, ebiten.KeyL}
	confirmKeys = []ebiten.Key{ebiten.KeyEnter, ebiten.KeySpace, ebiten.KeyZ, ebiten.KeyNumpadEnter}
	cancelKeys  = []ebiten.Key{ebiten.KeyEscape, ebiten.KeyX, ebiten.KeyBackspace}
)

func anyPressed(keys []ebiten.Key) bool {
	for _, k := range keys {
		if ebiten.IsKeyPressed(k) {
			return true
		}
	}
	return false
}

func anyJustPressed(keys []ebiten.Key) bool {
	for _, k := range keys {
		if inpututil.IsKeyJustPressed(k) {
			return true
		}
	}
	return false
}

// Confirm reports an accept press this frame.
func Confirm() bool { return anyJustPressed(confirmKeys) }

// Cancel reports a back press this frame.
func Cancel() bool { return anyJustPressed(cancelKeys) }

// HeldDir returns the direction currently held, for continuous walking.
func HeldDir() (core.Dir, bool) {
	switch {
	case anyPressed(upKeys):
		return core.DirUp, true
	case anyPressed(downKeys):
		return core.DirDown, true
	case anyPressed(leftKeys):
		return core.DirLeft, true
	case anyPressed(rightKeys):
		return core.DirRight, true
	}
	return core.DirDown, false
}

// MenuDir returns a single directional press, for cursor movement.
func MenuDir() (core.Dir, bool) {
	switch {
	case anyJustPressed(upKeys):
		return core.DirUp, true
	case anyJustPressed(downKeys):
		return core.DirDown, true
	case anyJustPressed(leftKeys):
		return core.DirLeft, true
	case anyJustPressed(rightKeys):
		return core.DirRight, true
	}
	return core.DirDown, false
}

// heroSpriteKey maps a class and facing to a manifest key, falling back to the
// idle sheet when a directional one is missing.
func heroSpriteKey(c *model.Character, d core.Dir, moving bool) string {
	class := "fighter"
	switch c.Class {
	case model.ClassThief:
		class = "thief"
	case model.ClassMage:
		class = "mage"
	}
	if !moving {
		return "hero/" + class + "/idle"
	}
	return "hero/" + class + "/" + [...]string{"down", "left", "right", "up"}[d]
}
