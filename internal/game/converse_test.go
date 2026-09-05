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

	// What every way into a node guarantees, which is an intersection and not a
	// union — and getting that backwards is how this test passed the first time
	// it was provoked. A conversation is a graph with cycles in it: almost every
	// branch offers a way back to the opening, so "reachable having passed a
	// story gate" is true of the opening itself. What has to hold is that
	// *every* path into a node passed the gate, so a node's guarantee is
	// narrowed by each way in rather than widened.
	//
	// Optimistic initialisation: the root guarantees nothing, everything else
	// starts out claiming everything, and each edge takes away what it cannot
	// promise until nothing moves.
	every := []talk.Need{talk.Story, talk.Waiting, talk.Ending, talk.NoStory,
		talk.Hurt, talk.Blood, talk.Errand, talk.Rich, talk.Broke}
	guar := map[string]map[talk.Need]bool{}
	for _, n := range tr.Nodes {
		guar[n.ID] = map[talk.Need]bool{}
		if n.ID == tr.Root {
			continue
		}
		for _, need := range every {
			guar[n.ID][need] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range tr.Nodes {
			for _, o := range n.Options {
				if o.Goto == "" {
					continue
				}
				via := map[talk.Need]bool{}
				for need := range guar[n.ID] {
					via[need] = true
				}
				if o.Need != talk.Always {
					via[o.Need] = true
				}
				for need := range guar[o.Goto] {
					if !via[need] {
						delete(guar[o.Goto], need)
						changed = true
					}
				}
			}
		}
	}

	for _, n := range tr.Nodes {
		// Every line the node can say, the per-lineage voices included: a voice
		// is an alternative to Text, not an exception to the rule about what a
		// line may name.
		lines := append([]string(nil), n.Text...)
		for _, v := range n.Voices {
			lines = append(lines, v...)
		}
		for _, line := range lines {
			for token, allowed := range guarded {
				if !strings.Contains(line, token) {
					continue
				}
				held, got := guar[n.ID], false
				for _, need := range allowed {
					if held[need] {
						got = true
					}
				}
				if !got {
					t.Errorf("%s says %s, but there is a way in that does not "+
						"pass %v -- the player would be read a sentence with a "+
						"hole in it", n.ID, token, allowed)
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

// Everybody answers as themselves.
//
// A company of three that all say the same sentence is one person with three
// portraits, which is the thing the lineages exist to stop — they already shift
// the stat line and unlock a technique, and until now the only place they
// showed up in a *conversation* was the caption under the face.
func TestALineageAnswersInItsOwnVoice(t *testing.T) {
	g := storyGame(t)
	tr, ok := g.Data.Talk.Tree("companion")
	if !ok {
		t.Fatal("there is no companion conversation")
	}

	// Every node with voices in it has one for every lineage, or none: a table
	// with four of six filled is two companions falling back to an answer
	// written for somebody else, and it looks exactly like the four being
	// deliberate.
	for _, n := range tr.Nodes {
		if len(n.Voices) == 0 {
			continue
		}
		for _, l := range model.Lineages {
			if len(n.Voices[string(l.Kind)]) == 0 {
				t.Errorf("%s has voices but none for %s, who will be handed "+
					"somebody else's answer", n.ID, l.Kind)
			}
		}
	}

	// And two different companions asked the same thing get different answers.
	node, ok := tr.Node("blood")
	if !ok {
		t.Fatal("there is nothing to ask about the blood")
	}
	seen := map[string]string{}
	for _, l := range model.Lineages {
		c := &model.Character{Name: "X", Class: model.ClassFighter, Blood: l.Kind}
		lines := voiceOf(c, node)
		if len(lines) == 0 {
			t.Fatalf("%s was given nothing to say", l.Kind)
		}
		if was, dup := seen[lines[0]]; dup {
			t.Errorf("%s and %s open with the same sentence", was, l.Kind)
		}
		seen[lines[0]] = string(l.Kind)
	}
	// Somebody ordinary falls through to the plain answer, which is right:
	// there is nothing to say about being a person in a special voice.
	plain := &model.Character{Name: "X", Class: model.ClassFighter}
	if got := voiceOf(plain, node); len(got) == 0 || got[0] != node.Text[0] {
		t.Error("an ordinary person did not get the ordinary answer")
	}
}
