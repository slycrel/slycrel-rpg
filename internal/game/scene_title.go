package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
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
}

func newTitleScene(g *Game) *titleScene {
	t := &titleScene{}
	saves := save.List(g.Root)
	cont := ui.MenuItem{Label: "Continue", Detail: "no saves", Disabled: true}
	if len(saves) > 0 {
		cont = ui.MenuItem{Label: "Continue", Detail: humanAge(saves[0].Saved)}
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
	render.TextCenter(dst, "18+  -  contains adults behaving exactly as expected",
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

	nameRNG *core.RNG
	statRNG *core.RNG
}

// Rows on the first screen. Indexed by these rather than by number, because
// three of them do something on left/right and the fourth does not.
const (
	rowName = iota
	rowFace
	rowLook
	rowOnward
)

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

	c.who.SetItems([]ui.MenuItem{
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

	render.TextCenter(dst, "up/down pick a thing  -  left/right change it",
		render.ScreenW/2, 234, render.ColInkFaint)
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
	g.Player.Weapon = g.Data.StarterWeapon()
	g.Player.Armor = g.Data.StarterArmor()
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
	if poi := g.World.POIAt(g.World.Start.X, g.World.Start.Y); poi != nil {
		g.Say(poi.Name, poi.Tag+"\n\nPress Z on a location to go inside. M opens the map.")
	}
}
