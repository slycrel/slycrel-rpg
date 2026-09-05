package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// Honor and Faith are the ledger you spend. Fame, Renown and Shame are the
// ledger the world reads — see standing.go — and the split is the point.
//
// A read number is something other people do to you: you cannot pay it down,
// you can only outweigh it. A spent number is something you banked on purpose,
// and the only interesting question about it is what you traded it for. Four
// numbers on a character sheet is three too many if they all do the same job;
// two ledgers with two jobs is a reason for each of them to exist.
//
// Both are ignorable. Neither is checked at a gate, neither locks anything, and
// a player who never tithes and never sees a companion's story out plays the
// whole game. What they lose is the two places where a run gets to be cheaper
// or quieter than it would otherwise have been, and those get more expensive to
// go without the longer the run lasts.

// Honor is what you did when it cost you: seeing a companion's story through to
// an ending that paid you nothing, settling somebody's debt out of your own
// purse. It is spent at the hiring board, where what the next one asks for
// depends on what the last one told people.
//
// It moves the ongoing cut rather than the fee up front, which is the whole
// reason it does not double up with Standing. What a mercenary charges to walk
// out of the gate with you is a question about who you are — hazard pay for the
// notorious, a discount for the celebrated. What they want off every haul
// afterwards is a question about whether you will still be there at the end of
// it, and those have different answers.

// The relief honour buys is deliberately small and hard-capped. A cut is a
// percentage of everything for the rest of the run, so four points of it
// compounds into real money; more than that and hiring stops being a decision.
const (
	maxRelief = 4  // most honour can take off a cut, in percentage points
	minCut    = 3  // nobody works for nothing, however well you are thought of
	maxCut    = 30 // and nobody signs away a third of the haul
)

// AskingCut adjusts a rolled cut for the employer's reputation for keeping the
// people they hire.
//
// Honour halves into percentage points because it arrives roughly one at a
// time and a cut is rolled in a ten-point band: a point of honour moving a
// point of cut would let two finished backstories wipe out the roll entirely.
// Negative honour raises the cut by the same arithmetic, and it should — the
// last three people you took on are all still in that town.
func AskingCut(rolled, honor int) int {
	return core.Clamp(rolled-core.Clamp(honor/2, -maxRelief, maxRelief), minCut, maxCut)
}

// Faith is what you put in the plate, and it comes back as the one thing money
// cannot buy: being forgotten about.

// maxPenance caps what a single altar will lift. Altars are scattered and each
// one only works once, so without a cap a player holding enough faith could
// launder an entire run's worth of shame at the first shrine they walked into.
// Three is enough to climb out of notoriety in one visit and not enough to do
// it without having tithed for it first.
const maxPenance = 3

// Penance is what one confession lifts: points of Shame, and the same number
// of points of Renown, paid for with the same number of points of Faith.
//
// Renown is the price rather than Fame, and that is the whole design. Lifting
// Shame and Fame together would be free and useless — Read weighs one against
// the other, so a player who lost both would stand exactly where they started,
// which is a button that does nothing dressed as a sacrament. Taking Renown
// instead means the deeds survive and the face stops being known: what penance
// actually sells is anonymity. You walk out of a shrine having done everything
// you did, with nobody able to place you.
//
// That is a real trade in both directions. It is the way out of Notorious, and
// it costs the Renown that Celebrated is made of.
func Penance(faith, shame int) int {
	return core.Clamp(core.Min(faith, shame), 0, maxPenance)
}

// Confess applies a penance to a character, and reports what it lifted.
//
// Everything clamps at nothing rather than trusting the caller: this is
// reachable from a menu that was drawn a frame ago, and a shrine that ran a
// character's Renown negative would be a bug nobody found until the sheet
// looked wrong four towns later.
func Confess(c *model.Character) int {
	n := Penance(c.Faith, c.Shame)
	if n <= 0 {
		return 0
	}
	c.Faith = core.Max(0, c.Faith-n)
	c.Shame = core.Max(0, c.Shame-n)
	c.Renown = core.Max(0, c.Renown-n)
	return n
}

// The band a hireling's cut is rolled in, and a way to roll it from a name.
//
// It is a band rather than a number so that two people outside the same inn are
// not the same offer, and it is reachable from a name so that the offer can be
// *quoted before it is accepted*. It used to be rolled inside Recruit, which
// happens after the money changes hands — so the pitch said "a cut of the coin
// after" and the actual figure arrived in the line confirming the hire. That is
// a price told to you once you have paid it.
const (
	cutLow  = 8
	cutHigh = 18
)

// RolledCut is what this person asks a stranger, before reputation moves it.
func RolledCut(g *core.RNG) int { return g.Between(cutLow, cutHigh) }

// CutFromName is RolledCut for somebody who has not been hired yet.
//
// Keyed on the name so the number is the same every time the offer is opened
// and the same when it is taken — a price that changed between reading it and
// agreeing to it would be worse than not quoting one. The hash is the same
// shape the portrait pool uses to keep a face stable for the same reason.
func CutFromName(name string) int {
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return cutLow + int(h%uint32(cutHigh-cutLow+1))
}
