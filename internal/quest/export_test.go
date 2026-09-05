package quest

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// MadeDestination exposes the placement search to the external test package.
//
// Exposed for the reason ClampedMean is in internal/rules: the thing worth
// checking is a decision, and testing a decision only through the thing that
// consumes it means sampling. This one had exactly that problem — the
// assertion that a destination never lands on a location passed with the check
// deleted, because a random tile almost never hits one of forty-five markers.
//
// With the search reachable directly it can be handed a world where every
// square it could possibly return is one Placeable refuses, and then there is
// no sampling left: a correct search finds nowhere and says so, and a search
// that has stopped consulting the rule returns one of them on the first call.
func MadeDestination(g *core.RNG, w *world.Map, home *world.POI) (core.Point, string, bool) {
	return madeDestination(g, w, home)
}

// QuestRange is how far an errand will send you, so a test can build a world
// exactly that big and know it has covered every square the search can reach.
const QuestRange = questRange
