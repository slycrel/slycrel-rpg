package game

import (
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Somebody walking behind you who is not in the company.
//
// **An escortee is a fourth follower and not a fourth party member**, and the
// reason is measured rather than chosen: the battle panel is 156 pixels tall
// and a row needs about fifty — a 34x46 stage, a name, hit points, two meters
// and the condition pips. Four rows want two hundred. The panel cannot grow;
// it runs from six to a hundred and sixty-two and the transcript starts at a
// hundred and sixty-eight. So a fourth combatant means rows too short to read
// who is about to fall over, which is the reason party.MaxSize is three.
//
// That constraint turns out to be the feature. An escortee is not a hireling
// you got for free — they are baggage that talks. They do not take a slot, so a
// full company can take the errand; they never appear in the battle panel; and
// the ones who help do it from off it, in a line of transcript, which is
// exactly as much presence as somebody sheltering behind three armed people
// should have.
type escortee struct {
	// Who, and which errand they belong to. The quest is the owner: close it
	// and they are gone, which is why nothing here is saved separately.
	Quest *quest.Quest
	Name  string
	Look  string
	// Helps is set for the ones who join in rather than hide.
	Helps bool
	// walker is where they are standing, rebuilt from the hero on every map
	// exactly as the company's line is.
	sprite string
}

// escorting reports whether somebody is being walked somewhere, and who.
//
// Derived from the quest log rather than stored, so it cannot disagree with it.
// A save carries the errand; the person is a fact about the errand, and
// rebuilding them costs nothing next to the bug where the two drift apart and
// the player is escorting a ghost.
func (g *Game) escorting() *escortee {
	for _, q := range g.Quests.Active() {
		if q.Kind != quest.Escort || q.Complete() {
			continue
		}
		look := escortLooks[len(q.Escortee)%len(escortLooks)]
		return &escortee{
			Quest: q, Name: q.Escortee, Look: look, Helps: q.Helps,
			sprite: look + "/idle",
		}
	}
	return nil
}

// escortLooks are the sheets somebody being walked somewhere turns up wearing.
//
// Keyed off the length of their name rather than rolled, so the same person
// looks the same on every map without anything being stored about them. It is
// the same trick the settlement uses to decide which villager is holding the
// errand — a hash of something stable, rather than a field.
var escortLooks = []string{"hero/druid", "hero/mage", "hero/thief", "hero/fighter"}

// escortCount is how many extra walkers the marching line needs.
func (g *Game) escortCount() int {
	if g.escorting() == nil {
		return 0
	}
	return 1
}

// expireEscorts closes any errand whose clock has run out.
//
// Checked on the step rather than on arriving, because a deadline that is only
// noticed when you reach the place is not a deadline — it is a surprise at the
// end of a walk you had already finished.
func (g *Game) expireEscorts() {
	for _, q := range g.Quests.Active() {
		if !q.Expired(g.Clock.Step) {
			continue
		}
		g.Quests.Close(q)
		who := q.Escortee
		if who == "" {
			who = "your charge"
		}
		g.Log.AddColor(render.ColBlood,
			"%s has given up on reaching %s in time, and goes home.", who, q.TargetName)
	}
}

// escortHelp is the one thing an escortee does in a fight, when they are the
// helping sort.
//
// Off the panel and into the transcript. They have no meters, no turn and no
// target of their own — what they have is a rock and an opinion, and the point
// is that the player can tell the difference between walking somebody useful
// and walking somebody who screams.
func escortHelp(g *Game) (string, int, bool) {
	e := g.escorting()
	if e == nil || !e.Helps || g.Player == nil {
		return "", 0, false
	}
	if !g.RNG.Chance(0.25) {
		return "", 0, false
	}
	// A share of what the hero's own blow is worth, so it stays proportionate
	// at every level without a second damage table to keep in step.
	lo, hi := 1, 1+g.Player.Level
	return e.Name, g.RNG.Between(lo, hi), true
}

// drawEscortee paints whoever is being walked somewhere, at the back of the
// line. Split from drawFollowers because that one indexes the company and this
// is deliberately not in it.
func (g *Game) drawEscortee(ctx *render.Ctx, line party.Line) {
	e := g.escorting()
	if e == nil || len(line) <= len(g.Allies) {
		return
	}
	w := &line[len(g.Allies)]
	sp := g.Assets.Get(heroSpriteKey(&model.Character{Sprite: e.Look}, w.Dir(), w.Moving()))
	if sp == nil {
		return
	}
	x, y := w.Pixel()
	frame := g.Tick() / 14
	if w.Moving() {
		frame = g.Tick() / 6
	}
	ctx.Shadow(x, y)
	ctx.World(sp, frame, x, y, false)
}

// escortLabel is what floats over them, so a player can tell which of the
// people behind them is the one they are being paid for.
func (g *Game) escortLabel() (string, world.EntityKind, bool) {
	e := g.escorting()
	if e == nil {
		return "", "", false
	}
	return e.Name, world.ENPC, true
}
