package quest_test

import (
	"strings"
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// realQuests generates a spread of errands through the actual writer and the
// actual world, because the thing under test is the writing and a stub writer
// returns "fetch/ask".
func realQuests(t *testing.T) []*quest.Quest {
	t.Helper()
	tb := tables(t)
	w := content.New(&tb.Text)
	m := world.Generate(1994, w)
	g := core.NewRNG(1994)

	var out []*quest.Quest
	for i := range m.POIs {
		if !m.POIs[i].Kind.Settlement() {
			continue
		}
		for _, k := range []quest.Kind{quest.Fetch, quest.Cull, quest.Delve, quest.Deliver} {
			if q, ok := quest.Generate(g, m, tb, w, i, "Dregg", k); ok {
				out = append(out, q)
			}
		}
	}
	if len(out) < 8 {
		t.Fatalf("only %d errands generated; this is not a sample", len(out))
	}
	return out
}

// Every objective opens with a verb telling the player what to physically do.
//
// This is the rule the journal was breaking wholesale. It opened with the
// giver's nag — a line in character, which is a fine thing to have and is not
// an instruction — so a player coming back after a week read "Still 4 Chitin
// Scrap. The number has not changed. I would have mentioned." and had to
// reconstruct from it what they were meant to do, where, and how much was left.
//
// The verb list is closed on purpose. It is not that these five words are
// special; it is that a new errand kind has to add its verb here, which is a
// prompt to write an objective in the imperative rather than a description of
// a situation.
func TestEveryObjectiveOpensWithAnAction(t *testing.T) {
	verbs := []string{"Find ", "Kill ", "Travel ", "Carry ", "Go back "}
	for _, q := range realQuests(t) {
		for _, state := range []int{0, q.Need} {
			q.Have = state
			line := q.Objective()
			ok := false
			for _, v := range verbs {
				if strings.HasPrefix(line, v) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s at %d/%d: %q does not begin with an action",
					q.Kind, q.Have, q.Need, line)
			}
			if !strings.HasSuffix(line, ".") {
				t.Errorf("%s: %q is not a sentence", q.Kind, line)
			}
		}
	}
}

// An errand that happens in a region has to name the region.
//
// Fetch and cull point at no POI, so before Where existed they named no
// location of any kind: the ask said what to bring and the journal had no
// "Where" row at all. Half the errands in the game told the player to go and
// get four of something and never said where any of it was.
func TestAFetchOrACullSaysWhere(t *testing.T) {
	for _, q := range realQuests(t) {
		if q.Kind != quest.Fetch && q.Kind != quest.Cull {
			continue
		}
		if q.Where == "" {
			t.Errorf("%s from %s names no country to look in", q.Kind, q.GiverPlace)
			continue
		}
		if !strings.Contains(q.Objective(), q.Where) {
			t.Errorf("%s knows it happens in %q and the objective %q does not say so",
				q.Kind, q.Where, q.Objective())
		}
		// And the region has to be tied to somewhere real, not floated.
		if q.GiverPlace != "" && !strings.Contains(q.Where, q.GiverPlace) {
			t.Errorf("%q does not name the settlement it is outside of", q.Where)
		}
	}
}

// Prose says the species; labels say the whole name.
//
// CLAUDE.md has had this rule since the battle transcript learned it, and the
// quest generator never did — it interpolated the full name into flowing
// sentences, so a cull errand read "There's Wolf, Deeply Unimpressed out
// there". Sixty-eight of the seventy-nine creatures in the game carry a comma,
// so this was not an edge case, it was the common one.
func TestProseSaysTheSpeciesAndLabelsSayTheName(t *testing.T) {
	for _, q := range realQuests(t) {
		if q.MonsterName == "" || !strings.Contains(q.MonsterName, ",") {
			continue
		}
		for _, tc := range []struct{ what, line string }{
			{"ask", q.Ask},
			{"nag", q.NagLine()},
			{"thank", q.Thank},
			{"objective", q.Objective()},
		} {
			if strings.Contains(tc.line, q.MonsterName) {
				t.Errorf("the %s puts the whole name %q inside a sentence: %q",
					tc.what, q.MonsterName, tc.line)
			}
		}
		// The label is where the epithet belongs, and it must still be there —
		// otherwise this rule has been satisfied by throwing the joke away.
		if !strings.Contains(q.Title, q.MonsterName) {
			t.Errorf("the title %q has lost the epithet from %q", q.Title, q.MonsterName)
		}
	}
}

// A nag counts what is left, and it has to keep counting.
//
// The nag was filled at generation and quoted Need, so somebody holding three
// of four was told "Still 4 Chitin Scrap. The number has not changed. I would
// have mentioned." It had changed. It had changed three times.
func TestTheNagCountsDown(t *testing.T) {
	counted := 0
	for _, q := range realQuests(t) {
		if q.Kind != quest.Fetch && q.Kind != quest.Cull {
			continue
		}
		q.Have = 0
		first := q.NagLine()
		q.Have = q.Need - 1
		later := q.NagLine()
		if first == later {
			t.Errorf("%s: the nag reads the same at 0/%d and %d/%d: %q",
				q.Kind, q.Need, q.Need-1, q.Need, first)
			continue
		}
		if !strings.Contains(later, "1") {
			t.Errorf("%s: one left and the nag says %q", q.Kind, later)
		}
		counted++
	}
	if counted == 0 {
		t.Fatal("no counted errands in the sample, so nothing was checked")
	}
}

// Nothing reaches the player with a placeholder still in it.
func TestNoLineKeepsItsPlaceholders(t *testing.T) {
	for _, q := range realQuests(t) {
		for _, state := range []int{0, q.Need} {
			q.Have = state
			for _, line := range []string{q.Title, q.Ask, q.NagLine(), q.Thank, q.Objective(), q.Where} {
				if i := strings.IndexByte(line, '{'); i >= 0 {
					t.Errorf("%s: %q still holds a placeholder", q.Kind, line)
				}
			}
		}
	}
}

// A title leads with what the errand is, not with how much of it there is.
//
// "3 x Chitin Scrap" named a quantity of a thing and no action at all, and the
// journal row beside it already shows the count — so the title was spending its
// only line on the one fact that was already on screen.
func TestATitleLeadsWithTheJob(t *testing.T) {
	for _, q := range realQuests(t) {
		if q.Title == "" {
			t.Errorf("%s has no title", q.Kind)
			continue
		}
		if c := q.Title[0]; c >= '0' && c <= '9' {
			t.Errorf("the title %q opens with a number rather than a verb", q.Title)
		}
	}
}

// An old save has none of this and must still produce whole sentences.
//
// Where and GiverPlace were added today, so every quest in every file written
// before today unmarshals with both empty — which is the case the zero value
// was chosen for, and the case nothing else in this file exercises.
func TestAnErrandThatDoesNotKnowWhereStillReads(t *testing.T) {
	q := &quest.Quest{
		Kind: quest.Fetch, State: quest.Active, Giver: "Dregg",
		Item: "Chitin Scrap", Need: 4,
	}
	for _, state := range []int{0, 4} {
		q.Have = state
		line := q.Objective()
		if strings.Contains(line, " in .") || strings.Contains(line, "  ") ||
			strings.HasSuffix(line, " .") {
			t.Errorf("an errand with no place produced %q", line)
		}
		if !strings.HasSuffix(line, ".") {
			t.Errorf("an errand with no place produced %q, which is not a sentence", line)
		}
	}
}

// A made destination is the same errand, said the same way.
//
// A delivery to a town and a delivery to a crossroads have to read alike — the
// objective is still a verb and a named place, the title still leads with the
// job, and nothing may leak the fact that one of the two places is a fiction
// this errand is carrying around with it.
func TestAnInventedDestinationReadsLikeARealOne(t *testing.T) {
	made := 0
	for _, q := range realQuests(t) {
		if !q.Made {
			continue
		}
		made++
		if q.TargetName == "" {
			t.Fatal("a made destination has no name")
		}
		if !strings.Contains(q.Objective(), q.TargetName) {
			t.Errorf("the objective %q does not name %q", q.Objective(), q.TargetName)
		}
		if !strings.Contains(q.Title, q.TargetName) {
			t.Errorf("the title %q does not name %q", q.Title, q.TargetName)
		}
		// And it is the same place every time it is asked for, which is the
		// whole of why it costs the save format nothing: the place is a
		// function of the quest, and the quest is already saved.
		if q.SiteSeed() != q.SiteSeed() {
			t.Error("a made destination does not have a stable seed")
		}
	}
	if made == 0 {
		t.Skip("this world generated no made destinations, so nothing was checked")
	}
	// Two errands must not invent the same place, or two people send you to one
	// crossroads and whichever you reach first answers both.
	seeds := map[int64]bool{}
	for _, q := range realQuests(t) {
		if !q.Made {
			continue
		}
		if seeds[q.SiteSeed()] {
			t.Errorf("two errands share a destination seed")
		}
		seeds[q.SiteSeed()] = true
	}
}
