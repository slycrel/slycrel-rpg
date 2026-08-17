// Package saga is the long story: a chain of places strung across the
// continent with an authored ending on the far end of it.
//
// It is the third thing in the game built on the same split, and by now that
// split is the house style: the *writing* is authored and lives in
// data/text/sagas.json, and the *staging* — which ruin, which creature, which
// town — is drawn from the world at the moment the saga is cast and then frozen
// into the save. Quests are generated from the world and forgettable on
// purpose; a companion's thread is authored and belongs to a person; a saga is
// authored and belongs to the map.
//
// The one idea that makes it work is that its legs are spread across the reach
// of the continent, each aimed at its share of the way out. Nothing here gates
// on level, checks a flag, or refuses to advance — the difficulty curve does
// the pacing on its own, because the danger of a region is a function of how
// far out it is. A player who runs the whole spine at level three will die in
// the fourth region, and will have been given three increasingly obvious
// warnings on the way.
//
// "Spread across" rather than "increasing", and the difference is the whole
// feature. The first version took the nearest place further out than the last,
// which is strictly increasing and packed all five legs into the near end —
// 6, 11, 12, 16 and 17 tiles, every one of them inside the eighteen-tile radius
// RegionLevel reads. The country around the last leg was the country around the
// first, and the pacing claim above was simply untrue. cmd/balance prints that
// column now: it went from the far end being rougher in 6 stagings out of 16 to
// 16 out of 16.
//
// An arc is the same machinery, shorter, and found rather than given: three or
// four legs, cast when you walk into somewhere nobody sent you.
package saga

import (
	"fmt"
	"sort"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Trigger is what a leg waits for.
//
// Three, and all three are things the game already notices for quests — a door,
// a cleared room, a kill. A saga that needed its own bookkeeping would be a
// saga that quietly stopped advancing the next time somebody rearranged a
// scene, and it is the one system in the game a player cannot route around.
type Trigger string

const (
	// Reach fires on walking into the leg's place.
	Reach Trigger = "reach"
	// Clear fires when the leg's place is emptied.
	Clear Trigger = "clear"
	// Hunt counts the cast creature put down, anywhere.
	Hunt Trigger = "hunt"
)

// Leg is one step of a saga: somewhere to go, something to do, and what you
// learn for having done it.
type Leg struct {
	Trigger Trigger `json:"trigger"`
	// Need is how many, for Hunt. Zero reads as one.
	Need int `json:"need,omitempty"`
	// Place is the sort of location this leg wants: "delve" for a dungeon, cave
	// or ruin, "settlement" for somewhere with a roof, empty for either.
	//
	// Hunt legs still take a place, because a hunt with nowhere attached is a
	// leg the player cannot be pointed at, and the whole spine is a line of
	// pins on a map.
	Place string `json:"place,omitempty"`
	Text  string `json:"text"` // what happens when the leg comes due
	Note  string `json:"note"` // the journal's line while it is outstanding
}

// Ending is one way the last leg can go. Same rules as a companion's: at least
// one costs nothing, and none of them beats another on every axis.
type Ending struct {
	Label string `json:"label"`
	Text  string `json:"text"`
	Coins int64  `json:"coins,omitempty"`
	XP    int64  `json:"xp,omitempty"`
	Fame  int    `json:"fame,omitempty"`
	Shame int    `json:"shame,omitempty"`
	Honor int    `json:"honor,omitempty"`
}

// Costs reports whether taking this ending needs money up front.
func (e Ending) Costs() int64 {
	if e.Coins < 0 {
		return -e.Coins
	}
	return 0
}

// Skeleton is an authored saga with nowhere cast in it yet.
//
// The placeholders the writing may use are {P} the place this leg happens at,
// {X} the creature the saga is about, and {I} something out there drops.
// Nothing else may be named, which is the same rule the quest generator and the
// thread caster follow: a skeleton that mentions a specific ruin is a skeleton
// that breaks on the next seed.
type Skeleton struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Arc marks a short optional one, found in the world rather than handed
	// over at the start.
	Arc bool `json:"arc,omitempty"`
	// Opening is what starts it: the letter, the notice, the person at the
	// gate. It names {P}, which for the opening is the first leg's place.
	Opening string   `json:"opening"`
	Legs    []Leg    `json:"legs"`
	Endings []Ending `json:"endings"`
}

// Book is every authored saga, as loaded from data.
type Book struct {
	Sagas []Skeleton `json:"sagas"`
}

// Get finds a skeleton by id.
func (b *Book) Get(id string) (*Skeleton, bool) {
	if b == nil {
		return nil, false
	}
	for i := range b.Sagas {
		if b.Sagas[i].ID == id {
			return &b.Sagas[i], true
		}
	}
	return nil, false
}

// Spines lists the long ones, and Arcs the short optional ones.
func (b *Book) Spines() []*Skeleton { return b.pick(false) }

// Arcs lists the short optional ones.
func (b *Book) Arcs() []*Skeleton { return b.pick(true) }

func (b *Book) pick(arc bool) []*Skeleton {
	if b == nil {
		return nil
	}
	var out []*Skeleton
	for i := range b.Sagas {
		if b.Sagas[i].Arc == arc {
			out = append(out, &b.Sagas[i])
		}
	}
	return out
}

// State is where a saga has got to.
type State string

const (
	// Open: still working through the legs.
	Open State = "open"
	// Ready: the last leg is done and the choice is waiting.
	Ready State = "ready"
	// Closed: finished, one way or another.
	Closed State = "closed"
)

// Saga is one cast story in progress.
//
// Everything pointing at the world is an index or a name, so it serialises to a
// few lines of JSON and survives a save without the continent needing to be
// there when it is read back.
type Saga struct {
	Skeleton string `json:"skeleton"`
	Title    string `json:"title"`
	State    State  `json:"state"`

	// Places is one location index per leg, in order and at increasing
	// distance from where the run began.
	Places []int `json:"places"`
	// PlaceNames is parallel to Places, so the journal and the prose can name a
	// location without the world being loaded.
	PlaceNames []string `json:"placeNames"`

	// Roles are the filled non-place placeholders, stored filled rather than
	// re-derived so the story cannot change under a save.
	Roles     map[string]string `json:"roles,omitempty"`
	MonsterID string            `json:"monsterID,omitempty"`

	At   int `json:"at"`   // the leg being worked on
	Have int `json:"have"` // progress within it

	Ended string `json:"ended,omitempty"`
}

// Catalog is the slice of the content tables casting needs, as an interface so
// this package stays independent of how content is loaded.
type Catalog interface {
	BiomeDrops(biome string) []string
	BiomeMonsters(biome string, level int) []*model.MonsterDef
}

// Fill substitutes the cast into a line, for the leg currently in play.
func (s *Saga) Fill(text string) string { return s.FillAt(text, s.At) }

// FillAt substitutes the cast into a line as of a particular leg, which is what
// {P} depends on.
func (s *Saga) FillAt(text string, leg int) string {
	if s == nil {
		return text
	}
	out := text
	for k, v := range s.Roles {
		out = strings.ReplaceAll(out, k, v)
	}
	if leg >= 0 && leg < len(s.PlaceNames) {
		out = strings.ReplaceAll(out, "{P}", s.PlaceNames[leg])
	}
	return out
}

// Place is the location index the current leg points at, or -1.
func (s *Saga) Place() int {
	if s == nil || s.At < 0 || s.At >= len(s.Places) {
		return -1
	}
	return s.Places[s.At]
}

// PlaceName is what to call where you are being sent.
func (s *Saga) PlaceName() string {
	if s == nil || s.At < 0 || s.At >= len(s.PlaceNames) {
		return ""
	}
	return s.PlaceNames[s.At]
}

// leg returns the leg being worked on.
func (s *Saga) leg(b *Book) (Leg, bool) {
	sk, ok := b.Get(s.Skeleton)
	if !ok || s.At < 0 || s.At >= len(sk.Legs) {
		return Leg{}, false
	}
	return sk.Legs[s.At], true
}

// Note is the journal's line for what is outstanding.
func (s *Saga) Note(b *Book) string {
	if s.State == Closed {
		return s.Ended
	}
	if s.State == Ready {
		return "it is finished, and there is a decision in it"
	}
	l, ok := s.leg(b)
	if !ok {
		return ""
	}
	return s.Fill(l.Note)
}

// Progress renders the counter for a leg that has one.
//
// Only the hunts. A leg that is "go there" has no halfway, and "0 / 1" beside
// a story reads as a chore with a quantity rather than a place to be.
func (s *Saga) Progress(b *Book) string {
	l, ok := s.leg(b)
	if !ok || s.State != Open || l.Trigger != Hunt {
		return ""
	}
	return fmt.Sprintf("%d / %d", s.Have, core.Max(1, l.Need))
}

// Options are the endings on offer once the last leg is done.
func (s *Saga) Options(b *Book) []Ending {
	sk, ok := b.Get(s.Skeleton)
	if !ok || s.State != Ready {
		return nil
	}
	out := make([]Ending, 0, len(sk.Endings))
	for _, e := range sk.Endings {
		e.Text = s.FillAt(e.Text, len(s.Places)-1)
		out = append(out, e)
	}
	return out
}

// Resolve closes a saga on a chosen ending. Paying it out is the caller's
// business, because the purse lives somewhere this package cannot see.
func (s *Saga) Resolve(e Ending) {
	s.State = Closed
	s.Ended = e.Label
}

// --- casting ---------------------------------------------------------------

// Cast fills a skeleton with places this continent actually has, and reports
// false when it cannot.
//
// from is where the spine starts — the capital for the main story, and wherever
// you were standing for an arc you stumbled into. taken lists the ids already
// running, so a second saga is a different one.
func Cast(g *core.RNG, b *Book, w *world.Map, cat Catalog, sk *Skeleton, from core.Point, level int, taken []string) (*Saga, bool) {
	if g == nil || b == nil || w == nil || sk == nil || len(sk.Legs) == 0 || len(sk.Endings) == 0 {
		return nil, false
	}
	for _, id := range taken {
		if id == sk.ID {
			return nil, false
		}
	}

	places, names, ok := stagePlaces(g, sk, w, from)
	if !ok {
		return nil, false
	}

	s := &Saga{
		Skeleton: sk.ID, Title: sk.Title, State: Open,
		Places: places, PlaceNames: names,
		Roles: map[string]string{},
	}

	// The creature and the thing come from around where the spine *starts*,
	// not around its far end. A saga is cast before the player has been
	// anywhere, so a monster drawn from the last leg's region would be one they
	// have no way to find for hours — the same trap the companion threads hit,
	// and the same answer: cast from where the company is standing now.
	biome := biomeAround(w, from)
	body := skeletonText(sk)
	if strings.Contains(body, "{X}") || usesTrigger(sk, Hunt) {
		mons := cat.BiomeMonsters(biome, core.Max(1, level))
		if len(mons) == 0 {
			return nil, false
		}
		m := core.Pick(g, mons)
		s.Roles["{X}"] = m.Name
		s.MonsterID = m.ID
	}
	if strings.Contains(body, "{I}") {
		drops := cat.BiomeDrops(biome)
		if len(drops) == 0 {
			return nil, false
		}
		s.Roles["{I}"] = core.Pick(g, drops)
	}
	return s, true
}

// stagePlaces picks one location per leg, ordered outward.
//
// This is the whole pacing mechanism. Candidates are sorted by distance from
// the start and then dealt out in order, so leg two is always further than leg
// one and leg five is somewhere a level-three character has no business being.
// Nothing else in the saga knows anything about difficulty; it does not need
// to, because the danger of a region is already a function of how far out it
// is and the spine is a line pointing away from home.
func stagePlaces(g *core.RNG, sk *Skeleton, w *world.Map, from core.Point) ([]int, []string, bool) {
	// Bucketed by what each leg needs, because a leg that wants a dungeon
	// cannot be handed a village.
	pools := map[string][]int{}
	for i, p := range w.POIs {
		pools[""] = append(pools[""], i)
		switch {
		case p.Kind.Settlement():
			pools["settlement"] = append(pools["settlement"], i)
		case p.Kind == world.KindDungeon || p.Kind == world.KindCave || p.Kind == world.KindRuin:
			pools["delve"] = append(pools["delve"], i)
		}
	}
	for k := range pools {
		idx := pools[k]
		sort.Slice(idx, func(a, c int) bool {
			return w.POIs[idx[a]].Pos.Manhattan(from) < w.POIs[idx[c]].Pos.Manhattan(from)
		})
		pools[k] = idx
	}

	// How far the continent reaches from here, which is what the legs are
	// spread across.
	span := 0
	for _, i := range pools[""] {
		if d := w.POIs[i].Pos.Manhattan(from); d > span {
			span = d
		}
	}

	used := map[int]bool{}
	// reach is how far out the previous leg was, so each one has to beat it.
	reach := 0
	var places []int
	var names []string

	for n, l := range sk.Legs {
		// Where this leg wants to be: its share of the way out.
		//
		// The first version took the *nearest* place further than the last one,
		// which sounds like the same thing and is not. It packed every leg into
		// the near end — five legs at 6, 11, 12, 16 and 17 tiles, all inside
		// the eighteen-tile radius RegionLevel reads — so the country around
		// the last one was the country around the first, and the claim that a
		// spine paces itself by geography was simply false. cmd/balance now
		// prints that column, which is how this was found rather than shipped.
		target := legTarget(n, len(sk.Legs), span)
		pick, best := -1, 0
		for _, i := range pools[l.Place] {
			if used[i] {
				continue
			}
			d := w.POIs[i].Pos.Manhattan(from)
			if d <= reach {
				continue
			}
			if gap := core.Abs(d - target); pick < 0 || gap < best {
				pick, best = i, gap
			}
		}
		if pick < 0 {
			// Not enough continent for this story, which is a supported answer
			// — the same one the thread caster gives when there is no ruin to
			// end at. A saga that cannot be finished is worse than no saga.
			return nil, nil, false
		}
		used[pick] = true
		reach = w.POIs[pick].Pos.Manhattan(from)
		places = append(places, pick)
		names = append(names, w.POIs[pick].Name)
	}
	return places, names, true
}

// legTarget is how far out leg n of a story wants to be, given how far the
// continent reaches.
//
// The first leg lands on the doorstep — inside the home region, the one stretch
// of ground the opening was tuned for — and the last lands at the far edge,
// with the rest spaced between. Spreading evenly from zero instead put first
// legs 13 to 27 tiles out against a home region that ends at 14, which is the
// story handing a level-one character a difficulty they did not choose and then
// pointing the compass at it.
//
// The spine is offered at the gate on the first morning. Where it sends you
// first is the one leg that has to be somewhere a new character can go.
func legTarget(n, legs, span int) int {
	near := world.HomeRadius
	if span <= near || legs <= 1 {
		return span
	}
	return near + (span-near)*n/(legs-1)
}

// biomeAround reports the monster table around a point, for casting.
func biomeAround(w *world.Map, at core.Point) string {
	return w.At(at.X, at.Y).Biome()
}

func skeletonText(sk *Skeleton) string {
	var b strings.Builder
	b.WriteString(sk.Title)
	b.WriteString(sk.Opening)
	for _, l := range sk.Legs {
		b.WriteString(l.Text)
		b.WriteString(l.Note)
	}
	for _, e := range sk.Endings {
		b.WriteString(e.Label)
		b.WriteString(e.Text)
	}
	return b.String()
}

func usesTrigger(sk *Skeleton, want Trigger) bool {
	for _, l := range sk.Legs {
		if l.Trigger == want {
			return true
		}
	}
	return false
}

// --- progress --------------------------------------------------------------

// Log is every saga the run is carrying.
type Log struct {
	Sagas []*Saga `json:"sagas,omitempty"`
}

// Add records a newly cast saga.
func (l *Log) Add(s *Saga) { l.Sagas = append(l.Sagas, s) }

// IDs lists the skeletons already in play, for casting.
func (l *Log) IDs() []string {
	var out []string
	for _, s := range l.Sagas {
		out = append(out, s.Skeleton)
	}
	return out
}

// Running lists the ones still going.
func (l *Log) Running() []*Saga {
	var out []*Saga
	for _, s := range l.Sagas {
		if s.State != Closed {
			out = append(out, s)
		}
	}
	return out
}

// Waiting lists the ones with a decision outstanding.
func (l *Log) Waiting() []*Saga {
	var out []*Saga
	for _, s := range l.Sagas {
		if s.State == Ready {
			out = append(out, s)
		}
	}
	return out
}

// Event is something that happened that a leg might be waiting for.
type Event struct {
	Kind Trigger
	// POI is where it happened, for Reach and Clear.
	POI int
	// Monster is the creature's id, for Hunt.
	Monster string
	// N is how many, for Hunt. Zero reads as one.
	N int
}

// Fired is a leg that has just come due.
type Fired struct {
	Saga *Saga
	Text string
	// Last reports that the saga has run out of legs and is now waiting on its
	// ending.
	Last bool
}

// Advance applies an event to every running saga and returns what came due.
func (l *Log) Advance(b *Book, ev Event) []Fired {
	var out []Fired
	n := core.Max(1, ev.N)

	for _, s := range l.Sagas {
		if s.State != Open {
			continue
		}
		sk, ok := b.Get(s.Skeleton)
		if !ok || s.At >= len(sk.Legs) {
			continue
		}
		leg := sk.Legs[s.At]
		if leg.Trigger != ev.Kind {
			continue
		}

		switch ev.Kind {
		case Reach, Clear:
			if ev.POI != s.Place() {
				continue
			}
			s.Have = core.Max(1, leg.Need)
		case Hunt:
			if ev.Monster != s.MonsterID {
				continue
			}
			s.Have += n
		}
		if s.Have < core.Max(1, leg.Need) {
			continue
		}

		f := Fired{Saga: s, Text: s.Fill(leg.Text)}
		s.At++
		s.Have = 0
		if s.At >= len(sk.Legs) {
			s.State = Ready
			f.Last = true
		}
		out = append(out, f)
	}
	return out
}
