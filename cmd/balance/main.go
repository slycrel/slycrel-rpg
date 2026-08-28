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

	"github.com/slycrel/slycrel-rpg/internal/content"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/party"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/sky"
	"github.com/slycrel/slycrel-rpg/internal/world"
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

	reportOpening(out, core.NewRNG(*seed^0x09E4), t, *fights)
	reportCombat(out, g, t, *fights)
	// Its own generator, not the shared one. That keeps this section's
	// placement in the report free: dropping it in the middle of the sequence
	// would otherwise shift every number after it and cost the cheapest check
	// there is, which is diffing the report against the last one.
	reportArcs(out, core.NewRNG(*seed^0x5ACB), t, *fights/2)
	reportDanger(out, core.NewRNG(*seed^0xD1E), t, *fights/3)
	reportWard(out, core.NewRNG(*seed^0x3A7D), t, *fights)
	// Its own generator too, for the same reason as ARCS above.
	reportShapes(out, core.NewRNG(*seed^0x5411), t, *fights)
	reportEndurance(out, g, t, *fights/4)
	reportProgression(out, g, t, *fights/50)
	reportEconomy(out, t)
	reportSaga(out, t)
	reportSky(out)
	reportSupplies(out, t)
	reportCompany(out)
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

// dangerTargets is the design brief for how lethal each band should be, as a
// death rate, and it is a brief rather than a measurement: these are the
// numbers the game is supposed to hit, written down so the report can disagree
// with them out loud.
//
//	"challenging but rewarding, deaths relatively rare... at or above level
//	very rare, and under-levelled increasing in complexity (+5 levels is
//	likely going to kill you, and ideally that's because you made poor
//	choices to get into that situation, not an un-fun surprise)"
//
// The last clause is the one that constrains the shape rather than the values.
// A death at +5 has to be the end of a visible slide, so the curve wants to
// climb steeply and continuously through the middle bands: if +1 and +3 are
// both comfortable and +5 is fatal, the player gets no warning and the death
// reads as the game cheating. A band that is safer than its neighbour below it
// is the same failure wearing a nicer number.
var dangerTargets = []struct {
	delta    int     // encounter level relative to the player
	maxDeath float64 // upper bound on the death rate, as a percentage
	minDeath float64 // lower bound, because a band that cannot hurt you is not a band
	label    string
}{
	{-2, 1, 0, "over-levelled, should be a formality"},
	{0, 5, 0, "on-level, very rare deaths"},
	{2, 20, 2, "starting to cost something"},
	{3, 35, 5, "challenging, and the last comfortable warning"},
	// The floor here is low on purpose now that the simulator runs away when a
	// competent player would. "Likely to kill you" cannot mean "kills you
	// whatever you do" in a game with a flee button — it means you cannot win
	// it and getting out costs you. The outcome table under this one is where
	// that is actually checked: what should be near zero at +5 is the *win*
	// rate, not the survival rate.
	{5, 100, 20, "unwinnable, and expensive to walk away from"},
}

// reportOpening measures the first ten minutes, which nothing else here does.
//
// Every other section dresses its subject through Equip — best weapon and
// armour of the expected tier — and calls that "on curve". A character who has
// just been created is not on that curve and cannot be: startRun hands out
// StarterKit, which is the *cheapest* thing in the tables their class can hold,
// and the cheapest entries used to be Bare Hands at strike 1 and Regrettable
// Rags at defence 0. So the report was describing a level-one character
// carrying a mace and wearing boiled leather while the actual one had neither,
// and the opening of the game was the only part of it never measured.
func reportOpening(out *os.File, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "OPENING — the first fight, with what you actually start holding\n")
	fmt.Fprintf(out, "against what every other section assumes you are wearing\n\n")

	// A row per class, because the kit is per class now: a Mage opens holding
	// a humming stick and a Fighter a table leg, and reporting one of them as
	// "what you start with" would be describing two thirds of a game.
	for _, class := range model.AllClasses {
		w, a := t.StarterKit(class)
		onCurve := &model.Character{Level: 1, Class: class}
		t.Equip(onCurve)
		fmt.Fprintf(out, "  %-8s starts %s / %s, on curve %s / %s\n",
			strings.ToLower(string(class)), w.Name, a.Name,
			onCurve.Weapon.Name, onCurve.Armor.Name)
	}
	fmt.Fprintln(out)

	fmt.Fprintf(out, "%-9s %-12s %9s %9s %9s %8s\n",
		"class", "kit", "win", "died", "rounds", "hp left")
	fmt.Fprintln(out, strings.Repeat("-", 62))

	for _, class := range model.AllClasses {
		for _, kit := range []string{"as created", "on curve"} {
			var wins, deaths, rounds, hp, n int
			for i := 0; i < fights; i++ {
				c := rules.NewCharacter(g, "Subject", class)
				if kit == "on curve" {
					t.Equip(c)
				} else {
					c.Weapon, c.Armor = t.StarterKit(class)
				}
				// What the ground around the capital actually throws, rather
				// than plains at exactly level one.
				//
				// This section used to assume both, and so reported the opening
				// as a 0.2% chance of dying while a real first hour was nothing
				// of the sort: the danger formula reads every location within
				// eighteen tiles, and a level-four ruin sixteen tiles out was
				// handing fresh characters level-three fights in hills and
				// mountains. The home region caps that at the player's level
				// now, so this samples the same spread the game does.
				biome, enc := openingRoll(g)
				mons := t.PickMonsters(g, biome, enc, 1)
				if len(mons) == 0 {
					continue
				}
				fresh := *c
				r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
					enc, 60, t.SpellsFor(c))
				if r.Won {
					wins++
				}
				if r.Died() {
					deaths++
				}
				rounds += r.Rounds
				hp += r.HPLeft * 100 / core.Max(1, c.MaxHP)
				n++
			}
			if n == 0 {
				continue
			}
			f := func(v int) float64 { return float64(v) * 100 / float64(n) }
			fmt.Fprintf(out, "%-9s %-12s %8.1f%% %8.1f%% %9.1f %7d%%\n",
				class, kit, f(wins), f(deaths), float64(rounds)/float64(n), hp/n)
		}
	}

	// What the gap costs in coins, since the answer to it might simply be
	// "walk into the shop first" — in which case the game has to say so.
	// What the purse can actually reach, rolled rather than quoted: the range
	// lives in rules.NewCharacter and a number repeated here would drift.
	lo, hi := 1<<30, 0
	for i := 0; i < 500; i++ {
		c := rules.NewCharacter(g, "Subject", model.ClassFighter)
		if int(c.Coins) < lo {
			lo = int(c.Coins)
		}
		if int(c.Coins) > hi {
			hi = int(c.Coins)
		}
	}
	// What closing the gap costs, per class, since the shelves differ now.
	fmt.Fprintf(out, "\n  a new character carries %d-%d coins; on curve costs", lo, hi)
	for _, class := range model.AllClasses {
		onCurve := &model.Character{Level: 1, Class: class}
		t.Equip(onCurve)
		fmt.Fprintf(out, " %s %d", strings.ToLower(string(class)),
			onCurve.Weapon.Cost+onCurve.Armor.Cost)
	}
	fmt.Fprint(out, "\n\n")
}

// openingBiomes are what turned up within twelve tiles of the start across a
// spread of seeds, in roughly the proportions they turned up in. The ground
// around the capital is whatever the noise put there, and assuming plains was
// the other half of why this section read the first hour as harmless.
var openingBiomes = []string{"plains", "plains", "plains", "forest", "forest", "hills", "coast"}

// openingRoll picks a fight of the sort a new character actually gets handed:
// somewhere near home, at the level the home region allows.
func openingRoll(g *core.RNG) (string, int) {
	// The home cap holds this to the player's own level; the spread below it is
	// the ordinary jitter.
	enc := core.Max(1, 1+g.Between(-1, 0))
	return core.Pick(g, openingBiomes), enc
}

// reportDanger measures the death curve against the brief above.
//
// Win rate is what the rest of the report shows; this shows the complement,
// because "deaths relatively rare" is a statement about losing and a 92% win
// rate reads as fine right up until you notice it means one run in twelve ends.
func reportDanger(out *os.File, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "DANGER — death rate by how far over your head you are\n")
	fmt.Fprintf(out, "on-curve gear, fought in the region that far out, one row per class because\n")
	fmt.Fprintf(out, "an average across three classes hides a class that never dies\n\n")
	fmt.Fprintf(out, "%-6s %-9s %-8s", "level", "class", "region")
	for _, d := range dangerTargets {
		fmt.Fprintf(out, "%9s", fmt.Sprintf("%+d", d.delta))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, strings.Repeat("-", 60))

	// worst[i] is the highest death rate seen in band i, and best the lowest.
	worst := make([]float64, len(dangerTargets))
	best := make([]float64, len(dangerTargets))
	for i := range best {
		best[i] = 100
	}
	monotonic := true

	for level := 1; level <= maxLevel; level += 2 {
		for _, class := range model.AllClasses {
			fmt.Fprintf(out, "%-6d %-9s %-8s", level, class, biomeForLevel(level))
			prev := -1.0
			for i, d := range dangerTargets {
				enc := core.Max(1, level+d.delta)
				biome := biomeForLevel(enc)
				var deaths, n int
				for k := 0; k < fights; k++ {
					c := rules.BuildCharacter(g, class, level)
					equip(t, c)
					mons := t.PickMonsters(g, biome, enc, 1)
					if len(mons) == 0 {
						continue
					}
					fresh := *c
					r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
						enc, 60, t.SpellsFor(c))
					if r.Died() {
						deaths++
					}
					n++
				}
				rate := 0.0
				if n > 0 {
					rate = float64(deaths) * 100 / float64(n)
				}
				if rate > worst[i] {
					worst[i] = rate
				}
				if rate < best[i] {
					best[i] = rate
				}
				// A band easier than the one below it is the shape failure:
				// the slide toward a death at +5 has to be visible all the way.
				if prev >= 0 && rate+2 < prev {
					monotonic = false
				}
				prev = rate
				fmt.Fprintf(out, "%8.1f%%", rate)
			}
			fmt.Fprintln(out)
		}
	}

	fmt.Fprintf(out, "\nagainst the brief\n")
	ok := true
	for i, d := range dangerTargets {
		status := "ok"
		if worst[i] > d.maxDeath {
			status = fmt.Sprintf("OVER by %.1f", worst[i]-d.maxDeath)
			ok = false
		} else if best[i] < d.minDeath {
			status = fmt.Sprintf("UNDER by %.1f", d.minDeath-best[i])
			ok = false
		}
		fmt.Fprintf(out, "  %+d  %-42s %4.0f-%-4.0f%%  %s\n",
			d.delta, d.label, best[i], worst[i], status)
	}
	if !monotonic {
		fmt.Fprintf(out, "\n  SHAPE: some band is easier than the one below it. A death at +5 is\n"+
			"  only fair if the danger climbed visibly on the way there.\n")
		ok = false
	}
	if ok {
		fmt.Fprintf(out, "\n  the curve matches the brief.\n")
	}

	// What actually happens five levels over, split three ways.
	//
	// "A death you chose" is the whole clause the brief turns on, and it is
	// only checkable here: if the player can see it going badly and leave, then
	// dying at +5 means having stayed. A band where death and escape are both
	// common is a band with a decision in it. One where death is the only
	// outcome is an ambush with extra steps.
	fmt.Fprintf(out, "\nfive levels over: what happens\n")
	fmt.Fprintf(out, "%-6s %-9s %9s %9s %9s\n", "level", "class", "won", "fled", "died")
	fmt.Fprintln(out, strings.Repeat("-", 46))
	for level := 1; level <= maxLevel; level += 4 {
		enc := level + 5
		biome := biomeForLevel(enc)
		for _, class := range model.AllClasses {
			var won, fled, died, n int
			for k := 0; k < fights; k++ {
				c := rules.BuildCharacter(g, class, level)
				equip(t, c)
				mons := t.PickMonsters(g, biome, enc, 1)
				if len(mons) == 0 {
					continue
				}
				fresh := *c
				r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
					enc, 60, t.SpellsFor(c))
				switch {
				case r.Won:
					won++
				case r.Fled:
					fled++
				default:
					died++
				}
				n++
			}
			if n == 0 {
				continue
			}
			f := func(v int) float64 { return float64(v) * 100 / float64(n) }
			fmt.Fprintf(out, "%-6d %-9s %8.1f%% %8.1f%% %8.1f%%\n",
				level, class, f(won), f(fled), f(died))
		}
	}
	fmt.Fprintln(out)
}

// reportWard measures what ignoring the ward slot costs, which is the whole
// design of the matchup axis: a player is allowed to skip it, and skipping it
// is supposed to be free early and progressively worse later.
//
// Two things have to be true for that to work, and neither is visible in the
// DANGER table because both are averaged away there. Magical attackers have to
// be rare at the bottom of the game and common at the top. And wearing ward has
// to be worth the slot when they are common, without being mandatory.
func reportWard(out *os.File, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "WARD — what skipping the anti-magic slot costs\n")
	fmt.Fprintf(out, "death rate three levels over, against magical attackers only, with the\n")
	fmt.Fprintf(out, "on-curve charm and with the best ward charm of the same tier\n\n")
	fmt.Fprintf(out, "%-6s %-9s %10s %10s %10s %10s\n",
		"level", "class", "magic %", "no ward", "warded", "slot worth")
	fmt.Fprintln(out, strings.Repeat("-", 60))

	for level := 3; level <= maxLevel; level += 2 {
		enc := level + 3
		biome := biomeForLevel(enc)

		// How much of what lives out there attacks that way at all. Zero here
		// means the slot is dead weight at this level, which is intended low
		// down and a problem high up.
		magicShare := 0.0
		if defs := t.Monsters[biome]; len(defs) > 0 {
			n := 0
			for _, d := range defs {
				if core.Abs(d.Level-enc) <= 3 && d.Magic {
					n++
				}
			}
			total := 0
			for _, d := range defs {
				if core.Abs(d.Level-enc) <= 3 {
					total++
				}
			}
			if total > 0 {
				magicShare = float64(n) * 100 / float64(total)
			}
		}

		for _, class := range model.AllClasses {
			// Only magical attackers: the question is what the slot is for,
			// and a table that mixed in the creatures it does nothing against
			// would report the answer diluted by however many of those there
			// happen to be.
			rate := func(withWard bool) float64 {
				var deaths, n int
				for i := 0; i < fights; i++ {
					c := rules.BuildCharacter(g, class, level)
					equip(t, c)
					if withWard {
						if ch, ok := bestWardCharm(t, gamedata.GearTierFor(level)); ok {
							c.Charm = ch
						}
					}
					mons := t.PickMonsters(g, biome, enc, 1)
					if len(mons) == 0 || !mons[0].Def.Magic {
						continue
					}
					fresh := *c
					r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
						enc, 60, t.SpellsFor(c))
					if r.Died() {
						deaths++
					}
					n++
				}
				if n == 0 {
					return -1
				}
				return float64(deaths) * 100 / float64(n)
			}
			// When no ward charm exists at this tier the two columns are the
			// same build measured twice, and the gap between them is noise.
			// Say so rather than printing a delta that means nothing.
			if _, ok := bestWardCharm(t, gamedata.GearTierFor(level)); !ok {
				bare := rate(false)
				if bare < 0 {
					fmt.Fprintf(out, "%-6d %-9s %9.0f%% %10s %10s %10s\n",
						level, class, magicShare, "-", "-", "nothing out there")
					continue
				}
				fmt.Fprintf(out, "%-6d %-9s %9.0f%% %9.1f%% %10s %10s\n",
					level, class, magicShare, bare, "-", "none sold yet")
				continue
			}
			bare, warded := rate(false), rate(true)
			if bare < 0 || warded < 0 {
				fmt.Fprintf(out, "%-6d %-9s %9.0f%% %10s %10s %10s\n",
					level, class, magicShare, "-", "-", "nothing out there")
				continue
			}
			fmt.Fprintf(out, "%-6d %-9s %9.0f%% %9.1f%% %9.1f%% %9.1f pts\n",
				level, class, magicShare, bare, warded, bare-warded)
		}
	}
	fmt.Fprintln(out)
}

// bestWardCharm returns the strongest anti-magic charm a shop of this tier
// carries, which is what a player who decided to answer the axis would buy.
func bestWardCharm(t *gamedata.Tables, tier int) (model.Charm, bool) {
	var best model.Charm
	found := false
	for _, c := range t.Charms {
		if c.Tier > tier || c.Bonus.Ward <= 0 {
			continue
		}
		if !found || c.Bonus.Ward > best.Bonus.Ward {
			best, found = c, true
		}
	}
	return best, found
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

	// Which build wins *when* is the thing this table is actually for, and a
	// count of levels won hides it. The order below is the whole finding: the
	// off arm changes hands partway up the game, because what is swinging at
	// you changes with it.
	if len(levels) > 1 {
		fmt.Fprintf(out, "\nwho wins, in order of level: ")
		for _, l := range levels {
			rows := stretch[l]
			best := rows[0]
			for _, r := range rows[1:] {
				if r.over > best.over {
					best = r
				}
			}
			fmt.Fprintf(out, "%s ", best.build)
		}
		fmt.Fprintln(out)
		fmt.Fprint(out, `
Read that against the WARD table below rather than as a list. Nothing that
attacks with magic exists under level ten, and by thirteen two thirds of the
blows landing on you are magical — so a shield, which stops steel and nothing
else, is worth most exactly where there is least magic about, and the silvered
one is worth most where the plain one has stopped mattering. The two-hander
sits in between: it buys damage, which works on everything, at the cost of an
arm that is worth a great deal early and very little late.

balanced winning nothing is not a fault to fix. It is the straightforward
build, it is what Equip means, and every other number in this report is
measured against it — a middle option that is never best and never worst is
what a baseline is.
`)
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
	fmt.Fprintf(out, "exchange rates it trades at, read off a Fighter's lane\n\n")
	fmt.Fprintf(out, "%-6s %14s %14s %14s %14s %14s\n",
		"tier", "weapon step", "armour step", "shield step", "charm def step", "barrier step")
	fmt.Fprintln(out, strings.Repeat("-", 81))

	best := func(tier int) (int, int, int, int, int) {
		ws, as := t.StockForClass(tier, model.ClassFighter)
		ss, cs := t.SidearmsFor(tier)
		var strike, def, shield, charm, barrier int
		for _, w := range ws {
			if w.Strike > strike {
				strike = w.Strike
			}
		}
		for _, a := range as {
			if a.Defense > def {
				def = a.Defense
			}
		}
		// The best thing a Fighter could put on that arm, which since casters
		// got a slot is no longer simply the last row of the band: a talisman
		// blocks nothing, so reading the tail of the list turned the shield
		// step column into zeroes and one negative number.
		for _, sh := range ss {
			if model.CanHoldShield(model.ClassFighter, sh) && sh.Defense > shield {
				shield = sh.Defense
			}
		}
		if len(cs) > 0 {
			charm = cs[len(cs)-1].Bonus.Defense
		}
		// And the caster's arm, which is measured in a different unit: a pool
		// spent once rather than a reduction on every blow.
		for _, sh := range ss {
			if sh.Absorb > barrier {
				barrier = sh.Absorb
			}
		}
		return strike, def, shield, charm, barrier
	}

	for tier := 2; tier <= 5; tier++ {
		s0, d0, sh0, ch0, b0 := best(tier - 1)
		s1, d1, sh1, ch1, b1 := best(tier)
		fmt.Fprintf(out, "%-6d %13d+ %13d+ %13d+ %13d+ %13d+\n",
			tier, s1-s0, d1-d0, sh1-sh0, ch1-ch0, b1-b0)
	}
	fmt.Fprint(out, `
Three things fall out of this table.

The weapon step is even now - five a band, all the way up - which is
what "a band behind on the weapon" has to be if a build that pays in
weapon bands is going to be consistently one thing rather than lurching
in and out of viability by level. It ran +2, +5, +5, +4 before the lanes
went in, and that unevenness was the 13.5-point hole attrition used to
fall into at level 9.

A sidearm band is worth about a quarter of a main-gear band, which caps
how different any two builds can be. That is not an oversight -
TestShieldsStaySecondaryToArmour in internal/gamedata deliberately holds
a shield under half the body armour of its own band, so the slot stays a
sidearm. Widening the arcs means revisiting that rule on purpose, not
quietly inflating the shield table until the test goes red.

And the barrier column is not in the same unit as the three beside it.
A shield step is a point off every blow for the rest of the fight; a
barrier step is a lump of damage stopped once and then gone. Ten of the
one is not ten of the other, and the reason the caster's arm carries a
number this much larger is that it has to cover a whole fight in a
single payment. Read it against the fight length in COMBAT, not against
the shield column.

`)
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

// reportShapes measures the compositions, which is where the on-level fight
// stopped being a foregone conclusion.
//
// It is a section of its own rather than a rewrite of COMBAT and DANGER, and
// that is deliberate: those two measure one creature against one character on
// curve, which is a controlled reading and still the right baseline for every
// other number in this report. What they cannot say is what a *fight* is like,
// because until shapes existed a fight was n creatures of no particular
// arrangement.
//
// The claim being tested is not that the shapes are equally hard. It is that
// they are close enough on total threat to all be on-level fights, and far
// enough apart on how they get there to be worth telling apart. A shape that
// wins ten points more than mixed is a shape the player should always want, and
// a shape that wins thirty less is a death sentence wearing a description.
func reportShapes(out *os.File, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "SHAPES — what an encounter is made of\n")
	fmt.Fprintf(out, "on-curve gear, on-level fights, the party-scaled size a solo hero rolls\n\n")
	fmt.Fprintf(out, "%-12s %-10s %8s %8s %8s %8s %8s\n",
		"level", "shape", "seen", "win", "died", "rounds", "hp left")
	fmt.Fprintln(out, strings.Repeat("-", 70))

	// The oddity's roster is probed by name at the end, because it is the one
	// place in the game built for contrast — constructs that stop steel beside
	// things that stop magic — and biomeForLevel never sends anybody there.
	probes := []struct {
		level int
		biome string
	}{{3, ""}, {7, ""}, {11, ""}, {13, ""}, {6, "oddity"}, {12, "oddity"}}
	for _, probe := range probes {
		level, biome := probe.level, probe.biome
		if biome == "" {
			biome = biomeForLevel(level)
		}
		type tally struct{ n, wins, deaths, rounds, hp, maxhp int }
		by := map[gamedata.Shape]*tally{}
		order := []gamedata.Shape{}

		for i := 0; i < fights; i++ {
			c := rules.BuildCharacter(g, model.AllClasses[i%len(model.AllClasses)], level)
			equip(t, c)
			enc := t.PickEncounter(g, biome, level, 1+g.Intn(2))
			if len(enc.Monsters) == 0 {
				continue
			}
			if by[enc.Shape] == nil {
				by[enc.Shape] = &tally{}
				order = append(order, enc.Shape)
			}
			a := by[enc.Shape]
			a.n++
			fresh := *c
			r := rules.SimulateGroup(g, &fresh, enc.Monsters, 60, t.SpellsFor(c))
			if r.Won {
				a.wins++
			}
			if r.Died() {
				a.deaths++
			}
			a.rounds += r.Rounds
			a.hp += r.HPLeft
			a.maxhp += fresh.MaxHP
		}

		label := fmt.Sprintf("%d", level)
		if probe.biome != "" {
			label = fmt.Sprintf("%d %s", level, probe.biome)
		}
		sort.Slice(order, func(i, j int) bool { return by[order[i]].n > by[order[j]].n })
		for _, sh := range order {
			a := by[sh]
			if a.n == 0 {
				continue
			}
			fmt.Fprintf(out, "%-12s %-10s %7.0f%% %7.1f%% %7.1f%% %8.1f %7.0f%%\n",
				label, sh, 100*float64(a.n)/float64(fights),
				100*float64(a.wins)/float64(a.n), 100*float64(a.deaths)/float64(a.n),
				float64(a.rounds)/float64(a.n), 100*float64(a.hp)/float64(a.maxhp))
		}
		fmt.Fprintln(out)
	}

	fmt.Fprint(out, `A shape is a composition, not a difficulty dial. What it is allowed to
move is how the fight is won - a pack wants the technique that hits
everything, a brute wants the one that hits hardest, an escort wants you
to pick a target and get to it, and a mismatch wants two answers because
neither creature is the other one's problem. What it is not allowed to
move much is the win rate.

The "seen" column is the other half of it: a shape the roster cannot
supply never appears rather than appearing as something else. Nothing
attacks with magic below level ten, so nothing escorts anything below
level ten either, and no number anywhere had to say so twice.

That column is also where this section earned its keep. The mismatch is
almost absent from the ordinary biomes and shows up reliably only in the
oddity, because it needs one creature that beats another by three on
armour while being beaten by three on ward, and the fantasy rosters were
never written to contrast. The joke zone was - constructs that stop
steel beside things that stop magic - so the one place in the game where
the matchup axis is the whole encounter is the one where everything is
the wrong century. That is a content gap in the other eight biomes, and
this is it being stated rather than guessed at.

`)
}

func reportProgression(out *os.File, g *core.RNG, t *gamedata.Tables, runs int) {
	fmt.Fprintf(out, "PROGRESSION — experience needed against experience offered,\n")
	fmt.Fprintf(out, "and what that costs in trips back to an inn\n\n")
	fmt.Fprintf(out, "%-5s %10s %10s %10s %8s %8s %8s\n",
		"level", "xp total", "xp step", "xp/fight", "fights", "per rest", "trips")
	fmt.Fprintln(out, strings.Repeat("-", 70))

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
		fights := float64(step) / per
		// The number this section was missing, and the reason it was missing
		// it: how far one rest gets you lives in ENDURANCE and how far a level
		// is lives here, and the quotient — how many times you walk back to an
		// inn for one level — was in neither. It runs from a tenth of a trip at
		// level one to eleven and a half at fourteen, which is the same walk
		// eleven times, and no single table could see it.
		endur := enduranceAt(g, t, level, runs)
		trips := "-"
		if endur > 0 {
			trips = fmt.Sprintf("%.1f", fights/endur)
		}
		fmt.Fprintf(out, "%-5d %10d %10d %10.1f %8.1f %8.1f %8s\n",
			level+1, rules.XPForLevel(level+1), step, per, fights, endur, trips)
	}
	fmt.Fprint(out, `
A trip is a round walk to a bed and back, and the column is what one level
costs in them. Camping is the answer to it: a kit is half of both pools back
without the walk, at the price of the kit and a roll on whether anything
finds you. It does not fill the pools, wake you at dawn or write a
checkpoint, which is what an inn still sells.

`)
}

// enduranceAt is ENDURANCE's number for one level, so PROGRESSION can divide by
// it. Measured rather than passed in: the two sections run independently and a
// figure copied between them is a figure that drifts.
func enduranceAt(g *core.RNG, t *gamedata.Tables, level, runs int) float64 {
	total := 0
	for i := 0; i < runs; i++ {
		sim := rules.BuildCharacter(g, model.ClassFighter, level)
		equip(t, sim)
		spells := t.SpellsFor(sim)
		survived := 0
		for survived < 60 {
			mons := t.PickMonsters(g, biomeForLevel(level), level, 1)
			if len(mons) == 0 {
				break
			}
			r := rules.SimulateFight(g, sim, []*model.MonsterDef{mons[0].Def}, level, 60, spells)
			if !r.Won || sim.HP <= 0 {
				break
			}
			survived++
		}
		total += survived
	}
	return float64(total) / float64(runs)
}

func reportEconomy(out *os.File, t *gamedata.Tables) {
	fmt.Fprintf(out, "ECONOMY — what the tier you should be wearing costs\n")
	fmt.Fprintf(out, "one row per class, because the shelf a class can read is not the shelf\n\n")
	fmt.Fprintf(out, "%-8s %-5s %5s %30s %7s %30s %7s\n",
		"class", "level", "tier", "weapon", "cost", "armor", "cost")
	fmt.Fprintln(out, strings.Repeat("-", 96))
	for _, class := range []model.Class{model.ClassFighter, model.ClassThief, model.ClassMage} {
		for level := 1; level <= maxLevel; level += 4 {
			c := &model.Character{Level: level, Class: class}
			t.Equip(c)
			fmt.Fprintf(out, "%-8s %-5d %5d %30s %7d %30s %7d\n",
				strings.ToLower(string(class)), level, gamedata.GearTierFor(level),
				c.Weapon.Name, c.Weapon.Cost, c.Armor.Name, c.Armor.Cost)
		}
	}
	fmt.Fprintln(out)
}

// shopStock is the restorative shelf, in the order the shop lists it. Kept
// here so this section measures what a player can actually walk in and buy,
// rather than every healing item that exists in the data — several of which
// only ever come off a monster.
var shopStock = []string{
	"Small Beer", "Field Poultice", "Physician's Draught",
	"Bottled Nap", "Philosopher's Espresso", "Bitter Root", "Suspicious Pollen",
	"Smelling Salts, Militant", "Still-Warm Heart",
	"Damp Compress", "Broad Antidote",
	"Bedroll and Some Firewood", "Proper Camp Kit",
}

// reportSupplies is what staying upright costs at a counter, and it exists
// because the combat simulator deliberately does not model potions — ENDURANCE
// says "no potions" in its own header.
//
// That was fine while items were a convenience. It stopped being fine when the
// thief started leaving the counter with two of everything restorative, which
// is the class's compensation for having no healing technique at all: the whole
// mechanic lives on the buying side of the game, where the fight simulator
// cannot see it. Measuring it here rather than teaching SimulateFight to drink
// is the honest trade — modelling potion use would re-tune every endurance
// number in the report to answer a question about prices.
func reportSupplies(out *os.File, t *gamedata.Tables) {
	fmt.Fprintf(out, "SUPPLIES — what the pack costs, and what the thief pays for it\n\n")
	fmt.Fprintf(out, "%-26s %-8s %6s %7s %9s %9s\n",
		"item", "kind", "power", "price", "per point", "as thief")
	fmt.Fprintln(out, strings.Repeat("-", 70))

	thief := &model.Character{Class: model.ClassThief}

	for _, name := range shopStock {
		it, ok := t.Item(name)
		if !ok {
			fmt.Fprintf(out, "%-26s MISSING — the shop lists it and the data does not have it\n", name)
			continue
		}
		price := it.Value * 2
		n := rules.SleightOfHand(thief, it.Kind)
		// Cost per point of effect, which is the only figure that compares a
		// Small Beer to a Still-Warm Heart. Items with no power — a cure, an
		// antidote — have nothing to divide by and say so.
		per, asThief := "-", "-"
		if it.Power > 0 {
			per = fmt.Sprintf("%.1f", float64(price)/float64(it.Power))
			asThief = fmt.Sprintf("%.1f", float64(price)/float64(it.Power*n))
		}
		fmt.Fprintf(out, "%-26s %-8s %6d %7d %9s %9s\n",
			it.Name, it.Kind, it.Power, price, per, asThief)
	}

	fmt.Fprintf(out, "\nA thief pays for one restorative and leaves with two, so its column is\n")
	fmt.Fprintf(out, "half the one beside it. That is the whole of what it gets for having no\n")
	fmt.Fprintf(out, "way to heal itself, and it is spent at a counter rather than in a fight.\n\n")
}

// reportSaga measures the one claim the long stories rest on.
//
// A saga has no level gate anywhere in it. Its legs are dealt out at increasing
// distance from where the story starts, and the whole design is the assertion
// that this is enough — that the danger of a region rises with how far out it
// is, so a spine paces itself and a player who runs it too early dies with
// warning rather than by ambush.
//
// That claim was made in a commit message and never measured, which is exactly
// the habit this whole command exists to break. So: cast every story on several
// continents and print how far out each leg is and how dangerous the country
// around it is, using the game's own RegionLevel rather than a copy of it.
//
// What the column has to do is climb. A leg that is further out and no more
// dangerous is a leg the geography is not pacing, and enough of those would
// mean the spine needs a gate after all.
func reportSaga(out *os.File, t *gamedata.Tables) {
	fmt.Fprintf(out, "SAGA — how far out each leg is, and how rough the country there is\n\n")
	fmt.Fprintf(out, "%-18s %-5s %s\n", "story", "leg", "distance / region level, per seed")
	fmt.Fprintln(out, strings.Repeat("-", 74))

	seeds := []int64{1, 7, 1994, 20260817}
	var climbs, total int
	firstMin, firstMax := 1<<30, 0

	for _, sk := range t.Sagas.Sagas {
		kind := "spine"
		if sk.Arc {
			kind = "arc"
		}
		// One row per leg, with every seed's answer on it, because the shape
		// worth seeing is whether the numbers rise left to right down the
		// column rather than what any single continent happened to produce.
		type run struct {
			dist, level []int
		}
		runs := map[int64]run{}
		for _, seed := range seeds {
			w := world.Generate(seed, content.New(&t.Text))
			g := core.NewRNG(seed)
			s, ok := saga.Cast(g, &t.Sagas, w, t, &sk, w.Start, 4, nil)
			if !ok {
				continue
			}
			r := run{}
			for _, idx := range s.Places {
				p := w.POIs[idx]
				r.dist = append(r.dist, p.Pos.Manhattan(w.Start))
				r.level = append(r.level, w.RegionLevel(p.Pos))
			}
			if len(r.dist) > 0 {
				firstMin = core.Min(firstMin, r.dist[0])
				firstMax = core.Max(firstMax, r.dist[0])
			}
			runs[seed] = r
			// Did the danger actually climb from the first leg to the last?
			if len(r.level) > 1 {
				total++
				if r.level[len(r.level)-1] > r.level[0] {
					climbs++
				}
			}
		}

		for leg := 0; ; leg++ {
			line, any := "", false
			for _, seed := range seeds {
				r, ok := runs[seed]
				if !ok || leg >= len(r.dist) {
					line += "     . "
					continue
				}
				any = true
				line += fmt.Sprintf(" %3d/%-2d", r.dist[leg], r.level[leg])
			}
			if !any {
				break
			}
			label := ""
			if leg == 0 {
				label = fmt.Sprintf("%s (%s)", sk.ID, kind)
			}
			fmt.Fprintf(out, "%-18s %-5d %s\n", label, leg+1, line)
		}
	}

	fmt.Fprintf(out, "\nEach cell is tiles-from-the-start / how rough the country is there.\n")
	fmt.Fprintf(out, "The far end is rougher than the near end in %d of %d stagings.\n", climbs, total)
	// Where the first leg lands matters more than where the last one does. The
	// spine is offered at level one, the compass points straight at it, and the
	// home region — the only ground tuned for a fresh character — stops at
	// HomeRadius. A first leg well beyond it is the opening being handed a
	// difficulty nobody chose.
	fmt.Fprintf(out, "First legs land %d-%d tiles out; the home region ends at %d.\n",
		firstMin, firstMax, world.HomeRadius)
	if total > 0 && climbs*2 < total {
		fmt.Fprintf(out, "WARNING: distance is not buying difficulty, so the spine is not pacing\n")
		fmt.Fprintf(out, "         itself and would need a level gate after all.\n")
	}
	fmt.Fprintln(out)
}

// reportSky is what the time of day and the weather do to the overworld loop.
//
// It is a statement rather than a simulation, and that is the honest shape for
// it. Two of the three terms are already measured elsewhere and re-simulating
// them would be inventing a second answer: night adds one to the encounter
// level, and what one level over costs is exactly what the DANGER table above
// spends four hundred thousand fights establishing. The third term — how often
// a step turns into a fight at all — is not simulated anywhere, because
// SimulateFight starts once a fight exists and knows nothing about the walking.
//
// So this prints the multipliers and lets them be read against the section that
// already has the numbers. What it is really for is the shape: night and
// weather have to pull opposite ways, and a table where every row moved the
// same direction would mean the correct play is always "wait for a clear noon".
func reportSky(out *os.File) {
	fmt.Fprintf(out, "SKY — what the light and the weather do to a step\n\n")
	fmt.Fprintf(out, "%-9s %8s %8s %7s   %s\n", "phase", "sight", "prowl", "level", "share of the day")
	fmt.Fprintln(out, strings.Repeat("-", 62))

	share := map[sky.Phase]int{}
	for step := 0; step < sky.DayLength; step++ {
		share[(sky.Clock{Step: step}).Phase()]++
	}
	for _, p := range []sky.Phase{sky.Dawn, sky.Day, sky.Dusk, sky.Night} {
		fmt.Fprintf(out, "%-9s %8d %7.2fx %+7d   %d%%\n",
			p.Name(), p.Sight(), p.Prowl(), p.LevelShift(),
			share[p]*100/sky.DayLength)
	}

	fmt.Fprintf(out, "\n%-9s %8s %8s\n", "weather", "sight", "prowl")
	fmt.Fprintln(out, strings.Repeat("-", 62))
	for _, w := range []sky.Weather{sky.Clear, sky.Cloudy, sky.Rain, sky.Snow, sky.Storm} {
		fmt.Fprintf(out, "%-9s %+8d %7.2fx\n", w.Name(), w.Sight(), w.Prowl())
	}

	// The corners, which is the only part of this that is not the two tables
	// above read twice.
	fmt.Fprintf(out, "\n%-22s %8s %8s\n", "an evening out", "sight", "prowl")
	fmt.Fprintln(out, strings.Repeat("-", 62))
	for _, c := range []struct {
		label string
		p     sky.Phase
		w     sky.Weather
	}{
		{"clear noon", sky.Day, sky.Clear},
		{"clear night", sky.Night, sky.Clear},
		{"wet day", sky.Day, sky.Rain},
		{"stormy night", sky.Night, sky.Storm},
	} {
		fmt.Fprintf(out, "%-22s %8d %7.2fx\n",
			c.label, sky.Sight(c.p, c.w), sky.Prowl(c.p, c.w))
	}

	worst := sky.Prowl(sky.Night, sky.Clear)
	best := sky.Prowl(sky.Night, sky.Storm)
	fmt.Fprintf(out, "\nThe night to be afraid of is the clear one: %.2fx against %.2fx in a storm,\n",
		worst, best)
	fmt.Fprintf(out, "and a step at night draws an encounter one level over, which the DANGER\n")
	fmt.Fprintf(out, "table above prices. Nothing here is a gate: a bed at an inn buys the morning.\n")
	if best >= worst {
		fmt.Fprintf(out, "WARNING: weather does not cover for you, so there is nothing to read in the sky.\n")
	}
	fmt.Fprintln(out)
}

// reportCompany shows what the people following you take off every haul, and
// what honour does to it.
//
// This is here because the cut is the one term in the economy that the fight
// simulator cannot see. Coins come out of Skim at the end of a battle, one
// deduction per companion, and they add: two hirelings at the top of the roll
// take better than a third of everything before the hero touches it. Nothing
// else in the report says so, and a number that large has no business being
// invisible.
//
// The percentages are exact — Recruit rolls in a known band and AskingCut is
// arithmetic on it — so this section states the rule rather than sampling it.
// What it deliberately does not do is convert to coins: that would need a model
// of what a haul is worth per level, and inventing one to put a confident
// number under it is how a report starts measuring a fiction.
func reportCompany(out *os.File) {
	fmt.Fprintf(out, "THE COMPANY'S SHARE — what honour is worth at the hiring board\n\n")

	// The band Recruit rolls in. Named rather than repeated so this section
	// fails loudly if the roll ever moves.
	const (
		rollLow  = 8
		rollMid  = 13
		rollHigh = 18
	)
	allies := party.MaxSize - 1

	fmt.Fprintf(out, "%-7s %8s %8s %8s %s\n",
		"honour", "low", "mid", "high", "full company, worst roll")
	fmt.Fprintln(out, strings.Repeat("-", 60))

	worst := 0
	for _, honor := range []int{-8, -6, -4, -2, 0, 2, 4, 6, 8} {
		hi := rules.AskingCut(rollHigh, honor)
		full := hi * allies
		if honor == 0 {
			worst = full
		}
		fmt.Fprintf(out, "%-7d %7d%% %7d%% %7d%% %23d%%\n",
			honor,
			rules.AskingCut(rollLow, honor),
			rules.AskingCut(rollMid, honor),
			hi, full)
	}

	fmt.Fprintf(out, "\n%d companions, each taking a cut of the same haul; the shares add.\n", allies)
	// Half the haul is where coins stop being the reward for a fight and start
	// being a rounding error on somebody else's wages.
	if worst > 50 {
		fmt.Fprintf(out, "WARNING: a full company at the top of the roll takes %d%% of every haul.\n", worst)
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
