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
	reportEndurance(out, g, t, *fights/4)
	reportProgression(out, g, t)
	reportEconomy(out, t)
	reportMonsterSpread(out, t)
}

// gearTierFor is the gear a player is expected to be carrying at a level. The
// shop stocks by tier, and tiers span roughly three levels each.
func gearTierFor(level int) int {
	return core.Clamp(1+(level-1)/3, 1, 5)
}

// equip fits a character with the best gear of their expected tier, which is
// the "on curve" assumption the rest of the report measures against.
func equip(t *gamedata.Tables, c *model.Character) {
	ws, as := t.StockFor(gearTierFor(c.Level))
	if len(ws) > 0 {
		c.Weapon = ws[len(ws)-1]
	}
	if len(as) > 0 {
		c.Armor = as[len(as)-1]
	}
}

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
	fmt.Fprintf(out, "win rates against an encounter at your level, two under, and three over\n\n")
	fmt.Fprintf(out, "%-5s %-9s %-10s %8s %8s %8s %7s %8s\n",
		"level", "class", "biome", "under", "on-level", "over", "rounds", "hp left%")
	fmt.Fprintln(out, strings.Repeat("-", 74))

	for level := 1; level <= maxLevel; level++ {
		biome := biomeForLevel(level)
		for _, class := range model.AllClasses {
			rate := func(encLevel int) (winPct float64, rounds float64, hp int) {
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
			under, _, _ := rate(core.Max(1, level-2))
			on, rounds, hp := rate(level)
			over, _, _ := rate(level + 3)
			fmt.Fprintf(out, "%-5d %-9s %-10s %7.1f%% %7.1f%% %7.1f%% %7.1f %7d%%\n",
				level, class, biome, under, on, over, rounds, hp)
		}
	}
	fmt.Fprintln(out)
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
		tier := gearTierFor(level)
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
	fmt.Fprintln(out)
}
