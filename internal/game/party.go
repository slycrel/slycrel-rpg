package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Party returns the hero followed by every companion.
func (g *Game) Party() []*model.Character { return party.Members(g.Player, g.Allies) }

// LivingParty returns the members still on their feet.
func (g *Game) LivingParty() []*model.Character { return party.Living(g.Party()) }

// PartyFull reports whether there is no room for another hireling.
func (g *Game) PartyFull() bool { return party.Full(len(g.Allies)) }

// reclaim moves a departing companion's supplies into the hero's pack and
// reports how many items came back.
func (g *Game) reclaim(c *model.Character) int {
	n := 0
	for _, it := range c.Bag {
		n += it.Count
		g.Player.AddItem(it)
	}
	c.Bag = nil
	return n
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// uniqueName keeps two members of one company from answering to the same thing.
func (g *Game) uniqueName(name string) string { return party.UniqueName(g.Party(), name) }

// encounterSize scales a rolled encounter to the size of the company.
func (g *Game) encounterSize(base int) int {
	return party.EncounterSize(g.RNG, base, len(g.Allies))
}

// restParty puts everyone back to full.
func (g *Game) restParty() { party.Rest(g.Party()) }

// --- hiring ---------------------------------------------------------------

// offerRecruit runs the negotiation with someone loitering outside an inn.
//
// The hireling is rolled at the hero's level rather than at some level stamped
// on the entity when the town generated. A town you walk back into at level 12
// should not be offering the level-3 hopeful it was offering then — and pricing
// the fee off the same number keeps the trade honest at both ends.
func (g *Game) offerRecruit(e *world.Entity) {
	level := core.Max(1, g.Player.Level)
	blood := model.MonsterKind(e.Blood)
	cost := rules.HireCost(level, blood, rules.Read(g.Player))

	trade := strings.ToLower(e.Class)
	if l, ok := model.LineageOf(blood); ok {
		trade = fmt.Sprintf("%s, and %s", trade, l.Tag)
	}
	body := fmt.Sprintf("%s\n\nA %s. %d coins up front, and a cut of everything after.",
		e.Line, trade, cost)

	switch {
	case g.PartyFull():
		g.Say(e.Name, e.Line+"\n\nYou already have as many people as you can keep track of. "+
			"They take this well, which is somehow worse.")
		return
	case g.Player.Coins < cost:
		g.Say(e.Name, body+"\n\nYou do not have it. They resume looking at the middle distance.")
		return
	}

	// The purse is already checked above, so this cannot be greyed out — but it
	// says what the hire costs against what you are holding, which is the half
	// that was missing.
	g.AskMenu(e.Name, fmt.Sprintf("%s\n\nYou have %d coins.", body, g.Player.Coins),
		[]ui.MenuItem{
			{Label: "Pay", Detail: fmt.Sprintf("%d coins", cost)},
			{Label: "Walk away"},
		},
		func(g *Game, choice int) {
			if choice != 0 {
				return
			}
			// Re-check: the box is modal, but the price was quoted a frame ago
			// and nothing guarantees the purse survived the interval.
			if g.Player.Coins < cost || g.PartyFull() {
				return
			}
			g.Player.Coins -= cost
			g.spend(e)
			g.hire(e, level)
		})
}

// hire rolls the companion the entity was advertising and forms them up behind
// the player.
func (g *Game) hire(e *world.Entity, level int) {
	class := model.Class(e.Class)
	switch class {
	case model.ClassFighter, model.ClassThief, model.ClassMage:
	default:
		class = model.ClassFighter
	}

	c := rules.Recruit(g.RNG, g.uniqueName(e.Name), class, model.MonsterKind(e.Blood), level)
	g.Data.Equip(c)
	c.Sprite = e.Look
	c.Portrait = allyPortrait(g.RNG)

	// Hiring happens outside an inn in front of whoever is passing.
	g.Player.Renown++

	g.Allies = append(g.Allies, c)
	g.reformLines()
	g.ensureThreads()
	g.Sound.Play("world/coins")

	what := fmt.Sprintf("%s, %s, level %d. Takes %d%% of the coin.",
		c.Name, c.Class, c.Level, c.Cut)
	if l, ok := model.LineageOf(c.Blood); ok {
		what += fmt.Sprintf("\n%s. %s", capitalise(l.Tag), l.Note)
		// Say what the ancestry actually bought, since the numbers are already
		// folded into the stat line by the time anyone can look at it.
		if ability := g.bloodAbility(c); ability != "" {
			what += fmt.Sprintf("\nKnows %s, which nobody else will.", ability)
		}
	}
	g.Say("", g.Write.RecruitJoin(g.RNG, c.Name)+"\n\n"+what)
}

// bloodAbility names the technique a companion's ancestry grants, if they are
// high enough level to have grown into it yet.
func (g *Game) bloodAbility(c *model.Character) string {
	for _, s := range g.Data.SpellsFor(c) {
		if s.Blood != "" {
			return s.Name
		}
	}
	return ""
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// dismiss lets a companion go, and gives back nothing. The fee was for the
// hiring, not for the arrangement.
func (g *Game) dismiss(c *model.Character) {
	for i, a := range g.Allies {
		if a != c {
			continue
		}
		g.Allies = append(g.Allies[:i], g.Allies[i+1:]...)
		g.reformLines()
		g.Threads.Drop(c.Name)
		g.Log.AddColor(render.ColInkDim, "%s", g.Write.RecruitLeave(g.RNG, c.Name))
		// Whatever you bought them comes back. They are being let go, not
		// robbing you — and a pack that vanished on dismissal would make
		// supplying anybody a bet rather than a purchase.
		if n := g.reclaim(c); n > 0 {
			g.Log.AddColor(render.ColGold, "They hand back %d %s first.", n, plural(n, "thing", "things"))
		}
		return
	}
}

// allyPortrait picks a battle portrait for a hireling. The hero is always
// m_01, so anyone else drawing from the wider pool is immediately not the hero.
func allyPortrait(g *core.RNG) string {
	pool := []string{
		"portrait/male/m_07", "portrait/male/m_14", "portrait/male/m_22",
		"portrait/male/m_31", "portrait/male/cultist_02",
		"portrait/female/f_03", "portrait/female/f_08", "portrait/female/f_12",
		"portrait/female/f_17", "portrait/female/f_24",
	}
	return core.Pick(g, pool)
}

// --- getting back up -------------------------------------------------------

// rescueToTown is what happens when the hero falls and somebody is still
// standing: the company picks him up and walks him to the nearest settlement.
//
// This is the answer to the question the roadmap had left open about death. It
// is not a free life — it costs a large share of the purse, a point of Shame,
// and however far you had walked — but it is not the end of the run either, and
// it turns hirelings from a damage sponge into the reason there is still a run
// to come back to. Dying alone still ends it.
func (g *Game) rescueToTown() {
	carrier := "Somebody"
	for _, c := range g.Allies {
		if c.Alive() {
			carrier = c.Name
			break
		}
	}

	// Back to the continent first: the fight may have started inside a
	// location, and waking up in the dungeon you died in would be a strange
	// definition of rescue.
	g.dropToOverworld()

	town := g.nearestSettlement()
	place := "somewhere with a roof"
	if town != nil {
		place = town.Name
		g.Walk.Place(town.Pos)
		g.World.Reveal(town.Pos, 8)
		town.Discovered, town.Visited = true, true
	}
	g.follow.Place(g.Walk.Tile)

	fee := rules.RescueFee(g.Player.Coins)
	g.Player.Coins -= fee
	g.Player.Shame++
	// Being carried through the gate is the most public thing that can happen
	// to anybody, and it is not the deeds half that it adds to.
	g.Player.Renown++
	g.restParty()
	g.sinceFight = 0

	body := g.Write.Rescue(g.RNG, carrier, place)
	if fee > 0 {
		body += fmt.Sprintf("\n\nIt cost %d coins. Nobody itemised it.", fee)
	} else {
		body += "\n\nYou had nothing to take, which they establish thoroughly."
	}
	g.Sound.Play("world/enter")
	g.Say("", body)
}

// nearestSettlement finds somewhere with a bed, measured from where the player
// is standing. Every world generates a capital, so this only comes back nil for
// a world that has no locations at all.
func (g *Game) nearestSettlement() *world.POI {
	var best *world.POI
	bestD := 1 << 30
	for _, p := range g.World.POIs {
		if !p.Kind.Settlement() {
			continue
		}
		if d := p.Pos.Manhattan(g.Walk.Tile); d < bestD {
			best, bestD = p, d
		}
	}
	return best
}

// --- the line -------------------------------------------------------------

// reformLines resizes the follower walkers to match the roster and stacks them
// on whoever they are following. Called whenever the party changes or a map is
// entered.
func (g *Game) reformLines() {
	g.follow = party.Fit(g.follow, len(g.Allies), g.Walk.Tile, 9)
	g.localFollow = party.Fit(g.localFollow, len(g.Allies), g.LocalWalk.Tile, 7)
}

// drawFollowers paints the companions behind the hero. They are drawn back to
// front so the one nearest the player overlaps the one behind it, and the hero
// is drawn after this so nothing ever covers the character you are steering.
func (g *Game) drawFollowers(ctx *render.Ctx, line party.Line) {
	for i := len(line) - 1; i >= 0; i-- {
		if i >= len(g.Allies) {
			continue
		}
		c := g.Allies[i]
		w := &line[i]
		px, py := w.Pixel()
		sp := g.Assets.Get(heroSpriteKey(c, w.Dir(), w.Moving()))
		frame := g.Tick() / 14
		if w.Moving() {
			frame = g.Tick() / 6
		}
		ctx.Shadow(px, py)
		ctx.World(sp, frame, px, py, false)
	}
}

// --- the party panel ------------------------------------------------------

// partyRowH is the height of one member's row in the battle panel.
const partyRowH = 16

// drawPartyPanel paints the company's meters into the battle screen's left
// panel. A solo hero keeps the original portrait-and-bars layout, because
// shrinking a one-person party into a list of one would be a downgrade for the
// majority of runs; the rows only appear once there is a party to list.
func (g *Game) drawPartyPanel(dst *ebiten.Image, x, y, w, h float64, hurt map[*model.Character]int) {
	party := g.Party()
	if len(party) == 1 {
		g.drawSoloPanel(dst, x, y, w, h, hurt)
		return
	}

	ui.TitledPanel(dst, "", x, y, w, h)
	for i, c := range party {
		ry := y + 6 + float64(i)*partyRowH
		tint := memberTint(c, hurt[c])

		render.ScreenFit(dst, g.Assets.Get(portraitOf(c)), 0, x+4, ry, 14, 14, tint)

		name := render.ColInk
		if !c.Alive() {
			name = render.ColInkFaint
		}
		render.Text(dst, render.Trunc(c.Name, 80), x+22, ry+1, name)

		ui.Bar(dst, x+106, ry+4, 46, 5, c.HPFrac(), render.ColBlood)
		ui.Bar(dst, x+156, ry+4, 26, 5, c.PsycheFrac(), render.ColMagic)
		// Under the meters rather than beside the name: a row is sixteen pixels
		// and the bars only use six of them, so this is the one place in the
		// panel that was not already spoken for. Squeezing the name instead
		// turned "Ilsabet Dun" into "Ilsabet.".
		drawEffectPips(dst, x+106, ry+10, c.Active)
	}
}

// effectColour is the pip a condition draws as. They are read at three pixels
// square, so the palette has to separate on hue alone: green is something in
// you, orange is something on you, gold is help and grey is harm.
func effectColour(k model.EffectKind) color.RGBA {
	switch k {
	case model.EffectPoison:
		return color.RGBA{0x5C, 0xC0, 0x50, 0xFF}
	case model.EffectBurn:
		return color.RGBA{0xE8, 0x7A, 0x28, 0xFF}
	case model.EffectBless:
		return color.RGBA{0xE0, 0xB0, 0x4C, 0xFF}
	case model.EffectQuicken:
		return color.RGBA{0x60, 0xC8, 0xE0, 0xFF}
	case model.EffectStun:
		return color.RGBA{0xF0, 0xE8, 0xC0, 0xFF}
	default: // weaken
		return color.RGBA{0x90, 0x70, 0xB0, 0xFF}
	}
}

// maxPips is how many conditions fit beside a name before the row runs out of
// width. Nothing in the game stacks more than this at once; if something ever
// does, the overflow is silent rather than drawn over the meters.
const maxPips = 4

// drawEffectPips paints one small square per active condition. Without these
// the player is poisoned and has no way of knowing except by watching their
// own hit points drop for reasons the transcript has already scrolled past.
func drawEffectPips(dst *ebiten.Image, x, y float64, list model.Effects) {
	for i, e := range list {
		if i >= maxPips {
			return
		}
		px := x + float64(i)*4
		render.Rect(dst, px, y, 3, 3, effectColour(e.Kind))
		render.Frame(dst, px, y, 3, 3, color.RGBA{0, 0, 0, 0x80})
	}
}

// drawSoloPanel is the original one-hero layout: a large portrait and labelled
// meters, which reads better than a single row in a list.
func (g *Game) drawSoloPanel(dst *ebiten.Image, x, y, w, h float64, hurt map[*model.Character]int) {
	p := g.Player
	ui.TitledPanel(dst, render.Trunc(p.Name, w-68), x, y, w, h)
	render.ScreenFit(dst, g.Assets.Get(portraitOf(p)), 0, x+4, y+4, 46, 46, memberTint(p, hurt[p]))
	ui.StatBars(dst, x+56, y+6, w-66, p.HP, p.MaxHP, p.Psyche, p.MaxPsyche)
	drawEffectPips(dst, x+56, y+h-10, p.Active)
}

// memberTint flashes a member red on the frames just after they were hit, and
// leaves anyone unconscious drawn cold.
func memberTint(c *model.Character, hurt int) color.Color {
	if !c.Alive() {
		return color.RGBA{0x50, 0x40, 0x50, 0xB0}
	}
	if hurt > 0 && (hurt/3)%2 == 0 {
		return color.RGBA{0xFF, 0x80, 0x80, 0xFF}
	}
	return nil
}

// portraitOf returns a character's battle portrait, defaulting to the hero's.
func portraitOf(c *model.Character) string {
	if c.Portrait != "" {
		return c.Portrait
	}
	return "portrait/male/m_01"
}
