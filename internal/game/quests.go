package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// maxActiveQuests caps how much the player can be carrying. A quest log you
// cannot hold in your head is a chore list, not a set of intentions.
const maxActiveQuests = 6

// talkTo runs a conversation with a townsperson: hand in what is finished,
// nag about what is not, otherwise offer something new or just chat.
func (g *Game) talkTo(e *world.Entity) {
	poiIdx := g.currentPOIIndex()
	if poiIdx < 0 {
		g.Say(e.Name, e.Line)
		return
	}
	g.Quests.SyncFetch(g.Player.Bag)

	// The person who asked is the person you report back to.
	//
	// This used to key off the settlement alone, which had two consequences and
	// both were wrong. Any townsperson would accept the hand-in, so the errand
	// never sent you back to anybody. And any townsperson would recite the nag
	// line for an errand they had not given you, so after taking one job the
	// whole town had nothing else to say.
	if q := g.Quests.From(poiIdx, e.Name); q != nil {
		if q.Complete() {
			g.offerTurnIn(e, q)
		} else {
			g.Say(e.Name, q.Nag)
		}
		return
	}

	// One errand per settlement at a time, so a town is a place rather than a
	// queue, and only some people have anything to ask. Everybody else has
	// their own line, which is the whole reason they have one.
	if !g.Quests.HasFrom(poiIdx) && g.Quests.CountActive() < maxActiveQuests &&
		g.wantsToAsk(e, poiIdx) {
		if q, ok := quest.Generate(g.RNG, g.World, g.Data, g.Write, poiIdx, e.Name); ok {
			g.offerQuest(e, q)
			return
		}
	}
	g.Say(e.Name, g.townLine(e))
}

// townLine is what a person in a settlement opens with.
//
// Some of the time it is a reaction to who you are rather than their own line,
// which is the payoff for reputation being two numbers: a stranger telling you
// your own legend without placing you reads completely differently from one
// asking whether you have actually done anything. Not every time — a town where
// every single person comments on you is a town that has stopped being a place
// and started being a mirror.
func (g *Game) townLine(e *world.Entity) string {
	if g.RNG.Chance(0.45) {
		if s := g.Write.StandingLine(g.RNG, rules.Read(g.Player).Key()); s != "" {
			return s
		}
	}
	return e.Line
}

// wantsToAsk decides, stably, whether this particular person has an errand.
// Derived from position rather than stored, like everything else about an
// interior, so the same villager is always the one with the problem.
func (g *Game) wantsToAsk(e *world.Entity, poiIdx int) bool {
	if g.Local == nil {
		return false
	}
	return unitHash(e.Pos.X, e.Pos.Y, g.Local.POI.Seed, 0x9151) < 0.45
}

func (g *Game) offerQuest(e *world.Entity, q *quest.Quest) {
	body := fmt.Sprintf("%s\n\nReward: %d coins, %d experience.", q.Ask, q.RewardCoins, q.RewardXP)
	g.Ask(e.Name, body, []string{"Take the job", "Decline"}, func(g *Game, choice int) {
		if choice != 0 {
			return
		}
		g.Quests.Add(q)
		g.Quests.SyncFetch(g.Player.Bag)
		g.Sound.Play("ui/page")
		g.Log.AddColor(render.ColGold, "Took on: %s", q.Title)
	})
}

func (g *Game) offerTurnIn(e *world.Entity, q *quest.Quest) {
	g.Ask(e.Name, q.Thank+"\n\n"+q.Title+" — complete.",
		[]string{"Collect", "Later"}, func(g *Game, choice int) {
			if choice != 0 {
				return
			}
			g.completeQuest(q)
		})
}

// completeQuest pays out and closes an errand, taking the goods with it.
func (g *Game) completeQuest(q *quest.Quest) {
	// A fetch quest consumes what it asked for; otherwise the same five pelts
	// could settle every debt in the realm.
	if q.Kind == quest.Fetch {
		left := q.Need
		for i := 0; i < len(g.Player.Bag) && left > 0; {
			if g.Player.Bag[i].Name != q.Item {
				i++
				continue
			}
			take := q.Need
			if g.Player.Bag[i].Count < take {
				take = g.Player.Bag[i].Count
			}
			g.Player.Bag[i].Count -= take
			left -= take
			if g.Player.Bag[i].Count <= 0 {
				g.Player.Bag = append(g.Player.Bag[:i], g.Player.Bag[i+1:]...)
				continue
			}
			i++
		}
	}

	g.Player.Coins += q.RewardCoins
	g.Player.TotalXP += q.RewardXP
	g.Player.SpendXP += q.RewardXP
	g.Player.Fame++
	g.Quests.Close(q)

	g.Sound.Play("world/coins")
	g.Log.AddColor(render.ColGold, "%s — done. %d coins, %d experience.",
		q.Title, q.RewardCoins, q.RewardXP)
	g.applyPendingLevels()
	g.Quests.SyncFetch(g.Player.Bag)
}

// currentPOIIndex reports where the player is standing, as a world index.
func (g *Game) currentPOIIndex() int {
	if g.Local == nil {
		return -1
	}
	return g.poiIndex(g.Local.POI)
}

// noteQuestProgress reports newly finished errands to the player. Progress is
// worth interrupting for; the log alone is too quiet.
func (g *Game) noteQuestProgress(done []*quest.Quest) {
	for _, q := range done {
		g.Sound.Play("ui/page")
		g.Log.AddColor(render.ColGold, "%s — conditions met. Go and see %s.", q.Title, q.Giver)
	}
}

// applyPendingLevels banks any level-ups the player has earned. Combat does
// this inline with its own narration; quest rewards need it too, and out of
// combat there is no battle log to write into.
func (g *Game) applyPendingLevels() {
	for rules.PendingLevels(g.Player) > 0 {
		rules.LevelUp(g.RNG, g.Player)
		g.Sound.Play("fight/levelup")
		g.Log.AddColor(render.ColHeal, "%s", g.Write.LevelUpLine(g.RNG, g.Player.Level))
	}
}
