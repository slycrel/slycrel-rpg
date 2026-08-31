package gamedata

import (
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
)

// Encounter shapes.
//
// The complaint this answers is one the balance report has been making about
// itself for a long time without anybody hearing it: every build wins 96 to 100
// per cent of on-level fights *by design*, which is why the ARCS and DANGER
// sections only ever compare builds on the stretch fights three levels over. So
// the fight a player is supposed to be having is decided before it starts, and
// the only thing that varies is how much it costs.
//
// The fix is not to make on-level fights harder — that breaks the brief and
// makes the whole game a coin flip. It is to make them *different from each
// other*. Everything needed was already built and unused: an armour axis and a
// ward axis that no encounter ever put in the same room, multi-target
// techniques with nothing to point them at, and a monster table with speed and
// plating spread across it that PickMonsters averaged flat.
//
// A shape is a composition rule, not a difficulty dial. Each is meant to land
// near the others on total threat and nowhere near them on what to do about it.
type Shape string

const (
	// ShapeMixed is what the game threw before there were shapes: a handful of
	// whatever lives here, at the level asked for. It stays the commonest by a
	// wide margin, because a texture that happens every time is the texture.
	ShapeMixed Shape = "mixed"
	// ShapePack is more of them, each smaller and quicker. The fight a
	// wide-swing technique and a point of armour are for.
	ShapePack Shape = "pack"
	// ShapeBrute is one of them, much larger. Scaled up rather than picked from
	// above its band, which keeps the promise an encounter level makes: it is
	// the same creature you would have met, with more of it.
	ShapeBrute Shape = "brute"
	// ShapeEscort is something that attacks with magic standing behind things
	// that stop steel. Kill order is the whole fight.
	ShapeEscort Shape = "escort"
	// ShapeMismatch is one thing that stops steel beside one that stops magic,
	// so no single answer covers the room. This is the matchup axis being the
	// encounter rather than an occasional surprise.
	ShapeMismatch Shape = "mismatch"
)

// Encounter is a fight and what sort of fight it is.
type Encounter struct {
	Monsters []*model.Monster
	Shape    Shape
}

// Line is how the transcript opens, which is where a shape has to be legible.
//
// A player cannot read a composition off three portraits and a health bar in
// the second before they choose. Naming the shape in one clause is the whole
// difference between "a fight" and "this kind of fight" — and it costs a
// sentence rather than an interface.
func (e Encounter) Line() string {
	switch e.Shape {
	case ShapePack:
		return "a lot of them, moving together"
	case ShapeBrute:
		return "one of them, and it is enormous"
	case ShapeEscort:
		return "something behind something else"
	case ShapeMismatch:
		return "two of them, and they are not the same problem"
	}
	if len(e.Monsters) > 1 {
		return ""
	}
	return ""
}

// Reshaping constants. Each is the amount of "not a mixed fight" a shape gets,
// and all of them are tuned against the SHAPES section rather than picked.
const (
	packBodies  = 2    // extra creatures beyond what a mixed fight would send
	packHP      = 0.55 // ... each of them this much of itself
	packOffense = 0.72 // ... and each hitting this much as hard

	bruteHP      = 2.05 // one creature, this much of itself
	bruteOffense = 1.10

	guards       = 1 // things standing in front of the caster
	guardHP      = 0.75
	guardOffense = 0.70
	casterHP     = 0.85 // the thing behind them dies quickly, by design

	// A mismatch is the one shape whose members are not reshaped at all in the
	// first draft, and it showed: two full-strength near-level creatures is a
	// bigger fight than the one-and-a-half a mixed roll averages, so it swung
	// between 57% and 98% win by biome depending on what the roster happened to
	// contrast with what.
	// A mismatch keeps both halves whole and takes the edge off their swing
	// instead. Cutting hit points was the first draft and it made the shape the
	// *easiest* in the game: half of a lopsided pair is always soft to whatever
	// you happen to be holding, so shrinking them turned "you have to switch"
	// into "one of them is a free kill". What the shape wants to cost is a
	// round spent changing your mind, which is length rather than danger.
	mismatchHP      = 0.90
	mismatchOffense = 1.00
)

// PickEncounter rolls the fight the game actually throws: a shape, and the
// creatures to fill it.
//
// size is what the party-scaled roll asked for, and it is honoured as the
// *mixed* count — every other shape is expressed relative to it, so hiring a
// second companion still makes fights bigger in the way it always did.
func (t *Tables) PickEncounter(g *core.RNG, biome string, level, size int) Encounter {
	pool := t.poolFor(biome, level)
	if len(pool) == 0 {
		return Encounter{}
	}
	size = core.Max(1, size)

	shape := t.rollShape(g, pool, level, size)
	var mons []*model.Monster

	switch shape {
	case ShapePack:
		// A band down and biased towards whatever in the roster is quick and
		// thin, because the point of a pack is the number of attacks rather
		// than the weight of any one of them.
		lower := t.poolFor(biome, core.Max(1, level-1))
		mons = drawFrom(g, lower, core.Max(1, level-1), size+packBodies,
			func(d *model.MonsterDef) int { return 1 + core.Max(0, d.Speed-d.Defense) })
		scale(mons, packHP, packOffense, "pack")

	case ShapeBrute:
		// Scaled rather than drawn from above its band. Picking a level+2
		// creature would break the promise the encounter level makes — that is
		// the accidental overshoot poolFor exists to stop — where doubling one
		// creature is the same thing you would have met with more of it. The
		// dungeon boss has worked this way since it was written.
		mons = drawFrom(g, pool, level, 1,
			func(d *model.MonsterDef) int { return 1 + d.HP/8 })
		scale(mons, bruteHP, bruteOffense, "brute")
		mons[0].Name = "A Very Large " + mons[0].Def.Name

	case ShapeEscort:
		casters := filter(pool, func(d *model.MonsterDef) bool { return d.Magic })
		mons = drawFrom(g, casters, level, 1, nil)
		scale(mons, casterHP, 1, "caster")
		front := drawFrom(g, pool, core.Max(1, level-1), guards,
			func(d *model.MonsterDef) int { return 1 + core.Max(0, d.Defense-d.Ward) })
		scale(front, guardHP, guardOffense, "guard")
		mons = append(mons, front...)

	case ShapeMismatch:
		plated, warded := contrast(pool, level)
		mons = []*model.Monster{plated.Spawn(g, level), warded.Spawn(g, level)}
		scale(mons, mismatchHP, mismatchOffense, "mismatch")

	default:
		shape = ShapeMixed
		mons = drawFrom(g, pool, level, size, nil)
	}

	return Encounter{Monsters: nameGroup(mons), Shape: shape}
}

// shapeOdds is how often each shape turns up, before feasibility. Mixed keeps
// the plurality on purpose: shapes are meant to be the thing you notice, and a
// game where every fight is a special composition has no ordinary fight to
// notice them against.
var shapeOdds = []struct {
	shape  Shape
	weight int
}{
	{ShapeMixed, 42},
	{ShapePack, 18},
	{ShapeBrute, 14},
	{ShapeEscort, 14},
	{ShapeMismatch, 12},
}

// rollShape picks a shape the roster can actually supply.
//
// Feasibility is read off the pool rather than off the level, which is what
// keeps this honest as content is added: an escort needs something in the biome
// that attacks with magic, and nothing does below level ten by design, so the
// shape simply does not come up before then without anybody writing that number
// down twice.
func (t *Tables) rollShape(g *core.RNG, pool []*model.MonsterDef, level, size int) Shape {
	var shapes []Shape
	var weights []int
	for _, o := range shapeOdds {
		if !feasible(o.shape, pool, level, size) {
			continue
		}
		shapes = append(shapes, o.shape)
		weights = append(weights, o.weight)
	}
	if len(shapes) == 0 {
		return ShapeMixed
	}
	i := g.Weighted(weights)
	if i < 0 {
		return ShapeMixed
	}
	return shapes[i]
}

func feasible(s Shape, pool []*model.MonsterDef, level, size int) bool {
	switch s {
	case ShapePack:
		// A pack of one is a mixed fight with extra steps.
		return level > 1 && size+packBodies >= 3
	case ShapeEscort:
		return len(filter(pool, func(d *model.MonsterDef) bool { return d.Magic })) > 0
	case ShapeMismatch:
		plated, warded := contrast(pool, level)
		// Each has to actually beat the other on its own axis, or "no single
		// answer covers the room" is a claim about two creatures that are the
		// same creature.
		return plated != nil && warded != nil && plated.ID != warded.ID &&
			plated.Defense >= warded.Defense+3 && warded.Ward >= plated.Ward+3
	}
	return true
}

// contrast returns the best-armoured and the best-warded creature in a band,
// for the shape that wants two different problems in one room.
//
// It picks by *resistance* and not by lopsidedness, which is the second attempt
// and the one that works. Picking the most lopsided pair guarantees that each
// half is soft on the other axis, so a fighter deletes the warded one in a
// round and grinds the plated one alone — the shape reduced the effective
// number of enemies instead of raising it, and read as the easiest fight in the
// game at 99% win. What it wants is two creatures that are each genuinely hard
// to hurt the way you are currently hurting things.
//
// Nearness to the band is part of the score rather than a filter, because
// lopsidedness on its own picked whatever had the widest gap and that is
// usually something small from the bottom of the roster.
func contrast(pool []*model.MonsterDef, level int) (plated, warded *model.MonsterDef) {
	bestP, bestW := -1, -1
	for _, d := range pool {
		near := core.Max(0, 12-core.Abs(d.Level-level)*4)
		if n := d.Defense + near; n > bestP {
			plated, bestP = d, n
		}
		if n := d.Ward + near; n > bestW {
			warded, bestW = d, n
		}
	}
	return plated, warded
}

func filter(pool []*model.MonsterDef, ok func(*model.MonsterDef) bool) []*model.MonsterDef {
	out := pool[:0:0]
	for _, d := range pool {
		if ok(d) {
			out = append(out, d)
		}
	}
	return out
}

// scale resizes a spawned creature in place. Hit points move the length of the
// fight and offense moves what it costs; a shape usually wants one and not the
// other.
// scale resizes a group and stamps what it was resized *as*.
//
// The stamp is the point of the second parameter existing at all. Two creatures
// off one definition scaled differently must never share a slot on the field —
// see model.SameKind — and working that out by comparing the numbers afterwards
// is an inference that holds only while no two scalings can land on the same
// answer. The multipliers floor at one, so a quiet enough creature defeats it.
// Here is where the fact is known, so here is where it is recorded.
func scale(mons []*model.Monster, hp, offense float64, build string) {
	for _, m := range mons {
		m.MaxHP = core.Max(1, int(float64(m.MaxHP)*hp))
		m.HP = m.MaxHP
		m.Offense = core.Max(1, int(float64(m.Offense)*offense))
		m.Build = build
	}
}
