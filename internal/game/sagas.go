package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// The long stories: the spine that starts at the gate, and the arcs found out
// in the world.
//
// Nearly all of the interesting decisions live in internal/saga. What is here
// is the wiring — which event fires which trigger, and where it is safe to put
// a box on screen — which is what this package is supposed to be left holding.

// arcCap is how many optional arcs can run at once.
//
// Two, matching the resident stories, and for the same reason: what these
// compete for is not the player's time but their attention, and a journal with
// six long stories in it is a journal nobody reads. The spine does not count
// against it — there is only ever one of those, and it is the reason to be
// here.
const arcCap = 2

// arcChance is how often walking into somewhere nobody sent you turns out to
// have a story in it.
//
// One in six, and only in places that are not settlements. An arc is meant to
// feel found, and a thing that happens every time you open a door is not found,
// it is issued.
const arcChance = 6

// beginSaga casts the main spine and delivers its opening.
//
// Called once, from startRun, so the reason to be here is there before the
// player has done anything. A spine offered later would have to interrupt
// something, and a spine offered on a condition would be a spine most runs
// never see.
func (g *Game) beginSaga() {
	if g.World == nil || g.Data == nil {
		return
	}
	spines := g.Data.Sagas.Spines()
	if len(spines) == 0 {
		return
	}
	// Forked off the run's seed so the same seed opens the same story, and so
	// that casting cannot shift the rolls the player is about to make.
	rng := g.RNG.Fork("saga", g.Seed)
	sk := core.Pick(rng, spines)
	s, ok := saga.Cast(rng, &g.Data.Sagas, g.World, g.Data, sk,
		g.World.Start, core.Max(1, g.Player.Level), g.Sagas.IDs())
	if !ok {
		// A continent too small to stage this one. Supported, and silent: the
		// alternative is a story that cannot be finished, which is worse than
		// no story at all.
		return
	}
	g.Sagas.Add(s)
	g.showSagaDestination(s)
	g.SayThen("", s.FillAt(sk.Opening, 0), func(g *Game) {
		g.Log.AddColor(render.ColGold, "%s. %s", s.Title, s.Note(&g.Data.Sagas))
	})
}

// offerArc rolls for a short story in a place nobody sent the player to.
//
// The roll is against the location's own seed rather than the live generator,
// so whether a given ruin has something in it is a fact about the ruin and not
// about the order you visited things in. Walking out and back in does not
// reroll it, which is the same rule the errand givers and the storytellers
// follow.
func (g *Game) offerArc(idx int) {
	if g.World == nil || g.Data == nil || idx < 0 || idx >= len(g.World.POIs) {
		return
	}
	p := g.World.POIs[idx]
	if p.Kind.Settlement() {
		return
	}
	// Arcs counted, not stories: the spine does not compete for the cap, and a
	// finished spine must not free a slot for a third arc. Counting Running()
	// did both wrong.
	arcs := 0
	for _, s := range g.Sagas.Running() {
		if sk, ok := g.Data.Sagas.Get(s.Skeleton); ok && sk.Arc {
			arcs++
		}
	}
	if arcs >= arcCap {
		return
	}
	// Positive remainder. A location's seed comes off an RNG and can be
	// negative, and Go's % keeps the sign of the dividend — so half the
	// continent would have been quietly ineligible, in a way that would have
	// looked exactly like the rate being what it is.
	if ((p.Seed%int64(arcChance))+int64(arcChance))%int64(arcChance) != 0 {
		return
	}
	pool := g.Data.Sagas.Arcs()
	if len(pool) == 0 {
		return
	}

	rng := g.RNG.Fork("arc", g.Seed^p.Seed)
	sk := core.Pick(rng, pool)
	s, ok := saga.Cast(rng, &g.Data.Sagas, g.World, g.Data, sk,
		p.Pos, core.Max(1, g.Player.Level), g.Sagas.IDs())
	if !ok {
		return
	}
	g.Sagas.Add(s)
	g.showSagaDestination(s)
	g.Sound.Play("ui/page")
	g.SayThen("", s.FillAt(sk.Opening, 0), func(g *Game) {
		g.Log.AddColor(render.ColGold, "%s. %s", s.Title, s.Note(&g.Data.Sagas))
	})
}

// showSagaDestination puts the current leg's place on the map.
//
// Without this the whole feature has a dead end in it. A spine's legs are
// deliberately further out each time, so by the third one the player is being
// sent somewhere they have every chance of never having seen — and a journal
// line naming a place the map has not heard of is not directions, it is a
// riddle.
func (g *Game) showSagaDestination(s *saga.Saga) {
	idx := s.Place()
	if g.World == nil || idx < 0 || idx >= len(g.World.POIs) {
		return
	}
	p := g.World.POIs[idx]
	// Followed whether or not the map already knew about it. Revealing is a
	// one-time thing and following is not: a leg pointing at somewhere the
	// player has already walked past still needs a direction on it.
	g.trackIfIdle(idx, p.Name)
	if p.Discovered {
		return
	}
	p.Discovered = true
	g.World.Reveal(p.Pos, 4)
	g.Log.AddColor(render.ColGold, "%s is on your map now.", p.Name)
}

// advanceSagas reports something that happened and queues whatever came due.
//
// Nothing is shown here, for the same reason thread beats are not: a leg can
// come due in the middle of a fight, and a message box over a battle is both
// ugly and briefly ambiguous about who is being asked. The queue drains
// somewhere it is safe to interrupt.
func (g *Game) advanceSagas(ev saga.Event) {
	if g.Data == nil {
		return
	}
	for _, f := range g.Sagas.Advance(&g.Data.Sagas, ev) {
		g.pendingLegs = append(g.pendingLegs, f)
		if !f.Last {
			g.showSagaDestination(f.Saga)
		}
	}
}

// serviceSagas says one queued leg if the moment is right for it, and reports
// whether it put a box up.
//
// Ordered before the companions' beats by the caller, because a saga leg is the
// thing the player went somewhere for and a backstory beat is a thing that
// happened on the way.
func (g *Game) serviceSagas() bool {
	switch g.Top().(type) {
	case *overworldScene, *localScene:
	default:
		return false
	}
	if len(g.pendingLegs) == 0 {
		return false
	}

	f := g.pendingLegs[0]
	g.pendingLegs = g.pendingLegs[1:]
	g.Sound.Play("ui/page")
	if !f.Last {
		g.SayThen("", f.Text, func(g *Game) {
			g.Log.AddColor(render.ColGold, "%s. %s", f.Saga.Title, f.Saga.Note(&g.Data.Sagas))
		})
		return true
	}
	// The last leg is the setup for the choice, so it and the options share a
	// box rather than making the player read the same words twice.
	g.offerSagaEnding(f.Saga, f.Text)
	return true
}

// offerSagaEnding puts the choice at the end of a saga, and reports whether
// there was anything to put.
//
// Backing out is allowed and costs nothing. An ending is irreversible and a
// modal box that cannot be escaped is a poor place to be asked; the choice is
// put again the next time the player walks into a settlement, which is where
// the companion endings are re-offered too.
func (g *Game) offerSagaEnding(s *saga.Saga, setup string) bool {
	opts := s.Options(&g.Data.Sagas)
	if len(opts) == 0 {
		return false
	}
	level := int64(core.Max(1, g.Player.Level))

	rows := make([]ui.MenuItem, 0, len(opts)+1)
	for _, e := range opts {
		row := ui.MenuItem{Label: e.Label}
		if cost := e.Costs() * level; cost > 0 {
			row.Detail = fmt.Sprintf("%d coins", cost)
			row.Disabled = g.Player.Coins < cost
		}
		rows = append(rows, row)
	}
	rows = append(rows, ui.MenuItem{Label: "Not yet"})

	if setup == "" {
		setup = s.Fill(s.Title) + "\n\nThere is still a decision in it."
	}
	setup += fmt.Sprintf("\n\nYou have %d coins.", g.Player.Coins)

	g.AskMenu("", setup, rows, func(g *Game, choice int) {
		if choice < 0 || choice >= len(opts) {
			g.remindEndings = true // ask again in the next town
			return
		}
		g.resolveSaga(s, opts[choice])
	})
	return true
}

// resolveSaga applies an ending and closes the story.
func (g *Game) resolveSaga(s *saga.Saga, e saga.Ending) {
	level := int64(core.Max(1, g.Player.Level))
	coins, xp := e.Coins*level, e.XP*level

	// The purse is checked here rather than at the menu because the box was
	// drawn a frame ago and nothing guarantees the coins survived the interval.
	// Every saga is authored with an ending that costs nothing, so refusing
	// here can never leave a player holding a story they cannot finish.
	if coins < 0 && g.Player.Coins < -coins {
		g.Say("", "You do not have it, and saying so out loud does not help.")
		return
	}

	s.Resolve(e)
	g.Player.Coins += coins
	g.Player.TotalXP += xp
	g.Player.SpendXP += xp
	g.Player.Fame += e.Fame
	g.Player.Shame += e.Shame
	g.Player.Honor += e.Honor

	g.Sound.Play("world/coins")
	g.SayThen("", e.Text, func(g *Game) {
		g.Log.AddColor(render.ColGold, "%s - %s.", s.Title, e.Label)
		switch {
		case coins > 0:
			g.Log.AddColor(render.ColGold, "%d coins.", coins)
		case coins < 0:
			g.Log.AddColor(render.ColInkDim, "It cost %d coins.", -coins)
		}
		if xp > 0 {
			g.applyPendingLevels()
		}
	})
}

// remindSagaEndings puts one outstanding saga choice again, for a player who
// said "not yet" and then walked into a town.
func (g *Game) remindSagaEndings() bool {
	for _, s := range g.Sagas.Waiting() {
		if g.offerSagaEnding(s, "") {
			return true
		}
	}
	return false
}

// sagasOnEnteringPOI fires the location triggers and offers an arc.
func (g *Game) sagasOnEnteringPOI(idx int) {
	if idx < 0 {
		return
	}
	g.advanceSagas(saga.Event{Kind: saga.Reach, POI: idx})
	// Only when nothing else is already pending, so walking into the place a
	// leg was pointing at does not also hand you a second story on the doorstep.
	if len(g.pendingLegs) == 0 {
		g.offerArc(idx)
	}
}
