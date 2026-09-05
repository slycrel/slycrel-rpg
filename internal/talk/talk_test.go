package talk

import (
	"strings"
	"testing"
)

// Check has to catch the things that do not crash.
//
// A broken conversation is not a panic. It is an option that leads nowhere, a
// branch nobody can reach, or a condition that is never true — and all three
// look, in play, exactly like writing somebody has not finished. So each of
// them is provoked here, because a validator nobody has watched fail is a
// validator that returns nil.
func TestCheckCatchesTheThingsThatDoNotCrash(t *testing.T) {
	ok := func() *Book {
		return &Book{Trees: []*Tree{{
			ID: "t", Root: "root",
			Nodes: []*Node{
				{ID: "root", Text: []string{"hello"}, Options: []Option{
					{Label: "more", Goto: "more"},
					{Label: "bye"},
				}},
				{ID: "more", Text: []string{"more"}},
			},
		}}}
	}
	if err := ok().Check(); err != nil {
		t.Fatalf("a sound tree was rejected: %v", err)
	}

	for _, tc := range []struct {
		name   string
		break_ func(b *Book)
		want   string
	}{
		{"an answer that leads nowhere", func(b *Book) {
			b.Trees[0].Nodes[0].Options[0].Goto = "elsewhere"
		}, "does not exist"},
		{"a branch nobody can reach", func(b *Book) {
			b.Trees[0].Nodes[0].Options = b.Trees[0].Nodes[0].Options[1:]
		}, "nothing leads here"},
		{"nowhere to start", func(b *Book) {
			b.Trees[0].Root = "opening"
		}, "to start at"},
		{"somebody who says nothing", func(b *Book) {
			b.Trees[0].Nodes[1].Text = nil
		}, "says nothing"},
		{"a condition that is not one", func(b *Book) {
			b.Trees[0].Nodes[0].Options[0].Need = "when-the-moon-is-right"
		}, "which is not a condition"},
		{"an option with no words on it", func(b *Book) {
			b.Trees[0].Nodes[0].Options[0].Label = ""
		}, "no words on it"},
		{"two nodes with one name", func(b *Book) {
			b.Trees[0].Nodes = append(b.Trees[0].Nodes, &Node{ID: "more", Text: []string{"again"}})
		}, "two nodes called"},
		{"two trees with one id", func(b *Book) {
			b.Trees = append(b.Trees, &Tree{ID: "t", Root: "root",
				Nodes: []*Node{{ID: "root", Text: []string{"hi"}}}})
		}, "two trees with this id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := ok()
			tc.break_(b)
			err := b.Check()
			if err == nil {
				t.Fatalf("%s loaded without complaint", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the complaint was %q, which does not mention %q", err, tc.want)
			}
		})
	}
}

// The need vocabulary is closed, and the count says so.
//
// The table and the constants are two lists of the same thing, and a need added
// to one and not the other is an option that never appears — which nobody
// notices, because nobody misses a conversation they were never offered.
func TestEveryNeedIsInTheTable(t *testing.T) {
	all := []Need{Always, Story, Waiting, Ending, NoStory, Hurt, Blood, Errand, Rich, Broke}
	if len(all) != needCount {
		t.Errorf("%d needs written down here, %d declared in the package", len(all), needCount)
	}
	for _, n := range all {
		if !needs[n] {
			t.Errorf("%q is a need the table does not have", n)
		}
	}
}
