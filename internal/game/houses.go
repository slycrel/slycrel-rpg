package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// What a household does when you walk in on it.
//
// Which of the three things happens is not a roll. rules.Read turns Fame,
// Shame and Renown into how a town takes somebody, and until there were doors
// to open the only things that ever came of it were a line of dialogue and a
// markup at a counter — both of which happen whether the player notices or not.
// A room with one person in it is somewhere the reading can pay or cost.
//
// Celebrated is thanked, Notorious is set upon, and everybody else has a
// conversation. The two states in between get the conversation on purpose:
// Rumoured is somebody whose deeds travel without their face, and Recognised is
// the reverse, and neither is a person a householder standing in their own
// kitchen would know to thank or to be afraid of. That is the whole point of
// reputation being two numbers.

// What a household remembers, filed against the town under names no entity
// has.
//
// MarkUsed keys on a string and the replay in BuildLocal only ever matches an
// entity's real kind, so a mark under a name of its own is a fact about the
// square rather than a thing taken off it. Which is what these are: spending
// the resident would delete them, and somebody who has given you a purse is
// still standing in their own house afterwards with something to say.
const (
	markGift   = "gift"
	markGrudge = "grudge"
)

// knock is the interaction with somebody at home.
func (g *Game) knock(e *world.Entity) {
	switch rules.Read(g.Player) {
	case rules.Celebrated:
		if g.firstTime(markGift, e) {
			g.thanked(e)
			return
		}
	case rules.Notorious:
		if g.firstTime(markGrudge, e) {
			g.setUpon(e)
			return
		}
	}
	g.SayAs(e.Name, g.roleOf(e), g.faceOf(e), g.townLine(e))
}

// firstTime reports whether a household has yet to do something, and records
// that it has by saying so. Once each: a purse that came out of a drawer is not
// in the drawer any more, and a dog that has already been set on you has made
// its point.
func (g *Game) firstTime(what string, e *world.Entity) bool {
	p := g.here()
	if p == nil {
		return false
	}
	if p.IsUsed(what, e.Pos, g.floor) {
		return false
	}
	p.MarkUsed(what, e.Pos, g.floor)
	return true
}

// thanked is what somebody does about a hero standing in their front room.
//
// About half a chest, scaled the same way, because the point is that a town
// that knows you is worth walking around rather than that houses are a income.
// A player who has earned Celebrated has earned it across a whole run.
func (g *Game) thanked(e *world.Entity) {
	coins := int64(g.RNG.Between(4, 12) * core.Max(1, g.Local.POI.Level))
	g.Player.Coins += coins
	g.Sound.Play("world/chest")
	g.SayAs(e.Name, g.roleOf(e), g.faceOf(e), fmt.Sprintf("%s\n\n%d coins, and no arguing about it.",
		core.Pick(g.RNG, houseThanks), coins))
}

// setUpon is what somebody does about the other sort of reputation.
//
// What turns up is drawn from the same table as everything else outdoors here,
// which for a settlement is the plains roster: livestock and a freelance tax
// collector. That is the right list. Nobody in a village keeps a demon, and the
// thing a householder sets on somebody they have heard the wrong stories about
// is the dog.
func (g *Game) setUpon(e *world.Entity) {
	enc := g.Data.PickEncounter(g.RNG, g.Local.Biome, g.Local.POI.Level, g.encounterSize(1))
	if len(enc.Monsters) == 0 {
		// Nothing in the roster for this band, which is not a reason to say
		// nothing. They are still not pleased to see you.
		g.SayAs(e.Name, g.roleOf(e), g.faceOf(e), core.Pick(g.RNG, houseTrouble))
		return
	}
	where := g.Local.POI.Name
	g.SayAsThen(e.Name, g.roleOf(e), g.faceOf(e), core.Pick(g.RNG, houseTrouble),
		func(g *Game) { g.Push(newBattleScene(g, enc, where)) })
}

// houseThanks is a person recognising somebody in their own doorway. They are
// all mid-gesture, because a reward you are handed by somebody already halfway
// through giving it is a reaction and a reward you are offered is a menu.
var houseThanks = []string{
	"They know the face. They are already going through a drawer, and they have not asked whether you want it.",
	"\"You did the -- you did, didn't you. Sit down. No. You're busy. Take this and go and be busy.\"",
	"They put down what they were holding, cross the room, and press something into your hand before you can get a word out.",
	"\"My neighbour said you'd come through here one day. My neighbour is going to be insufferable about this.\"",
	"\"We don't have much. We've got this. Don't make a thing of it.\"",
}

// houseTrouble is the other half. Nobody in here says what they heard, because
// the player knows what they did and a townsperson reciting it back would be
// the game marking its own homework.
var houseTrouble = []string{
	"\"Out. OUT. I know exactly who you are and I want you nowhere near my kitchen.\"",
	"They do not shout. They walk to the back door, open it, and whistle.",
	"\"We heard. We ALL heard. You've got until I finish this sentence.\"",
	"\"No. Not in here. Not after that.\" Something in the yard is already awake.",
}
