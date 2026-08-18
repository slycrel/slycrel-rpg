package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/ui"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// tithe is what the plate expects. Flat rather than scaled: the point of a
// shrine is that it charges a peasant and a warlord the same, and at level
// twelve the coins are not what you are spending anyway.
const tithe = 25

// offerAltar runs a shrine.
//
// One visit, one thing, and the altar is spent afterwards — which is why it
// goes through g.spend rather than setting Used on the live entity alone. The
// old version did the latter, so the altar came back every time the interior
// regenerated: a shrine you could walk out of and back into was an unlimited
// full heal for 25 coins, and now that faith buys something it would have been
// an unlimited supply of that too.
//
// The two things it does are deliberately not stacked into one row. Praying
// puts the company back on its feet and banks a point of faith; confessing
// spends that faith to make the world stop being able to place you. Both are
// worth the walk, and having to pick is what makes the second one cost
// anything.
func (g *Game) offerAltar(e *world.Entity) {
	p := g.Player
	lift := rules.Penance(p.Faith, p.Shame)

	// Why a greyed row is greyed, in the column that greys it. "Confess" with
	// no explanation reads as a bug; "nothing to lift" reads as good news.
	confessDetail := fmt.Sprintf("%d faith", lift)
	switch {
	case p.Shame <= 0:
		confessDetail = "nothing to lift"
	case p.Faith <= 0:
		confessDetail = "no faith"
	}

	// The standing goes in the body because this is the only counter in the
	// game that sells a different one, and a player deciding whether to confess
	// should not have to remember what they are currently called.
	body := fmt.Sprintf("%s\n\nThe offering plate is right there. It is a large plate. "+
		"You have %d coins, %d faith, and %d shame, and out there you are %s.",
		e.Line, p.Coins, p.Faith, p.Shame, rules.Read(p).Name())

	g.AskMenu(e.Name, body, []ui.MenuItem{
		{Label: "Pray", Detail: fmt.Sprintf("%d coins", tithe),
			Disabled: p.Coins < tithe},
		{Label: "Confess", Detail: confessDetail, Disabled: lift <= 0},
		{Label: "Leave it alone"},
	}, func(g *Game, choice int) {
		switch choice {
		case 0:
			// Re-checked because the box was drawn a frame ago and the purse is
			// not guaranteed to have survived the interval.
			if g.Player.Coins < tithe {
				return
			}
			g.Player.Coins -= tithe
			g.spend(e)
			g.restParty()
			// Somewhere safe that put the company back on its feet, which is
			// the same thing a bed is. An altar is scattered and one-shot, so
			// this is a checkpoint you find rather than one you buy.
			g.autosave()
			g.Player.Faith++
			g.Say("", "Something old and largely retired takes an interest. "+
				"You are made whole, and faintly indebted.")

		case 1:
			n := rules.Confess(g.Player)
			if n <= 0 {
				return
			}
			g.spend(e)
			g.Sound.Play("world/enter")
			// Say what it took as well as what it lifted. Renown is the price
			// and it is not obvious from the outside — a player who confessed
			// their way out of notoriety and then found the hiring board more
			// expensive deserves to have been told why.
			g.Say("", fmt.Sprintf(
				"You say it out loud in a room built for it. Something writes it down and closes the book.\n\n"+
					"%d shame lifted. Nobody can place you either, which was the price.",
				n))
		}
	})
}
