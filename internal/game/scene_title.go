package game

import (
	"fmt"
	"image/color"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// titleScene is the front door: a name, a subtitle, and three ways in.
type titleScene struct {
	menu ui.Menu
	// cam pans slowly across the continent behind everything else.
	cam render.Camera
	// drift is where the pan has got to, in pixels along the coast.
	drift float64
	// newest is when the most recent save was written, for the Continue row.
	//
	// Held as a time rather than as the rendered string, because this scene is
	// built once at launch and then sits there for as long as nobody presses
	// anything. Baking "just now" into the row at construction meant it still
	// said "just now" twenty minutes later, which is the one screen where that
	// is most likely to be read and most likely to be wrong.
	newest time.Time
}

func newTitleScene(g *Game) *titleScene {
	t := &titleScene{}
	saves := save.List(g.Root)
	cont := ui.MenuItem{Label: "Continue", Detail: "no saves", Disabled: true}
	if len(saves) > 0 {
		t.newest = saves[0].Saved
		cont = ui.MenuItem{Label: "Continue", Detail: humanAge(t.newest)}
	}
	t.menu.SetItems([]ui.MenuItem{
		{Label: "Begin", Detail: "a new mistake"},
		cont,
		{Label: "Quit", Detail: "coward"},
	})
	// The world behind the menu is this seed's actual continent.
	//
	// The art pass that was meant to land here went looking through 4,488
	// bundled GUI PNGs for something to dress this screen with, and the honest
	// answer was that none of it fits: painted mobile interfaces at two to four
	// times this game's scale, against a seven-by-thirteen font. What the game
	// does have, and what it has more of than anything else, is terrain — and
	// the one screen with no art on it at all was the first one anybody sees.
	//
	// So the title shows the place you are about to play. It costs one world
	// generation, which the tour already does several times a second, and it
	// means "seed 1041359034192" at the bottom of the screen is legible as a
	// promise rather than a number.
	if g.World == nil && g.Write != nil {
		g.World = world.Generate(g.Seed, g.Write)
	}
	if g.World != nil {
		t.cam.CenterOn(float64(g.World.Start.X*assetsys.TileSize),
			float64(g.World.Start.Y*assetsys.TileSize))
	}

	return t
}

func (t *titleScene) Update(g *Game) error {
	g.Sound.Ambience("")
	// Re-age the Continue row against the clock rather than against whenever
	// this screen happened to be built. See the field.
	//
	// Found by label, not by row number. This menu has three rows and has had
	// three rows forever, which is exactly the argument that was being made
	// about the pause menu right up until somebody inserted a fourth.
	if !t.newest.IsZero() {
		for i := range t.menu.Items {
			if t.menu.Items[i].Label == "Continue" {
				t.menu.Items[i].Detail = humanAge(t.newest)
				break
			}
		}
	}
	// A slow drift east, so the screen is never still and never arrives
	// anywhere.
	t.drift += 0.25
	g.MenuNav(&t.menu)
	if g.Accept() {
		switch t.menu.Index {
		case 0:
			g.Replace(newCreateScene(g))
		case 1:
			g.Push(newSlotScene(g, slotLoad))
		case 2:
			g.Quit()
		}
	}
	return nil
}

func (t *titleScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x14, 0x10, 0x1C, 0xFF})
	t.drawWorld(g, dst)

	render.TextCenter(dst, "S L Y C R E L", render.ScreenW/2, 54, render.ColGold)
	render.TextCenter(dst, "an open world of poor decisions", render.ScreenW/2, 72, render.ColInkDim)
	render.TextCenter(dst, "comic violence, and adults behaving exactly as expected",
		render.ScreenW/2, 88, render.ColInkFaint)

	ui.Panel(dst, render.ScreenW/2-104, 116, 208, 52)
	t.menu.Draw(dst, render.ScreenW/2-84, 124, 176)

	render.TextCenter(dst, "arrows / WASD to move - Z or Enter to confirm - X to go back",
		render.ScreenW/2, 232, render.ColInkFaint)
	render.TextCenter(dst, fmt.Sprintf("seed %d", g.Seed), render.ScreenW/2, 248, render.ColInkFaint)
	render.TextCenter(dst, BuildStamp, render.ScreenW/2, 258, render.ColInkFaint)
}

// drawWorld paints the continent behind the menu, well under.
//
// Pushed most of the way to black on purpose. This is a backdrop and the thing
// in front of it is a list of three words a player has to read — terrain at
// full strength would be a screenshot with a menu lost in it.
func (t *titleScene) drawWorld(g *Game, dst *ebiten.Image) {
	if g.World == nil {
		return
	}
	const ts = assetsys.TileSize
	t.cam.CenterOn(
		float64(g.World.Start.X*ts)+t.drift,
		float64(g.World.Start.Y*ts))

	x0 := int(t.cam.X)/ts - 1
	y0 := int(t.cam.Y)/ts - 1
	x1 := x0 + render.ScreenW/ts + 3
	y1 := y0 + render.ScreenH/ts + 3

	ground := g.ground()
	ox, oy := t.cam.Offset()
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			ground.Draw(dst, float64(tx*ts)+ox, float64(ty*ts)+oy, tx, ty, g.materialAt)
		}
	}
	g.drawDecor(dst, t.cam, x0, y0, x1, y1)

	// And then most of the way out again. A multiply rather than a black wash,
	// for the same reason the night tint is one: a wash lifts the darks and the
	// terrain turns to grey fog, where multiplying leaves it legible as terrain
	// and simply far away.
	render.Multiply(dst, color.RGBA{0x74, 0x66, 0x88, 0xFF})

	// Then a vignette top and bottom, because the words go there.
	//
	// The first pass dimmed the whole frame evenly and the footer — three lines
	// of faint grey about seeds and build stamps — came out illegible against
	// grass. Darkening only where the text is keeps the middle of the screen as
	// scenery and the edges as paper, which is what the layout was already
	// doing with a horizon line before there was anything behind it.
	const ink = 0x0C
	render.VFade(dst, 0, 0, render.ScreenW, 108, color.RGBA{ink, 0x08, 0x14, 0}, 0xE8, 0x30)
	render.VFade(dst, 0, render.ScreenH-84, render.ScreenW, 84, color.RGBA{ink, 0x08, 0x14, 0}, 0x30, 0xE8)
}

// createScene builds a character in two passes: who they are, then what they
// do.
//
// One screen used to carry all of it, and it was at its limit before the
// portrait and the walk sheet were choices at all — up and down picked a class,
// left rerolled the numbers, right rerolled the name, and every one of those
// had to be explained in a footer that read like a keyboard shortcut list.
// Splitting it costs one keypress and buys a screen that can afford to show a
// face at a size you can see.
//
// The three rolled characters are the ones the game gets. They used to be
// previews in the ordinary sense — rolled from their own forked generator to
// keep the panel steady while browsing — and then startRun rolled a *fresh*
// character from the main stream and handed that to the player instead. So the
// prospects panel had never once shown the hit points anybody actually started
// with, and a stat reroll would have been rerolling numbers that never left
// this screen.
type createScene struct {
	// step 0 is the person, step 1 is the trade.
	step int

	// Step 0: name, face and walk sheet, and the rows that change them.
	who     ui.Menu
	name    string
	epithet string
	faces   []string
	faceIdx int
	lookIdx int

	// Step 1: the class list and the three throws it is choosing between.
	menu   ui.Menu
	rolled map[model.Class]*model.Character
	// shown is the class the preview panel is describing. It sticks when the
	// cursor moves onto a row that is not a class, so nothing blanks out.
	shown model.Class

	// seedText is the world seed as the player is editing it.
	//
	// A string rather than the int64 it parses to, because a half-typed number
	// is a real state the screen has to be able to show: deleting the last
	// digit of "1994" has to leave "199" on screen and not silently snap the
	// continent to 199. Empty means the player has cleared the field, which
	// shows as a dash and commits nothing.
	seedText string

	nameRNG *core.RNG
	statRNG *core.RNG
}

// Rows on the first screen. Indexed by these rather than by number, because
// all of them but the last do something on left/right, one of them also takes
// typing, and which is which has already moved once.
const (
	// The world comes first because it is the only choice here that is not
	// about the person. Changing it rerolls everything below it — see
	// setSeed — so a list that read Name, Face, Look, World would spend three
	// rows on decisions the fourth one throws away.
	rowWorld = iota
	rowName
	rowFace
	rowLook
	rowOnward
)

// seedDigits caps the field. Seeds are taken modulo 1<<40, so thirteen digits
// covers every value the game can actually hold and a fourteenth would be a
// number the run quietly would not be.
const seedDigits = 13

func newCreateScene(g *Game) *createScene {
	c := &createScene{
		rolled:  map[model.Class]*model.Character{},
		nameRNG: g.RNG.Fork("names", g.Seed),
		statRNG: g.RNG.Fork("stats", g.Seed),
		faces:   g.heroFaces(),
	}
	c.rerollStats()
	c.rerollName(g)
	// Start somewhere other than the top of both lists, so a player who presses
	// straight through gets somebody rather than the first entry twice.
	//
	// Forked off the seed, so it varies run to run and not within one: the same
	// seed is the same continent and the same opening face, which is what makes
	// -seed 1994 worth having. Fork never reads its receiver, so the seed has to
	// be in the salt or every run of every seed would open on the same person.
	c.faceIdx = g.RNG.Fork("face", g.Seed).Intn(len(c.faces))
	c.lookIdx = g.RNG.Fork("look", g.Seed).Intn(len(heroLooks))

	c.seedText = strconv.FormatInt(g.Seed, 10)
	c.who.SetItems([]ui.MenuItem{
		{Label: "World"},
		{Label: "Name"},
		{Label: "Face"},
		{Label: "Look"},
		{Label: "That is me"},
	})
	c.refreshWho()

	items := make([]ui.MenuItem, 0, len(model.AllClasses)+1)
	for _, cl := range model.AllClasses {
		items = append(items, ui.MenuItem{Label: string(cl), Data: cl})
	}
	items = append(items, ui.MenuItem{Label: "Back"})
	c.menu.SetItems(items)
	c.shown = model.AllClasses[0]
	return c
}

// refreshWho writes the current answers into the detail column, which is what
// makes left and right legible: the row says what it is holding, so changing it
// is visibly changing that and not something else on the screen.
func (c *createScene) refreshWho() {
	// Nothing. The name is the headline across the top of the screen at full
	// width, which is where a generated "Sister Agatha Blunt the Well-Meaning
	// Disaster" can actually be read; the same string in a detail column
	// truncates to "Sister Agatha." and says less than the blank does.
	//
	// It is also the one row here that is not a position in a list. Face and
	// Look say which of how many because that is a thing worth knowing; a name
	// has no index to report, only a new one.
	c.who.Items[rowFace].Detail = fmt.Sprintf("%d of %d", c.faceIdx+1, len(c.faces))
	c.who.Items[rowLook].Detail = heroLooks[c.lookIdx].Name
	// The seed reads as itself. It was already printed at the bottom of the
	// title screen and on the pause menu, where it was a number the game showed
	// you and would not take back — the one piece of state the whole continent
	// is a function of, and the only one that could not be set.
	seed := c.seedText
	if seed == "" {
		seed = "-"
	}
	c.who.Items[rowWorld].Detail = seed
}

// face is the portrait key currently chosen.
func (c *createScene) face() string {
	if len(c.faces) == 0 {
		return ""
	}
	return c.faces[((c.faceIdx%len(c.faces))+len(c.faces))%len(c.faces)]
}

// cycle steps an index around a list of n, in either direction.
func cycle(i, n, delta int) int {
	if n <= 0 {
		return 0
	}
	return ((i+delta)%n + n) % n
}

// rerollStats rolls a fresh set of all three, so whichever class the cursor
// lands on is showing numbers from the same throw and the comparison between
// them stays a fair one.
func (c *createScene) rerollStats() {
	for _, cl := range model.AllClasses {
		c.rolled[cl] = rules.NewCharacter(c.statRNG, "", cl)
	}
}

// setSeed adopts a new continent, and rerolls the person to match.
//
// Rerolling the person is the part worth arguing about, and it is deliberate.
// Everything on this screen is forked off the seed — the opening name, the
// opening face, the throw behind each class — so a seed applied to the world
// but not to the person would make "seed 1994" mean two different runs
// depending on whether it was typed before or after the arrow keys. The title
// screen already promises otherwise, and `-seed 1994` on the command line has
// always rerolled all of it, because the seed is set before any of this exists.
// The World row sits at the top of the list so this reads as the order it is.
//
// The world itself is dropped rather than regenerated. Nothing on this screen
// draws it, the title screen behind rebuilds any world it finds missing, and
// startRun generates one unconditionally — so regenerating a continent on every
// keystroke would be thirteen continents nobody looks at.
func (c *createScene) setSeed(g *Game, seed int64) {
	g.Seed = seed
	g.RNG = core.NewRNG(seed)
	g.World = nil

	c.nameRNG = g.RNG.Fork("names", g.Seed)
	c.statRNG = g.RNG.Fork("stats", g.Seed)
	c.rerollStats()
	c.rerollName(g)
	c.faceIdx = g.RNG.Fork("face", g.Seed).Intn(len(c.faces))
	c.lookIdx = g.RNG.Fork("look", g.Seed).Intn(len(heroLooks))
	c.refreshWho()
}

// commitSeed parses the edited field and adopts it, if it says anything.
//
// A cleared or unparseable field is left alone rather than treated as zero.
// Zero is the sentinel the -seed flag uses for "pick one from the clock", so
// committing it here would hand the player a continent they did not ask for at
// the exact moment they were most specifically asking.
func (c *createScene) commitSeed(g *Game) {
	n, err := strconv.ParseInt(c.seedText, 10, 64)
	if err != nil || n <= 0 {
		c.refreshWho()
		return
	}
	if n != g.Seed {
		c.setSeed(g, n)
		return
	}
	c.refreshWho()
}

// typeSeed handles digits and backspace while the cursor is on the World row.
//
// Typed rather than cycled with the arrows, because a seed is thirteen digits
// and nobody is going to press right four hundred billion times. This is the
// only text entry in the game and it is deliberately the narrowest possible
// one: ten keys and a rubout, on one row, with the arrows still doing what they
// do everywhere else on this screen.
func (c *createScene) typeSeed(g *Game) bool {
	changed := false
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= '0' && r <= '9' && len(c.seedText) < seedDigits {
			c.seedText += string(r)
			changed = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && c.seedText != "" {
		c.seedText = c.seedText[:len(c.seedText)-1]
		changed = true
	}
	if changed {
		g.Sound.Play("ui/move")
		c.commitSeed(g)
	}
	return changed
}

func (c *createScene) rerollName(g *Game) {
	c.name = g.Write.HeroName(c.nameRNG)
	c.epithet = g.Write.Epithet(c.nameRNG)
}

func (c *createScene) Update(g *Game) error {
	if c.step == 0 {
		return c.updateWho(g)
	}
	return c.updateWhat(g)
}

// updateWho is the first screen: up and down pick a field, left and right
// change the one you are on. Every row that can change says what it currently
// holds, so there is nothing to explain about which arrow does what.
func (c *createScene) updateWho(g *Game) error {
	// Typing comes before anything else reads the keyboard, and only on the row
	// that wants it. Otherwise a digit would fall through to whatever else is
	// listening, and — more to the point — backspace has to reach the field
	// rather than being swallowed as a cancel.
	if c.who.Index == rowWorld && c.typeSeed(g) {
		return nil
	}
	if g.Back() {
		g.Replace(newTitleScene(g))
		return nil
	}
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirUp:
			c.who.Move(-1)
		case core.DirDown:
			c.who.Move(1)
		case core.DirLeft, core.DirRight:
			step := 1
			if d == core.DirLeft {
				step = -1
			}
			switch c.who.Index {
			case rowWorld:
				// A whole new continent, from the clock rather than from the
				// game's own RNG. Drawing it from g.RNG would make the "random"
				// seed a pure function of the seed it is replacing, so the same
				// starting seed would offer the same next one every time.
				c.seedText = strconv.FormatInt(time.Now().UnixNano()%(1<<40), 10)
				c.commitSeed(g)
			case rowName:
				c.rerollName(g)
			case rowFace:
				c.faceIdx = cycle(c.faceIdx, len(c.faces), step)
			case rowLook:
				c.lookIdx = cycle(c.lookIdx, len(heroLooks), step)
			default:
				// The last row holds nothing to cycle. Say so rather than
				// silently doing nothing, which reads as the key not working.
				g.Sound.Play("ui/deny")
				return nil
			}
			c.refreshWho()
			g.Sound.Play("ui/move")
		}
		return nil
	}
	if g.Accept() {
		c.step = 1
	}
	return nil
}

// updateWhat is the second screen: the class, and the throw it comes with.
func (c *createScene) updateWhat(g *Game) error {
	if g.Back() {
		c.step = 0
		return nil
	}
	// Left and right both roll again. They were two different rerolls when the
	// name lived on this screen; now there is only one thing here to throw, and
	// giving it both arrows means neither of them is a dead key.
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirUp:
			c.menu.Move(-1)
		case core.DirDown:
			c.menu.Move(1)
		case core.DirLeft, core.DirRight:
			c.rerollStats()
			g.Sound.Play("ui/move")
		}
		return nil
	}

	if !g.Accept() {
		return nil
	}
	it, _ := c.menu.Selected()
	if it.Label == "Back" {
		c.step = 0
		return nil
	}
	if class, ok := it.Data.(model.Class); ok {
		g.startRun(c.dress(c.rolled[class]), c.name, c.epithet)
	}
	return nil
}

// dress puts the chosen look and face on the character being handed over.
//
// heroSpriteKey and portraitOf both fall back to a class default when these are
// empty, which is how every hero looked before there was a choice — so writing
// them is the whole of the choice taking effect, and anything downstream that
// overwrote them would silently undo the only screen in the game where the
// player picks their own art.
func (c *createScene) dress(p *model.Character) *model.Character {
	if p == nil {
		return nil
	}
	p.Sprite = heroLooks[c.lookIdx].Key
	p.Portrait = c.face()
	return p
}

func (c *createScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x14, 0x10, 0x1C, 0xFF})
	if c.step == 0 {
		c.drawWho(g, dst)
		return
	}
	c.drawWhat(g, dst)
}

// drawWho puts the face at a size worth choosing between, the walk sheet beside
// it doing what it will do in the world, and the three rows that change them.
func (c *createScene) drawWho(g *Game, dst *ebiten.Image) {
	render.TextCenter(dst, "WHO ARE YOU, THEN", render.ScreenW/2, 12, render.ColGold)
	render.TextCenter(dst, c.name+" "+c.epithet, render.ScreenW/2, 28, render.ColInk)

	// The portrait, big, with a strip of neighbours under it. The strip is not
	// decoration: cycling one at a time through sixty-eight faces with nothing
	// but a counter to say where you are is the kind of control that gets used
	// twice and then pressed until it stops.
	ui.TitledPanel(dst, "face", 12, 42, 136, 160)
	ui.Slot(dst, 27, 51, 106, 106, nil)
	if sp := g.Assets.Get(c.face()); sp != nil {
		render.ScreenFit(dst, sp, 0, 28, 52, 104, 104, nil)
	}
	const (
		thumb = 20
		gap   = 3
	)
	stripX := 12 + (136-(5*thumb+4*gap))/2.0
	for i := -2; i <= 2; i++ {
		key := c.faces[cycle(c.faceIdx, len(c.faces), i)]
		x := stripX + float64(i+2)*(thumb+gap)
		edge := color.Color(nil)
		if i == 0 {
			edge = render.ColGold
		}
		ui.Slot(dst, x-2, 164, thumb+4, thumb+4, edge)
		if sp := g.Assets.Get(key); sp != nil {
			render.ScreenFit(dst, sp, 0, x, 166, thumb, thumb, nil)
		}
	}

	// The walk sheet, idling, at twice the size it is in the world. Animated,
	// because a still frame of a sprite whose whole job is to move tells you
	// almost nothing about it — and doubled, because ScreenFit will not
	// magnify, so fitting a 64-pixel sheet into any box just centres it small.
	ui.TitledPanel(dst, "in the world", 156, 42, 136, 160)
	look := heroLooks[c.lookIdx]
	if sp := g.Assets.Get(look.Key + "/idle"); sp != nil {
		render.Screen(dst, sp, g.Tick()/14, 160, 52, 2)
	}
	render.TextCenter(dst, look.Name, 224, 186, render.ColInkDim)

	ui.TitledPanel(dst, "and so", 300, 42, 168, 160)
	c.who.Draw(dst, 312, 54, 144)

	// The World row is the only one with a control the others do not have, so
	// it is the only one that gets a line of its own. A footer that listed
	// every key on every row would be the keyboard-shortcut list this screen
	// was split in two to get rid of.
	hint := "up/down pick a thing  -  left/right change it"
	if c.who.Index == rowWorld {
		hint = "type a seed  -  left/right for a new world"
	}
	render.TextCenter(dst, hint, render.ScreenW/2, 234, render.ColInkFaint)
	render.TextCenter(dst, "Z to go on  -  X to go back", render.ScreenW/2, 246, render.ColInkFaint)
}

// drawWhat is the class list and the throw behind it.
func (c *createScene) drawWhat(g *Game, dst *ebiten.Image) {
	render.TextCenter(dst, "AND WHAT DO YOU DO", render.ScreenW/2, 12, render.ColGold)

	// The person from the last screen stays on this one, small, so the choice
	// being made is visibly being made about somebody.
	ui.Slot(dst, 11, 19, 30, 30, nil)
	if sp := g.Assets.Get(c.face()); sp != nil {
		render.ScreenFit(dst, sp, 0, 12, 20, 28, 28, nil)
	}
	render.Text(dst, c.name+" "+c.epithet, 46, 28, render.ColInkDim)
	render.TextRight(dst, "< new numbers >", render.ScreenW-12, 28, render.ColInkFaint)

	ui.TitledPanel(dst, "class", 12, 52, 236, 126)
	c.menu.Draw(dst, 24, 62, 216)

	// Stat preview for whichever class the cursor was last on.
	if it, _ := c.menu.Selected(); it.Data != nil {
		if cl, ok := it.Data.(model.Class); ok {
			c.shown = cl
		}
	}
	if p := c.rolled[c.shown]; p != nil {
		cl := c.shown
		ui.TitledPanel(dst, "prospects", 258, 52, 210, 126)
		x, y := 268, 62.0
		for _, ln := range render.Wrap(cl.Blurb(), 186) {
			render.Text(dst, ln, float64(x), y, render.ColInkDim)
			y += render.LineH
		}
		y += 4
		// Every number is coloured against the band its own class rolls it in,
		// which is the only comparison that means anything: eight Strength is a
		// poor Fighter and a good Mage, and a player deciding between the three
		// cannot be expected to know either band. Green is a lucky roll for
		// this class, red an unlucky one, and most rolls are neither.
		b := rules.StartingBands(cl)
		for _, row := range []struct {
			label string
			value string
			frac  float64
		}{
			{"Hit points", fmt.Sprint(p.MaxHP), b.HP.Frac(p.MaxHP)},
			{"Psyche", fmt.Sprint(p.MaxPsyche), b.Psy.Frac(p.MaxPsyche)},
			{"Strength", fmt.Sprint(p.Strength), b.Str.Frac(p.Strength)},
			{"Dexterity", fmt.Sprint(p.Dexterity), b.Dex.Frac(p.Dexterity)},
			{"Speed", fmt.Sprint(p.Speed), b.Spd.Frac(p.Speed)},
			// The purse is rolled the same for everybody, so it grades against
			// its own band rather than the class's.
			{"Purse", fmt.Sprintf("%d coins", p.Coins), rules.StartingCoins.Frac(int(p.Coins))},
		} {
			render.Text(dst, row.label, float64(x), y, render.ColInkDim)
			render.TextRight(dst, row.value, 458, y, gradeFrac(row.frac))
			y += render.LineH
		}
	}

	render.TextCenter(dst, "up/down pick a trade  -  left/right roll again",
		render.ScreenW/2, 234, render.ColInkFaint)
	render.TextCenter(dst, "Z to begin  -  X to go back", render.ScreenW/2, 246, render.ColInkFaint)
}

// startRun generates the continent and drops the player into it.
//
// It takes the character rather than rolling one. Rolling here is what made the
// creation screen's stat panel a work of fiction: it showed numbers from one
// throw and the game began with another.
func (g *Game) startRun(p *model.Character, name, epithet string) {
	g.Player = p
	g.Player.Name = name
	g.Player.Epithet = epithet
	g.Player.Weapon, g.Player.Armor = g.Data.StarterKit(g.Player.Class)
	if it, ok := g.Data.Item("Small Beer"); ok {
		it.Count = 2
		g.Player.AddItem(it)
	}

	// A new run starts alone. Without this, dying and starting again would hand
	// the next character the previous one's hirelings, who would arrive already
	// paid for and levelled to somebody else's career.
	g.Allies = nil
	g.follow, g.localFollow = nil, nil
	g.Threads, g.pendingBeats, g.remindEndings = thread.Log{}, nil, false
	g.Sagas, g.pendingLegs = saga.Log{}, nil

	g.World = world.Generate(g.Seed, g.Write)
	g.Quests = quest.Log{}
	g.Walk = core.NewWalker(9)
	g.Walk.Place(g.World.Start)

	g.Sound.Play("vo/welcome")
	g.Log.Clear()
	g.Log.AddColor(render.ColGold, "%s %s steps out into a world that did not ask for them.",
		g.Player.Name, g.Player.Epithet)
	g.Replace(newOverworldScene(g))
	// The reason to be here, before anything else has had a chance to happen.
	// Pushed under the welcome box rather than over it, so the player reads the
	// controls first and the story second.
	g.ensureSaga()
	// The opening is a checkpoint too, or a character who dies before finding
	// their first bed has nowhere to go back to and the whole safety net only
	// starts working once you can afford one.
	g.autosave()
	if poi := g.World.POIAt(g.World.Start.X, g.World.Start.Y); poi != nil {
		g.Say(poi.Name, poi.Tag+"\n\nPress Z on a location to go inside. M opens the map.")
	}
}
