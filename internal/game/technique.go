package game

import (
	"fmt"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// techniqueBlurb is what a technique actually does, in words, for the caster
// about to spend psyche on it.
//
// Derived from the rules rather than authored beside each row, which is the
// same split the rest of the content follows: the joke is in the name and the
// cast line, and the *answer* is arithmetic that must never disagree with the
// arithmetic the dice use. A `desc` field on thirty-five techniques would be
// thirty-five numbers to keep in step with a balance pass, and the first one
// that drifted would be a lie the player had no way to catch.
//
// It quotes a real magnitude off rules.SpellPower for the character holding it,
// so a rod in the hand and a level-up both move the number on the screen. That
// is the whole reason this exists: the shop tells a caster what a wand is worth
// and the battle screen used to tell them nothing at all.
func techniqueBlurb(c *model.Character, s model.Spell) []string {
	var out []string
	add := func(format string, args ...any) {
		out = append(out, render.Wrap(fmt.Sprintf(format, args...), render.ScreenW-40)...)
	}

	// The magnitude, in the unit the kind actually uses. Damage and healing go
	// through the spell roll; a condition's strength is the authored power and
	// nothing else, so quoting the rolled figure for a weakening would be a
	// number that appears nowhere in the game.
	rolled := int(rules.SpellPower(c, s))

	switch s.Kind {
	case model.SpellDamage:
		add("Damage%s. About %d, less what it resists with.", reach(s), rolled)
	case model.SpellDrain:
		add("Damage%s, about %d, and you keep half of it back.", reach(s), rolled)
	case model.SpellHeal:
		add("Restores about %d hit points%s.", rolled, toWhom(s))
	case model.SpellRevive:
		add("Puts somebody who has gone down back on their feet.")
	case model.SpellWeaken:
		add("Hits %d softer for the rest of the fight%s.", s.Power, reach(s))
	case model.SpellBless:
		add("Hits %d harder for the rest of the fight%s.", s.Power, toWhom(s))
	case model.SpellStun:
		add("Costs it the next turn%s.", reach(s))
	case model.SpellPoison:
		add("Damage at the end of every round%s: %d a round, for four.", reach(s), s.Power)
	case model.SpellBurn:
		add("Burning%s: %d a round for three rounds, which is more and sooner.",
			reach(s), s.Power)
	case model.SpellSap:
		add("Takes %d off what they hit for%s and puts it on yours. Both last "+
			"the rest of the fight.", s.Power, reach(s))
	case model.SpellPact:
		add("Damage%s, about %d — far above what it costs. You hit %d softer "+
			"for the rest of the fight.", reach(s), rolled, rules.PactCost(s))
	}

	cost := rules.PsycheCost(c, s)
	line := fmt.Sprintf("%d psyche, of the %d you have.", cost, c.Psyche)
	if rate := rules.PsycheRate(c.Class); rate > 1 {
		// Said where the surcharge is paid. A Fighter reading "6 psyche" beside
		// a Mage's "4" for the same-looking move deserves the reason.
		line = fmt.Sprintf("%s Technique comes dear to a %s.", line, strings.ToLower(string(c.Class)))
	}
	add("%s", line)
	return out
}

// reach names who a technique lands on, as a clause that reads on the end of a
// sentence rather than as a field.
func reach(s model.Spell) string {
	if s.Target == model.TargetAll {
		return ", to everything opposite"
	}
	return ""
}

func toWhom(s model.Spell) string {
	switch s.Target {
	case model.TargetAll:
		return ", for everybody"
	case model.TargetSelf:
		return ", for you"
	}
	return ""
}
