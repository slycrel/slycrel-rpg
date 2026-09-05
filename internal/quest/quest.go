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
	"strings"

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
	// Escort: take a person somewhere, and have them with you while you do it.
	//
	// The nearest relative is Deliver and the difference is that the parcel
	// walks behind you. That is the whole feature: they are visible, they are
	// slow, and sometimes they are on a clock.
	//
	// What an escort deliberately is *not* is a risk of losing them. Companions
	// in this game cannot die — a party member at zero hit points is out of the
	// fight and back on their feet the moment it ends, which checkEnd says in
	// as many words — so an errand whose stake was "keep them alive" would be
	// an errand with no stake at all, dressed as one. The costs are real
	// instead: a deadline that the clock spends whether you fight or not, and
	// company that draws more attention on the road than you would alone.
	Escort Kind = "escort"
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

	// Made says the destination is one this errand invented rather than one
	// already on the map, and TargetAt is where it is.
	//
	// The world has about forty locations on it and every errand in the game
	// has always pointed at one of them, so a run's errands send you to the
	// same forty markers over and over. A made place is a crossroads, a burnt
	// farm, a camp expecting you — somewhere that exists *because* an errand
	// says so, generated from this quest's own ID and therefore the same place
	// on the second visit as on the first, and gone when the errand closes.
	//
	// It costs the save format nothing, which is the whole reason it is shaped
	// this way: the quest is already saved, so a place derived from the quest
	// is already saved. It is the same seam the wayside found.
	//
	// An explicit flag rather than reading TargetPOI as -1, because a zero
	// value that means something real is a zero value that turns every old
	// save into a silent claim about location zero — which is a lesson this
	// file has already been taught twice.
	Made     bool       `json:"made,omitempty"`
	TargetAt core.Point `json:"targetAt,omitempty"`

	// Escortee is who is walking behind you, and Helps says they are the sort
	// who joins in rather than the sort who hides.
	Escortee string `json:"escortee,omitempty"`
	Helps    bool   `json:"helps,omitempty"`
	// Due is the clock step the errand expires at, or nought for one with no
	// deadline at all — which is most of them, and is the zero value meaning
	// the safe thing rather than "expired at the beginning of time".
	Due int `json:"due,omitempty"`

	// Item is what a fetch quest wants; Monster is what a cull quest counts.
	Item        string `json:"item,omitempty"`
	MonsterID   string `json:"monsterID,omitempty"`
	MonsterName string `json:"monsterName,omitempty"`

	// Where is the country a fetch or a cull happens in, already filled with
	// the settlement's name: "the woods outside Blightford".
	//
	// A delve and a delivery point at a POI and have TargetName for it; the
	// other two happen in a region, and until this field existed they named no
	// location whatsoever. The player was told to bring four of something and
	// never told where any of it was.
	//
	// The empty string is the honest zero value and every save written before
	// today supplies it: it means "this errand does not know where", and every
	// line that uses it is written to be a whole sentence without it. It must
	// never be filled in with a guess — naming a place that is not there is
	// the one thing this generator has always refused to do.
	Where string `json:"where,omitempty"`

	// GiverPlace is the settlement the giver is standing in, so the errand can
	// say where to take it back to without the quest package holding a map.
	GiverPlace string `json:"giverPlace,omitempty"`

	Need int `json:"need"`
	Have int `json:"have"`

	RewardCoins int64 `json:"rewardCoins"`
	RewardXP    int64 `json:"rewardXP"`
}

// Expired reports whether a deadline has passed. An errand with no deadline
// never expires, which is what a nought here has to mean: every quest in every
// save written before deadlines existed carries one.
func (q *Quest) Expired(step int) bool {
	return q.Due > 0 && step > q.Due && !q.Complete()
}

// DueIn is how many days are left, for the journal to say. Negative when the
// deadline has gone, and meaningless when there is not one.
func (q *Quest) DueIn(step, dayLength int) int {
	if q.Due <= 0 || dayLength <= 0 {
		return 0
	}
	return (q.Due - step) / dayLength
}

// SiteSeed is the seed for a made destination, so the place an errand invents
// is the same place every time it is walked to.
func (q *Quest) SiteSeed() int64 { return core.SeedFrom(q.ID) }

// Complete reports whether the conditions are met.
func (q *Quest) Complete() bool { return q.Have >= q.Need }

// Remaining is how many are still wanted, which is what a person nagging you
// about an errand would actually say.
//
// The nag lines used to quote Need — the number originally asked for — so
// somebody holding three of four Chitin Scrap was told "Still 4 Chitin Scrap.
// The number has not changed." It had changed. It had changed three times.
func (q *Quest) Remaining() int { return core.Max(0, q.Need-q.Have) }

// Species is the creature's name as prose should say it: the half before the
// comma.
//
// CLAUDE.md has had this rule since the battle transcript learned it — "prose
// uses Monster.Short()", because "Wolf, Deeply Unimpressed B bites Bosk" is
// unreadable — and the quest generator never did. It interpolated the whole
// name into flowing sentences, so a cull errand read "There's Wolf, Deeply
// Unimpressed out there", with the comma the name plate wants sitting in the
// middle of a sentence that cannot have one. Sixty-eight of the seventy-nine
// creatures in the game carry a comma.
//
// The full name is still what the title and the journal row show, because
// those are labels, and a label is exactly where the epithet belongs.
func (q *Quest) Species() string {
	head, _ := model.SplitName(q.MonsterName)
	return head
}

// Objective is the one line that says what to physically do next.
//
// It is computed rather than stored, which is the point of it. A stored line
// is written once, at the moment the errand is taken, and is wrong from the
// first creature killed; this one is re-derived every time it is read, so it
// counts down, and it changes to "go back and say so" the moment the counting
// stops. It also cannot rot in an old save, having never been in one.
//
// Every branch is an imperative verb and a named place, in that order, because
// that is the sentence a player who has put the game down for a week needs and
// the flavour lines are constitutionally incapable of being. The giver's own
// voice is still there, above this, saying the same thing in character — the
// two are not redundant, they are the difference between what somebody said to
// you and what you wrote down afterwards.
func (q *Quest) Objective() string {
	// One action, not two. The finished errand has its own line, so an
	// outstanding one saying "…then go back to Dregg" is announcing a step the
	// player cannot take yet and will be told about when they can. It also
	// doubled the length of the only sentence on this screen that has to be
	// read in a hurry.
	if q.Complete() {
		back := "Go back to " + q.Giver
		// Not twice in one sentence. "…in the woods outside Crown of the Sunken
		// Barge, then go back to Dregg in Crown of the Sunken Barge" is what
		// naming both halves independently produces, and the generated place
		// names in this game are long enough that it wrapped to three lines.
		if q.GiverPlace != "" && !strings.Contains(q.Where, q.GiverPlace) {
			back += " in " + q.GiverPlace
		}
		return back + "."
	}
	switch q.Kind {
	case Fetch:
		return "Find " + fmt.Sprintf("%d more %s", q.Remaining(), q.Item) + q.inWhere() + "."
	case Cull:
		return "Kill " + fmt.Sprintf("%d more %s", q.Remaining(), q.Species()) + q.inWhere() + "."
	case Delve:
		return "Travel to " + q.TargetName + " and clear it out."
	case Deliver:
		return "Carry the parcel to " + q.TargetName + "."
	case Escort:
		return "Walk " + q.Escortee + " to " + q.TargetName + "."
	}
	return "Go back to " + q.Giver + "."
}

// inWhere is " in the woods outside Blightford", or nothing at all when the
// errand does not know where — which is every quest in every save written
// before the field existed.
func (q *Quest) inWhere() string {
	if q.Where == "" {
		return ""
	}
	return " in " + q.Where
}

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
	// PersonName names whoever an errand is about — the person an escort is
	// carrying, who has to be called something before the errand can say who
	// they are.
	PersonName(g *core.RNG) string
	// QuestWhere names the country around a settlement, with {P} left in it.
	// Empty when the biome has no phrase — an errand that cannot say where
	// says nothing, rather than saying "somewhere nearby".
	QuestWhere(g *core.RNG, biome string) string
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
// Generate builds an errand. prefer names the kind this giver ought to ask for
// and is honoured when the settlement can support it.
//
// The preference exists so a face can be chosen before the quest is: the
// portrait pools are keyed by errand, and a person whose face changed when they
// handed you a job — and changed back when you finished it — would undo the one
// thing a face is for. Passing the empty string picks at random, which is what
// this always did.
//
// It is a preference rather than an instruction because the candidate list is
// built from what actually exists nearby. A settlement with no dungeon in reach
// cannot ask for a delve however much its resident looks the part, and offering
// an errand that points at nothing would be a worse failure than a face that
// does not quite match the job.
// clockNow and dayLength are set by the caller through Generate's clock
// arguments; see the signature.
func Generate(g *core.RNG, w *world.Map, cat Catalog, wr Writer, giverPOI int, giver string,
	prefer Kind, clockNow, dayLength int) (*Quest, bool) {
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
		kinds = append(kinds, Deliver, Escort)
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

	chosen := core.Pick(g, kinds)
	for _, k := range kinds {
		if k == prefer {
			chosen = prefer
			break
		}
	}
	q := &Quest{
		State:      Active,
		Giver:      giver,
		GiverPOI:   giverPOI,
		GiverPlace: home.Name,
		Kind:       chosen,
	}
	// Where a fetch or a cull happens. Filled with the settlement's own name
	// here rather than at display time, because this is the one place that has
	// the map — and it stays empty when the biome has no phrase, which every
	// line downstream is written to survive.
	if q.Kind == Fetch || q.Kind == Cull {
		if phrase := wr.QuestWhere(g, biome); phrase != "" {
			q.Where = replaceAll(phrase, "{P}", home.Name)
		}
	}

	switch q.Kind {
	case Delve:
		target := core.Pick(g, nearbyDelves(w, home))
		q.TargetPOI, q.TargetName = target.idx, target.poi.Name
		q.Need = 1
		q.RewardCoins = int64(60+g.Intn(40)) * int64(target.poi.Level)
		q.RewardXP = int64(40+g.Intn(30)) * int64(target.poi.Level)

	case Escort:
		// Where a delivery goes, and by the same rules: sometimes a town,
		// sometimes somewhere this errand invented. A person being taken to a
		// crossroads to meet somebody is if anything the more natural of the
		// two.
		if at, name, ok := madeDestination(g, w, home); ok && g.Chance(0.5) {
			q.Made, q.TargetAt, q.TargetName = true, at, name
			q.TargetPOI = -1
		} else {
			target := core.Pick(g, nearbySettlements(w, home, giverPOI))
			q.TargetPOI, q.TargetName = target.idx, target.poi.Name
		}
		q.Need = 1
		q.Escortee = wr.PersonName(g)
		// Sometimes they fight, sometimes there is a clock, sometimes both, and
		// sometimes it is only a walk. Four shapes out of two coins, because an
		// errand kind whose every instance is the same errand is one errand
		// with a lot of names.
		q.Helps = g.Chance(0.5)
		if g.Chance(0.5) {
			// Two to four days. The clock spends whether you fight or not,
			// which is the point of it — this is the only errand in the game
			// that can be failed rather than abandoned.
			q.Due = clockNow + g.Between(2, 4)*dayLength
		}
		q.RewardCoins = int64(40+g.Intn(30)) * int64(core.Max(1, home.Level))
		q.RewardXP = int64(25+g.Intn(20)) * int64(core.Max(1, home.Level))

	case Deliver:
		// Sometimes to a town, sometimes to somewhere that is not on the map.
		//
		// A run's errands used to send you to the same forty markers, because
		// forty markers is all the world has. A made destination is a
		// crossroads or a burnt farm that exists because this errand says so —
		// it is somewhere new every time, and it is the only kind of
		// destination the generator can produce more of.
		//
		// Still only ever *near* somewhere real: the tile is picked off the map
		// and checked for walkable ground, which is the same rule the rest of
		// this generator follows. Naming a place is fine as long as walking
		// there finds one.
		if at, name, ok := madeDestination(g, w, home); ok && g.Chance(0.45) {
			q.Made, q.TargetAt, q.TargetName = true, at, name
			q.TargetPOI = -1
		} else {
			target := core.Pick(g, nearbySettlements(w, home, giverPOI))
			q.TargetPOI, q.TargetName = target.idx, target.poi.Name
		}
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
	// The nag is stored with its placeholders still in it and filled on the way
	// out, because it is the one line that has to be true *later*. It counts
	// what is left, and what is left changes; a line filled here is a line
	// written before the first creature died. NagLine is what reads it.
	//
	// An old save holds a nag that was filled at generation, and running fill
	// over an already-filled string changes nothing — so this degrades to
	// exactly the previous behaviour rather than to a sentence with braces in
	// it.
	q.Nag = wr.QuestLine(g, string(q.Kind), "nag")
	q.Thank = q.fill(wr.QuestLine(g, string(q.Kind), "thank"))
	return q, true
}

// NagLine is what the giver says while the errand is still outstanding, filled
// against the counter as it stands now.
func (q *Quest) NagLine() string { return q.fill(q.Nag) }

// titleText is the errand's name in a list.
//
// A title is a label, so it carries the whole creature name — the comma is
// correct here and wrong in a sentence — and it leads with a verb, so a journal
// row says what the errand *is* rather than only what it involves. "3 x Chitin
// Scrap" named a quantity of a thing and no action at all; the row beside it
// already shows the count, so the title never needed to.
func (q *Quest) titleText() string {
	switch q.Kind {
	case Delve:
		return "Clear out " + q.TargetName
	case Deliver:
		return "Deliver a parcel to " + q.TargetName
	case Escort:
		return "Take " + q.Escortee + " to " + q.TargetName
	case Fetch:
		return "Gather " + q.Item
	default:
		return "Cull the " + q.MonsterName
	}
}

// fill substitutes the quest's particulars into a template.
func (q *Quest) fill(s string) string {
	r := map[string]string{
		"{N}": fmt.Sprint(q.Need),
		"{R}": fmt.Sprint(q.Remaining()),
		"{I}": q.Item,
		// The species, not the whole name: this goes into sentences. See
		// Quest.Species.
		"{M}": q.Species(),
		"{P}": q.TargetName,
		"{E}": q.Escortee,
		"{W}": q.Where,
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

// madeDestination finds open ground for a place an errand invents, and a name
// for it.
//
// Between a quarter and most of the way to the edge of what an errand will
// send you, on ground you can actually stand on, and not on top of a location
// that already exists — a made place sharing a square with a real one is two
// things on one tile and a player who cannot tell which they walked into.
//
// Returns false when nowhere works, and the caller falls back to a real
// settlement. **A destination that cannot be reached is worse than a dull one**,
// which is the rule this generator has followed since it was written.
func madeDestination(g *core.RNG, w *world.Map, home *world.POI) (core.Point, string, bool) {
	for try := 0; try < 40; try++ {
		d := g.Between(questRange/4, questRange*3/4)
		dx := g.Between(-d, d)
		dy := d - core.Abs(dx)
		if g.Chance(0.5) {
			dy = -dy
		}
		p := core.Point{X: home.Pos.X + dx, Y: home.Pos.Y + dy}
		if !Placeable(w, p) {
			continue
		}
		return p, core.Pick(g, madeNames), true
	}
	return core.Point{}, "", false
}

// Placeable reports whether an errand may invent a place on this tile.
//
// **A named rule rather than a condition inside a loop, because a rule can be
// tested and a condition can only be sampled.** The first version of this was
// two clauses inside madeDestination, and the test written for it asserted that
// no generated destination landed on a location — which passed with the clause
// deleted, because a tile picked at random almost never hits one of forty-five
// markers. An assertion that cannot be made to fail is not a check, it is a
// sentence.
//
// Split out, there are two provable statements instead: this function says no
// to a location's own square and yes to open ground, directly and with no dice
// in it; and madeDestination returns only squares this says yes to. Break
// either and something fails on the first run.
//
// The same shape is available anywhere else in this game that decides where
// something may stand. There are several, and they are all currently conditions
// inside loops.
func Placeable(w *world.Map, p core.Point) bool {
	return w.Walkable(p.X, p.Y) && w.POIAt(p.X, p.Y) == nil
}

// madeNames are what a place an errand invents is called.
//
// Deliberately generic and deliberately not run through the place-name
// generator: those names — "Crown of the Sunken Barge" — are for locations that
// are on a map and have been there a while. This is a crossroads. Somebody
// says "meet them at the burnt farm" because that is what it is, not because
// it has a name.
var madeNames = []string{
	"the crossroads",
	"the burnt farm",
	"the old ford",
	"the drovers' camp",
	"the milestone",
	"the broken bridge",
	"the shepherd's hut",
	"the boundary stone",
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

// From returns the errand this particular person at this particular place gave
// you, if any.
//
// The person matters as much as the place. Looking up by settlement alone meant
// every townsperson nagged you about an errand they had not given you, and
// every townsperson would accept the hand-in for it — so the one thing a quest
// asks of you socially, going back to the person who asked, was not being asked
// at all.
func (l *Log) From(poiIdx int, giver string) *Quest {
	for _, q := range l.Active() {
		if q.GiverPOI == poiIdx && q.Giver == giver {
			return q
		}
	}
	return nil
}

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
		if (q.Kind == Deliver || q.Kind == Escort) && q.TargetPOI == idx && !q.Complete() {
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

// MadeAt returns the errand whose invented destination is this tile, if any.
//
// Only errands still outstanding: a made place exists because the errand does,
// so when the errand closes the place stops being there. Somebody walking back
// to the crossroads a week later finds a crossroads, which is what it always
// was — the point was never the ground, it was who was standing on it.
func (l *Log) MadeAt(at core.Point) *Quest {
	for _, q := range l.Active() {
		if q.Made && q.TargetAt == at {
			return q
		}
	}
	return nil
}

// OnReachedMade advances an errand whose destination is one it invented.
//
// Its own entry point rather than a branch inside OnEnteredPOI, because that
// one is keyed on a POI index and a made place has none. Same shape, same
// result: arriving is what completes a delivery, wherever it is delivered to.
func (l *Log) OnReachedMade(q *Quest) []*Quest {
	if q == nil || !q.Made || q.Complete() {
		return nil
	}
	q.Have = q.Need
	return []*Quest{q}
}
