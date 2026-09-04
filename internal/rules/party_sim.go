package rules

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// PartyResult is what an encounter cost a company, as opposed to a person.
type PartyResult struct {
	Won  bool
	Fled bool
	// Rounds counts rounds that were fought. The round that discovers the fight
	// is over is not one of them — see the solo loop, which counted it for the
	// life of the report.
	Rounds int
	// Standing is how many of the company were still upright at the end, which
	// is the number a solo report has no way to have: a fight can be won with
	// two of three on the floor, and that is a different afternoon from a fight
	// won intact.
	Standing int
	// Down counts how many times somebody went to the floor, and Revives how
	// many times somebody was stood back up. A revive was the last technique in
	// the game no report could price, because pricing it needs somebody else to
	// be down.
	Down, Revives int

	Swings, Casts int
	CastsBy       [castKindCount]int

	DamageDealt, DamageTaken int
	// HPLeft is per member, in the order they were passed in.
	HPLeft []int
}

// SimulateParty plays an encounter against a whole company.
//
// **Every fight in the balance report is one character, and the game has not
// been a solo game since hirelings landed.** That is the largest single thing
// this report has been unable to see: a companion is an extra sword and, more
// usefully, an extra place for a claw to land, and neither of those exists in a
// simulation of one person. CROWDS' wider columns have always meant "a solo
// hero after their company was killed", which is a real situation and not the
// one most fights are.
//
// The order is the battle screen's, deliberately and in detail. Each member
// rolls initiative against the fastest creature and goes before or after the
// monsters as a group — not as one side moving and then the other — and anybody
// who lost initiative may be on the floor by the time their turn arrives, which
// is checked rather than assumed. A creature swings at whoever is upright,
// chosen at random, which is the whole mechanism by which a party spreads
// damage.
//
// The hero plays the policy the rest of the report measures; the companions
// play ChooseAllyMove, which is the same brain the game gives them. Two brains
// rather than one is not a shortcut — it is what the game does, and a report
// where the companions fought better than they do would be pricing a party
// nobody can field.
func SimulateParty(g *core.RNG, party []*model.Character, spellsFor func(*model.Character) []model.Spell,
	mons []*model.Monster, maxRounds int, pol Policy) PartyResult {

	var res PartyResult
	if len(party) == 0 {
		return res
	}
	hero := party[0]
	spent := map[*model.Character]int{}
	wasDown := map[*model.Character]bool{}

	// Whatever is on anybody's off arm goes up before the first blow.
	for _, c := range party {
		Raise(c)
	}

	standing := func() []*model.Character {
		var out []*model.Character
		for _, c := range party {
			if c.Alive() {
				out = append(out, c)
			}
		}
		return out
	}
	hurt := func(m *model.Monster, by *model.Character, dmg int) {
		m.HP = core.Max(0, m.HP-dmg)
		res.DamageDealt += dmg
		Siphon(by, dmg)
		if m.HP == 0 {
			m.Dead = true
		}
	}

	for res.Rounds = 1; res.Rounds <= maxRounds; res.Rounds++ {
		living := livingMonsters(mons)
		if len(living) == 0 {
			res.Rounds--
			res.Won = true
			break
		}
		up := standing()
		if len(up) == 0 {
			res.Rounds--
			break
		}

		fastest := 0
		for _, m := range living {
			if m.Speed > fastest {
				fastest = m.Speed
			}
		}

		// One member's turn: the hero's policy, or a companion's.
		act := func(c *model.Character) {
			if !c.Alive() || res.Fled {
				return
			}
			live := livingMonsters(mons)
			if len(live) == 0 {
				return
			}
			book := spellsFor(c)

			if c == hero {
				if !pol.NeverFlee && wantsOut(c, live) {
					// The hero's decision, and it takes everybody with them —
					// which is what the battle screen's Flee row does.
					if g.Chance(FleeChance(c.Spd(), fastest)) {
						res.Fled = true
					}
					return
				}
				if !pol.NeverCast {
					if s, ok := bestSpell(c, book, live); ok {
						pay(c, s, spent)
						res.Casts++
						if i := castIndex(s.Kind); i >= 0 {
							res.CastsBy[i]++
						}
						castParty(g, c, s, party, nil, live, hurt, &res)
						return
					}
				}
				res.Swings++
				swingAt(g, c, live[0], hurt)
				// The second swing, and only the plain swing repeats.
				if len(livingMonsters(mons)) > 0 && ExtraSwing(g, c) {
					res.Swings++
					swingAt(g, c, livingMonsters(mons)[0], hurt)
				}
				return
			}

			// A companion. weakestLiving is the screen's choice too: finishing
			// something off removes an attacker from the round.
			target := weakestOf(live)
			move := ChooseAllyMove(g, c, book, party, target)
			switch move.Kind {
			case AllyCast:
				if pol.NeverCast {
					break
				}
				pay(c, move.Spell, spent)
				res.Casts++
				if i := castIndex(move.Spell.Kind); i >= 0 {
					res.CastsBy[i]++
				}
				if move.Spell.Kind == model.SpellRevive && move.Ally != nil && !move.Ally.Alive() {
					res.Revives++
				}
				castParty(g, c, move.Spell, party, move.Ally, live, hurt, &res)
				return
			case AllyGuard:
				return
			}
			res.Swings++
			swingAt(g, c, target, hurt)
			if len(livingMonsters(mons)) > 0 && ExtraSwing(g, c) {
				res.Swings++
				swingAt(g, c, weakestOf(livingMonsters(mons)), hurt)
			}
		}

		var before, after []*model.Character
		for _, c := range up {
			if Initiative(g, c.Spd(), fastest) {
				before = append(before, c)
			} else {
				after = append(after, c)
			}
		}

		monsterTurns := func() {
			for _, m := range living {
				if m.Dead {
					continue
				}
				if Has(m.Active, model.EffectStun) {
					m.Active = Remove(m.Active, model.EffectStun)
					continue
				}
				alive := standing()
				if len(alive) == 0 {
					return
				}
				switch ChooseMonsterAction(g, m, len(livingMonsters(mons)) == 1) {
				case MonFlee:
					m.Dead, m.Fled = true, true
					continue
				case MonDefend:
					continue
				}
				// Whoever is upright, at random. This is the mechanism a party
				// buys: three places for a claw to land instead of one.
				tgt := core.Pick(g, alive)
				if Dodged(g, tgt, m.Speed) {
					if CanCounter(tgt) {
						hurt(m, tgt, CounterDamage(g, tgt, m))
					}
					continue
				}
				dmg := core.Max(0, MonsterDamage(g, tgt, m)+OffenseMod(m.Active))
				var soaked int
				tgt.Active, dmg, soaked = Soak(tgt.Active, dmg)
				_ = soaked
				tgt.HP = core.Max(0, tgt.HP-dmg)
				res.DamageTaken += dmg
				if e, ok := RollAffliction(g, m.Def.Inflicts); ok && tgt.HP > 0 {
					tgt.Active = Apply(tgt.Active, e)
				}
				if !tgt.Alive() && !wasDown[tgt] {
					wasDown[tgt] = true
					res.Down++
				}
			}
		}

		for _, c := range before {
			act(c)
		}
		if res.Fled {
			break
		}
		monsterTurns()
		for _, c := range after {
			act(c)
		}
		if res.Fled {
			break
		}

		// Conditions bite at the end of the round, on both sides.
		for _, m := range living {
			if m.Dead {
				continue
			}
			for _, t := range TickDamage(g, m.Active) {
				hurt(m, hero, t.Damage)
			}
			m.Active, _ = Advance(m.Active)
		}
		for _, c := range party {
			if !c.Alive() {
				continue
			}
			for _, t := range TickDamage(g, c.Active) {
				d := t.Damage
				c.Active, d, _ = Soak(c.Active, d)
				c.HP = core.Max(0, c.HP-d)
				res.DamageTaken += d
			}
			c.Active, _ = Advance(c.Active)
			if !c.Alive() && !wasDown[c] {
				wasDown[c] = true
				res.Down++
			}
		}
	}

	for _, c := range party {
		c.Active = nil
		CatchBreath(c, 0, spent[c])
		res.HPLeft = append(res.HPLeft, c.HP)
		if c.Alive() {
			res.Standing++
		}
	}
	if len(livingMonsters(mons)) == 0 && res.Standing > 0 && !res.Fled {
		res.Won = true
	}
	return res
}

// pay takes a technique's psyche off whoever cast it.
func pay(c *model.Character, s model.Spell, spent map[*model.Character]int) {
	cost := PsycheCost(c, s)
	c.Psyche -= cost
	spent[c] += cost
}

// swingAt is one plain weapon blow.
func swingAt(g *core.RNG, c *model.Character, m *model.Monster, hurt func(*model.Monster, *model.Character, int)) {
	if m == nil || m.Dead {
		return
	}
	sw := PlayerAttack(g, c, m, OffenseMod(c.Active), DexterityMod(c.Active))
	if sw.Miss {
		return
	}
	hurt(m, c, sw.Damage)
}

// weakestOf is the creature a companion goes for: the one closest to falling
// over, since finishing something off removes an attacker from the round.
func weakestOf(live []*model.Monster) *model.Monster {
	var best *model.Monster
	for _, m := range live {
		if m.Dead {
			continue
		}
		if best == nil || m.HP < best.HP {
			best = m
		}
	}
	return best
}

// castParty resolves a technique cast by anybody in the company, on either side.
func castParty(g *core.RNG, c *model.Character, s model.Spell, party []*model.Character,
	ally *model.Character, live []*model.Monster,
	hurt func(*model.Monster, *model.Character, int), res *PartyResult) {

	if s.Kind.Side() == model.SideParty {
		for _, t := range allyTargets(s, party, c, ally) {
			CastOnAlly(g, c, s, t)
		}
		return
	}
	target := live[0]
	for _, m := range sapTargets(s, live, target) {
		landed := CastAtFoe(g, c, s, m)
		if landed.Damage > 0 {
			hurt(m, c, landed.Damage)
		}
		if landed.Drained > 0 {
			c.Heal(landed.Drained)
		}
	}
	CastOnCaster(c, s)
}

// allyTargets is who a party-side technique reaches: everybody, or the one it
// was aimed at.
//
// The aim matters and was nearly dropped. ChooseAllyMove already decides who a
// companion is healing — the whole point of the targeting rework was that a
// medic patches whoever is worst off rather than only themselves — and a
// simulator that took the caster instead would have measured a party of three
// people each looking after their own hit points, which is not a party.
//
// The hero's own policy has nobody else in mind, so it passes nil and gets
// itself, which is what the solo report has always measured.
func allyTargets(s model.Spell, party []*model.Character, caster, aimed *model.Character) []*model.Character {
	if s.Target == model.TargetAll {
		return party
	}
	if aimed != nil {
		return []*model.Character{aimed}
	}
	return []*model.Character{caster}
}
