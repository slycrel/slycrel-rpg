package game

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/quest"
)

func escortQuest(name string) *quest.Quest {
	return &quest.Quest{
		ID: "escort-1", Kind: quest.Escort, State: quest.Active,
		Escortee: name, TargetName: "the old ford", Need: 1,
	}
}

// Somebody being walked somewhere walks behind you, and is not in the company.
//
// Both halves. The line has to grow or they are invisible; the company must not,
// or they are a free hireling and they turn up in the battle panel — which is
// the thing that cannot happen, because the panel is 156 pixels and three rows
// of about fifty is all it holds.
func TestAnEscorteeFollowsButDoesNotJoin(t *testing.T) {
	g := storyGame(t)
	before := len(g.Party())

	g.Quests.Add(escortQuest("Marda Quist"))
	g.reformLines()

	e := g.escorting()
	if e == nil {
		t.Fatal("an active escort put nobody behind the player")
	}
	if e.Name != "Marda Quist" {
		t.Errorf("the person walking behind is %q", e.Name)
	}
	if got := len(g.follow); got != len(g.Allies)+1 {
		t.Errorf("the marching line is %d long for %d companions and one charge", got, len(g.Allies))
	}
	if got := len(g.Party()); got != before {
		t.Errorf("the company went from %d to %d; an escortee is not a party member", before, got)
	}
}

// Handing them over ends it, and the line closes up.
func TestArrivingSendsTheEscorteeOnTheirWay(t *testing.T) {
	g := storyGame(t)
	q := escortQuest("Marda Quist")
	g.Quests.Add(q)
	g.reformLines()
	if len(g.follow) != len(g.Allies)+1 {
		t.Fatal("nobody was following to begin with")
	}

	q.Have = q.Need
	g.reformLines()
	if g.escorting() != nil {
		t.Error("somebody is still being escorted after the errand was finished")
	}
	if got := len(g.follow); got != len(g.Allies) {
		t.Errorf("the line is still %d long for %d companions", got, len(g.Allies))
	}
}

// A deadline closes the errand by walking, and says so.
//
// On the step rather than on arrival, because a deadline only noticed when you
// reach the place is not a deadline — it is a surprise at the end of a walk you
// had already finished.
func TestARunOutClockSendsThemHome(t *testing.T) {
	g := storyGame(t)
	q := escortQuest("Marda Quist")
	q.Due = 10
	g.Quests.Add(q)

	g.Clock.Step = 5
	g.expireEscorts()
	if len(g.Quests.Active()) != 1 {
		t.Fatal("the errand closed before its deadline")
	}

	g.Clock.Step = 11
	g.expireEscorts()
	if len(g.Quests.Active()) != 0 {
		t.Error("the errand outlived its deadline")
	}
	if g.escorting() != nil {
		t.Error("somebody is still following after their errand ran out")
	}
}

// The ones who help do it off the panel, and only sometimes.
//
// The rate matters as much as the fact: something that fired every round would
// be a fourth party member with no meters, which is the arrangement this whole
// design exists to avoid.
func TestOnlyTheHelpfulSortJoinsIn(t *testing.T) {
	g := storyGame(t)
	g.RNG = core.NewRNG(4)

	shy := escortQuest("Marda Quist")
	g.Quests.Add(shy)
	for i := 0; i < 400; i++ {
		if _, _, ok := escortHelp(g); ok {
			t.Fatal("somebody who does not help, helped")
		}
	}

	shy.Helps = true
	hits, rounds := 0, 4000
	for i := 0; i < rounds; i++ {
		if _, dmg, ok := escortHelp(g); ok {
			hits++
			if dmg <= 0 {
				t.Fatal("a helping hand landed for nothing")
			}
		}
	}
	if hits == 0 {
		t.Fatal("the helpful sort never helped")
	}
	if share := float64(hits) / float64(rounds); share > 0.45 {
		t.Errorf("they join in %.0f%% of rounds, which is a party member without a health bar", share*100)
	}
}
