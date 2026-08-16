// Package thread is the companion backstory system.
//
// A hireling arrives with a name, a lineage and a sales pitch, and nothing
// behind any of it. A thread is the two or three steps that turn that into a
// person: the previous employer a part-undead is still technically contracted
// to, the arrangement a part-demon will not discuss. It surfaces while you
// travel together and it resolves somewhere the world already generated.
//
// The split is the whole design. The *skeleton* — the beats, the words, the
// choice at the end — is authored, and lives in data/text/threads.json so the
// writing can be revised without touching Go. The *cast* — which ruin, which
// creature, which trophy — is drawn from the world at the moment the companion
// is hired, and then frozen into the save. That keeps the prose hand-made and
// the staging generated, and it means a thread can never name a place this
// continent does not contain, which is the same rule the quest generator
// follows.
//
// A thread is therefore an ordered errand with a fixed cast, where a quest is a
// stateless one. That difference is why this is its own package rather than a
// fifth quest.Kind: quests are interchangeable and forgettable on purpose, and
// a backstory that could be re-rolled would be neither.
package thread

import (
	"fmt"
	"sort"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Trigger is what a beat waits for.
//
// The list is short on purpose. Every entry has to be something the game
// already notices — a step, a fight, a kill, a door — because a trigger that
// needs new bookkeeping is a trigger that will quietly stop firing the next
// time somebody rearranges the scene it lived in.
type Trigger string

const (
	// Travel counts steps taken together on the continent.
	Travel Trigger = "travel"
	// Fights counts battles the company comes out of.
	Fights Trigger = "fights"
	// Kills counts the cast creature, specifically, put down.
	Kills Trigger = "kills"
	// Reach fires on arriving at the cast place.
	Reach Trigger = "reach"
	// Town fires on walking into any settlement.
	Town Trigger = "town"
)

// Counted reports whether a trigger accumulates rather than simply happening.
func (t Trigger) Counted() bool { return t == Travel || t == Fights || t == Kills }

// Beat is one step of a thread: what has to happen, and what gets said when it
// does.
type Beat struct {
	Trigger Trigger `json:"trigger"`
	// Need is how many, for the counted triggers. Zero reads as one. When the
	// writing states the number out loud — "at three they stop counting" — the
	// two have to be changed together; nothing checks that for you.
	Need int `json:"need,omitempty"`
	// Text is what the companion says when the beat comes due.
	Text string `json:"text"`
	// Note is the journal's one-line summary of what you are waiting on. It is
	// written from the player's side: "walk with them" rather than "travel 90".
	Note string `json:"note"`
}

// Ending is one way the last beat can go.
//
// Every thread offers a choice rather than a payout, because a backstory that
// resolves itself while you watch is a cutscene. The choices are authored as
// trade-offs like everything else in the game: the one that pays is rarely the
// one that settles anything.
//
// Coins and XP are *per level of the companion*. A flat number would be a
// fortune at level one and a rounding error at twelve, and threads are cast at
// whatever level the company happened to be.
type Ending struct {
	Label string `json:"label"` // the menu option
	Text  string `json:"text"`  // what happens, in their voice or the world's
	Coins int64  `json:"coins,omitempty"`
	XP    int64  `json:"xp,omitempty"`
	// Cut adjusts the companion's share of every haul from here on, which is
	// the one lasting thing a thread can change about them.
	Cut   int `json:"cut,omitempty"`
	Fame  int `json:"fame,omitempty"`
	Shame int `json:"shame,omitempty"`
}

// Costs reports whether taking this ending needs money up front.
func (e Ending) Costs() int64 {
	if e.Coins < 0 {
		return -e.Coins
	}
	return 0
}

// Skeleton is an authored thread with nobody cast in it yet.
//
// The placeholders the writing may use are {N} the companion, {P} the place the
// thread ends at, {X} the creature it is about, and {I} something out there
// drops. Nothing else may be named: a skeleton that mentions a specific ruin
// would be a skeleton that breaks on the next seed.
//
// Which roles a skeleton needs is read out of its own writing rather than
// declared beside it, so adding "{X}" to a line is the whole of adding an
// antagonist. See castRoles.
type Skeleton struct {
	ID string `json:"id"`
	// Blood restricts the skeleton to one ancestry, as a model.MonsterKind
	// string. Empty means anybody, which is what the ordinary hirelings draw
	// from.
	Blood string `json:"blood,omitempty"`
	Title string `json:"title"`
	// Place is the sort of location {P} has to be: "delve" for a dungeon, cave
	// or ruin, "settlement" for somewhere with a roof. Ignored when the writing
	// never mentions a place.
	Place   string   `json:"place,omitempty"`
	Beats   []Beat   `json:"beats"`
	Endings []Ending `json:"endings"`
}

// Book is the loaded set of skeletons.
type Book struct {
	Threads []Skeleton `json:"threads"`
}

// Get finds a skeleton by ID.
func (b *Book) Get(id string) (*Skeleton, bool) {
	if b == nil {
		return nil, false
	}
	for i := range b.Threads {
		if b.Threads[i].ID == id {
			return &b.Threads[i], true
		}
	}
	return nil, false
}

// For lists the skeletons somebody of this ancestry could be cast in: the ones
// written for their lineage, plus the ones written for anybody.
func (b *Book) For(blood model.MonsterKind) []*Skeleton {
	if b == nil {
		return nil
	}
	var out []*Skeleton
	for i := range b.Threads {
		s := &b.Threads[i]
		if s.Blood == "" || s.Blood == string(blood) {
			out = append(out, s)
		}
	}
	return out
}

// State is where a thread has got to.
type State string

const (
	// Open: still working through the beats.
	Open State = "open"
	// Ready: the last beat has fired and the choice is waiting to be made.
	Ready State = "ready"
	// Closed: resolved, one way or the other.
	Closed State = "closed"
)

// Thread is one companion's backstory, cast and in progress.
//
// Everything pointing at the world is an index or a name, exactly like a quest,
// so the whole thing serialises to a few lines of JSON and survives a save
// without needing the continent to be there when it is read back.
type Thread struct {
	Skeleton string `json:"skeleton"`
	// Owner is the companion's name. Names are made unique within a company
	// when somebody is hired, so this is a usable key — and it is a good deal
	// friendlier to read in a save file than an index would be.
	Owner string `json:"owner"`
	Title string `json:"title"`
	State State  `json:"state"`

	// Roles are the filled placeholders: {P} the place, {I} the thing, {X} the
	// antagonist, {N} the companion. Stored filled rather than re-derived, so
	// the story cannot change under a save.
	Roles map[string]string `json:"roles,omitempty"`

	// PlacePOI and MonsterID are the machine-readable halves of {P} and {X},
	// which is what the triggers actually compare against. PlacePOI is -1 for a
	// thread that ends nowhere in particular, and is written out even so:
	// omitting it would make "no place" and "the first location on the map"
	// the same three absent characters in a hand-edited save.
	PlacePOI  int    `json:"placePOI"`
	MonsterID string `json:"monsterID,omitempty"`

	At   int `json:"at"`   // the beat being waited on
	Have int `json:"have"` // progress within it

	// Ended records which choice was taken, for the journal.
	Ended string `json:"ended,omitempty"`
}

// Catalog is the slice of the content tables casting needs. Taking an interface
// rather than the tables keeps this package independent of how content is
// loaded, and lets a test cast a thread without touching the disk.
type Catalog interface {
	BiomeDrops(biome string) []string
	BiomeMonsters(biome string, level int) []*model.MonsterDef
}

// castRange is how far away a thread will stage its resolution. Far enough to
// be somewhere you have to go on purpose, close enough that it is not the whole
// continent.
const castRange = 70

// Cast fills a skeleton with things this world actually contains, and reports
// false when it cannot — a continent with no ruin on it simply does not offer
// the threads that need one, in the same way the quest generator declines to
// invent a delve.
//
// taken lists the skeleton IDs already running, so a company of three is three
// different stories.
func Cast(g *core.RNG, b *Book, w *world.Map, cat Catalog, owner *model.Character, from core.Point, taken []string) (*Thread, bool) {
	if g == nil || b == nil || w == nil || owner == nil {
		return nil, false
	}
	busy := map[string]bool{}
	for _, id := range taken {
		busy[id] = true
	}

	// Every candidate is cast first and one of the results is chosen, rather
	// than choosing a skeleton and then trying to fill it. Picking first and
	// then discovering that the continent has no ruin on it would mean either a
	// broken thread or a retry loop that quietly favours the easy skeletons.
	type candidate struct {
		s *Skeleton
		c cast
	}
	var usable, ownBlood []candidate
	for _, s := range b.For(owner.Blood) {
		if busy[s.ID] || len(s.Beats) == 0 || len(s.Endings) == 0 {
			continue
		}
		c, ok := castRoles(g, s, w, cat, owner, from)
		if !ok {
			continue
		}
		usable = append(usable, candidate{s, c})
		if s.Blood != "" {
			ownBlood = append(ownBlood, candidate{s, c})
		}
	}
	if len(usable) == 0 {
		return nil, false
	}
	// A story written for this ancestry wins when there is one, because it is
	// the reason the ancestry is in the game: the whole pitch of a part-demon
	// is the arrangement they will not discuss. Hiring a second part-demon
	// falls through to the general pool on its own, since taken already rules
	// the lineage thread out.
	if len(ownBlood) > 0 {
		usable = ownBlood
	}

	got := core.Pick(g, usable)
	return &Thread{
		Skeleton:  got.s.ID,
		Owner:     owner.Name,
		Title:     got.s.Title,
		State:     Open,
		Roles:     got.c.text,
		PlacePOI:  got.c.placePOI,
		MonsterID: got.c.monsterID,
	}, true
}

// cast is the filled roles, in both the readable and the comparable form.
type cast struct {
	text      map[string]string
	placePOI  int
	monsterID string
}

// castRoles fills every placeholder a skeleton mentions.
//
// The requirements are read out of the writing rather than declared alongside
// it. An author who adds "{X}" to a line has just said the thread needs an
// antagonist, and there is no second list to forget to update.
func castRoles(g *core.RNG, s *Skeleton, w *world.Map, cat Catalog, owner *model.Character, from core.Point) (cast, bool) {
	c := cast{text: map[string]string{"{N}": owner.Name}, placePOI: -1}
	body := skeletonText(s)

	needPlace := strings.Contains(body, "{P}") || usesTrigger(s, Reach)
	if needPlace {
		p, idx, ok := pickPlace(g, s.Place, w, from)
		if !ok {
			return cast{}, false
		}
		c.text["{P}"] = p.Name
		c.placePOI = idx
	}

	// The thing and the antagonist come from the biome around wherever the
	// company is standing *now*, not around the place the thread ends at.
	//
	// That is the opposite of the obvious choice and it is deliberate. The
	// counted beats fire early, in the stretch before the thread has even
	// mentioned a destination — so a creature cast from the far end would be
	// one the player has no reason to be anywhere near yet, and a thread could
	// sit on "put down three of them" for the rest of a run. What is outside
	// the town they were hired in is what the company is about to fight.
	biome := biomeAround(w, from)

	if strings.Contains(body, "{I}") {
		drops := cat.BiomeDrops(biome)
		if len(drops) == 0 {
			return cast{}, false
		}
		c.text["{I}"] = core.Pick(g, drops)
	}

	if strings.Contains(body, "{X}") || usesTrigger(s, Kills) {
		mons := cat.BiomeMonsters(biome, core.Max(1, owner.Level))
		if len(mons) == 0 {
			return cast{}, false
		}
		m := core.Pick(g, mons)
		c.text["{X}"] = m.Name
		c.monsterID = m.ID
	}
	return c, true
}

// skeletonText is every word of a skeleton, for scanning.
func skeletonText(s *Skeleton) string {
	var b strings.Builder
	b.WriteString(s.Title)
	for _, beat := range s.Beats {
		b.WriteString(beat.Text)
		b.WriteString(beat.Note)
	}
	for _, e := range s.Endings {
		b.WriteString(e.Label)
		b.WriteString(e.Text)
	}
	return b.String()
}

func usesTrigger(s *Skeleton, want Trigger) bool {
	for _, b := range s.Beats {
		if b.Trigger == want {
			return true
		}
	}
	return false
}

// pickPlace finds somewhere of the right sort within reach, preferring
// somewhere the player has not already been. A resolution staged in the town
// you are standing in is not a journey.
func pickPlace(g *core.RNG, kind string, w *world.Map, from core.Point) (*world.POI, int, bool) {
	var near, far []int
	for i, p := range w.POIs {
		if !placeMatches(kind, p) {
			continue
		}
		d := p.Pos.Manhattan(from)
		if d == 0 || d > castRange {
			continue
		}
		if p.Visited {
			far = append(far, i)
			continue
		}
		near = append(near, i)
	}
	pool := near
	if len(pool) == 0 {
		pool = far
	}
	if len(pool) == 0 {
		return nil, -1, false
	}
	idx := core.Pick(g, pool)
	return w.POIs[idx], idx, true
}

func placeMatches(kind string, p *world.POI) bool {
	switch kind {
	case "settlement":
		return p.Kind.Settlement()
	default: // "delve" and anything unrecognised
		return p.Kind == world.KindDungeon || p.Kind == world.KindCave || p.Kind == world.KindRuin
	}
}

// biomeAround reports the dominant monster table near a point, which is what a
// thread about "whatever is out there" should actually mean.
func biomeAround(w *world.Map, at core.Point) string {
	counts := map[string]int{}
	for dy := -6; dy <= 6; dy++ {
		for dx := -6; dx <= 6; dx++ {
			t := w.At(at.X+dx, at.Y+dy)
			if !t.Passable() {
				continue
			}
			counts[t.Biome()]++
		}
	}
	// Sorted so a tie resolves the same way every time. Two biomes with equal
	// counts is common on a coast, and a thread that cast differently on each
	// load would be a thread that disagreed with its own save.
	best, bestN := "plains", 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, b := range keys {
		if counts[b] > bestN {
			best, bestN = b, counts[b]
		}
	}
	return best
}

// Fill substitutes a thread's cast into a line of its writing.
func (t *Thread) Fill(s string) string {
	for k, v := range t.Roles {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

// beat returns the beat currently being waited on.
func (t *Thread) beat(b *Book) (Beat, bool) {
	s, ok := b.Get(t.Skeleton)
	if !ok || t.At < 0 || t.At >= len(s.Beats) {
		return Beat{}, false
	}
	return s.Beats[t.At], true
}

// Awaiting reports the trigger the thread is currently waiting on, or an empty
// trigger when it is out of beats. It is how the game knows a thread is about
// to send the player somewhere before it says so out loud.
func (t *Thread) Awaiting(b *Book) Trigger {
	if t.State != Open {
		return ""
	}
	bt, ok := t.beat(b)
	if !ok {
		return ""
	}
	return bt.Trigger
}

// Need is how many of the current beat's trigger it is waiting for.
func (t *Thread) Need(b *Book) int {
	bt, ok := t.beat(b)
	if !ok {
		return 0
	}
	return core.Max(1, bt.Need)
}

// Note is the journal's line for where the thread has got to.
func (t *Thread) Note(b *Book) string {
	switch t.State {
	case Closed:
		return t.Ended
	case Ready:
		return "waiting on you"
	}
	bt, ok := t.beat(b)
	if !ok {
		return ""
	}
	return t.Fill(bt.Note)
}

// Progress renders the counter for the journal, empty for the beats that are
// not counted — "arrive at the ruin" has no halfway.
func (t *Thread) Progress(b *Book) string {
	if t.State != Open {
		return ""
	}
	bt, ok := t.beat(b)
	if !ok || !bt.Trigger.Counted() {
		return ""
	}
	return fmt.Sprintf("%d / %d", core.Min(t.Have, core.Max(1, bt.Need)), core.Max(1, bt.Need))
}

// Options lists the ways the thread can end, with the cast substituted in.
func (t *Thread) Options(b *Book) []Ending {
	s, ok := b.Get(t.Skeleton)
	if !ok {
		return nil
	}
	out := make([]Ending, len(s.Endings))
	for i, e := range s.Endings {
		e.Label = t.Fill(e.Label)
		e.Text = t.Fill(e.Text)
		out[i] = e
	}
	return out
}

// Resolve closes the thread on a chosen ending. Applying what the ending pays
// or costs is the caller's business, because the purse and the roster live
// somewhere this package deliberately cannot see.
func (t *Thread) Resolve(e Ending) {
	t.State = Closed
	t.Ended = e.Label
}

// --- progress -------------------------------------------------------------

// Log is every thread the company is carrying.
type Log struct {
	Threads []*Thread `json:"threads,omitempty"`
}

// Add records a newly cast thread.
func (l *Log) Add(t *Thread) { l.Threads = append(l.Threads, t) }

// For finds the thread belonging to a companion, if they have one.
func (l *Log) For(owner string) *Thread {
	for _, t := range l.Threads {
		if t.Owner == owner {
			return t
		}
	}
	return nil
}

// Drop forgets a departing companion's thread. Letting somebody go takes their
// story with them: it was never the player's, and leaving it in the journal
// would mean a beat that can never fire again sitting there forever.
func (l *Log) Drop(owner string) {
	out := l.Threads[:0]
	for _, t := range l.Threads {
		if t.Owner != owner {
			out = append(out, t)
		}
	}
	l.Threads = out
}

// Running lists the threads still going, in the order they were cast.
func (l *Log) Running() []*Thread {
	var out []*Thread
	for _, t := range l.Threads {
		if t.State != Closed {
			out = append(out, t)
		}
	}
	return out
}

// Waiting lists the threads whose last beat has fired and whose choice has not
// been made. A player who backs out of the box gets asked again rather than
// losing the ending.
func (l *Log) Waiting() []*Thread {
	var out []*Thread
	for _, t := range l.Threads {
		if t.State == Ready {
			out = append(out, t)
		}
	}
	return out
}

// IDs lists every skeleton this run has used, resolved ones included, which is
// what is handed to Cast to keep it off the table.
//
// Finished counts as used: the player has read those words, and hearing them
// again out of a different mouth is worse than the new hireling simply not
// having a story. A run that gets through the whole book stops handing them
// out, which is the graceful end of a finite amount of writing rather than a
// failure — Cast reporting false is a supported answer.
func (l *Log) IDs() []string {
	var out []string
	for _, t := range l.Threads {
		out = append(out, t.Skeleton)
	}
	return out
}

// Event is something that happened that a beat might be waiting for.
type Event struct {
	Kind Trigger
	// N is how much happened. Zero reads as one.
	N int
	// POI is the location, for Reach and Town.
	POI int
	// Monster is the creature's ID, for Kills.
	Monster string
}

// Fired is a beat that has just come due.
type Fired struct {
	Thread *Thread
	// Text is what the companion says, cast substituted.
	Text string
	// Last reports that the thread has run out of beats and is now waiting on
	// its ending.
	Last bool
}

// Advance applies an event to every running thread and returns the beats that
// came due.
//
// Only companions who are actually present should generate events, but that is
// the caller's judgement: this package has no way of knowing who was in the
// room, and guessing would be worse than asking.
func (l *Log) Advance(b *Book, ev Event) []Fired {
	var out []Fired
	n := core.Max(1, ev.N)

	for _, t := range l.Threads {
		if t.State != Open {
			continue
		}
		s, ok := b.Get(t.Skeleton)
		if !ok || t.At >= len(s.Beats) {
			continue
		}
		bt := s.Beats[t.At]
		if bt.Trigger != ev.Kind {
			continue
		}

		switch ev.Kind {
		case Reach:
			if ev.POI != t.PlacePOI {
				continue
			}
			t.Have = core.Max(1, bt.Need)
		case Kills:
			if ev.Monster != t.MonsterID {
				continue
			}
			t.Have += n
		case Town:
			t.Have = core.Max(1, bt.Need)
		default:
			t.Have += n
		}

		if t.Have < core.Max(1, bt.Need) {
			continue
		}

		f := Fired{Thread: t, Text: t.Fill(bt.Text)}
		t.At++
		t.Have = 0
		if t.At >= len(s.Beats) {
			t.State = Ready
			f.Last = true
		}
		out = append(out, f)
	}
	return out
}
