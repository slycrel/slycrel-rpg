package game

import (
	"fmt"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/talk"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// Talking to the people you hired.
//
// Until now a companion was somebody you paid for, equipped, and read a
// paragraph out of every forty minutes when a beat came due. There was no way
// to start a conversation — the only mouth in the game that opened on purpose
// belonged to somebody standing still in a town — and the one thing a player
// most obviously wants from a person who said something cryptic four minutes
// ago is to be able to ask them about it.
//
// The tree is authored (data/text/talk.json) and what is *true* is answered
// here. That split is the whole design and it is the same one threads use: the
// writing does not know what a thread is and this file does not know what
// anybody says.

// converse opens a conversation with a companion at a given node.
//
// It recurses through the message box's callback rather than looping, because
// the box is a scene and a scene runs for as many frames as the player looks at
// it. Each answer pushes the next node; the option with no destination pushes
// nothing, which is how a conversation ends.
func (g *Game) converse(c *model.Character, tr *talk.Tree, id string) {
	n, ok := tr.Node(id)
	if !ok || len(n.Text) == 0 {
		return
	}
	body := g.fillTalk(c, core.Pick(g.RNG, voiceOf(c, n)))

	opts := g.openTo(c, n.Options)
	if len(opts) == 0 {
		g.SayAs(c.Name, companionRole(c), c.Portrait, body)
		return
	}
	items := make([]ui.MenuItem, len(opts))
	for i, o := range opts {
		items[i] = ui.MenuItem{Label: o.Label}
	}
	g.AskAs(c.Name, companionRole(c), c.Portrait, body, items, func(g *Game, choice int) {
		// Backing out of a conversation is leaving it, at any depth. A cancel
		// that walked one node back up would need a history nothing else in the
		// game keeps, and "never mind" is already on the menu as an answer.
		if choice < 0 || choice >= len(opts) {
			return
		}
		if to := opts[choice].Goto; to != "" {
			g.converse(c, tr, to)
		}
	})
}

// voiceOf is the version of a line this particular person would say.
//
// Lineage before class, because the blood is the more distinctive fact and it
// is what the sales pitch already leads with — a part-undead thief and a
// part-undead fighter have more in common in a conversation than two thieves
// do. Falling through to Text is the ordinary case: most people are just
// people, and there is nothing to say about that in a special voice.
func voiceOf(c *model.Character, n *talk.Node) []string {
	if v := n.Voices[string(c.Blood)]; len(v) > 0 {
		return v
	}
	if v := n.Voices[string(c.Class)]; len(v) > 0 {
		return v
	}
	return n.Text
}

// openTo is the options this person can currently be asked, in the order they
// were written.
func (g *Game) openTo(c *model.Character, all []talk.Option) []talk.Option {
	var out []talk.Option
	for _, o := range all {
		if g.needMet(c, o.Need) {
			out = append(out, o)
		}
	}
	return out
}

// needMet answers one of the conversation vocabulary's conditions.
//
// Every branch is a lookup or a comparison against something the game already
// tracks. Nothing here may have side effects and nothing may roll: this runs
// once per option every time a box opens, and a condition that rolled would
// make an option flicker in and out of a menu the player is looking at.
func (g *Game) needMet(c *model.Character, need talk.Need) bool {
	t := g.threadOf(c)
	switch need {
	case talk.Always:
		return true
	case talk.Story:
		return t != nil && t.Told(&g.Data.Threads) != ""
	case talk.Waiting:
		return t != nil && t.State == thread.Open && t.Note(&g.Data.Threads) != ""
	case talk.Ending:
		return t != nil && t.State == thread.Ready
	case talk.NoStory:
		return t == nil
	case talk.Hurt:
		return c.MaxHP > 0 && c.HP*2 < c.MaxHP
	case talk.Blood:
		return c.Blood != ""
	case talk.Errand:
		_, on := g.tracked()
		return on
	case talk.Rich:
		return g.Player != nil && g.Player.Coins >= richPurse
	case talk.Broke:
		return g.Player != nil && g.Player.Coins < brokePurse
	}
	return false
}

// What counts as money, in coins.
//
// Both are read off what things cost rather than picked: a night at an inn and
// a starter weapon are the two purchases a player makes before they have
// anything, so "broke" is not being able to sleep and "rich" is being able to
// stop thinking about either. Nothing hangs off these but which line somebody
// says, which is why they are allowed to be this rough.
const (
	brokePurse = 40
	richPurse  = 600
)

// threadOf is a companion's backstory, or nil.
func (g *Game) threadOf(c *model.Character) *thread.Thread {
	if g.Data == nil || c == nil {
		return nil
	}
	return g.Threads.For(&g.Data.Threads, c.Name)
}

// fillTalk substitutes what a line is allowed to know.
//
// The tokens are the same shape threads use and the list is just as short. A
// conversation may name the person, their employer, their cut, what their story
// is waiting on and what it last said — and that is all, because every token
// here is a promise that the thing it names exists, and the conditions above
// are what keep that promise. {S} only ever appears under a Story need and {W}
// only under Waiting or Ending.
func (g *Game) fillTalk(c *model.Character, s string) string {
	you := "you"
	if g.Player != nil {
		you = g.Player.Name
	}
	told, waiting := "", ""
	if t := g.threadOf(c); t != nil {
		told, waiting = t.Told(&g.Data.Threads), t.Note(&g.Data.Threads)
	}
	return strings.NewReplacer(
		"{N}", c.Name,
		"{Y}", you,
		"{C}", fmt.Sprintf("%d%%", c.Cut),
		"{S}", told,
		"{W}", waiting,
	).Replace(s)
}

// companionRole is the caption under a companion's face: what they are, in the
// two or three words the portrait column has room for.
func companionRole(c *model.Character) string {
	trade := strings.ToLower(string(c.Class))
	if c.Blood != "" {
		return string(c.Blood) + " " + trade
	}
	return trade
}

// talkToCompany opens a conversation with somebody you are travelling with.
//
// One companion goes straight through; two get asked which. The menu is not
// skipped for two the way it is for one, because a party of two is exactly when
// picking the wrong one matters — the whole point is that they are different
// people.
func (g *Game) talkToCompany() {
	tr, ok := g.Data.Talk.Tree("companion")
	if !ok {
		return
	}
	switch len(g.Allies) {
	case 0:
		return
	case 1:
		g.converse(g.Allies[0], tr, tr.Root)
		return
	}
	items := make([]ui.MenuItem, len(g.Allies))
	for i, c := range g.Allies {
		items[i] = ui.MenuItem{Label: c.Name, Detail: companionRole(c)}
	}
	g.AskMenu("Your company", "Who?", items, func(g *Game, choice int) {
		if choice < 0 || choice >= len(g.Allies) {
			return
		}
		g.converse(g.Allies[choice], tr, tr.Root)
	})
}
