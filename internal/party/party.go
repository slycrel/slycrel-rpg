// Package party is the company: who is in it, what that costs the encounter
// table, and how they walk in a line behind the person steering.
//
// It exists apart from the scenes because none of this is drawing. The roster
// rules and the marching order are the parts of the party feature most likely
// to be wrong — two bugs in them have been found by reading rather than by a
// test — and keeping them here means they can be exercised without a window to
// draw into, on a machine whose display has other ideas.
package party

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// MaxSize is the size of the company, hero included.
//
// Three is not arbitrary. The battle screen gives the party a 188x58 panel
// beside the command menu, which is three legible rows; a fourth would mean
// shrinking the meters to the point where you cannot tell at a glance who is
// about to fall over, and knowing that is the whole reason the panel exists.
const MaxSize = 3

// MaxFoes is the biggest group the battle screen can lay out legibly. Four
// portraits across 480 pixels leaves each one 56 wide with room for a name
// plate under it; a fifth would start truncating names to initials.
const MaxFoes = 4

// Members returns the hero followed by every companion, which is the order
// everything else — turn order, the panel, the experience split — reads them in.
func Members(hero *model.Character, allies []*model.Character) []*model.Character {
	out := make([]*model.Character, 0, 1+len(allies))
	if hero != nil {
		out = append(out, hero)
	}
	return append(out, allies...)
}

// Living returns the members still on their feet.
func Living(members []*model.Character) []*model.Character {
	var out []*model.Character
	for _, c := range members {
		if c.Alive() {
			out = append(out, c)
		}
	}
	return out
}

// Full reports whether a company with that many hirelings has room for another.
func Full(allies int) bool { return allies+1 >= MaxSize }

// Rest puts everyone back to full. Anything that restores the hero — a bed, an
// altar, being carried into town — goes through here, because a party that
// heals one member at a time arrives at the next fight in three different
// conditions for no reason the player can see.
func Rest(members []*model.Character) {
	for _, c := range members {
		c.HP = c.MaxHP
		c.Psyche = c.MaxPsyche
	}
}

// UniqueName stops two members of one company answering to the same thing.
//
// The given-name pool is thirty deep and the hero draws from it too, so a
// collision is far from rare — and a party panel with two identical rows, or a
// transcript where you cannot tell who just went down, is unreadable.
//
// A regnal number rather than "the Lesser": the panel gives a name eighty
// pixels, and a suffix that gets truncated to "Bosk the." has solved nothing.
// Nobody involved acknowledges the number.
func UniqueName(members []*model.Character, name string) string {
	taken := func(n string) bool {
		for _, c := range members {
			if c.Name == n {
				return true
			}
		}
		return false
	}
	if !taken(name) {
		return name
	}
	for _, numeral := range []string{" II", " III", " IV"} {
		if !taken(name + numeral) {
			return name + numeral
		}
	}
	return name + " V"
}

// EncounterSize scales a rolled encounter to the size of the company.
//
// A party of three walking into a wood does not meet the single wolf a lone
// traveller meets. Without this, hiring anyone would be a straight discount on
// difficulty rather than a trade — you would be buying the same fights with
// more swords, and the whole curve the balance pass tuned would go soft.
func EncounterSize(g *core.RNG, base, allies int) int {
	if allies > 0 {
		base += g.Intn(allies + 1)
	}
	return core.Clamp(base, 1, MaxFoes)
}
