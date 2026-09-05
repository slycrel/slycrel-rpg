// Package talk is the conversation trees.
//
// A message box has always been one thing said and then gone, which is the
// right shape for a signpost and the wrong one for somebody you have been
// walking beside for four hours. The difference a tree makes is not branching
// for its own sake — it is that the player gets to *ask*, and that what is
// available to ask about is a fact about the run rather than a fixed list.
//
// The split is the one threads already use and for the same reason: the words
// are authored and live in data/text/talk.json, and what is *true* comes from
// the game at the moment the box opens. A node says what somebody says; a
// Need says when the option to hear it exists. Nothing in here knows what a
// companion is.
//
// This is deliberately not a scripting language. There are no variables, no
// arithmetic and no side effects — an option is shown or it is not, and taking
// it moves to another node or ends the conversation. Everything a tree can ask
// about the world is in the Need vocabulary below, which is closed and checked
// at load. A tree that could ask arbitrary questions would be a second copy of
// the rules, which is the thing this codebase keeps writing down.
package talk

import (
	"fmt"
	"sort"
	"strings"
)

// Need is a condition on an option, drawn from a closed vocabulary.
//
// The list is short on purpose, exactly like thread.Trigger: every entry has to
// be something the game already knows without being asked twice, because a
// condition that needs new bookkeeping is a condition that will quietly stop
// being true the next time somebody rearranges what it reads.
type Need string

const (
	// Always is the empty need, which every option has unless it says
	// otherwise. Named rather than left blank so that the table below can be
	// exhaustive and the guard at the bottom of this file can mean something.
	Always Need = ""
	// Story is set when this person has a backstory that has begun.
	Story Need = "story"
	// Waiting is set when their story is open and waiting on something the
	// player can go and do.
	Waiting Need = "waiting"
	// Ending is set when their story has reached the choice at the end of it.
	Ending Need = "ending"
	// NoStory is set when they have none, which is most people most of the
	// time and is a different silence from having one and not mentioning it.
	NoStory Need = "nostory"
	// Hurt is set below half health.
	Hurt Need = "hurt"
	// Blood is set when they are not entirely human.
	Blood Need = "blood"
	// Errand is set when the party has an errand it is in the middle of.
	Errand Need = "errand"
	// Rich and Broke are the two ends of the purse, and they are here because
	// the one thing a hireling is contractually interested in is the money.
	Rich  Need = "rich"
	Broke Need = "broke"
)

// needs is every condition, which is what makes the vocabulary closed. A tree
// naming anything not in here fails to load rather than silently never showing
// the option — the failure that would otherwise be invisible is the option that
// never appears, and nobody notices a conversation they were never offered.
var needs = map[Need]bool{
	Always: true, Story: true, Waiting: true, Ending: true, NoStory: true,
	Hurt: true, Blood: true, Errand: true, Rich: true, Broke: true,
}

// needCount is how many there are, so that adding one to the constants above
// and forgetting the map is a build-time failure rather than a condition that
// is never true.
const needCount = 10

func init() {
	if len(needs) != needCount {
		panic(fmt.Sprintf("talk: %d needs in the table, %d declared", len(needs), needCount))
	}
}

// Option is one thing the player can say.
type Option struct {
	// Label is the menu row. Kept short: it is what the player says, not what
	// they mean, and a row that runs to two lines is a paragraph with a cursor
	// next to it.
	Label string `json:"label"`
	// Goto is the node this leads to. Empty ends the conversation, which is
	// what "Nothing." is for and is a real answer rather than a missing one.
	Goto string `json:"goto,omitempty"`
	// Need is when this option exists at all.
	Need Need `json:"need,omitempty"`
	// Once marks an option that is spent by taking it, for the beats of a
	// conversation that are news the first time and a repeat afterwards.
	Once bool `json:"once,omitempty"`
}

// Node is one thing said and the ways to answer it.
type Node struct {
	ID string `json:"id"`
	// Text is what is said, as variants. One is picked, so that the second time
	// a player asks the same question they are not read the same sentence — the
	// cheapest anti-staleness there is, and the only one that costs nothing but
	// writing.
	Text []string `json:"text"`
	// Options are the answers. A node with none is the end of the branch and
	// the box closes after it.
	Options []Option `json:"options"`
}

// Tree is a whole conversation.
type Tree struct {
	ID    string  `json:"id"`
	Root  string  `json:"root"`
	Nodes []*Node `json:"nodes"`

	byID map[string]*Node
}

// Book is the file: every tree in the game, by id.
type Book struct {
	Trees []*Tree `json:"trees"`
}

// Tree returns a conversation by id.
func (b *Book) Tree(id string) (*Tree, bool) {
	for _, t := range b.Trees {
		if t.ID == id {
			return t, true
		}
	}
	return nil, false
}

// Node returns a node by id.
//
// The index is built on first use rather than at load, because Book arrives
// through encoding/json and a field nobody set is a field nobody can rely on.
func (t *Tree) Node(id string) (*Node, bool) {
	if t.byID == nil {
		t.byID = make(map[string]*Node, len(t.Nodes))
		for _, n := range t.Nodes {
			t.byID[n.ID] = n
		}
	}
	n, ok := t.byID[id]
	return n, ok
}

// Check reports everything wrong with a book, as one error naming all of it.
//
// Called at load rather than left to a test, because the failure a broken tree
// produces in play is not a crash — it is an option that leads nowhere, or a
// branch nobody can reach, and both look exactly like writing that has not been
// finished yet. A conversation that cannot be walked should refuse to load the
// way a monster with no name refuses to load.
func (b *Book) Check() error {
	var bad []string
	seen := map[string]bool{}
	for _, t := range b.Trees {
		if t.ID == "" {
			bad = append(bad, "a tree with no id")
			continue
		}
		if seen[t.ID] {
			bad = append(bad, fmt.Sprintf("%s: two trees with this id", t.ID))
		}
		seen[t.ID] = true
		bad = append(bad, t.check()...)
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return fmt.Errorf("talk: %s", strings.Join(bad, "; "))
}

func (t *Tree) check() []string {
	var bad []string
	ids := map[string]bool{}
	for _, n := range t.Nodes {
		if n.ID == "" {
			bad = append(bad, fmt.Sprintf("%s: a node with no id", t.ID))
			continue
		}
		if ids[n.ID] {
			bad = append(bad, fmt.Sprintf("%s: two nodes called %q", t.ID, n.ID))
		}
		ids[n.ID] = true
		if len(n.Text) == 0 {
			bad = append(bad, fmt.Sprintf("%s/%s: says nothing", t.ID, n.ID))
		}
	}
	root := t.Root
	if root == "" {
		root = "root"
	}
	if !ids[root] {
		bad = append(bad, fmt.Sprintf("%s: no node called %q to start at", t.ID, root))
	}

	// Every answer goes somewhere that exists, and every need is one the game
	// can answer.
	reached := map[string]bool{root: true}
	for _, n := range t.Nodes {
		for _, o := range n.Options {
			if o.Label == "" {
				bad = append(bad, fmt.Sprintf("%s/%s: an option with no words on it", t.ID, n.ID))
			}
			if !needs[o.Need] {
				bad = append(bad, fmt.Sprintf("%s/%s: %q needs %q, which is not a condition",
					t.ID, n.ID, o.Label, o.Need))
			}
			if o.Goto == "" {
				continue
			}
			if !ids[o.Goto] {
				bad = append(bad, fmt.Sprintf("%s/%s: %q leads to %q, which does not exist",
					t.ID, n.ID, o.Label, o.Goto))
			}
			reached[o.Goto] = true
		}
	}
	for _, n := range t.Nodes {
		if n.ID != "" && !reached[n.ID] {
			bad = append(bad, fmt.Sprintf("%s/%s: nothing leads here", t.ID, n.ID))
		}
	}
	return bad
}

// Start is the node a conversation opens on.
func (t *Tree) Start() (*Node, bool) {
	root := t.Root
	if root == "" {
		root = "root"
	}
	return t.Node(root)
}
