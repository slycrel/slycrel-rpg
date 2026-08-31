package gamedata

import "github.com/slycrel/slycrel-rpg/internal/model"

// Dressing a build to a purse rather than to a table of band offsets.
//
// The band offsets say what shape a build is. They do not say what it costs,
// and for the life of the ARCS section nobody had checked — so the report was
// comparing a duelist holding 2,600 coins of gear against a balanced build
// holding 2,220 and calling the difference a finding about two-handed weapons.
// Attrition underspent by about a tenth at every level from five up and lost at
// every one of them; both rivals outspent balanced by 37% at level one and both
// beat it. That is not a build measurement, it is a budget measurement, and the
// plan had already written down the rule it was breaking: an archetype that
// underspends measures the spec, not the content.
//
// So: every build gets the same purse, which is what the baseline costs at that
// level, and buys its own shape inside it.

// giveWayOrder is which slot loses a band when a build cannot afford itself.
//
// Flourishes first, then the coat, and the weapon last — which is the order the
// tables are already described in, the sidearms being "what you buy with
// whatever the sword and the coat left over". A build that has to give up its
// weapon to fit has stopped being itself, so that is the last resort rather
// than the first saving.
var giveWayOrder = []int{slotCharm, slotShield, slotArmor, slotWeapon}

// upgradeOrder is where a build's leftover money goes. Only the sidearms, and
// for the same reason: those are the slots the design already calls the change
// from the main purchases. Letting spare coin buy a better sword would turn
// "attrition, with money left over" into "balanced", which is the one thing the
// comparison must not do.
var upgradeOrder = []int{slotShield, slotCharm}

const (
	slotWeapon = iota
	slotArmor
	slotShield
	slotCharm
)

// EquipWithin dresses a character in an archetype's shape for as much of a
// purse as the shelves let it spend, and never a coin more.
//
// Exact equality is not on offer and the report says so rather than pretending:
// gear comes in bands, the gaps between them are large, and a build whose next
// upgrade costs more than it has left simply stops. What this removes is the
// half of the confound that was indefensible — a build being measured while
// holding more gear than the baseline could afford.
//
// A budget of zero means "no limit", which is what Equip and EquipAs do.
func (t *Tables) EquipWithin(c *model.Character, a Archetype, budget int) {
	if budget <= 0 {
		t.EquipAs(c, a)
		return
	}
	base := GearTierFor(c.Level)
	back := [4]int{a.Weapon.Back, a.Armor.Back, a.Shield.Back, a.Charm.Back}

	dress := func() int {
		t.equipBands(c, a, base, back)
		return GearCost(c)
	}

	// Down first: a build that cannot afford its own shape gives bands away in
	// the order above until it fits or has nothing left to give.
	for i := 0; dress() > budget && i < 16; i++ {
		moved := false
		for _, s := range giveWayOrder {
			if a.slotSkipped(s) {
				continue
			}
			// A sidearm may be given up entirely — the baseline itself carries
			// no shield and no charm at level one, so "nothing on that arm" is
			// a band like any other. A weapon or a coat may not: a build that
			// walked into a fight bare-handed would be measuring an arithmetic
			// error rather than a shape, which is a mistake EquipAs made once
			// already and floors against.
			floor := 1
			if s == slotShield || s == slotCharm {
				floor = 0
			}
			if base-back[s] <= floor {
				continue
			}
			back[s]++
			moved = true
			break
		}
		if !moved {
			break
		}
	}

	// Then up, with whatever is left, into the sidearms only. Never above the
	// band the level itself is on: the question is which shape spends a level's
	// money best, not who can afford to shop above their station.
	for i := 0; i < 8; i++ {
		moved := false
		for _, s := range upgradeOrder {
			if a.slotSkipped(s) || back[s] <= 0 {
				continue
			}
			back[s]--
			if dress() > budget {
				back[s]++
				continue
			}
			moved = true
			break
		}
		if !moved {
			break
		}
	}
	dress()
}

// slotSkipped reports a slot this build's own definition removes.
//
// Deliberately not a check on the hand count. A two-handed build closes its own
// off arm, but it closes it through the weapon it actually ends up holding —
// and a Mage cannot hold a two-hander, so a Mage duelist has a free arm and a
// shield on it. That is the archetype's own documented subtlety, and reading
// Hands here instead of the weapon put a shield the fitter could not take off
// onto three classes' worth of level-one duelists, eight coins over a purse of
// sixty-six. Where the arm really is closed, stepping its band is a no-op and
// costs one wasted turn of a bounded loop.
func (a Archetype) slotSkipped(slot int) bool {
	switch slot {
	case slotWeapon:
		return a.Weapon.Skip
	case slotArmor:
		return a.Armor.Skip
	case slotShield:
		return a.Shield.Skip
	default:
		return a.Charm.Skip
	}
}

// equipBands is EquipAs with the band offsets supplied rather than read off the
// archetype, which is the whole of what fitting to a purse needs.
func (t *Tables) equipBands(c *model.Character, a Archetype, base int, back [4]int) {
	fit := a
	fit.Weapon.Back = back[slotWeapon]
	fit.Armor.Back = back[slotArmor]
	fit.Shield.Back = back[slotShield]
	fit.Charm.Back = back[slotCharm]
	t.EquipAs(c, fit)
}
