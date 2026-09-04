package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/ui"
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
		g.SayAs(e.Name, g.roleOf(e), g.faceOf(e), e.Line)
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
			g.SayAs(e.Name, g.roleOf(e), g.faceOf(e), q.NagLine())
		}
		return
	}

	// Somebody in the middle of their own story, which takes precedence over
	// being handed a new errand: a person who has started telling you something
	// and then offers you a job instead has stopped being a person.
	if g.talkToResident(e, poiIdx) {
		return
	}

	// One errand per settlement at a time, so a town is a place rather than a
	// queue, and only some people have anything to ask. Everybody else has
	// their own line, which is the whole reason they have one.
	if !g.Quests.HasFrom(poiIdx) && g.Quests.CountActive() < maxActiveQuests &&
		g.wantsToAsk(e, poiIdx) {
		// The kind this person was always going to ask for, so their face is
		// the same before, during and after the errand.
		prefer, _ := g.questFaceKind(e)
		if q, ok := quest.Generate(g.RNG, g.World, g.Data, g.Write, poiIdx, e.Name, prefer); ok {
			g.offerQuest(e, q)
			return
		}
	}
	g.SayAs(e.Name, g.roleOf(e), g.faceOf(e), g.townLine(e))
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
	// Something true about the run comes first, when there is something true
	// and the roll allows it. See adviceKey for what counts.
	if g.RNG.Chance(adviceChance) {
		if s := g.Write.Advice(g.RNG, g.adviceKey()); s != "" {
			return s
		}
	}
	if g.RNG.Chance(0.45) {
		if s := g.Write.StandingLine(g.RNG, rules.Read(g.Player).Key()); s != "" {
			return s
		}
	}
	return e.Line
}

// adviceChance is how often a villager with something useful to say says it.
//
// A third, for the same reason the standing lines are not every time: a town
// where everybody tells you what to do next has stopped being a place and
// started being a tutorial with legs. Two in three conversations are still the
// person's own line, which is the whole reason they were written one each.
const adviceChance = 0.33

// adviceKey names the most pressing thing about the run, or the empty string
// when nothing stands out.
//
// Ordered, and the order is the point: whichever comes back is the only one
// anybody will mention, so the first match has to be the thing actually worth
// saying. Money problems come before anything that costs money, because a
// stranger telling a broke player to go and buy a bed is the same failure as
// the shop offering a sword nobody can afford — the game holding out something
// it is about to take away. "Never offer a choice you are about to refuse"
// applies to advice as much as to menus.
func (g *Game) adviceKey() string {
	if g.Player == nil {
		return ""
	}
	party := g.Party()
	bed := innCost(g.Player.Level, len(party))
	hire := rules.HireCost(core.Max(1, g.Player.Level), "", rules.Read(g.Player))

	switch {
	case g.Player.Coins < bed:
		return "broke"
	case partyFrac(party) < hurtBelow:
		return "hurt"
	// Only when it is a thing they could actually do. Somebody who cannot
	// afford company does not need to be told company exists; they need the
	// line above, and they already got it.
	case len(g.Allies) == 0 && g.Player.Coins >= hire:
		return "alone"
	case g.Player.Coins >= hire*flushMultiple:
		return "flush"
	case g.Quests.CountActive() == 0:
		return "idle"
	}
	return ""
}

const (
	// hurtBelow is the share of the party's hit points under which somebody
	// will mention the inn. Not a sliver: a party at two thirds is a party
	// mid-adventure, and being told to go to bed is not advice, it is nagging.
	hurtBelow = 0.45
	// flushMultiple is how many hirelings' worth of unspent coin counts as
	// walking around rich enough for a stranger to notice.
	flushMultiple = 3
)

// innCost is what a night costs, mirrored from the inn so the advice cannot
// quote a threshold the landlord disagrees with.
func innCost(level, beds int) int64 {
	return int64((10 + level*4) * core.Max(1, beds))
}

// partyFrac is the party's hit points as a share of what they would be at full.
// Everybody together rather than the hero alone, because a companion at a
// tenth is exactly the state somebody should be told to sleep off.
func partyFrac(party []*model.Character) float64 {
	var have, max int
	for _, c := range party {
		have += core.Max(0, c.HP)
		max += c.MaxHP
	}
	if max <= 0 {
		return 1
	}
	return float64(have) / float64(max)
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
	g.AskAs(e.Name, g.roleOf(e), g.faceOf(e), body, []ui.MenuItem{
		{Label: "Take the job"}, {Label: "Decline"},
	}, func(g *Game, choice int) {
		if choice != 0 {
			return
		}
		g.Quests.Add(q)
		g.Quests.SyncFetch(g.Player.Bag)
		g.Sound.Play("ui/page")
		g.Log.AddColor(render.ColGold, "Took on: %s", q.Title)
		// Point at it, if it points anywhere and nothing else is being
		// followed. An errand that names a ruin is an errand about walking to
		// a ruin, and the player has just agreed to do that.
		if idx, label, ok := g.destinationOf(q); ok {
			g.trackIfIdle(idx, label)
		}
	})
}

func (g *Game) offerTurnIn(e *world.Entity, q *quest.Quest) {
	g.AskAs(e.Name, g.roleOf(e), g.faceOf(e), q.Thank+"\n\n"+q.Title+" — complete.",
		[]ui.MenuItem{{Label: "Collect"}, {Label: "Later"}}, func(g *Game, choice int) {
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
