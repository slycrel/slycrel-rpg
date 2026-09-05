package game

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/talk"
)

// A line may only name a thing the option that reached it guarantees exists.
//
// This is the writing's half of the rule the rest of the game follows: never
// name something that might not exist. {S} is the last thing a companion's
// story said, and it is empty for somebody who has no story — so a node using
// it can only be reached under a Story need, or a player with no story hears a
// sentence with a hole in it. Same for {W}, which is what the story is waiting
// on and is empty unless it is waiting.
//
// Checked by walking the tree rather than by reading it: an option added later
// that reaches a gated node from an ungated one is exactly the mistake this
// exists to catch, and it is invisible in the file.
func TestNoLineNamesSomethingItsWayInDoesNotGuarantee(t *testing.T) {
	g := storyGame(t)
	tr, ok := g.Data.Talk.Tree("companion")
	if !ok {
		t.Fatal("there is no companion conversation to check")
	}

	// The tokens that are only true under some conditions, and which.
	guarded := map[string][]talk.Need{
		"{S}": {talk.Story},
		"{W}": {talk.Waiting, talk.Ending},
	}

	// How every node can be arrived at: the union of the needs on the options
	// leading to it, walked to a fixed point so that a chain of ungated hops
	// off a gated option keeps the guarantee.
	under := map[string]map[talk.Need]bool{tr.Root: {talk.Always: true}}
	for changed := true; changed; {
		changed = false
		for _, n := range tr.Nodes {
			here, seen := under[n.ID]
			if !seen {
				continue
			}
			for _, o := range n.Options {
				if o.Goto == "" {
					continue
				}
				to := under[o.Goto]
				if to == nil {
					to = map[talk.Need]bool{}
					under[o.Goto] = to
				}
				add := func(need talk.Need) {
					if !to[need] {
						to[need] = true
						changed = true
					}
				}
				if o.Need != talk.Always {
					add(o.Need)
					continue
				}
				// An ungated hop carries whatever got you this far.
				for need := range here {
					add(need)
				}
			}
		}
	}

	for _, n := range tr.Nodes {
		for _, line := range n.Text {
			for token, allowed := range guarded {
				if !strings.Contains(line, token) {
					continue
				}
				held := under[n.ID]
				got := false
				for _, need := range allowed {
					if held[need] {
						got = true
					}
				}
				if !got {
					t.Errorf("%s says %s, but can be reached without %v — the "+
						"player would be read a sentence with a hole in it",
						n.ID, token, allowed)
				}
			}
		}
	}
}

// Which options a person is offered is a fact about them.
func TestTheOptionsOfferedAreAFactAboutThePerson(t *testing.T) {
	g := storyGame(t)
	g.Player.Coins = 200

	whole := &model.Character{Name: "Bosk", Class: model.ClassFighter,
		Ally: true, HP: 30, MaxHP: 30, Cut: 12}
	bleeding := &model.Character{Name: "Wren", Class: model.ClassThief,
		Ally: true, HP: 4, MaxHP: 30, Cut: 9}
	odd := &model.Character{Name: "Nessa", Class: model.ClassMage,
		Ally: true, HP: 20, MaxHP: 20, Cut: 20, Blood: model.KindFey}

	for _, tc := range []struct {
		who  *model.Character
		need talk.Need
		want bool
	}{
		{whole, talk.Always, true},
		{whole, talk.Hurt, false},
		{bleeding, talk.Hurt, true},
		{whole, talk.Blood, false},
		{odd, talk.Blood, true},
		// Nobody here has a story, which is the state most people are in most
		// of the time and is a different silence from having one.
		{whole, talk.Story, false},
		{whole, talk.Waiting, false},
		{whole, talk.Ending, false},
		{whole, talk.NoStory, true},
		{whole, talk.Rich, false},
		{whole, talk.Broke, false},
	} {
		if got := g.needMet(tc.who, tc.need); got != tc.want {
			t.Errorf("%s meets %q = %v, want %v", tc.who.Name, tc.need, got, tc.want)
		}
	}

	// And a hurt companion is offered a line a whole one is not.
	hurtOpts := []talk.Option{{Label: "fine"}, {Label: "you're bleeding", Need: talk.Hurt}}
	if got := len(g.openTo(whole, hurtOpts)); got != 1 {
		t.Errorf("an unhurt companion was offered %d of 2 options", got)
	}
	if got := len(g.openTo(bleeding, hurtOpts)); got != 2 {
		t.Errorf("a bleeding companion was offered %d of 2 options", got)
	}
}

// Talking to somebody walks the tree and the box carries their face.
func TestATalkWalksTheTree(t *testing.T) {
	g := storyGame(t)
	g.Player.Coins = 200
	c := &model.Character{Name: "Nessa", Class: model.ClassMage, Ally: true,
		HP: 20, MaxHP: 20, Cut: 17, Blood: model.KindFey, Portrait: "face/x"}
	g.Allies = []*model.Character{c}

	g.talkToCompany()
	m, ok := g.Top().(*messageScene)
	if !ok {
		t.Fatal("T with one companion did not open a conversation")
	}
	if m.speaker != c.Name {
		t.Errorf("the box is titled %q, not %q", m.speaker, c.Name)
	}
	if m.role != "fey mage" {
		t.Errorf("the caption reads %q, not %q", m.role, "fey mage")
	}
	if len(m.choices) == 0 {
		t.Fatal("the opening node offered nothing to say")
	}

	// Walk to the one node that quotes a number, and check the number is
	// theirs. A cut printed from anywhere but the companion is the same bug
	// the hiring quote had.
	cut := -1
	for i, label := range m.choices {
		if strings.Contains(strings.ToLower(label), "cut") {
			cut = i
		}
	}
	if cut < 0 {
		t.Fatalf("no way to ask about the cut, from %v", m.choices)
	}
	g.demoChoose(cut)
	m, ok = g.Top().(*messageScene)
	if !ok {
		t.Fatal("asking about the cut said nothing")
	}
	if !strings.Contains(strings.Join(m.body, " "), "17%") {
		t.Errorf("they quoted %q, and their cut is 17%%", strings.Join(m.body, " "))
	}
	if strings.Contains(strings.Join(m.body, " "), "{") {
		t.Errorf("an unfilled token survived into the box: %q", strings.Join(m.body, " "))
	}
}

// Two companions get asked which, one does not.
func TestOneCompanionIsNotAskedWhich(t *testing.T) {
	g := storyGame(t)
	one := &model.Character{Name: "Nessa", Class: model.ClassMage, Ally: true, HP: 9, MaxHP: 9}
	two := &model.Character{Name: "Wren", Class: model.ClassThief, Ally: true, HP: 9, MaxHP: 9}

	g.Allies = []*model.Character{one}
	g.talkToCompany()
	if m := g.Top().(*messageScene); m.speaker != "Nessa" {
		t.Errorf("one companion opened a box titled %q", m.speaker)
	}
	g.dropOverlays()

	g.Allies = []*model.Character{one, two}
	g.talkToCompany()
	m, ok := g.Top().(*messageScene)
	if !ok {
		t.Fatal("two companions opened nothing")
	}
	if len(m.choices) != 2 {
		t.Errorf("two companions offered %d names", len(m.choices))
	}
	g.demoChoose(1)
	if m := g.Top().(*messageScene); m.speaker != "Wren" {
		t.Errorf("picking the second companion opened a box titled %q", m.speaker)
	}
}
