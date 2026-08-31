package rules_test

import (
	"testing"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// mob is a creature that is whatever the test needs it to be.
func mob(def *model.MonsterDef, offense int) *model.Monster {
	return &model.Monster{Def: def, Name: def.Name, HP: 20, MaxHP: 20, Offense: offense}
}

func TestStacksGroupIdenticalCreaturesAndKeepFieldOrder(t *testing.T) {
	wolf := &model.MonsterDef{ID: "wolf", Name: "Wolf"}
	crab := &model.MonsterDef{ID: "crab", Name: "Crab"}

	// Not adjacent on purpose: what the player chooses between is kinds of
	// creature, not seating, so a field rolled wolf/crab/wolf is two slots.
	field := []*model.Monster{mob(wolf, 5), mob(crab, 5), mob(wolf, 5)}
	st := rules.Stacks(field)

	if len(st) != 2 {
		t.Fatalf("got %d stacks, want 2: %+v", len(st), st)
	}
	if got := st[0].At; len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Errorf("the wolves stacked as %v, want [0 2]", got)
	}
	if got := st[1].At; len(got) != 1 || got[0] != 1 {
		t.Errorf("the crab stacked as %v, want [1]", got)
	}
	// A stack takes the position of its first member, so the field still reads
	// in the order it was rolled.
	if st[0].Any() != 0 {
		t.Errorf("the first stack starts at %d, want 0", st[0].Any())
	}
}

// The rule that makes staggering safe: two creatures off one definition scaled
// differently are two kinds, and must not share a slot. An escort scales its
// caster and leaves its guards alone, so this is a shape the game actually
// rolls rather than a hypothetical.
func TestDifferentlyScaledCreaturesDoNotStack(t *testing.T) {
	wolf := &model.MonsterDef{ID: "wolf", Name: "Wolf"}
	field := []*model.Monster{mob(wolf, 5), mob(wolf, 9), mob(wolf, 5)}

	st := rules.Stacks(field)
	if len(st) != 2 {
		t.Fatalf("got %d stacks, want 2 — the scaled-up wolf is a different kind", len(st))
	}
	if got := st[0].At; len(got) != 2 {
		t.Errorf("the two ordinary wolves stacked as %v, want two of them", got)
	}
	if got := st[1].At; len(got) != 1 || got[0] != 1 {
		t.Errorf("the scaled wolf stacked as %v, want [1] alone", got)
	}

	// And hit points are not part of it, or nothing would ever stack: Spawn
	// jitters HP by an eighth.
	field[0].HP, field[0].MaxHP = 17, 17
	field[2].HP, field[2].MaxHP = 23, 23
	if len(rules.Stacks(field)) != 2 {
		t.Error("a difference in hit points split a stack; it must not")
	}
}

func TestTheQueueAdvancesAsItsFrontFalls(t *testing.T) {
	wolf := &model.MonsterDef{ID: "wolf", Name: "Wolf"}
	crab := &model.MonsterDef{ID: "crab", Name: "Crab"}
	field := []*model.Monster{mob(wolf, 5), mob(wolf, 5), mob(crab, 5)}
	st := rules.Stacks(field)

	if got := rules.Targets(field, st); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("targets are %v, want the front wolf and the crab", got)
	}
	if n := st[0].Standing(field); n != 2 {
		t.Errorf("the wolf stack has %d standing, want 2", n)
	}

	// Kill the front. The one behind steps up, and the slot is still a target.
	field[0].Dead = true
	if got := st[0].Front(field); got != 1 {
		t.Errorf("after the front fell the queue offers %d, want 1", got)
	}
	if n := st[0].Standing(field); n != 1 {
		t.Errorf("the wolf stack has %d standing, want 1", n)
	}
	if got := rules.Targets(field, st); len(got) != 2 || got[0] != 1 {
		t.Errorf("targets are %v, want the second wolf now leading", got)
	}

	// Kill the rest of it. The slot stops being a target but still knows who
	// to draw, so it goes dim rather than disappearing and reshuffling the
	// field under the player's cursor.
	field[1].Dead = true
	if got := st[0].Front(field); got != -1 {
		t.Errorf("a spent stack offers %d, want -1", got)
	}
	if got := st[0].Any(); got != 0 {
		t.Errorf("a spent stack draws %d, want its first member", got)
	}
	if got := rules.Targets(field, st); len(got) != 1 || got[0] != 2 {
		t.Errorf("targets are %v, want only the crab", got)
	}
}

func TestStackOfFindsWhereACreatureIsStanding(t *testing.T) {
	wolf := &model.MonsterDef{ID: "wolf", Name: "Wolf"}
	crab := &model.MonsterDef{ID: "crab", Name: "Crab"}
	field := []*model.Monster{mob(wolf, 5), mob(crab, 5), mob(wolf, 5)}
	st := rules.Stacks(field)

	for i, want := range []int{0, 1, 0} {
		if got := rules.StackOf(st, i); got != want {
			t.Errorf("creature %d is in slot %d, want %d", i, got, want)
		}
	}
	if got := rules.StackOf(st, 99); got != -1 {
		t.Errorf("a creature not on the field is in slot %d, want -1", got)
	}
}
