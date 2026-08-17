package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// ensureThreads casts a backstory for anybody in the company who does not have
// one yet.
//
// This runs after hiring and after loading rather than only at the moment of
// hire, so that a save written before threads existed grows them on the way in
// instead of carrying a company of blanks forever. It is also the one place
// that has to know a thread can decline to cast: a continent with no ruin on it
// simply does not offer the threads that need one, and a companion without a
// backstory is a companion, not a bug.
func (g *Game) ensureThreads() {
	if g.World == nil || g.Data == nil {
		return
	}
	for _, c := range g.Allies {
		if g.Threads.For(&g.Data.Threads, c.Name) != nil {
			continue
		}
		// Forked so that casting a backstory cannot shift the encounter rolls
		// the player is in the middle of. Fork derives its stream from the
		// label and salt alone and never touches the parent's state, so the
		// run's seed has to go into the salt or every world would hand the same
		// person the same story. The name goes in too, so hiring two people in
		// one town does not cast them both in the same one.
		rng := g.RNG.Fork("thread", g.Seed^nameSalt(c.Name))
		t, ok := thread.Cast(rng, &g.Data.Threads, g.World, g.Data, c, g.Walk.Tile, g.Threads.IDs())
		if !ok {
			continue
		}
		g.Threads.Add(t)
	}
}

// nameSalt turns a name into a stable number. Unsigned throughout: the whole
// point is a value that varies wildly with the input, and negating a wrapped
// signed hash has one input that stays negative.
func nameSalt(name string) int64 {
	var h uint64 = 14695981039346656037
	for _, r := range name {
		h = (h ^ uint64(r)) * 1099511628211
	}
	return int64(h >> 1)
}

// advanceThreads reports something that happened to every running backstory and
// queues whatever came due.
//
// Nothing is shown here. A beat can fire in the middle of a battle, and a
// message box over a fight would be both ugly and, since the battle scene reads
// input of its own, briefly ambiguous about who is being asked. The queue is
// drained somewhere it is safe to interrupt.
func (g *Game) advanceThreads(ev thread.Event) {
	if g.Data == nil {
		return
	}
	for _, f := range g.Threads.Advance(&g.Data.Threads, ev) {
		g.pendingBeats = append(g.pendingBeats, f)
		g.showThreadDestination(f.Thread)
	}
}

// showThreadDestination puts the place a thread ends at onto the map, at the
// moment the thread starts waiting for the player to go there.
//
// Without this the feature has a dead end in it. Casting prefers somewhere the
// player has not been, the destination is a median forty-odd tiles out, and the
// only thing naming it is a journal line — so a companion could be waiting at a
// ruin the map has never heard of, with no way to find it but wandering. The
// beat that says "it is at {P}" fires on arrival, which is too late to be
// directions; this is the directions.
func (g *Game) showThreadDestination(t *thread.Thread) {
	if t.Awaiting(&g.Data.Threads) != thread.Reach {
		return
	}
	if g.World == nil || t.PlacePOI < 0 || t.PlacePOI >= len(g.World.POIs) {
		return
	}
	p := g.World.POIs[t.PlacePOI]
	if p.Discovered {
		return
	}
	p.Discovered = true
	g.World.Reveal(p.Pos, 4)
	g.Log.AddColor(render.ColGold, "%s puts %s on your map.", t.Owner, p.Name)
}

// travelWithCompany records a step for the backstories, and only counts one
// when there is somebody to take it with. Walking the continent alone is not
// travelling with anybody, and a thread is told by the person walking beside
// you: with nobody hired there is no thread to advance in any case, so the
// guard is really about saying what the count means.
func (g *Game) travelWithCompany() {
	if len(g.Allies) == 0 {
		return
	}
	g.advanceThreads(thread.Event{Kind: thread.Travel, N: 1})
}

// serviceThreads says one queued beat, if the moment is right for it, and
// otherwise puts any outstanding ending again. It reports whether it put a box
// up, so the caller can stop for the frame.
//
// Stopping matters. A scene's Update runs to the end whether or not something
// was pushed onto the stack during it, so a step taken later in the same frame
// could push an encounter *over* the box and have the companion's line arrive
// after a fight that has nothing to do with it.
//
// One box at a time and only over a scene that expects to be interrupted, which
// in practice means the overworld or a location's interior. Everything else
// waits its turn, which is what keeps two companions who both had something to
// say from talking over each other.
func (g *Game) serviceThreads() bool {
	switch g.Top().(type) {
	case *overworldScene, *localScene:
	default:
		return false
	}

	if len(g.pendingBeats) > 0 {
		f := g.pendingBeats[0]
		g.pendingBeats = g.pendingBeats[1:]
		g.Sound.Play("ui/page")
		if !f.Last {
			g.Say(f.Thread.Owner, f.Text)
			return true
		}
		// The last beat is the setup for the choice, so it and the options
		// share a box rather than making the player read the same words twice.
		// Even if the choice cannot be put right now, the beat was consumed and
		// the frame is over.
		g.offerThreadEnding(f.Thread, f.Text)
		return true
	}

	// A player who said "not yet" gets asked again on the next town, and only
	// on the next town: nagging on the road would make "not yet" meaningless.
	if g.remindEndings {
		g.remindEndings = false
		// A saga's choice outranks a companion's for the same reason its legs
		// do, and only one box goes up at a time.
		if g.remindSagaEndings() {
			return true
		}
		return g.remindThreadEndings()
	}
	return false
}

// offerThreadEnding puts the choice at the end of a thread to the player, and
// reports whether there was anything to put.
//
// Backing out is allowed and costs nothing: the thread stays waiting and is put
// again the next time the company walks into a settlement. An ending is the
// only irreversible thing a companion ever asks for, and a modal box that
// cannot be escaped is a poor place to be asked.
func (g *Game) offerThreadEnding(t *thread.Thread, setup string) bool {
	// A resident has no character sheet, so there is nobody to find and the
	// scaling comes off the player instead. Everything else about the box is
	// the same, which is the point of them being one system.
	owner := g.allyNamed(t.Owner)
	if owner == nil && !t.IsResident(&g.Data.Threads) {
		return false
	}
	level := core.Max(1, g.Player.Level)
	if owner != nil {
		level = core.Max(1, owner.Level)
	}
	opts := t.Options(&g.Data.Threads)
	if len(opts) == 0 {
		return false
	}

	// The price goes in the detail column and an ending nobody can pay for is
	// greyed out, rather than being offered and then refused after the fact.
	rows := make([]ui.MenuItem, 0, len(opts)+1)
	for _, e := range opts {
		row := ui.MenuItem{Label: e.Label}
		if cost := e.Costs() * int64(level); cost > 0 {
			row.Detail = fmt.Sprintf("%d coins", cost)
			row.Disabled = g.Player.Coins < cost
		}
		rows = append(rows, row)
	}
	rows = append(rows, ui.MenuItem{Label: "Not yet"})

	if setup == "" {
		setup = t.Fill(t.Title) + "\n\n" + t.Owner + " is still waiting on an answer."
	}
	// And what you have, next to what things cost. Quoting a price on a screen
	// that does not say what is in the purse is asking somebody to remember a
	// number from another room.
	setup += fmt.Sprintf("\n\nYou have %d coins.", g.Player.Coins)

	g.AskMenu(t.Owner, setup, rows, func(g *Game, choice int) {
		if choice < 0 || choice >= len(opts) {
			return // "Not yet", or backed out: ask again in the next town
		}
		g.resolveThread(t, opts[choice], owner)
	})
	return true
}

// resolveThread applies an ending and closes the thread.
//
// owner is nil for a resident's, who has no sheet to adjust and no cut to take.
func (g *Game) resolveThread(t *thread.Thread, e thread.Ending, owner *model.Character) {
	lv := core.Max(1, g.Player.Level)
	if owner != nil {
		lv = core.Max(1, owner.Level)
	}
	level := int64(lv)
	coins := e.Coins * level
	xp := e.XP * level

	// The purse is checked here rather than at the menu because the box was
	// drawn a frame ago and nothing guarantees the coins survived the interval.
	// Every thread is authored with a free ending, so refusing here can never
	// leave the player holding a story they cannot finish.
	if coins < 0 && g.Player.Coins < -coins {
		g.Say(t.Owner, "You do not have it. "+t.Owner+
			" says that is fine in the tone of somebody filing it away.")
		return
	}

	t.Resolve(e)
	g.Player.Coins += coins
	g.Player.TotalXP += xp
	g.Player.SpendXP += xp
	g.Player.Fame += e.Fame
	g.Player.Shame += e.Shame
	g.Player.Honor += e.Honor
	if e.Cut != 0 && owner != nil {
		// Floored at nothing rather than at their starting share: a companion
		// working for free is a thing a story can earn, and one charging the
		// whole haul is not.
		owner.Cut = core.Clamp(owner.Cut+e.Cut, 0, 40)
	}

	g.Sound.Play("world/coins")
	g.SayThen(t.Owner, e.Text, func(g *Game) {
		g.Log.AddColor(render.ColGold, "%s - %s.", t.Title, e.Label)
		switch {
		case coins > 0:
			g.Log.AddColor(render.ColGold, "%d coins.", coins)
		case coins < 0:
			g.Log.AddColor(render.ColInkDim, "It cost %d coins.", -coins)
		}
		if e.Cut != 0 && owner != nil {
			g.Log.AddColor(render.ColInkDim, "%s now takes %d%%.", owner.Name, owner.Cut)
		}
		if xp > 0 {
			g.applyPendingLevels()
		}
	})
}

// allyNamed finds a companion by the name their thread is keyed on.
func (g *Game) allyNamed(name string) *model.Character {
	for _, c := range g.Allies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// threadsOnEnteringPOI fires the location triggers and re-offers any ending the
// player walked away from. A town is where a companion has the nerve to bring
// it up again.
func (g *Game) threadsOnEnteringPOI(idx int) {
	if idx < 0 {
		return
	}
	// Return is the resident's trigger and fires whether or not anybody is
	// walking behind you — that is rather the point of somebody who stays put.
	// Everything else is a companion's and needs one.
	town := g.World.POIs[idx].Kind.Settlement()
	if town {
		g.advanceThreads(thread.Event{Kind: thread.Return, POI: idx})
		// Armed whether or not anybody is walking behind you: a saga's ending
		// is the player's own business and does not need a companion present.
		g.remindEndings = true
	}
	if len(g.Allies) == 0 {
		return
	}
	g.advanceThreads(thread.Event{Kind: thread.Reach, POI: idx})
	if town {
		g.advanceThreads(thread.Event{Kind: thread.Town, POI: idx})
	}
}

// remindThreadEndings puts one outstanding choice again, for a player who said
// "not yet" and then walked into a town.
func (g *Game) remindThreadEndings() bool {
	for _, t := range g.Threads.Waiting() {
		if g.offerThreadEnding(t, "") {
			return true
		}
	}
	return false
}
