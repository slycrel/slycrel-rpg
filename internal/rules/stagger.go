package rules

import "github.com/slycrel/slycrel-rpg/internal/model"

// Stack is a queue of identical creatures occupying one place on the field.
//
// Six wolves used to be six portraits across three columns, which is a layout
// that cannot fit its own names and a joke printed six times. They are one slot
// now, with a number on it, and only the one at the front can be hit — so a
// crowd is a queue rather than a menu. The ones behind still swing; what
// staggering takes away is the choice of which identical wolf to kill first,
// which was never a choice.
//
// One consequence is worth knowing before it is reported as a bug. A condition
// that lands on one creature lands on the *front* of a queue, and the slot
// draws the front's pips — so a stunned slot of three is one stunned wolf and
// two swinging ones, and the screen has no way to say "one of these is
// stunned" because it is drawing one portrait. Anything over the whole field
// reaches every member and the pips are then honest. The alternative is a
// per-member readout inside one slot, which is the crowded field this feature
// exists to un-crowd.
//
// It is a rule rather than a drawing trick, which is why it lives here and not
// beside the battle screen: the screen lays stacks out and the transcript names
// them, and two copies of "which of these are the same creature" would be two
// chances to disagree.
//
// The first draft of this paragraph also claimed the balance report needs it.
// It does not, and the reason is the good news about this whole feature:
// SimulateGroup has always attacked the first thing still standing, so the
// simulator was fighting a stacked field before there were stacks. Nothing in
// cmd/balance calls anything here. If that changes — a report that prices what
// a queue does to a crowd fight — this is where it would call.
type Stack struct {
	// At is the field indices of this stack's members, in the order they queue.
	At []int
}

// Stacks groups a field into queues of identical creatures.
//
// Order is preserved: a stack takes the position of its first member, so a
// field reads left to right the way it was rolled. Members do not have to be
// adjacent — an encounter that rolled wolf, goblin, wolf is a stack of two
// wolves and a stack of one goblin, in that order, because what the player is
// choosing between is kinds of creature and not seating.
func Stacks(mons []*model.Monster) []Stack {
	var out []Stack
	placed := make([]bool, len(mons))
	for i, m := range mons {
		if placed[i] {
			continue
		}
		s := Stack{At: []int{i}}
		placed[i] = true
		for j := i + 1; j < len(mons); j++ {
			if placed[j] || !model.SameKind(m, mons[j]) {
				continue
			}
			s.At = append(s.At, j)
			placed[j] = true
		}
		out = append(out, s)
	}
	return out
}

// Front is the field index of the creature at the head of the queue — the only
// one anything single-target may hit — or -1 when the whole stack is down.
func (s Stack) Front(mons []*model.Monster) int {
	for _, i := range s.At {
		if i >= 0 && i < len(mons) && !mons[i].Dead {
			return i
		}
	}
	return -1
}

// Standing is how many of this stack are still in the fight, which is the
// number the slot wears.
func (s Stack) Standing(mons []*model.Monster) int {
	n := 0
	for _, i := range s.At {
		if i >= 0 && i < len(mons) && !mons[i].Dead {
			n++
		}
	}
	return n
}

// Any is a member to draw when the whole stack is down, so a spent slot is a
// dim portrait rather than a hole in the field.
func (s Stack) Any() int {
	if len(s.At) == 0 {
		return -1
	}
	return s.At[0]
}

// StackOf is which stack a field index belongs to, for the callers that are
// holding a creature and need to know where it is standing — a floating damage
// number, a burst, a flash.
func StackOf(stacks []Stack, idx int) int {
	for s, st := range stacks {
		for _, i := range st.At {
			if i == idx {
				return s
			}
		}
	}
	return -1
}

// Targets is the field indices anything single-target may choose between: the
// front of every stack with somebody still standing in it.
//
// This is the whole of what staggering changes about the fight. Everything that
// hits the field at large — a spell over everybody, a sap — still reaches past
// it, because those never asked the player to point at anything.
func Targets(mons []*model.Monster, stacks []Stack) []int {
	var out []int
	for _, s := range stacks {
		if i := s.Front(mons); i >= 0 {
			out = append(out, i)
		}
	}
	return out
}
