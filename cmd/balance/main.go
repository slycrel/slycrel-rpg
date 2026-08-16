// Command balance simulates the game's arithmetic and reports where the
// numbers disagree with each other.
//
// The formulas in internal/rules are a faithful port and are not in question.
// What is in question is everything invented around them: 54 monsters' stats,
// the weapon and armour tables, prices, and experience awards. None of those
// were ever checked against the curve they feed, and a quest chain that assumes
// a difficulty ramp will expose any gap immediately.
//
//	go run ./cmd/balance            # the full report
//	go run ./cmd/balance -fights 5000
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// maxLevel is the top of the band content is written for.
const maxLevel = 14

func main() {
	fights := flag.Int("fights", 2000, "fights simulated per data point")
	seed := flag.Int64("seed", 20260815, "simulation seed")
	flag.Parse()

	root, err := gamedata.FindRoot()
	if err != nil {
		log.Fatalf("balance: %v", err)
	}
	t, err := gamedata.Load(root)
	if err != nil {
		log.Fatalf("balance: %v", err)
	}

	g := core.NewRNG(*seed)
	out := os.Stdout

	reportCombat(out, g, t, *fights)
	// Its own generator, not the shared one. That keeps this section's
	// placement in the report free: dropping it in the middle of the sequence
	// would otherwise shift every number after it and cost the cheapest check
	// there is, which is diffing the report against the last one.
	reportArcs(out, core.NewRNG(*seed^0x5ACB), t, *fights/2)
	reportEndurance(out, g, t, *fights/4)
	reportProgression(out, g, t)
	reportEconomy(out, t)
	reportMonsterSpread(out, t)
}

// equip fits a character with the best gear of their expected tier, which is
// the "on curve" assumption the rest of the report measures against. The
// definition lives in gamedata because the game hires companions against the
// same curve, and two copies of it would drift.
func equip(t *gamedata.Tables, c *model.Character) { t.Equip(c) }

// biomeForLevel is roughly where a character of this level would be fighting,
// following the world's "danger radiates outward from the capital" layout.
func biomeForLevel(level int) string {
	switch {
	case level <= 2:
		return "plains"
	case level <= 4:
		return "forest"
	case level <= 6:
		return "hills"
	case level <= 8:
		return "swamp"
	case level <= 10:
		return "dungeon"
	default:
		return "mountain"
	}
}

func reportCombat(out *os.File, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "COMBAT — on-curve gear, techniques used, no potions\n")
	fmt.Fprintf(out, "win rates against an encounter at your level, two under, and three over\n")
	fmt.Fprintf(out, "the biome column is where you are; \"over\" is measured in the region three\n")
	fmt.Fprintf(out, "levels further out, because that is what straying actually means\n\n")
	fmt.Fprintf(out, "%-5s %-9s %-10s %8s %8s %8s %7s %8s\n",
		"level", "class", "biome", "under", "on-level", "over", "rounds", "hp left%")
	fmt.Fprintln(out, strings.Repeat("-", 74))

	for level := 1; level <= maxLevel; level++ {
		biome := biomeForLevel(level)
		for _, class := range model.AllClasses {
			rate := func(biome string, encLevel int) (winPct float64, rounds float64, hp int) {
				var wins, totalRounds, totalHP int
				for i := 0; i < fights; i++ {
					c := rules.BuildCharacter(g, class, level)
					equip(t, c)
					spells := t.SpellsFor(c)
					mons := t.PickMonsters(g, biome, encLevel, 1)
					if len(mons) == 0 {
						continue
					}
					fresh := *c // one fight in isolation, full resources
					r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def}, encLevel, 60, spells)
					if r.Won {
						wins++
					}
					totalRounds += r.Rounds
					totalHP += r.HPLeft * 100 / core.Max(1, c.MaxHP)
				}
				return float64(wins) * 100 / float64(fights),
					float64(totalRounds) / float64(fights), totalHP / fights
			}
			// Under and on-level happen where you already are. Three over
			// does not: in the world proper the encounter level is blended
			// with the danger of the region you are standing in, so straying
			// three levels up means having walked into the next region, not
			// meeting an inflated version of the local wildlife. Rolling the
			// local table at level+3 was measuring a fight that mostly cannot
			// be rolled — eight of fourteen biomes top out before level+3, and
			// the column was quietly reporting a near-on-level fight.
			under, _, _ := rate(biome, core.Max(1, level-2))
			on, rounds, hp := rate(biome, level)
			over, _, _ := rate(biomeForLevel(level+3), level+3)
			fmt.Fprintf(out, "%-5d %-9s %-10s %7.1f%% %7.1f%% %7.1f%% %7.1f %7d%%\n",
				level, class, biome, under, on, over, rounds, hp)
		}
	}
	fmt.Fprintln(out)
}

// reportArcs asks whether there is more than one way to be correctly levelled.
//
// Everything else in this report measures the balanced build, which makes "on
// curve" and "the way we expect you to play" the same sentence. This section
// runs the same simulation over each archetype so the question becomes "is each
// of these playable" rather than "is the one balanced".
//
// Averaged across the three classes. Per-class detail for the balanced build is
// in COMBAT above, and nine rows a level would bury the comparison this section
// exists to make.
//
// What it cannot see: an arc defined by spending coin on other people. The
// simulator fights one character, so a company build shows up only as worse
// personal gear and would read as strictly bad. That is a real gap and not one
// to paper over with a guess — it needs SimulateFight to take a party first.
func reportArcs(out *os.File, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "ARCS — is there more than one way to be correctly levelled?\n")
	fmt.Fprintf(out, "win rates averaged across the three classes, on-level where you are and\n")
	fmt.Fprintf(out, "three over in the region that far out\n\n")
	for _, a := range gamedata.Archetypes {
		fmt.Fprintf(out, "  %-10s %s\n", a.Name, a.Note)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%-5s %-10s %7s %9s %8s %8s %9s\n",
		"level", "build", "cost", "on-level", "over", "rounds", "hp left")
	fmt.Fprintln(out, strings.Repeat("-", 64))

	// The comparison is made on the stretch fights, three levels over, not on
	// the on-level ones.
	//
	// On-level is saturated: every build wins 96-100% of those at every level,
	// which is by design — an on-level fight is meant to be winnable — but it
	// means the column cannot tell two builds apart. A gap measured there says
	// "all three are fine" no matter what the gear tables contain, which is the
	// reassuring answer and the useless one. What separates a build is the
	// fight it was not supposed to take.
	type row struct {
		build string
		over  float64
	}
	stretch := map[int][]row{}

	for level := 1; level <= maxLevel; level += 2 {
		biome := biomeForLevel(level)
		for _, a := range gamedata.Archetypes {
			var cost int
			rate := func(biome string, encLevel int) (winPct, rounds float64, hp int) {
				var wins, totalRounds, totalHP, n int
				for _, class := range model.AllClasses {
					for i := 0; i < fights; i++ {
						c := rules.BuildCharacter(g, class, level)
						t.EquipAs(c, a)
						cost = gamedata.GearCost(c)
						mons := t.PickMonsters(g, biome, encLevel, 1)
						if len(mons) == 0 {
							continue
						}
						fresh := *c
						r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
							encLevel, 60, t.SpellsFor(c))
						if r.Won {
							wins++
						}
						totalRounds += r.Rounds
						totalHP += r.HPLeft * 100 / core.Max(1, c.MaxHP)
						n++
					}
				}
				if n == 0 {
					return 0, 0, 0
				}
				return float64(wins) * 100 / float64(n),
					float64(totalRounds) / float64(n), totalHP / n
			}
			on, rounds, hp := rate(biome, level)
			over, _, _ := rate(biomeForLevel(level+3), level+3)

			stretch[level] = append(stretch[level], row{a.Name, over})
			fmt.Fprintf(out, "%-5d %-10s %7d %8.1f%% %7.1f%% %8.1f %8d%%\n",
				level, a.Name, cost, on, over, rounds, hp)
		}
		fmt.Fprintln(out)
	}

	// The verdict. A build that is never the best one at any level is not a
	// playstyle, it is a trap with a name, and the point of measuring before
	// writing content for three arcs is to find that out first.
	fmt.Fprintf(out, "stretch fights (three levels over), best build and the gap to the worst\n")
	levels := make([]int, 0, len(stretch))
	for l := range stretch {
		levels = append(levels, l)
	}
	sort.Ints(levels)

	wins := map[string]int{}
	worstGap := 0.0
	for _, l := range levels {
		rows := stretch[l]
		best, worst := rows[0], rows[0]
		for _, r := range rows[1:] {
			if r.over > best.over {
				best = r
			}
			if r.over < worst.over {
				worst = r
			}
		}
		wins[best.build]++
		gap := best.over - worst.over
		if gap > worstGap {
			worstGap = gap
		}
		fmt.Fprintf(out, "  level %-3d %-10s +%.1f points over %s\n", l, best.build, gap, worst.build)
	}

	fmt.Fprintf(out, "\nlevels won, out of %d: ", len(levels))
	for _, a := range gamedata.Archetypes {
		fmt.Fprintf(out, "%s %d  ", a.Name, wins[a.Name])
	}
	fmt.Fprintf(out, "\nwidest gap at any level: %.1f points.\n", worstGap)

	switch {
	case wins[gamedata.Archetypes[0].Name] == len(levels):
		fmt.Fprintf(out, "VERDICT: balanced is never beaten. There is one arc, not three —\n"+
			"the other builds trade a real slot for a bonus too small to pay for it.\n")
	case worstGap <= 10:
		fmt.Fprintf(out, "VERDICT: no build is ever far behind and each wins somewhere.\n"+
			"The content already supports more than one arc.\n")
	default:
		fmt.Fprintf(out, "VERDICT: each build wins somewhere, but the gap is wide enough that\n"+
			"picking wrong for the level is a real mistake.\n")
	}
	fmt.Fprintln(out)
	reportSlotValue(out, t)
}

// reportSlotValue is the diagnosis behind the verdict above.
//
// An archetype is a trade: give up a band in one slot, buy a band in another.
// That trade can only pay if a whole sidearm slot is worth about as much as one
// band of a main slot. This table is where to look when it is not — it compares
// what stepping up a band buys against what the entire off-hand or charm slot
// buys at the same tier, and no simulation is needed to read the answer off it.
func reportSlotValue(out *os.File, t *gamedata.Tables) {
	fmt.Fprintf(out, "WHY — what one band is worth in each slot\n")
	fmt.Fprintf(out, "every archetype is a trade of bands between slots, so these are the\n")
	fmt.Fprintf(out, "exchange rates it trades at\n\n")
	fmt.Fprintf(out, "%-6s %14s %14s %14s %14s\n",
		"tier", "weapon step", "armour step", "shield step", "charm def step")
	fmt.Fprintln(out, strings.Repeat("-", 66))

	best := func(tier int) (int, int, int, int) {
		ws, as := t.StockFor(tier)
		ss, cs := t.SidearmsFor(tier)
		var strike, def, shield, charm int
		if len(ws) > 0 {
			strike = ws[len(ws)-1].Strike
		}
		if len(as) > 0 {
			def = as[len(as)-1].Defense
		}
		if len(ss) > 0 {
			shield = ss[len(ss)-1].Defense
		}
		if len(cs) > 0 {
			charm = cs[len(cs)-1].Bonus.Defense
		}
		return strike, def, shield, charm
	}

	for tier := 2; tier <= 5; tier++ {
		s0, d0, sh0, ch0 := best(tier - 1)
		s1, d1, sh1, ch1 := best(tier)
		fmt.Fprintf(out, "%-6d %13d+ %13d+ %13d+ %13d+\n",
			tier, s1-s0, d1-d0, sh1-sh0, ch1-ch0)
	}
	fmt.Fprintf(out, "\nTwo things fall out of this table.\n\n"+
		"The weapon step is uneven: +2 into tier 2, then +5, +5, +4. So \"a band\n"+
		"behind on the weapon\" costs two and a half times as much at tier 3 as it\n"+
		"does at tier 2, and any build paying in weapon bands lurches in and out\n"+
		"of viability by level rather than being consistently one thing. That is\n"+
		"the 13.5-point hole attrition falls into at level 9: it is buying a\n"+
		"+1 shield step with a -5 weapon step.\n\n"+
		"And a sidearm band is worth about a quarter of a main-gear band, which\n"+
		"caps how different any two builds can be. That is not an oversight —\n"+
		"TestShieldsStaySecondaryToArmour in internal/gamedata deliberately holds\n"+
		"a shield under half the body armour of its own band, so the slot stays a\n"+
		"sidearm. Widening the arcs means revisiting that rule on purpose, not\n"+
		"quietly inflating the shield table until the test goes red.\n\n")
}

// reportEndurance is the number that actually governs the overworld loop: how
// far you get from an inn before you have to turn back. One fight on a full
// psyche pool flatters a caster; a run of them on the same pool does not.
func reportEndurance(out *os.File, g *core.RNG, t *gamedata.Tables, runs int) {
	fmt.Fprintf(out, "ENDURANCE — on-level fights survived on one rest, no potions\n\n")
	fmt.Fprintf(out, "%-5s %-9s %10s %10s %12s\n", "level", "class", "median", "average", "died on 1st")
	fmt.Fprintln(out, strings.Repeat("-", 52))

	for level := 1; level <= maxLevel; level += 2 {
		biome := biomeForLevel(level)
		for _, class := range model.AllClasses {
			counts := make([]int, 0, runs)
			firstDeaths := 0
			for i := 0; i < runs; i++ {
				c := rules.BuildCharacter(g, class, level)
				equip(t, c)
				spells := t.SpellsFor(c)
				survived := 0
				for survived < 40 {
					mons := t.PickMonsters(g, biome, level, 1)
					if len(mons) == 0 {
						break
					}
					r := rules.SimulateFight(g, c, []*model.MonsterDef{mons[0].Def}, level, 60, spells)
					if !r.Won || c.HP <= 0 {
						break
					}
					survived++
				}
				if survived == 0 {
					firstDeaths++
				}
				counts = append(counts, survived)
			}
			sort.Ints(counts)
			var sum int
			for _, v := range counts {
				sum += v
			}
			median := 0
			if len(counts) > 0 {
				median = counts[len(counts)/2]
			}
			fmt.Fprintf(out, "%-5d %-9s %10d %10.1f %11.1f%%\n",
				level, class, median, float64(sum)/float64(core.Max(1, len(counts))),
				float64(firstDeaths)*100/float64(core.Max(1, len(counts))))
		}
	}
	fmt.Fprintln(out)
}

func reportProgression(out *os.File, g *core.RNG, t *gamedata.Tables) {
	fmt.Fprintf(out, "PROGRESSION — experience needed against experience offered\n\n")
	fmt.Fprintf(out, "%-5s %10s %10s %10s %8s\n", "level", "xp total", "xp step", "xp/fight", "fights")
	fmt.Fprintln(out, strings.Repeat("-", 50))

	for level := 1; level < maxLevel; level++ {
		step := rules.XPForLevel(level+1) - rules.XPForLevel(level)
		if level == 1 {
			step = rules.XPForLevel(2)
		}
		// Average experience from a typical encounter at this level.
		var total, n int
		for i := 0; i < 400; i++ {
			mons := t.PickMonsters(g, biomeForLevel(level), level, 1)
			if len(mons) == 0 {
				continue
			}
			total += mons[0].Def.XP
			n++
		}
		if n == 0 {
			continue
		}
		per := float64(total) / float64(n)
		fmt.Fprintf(out, "%-5d %10d %10d %10.1f %8.1f\n",
			level+1, rules.XPForLevel(level+1), step, per, float64(step)/per)
	}
	fmt.Fprintln(out)
}

func reportEconomy(out *os.File, t *gamedata.Tables) {
	fmt.Fprintf(out, "ECONOMY — what the tier you should be wearing costs\n\n")
	fmt.Fprintf(out, "%-5s %5s %28s %8s %28s %8s\n",
		"level", "tier", "weapon", "cost", "armor", "cost")
	fmt.Fprintln(out, strings.Repeat("-", 92))
	for level := 1; level <= maxLevel; level += 2 {
		tier := gamedata.GearTierFor(level)
		ws, as := t.StockFor(tier)
		w, a := model.Weapon{Name: "-"}, model.Armor{Name: "-"}
		if len(ws) > 0 {
			w = ws[len(ws)-1]
		}
		if len(as) > 0 {
			a = as[len(as)-1]
		}
		fmt.Fprintf(out, "%-5d %5d %28s %8d %28s %8d\n", level, tier, w.Name, w.Cost, a.Name, a.Cost)
	}
	fmt.Fprintln(out)
}

// reportMonsterSpread shows how the rosters are distributed by level, which is
// where holes in the content show up: a band with nothing to fight in it makes
// PickMonsters fall back to something wildly off-level.
func reportMonsterSpread(out *os.File, t *gamedata.Tables) {
	fmt.Fprintf(out, "MONSTER SPREAD — count by level, per biome\n\n")
	biomes := make([]string, 0, len(t.Monsters))
	for b := range t.Monsters {
		biomes = append(biomes, b)
	}
	sort.Strings(biomes)

	fmt.Fprintf(out, "%-11s", "biome")
	for l := 1; l <= maxLevel; l++ {
		fmt.Fprintf(out, "%3d", l)
	}
	fmt.Fprintln(out, "   total")
	fmt.Fprintln(out, strings.Repeat("-", 60))

	gaps := map[int]int{}
	for _, b := range biomes {
		counts := map[int]int{}
		for _, d := range t.Monsters[b] {
			counts[d.Level]++
		}
		fmt.Fprintf(out, "%-11s", b)
		for l := 1; l <= maxLevel; l++ {
			if counts[l] == 0 {
				fmt.Fprintf(out, "  .")
				gaps[l]++
			} else {
				fmt.Fprintf(out, "%3d", counts[l])
			}
		}
		fmt.Fprintf(out, "%8d\n", len(t.Monsters[b]))
	}

	fmt.Fprintf(out, "\nlevels with no monster in any biome: ")
	var empty []string
	for l := 1; l <= maxLevel; l++ {
		if gaps[l] == len(biomes) {
			empty = append(empty, fmt.Sprint(l))
		}
	}
	if len(empty) == 0 {
		fmt.Fprintln(out, "none")
	} else {
		fmt.Fprintln(out, strings.Join(empty, ", "))
	}

	// The line above asks a global question and answers it reassuringly. The
	// question that matters is local: biomeForLevel decides where a character
	// of level N is expected to be fighting, and if *that* biome has nothing at
	// level N then PickMonsters falls back to whatever is within three levels
	// and the player spends the band beating up something junior.
	//
	// This is invisible everywhere else in the report. It shows up only as a
	// level that looks suspiciously comfortable — which is exactly how it was
	// found: level 7 fighters were managing ten fights on one rest, and the
	// reason was that the swamp has nothing to fight at level 7.
	fmt.Fprintf(out, "\nholes where the player is actually sent\n")
	holes := 0
	for l := 1; l <= maxLevel; l++ {
		if b, lo, hi, ok := hole(t, biomeForLevel(l), l); !ok {
			holes++
			if hi == 0 {
				fmt.Fprintf(out, "  level %-3d %-10s nothing within three levels either\n", l, b)
				continue
			}
			fmt.Fprintf(out, "  level %-3d %-10s falls back to levels %d-%d\n", l, b, lo, hi)
		}
	}
	if holes == 0 {
		fmt.Fprintln(out, "  none")
	}

	// And the same question asked of the stretch probe, which COMBAT and ARCS
	// use for the "three levels over" column. That probe now rolls the region a
	// level+3 character would be in rather than the local one, because straying
	// three levels up means having walked somewhere else — but the check still
	// has to be made, since a region that tops out below the level being asked
	// for reports an easier fight than the column claims.
	fmt.Fprintf(out, "\nstretch probes (three levels over, in the region that far out)\n")
	short := 0
	for l := 1; l <= maxLevel; l++ {
		if b, _, hi, ok := hole(t, biomeForLevel(l+3), l+3); !ok {
			short++
			fmt.Fprintf(out, "  level %-3d %-10s tops out at %d, so \"three over\" is really %+d\n",
				l, b, hi, hi-l)
		}
	}
	if short == 0 {
		fmt.Fprintln(out, "  none")
	}
	fmt.Fprintln(out)
}

// hole reports whether a biome has anything at exactly this level, and what the
// ±3 fallback pool spans when it does not.
func hole(t *gamedata.Tables, biome string, level int) (b string, lo, hi int, ok bool) {
	lo, hi = maxLevel, 0
	for _, d := range t.Monsters[biome] {
		if d.Level == level {
			ok = true
		}
		if core.Abs(d.Level-level) <= 3 {
			if d.Level < lo {
				lo = d.Level
			}
			if d.Level > hi {
				hi = d.Level
			}
		}
	}
	return biome, lo, hi, ok
}
