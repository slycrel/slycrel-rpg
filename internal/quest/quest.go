// Package quest generates and tracks the errands people ask you to run.
//
// Quests are generated from the world rather than authored against it: the
// continent already has settlements, dungeons, biomes with monsters in them and
// monsters with drop tables, which is everything an errand needs to point at.
// A quest is therefore a few indices and a counter, and it survives a save as
// exactly that.
//
// The generator only ever points at things it has checked exist — a fetch quest
// names an item something nearby actually drops, a delve names a dungeon that
// is really there and not yet cleared. A quest that cannot be completed is
// worse than no quest at all.
package quest

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Kind is what the errand actually asks of you.
type Kind string

const (
	// Fetch: bring back N of something monsters drop.
	Fetch Kind = "fetch"
	// Cull: kill N of a particular creature.
	Cull Kind = "cull"
	// Delve: clear a named dungeon or cave.
	Delve Kind = "delve"
	// Deliver: carry something to another settlement.
	Deliver Kind = "deliver"
)

// State is where a quest is in its life.
type State string

const (
	Active State = "active"
	Done   State = "done" // conditions met, not yet handed in
	Closed State = "closed"
)

// Quest is one errand. Everything referring to the world is an index or a name,
// so the whole thing serialises to a few lines of JSON.
type Quest struct {
	ID    string `json:"id"`
	Kind  Kind   `json:"kind"`
	State State  `json:"state"`

	Title string `json:"title"`
	Ask   string `json:"ask"`   // what the giver says when offering
	Nag   string `json:"nag"`   // what they say while you are still at it
	Thank string `json:"thank"` // what they say on completion

	Giver    string `json:"giver"`
	GiverPOI int    `json:"giverPOI"`

	// Target is the location a delve or delivery points at.
	TargetPOI  int    `json:"targetPOI,omitempty"`
	TargetName string `json:"targetName,omitempty"`

	// Item is what a fetch quest wants; Monster is what a cull quest counts.
	Item        string `json:"item,omitempty"`
	MonsterID   string `json:"monsterID,omitempty"`
	MonsterName string `json:"monsterName,omitempty"`

	Need int `json:"need"`
	Have int `json:"have"`

	RewardCoins int64 `json:"rewardCoins"`
	RewardXP    int64 `json:"rewardXP"`
}

// Complete reports whether the conditions are met.
func (q *Quest) Complete() bool { return q.Have >= q.Need }

// Progress renders the counter for the log, or a plain state for the errands
// that are not counted.
func (q *Quest) Progress() string {
	switch q.Kind {
	case Fetch, Cull:
		return fmt.Sprintf("%d / %d", core.Min(q.Have, q.Need), q.Need)
	default:
		if q.Complete() {
			return "done"
		}
		return "outstanding"
	}
}

// Writer supplies the generated prose. The content package implements it.
type Writer interface {
	QuestLine(g *core.RNG, kind, part string) string
}

// Catalog is the slice of the content tables the generator needs. Taking an
// interface rather than the tables themselves keeps this package independent of
// how content happens to be loaded.
type Catalog interface {
	// BiomeDrops lists item names that monsters of a biome can drop.
	BiomeDrops(biome string) []string
	// BiomeMonsters lists the monsters of a biome near a level.
	BiomeMonsters(biome string, level int) []*model.MonsterDef
}

// Generate invents an errand for an NPC standing in a settlement, or reports
// false when the surrounding world offers nothing to ask for.
func Generate(g *core.RNG, w *world.Map, cat Catalog, wr Writer, giverPOI int, giver string) (*Quest, bool) {
	if giverPOI < 0 || giverPOI >= len(w.POIs) {
		return nil, false
	}
	home := w.POIs[giverPOI]

	// Only offer errands that point at something real and nearby. Building the
	// candidate list first means a kind is never chosen and then abandoned.
	var kinds []Kind
	if len(nearbyDelves(w, home)) > 0 {
		kinds = append(kinds, Delve)
	}
	if len(nearbySettlements(w, home, giverPOI)) > 0 {
		kinds = append(kinds, Deliver)
	}
	biome := biomeAround(w, home)
	if len(cat.BiomeDrops(biome)) > 0 {
		kinds = append(kinds, Fetch)
	}
	if len(cat.BiomeMonsters(biome, home.Level)) > 0 {
		kinds = append(kinds, Cull)
	}
	if len(kinds) == 0 {
		return nil, false
	}

	q := &Quest{
		State:    Active,
		Giver:    giver,
		GiverPOI: giverPOI,
		Kind:     core.Pick(g, kinds),
	}

	switch q.Kind {
	case Delve:
		target := core.Pick(g, nearbyDelves(w, home))
		q.TargetPOI, q.TargetName = target.idx, target.poi.Name
		q.Need = 1
		q.RewardCoins = int64(60+g.Intn(40)) * int64(target.poi.Level)
		q.RewardXP = int64(40+g.Intn(30)) * int64(target.poi.Level)

	case Deliver:
		target := core.Pick(g, nearbySettlements(w, home, giverPOI))
		q.TargetPOI, q.TargetName = target.idx, target.poi.Name
		q.Need = 1
		q.RewardCoins = int64(25+g.Intn(25)) * int64(core.Max(1, home.Level))
		q.RewardXP = int64(15+g.Intn(15)) * int64(core.Max(1, home.Level))

	case Fetch:
		q.Item = core.Pick(g, cat.BiomeDrops(biome))
		q.Need = g.Between(2, 5)
		q.RewardCoins = int64(18+g.Intn(14)) * int64(q.Need) * int64(core.Max(1, home.Level))
		q.RewardXP = int64(12+g.Intn(10)) * int64(q.Need) * int64(core.Max(1, home.Level))

	case Cull:
		m := core.Pick(g, cat.BiomeMonsters(biome, home.Level))
		q.MonsterID, q.MonsterName = m.ID, m.Name
		q.Need = g.Between(3, 6)
		q.RewardCoins = int64(m.Coins) * int64(q.Need) * 2
		q.RewardXP = int64(m.XP) * int64(q.Need) / 2
	}

	q.ID = fmt.Sprintf("%s-%d-%d", q.Kind, giverPOI, g.Intn(1<<24))
	q.Title = q.titleText()
	q.Ask = q.fill(wr.QuestLine(g, string(q.Kind), "ask"))
	q.Nag = q.fill(wr.QuestLine(g, string(q.Kind), "nag"))
	q.Thank = q.fill(wr.QuestLine(g, string(q.Kind), "thank"))
	return q, true
}

func (q *Quest) titleText() string {
	switch q.Kind {
	case Delve:
		return "Clear " + q.TargetName
	case Deliver:
		return "Parcel for " + q.TargetName
	case Fetch:
		return fmt.Sprintf("%d x %s", q.Need, q.Item)
	default:
		return fmt.Sprintf("Cull %d %s", q.Need, q.MonsterName)
	}
}

// fill substitutes the quest's particulars into a template.
func (q *Quest) fill(s string) string {
	r := map[string]string{
		"{N}": fmt.Sprint(q.Need),
		"{I}": q.Item,
		"{M}": q.MonsterName,
		"{P}": q.TargetName,
		"{G}": q.Giver,
	}
	for k, v := range r {
		s = replaceAll(s, k, v)
	}
	return s
}

func replaceAll(s, old, new string) string {
	out := ""
	for {
		i := indexOf(s, old)
		if i < 0 {
			return out + s
		}
		out += s[:i] + new
		s = s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// poiRef pairs a location with its index, which is what a quest stores.
type poiRef struct {
	idx int
	poi *world.POI
}

// questRange is how far an errand will send you. Far enough to be a trip,
// close enough that it is not the whole map.
const questRange = 55

func nearbyDelves(w *world.Map, home *world.POI) []poiRef {
	var out []poiRef
	for i, p := range w.POIs {
		if p.Cleared || p == home {
			continue
		}
		if p.Kind != world.KindDungeon && p.Kind != world.KindCave && p.Kind != world.KindRuin {
			continue
		}
		if p.Pos.Manhattan(home.Pos) > questRange {
			continue
		}
		out = append(out, poiRef{i, p})
	}
	return out
}

func nearbySettlements(w *world.Map, home *world.POI, homeIdx int) []poiRef {
	var out []poiRef
	for i, p := range w.POIs {
		if i == homeIdx || !p.Kind.Settlement() {
			continue
		}
		if p.Pos.Manhattan(home.Pos) > questRange {
			continue
		}
		out = append(out, poiRef{i, p})
	}
	return out
}

// biomeAround reports the dominant monster table near a location, which is what
// an errand about "the woods outside town" should actually mean.
func biomeAround(w *world.Map, home *world.POI) string {
	counts := map[string]int{}
	for dy := -6; dy <= 6; dy++ {
		for dx := -6; dx <= 6; dx++ {
			t := w.At(home.Pos.X+dx, home.Pos.Y+dy)
			if !t.Passable() {
				continue
			}
			counts[t.Biome()]++
		}
	}
	best, bestN := "plains", 0
	for b, n := range counts {
		if n > bestN {
			best, bestN = b, n
		}
	}
	return best
}

// --- progress -------------------------------------------------------------

// Log is the player's set of errands.
type Log struct {
	Quests []*Quest `json:"quests"`
}

// Add records a newly accepted errand.
func (l *Log) Add(q *Quest) { l.Quests = append(l.Quests, q) }

// Active lists errands still in hand, newest last.
func (l *Log) Active() []*Quest {
	var out []*Quest
	for _, q := range l.Quests {
		if q.State != Closed {
			out = append(out, q)
		}
	}
	return out
}

// CountActive reports how many errands are outstanding, which is what caps how
// many an NPC will pile on.
func (l *Log) CountActive() int { return len(l.Active()) }

// HasFrom reports whether this location has already given out an errand still
// in hand, so one town does not become a queue.
func (l *Log) HasFrom(poiIdx int) bool {
	for _, q := range l.Active() {
		if q.GiverPOI == poiIdx {
			return true
		}
	}
	return false
}

// OnMonsterKilled advances cull quests and returns those that just completed.
func (l *Log) OnMonsterKilled(id string) []*Quest {
	var done []*Quest
	for _, q := range l.Active() {
		if q.Kind == Cull && q.MonsterID == id && !q.Complete() {
			q.Have++
			if q.Complete() {
				done = append(done, q)
			}
		}
	}
	return done
}

// OnPOICleared advances delve quests.
func (l *Log) OnPOICleared(idx int) []*Quest {
	var done []*Quest
	for _, q := range l.Active() {
		if q.Kind == Delve && q.TargetPOI == idx && !q.Complete() {
			q.Have = q.Need
			done = append(done, q)
		}
	}
	return done
}

// OnEnteredPOI advances delivery quests.
func (l *Log) OnEnteredPOI(idx int) []*Quest {
	var done []*Quest
	for _, q := range l.Active() {
		if q.Kind == Deliver && q.TargetPOI == idx && !q.Complete() {
			q.Have = q.Need
			done = append(done, q)
		}
	}
	return done
}

// SyncFetch recounts fetch quests against the bag. Fetch is recounted rather
// than incremented because items can be sold, dropped or spent, and a counter
// that only goes up would let a player hand in things they no longer have.
func (l *Log) SyncFetch(bag []model.Item) {
	for _, q := range l.Active() {
		if q.Kind != Fetch {
			continue
		}
		have := 0
		for _, it := range bag {
			if it.Name == q.Item {
				have += it.Count
			}
		}
		q.Have = have
	}
}

// ReadyAt lists errands that can be handed in at a location.
func (l *Log) ReadyAt(poiIdx int) []*Quest {
	var out []*Quest
	for _, q := range l.Active() {
		if q.GiverPOI == poiIdx && q.Complete() {
			out = append(out, q)
		}
	}
	return out
}

// Close marks an errand handed in.
func (l *Log) Close(q *Quest) { q.State = Closed }
