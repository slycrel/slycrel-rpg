package quest_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

type stubNamer struct{}

func (stubNamer) PlaceName(*core.RNG, string) string    { return "Placename" }
func (stubNamer) PlaceTag(*core.RNG, string) string     { return "tag" }
func (stubNamer) PersonName(*core.RNG) string           { return "Person" }
func (stubNamer) NPCLine(*core.RNG) string              { return "line" }
func (stubNamer) SignText(*core.RNG) string             { return "sign" }
func (stubNamer) RecruitPitch(*core.RNG, string) string { return "pitch" }

type stubWriter struct{}

func (stubWriter) QuestLine(_ *core.RNG, kind, part string) string { return kind + "/" + part }

func tables(t *testing.T) *gamedata.Tables {
	t.Helper()
	root, err := gamedata.FindRoot()
	if err != nil {
		t.Fatalf("finding repo root: %v", err)
	}
	tb, err := gamedata.Load(root)
	if err != nil {
		t.Fatalf("loading content: %v", err)
	}
	return tb
}

// TestGeneratedQuestsAreCompletable is the property that matters most: an
// errand that cannot be finished is worse than no errand, and every reference
// a quest holds is an index or a name into content that must really exist.
func TestGeneratedQuestsAreCompletable(t *testing.T) {
	tb := tables(t)
	made := map[quest.Kind]int{}

	for _, seed := range []int64{1, 7, 1994, 20260815} {
		w := world.Generate(seed, stubNamer{})
		g := core.NewRNG(seed)

		for i, p := range w.POIs {
			if !p.Kind.Settlement() {
				continue
			}
			for try := 0; try < 12; try++ {
				q, ok := quest.Generate(g, w, tb, stubWriter{}, i, "Giver")
				if !ok {
					continue
				}
				made[q.Kind]++

				if q.Need <= 0 {
					t.Errorf("%s quest asks for %d of something", q.Kind, q.Need)
				}
				if q.RewardCoins <= 0 || q.RewardXP <= 0 {
					t.Errorf("%s quest pays %d coins and %d xp", q.Kind, q.RewardCoins, q.RewardXP)
				}
				if q.GiverPOI != i {
					t.Errorf("quest records giver location %d, was given at %d", q.GiverPOI, i)
				}

				switch q.Kind {
				case quest.Fetch:
					if _, ok := tb.Item(q.Item); !ok {
						t.Errorf("fetch quest wants %q, which is not an item", q.Item)
					}
				case quest.Cull:
					if _, ok := tb.ByID[q.MonsterID]; !ok {
						t.Errorf("cull quest names monster %q, which does not exist", q.MonsterID)
					}
				case quest.Delve, quest.Deliver:
					if q.TargetPOI < 0 || q.TargetPOI >= len(w.POIs) {
						t.Errorf("%s quest points at location %d of %d", q.Kind, q.TargetPOI, len(w.POIs))
						continue
					}
					if q.TargetPOI == i {
						t.Errorf("%s quest sends the player to where they already are", q.Kind)
					}
					if got := w.POIs[q.TargetPOI].Name; got != q.TargetName {
						t.Errorf("quest target name %q does not match location %q", q.TargetName, got)
					}
					if q.Kind == quest.Deliver && !w.POIs[q.TargetPOI].Kind.Settlement() {
						t.Errorf("delivery addressed to a %s", w.POIs[q.TargetPOI].Kind)
					}
				}
			}
		}
	}

	// All four shapes should turn up across a spread of worlds; one that never
	// generates is a rule that silently excludes itself.
	for _, k := range []quest.Kind{quest.Fetch, quest.Cull, quest.Delve, quest.Deliver} {
		if made[k] == 0 {
			t.Errorf("no %s quest was ever generated", k)
		}
	}
}

// TestFetchIsRecountedNotIncremented: items can be sold, drunk or handed over,
// so a counter that only goes up would let a player turn in things they no
// longer have.
func TestFetchIsRecountedNotIncremented(t *testing.T) {
	l := &quest.Log{}
	q := &quest.Quest{
		Kind: quest.Fetch, State: quest.Active, Item: "Rank Pelt", Need: 3,
	}
	l.Add(q)

	l.SyncFetch([]model.Item{{Name: "Rank Pelt", Count: 3}})
	if !q.Complete() {
		t.Fatalf("three of three is %s, expected complete", q.Progress())
	}

	// Sell two of them.
	l.SyncFetch([]model.Item{{Name: "Rank Pelt", Count: 1}})
	if q.Complete() {
		t.Errorf("quest still reads complete after the goods were sold: %s", q.Progress())
	}
	if q.Have != 1 {
		t.Errorf("recount says %d, bag holds 1", q.Have)
	}

	// And an empty bag.
	l.SyncFetch(nil)
	if q.Have != 0 {
		t.Errorf("recount says %d with an empty bag", q.Have)
	}
}

func TestProgressHooksOnlyAdvanceTheirOwnKind(t *testing.T) {
	l := &quest.Log{}
	cull := &quest.Quest{Kind: quest.Cull, State: quest.Active, MonsterID: "forest_goblin", Need: 2}
	delve := &quest.Quest{Kind: quest.Delve, State: quest.Active, TargetPOI: 4, Need: 1}
	deliver := &quest.Quest{Kind: quest.Deliver, State: quest.Active, TargetPOI: 9, Need: 1}
	l.Add(cull)
	l.Add(delve)
	l.Add(deliver)

	if done := l.OnMonsterKilled("plains_crow"); len(done) != 0 || cull.Have != 0 {
		t.Error("killing the wrong monster advanced a cull quest")
	}
	l.OnMonsterKilled("forest_goblin")
	if done := l.OnMonsterKilled("forest_goblin"); len(done) != 1 {
		t.Errorf("cull quest did not report completion at %d of %d", cull.Have, cull.Need)
	}

	if done := l.OnPOICleared(5); len(done) != 0 {
		t.Error("clearing the wrong location completed a delve quest")
	}
	if done := l.OnPOICleared(4); len(done) != 1 {
		t.Error("clearing the target did not complete the delve quest")
	}
	if done := l.OnEnteredPOI(9); len(done) != 1 {
		t.Error("arriving at the target did not complete the delivery")
	}
	// Delve and deliver must not have advanced each other.
	if delve.TargetPOI == deliver.TargetPOI {
		t.Fatal("test setup: targets collide")
	}
}

// TestOneErrandPerSettlement keeps a town from becoming a queue.
func TestOneErrandPerSettlement(t *testing.T) {
	l := &quest.Log{}
	l.Add(&quest.Quest{Kind: quest.Cull, State: quest.Active, GiverPOI: 3, Need: 1})
	if !l.HasFrom(3) {
		t.Error("location 3 has an outstanding errand but HasFrom says otherwise")
	}
	if l.HasFrom(4) {
		t.Error("location 4 has no errand but HasFrom says it does")
	}

	l.Quests[0].Have = 1
	if got := l.ReadyAt(3); len(got) != 1 {
		t.Error("a finished errand is not offered for hand-in where it was taken")
	}
	l.Close(l.Quests[0])
	if l.HasFrom(3) || l.CountActive() != 0 {
		t.Error("a closed errand is still counted as outstanding")
	}
}

// An errand is a thing one particular person asked you for, and the person is
// half of it. Looking up by settlement alone meant every townsperson recited
// the nag line for a job they had not given you — so after taking one, the
// whole town had nothing else to say — and every townsperson would accept the
// hand-in, so it never sent you back to anybody.
func TestAnErrandBelongsToWhoeverAskedForIt(t *testing.T) {
	l := &quest.Log{}
	mine := &quest.Quest{Kind: quest.Cull, State: quest.Active, GiverPOI: 3, Giver: "Ilsabet", Need: 2}
	l.Add(mine)

	if got := l.From(3, "Ilsabet"); got != mine {
		t.Error("the person who asked cannot find their own errand")
	}
	if got := l.From(3, "Somebody Else"); got != nil {
		t.Errorf("a bystander in the same town is holding %q", got.Title)
	}
	if got := l.From(4, "Ilsabet"); got != nil {
		t.Error("the same name in a different town matched")
	}

	// The settlement-level question still has to work, because it is what caps
	// a town at one errand.
	if !l.HasFrom(3) {
		t.Error("the town with an outstanding errand does not report one")
	}
	l.Close(mine)
	if l.From(3, "Ilsabet") != nil || l.HasFrom(3) {
		t.Error("a closed errand is still being offered")
	}
}
