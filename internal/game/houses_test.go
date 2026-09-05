package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Walking into a stranger's house does three different things, and which one is
// read off the player rather than rolled.
//
// This is the first place reputation *costs or pays* rather than tinting a line
// of dialogue and a price tag, so the thing worth guarding is that the branch
// is actually taken — and that it is taken once. A purse that comes out of a
// drawer every time you re-open the door is an income, and a dog that can be
// set on you twice is experience on tap.
func TestAHouseholdReadsYouBeforeItDecidesWhatToDo(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fame     int
		shame    int
		renown   int
		want     rules.Standing
		paysOnce bool
		fights   bool
	}{
		{name: "a nobody", want: rules.Unknown},
		{name: "a rumour", fame: 9, want: rules.Rumoured},
		{name: "a face", renown: 9, want: rules.Recognised},
		{name: "a hero", fame: 9, renown: 9, want: rules.Celebrated, paysOnce: true},
		{name: "a villain", fame: 2, shame: 9, want: rules.Notorious, fights: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, resident := gameInAHouse(t)
			g.Player.Fame, g.Player.Shame, g.Player.Renown = tc.fame, tc.shame, tc.renown
			if got := rules.Read(g.Player); got != tc.want {
				t.Fatalf("the fixture reads as %v, not %v, so this proves nothing",
					got.Name(), tc.want.Name())
			}

			purse := g.Player.Coins
			g.knock(resident)
			if paid := g.Player.Coins > purse; paid != tc.paysOnce {
				t.Errorf("%v was paid=%v, want %v", tc.want.Name(), paid, tc.paysOnce)
			}
			if fought := dismiss(g); fought != tc.fights {
				t.Errorf("%v was set upon=%v, want %v", tc.want.Name(), fought, tc.fights)
			}

			// And once. The second knock is a conversation whoever you are.
			g.dropOverlays()
			purse = g.Player.Coins
			g.knock(resident)
			if g.Player.Coins != purse {
				t.Errorf("%v was paid a second time for the same door", tc.want.Name())
			}
			if dismiss(g) {
				t.Errorf("%v was set upon a second time in the same house", tc.want.Name())
			}
		})
	}
}

// dismiss closes the message box a knock produced and reports whether a fight
// was waiting behind it.
//
// The box comes first on purpose — SayAsThen exists so that one event can put
// two screens up in the order the events happened, and a fight that arrived
// before the shout would be a dog with no owner.
func dismiss(g *Game) bool {
	g.demoChoose(0)
	_, fighting := g.Top().(*battleScene)
	return fighting
}

// gameInAHouse puts the party inside somebody's front room, the way walking
// through the door does.
func gameInAHouse(t *testing.T) (*Game, *world.Entity) {
	t.Helper()
	g := storyGame(t)
	poi := townWithAnInn(t, g)
	g.townPOI = poi
	g.Local = world.BuildLocal(poi, g.Write, 0)

	var door *world.Entity
	for _, e := range g.Local.Entities {
		if e.Kind == world.EHouseDoor {
			door = e
			break
		}
	}
	if door == nil {
		t.Fatal("the town generated with no houses in it")
	}
	g.enterRoom(door)
	resident := firstOfKind(g.Local, world.EResident)
	if resident == nil {
		t.Fatal("nobody lives here")
	}
	return g, resident
}
