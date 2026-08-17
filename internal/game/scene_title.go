package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// titleScene is the front door: a name, a subtitle, and three ways in.
type titleScene struct {
	menu ui.Menu
	// stars is a slow parallax field so the screen is not dead still.
	stars []star
}

type star struct {
	x, y, speed float64
	c           color.RGBA
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
	sg := core.NewRNG(g.Seed).Fork("title", 1)
	for i := 0; i < 70; i++ {
		t.stars = append(t.stars, star{
			x: sg.Float() * render.ScreenW, y: sg.Float() * render.ScreenH,
			speed: 0.05 + sg.Float()*0.35,
			c:     color.RGBA{uint8(140 + sg.Intn(110)), uint8(120 + sg.Intn(90)), uint8(160 + sg.Intn(90)), 0xFF},
		})
	}
	return t
}

func (t *titleScene) Update(g *Game) error {
	g.Sound.Ambience("")
	for i := range t.stars {
		t.stars[i].x -= t.stars[i].speed
		if t.stars[i].x < 0 {
			t.stars[i].x += render.ScreenW
		}
	}
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
	for _, s := range t.stars {
		render.Rect(dst, s.x, s.y, 1, 1, s.c)
	}
	// A horizon band, so the stars read as sky rather than static.
	render.Rect(dst, 0, 186, render.ScreenW, render.ScreenH-186, color.RGBA{0x1A, 0x16, 0x14, 0xFF})
	render.Rect(dst, 0, 186, render.ScreenW, 1, color.RGBA{0x4A, 0x3A, 0x2A, 0xFF})

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

// createScene rolls a character: pick a class, reroll either half of them, and
// go.
//
// The three rolled characters here are the ones the game gets. They used to be
// previews in the ordinary sense — rolled from their own forked generator to
// keep the panel steady while browsing — and then startRun rolled a *fresh*
// character from the main stream and handed that to the player instead. So the
// prospects panel had never once shown the hit points anybody actually started
// with, and a stat reroll would have been rerolling numbers that never left
// this screen.
type createScene struct {
	menu    ui.Menu
	rolled  map[model.Class]*model.Character
	name    string
	epithet string
	nameRNG *core.RNG
	statRNG *core.RNG
	// shown is the class the preview panel is describing. It sticks when the
	// cursor moves onto a row that is not a class, so nothing blanks out.
	shown model.Class
}

func newCreateScene(g *Game) *createScene {
	c := &createScene{
		rolled:  map[model.Class]*model.Character{},
		nameRNG: g.RNG.Fork("names", g.Seed),
		statRNG: g.RNG.Fork("stats", g.Seed),
	}
	c.rerollStats()
	c.rerollName(g)

	items := make([]ui.MenuItem, 0, len(model.AllClasses)+1)
	for _, cl := range model.AllClasses {
		items = append(items, ui.MenuItem{Label: string(cl), Data: cl})
	}
	items = append(items, ui.MenuItem{Label: "Back"})
	c.menu.SetItems(items)
	c.shown = model.AllClasses[0]
	return c
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
	if g.Back() {
		g.Replace(newTitleScene(g))
		return nil
	}

	// Up and down pick the class; left and right reroll the two halves of the
	// person. Both land in view — the numbers are on screen when you throw them
	// and so is the name — which a menu row underneath the list could not do,
	// since choosing it moved the cursor off the class whose stats you were
	// looking at.
	if d, ok := MenuDir(); ok {
		switch d {
		case core.DirUp:
			c.menu.Move(-1)
		case core.DirDown:
			c.menu.Move(1)
		case core.DirLeft:
			c.rerollStats()
			g.Sound.Play("ui/move")
		case core.DirRight:
			c.rerollName(g)
			g.Sound.Play("ui/move")
		}
		return nil
	}

	if !g.Accept() {
		return nil
	}
	it, _ := c.menu.Selected()
	if it.Label == "Back" {
		g.Replace(newTitleScene(g))
		return nil
	}
	if class, ok := it.Data.(model.Class); ok {
		g.startRun(c.rolled[class], c.name, c.epithet)
	}
	return nil
}

func (c *createScene) Draw(g *Game, dst *ebiten.Image) {
	dst.Fill(color.RGBA{0x14, 0x10, 0x1C, 0xFF})
	render.TextCenter(dst, "WHO ARE YOU, THEN", render.ScreenW/2, 12, render.ColGold)

	// The name goes above both panels and is always on screen.
	//
	// It used to live inside the stat preview, which only draws when the cursor
	// is over a class — so moving down to "Reroll name" made the name itself
	// disappear, and the player was rerolling something they could not see.
	// The two rerolls flank the name, on the side that triggers them, so which
	// arrow does which is answered by looking rather than remembering.
	render.Text(dst, "< new stats", 12, 28, render.ColInkFaint)
	render.TextCenter(dst, c.name+" "+c.epithet, render.ScreenW/2, 28, render.ColInk)
	render.TextRight(dst, "new name >", render.ScreenW-12, 28, render.ColInkFaint)

	ui.TitledPanel(dst, "class", 12, 44, 236, 134)
	c.menu.Draw(dst, 24, 54, 216)

	// Stat preview for whichever class the cursor was last on.
	if it, _ := c.menu.Selected(); it.Data != nil {
		if cl, ok := it.Data.(model.Class); ok {
			c.shown = cl
		}
	}
	if p := c.rolled[c.shown]; p != nil {
		cl := c.shown
		ui.TitledPanel(dst, "prospects", 258, 44, 210, 134)
		x, y := 268, 54.0
		for _, ln := range render.Wrap(cl.Blurb(), 186) {
			render.Text(dst, ln, float64(x), y, render.ColInkDim)
			y += render.LineH
		}
		y += 4
		for _, row := range [][2]string{
			{"Hit points", fmt.Sprint(p.MaxHP)},
			{"Psyche", fmt.Sprint(p.MaxPsyche)},
			{"Strength", fmt.Sprint(p.Strength)},
			{"Dexterity", fmt.Sprint(p.Dexterity)},
			{"Speed", fmt.Sprint(p.Speed)},
			{"Purse", fmt.Sprintf("%d coins", p.Coins)},
		} {
			render.Text(dst, row[0], float64(x), y, render.ColInkDim)
			render.TextRight(dst, row[1], 458, y, render.ColInk)
			y += render.LineH
		}
	}

	render.TextCenter(dst, "up/down class  -  left rolls stats  -  right rolls a name",
		render.ScreenW/2, 234, render.ColInkFaint)
	render.TextCenter(dst, "Z to accept  -  X to go back", render.ScreenW/2, 246, render.ColInkFaint)
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

	g.World = world.Generate(g.Seed, g.Write)
	g.Quests = quest.Log{}
	g.Walk = core.NewWalker(9)
	g.Walk.Place(g.World.Start)

	g.Sound.Play("vo/welcome")
	g.Log.Clear()
	g.Log.AddColor(render.ColGold, "%s %s steps out into a world that did not ask for them.",
		g.Player.Name, g.Player.Epithet)
	g.Replace(newOverworldScene(g))
	if poi := g.World.POIAt(g.World.Start.X, g.World.Start.Y); poi != nil {
		g.Say(poi.Name, poi.Tag+"\n\nPress Z on a location to go inside. M opens the map.")
	}
}
