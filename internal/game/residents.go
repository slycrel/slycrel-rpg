package game

import (
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Backstories for the people who stay where they are.
//
// The machinery is the companion system's, unchanged — same skeletons, same
// beats, same authored choice at the end — and the one thing that had to be
// added is an address. A companion's thread is keyed to a name in the party and
// told while it happens, because they are standing next to you. A resident's is
// keyed to a name in a settlement and told in installments on your return,
// because they were not there for any of it.
//
// That difference is the feature. What a person who cannot follow you can offer
// is the shape of a serial: you go away, things happen, you come back, and they
// have the next piece. Advance parks their beats in Owed rather than firing
// them, one at a time, so a month away yields one conversation rather than the
// whole story at once.

// residentCap is how many of these run at once.
//
// Low on purpose. The errand log already competes for the same attention and
// caps at its own small number, and a town where three people are all midway
// through telling you something is a town nobody can keep track of. Two also
// means the second one is usually somewhere else, which is what makes coming
// back to a particular place mean anything.
const residentCap = 2

// hasStory decides, stably, whether this particular townsperson is somebody
// with something going on.
//
// Derived from where they are standing, exactly like wantsToAsk, so the same
// villager is always the one — an interior is regenerated from the location's
// seed on every visit and nothing about a person is stored, so anything that
// varies per visit would make the storyteller a different body each time.
//
// Rarer than an errand. An errand is a transaction and it is fine for a town to
// be full of them; a backstory is somebody deciding you are worth telling, and
// that should not be the first thing every third person does.
func (g *Game) hasStory(e *world.Entity) bool {
	if g.Local == nil {
		return false
	}
	// A different salt from wantsToAsk's, or the two would be the same roll
	// wearing different thresholds and the person with an errand would always
	// be the person with a story.
	return unitHash(e.Pos.X, e.Pos.Y, g.Local.POI.Seed, 0x51DE) < 0.22
}

// residentThread returns the story this person is in the middle of telling, if
// there is one, and otherwise casts one if they are the sort of person who has
// one and there is room for it.
func (g *Game) residentThread(e *world.Entity, poiIdx int) *thread.Thread {
	if g.World == nil || g.Data == nil || poiIdx < 0 {
		return nil
	}
	if t := g.Threads.ForResident(&g.Data.Threads, poiIdx, e.Name); t != nil {
		return t
	}
	if !g.hasStory(e) || g.runningResidents() >= residentCap {
		return nil
	}
	// One per settlement, so a town is a place rather than a queue — the same
	// rule the errand log follows, and for the same reason.
	for _, t := range g.Threads.Threads {
		if t.HomePOI == poiIdx && t.IsResident(&g.Data.Threads) {
			return nil
		}
	}

	// Forked off the run's seed and the person, not off the live stream, so
	// striking up a conversation cannot shift an encounter roll — and so the
	// same person in the same town on the same seed is always cast in the same
	// story. Fork never reads its receiver, which is why both have to be in the
	// salt.
	rng := g.RNG.Fork("resident", g.Seed^nameSalt(e.Name)^int64(poiIdx))
	t, ok := thread.CastResident(rng, &g.Data.Threads, g.World, g.Data,
		e.Name, poiIdx, g.Player.Level, g.Walk.Tile, g.Threads.IDs())
	if !ok {
		return nil
	}
	g.Threads.Add(t)
	g.Log.AddColor(render.ColGold, "%s has something going on: %s", e.Name, t.Fill(t.Title))
	return t
}

// runningResidents counts the resident stories still open.
func (g *Game) runningResidents() int {
	n := 0
	for _, t := range g.Threads.Threads {
		if t.State != thread.Closed && t.IsResident(&g.Data.Threads) {
			n++
		}
	}
	return n
}

// talkToResident handles a conversation with somebody in the middle of their
// own story, and reports whether it took the conversation over.
//
// Three states, in the order they matter. They are waiting on an answer, so
// put the choice. They have been holding an installment, so hand it over. Or
// they are waiting on you to go and do something, in which case they say the
// journal note out loud — because a person who has told you they need something
// and then reverts to their idle line reads as having forgotten, and that was
// the single most common complaint about the errand givers before they learned
// to nag.
func (g *Game) talkToResident(e *world.Entity, poiIdx int) bool {
	t := g.residentThread(e, poiIdx)
	if t == nil {
		return false
	}

	// Something owed comes first even when the thread is already Ready: the
	// last beat is the setup for the choice, and offering the choice without it
	// would ask the player to decide something nobody has finished saying.
	if owed := t.Say(); owed != "" {
		g.Sound.Play("ui/page")
		if t.State == thread.Ready {
			g.offerThreadEnding(t, owed)
		} else {
			g.SayAs(t.Owner, g.roleOf(e), g.faceOf(e), owed)
			g.showThreadDestination(t)
		}
		return true
	}
	if t.State == thread.Ready {
		g.offerThreadEnding(t, "")
		return true
	}
	if t.State == thread.Closed {
		return false // their story is over; they are a townsperson again
	}

	note := t.Note(&g.Data.Threads)
	if note == "" {
		return false
	}
	if p := t.Progress(&g.Data.Threads); p != "" {
		note += "  (" + p + ")"
	}
	g.SayAs(t.Owner, g.roleOf(e), g.faceOf(e), note)
	return true
}

// residentJournalLine is one open resident story, for the log screen: the title
// and where to find the person telling it.
func (g *Game) residentJournalLine(t *thread.Thread) string {
	where := "somewhere"
	if g.World != nil && t.HomePOI >= 0 && t.HomePOI < len(g.World.POIs) {
		where = g.World.POIs[t.HomePOI].Name
	}
	return t.Owner + ", in " + where
}
