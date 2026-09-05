package game

import (
	"fmt"
	"image/color"

	"github.com/slycrel/slycrel-rpg/internal/assetsys"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// What a marker on the ground is worth walking to.
//
// The map used to draw a silhouette per monster kind, and a third of the roster
// is "beast" — so a wolf, a bear, an owl and an ox all stood in the grass as
// the same crab. A playthrough reported the icons as random, and they were
// effectively random: eight pictures over seventy-nine creatures, keyed to a
// category the player has no way to learn.
//
// What is worth knowing from across a field is not what species is standing
// there. It is whether to go over. So the marker says that and only that, in
// three states, and the picture underneath can be whatever it likes.

// rollOmen decides what a fresh overworld encounter will be.
//
// The shares live in internal/world beside the interior roll, because a marker
// that means one thing in a field and another underground is two markers.
func rollOmen(g *core.RNG) world.Omen {
	switch r := g.Float(); {
	case r < world.BoonShare:
		return world.OmenBoon
	case r < world.BoonShare+world.MysteryShare:
		return world.OmenMystery
	}
	return world.OmenHostile
}

// resolveMystery is what a mystery turns out to be when it is reached.
//
// Slightly against the player, and deliberately: a coin-flip marker that pays
// out half the time is a marker you always walk to, which makes it the same as
// no marker at all. At two in five it is a gamble somebody can decline, and
// declining is the only thing that makes taking it a decision.
func resolveMystery(g *core.RNG) world.Omen {
	if g.Chance(0.40) {
		return world.OmenBoon
	}
	return world.OmenHostile
}

// The three marks, hand-set at seven pixels for the reason the star beside them
// is: at this size a rasteriser produces a smudge and an argument.
//
// They are told apart by silhouette first and colour second, which is the right
// way round — colour on grass at dusk under rain is not something to hang a
// decision on, and the shapes are a spike, a ring and a hook.
var (
	// A blade pointing down at you: solid at the top, tapering to a point.
	//
	// Mass rather than outline. The first draft was two strokes meeting in a
	// stem and at seven pixels over a creature it read as a bird — the marks
	// are told apart by silhouette first, and a silhouette made of thin lines
	// has none at this size.
	hostileGlyph = []string{
		"#######",
		"#######",
		".#####.",
		".#####.",
		"..###..",
		"..###..",
		"...#...",
	}
	// A ring, closed and even. Nothing in the game's own vocabulary is a
	// circle, which is what makes it read as "not a creature".
	boonGlyph = []string{
		"..###..",
		".##.##.",
		"##...##",
		"##...##",
		"##...##",
		".##.##.",
		"..###..",
	}
	// A hook over a stop, which is a question mark with the dot moved up so it
	// survives being seven pixels tall.
	mysteryGlyph = []string{
		".#####.",
		"##...##",
		".....##",
		"...###.",
		"..##...",
		".......",
		"..##...",
	}
)

// omenMark is the glyph and colour for an omen, and whether there is one at all.
func omenMark(o world.Omen, tick int) ([]string, color.Color, bool) {
	switch o {
	case world.OmenHostile:
		// The blood red the transcript already uses for damage, so the one
		// colour that means "this will cost you" means it everywhere.
		return hostileGlyph, color.RGBA{0xC8, 0x40, 0x40, 0xFF}, true
	case world.OmenBoon:
		// Green, which is the only hue on this palette the interface never uses
		// for anything else — gold is already "yours" and blue is already magic.
		return boonGlyph, color.RGBA{0x70, 0xD0, 0x70, 0xFF}, true
	case world.OmenMystery:
		// Pale and pulsing. A mystery is the one mark that should catch the eye
		// without saying anything, so it is the brightest and the least stable.
		c := color.RGBA{0xD8, 0xD0, 0xF0, 0xFF}
		if (tick/18)%2 == 0 {
			c = color.RGBA{0xA0, 0x98, 0xC0, 0xFF}
		}
		return mysteryGlyph, c, true
	}
	return nil, nil, false
}

// What a boon actually is when you reach it.
//
// Four, and they are chosen so that no two of them are the same answer to the
// same problem: one restores what fighting costs, one restores what casting
// costs, one pays, and one tells you something. A player who is out of hit
// points and a player who is out of psyche are in different trouble, and the
// game had exactly one answer to both of them — walk back to an inn.
type boonKind int

const (
	boonSpring boonKind = iota // hit points
	boonWell                   // psyche
	boonCache                  // coins, and sometimes something in the pack
	boonScout                  // marks the nearest place you have not found
	// boonWayside is the one that is a place rather than an event: a fire in a
	// clearing with somebody selling something at it. The other four happen to
	// you in a sentence; this one you walk into and walk out of.
	boonWayside
)

// pickBoon chooses what is standing there, weighted by what the company could
// actually use.
//
// Read off the party rather than rolled flat, because a spring offered to
// somebody at full health is the marker teaching a player that green means
// nothing. The weighting is soft — a full party still meets springs — since a
// world that only ever hands you what you need is a world with no reason to
// plan.
func (g *Game) pickBoon(rng *core.RNG) boonKind {
	hurt, spent := false, false
	for _, c := range g.Party() {
		if c.HP*4 < c.MaxHP*3 {
			hurt = true
		}
		if c.MaxPsyche > 0 && c.Psyche*2 < c.MaxPsyche {
			spent = true
		}
	}
	pool := []boonKind{boonCache, boonScout, boonSpring, boonWell, boonWayside}
	if hurt {
		pool = append(pool, boonSpring, boonSpring)
	}
	if spent {
		pool = append(pool, boonWell, boonWell)
	}
	return core.Pick(rng, pool)
}

// grantBoon plays out a good encounter and says what it was.
//
// Everything here is deliberately modest. A field that fully heals a company is
// an inn you do not have to walk to, and the inn's job is not hit points — it
// is the place a run is written down. So a spring gives back a share of what is
// missing rather than all of it, and nothing here autosaves.
func (g *Game) grantBoon(rng *core.RNG, where string) {
	switch g.pickBoon(rng) {
	case boonSpring:
		healed := 0
		for _, c := range g.Party() {
			if !c.Alive() {
				continue
			}
			healed += c.Heal(core.Max(1, (c.MaxHP-c.HP)/2+c.MaxHP/10))
		}
		g.Sound.Play("world/loot")
		if healed == 0 {
			g.Say("", "A spring, clear and very cold. Nobody needs it, which is "+
				"its own kind of luck. You drink anyway.")
			return
		}
		g.Say("", fmt.Sprintf("A spring, clear and very cold. The company drinks and "+
			"stops limping. %d between you.", healed))

	case boonWell:
		back := 0
		for _, c := range g.Party() {
			if !c.Alive() || c.MaxPsyche == 0 {
				continue
			}
			n := core.Max(1, (c.MaxPsyche-c.Psyche)/2+c.MaxPsyche/10)
			before := c.Psyche
			c.Psyche = core.Clamp(c.Psyche+n, 0, c.MaxPsyche)
			back += c.Psyche - before
		}
		g.Sound.Play("world/loot")
		if back == 0 {
			g.Say("", "An old well with nothing down it but weather. Something "+
				"about standing near it is restful, and you are already rested.")
			return
		}
		g.Say("", fmt.Sprintf("An old well with nothing down it but weather. You "+
			"stand near it a while and come away sharper. %d SP.", back))

	case boonCache:
		coins := int64(rng.Between(8, 20) * core.Max(1, g.Player.Level))
		g.Player.Coins += coins
		g.Sound.Play("world/loot")
		g.Say("", fmt.Sprintf("Somebody buried this and did not come back for it. "+
			"%d coins, and a hole where a %s used to be.", coins, where))

	case boonWayside:
		g.enterWayside(rng)

	case boonScout:
		if idx, name, ok := g.nearestUnfound(); ok {
			g.World.POIs[idx].Discovered = true
			g.trackIfIdle(idx, name)
			g.Sound.Play("ui/page")
			g.Say("", fmt.Sprintf("A traveller going the other way, in a hurry and "+
				"talkative with it. %s, they say, and point. It goes on your map.", name))
			return
		}
		// Nothing left to point at, which is a real state late in a run.
		coins := int64(rng.Between(6, 14) * core.Max(1, g.Player.Level))
		g.Player.Coins += coins
		g.Sound.Play("world/loot")
		g.Say("", fmt.Sprintf("A traveller going the other way, out of directions "+
			"and embarrassed about it. They give you %d coins instead.", coins))
	}
}

// nearestUnfound is the closest location the player has not discovered.
//
// It only ever names somewhere that exists and has not been found, which is the
// same rule the quest generator follows: never name something that might not be
// there. When everything is found it returns false and the caller pays instead
// — a boon that silently did nothing would be the marker lying.
func (g *Game) nearestUnfound() (int, string, bool) {
	if g.World == nil {
		return 0, "", false
	}
	best, bestD := -1, 1<<30
	for i, p := range g.World.POIs {
		if p.Discovered {
			continue
		}
		if d := p.Pos.Manhattan(g.Walk.Tile); d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return 0, "", false
	}
	return best, g.World.POIs[best].Name, true
}

// enterWayside drops the party into a place that was not on the map.
//
// It is the same scene an interior uses, on a LocalMap with a POI that is not
// in the world's list — and everything downstream already handles that, because
// `currentPOIIndex` has always been able to return -1 and every caller of it
// already checks. Quests do not advance here, sagas do not fire here and no
// tracker points at it, which is right: this is somewhere nobody has heard of.
func (g *Game) enterWayside(rng *core.RNG) {
	if g.World == nil || g.Player == nil {
		return
	}
	// Its own seed, so a wayside is a different one every time. Nothing about
	// it is saved, so there is nothing for a stable seed to be stable for.
	l := world.BuildWayside(int64(rng.Intn(1<<30)), g.encounterLevel(g.Walk.Tile), g.Write)

	g.floor = 0
	g.Local = l
	g.LocalWalk = core.NewWalker(7)
	g.LocalWalk.Place(l.Entry)
	g.reformLines()
	g.localFollow.Place(l.Entry)
	g.Sound.Play("world/enter")
	g.Push(newLocalScene(g))
	g.Log.AddColor(render.ColGold, "A fire, and people around it. Nobody was expecting you.")
}

// enterMade drops the party into a place an errand invented.
//
// The same transient-interior path a wayside uses, and for the same reasons:
// the POI is not in the world's list, so `poiIndex` cannot find it, so a save
// taken here records the party standing on the overworld — which is where
// walking back out puts them. Nothing new is stored anywhere.
//
// What is different is the seed. A wayside is drawn fresh every time because it
// is weather; this one is derived from the quest's own ID, so the crossroads is
// the same crossroads on the second visit as on the first. An errand that moved
// its own destination between two walks would be the errand lying.
func (g *Game) enterMade(q *quest.Quest) {
	if g.World == nil || g.Player == nil {
		return
	}
	host := q.Giver + "'s man"
	if q.Kind == quest.Deliver {
		host = "somebody expecting a parcel"
	}
	l := world.BuildErrandSite(q.SiteSeed(), g.encounterLevel(g.Walk.Tile), g.Write,
		q.TargetName, "nobody's idea of a landmark", host)

	g.floor = 0
	g.Local = l
	g.LocalWalk = core.NewWalker(7)
	g.LocalWalk.Place(l.Entry)
	g.reformLines()
	g.localFollow.Place(l.Entry)
	g.Sound.Play("world/enter")
	g.Push(newLocalScene(g))
	g.Log.AddColor(render.ColGold, "%s. Somebody is here.", q.TargetName)

	// The errand advances on arriving, exactly as one pointing at a settlement
	// does — the difference between the two is where you are standing, not what
	// the errand is.
	g.noteQuestProgress(g.Quests.OnReachedMade(q))
}

// drawMadeMarker rings the spot an errand is pointing at.
//
// A ring on the ground rather than a building or a hole in a hill, because it
// is neither: the overworld's markers say what *kind* of place something is,
// and the honest answer here is "somewhere you were told to meet somebody".
// Gold, and it breathes, so it reads as a thing the errand put there rather
// than as a feature of the country.
func drawMadeMarker(ctx *render.Ctx, at core.Point, tick int) {
	const ts = assetsys.TileSize
	ox, oy := ctx.Cam.Offset()
	x, y := float64(at.X*ts)+ox, float64(at.Y*ts)+oy
	col := color.RGBA{0xE0, 0xB0, 0x4C, 0xFF}
	if (tick/24)%2 == 0 {
		col = color.RGBA{0xFF, 0xE0, 0x90, 0xFF}
	}
	// Two rings, the inner one dark, so it holds its shape over grass and over
	// sand. A single stroke at this size disappears into whichever of the two
	// it happens to be sitting on.
	render.Frame(ctx.Dst, x+1, y+1, ts-2, ts-2, color.RGBA{0x30, 0x24, 0x10, 0xC0})
	render.Frame(ctx.Dst, x+2, y+2, ts-4, ts-4, col)
}
