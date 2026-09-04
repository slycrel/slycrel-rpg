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
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

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

// The two bands the two sections that price things measure on, and why these
// two rather than DANGER's whole spread. On level and below, both axes are
// saturated by design — the brief asks for nought to five per cent deaths and
// the rest of the report records 96-100% wins — so nothing can show up there
// however good it is. Three over is the last band where a win rate
// discriminates; five over is the band where a death rate does.
//
// LANES and EXCHANGE share them on purpose: a lane is a bundle of stats and
// the exchange desk prices the stats, so the two have to be quoting the same
// fights or the shield table cannot be authored from the pair.
const (
	laneStretch = 3
	laneOver    = 5
)

// provenance is which tree, seed and sample produced the numbers below.
//
// It exists because a reviewer caught three constants in this repo justified by
// figures from a tree that no longer existed. Every one of them was true when
// it was written; three fixes to internal/rules landed the same evening and
// moved the whole table. A measurement quoted in a comment without the commit
// it came from is a measurement nobody can reproduce or falsify, and the cost
// of saying so is one line at the top of the report.
//
// Asked of git rather than read out of the build info, and the first draft did
// the opposite. runtime/debug's vcs.revision looked like the tidy answer — no
// subprocess, works from a distributed binary — and in a git *worktree* it
// stamps the parent repository's HEAD, which is a different commit on a
// different branch. It confidently printed a commit these numbers had never
// been near. A provenance line that is wrong is worse than no provenance line,
// because the whole point of it is to be believed.
//
// Falling back to "unstamped" rather than to a guess, for the same reason.
func provenance(root string, fights int, seed int64) string {
	rev, dirty := "unstamped", ""
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--short=10", "HEAD").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			rev = s
		}
	}
	if out, err := exec.Command("git", "-C", root, "status", "--porcelain").Output(); err == nil {
		if len(strings.TrimSpace(string(out))) > 0 {
			dirty = "+dirty"
		}
	}
	return fmt.Sprintf("tree %s%s, seed %d, %d fights a data point", rev, dirty, seed, fights)
}

func main() {
	fights := flag.Int("fights", 2000, "fights simulated per data point")
	seed := flag.Int64("seed", 20260815, "simulation seed")
	timings := flag.Bool("timings", false, "print wall clock per section")
	flag.Parse()

	root, err := gamedata.FindRoot()
	if err != nil {
		log.Fatalf("balance: %v", err)
	}
	t, err := gamedata.Load(root)
	if err != nil {
		log.Fatalf("balance: %v", err)
	}

	// The collector, not the scheduler, was what this report was waiting for.
	//
	// Running the sections concurrently took it from 66 seconds to 48 and spent
	// 150 seconds of CPU to do it — more than twice the serial run's total, for
	// a 27% saving. That is the signature of garbage collection rather than
	// contention: a fight allocates a character, an encounter and a spell list,
	// four million of them go through here, and ten goroutines allocating at
	// once make the collector run far more often against a heap that is barely
	// growing. The same run at GOGC=800 is 14 seconds and 184MB.
	//
	// So it is set here rather than left as something the reader has to know.
	// An explicit GOGC in the environment still wins: somebody measuring memory
	// deliberately should not have it quietly overridden by a default chosen
	// for speed.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(800)
	}

	out := os.Stdout

	fmt.Fprintf(out, "%s\n\n", provenance(root, *fights, *seed))

	// The report is written concurrently and printed in order.
	//
	// It is one long series of independent experiments and it was running them
	// on one core out of ten, which is not a performance complaint — it is what
	// decides how much simulator there can be. Every question this report
	// cannot afford to ask is a question it does not ask, and the two biggest
	// ones outstanding both cost fights: a policy that can choose every kind of
	// technique has more branches to sample, and a party fight is three
	// characters where a solo fight is one.
	//
	// **No number moves.** Each section already carries its own generator,
	// derived from the seed, precisely so that its placement in the sequence is
	// free — the note that used to sit above ARCS explaining why is now the
	// rule for all of them. Sections write into their own buffer and the
	// buffers are emptied in declared order, so the output is byte-identical to
	// the serial run and the cheapest check there is — diffing this against the
	// last one — still works.
	//
	// The three that share the main generator are the exception and they are
	// chained rather than freed. COMBAT, ENDURANCE and PROGRESSION draw from
	// `g` in that order; giving each its own stream would have been one line
	// and would have moved three sections' numbers for a speedup worth less
	// than the baselines it spent. They run in sequence, on one worker, into
	// three separate buffers that land at three separate places in the report.
	// The main generator, kept for the three sections that draw from it in
	// sequence. It is the same stream and the same order it always was.
	chained := core.NewRNG(*seed)
	sections := []section{
		{"OPENING", func(w io.Writer) { reportOpening(w, core.NewRNG(*seed^0x09E4), t, *fights) }},
		{"COMBAT", nil}, // chained, see below
		{"ARCS", func(w io.Writer) { reportArcs(w, core.NewRNG(*seed^0x5ACB), t, *fights/2) }},
		{"LANES", func(w io.Writer) { reportLanes(w, core.NewRNG(*seed^0x1A4E), t, *fights) }},
		{"DANGER", func(w io.Writer) { reportDanger(w, core.NewRNG(*seed^0xD1E), t, *fights/3) }},
		{"WARD", func(w io.Writer) { reportWard(w, core.NewRNG(*seed^0x3A7D), t, *fights) }},
		// Twice the sample, because it is measuring a derivative: a difference
		// of two rates carries both their noise, and the answer is then divided
		// by K. The seed goes in as a value rather than only through the
		// generator — core.RNG.Fork never reads its receiver, so a section that
		// only ever forks would run bit-identical streams at every -seed and
		// could never be checked by replication. Which is the gotcha this repo
		// already had written down.
		{"EXCHANGE", func(w io.Writer) { reportExchange(w, t, *fights*2, *seed^0xE7CB) }},
		{"PLAYSTYLES", func(w io.Writer) { reportPlaystyles(w, core.NewRNG(*seed^0x9147), t, *fights) }},
		{"CHARMS", func(w io.Writer) { reportCharms(w, core.NewRNG(*seed^0xC4A7), t, *fights/4) }},
		{"SHAPES", func(w io.Writer) { reportShapes(w, core.NewRNG(*seed^0x5411), t, *fights) }},
		{"CROWDS", func(w io.Writer) { reportCrowds(w, core.NewRNG(*seed^0xC70D), t, *fights/8) }},
		{"ENDURANCE", nil},   // chained
		{"PROGRESSION", nil}, // chained
		{"ECONOMY", func(w io.Writer) { reportEconomy(w, t) }},
		{"SAGA", func(w io.Writer) { reportSaga(w, t) }},
		{"SKY", func(w io.Writer) { reportSky(w) }},
		{"SUPPLIES", func(w io.Writer) { reportSupplies(w, t) }},
		{"COMPANY", func(w io.Writer) { reportCompany(w, core.NewRNG(*seed^0xC017), t) }},
		{"SPREAD", func(w io.Writer) { reportMonsterSpread(w, t) }},
	}
	// The chain, in the order it drew in when it was serial.
	chain := []struct {
		name string
		run  func(io.Writer)
	}{
		{"COMBAT", func(w io.Writer) { reportCombat(w, chained, t, *fights) }},
		{"ENDURANCE", func(w io.Writer) { reportEndurance(w, chained, t, *fights/4) }},
		{"PROGRESSION", func(w io.Writer) { reportProgression(w, chained, t, *fights/50) }},
	}
	runSections(out, sections, chain, *timings)
}

// parallelFor runs body(i) for every i below n, across the cores.
//
// **Only ever called where the work is order-independent, and that is a
// property of the generator rather than of the loop.** A section that draws
// from one stream in sequence produces different numbers the moment two
// iterations swap places, and half of this report does exactly that. The other
// half forks a fresh stream per iteration from a label and an index — and
// core.RNG.Fork never reads its receiver, so fight f draws the same dice
// whenever it runs and whoever runs it. Those loops, and no others, come
// through here.
//
// The check that it is honest is cheap and was run: the whole report, before
// and after, byte for byte.
func parallelFor(n int, body func(i int)) {
	if n <= 0 {
		return
	}
	// The caller is holding a slot and is about to stop working, so it gives it
	// up before asking for more. That is what keeps this from deadlocking when
	// every slot is held by a section that wants to fan out, and it is also
	// what keeps the two levels of parallelism from adding up: sections and the
	// cells inside them draw on one budget, so the machine runs NumCPU pieces
	// of work whichever shape they arrive in.
	//
	// Without it the report ran twenty goroutines on ten cores — ten sections
	// each spawning ten workers — and spent 149 seconds of CPU to save 14
	// seconds of wall clock. Contention is not parallelism.
	release()
	defer hold()

	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < cap(cores); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				hold()
				body(i)
				release()
			}
		}()
	}
	for i := 0; i < n; i++ {
		next <- i
	}
	close(next)
	wg.Wait()
}

// cores is the whole machine's budget, and everything that does work holds one
// slot of it: a whole section, or one cell of a section that has fanned out.
var cores = make(chan struct{}, runtime.NumCPU())

func hold()    { cores <- struct{}{} }
func release() { <-cores }

// section is one block of the report and the buffer it writes into.
//
// run is nil for the three that share the main generator; those are driven by
// the chain instead, which is the only ordering constraint left in here.
type section struct {
	name string
	run  func(io.Writer)
}

// runSections plays every section concurrently and prints them in order.
//
// The workers are bounded by the number of cores rather than let loose: each
// section holds a whole simulated character and its encounter, and twenty of
// those at once on a four-core machine is more contention than parallelism.
//
// A panic in a worker is re-raised on the main goroutine rather than swallowed.
// A report that silently lost a section would be worse than one that crashed —
// the missing block reads as "this was not measured" rather than as "this
// broke", and the two want telling apart.
func runSections(out io.Writer, sections []section, chain []struct {
	name string
	run  func(io.Writer)
}, timings bool) {
	bufs := make([]bytes.Buffer, len(sections))
	at := map[string]int{}
	for i, s := range sections {
		at[s.name] = i
	}

	type timing struct {
		name string
		dur  time.Duration
	}
	var mu sync.Mutex
	var took []timing
	note := func(name string, start time.Time) {
		mu.Lock()
		took = append(took, timing{name, time.Since(start)})
		mu.Unlock()
	}

	var wg sync.WaitGroup
	fail := make(chan any, len(sections)+1)

	// The chained three, in order, on one worker.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				fail <- r
			}
		}()
		hold()
		defer release()
		for _, c := range chain {
			start := time.Now()
			c.run(&bufs[at[c.name]])
			note(c.name, start)
		}
	}()

	for i := range sections {
		if sections[i].run == nil {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fail <- r
				}
			}()
			hold()
			defer release()
			start := time.Now()
			sections[i].run(&bufs[i])
			note(sections[i].name, start)
		}(i)
	}
	wg.Wait()
	close(fail)
	if r := <-fail; r != nil {
		panic(r)
	}

	for i := range bufs {
		_, _ = out.Write(bufs[i].Bytes())
	}

	if !timings {
		return
	}
	sort.Slice(took, func(i, j int) bool { return took[i].dur > took[j].dur })
	fmt.Fprintf(out, "TIMINGS — wall clock per section, slowest first\n")
	fmt.Fprintf(out, "the longest one is the floor on the whole report, which is what\n")
	fmt.Fprintf(out, "decides whether the next question is affordable\n\n")
	for _, tk := range took {
		fmt.Fprintf(out, "  %-12s %8.2fs\n", tk.name, tk.dur.Seconds())
	}
	fmt.Fprintln(out)
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

func reportCombat(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
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
func reportOpening(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
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
func reportDanger(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
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
func reportWard(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
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
// arcRuns is how many chains of on-level fights the endurance column averages
// over, per class. A chain costs what a dozen single fights do, so this is much
// smaller than the fight count beside it.
const arcRuns = 80

func reportArcs(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "ARCS — is there more than one way to be correctly levelled?\n")
	fmt.Fprintf(out, "one row per class, on the stretch fights three levels over, every build\n")
	fmt.Fprintf(out, "shopping with the same purse: what balanced costs that class at that level\n\n")
	for _, a := range gamedata.Archetypes {
		fmt.Fprintf(out, "  %-10s %s\n", a.Name, a.Note)
	}
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%-5s %-8s %-10s %7s %6s %8s %8s %9s\n",
		"level", "class", "build", "cost", "spent", "over", "rounds", "per rest")
	fmt.Fprintln(out, strings.Repeat("-", 74))

	// Per class, not averaged across the three, and this is the whole of what
	// this section got wrong for its entire life.
	//
	// Only a Fighter may hold a two-handed weapon, so the "duelist" row used to
	// average one real duelist, one build byte-identical to balanced (the Mage,
	// which cannot make the trade and falls back), and a Thief holding a
	// one-hander. At level thirteen the Fighter duelist beats balanced by five
	// to seven points and the Thief duelist loses by nine to eleven; the mean
	// of those is "within a point or two", which is what the table reported and
	// what this document concluded from. An average is not a measurement of
	// anything that exists.
	type cell struct {
		build string
		over  float64
		rest  float64
	}
	best := map[[2]int][]cell{} // level, class index -> builds
	spentThin := 0
	rows := 0

	for level := 1; level <= maxLevel; level += 2 {
		for ci, class := range model.AllClasses {
			purse := &model.Character{Level: level, Class: class}
			t.EquipAs(purse, gamedata.Archetypes[0])
			budget := gamedata.GearCost(purse)

			for _, a := range gamedata.Archetypes {
				var cost int
				rate := func(biome string, encLevel int) (winPct, rounds float64) {
					var wins, totalRounds, n int
					for i := 0; i < fights; i++ {
						c := rules.BuildCharacter(g, class, level)
						t.EquipWithin(c, a, budget)
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
						n++
					}
					if n == 0 {
						return 0, 0
					}
					return float64(wins) * 100 / float64(n), float64(totalRounds) / float64(n)
				}
				over, rounds := rate(biomeForLevel(level+3), level+3)

				// How far one rest goes, which is the axis attrition's premise
				// lives on — fights take longer and you are still standing at
				// the end — and which a win rate cannot see by construction.
				total, chains := 0, 0
				for i := 0; i < arcRuns; i++ {
					sim := rules.BuildCharacter(g, class, level)
					t.EquipWithin(sim, a, budget)
					sim.HP, sim.Psyche = sim.MaxHP, sim.MaxPsy()
					spells := t.SpellsFor(sim)
					for survived := 0; survived < 60; survived++ {
						mons := t.PickMonsters(g, biomeForLevel(level), level, 1)
						if len(mons) == 0 {
							break
						}
						r := rules.SimulateFight(g, sim, []*model.MonsterDef{mons[0].Def},
							level, 60, spells)
						if !r.Won || sim.HP <= 0 {
							break
						}
						total++
					}
					chains++
				}
				perRest := 0.0
				if chains > 0 {
					perRest = float64(total) / float64(chains)
				}

				spent := float64(cost) * 100 / float64(core.Max(1, budget))
				fmt.Fprintf(out, "%-5d %-8s %-10s %7d %5.0f%% %7.1f%% %8.1f %9.1f\n",
					level, class, a.Name, cost, spent, over, rounds, perRest)
				rows++
				if cost > budget {
					fmt.Fprintf(out, "      WARNING: %s outspends %s's purse by %d.\n",
						a.Name, class, cost-budget)
				}
				if spent < 90 {
					spentThin++
				}
				key := [2]int{level, ci}
				best[key] = append(best[key], cell{a.Name, over, perRest})
			}
		}
		fmt.Fprintln(out)
	}

	// The verdict, on both axes and with a threshold.
	//
	// LANES learned in the same session that reading a winner off whichever
	// column happens to be higher reports noise as a result; ARCS was still
	// doing it. A build takes a cell only when it leads by more than the wobble,
	// and both columns count — a build can be an arc by outlasting rather than
	// by out-hitting, which is exactly what attrition claims to be.
	const arcNoise = 2.0

	wonFight := map[string]int{}
	wonRest := map[string]int{}
	keys := make([][2]int, 0, len(best))
	for k := range best {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	worstGap := 0.0
	for _, k := range keys {
		cells := best[k]
		topF, topR := cells[0], cells[0]
		lowF := cells[0]
		for _, c := range cells[1:] {
			if c.over > topF.over {
				topF = c
			}
			if c.over < lowF.over {
				lowF = c
			}
			if c.rest > topR.rest {
				topR = c
			}
		}
		if gap := topF.over - lowF.over; gap > worstGap {
			worstGap = gap
		}
		// Only a clear lead counts.
		clear := true
		for _, c := range cells {
			if c.build != topF.build && topF.over-c.over < arcNoise {
				clear = false
			}
		}
		if clear {
			wonFight[topF.build]++
		}
		clear = true
		for _, c := range cells {
			if c.build != topR.build && topR.rest-c.rest < arcNoise/10 {
				clear = false
			}
		}
		if clear {
			wonRest[topR.build]++
		}
	}

	fmt.Fprintf(out, "cells won outright, of %d (level x class), by more than %.0f points\n",
		len(keys), arcNoise)
	fmt.Fprintf(out, "%-12s %10s %10s\n", "build", "the fight", "the rest")
	var never []string
	for _, a := range gamedata.Archetypes {
		fmt.Fprintf(out, "%-12s %10d %10d\n", a.Name, wonFight[a.Name], wonRest[a.Name])
		if wonFight[a.Name] == 0 && wonRest[a.Name] == 0 {
			never = append(never, a.Name)
		}
	}
	fmt.Fprintf(out, "\nwidest gap in any cell: %.1f points.\n", worstGap)
	if spentThin > 0 {
		fmt.Fprintf(out, "%d of %d rows came in more than a tenth under the purse — read the\n"+
			"spend column before believing any gap on those.\n", spentThin, rows)
	}

	switch {
	case len(never) > 0:
		fmt.Fprintf(out, "VERDICT: %s takes no cell on either axis. A build that is never the\n"+
			"best one anywhere is not a playstyle, it is a trap with a name.\n",
			strings.Join(never, " and "))
	case worstGap <= 10:
		fmt.Fprintf(out, "VERDICT: no build is ever far behind and each wins somewhere, on one\n"+
			"axis or the other. The content supports more than one arc.\n")
	default:
		fmt.Fprintf(out, "VERDICT: each build wins somewhere, but the gap is wide enough that\n"+
			"picking wrong for the level is a real mistake.\n")
	}

	fmt.Fprint(out, `
Read the class column before the build column. A build is only a build for
the classes that can make its trade: only a Fighter may hold a two-handed
weapon, so "duelist" means the two-hander for a Fighter, a fallback
one-hander for a Thief, and — for a Mage, who cannot hold one at all — a
build identical to balanced. Averaging those three into one row is how this
table spent its whole life reporting that the two-handed lane changes
nothing, while the Fighter duelist beat the baseline by five to seven points
at level thirteen and the Thief lost by nine to eleven.

The spend column is the second thing to read. Every build shops with the same
purse, but gear is banded and a build whose next upgrade costs more than it
has left simply stops — so a row several points behind at ninety per cent of
the money is not a verdict on a shape, it is a floor on one.

`)
	fmt.Fprintln(out)
	reportSlotValue(out, t)
}

// reportCrowds is how each class holds up as the numbers grow, and it exists
// because nothing else in this report could answer that.
//
// Two blind spots met here. SHAPES measures compositions but rotates the class
// per fight, so it reports an average and cannot see a class that cannot fight
// a crowd. And the stretch column — three levels over, which is how every build
// in this document is compared — is *saturated* for groups: three creatures
// three over is a win rate of nought for every class including the Fighter, so
// the comparison every conclusion rests on says nothing whatever about group
// fights. On level is the only place they are legible.
//
// It matters because groups are most of the game. SHAPES puts mixed at 42-57%
// of encounters and packs at 21-25%, so a class that folds against three is a
// class that folds against the majority of what the world throws.
//
// What the table is looking for is whether a defensive unit scales with the
// number of attackers. Flat reduction comes off *every* blow, so armour is
// worth more the more blows arrive; a chance to take nothing is worth the same
// share whatever the count. Those two should diverge as the field fills, and if
// they diverge too far the scheme has produced a class that cannot play half
// the encounter table.
func reportCrowds(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "CROWDS — how each class holds up as the numbers grow\n")
	fmt.Fprintf(out, "on level, because three levels over is a nought for everybody at three\n")
	fmt.Fprintf(out, "or more; the stretch column cannot see group fights at all\n\n")

	sizes := []int{1, 2, 3, 4, 6}
	fmt.Fprintf(out, "%-6s %-8s", "level", "class")
	for _, n := range sizes {
		fmt.Fprintf(out, " %8s", fmt.Sprintf("%d up", n))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, strings.Repeat("-", 60))

	// Per level as well as per size. A "widest gap" taken across every level at
	// once compares a Fighter at eleven with a Mage at thirteen and calls the
	// difference a fact about classes — which is the aggregation defect this
	// report has been finding all month, and this section had it before it
	// shipped.
	type cell struct{ level, size int }
	rate := map[cell]map[model.Class]float64{}
	trails := map[model.Class]int{}
	for _, level := range []int{3, 5, 7, 9, 11, 13} {
		for _, class := range model.AllClasses {
			fmt.Fprintf(out, "%-6d %-8s", level, class)
			for _, size := range sizes {
				var wins, n int
				for i := 0; i < fights; i++ {
					c := rules.BuildCharacter(g, class, level)
					t.Equip(c)
					mons := t.PickMonsters(g, biomeForLevel(level), level, size)
					if len(mons) == 0 {
						continue
					}
					fresh := *c
					r := rules.SimulateGroup(g, &fresh, mons, 60, t.SpellsFor(c))
					if r.Won {
						wins++
					}
					n++
				}
				pct := 0.0
				if n > 0 {
					pct = float64(wins) * 100 / float64(n)
				}
				fmt.Fprintf(out, " %7.1f%%", pct)
				k := cell{level, size}
				if rate[k] == nil {
					rate[k] = map[model.Class]float64{}
				}
				rate[k][class] = pct
			}
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "widest gap between two classes at the same level\n")
	for _, size := range sizes {
		gap, at := 0.0, 0
		for _, level := range []int{3, 5, 7, 9, 11, 13} {
			row := rate[cell{level, size}]
			if len(row) == 0 {
				continue
			}
			hi, lo := -1.0, 101.0
			var low model.Class
			for _, class := range model.AllClasses {
				if v := row[class]; v > hi {
					hi = v
				} else if v < lo {
					lo, low = v, class
				}
				if row[class] < lo {
					lo, low = row[class], class
				}
			}
			if hi-lo > gap {
				gap, at = hi-lo, level
			}
			// Only count a trailing class where the fight is still winnable
			// for somebody: everybody losing is not a fact about a class.
			if hi > 10 && hi-lo > 15 {
				trails[low]++
			}
		}
		fmt.Fprintf(out, "  %d up: %.1f points, at level %d\n", size, gap, at)
	}
	for _, class := range model.AllClasses {
		if trails[class] >= 6 {
			fmt.Fprintf(out, "WARNING: %s is the class left behind in %d of the "+
				"level-and-size cells where the fight is winnable at all.\n",
				class, trails[class])
		}
	}

	// What the world actually rolls, which is the column-weighting the table
	// above has never had.
	//
	// The open list said the CROWDS numbers were measured and untuned, and that
	// whether they are correct "depends on how often the world sends them,
	// which EncounterSize and the pack shape decide and no section reads
	// together". This is those two read together. It is a count rather than a
	// simulation: roll the size exactly as the overworld rolls it — base one or
	// two, plus nought to the number of allies — hand it to PickEncounter, and
	// tally how many creatures come back once the shape has had its say. A pack
	// adds two bodies to whatever it was given, an escort adds its guards, so
	// the size that goes in is not the number that arrives.
	fmt.Fprintf(out, "what the world rolls, per company — %% of encounters by creature count\n")
	fmt.Fprintf(out, "%-12s", "company")
	for n := 1; n <= 6; n++ {
		fmt.Fprintf(out, "%7d", n)
	}
	fmt.Fprintf(out, "%9s\n", "mean")
	fmt.Fprintln(out, strings.Repeat("-", 63))
	for allies := 0; allies <= 2; allies++ {
		count := map[int]int{}
		total, bodies := 0, 0
		for i := 0; i < fights*4; i++ {
			level := 3 + g.Intn(11)
			size := party.EncounterSize(g, 1+g.Intn(2), allies)
			enc := t.PickEncounter(g, biomeForLevel(level), level, size)
			if len(enc.Monsters) == 0 {
				continue
			}
			count[len(enc.Monsters)]++
			bodies += len(enc.Monsters)
			total++
		}
		if total == 0 {
			continue
		}
		label := "solo"
		if allies == 1 {
			label = "+1 ally"
		} else if allies > 1 {
			label = fmt.Sprintf("+%d allies", allies)
		}
		fmt.Fprintf(out, "%-12s", label)
		for n := 1; n <= 6; n++ {
			if count[n] == 0 {
				fmt.Fprintf(out, "%7s", ".")
				continue
			}
			fmt.Fprintf(out, "%6.0f%%", float64(count[n])*100/float64(total))
		}
		fmt.Fprintf(out, "%9.2f\n", float64(bodies)/float64(total))
	}

	fmt.Fprint(out, `
Which is the weighting to read the table above with, and it says the columns
are not equally interesting. A solo hero meets one or two creatures in three
quarters of encounters and three or more in the remaining quarter, so the
right-hand columns are a tail rather than a curiosity — a quarter is often
enough to be the thing that kills you. Hiring people moves the whole
distribution rightward, mean 1.90 to 2.62, which is the point of
EncounterSize: a party is not a discount on the fights, it is a different set
of them.

And the honest limit of all of this, which is the *actual* content of "CROWDS
is measured and untuned": every row above is one character against N
creatures, because there is no party simulator. rules.SimulateGroup takes one
Character. So the moment the company is what changes the rolls, the report can
price what the world sends but not what the party does about it, and the four-
and six-creature columns keep meaning "a solo hero after their company was
killed" rather than "what a party of three walks into". That is one missing
instrument, not a tuning pass, and it is the thing to build before anybody
tunes these numbers.

`)

	fmt.Fprint(out, `
Read the row across rather than the column down. A class that is level with
the others one-on-one and far behind at three has a defence that does not
scale with the number of attackers, and that is a real property of the
arithmetic rather than a tuning error: flat reduction comes off every blow,
so armour is worth more the more blows arrive, while a chance to take
nothing is worth the same share whatever the count.

That is the scheme working as designed. It stops being fine if the gap at
three or four is wide enough that one class cannot play the majority of the
encounter table - SHAPES puts mixed at 42-57% of what the world throws and
packs at another 21-25%.

Three things the columns do not say for themselves. These are *on-level*
creatures, and the game's own multi-creature encounters are not: a pack
shape draws its bodies a tier lower, which is why SHAPES reports a solo
hero winning 86% at level three while the three-creature column here reads
18%. So the absolute numbers are a worst case — "ambushed by three of your
own size" — and the reading is the comparison across a row, where every
class faces the same thing.

Four and six creatures are
party-sized rolls measured against one character, because EncounterSize
scales what the world sends with how many people are walking behind you - so
those columns are "what a solo hero meets after their company has been
killed", not what a party of three walks into. And read the level column as a
sawtooth rather than a curve: gear steps at 4, 7, 10 and 13 and the monsters
do not, so a character decays about thirteen points of win rate inside each
band and gets it back at the next one. Twelve is the hard level and thirteen
the easy one, and neither is a fact about a class.

`)
}

// reportLanes is the one measurement that decides gamedata.LaneForLevel, and
// it exists because that number used to be a lane nobody had ever compared.
//
// The off arm has three lanes and the balanced build has to pick one. For the
// whole life of this report it picked the wall, not because anything had been
// measured but because ArmBlock is the zero value of the field. A retired
// archetype called "warden" eventually asked the question by accident — it was
// balanced with the silvered shield and nothing else changed — and beat the
// baseline at identical spend from level seven up, by as much as 11.2 points.
//
// So the comparison is permanent now, and it is a clean one: the three builds
// below differ in exactly one slot. The cost column is *not* a proof that they
// spend the same — it is one lane's kit, printed once per level, and a reviewer
// was right to call the old claim theatre. The lanes differ by up to 175% at
// tier one; what makes the comparison sound is that a sidearm is a small part
// of an outfit, so the largest of those gaps moves a whole kit by about one per
// cent, against lane differences reaching fourteen.
//
// Three things changed after the first draft of this section was read back,
// and all three were the instrument rather than the content.
//
// It measured one axis, and it was the wrong one to measure alone. Win rate on
// the stretch fights is what the spiked lane is *built* to win: it trades guard
// for strike, so asking only "did you kill it" hands it the comparison before a
// die is rolled. An off arm is defensive gear and the question that binds it is
// whether you get out of a fight you should never have taken — so the second
// axis is the death rate five levels over, which DANGER already establishes is
// the band where the death-versus-flee split is the only thing still legible.
// A lane that wins both is better. A lane that wins one is a choice, and the
// baseline's job is to say out loud which choice it is making.
//
// It averaged three classes into one verdict, which ARCS had to be talked out
// of separately, and here it is worse than an average usually is: a Mage cannot
// hold a plank at all, so a third of the average was three copies of one number
// that could not move.
//
// And it called a gap of one point a result. The threshold was a named constant
// with a paragraph of reasoning attached and no measurement under it, while the
// table it gated already contained the experiment: every row where all three
// lanes dress the character identically is three identical builds measured
// three times, and the spread across those columns is sampling wobble by
// construction. There are twenty of them, they spread by up to 4.3 points at
// three times the old sample, and the constant was 1.0. The floor is read off
// them now, and the rows are marked in the table so the reader can check it.
//
// Anything that moves the shield tables, the monster rosters' magic, or the
// level bands will show up here as the crossover moving, and the constant in
// gamedata has to move with it.
func reportLanes(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "LANES — which off-arm lane is right, for whom, and from when\n")
	fmt.Fprintf(out, "identical builds differing in one slot, on both of DANGER's legible\n")
	fmt.Fprintf(out, "bands: won three levels over, died five levels over, %d fights a cell\n\n",
		fights)

	// In the order the columns print.
	lanes := []model.SidearmLane{model.ArmBlock, model.ArmStrike, model.ArmWard}
	laneNames := []string{"wall", "spiked", "silvered"}

	// The top of the game, averaged, because the last three levels are where
	// the lanes have separated and any one level of it is a sample.
	const laneTop = 12

	type cell struct {
		won, died, fled float64
		// diedSteel and diedMagic split the death rate by what killed you,
		// which is the only place the ward lane can possibly be worth its
		// price. See the matchup block at the bottom of the section.
		diedSteel, diedMagic float64
		item                 string
		cost                 int
	}

	// run fights one lane's build in one band and reports both rates off the
	// same batch, so a cell's two numbers are never each other's excuse.
	run := func(a gamedata.Archetype, class model.Class, level, delta int) (won, died, fled, diedSteel, diedMagic float64, item string, cost int) {
		enc := core.Max(1, level+delta)
		biome := biomeForLevel(enc)
		var wins, deaths, flights, n int
		var steelDeaths, steelN, magicDeaths, magicN int
		for f := 0; f < fights; f++ {
			c := rules.BuildCharacter(g, class, level)
			t.EquipAs(c, a)
			item, cost = c.Shield.Name, gamedata.GearCost(c)
			mons := t.PickMonsters(g, biome, enc, 1)
			if len(mons) == 0 {
				continue
			}
			def := mons[0].Def
			fresh := *c
			r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{def},
				enc, 60, t.SpellsFor(c))
			if r.Won {
				wins++
			}
			if r.Died() {
				deaths++
			}
			if r.Fled {
				flights++
			}
			if def.Magic {
				magicN++
				if r.Died() {
					magicDeaths++
				}
			} else {
				steelN++
				if r.Died() {
					steelDeaths++
				}
			}
			n++
		}
		if n > 0 {
			won = float64(wins) * 100 / float64(n)
			died = float64(deaths) * 100 / float64(n)
			fled = float64(flights) * 100 / float64(n)
		}
		if steelN > 0 {
			diedSteel = float64(steelDeaths) * 100 / float64(steelN)
		}
		if magicN > 0 {
			diedMagic = float64(magicDeaths) * 100 / float64(magicN)
		}
		return
	}

	// grid[level][class] is the three lanes in the order above.
	grid := make([]map[model.Class][]cell, maxLevel+1)
	for level := 1; level <= maxLevel; level++ {
		grid[level] = map[model.Class][]cell{}
		for _, class := range model.AllClasses {
			cs := make([]cell, len(lanes))
			for i, lane := range lanes {
				a := gamedata.Archetypes[0]
				a.Arm = lane
				won, _, _, _, _, item, cost := run(a, class, level, laneStretch)
				_, died, fled, steel, magic, _, _ := run(a, class, level, laneOver)
				cs[i] = cell{won: won, died: died, fled: fled,
					diedSteel: steel, diedMagic: magic, item: item, cost: cost}
			}
			grid[level][class] = cs
		}
	}

	// A null row is one where all three lanes dressed the character
	// identically: a Mage at any level, since it cannot hold a plank and
	// pickSidearm falls back to the talisman whichever lane is asked for, and
	// anybody at all below the level where the balanced build affords an off
	// arm. It is a free experiment with a known answer, and this section is the
	// one that needed one.
	null := func(cs []cell) bool {
		return cs[0].item == cs[1].item && cs[1].item == cs[2].item
	}
	spread := func(vs ...float64) float64 {
		lo, hi := vs[0], vs[0]
		for _, v := range vs {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
		return hi - lo
	}

	fmt.Fprintf(out, "%22s%28s%29s\n", "", "won on +3 fights", "died on +5 fights")
	fmt.Fprintf(out, "%-6s %-8s %6s", "level", "class", "cost")
	for pass := 0; pass < 2; pass++ {
		for _, n := range laneNames {
			fmt.Fprintf(out, " %8s", n)
		}
		fmt.Fprintf(out, " ")
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, strings.Repeat("-", 79))

	var nullWon, nullDied []float64
	for level := 1; level <= maxLevel; level++ {
		for _, class := range model.AllClasses {
			cs := grid[level][class]
			fmt.Fprintf(out, "%-6d %-8s %6d", level, class, cs[0].cost)
			for _, c := range cs {
				fmt.Fprintf(out, " %7.1f%%", c.won)
			}
			fmt.Fprintf(out, " ")
			for _, c := range cs {
				fmt.Fprintf(out, " %7.1f%%", c.died)
			}
			if null(cs) {
				fmt.Fprintf(out, " null")
				nullWon = append(nullWon, spread(cs[0].won, cs[1].won, cs[2].won))
				nullDied = append(nullDied, spread(cs[0].died, cs[1].died, cs[2].died))
			}
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out)
	}

	// What the lanes cost, because the whole comparison leans on them costing
	// about the same and the old section asserted that rather than printing it.
	// One row per band, since a band is where the price steps.
	fmt.Fprintf(out, "what the three lanes cost, per band, since the comparison leans on it\n")
	fmt.Fprintf(out, "%-6s %-8s %8s %8s %8s   %s\n",
		"level", "class", "wall", "spiked", "silvered", "widest gap, as a share of the kit")
	fmt.Fprintln(out, strings.Repeat("-", 79))
	for _, class := range model.AllClasses {
		last := -1
		for level := 1; level <= maxLevel; level++ {
			cs := grid[level][class]
			if cs[0].cost == last {
				continue
			}
			last = cs[0].cost
			gap := spread(float64(cs[0].cost), float64(cs[1].cost), float64(cs[2].cost))
			fmt.Fprintf(out, "%-6d %-8s %8d %8d %8d   %.1f%%\n",
				level, class, cs[0].cost, cs[1].cost, cs[2].cost,
				gap*100/float64(cs[0].cost))
		}
	}
	fmt.Fprintln(out)

	stats := func(xs []float64) (mean, worst float64) {
		for _, x := range xs {
			mean += x / float64(len(xs))
			if x > worst {
				worst = x
			}
		}
		return
	}
	meanWon, worstWon := stats(nullWon)
	meanDied, worstDied := stats(nullDied)
	fmt.Fprintf(out, "the noise floor, measured rather than asserted\n")
	fmt.Fprintf(out, "  %d rows above are three identical builds measured three times, so the\n", len(nullWon))
	fmt.Fprintf(out, "  spread across their columns is sampling wobble and nothing else:\n")
	fmt.Fprintf(out, "  won   mean %.1f  worst %.1f       died  mean %.1f  worst %.1f\n",
		meanWon, worstWon, meanDied, worstDied)
	fmt.Fprintf(out, "  Nothing below is called until it clears the worst of them. That is\n")
	fmt.Fprintf(out, "  deliberately conservative, because it gates a change to a constant, and\n")
	fmt.Fprintf(out, "  it is a floor measured at the null rows' own rates rather than at every\n")
	fmt.Fprintf(out, "  row's — a Fighter at 60%% and a Mage at 85%% do not wobble equally.\n\n")

	// best returns the winning lane in a set of three and how far clear of the
	// runner-up it is. Higher wins on won, lower wins on died.
	best := func(cs []cell, died bool) (lane int, margin float64) {
		v := func(c cell) float64 {
			if died {
				return -c.died
			}
			return c.won
		}
		for i := range cs {
			if v(cs[i]) > v(cs[lane]) {
				lane = i
			}
		}
		second := 0
		if lane == 0 {
			second = 1
		}
		for i := range cs {
			if i != lane && v(cs[i]) > v(cs[second]) {
				second = i
			}
		}
		return lane, v(cs[lane]) - v(cs[second])
	}

	// chooses reports whether this class ever has a lane to pick, which is the
	// question a Mage answers no to at every level.
	chooses := func(class model.Class) bool {
		for level := 1; level <= maxLevel; level++ {
			if !null(grid[level][class]) {
				return true
			}
		}
		return false
	}

	fmt.Fprintf(out, "crossover, per class and per axis\n")
	fmt.Fprintf(out, "  the first level from which the wall never again comes within the noise\n")
	fmt.Fprintf(out, "  floor of the best lane — \"never again\", because one level going the\n")
	fmt.Fprintf(out, "  other way is a sample and not a change of régime\n")
	crossed := map[model.Class][2]int{}
	for _, class := range model.AllClasses {
		if !chooses(class) {
			fmt.Fprintf(out, "  %-9s no lane to choose: every row is the talisman\n", class)
			continue
		}
		var got [2]int
		for axis := 0; axis < 2; axis++ {
			died := axis == 1
			floor := worstWon
			if died {
				floor = worstDied
			}
			for level := maxLevel; level >= 1; level-- {
				cs := grid[level][class]
				lane, _ := best(cs, died)
				gap := cs[lane].won - cs[0].won
				if died {
					gap = cs[0].died - cs[lane].died
				}
				if gap < floor {
					break
				}
				got[axis] = level
			}
		}
		crossed[class] = got
		fmt.Fprintf(out, "  %-9s won %-10s lived %s\n", class,
			laneCross(got[0]), laneCross(got[1]))
	}
	fmt.Fprintf(out, "  Equip leaves the wall at %d, for every class alike, and the answer to\n",
		gamedata.StrikeFromLevel())
	fmt.Fprintf(out, "  whether that constant wants a class is the two rows above it.\n\n")

	fmt.Fprintf(out, "the top of the game — levels %d-%d averaged, one level being a sample\n",
		laneTop, maxLevel)
	fmt.Fprintf(out, "%-8s %-8s %8s %8s %8s   %s\n",
		"class", "axis", "wall", "spiked", "silvered", "best, and by how much")
	fmt.Fprintln(out, strings.Repeat("-", 72))
	// Kept, because the matchup block below reads the same averages rather than
	// recomputing them and risking two answers to one question.
	top := map[model.Class][]cell{}
	for _, class := range model.AllClasses {
		cs := make([]cell, len(lanes))
		top[class] = cs
		for i := range lanes {
			n := float64(maxLevel - laneTop + 1)
			for level := laneTop; level <= maxLevel; level++ {
				cs[i].won += grid[level][class][i].won / n
				cs[i].died += grid[level][class][i].died / n
				cs[i].fled += grid[level][class][i].fled / n
				cs[i].diedSteel += grid[level][class][i].diedSteel / n
				cs[i].diedMagic += grid[level][class][i].diedMagic / n
				cs[i].item = grid[level][class][i].item
			}
		}
		// The two axes, and between them the rest of what happens five over,
		// because "died less" has two possible mechanisms and the ratio of
		// these three rows says which. A lane can die less by getting away —
		// the flee roll reads Speed, and two of these three planks charge two
		// points of it — or by killing the thing. The rows are printed in
		// outcome order so the reader can see which.
		for _, row := range []struct {
			label  string
			vals   []float64
			better func(a, b float64) bool
			floor  float64
		}{
			{"won +3", []float64{cs[0].won, cs[1].won, cs[2].won},
				func(a, b float64) bool { return a > b }, worstWon},
			{"won +5", []float64{100 - cs[0].fled - cs[0].died,
				100 - cs[1].fled - cs[1].died, 100 - cs[2].fled - cs[2].died}, nil, 0},
			{"fled +5", []float64{cs[0].fled, cs[1].fled, cs[2].fled}, nil, 0},
			{"died +5", []float64{cs[0].died, cs[1].died, cs[2].died},
				func(a, b float64) bool { return a < b }, worstDied},
		} {
			call := "how, not whether"
			if row.better != nil {
				lane, second := 0, 1
				for i, v := range row.vals {
					if row.better(v, row.vals[lane]) {
						lane = i
					}
				}
				if lane == 0 {
					second = 1
				} else {
					second = 0
				}
				for i, v := range row.vals {
					if i != lane && row.better(v, row.vals[second]) {
						second = i
					}
				}
				margin := row.vals[lane] - row.vals[second]
				if margin < 0 {
					margin = -margin
				}
				switch {
				case null(cs):
					call = "no choice: the talisman"
				case margin < row.floor:
					call = "nothing in it"
				default:
					call = fmt.Sprintf("%s by %.1f", laneNames[lane], margin)
				}
			}
			fmt.Fprintf(out, "%-8s %-8s %7.1f%% %7.1f%% %7.1f%%   %s\n",
				class, row.label, row.vals[0], row.vals[1], row.vals[2], call)
		}
	}

	// The matchup, which is the one place the ward lane can be worth its price
	// and the one place LANES was not looking.
	//
	// Every other number in this section averages over whatever the roster
	// throws, and the roster at the rim is a bit over half magical — so a lane
	// that only pays against casters is diluted with the fights it was never
	// for. The design says the ward slot is skippable early and progressively
	// worse to skip later, which is a claim about a *matchup*, and averaging is
	// exactly the operation that hides one. The report has been quoting "the
	// ward lane is never the answer" without ever having asked the question in
	// the form the design states it.
	fmt.Fprintf(out, "\nthe matchup — %d-%d, the death rate at +5 split by what is swinging\n",
		laneTop, maxLevel)
	fmt.Fprintf(out, "%-8s %-8s %8s %8s %8s   %s\n",
		"class", "against", "wall", "spiked", "silvered", "best")
	fmt.Fprintln(out, strings.Repeat("-", 64))
	wardPays := false
	for _, class := range model.AllClasses {
		cs := top[class]
		if null(cs) {
			continue
		}
		for _, row := range []struct {
			label string
			vals  [3]float64
		}{
			{"steel", [3]float64{cs[0].diedSteel, cs[1].diedSteel, cs[2].diedSteel}},
			{"magic", [3]float64{cs[0].diedMagic, cs[1].diedMagic, cs[2].diedMagic}},
		} {
			lane := 0
			for i, v := range row.vals {
				if v < row.vals[lane] {
					lane = i
				}
			}
			second := row.vals[0]
			for i, v := range row.vals {
				if i != lane && (second == row.vals[lane] || v < second) {
					second = v
				}
			}
			call := laneNames[lane]
			if margin := second - row.vals[lane]; margin < worstDied {
				call = "nothing in it"
			} else {
				call = fmt.Sprintf("%s by %.1f", laneNames[lane], margin)
				if lanes[lane] == model.ArmWard && row.label == "magic" {
					wardPays = true
				}
			}
			fmt.Fprintf(out, "%-8s %-8s %7.1f%% %7.1f%% %7.1f%%   %s\n",
				class, row.label, row.vals[0], row.vals[1], row.vals[2], call)
		}
	}
	if wardPays {
		fmt.Fprintf(out, "\n  The ward lane wins the fight it was designed for. The band trades on\n")
		fmt.Fprintf(out, "  matchup, which is the design as written, and the open question is\n")
		fmt.Fprintf(out, "  whether the world sends enough casters to make carrying it pay.\n")
	} else {
		fmt.Fprintf(out, "\n  The ward lane does not win even against casters, which is where its\n")
		fmt.Fprintf(out, "  whole case was. That is not an averaging artefact and no amount of\n")
		fmt.Fprintf(out, "  reweighting the roster rescues it: the item is mispriced.\n")
	}

	// And the fourth thing that can go on the arm, for the one class that may.
	//
	// A second weapon is not a lane — it is not a shield at all, which is the
	// point of it: it is priced on the weapon table, which steps about five a
	// band where the sidearm table steps two. So it does not belong in the
	// three columns above, and it does belong on the same page, because the
	// question it answers is the same one: what is that arm worth.
	//
	// Half the weapon's strike reaches the blow. Full strike would make it the
	// best thing on the arm at every level and turn the slot back into a
	// ladder, which is the state the three lanes were rescued from.
	fmt.Fprintf(out, "\nthe fourth option — a weapon on the arm, for the class that may hold one\n")
	fmt.Fprintf(out, "%-8s %-6s %8s %8s %8s   %s\n",
		"level", "axis", "lane", "sidearm", "kit", "against the lane it replaces")
	fmt.Fprintln(out, strings.Repeat("-", 74))
	// Five levels rather than three. The first draft sampled 5/9/13 and the
	// death axis came back +4.0 at nine against a floor of 3.6 — one cell over
	// the line with no way to tell an effect from a coin, since LANES' own
	// "two consecutive levels" rule needs neighbours to apply and this table
	// had none.
	for _, level := range []int{5, 7, 9, 11, 13} {
		for _, class := range model.AllClasses {
			probe := &model.Character{Class: class, Level: level}
			side := gamedata.Archetypes[0]
			side.OffHand = true
			t.EquipAs(probe, side)
			if !probe.Sidearm.Worn() {
				continue
			}
			// Both axes, for the reason the three lanes get both: the trade
			// being priced is strike for guard, and guard is spent on not
			// dying. Reading only the win rate three over would leave the
			// column the cost lands in unmeasured — which is what the first
			// draft did, and the level-nine row was the one that needed it.
			laneWon, _, _, _, _, _, laneCost := run(gamedata.Archetypes[0], class, level, laneStretch)
			armWon, _, _, _, _, _, armCost := run(side, class, level, laneStretch)
			_, laneDied, _, _, _, _, _ := run(gamedata.Archetypes[0], class, level, laneOver)
			_, armDied, _, _, _, _, _ := run(side, class, level, laneOver)

			for _, row := range []struct {
				label      string
				lane, arm  float64
				floor      float64
				betterDown bool
			}{
				{"won +3", laneWon, armWon, worstWon, false},
				{"died +5", laneDied, armDied, worstDied, true},
			} {
				d := row.arm - row.lane
				if row.betterDown {
					d = -d
				}
				call := fmt.Sprintf("%+.1f", d)
				if d < row.floor && d > -row.floor {
					call = "nothing in it"
				}
				cost := ""
				if !row.betterDown {
					cost = fmt.Sprintf("%+d", armCost-laneCost)
				}
				fmt.Fprintf(out, "%-8s %-6s %7.1f%% %7.1f%% %8s   %s\n",
					fmt.Sprintf("%d %s", level, class), row.label,
					row.lane, row.arm, cost, call)
			}
		}
	}
	fmt.Fprint(out, `
The kit column is what the swap does to the whole outfit's price, and it is
small because the two shelves are priced within a band of each other on
purpose — an off-hand weapon is bought instead of a plank, not as well as one.
It was reading as a large saving until GearCost was told the fifth slot
exists: the dagger was entering the kit for free, so the column was printing
the plank's price back as a discount.

What to want from these rows is *nothing in it*. A weapon on the arm that
beat the lane by more than the floor would be the ladder again with an extra
rung; one that lost by more than the floor would be a shelf nobody should
buy from. The thief now has four things it can put on that arm and the
report says which of them is a mistake, which is all this section has ever
been for.

`)

	// What this section exists to check, and it is not the crossover.
	//
	// The crossover is a derived number and comparing it against the constant
	// warns about arithmetic rather than about the game: a Thief's defensive
	// crossover lands three levels after a Fighter's purely because its gaps
	// spend three levels sitting just under the noise floor while pointing the
	// same way, and a constant set anywhere in that range does the Thief no
	// harm at all. The question that matters is the one the constant actually
	// decides — is the lane Equip hands ever behind the shelf? That catches
	// switching late, switching early and switching to the wrong thing, which
	// is the whole space of ways this can be wrong, and it is asked against
	// what LaneForLevel returns rather than against a lane named here, since a
	// check that names its own expected answer has stopped being a check.
	//
	// Two consecutive levels, and the same challenger both times. Fifty-odd
	// comparisons against a floor read off twenty null rows will throw single
	// levels over it whatever is true — three of them here, and they nominate a
	// different lane each time, which is what noise looks like when you ask it
	// to name a winner. A real advantage is persistent and belongs to one lane.
	// This is the same "and never again" rule the crossover uses, for the same
	// reason, applied to the thing that matters.
	fmt.Fprintf(out, "\nagainst the constant — is the lane Equip hands ever measurably behind?\n")
	agrees := true
	for _, class := range model.AllClasses {
		if !chooses(class) {
			continue
		}
		for axis, name := range []string{"won", "lived"} {
			died := axis == 1
			floor := worstWon
			if died {
				floor = worstDied
			}
			for ch := range lanes {
				// run is how many consecutive levels this challenger has been
				// clear of what Equip hands, and total is by how much.
				run, total := 0, 0.0
				flush := func(end int) {
					if run >= 2 {
						fmt.Fprintf(out, "  WARNING: %s at %d-%d holds the %s lane; the %s one is %.1f\n",
							class, end-run+1, end,
							laneNames[laneIndex(lanes, gamedata.LaneForLevel(end))],
							laneNames[ch], total/float64(run))
						fmt.Fprintf(out, "           better on %s. LaneForLevel is wrong there.\n", name)
						agrees = false
					}
					run, total = 0, 0
				}
				for level := 1; level <= maxLevel; level++ {
					cs := grid[level][class]
					held := laneIndex(lanes, gamedata.LaneForLevel(level))
					gap := -1.0
					if !null(cs) && ch != held {
						if died {
							gap = cs[held].died - cs[ch].died
						} else {
							gap = cs[ch].won - cs[held].won
						}
					}
					if gap > floor {
						run, total = run+1, total+gap
						continue
					}
					flush(level - 1)
				}
				flush(maxLevel)
			}
		}
	}
	if agrees {
		fmt.Fprintf(out, "  no: not for either class that has the choice, on either axis, at any\n")
		fmt.Fprintf(out, "  two levels running. And the two classes' crossovers above sit within a\n")
		fmt.Fprintf(out, "  level of each other on the axis that can see them, which is the answer\n")
		fmt.Fprintf(out, "  to whether this constant wants a class parameter. It does not.\n")
	}

	fmt.Fprint(out, `
The wall is not a mistake below the crossover and the table is not saying it
is. Three cells below level ten do put a lane clear of it — and they nominate
a different lane each time, which is how fifty-odd comparisons against a
measured floor behave. What the arm is, below level ten, is a slot where the
choice does not matter; at the top of the game it is the widest gap on this
page.

The ward lane is the one to watch. It is the best of the three in one cell
of the twenty-eight above, and at the top of the game it is the worst thing
you can put on the arm on the axis that kills you: fewest escapes, most
deaths, and the trade it makes to get there is three points of guard for
eleven of ward. (Eleven, not fifteen: the balanced build's off arm is a band
behind its own tier, so the fifteen-ward shield at the top of the shelf is
never the one in this table. A closing paragraph describing an item its own
table does not measure is exactly the kind of thing this report should not
do.) A band of three where
one is never the answer has stopped being a choice — which is the same
finding the charm bands have, in a different slot, and it is not fixed here.

`)
}

// laneIndex is which column a lane prints in, for the verdict lines that have
// to name a lane the constant chose rather than one this section named.
func laneIndex(lanes []model.SidearmLane, want model.SidearmLane) int {
	for i, l := range lanes {
		if l == want {
			return i
		}
	}
	return 0
}

// laneCross renders a crossover level, or the absence of one.
func laneCross(level int) string {
	if level == 0 {
		return "never"
	}
	return fmt.Sprintf("level %d", level)
}

// reportExchange is what one point in each stat is actually worth, and it
// exists because three tables in this game are priced in points and none of
// them was ever told the rate.
//
// A shield that pays three points of guard for eleven of ward is a trade the
// content author has to price. Until now the pricing was taste: the ward lane
// carries five to fifteen points of its stat where the strike lane carries two
// to eight, which looks like the ward one is being generous and is only a
// statement about which numbers were typed. Whether it is generous depends on
// the exchange rate, and the exchange rate is measurable — so it is measured
// here rather than argued about, and the three content tables that spend in
// these units (shields, charms, affixes) can be authored against it.
//
// The method is a nudge. Take the balanced build, add K points of one stat to
// whatever is already carrying it, and read the difference off the same two
// bands LANES uses: win rate on the stretch fights, death rate five over. The
// answer is per point, so the columns compare directly across stats.
//
// Three things to be careful of when reading it, all of them the reason the
// numbers are printed rather than reduced to one figure:
//
//   - **It is a local derivative, not a price.** A point of ward is worth
//     nothing at level five because nothing casts there, and it does not scale
//     linearly at thirteen either: ward is subtracted from a roll and clamped
//     at zero, so the tenth point is worth less than the first once it starts
//     taking whole hits to nothing. K is deliberately small for that reason.
//   - **It is per class.** A point of speed is not the same purchase for a
//     Thief who dodges with it as for a Fighter who does not.
//   - **The bands saturate at both ends.** On level nothing discriminates,
//     which is why these are the same two bands LANES measures on.
func reportExchange(out io.Writer, t *gamedata.Tables, fights int, seed int64) {
	fmt.Fprintf(out, "EXCHANGE — what one point in each stat is worth, measured\n")
	fmt.Fprintf(out, "the balanced build nudged a few points either way in one stat, against the\n")
	fmt.Fprintf(out, "same two bands LANES uses; per point, so the columns compare; %d fights\n",
		fights)
	fmt.Fprintf(out, "a cell, and a central difference because these curves saturate\n\n")

	// K is the width of the nudge, and it is now taken in both directions.
	//
	// A one-sided difference was the first draft's mistake and it was not a
	// small one. These curves saturate: a Fighter at the top of the game wins
	// 83% of the stretch fights, so *adding* six strike buys about 1.0 a point
	// while *removing* six costs between 1.5 and 3.7 a point. Pricing a shield
	// swap means removing one stat and adding another, and a rate measured only
	// upward underprices the removal by a factor of up to three — which is the
	// whole of why this table said the silvered shield should beat the spiked
	// one while LANES, measuring the actual items, said the opposite by eleven
	// points. Two instruments in one report disagreeing, and this was the one
	// that was wrong.
	//
	// A central difference costs one more batch a cell and reports the slope
	// through the operating point rather than the slope away from it. It does
	// not make the curve linear — a swap of eleven points is still outside what
	// any derivative can price, and LANES remains the instrument for whole
	// items — but it stops the sign of the error depending on which way the
	// content happens to move.
	const K = 6

	// The stats a content table can actually spend in, and how to spend them.
	// Each nudge goes into the slot that stat already lives in, so nothing here
	// invents a source the game does not have.
	nudges := []struct {
		name string
		add  func(c *model.Character, k int)
	}{
		{"strike", func(c *model.Character, k int) { c.Weapon.Strike += k }},
		{"defense", func(c *model.Character, k int) { c.Armor.Defense += k }},
		// Through Nudged, not through the pointer. Writing to c.Shield.Extra
		// directly reaches the row in the content table that every other
		// character of this tier is also wearing, and the first draft of this
		// section did exactly that: it buffed the whole tier a little more on
		// every one of four thousand iterations, so both sides of the
		// comparison ended up saturated and ward priced out at a flat nought.
		{"ward", func(c *model.Character, k int) {
			c.Shield = c.Shield.Nudged(model.Bonus{Ward: k})
		}},
		{"strength", func(c *model.Character, k int) { c.Strength += k }},
		{"dexterity", func(c *model.Character, k int) { c.Dexterity += k }},
		{"speed", func(c *model.Character, k int) { c.Speed += k }},
		// MaxPsyche without refilling Psyche, which prices the *power* a pool
		// grants rather than the casts it affords — SpellPower reads MaxPsy and
		// the pool itself is spent and refilled at rests. That is the right
		// question for a charm, since a charm raises the ceiling and a bed
		// fills it, but it was true by accident until it was written down.
		{"psyche", func(c *model.Character, k int) { c.MaxPsyche += k }},
	}

	// run fights the balanced build with one nudge applied, in one band —
	// paired against every other variant of the same cell.
	//
	// Fight f draws its character, its monster and its dice from a generator
	// forked on the cell and the fight number, never from the section's own
	// stream, so the baseline and all seven nudges meet the *same* subject in
	// the same encounter with the same opening rolls. Only the nudge differs.
	// That is common random numbers, and it is the difference between an
	// instrument that can resolve a point of ward and one that cannot: the
	// paired difference drops every source of variance the two runs share,
	// which here is nearly all of it. The streams do diverge once a decision
	// goes differently — which is the effect being measured, so it is the
	// variance that should survive.
	//
	// And it runs both directions of the nudge together, keeping the *paired*
	// outcomes, because that is what makes an error bar available at all.
	//
	// Pairing takes this section's noise floor away with one hand: LANES reads
	// its floor off rows where three different-looking builds are secretly the
	// same build, and here a nudge that changes nothing returns a bit-identical
	// stream and a floor of exactly nought, which would license believing
	// anything. The first replacement was two independent halves of each
	// estimate, printing the widest disagreement across the whole table. A
	// reviewer was right that this is the wrong statistic: one draw per cell,
	// below its own sigma two times in three, and then a maximum taken over
	// sixty-odd cells, which is an order statistic of the noisiest one rather
	// than an error bar on any of them.
	//
	// With paired binary outcomes there is an exact answer and it costs two
	// counters. For each fight, the two directions either agree or they do not;
	// the fights where they agree carry no information about the difference at
	// all, and the standard error of the difference is the square root of the
	// discordant count over the sample. That is McNemar's, it is per cell, and
	// it is what the table prints beside every rate now.
	type cell struct {
		up, down float64 // rate at +K/2 and -K/2, in points
		se       float64 // standard error of the difference between them
	}
	run := func(class model.Class, level, delta, k int, add func(*model.Character, int)) (won, died cell) {
		enc := core.Max(1, level+delta)
		biome := biomeForLevel(enc)
		label := fmt.Sprintf("exchange/%d/%s/%d/%d", seed, class, level, delta)
		// one fight, one direction.
		play := func(f, k int) (won, died bool, ok bool) {
			rg := core.NewRNG(seed).Fork(label, int64(f))
			c := rules.BuildCharacter(rg, class, level)
			equip(t, c)
			if add != nil && k != 0 {
				add(c, k)
			}
			mons := t.PickMonsters(rg, biome, enc, 1)
			if len(mons) == 0 {
				return false, false, false
			}
			fresh := *c
			r := rules.SimulateFight(rg, &fresh, []*model.MonsterDef{mons[0].Def},
				enc, 60, t.SpellsFor(c))
			return r.Won, r.Died(), true
		}
		var winUp, winDn, dieUp, dieDn, n int
		var wonDisc, diedDisc int
		for f := 0; f < fights; f++ {
			wu, du, ok := play(f, k)
			if !ok {
				continue
			}
			wd, dd, _ := play(f, -k)
			n++
			if wu {
				winUp++
			}
			if wd {
				winDn++
			}
			if du {
				dieUp++
			}
			if dd {
				dieDn++
			}
			if wu != wd {
				wonDisc++
			}
			if du != dd {
				diedDisc++
			}
		}
		if n == 0 {
			return
		}
		pct := func(v int) float64 { return float64(v) * 100 / float64(n) }
		se := func(disc int) float64 { return math.Sqrt(float64(disc)) * 100 / float64(n) }
		return cell{pct(winUp), pct(winDn), se(wonDisc)},
			cell{pct(dieUp), pct(dieDn), se(diedDisc)}
	}

	fmt.Fprintf(out, "%-6s %-8s %-10s %17s %17s\n",
		"level", "class", "stat", "won +3 /pt", "died +5 /pt")
	fmt.Fprintln(out, strings.Repeat("-", 64))

	// Every cell of the table, computed before any of it is printed.
	//
	// This is the section the whole report waits for — fifty-five seconds of a
	// sixty-four second run, because it is measuring a derivative and pays two
	// batches a cell to do it. The cells do not depend on each other and every
	// fight inside them draws from a stream forked on its own index, so they
	// can be computed in any order by anybody, and then printed in this one.
	type cellKey struct {
		level int
		class model.Class
		nudge int
	}
	var keys []cellKey
	for _, level := range []int{5, 9, 13} {
		for _, class := range model.AllClasses {
			for ni := range nudges {
				keys = append(keys, cellKey{level, class, ni})
			}
		}
	}
	type outRow struct{ won, wonSE, died, diedSE float64 }
	got := make([]outRow, len(keys))
	parallelFor(len(keys), func(i int) {
		k := keys[i]
		add := nudges[k.nudge].add
		// No baseline batch: a central difference is measured between the two
		// nudged runs, and the unnudged build is not one of its terms.
		w, _ := run(k.class, k.level, laneStretch, K/2, add)
		_, d := run(k.class, k.level, laneOver, K/2, add)
		// Central: the whole span from -K/2 to +K/2 over K, which is the slope
		// *through* the operating point rather than away from it. Dying less is
		// worth more, so that column's sign is flipped to keep every number in
		// the table "good is bigger".
		got[i] = outRow{(w.up - w.down) / K, w.se / K, (d.down - d.up) / K, d.se / K}
	})

	// told counts the rows whose sign the sample can actually support, which is
	// the only honest summary of a table this size.
	told, rows := 0, 0
	for i, k := range keys {
		r := got[i]
		for _, v := range []struct{ est, se float64 }{{r.won, r.wonSE}, {r.died, r.diedSE}} {
			rows++
			if v.est > 2*v.se || v.est < -2*v.se {
				told++
			}
		}
		fmt.Fprintf(out, "%-6d %-8s %-10s %10.2f ±%.2f %10.2f ±%.2f\n",
			k.level, k.class, nudges[k.nudge].name, r.won, r.wonSE, r.died, r.diedSE)
		// A blank line between classes, as the nested loops used to give.
		if i+1 == len(keys) || keys[i+1].class != k.class || keys[i+1].level != k.level {
			fmt.Fprintln(out)
		}
	}

	fmt.Fprintf(out, "the error bar beside each figure is McNemar's, which the pairing makes\n")
	fmt.Fprintf(out, "available: the fights where both directions agreed say nothing about the\n")
	fmt.Fprintf(out, "difference between them, so the standard error is the root of the\n")
	fmt.Fprintf(out, "discordant count over the sample. %d of %d figures above clear twice\n",
		told, rows)
	fmt.Fprintf(out, "their own — the rest are zeroes with decimal points in them, and now\n")
	fmt.Fprintf(out, "say so individually rather than against one global floor.\n\n")
	fmt.Fprint(out, `
There are two kinds of exact 0.00 above and they mean opposite things.

A Mage's strike and strength are the pairing proving itself: a Mage swings a
Focus, so those fields are read by nothing it does, the two runs never diverge
by a single die, and the answer is not "small" but *identical*. A stat that
cannot reach anything should say so in that voice, and one that does not
would be a plumbing bug this catches.

A Thief's speed at level thirteen is the other kind, and the first draft of
this paragraph filed it under the first. Speed is read — by initiative, by
the flee roll and by the dodge — but at the top of the game all three have
saturated: DodgeChance caps ten over, FleeChance caps at 9.4, initiative only
needs the difference to be non-negative. So six more points flip no branch and
the streams stay identical for a reason that has nothing to do with plumbing.
That is a content finding about the Thief's headline stat, sitting in the
report dressed as a proof of correctness, which is worse than not measuring
it.
`)

	fmt.Fprint(out, `
What this is for. Three tables spend in these units and none of them was
priced against a rate: shields trade guard for ward or strike, charms trade
one stat for another, affixes do both. "Fifteen points of ward for three of
guard" is not generous or stingy until the two are in the same currency, and
this is the exchange desk. Read the row for the level the item's band is
worn at, and for the class that can hold it.

`)
}

// reportPlaystyles measures a way of *playing* rather than a way of spending,
// which is the one thing ARCS above cannot do.
//
//	"at some point we might need full testing against each class and each
//	flavour of playstyle" — Jeremy
//
// ARCS has three archetypes and all three are budgets: give up a band here, buy
// one there. Nothing in the report has ever varied what the player *does*. This
// is the first thing that does, and it starts with the retreat, for two
// reasons.
//
// It is the most load-bearing piece of judgement in the simulator. The whole
// DANGER brief is a statement about deaths, and the difference between a death
// and a bad afternoon is almost always whether the simulated player left. Two
// separate bugs in the estimate feeding that decision turned up in one evening
// — one over-reading magical damage by three fifths, one taking a mean where it
// wanted the mean of a clamp — and neither would have been visible in any
// section here, because every section shared the same policy and so moved
// together. A column with the retreat switched off is the bound on how much
// that judgement is worth at all.
//
// And it is a real way people play. "Never run" is a style, not a mistake, and
// the game charges for it in exactly the coin the brief cares about.
//
// What this is not: the other two playstyles the open list names. The player
// who leans on items needs the simulator to carry a pack, which it deliberately
// does not — SUPPLIES prices the counter instead — and the player who fights
// behind two companions needs a party simulator, which does not exist at all.
// Both are named in the closing text rather than faked.
func reportPlaystyles(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "PLAYSTYLES — what a way of playing costs, as opposed to a way of spending\n")
	fmt.Fprintf(out, "on-curve gear, the same fights, one behaviour changed\n\n")

	fmt.Fprintf(out, "%-6s %-8s %-6s %8s %8s %8s   %s\n",
		"level", "class", "band", "won", "fled", "died", "vs the competent player")
	fmt.Fprintln(out, strings.Repeat("-", 74))

	// run plays one band under one policy, paired against the other policy by
	// the same trick EXCHANGE uses: the fight number seeds the stream, so both
	// styles meet the same subject in the same encounter and the difference is
	// the behaviour rather than the luck.
	run := func(class model.Class, level, delta int, pol rules.Policy) (won, fled, died, casts float64) {
		enc := core.Max(1, level+delta)
		biome := biomeForLevel(enc)
		label := fmt.Sprintf("playstyle/%s/%d/%d", class, level, delta)
		var wins, flights, deaths, n, spells int
		for f := 0; f < fights; f++ {
			rg := g.Fork(label, int64(f))
			c := rules.BuildCharacter(rg, class, level)
			equip(t, c)
			mons := t.PickMonsters(rg, biome, enc, 1)
			if len(mons) == 0 {
				continue
			}
			fresh := *c
			r := rules.SimulateFightAs(rg, &fresh, []*model.MonsterDef{mons[0].Def},
				enc, 60, t.SpellsFor(c), pol)
			if r.Won {
				wins++
			}
			if r.Fled {
				flights++
			}
			if r.Died() {
				deaths++
			}
			spells += r.Casts
			n++
		}
		if n > 0 {
			won = float64(wins) * 100 / float64(n)
			fled = float64(flights) * 100 / float64(n)
			died = float64(deaths) * 100 / float64(n)
			casts = float64(spells) / float64(n)
		}
		return
	}

	// The two bands where a retreat is a live option. On level almost nobody
	// wants out, and the sections above say so.
	bands := []struct {
		label string
		delta int
	}{{"+3", laneStretch}, {"+5", laneOver}}

	var worstCost, bestGain float64
	for _, level := range []int{5, 9, 13} {
		for _, class := range model.AllClasses {
			for _, b := range bands {
				baseWon, baseFled, baseDied, _ := run(class, level, b.delta, rules.Policy{})
				w, fl, d, _ := run(class, level, b.delta, rules.Policy{NeverFlee: true})

				// Holding the ground converts flights into something. The
				// question is which, and the answer is the whole section.
				deaths, wins := d-baseDied, w-baseWon
				note := fmt.Sprintf("%+.1f won, %+.1f died", wins, deaths)
				if deaths > worstCost {
					worstCost = deaths
				}
				if wins > bestGain {
					bestGain = wins
				}
				fmt.Fprintf(out, "%-6d %-8s %-6s %7.1f%% %7.1f%% %7.1f%%   %s\n",
					level, class, b.label, w, fl, d, note)
				_ = baseFled
			}
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "The rows are the never-run player; the note is the difference against the\n")
	fmt.Fprintf(out, "competent one the rest of this report measures. Standing your ground buys\n")
	fmt.Fprintf(out, "up to %.1f points of win rate and costs up to %.1f points of death rate.\n\n",
		bestGain, worstCost)

	fmt.Fprint(out, `That number is the price of the retreat, and it is worth having for a
reason beyond the playstyle: every other section in this report shares one
retreat policy, so a bug in it moves every section together and cancels out
of every comparison. Two such bugs turned up in a single evening — the
estimate feeding the decision was over-reading magical damage by three
fifths, and then taking a mean where it wanted the mean of a clamp. Neither
was visible anywhere else. This column is the only place the report can see
that judgement at all rather than through it.

Two playstyles the open list names are still not here, and are named rather
than faked. The player who leans on items needs the simulator to carry a
pack, which it deliberately does not — the "no potions" in every heading is
a decision, and SUPPLIES prices the counter instead. The player who fights
behind two companions needs a party simulator, which does not exist: every
fight in this report is one character, which is also why CROWDS' wider
columns mean "a solo hero after their company was killed".

`)

	reportSwingsOnly(out, run)
	reportRounds(out, g, t, fights/4)
	reportUnreachable(out, t)
}

// reportRounds says what a fight is actually made of.
//
// Every other section in this report answers "did they win". This one answers
// "with what", which is the question a player asking why no playstyle beats
// auto-attacks is really asking — and until the counters existed the report
// could not have answered it either way. A class that wins 90% of its fights by
// swinging and a class that wins 90% by stunning and poisoning produce the same
// row everywhere else in this document.
//
// Percentages of rounds rather than counts per fight, because the fights are
// different lengths by class and by level and the interesting number is the
// share. The rounds that go on neither — a flee, a feint, a guard — are the
// remainder, and are left out rather than padded to a hundred: a Thief's
// missing five per cent is its false retreat, which is a real thing it does and
// not a rounding error. A row can also run *over* a hundred, and that is the
// Fighter's second swing putting two actions in one round — which is worth
// seeing rather than normalising away, since it is the mechanic that makes the
// class's rounds worth more than anybody else's.
func reportRounds(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "ROUNDS — what the fight was actually made of\n")
	fmt.Fprintf(out, "share of rounds by what they were spent on, against the stretch fights\n")
	fmt.Fprintf(out, "three levels over. Short of 100%% is fleeing, feinting and guarding; over\n")
	fmt.Fprintf(out, "it is the second swing, which puts two actions in one round\n\n")

	kinds := rules.CastKinds()
	fmt.Fprintf(out, "%-6s %-8s %7s", "level", "class", "swing")
	for _, k := range kinds {
		fmt.Fprintf(out, " %7s", short(string(k)))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, strings.Repeat("-", 16+8*(len(kinds)+1)))

	for _, level := range []int{1, 5, 9, 13} {
		for _, class := range model.AllClasses {
			enc := core.Max(1, level+laneStretch)
			biome := biomeForLevel(enc)
			var swings, rounds int
			var by [16]int
			for f := 0; f < fights; f++ {
				rg := g.Fork(fmt.Sprintf("rounds/%s/%d", class, level), int64(f))
				c := rules.BuildCharacter(rg, class, level)
				equip(t, c)
				mons := t.PickMonsters(rg, biome, enc, 1)
				if len(mons) == 0 {
					continue
				}
				fresh := *c
				r := rules.SimulateFight(rg, &fresh, []*model.MonsterDef{mons[0].Def},
					enc, 60, t.SpellsFor(c))
				swings += r.Swings
				rounds += r.Rounds
				for i := range kinds {
					by[i] += r.CastsBy[i]
				}
			}
			if rounds == 0 {
				continue
			}
			pct := func(n int) float64 { return float64(n) * 100 / float64(rounds) }
			fmt.Fprintf(out, "%-6d %-8s %6.1f%%", level, class, pct(swings))
			for i := range kinds {
				if by[i] == 0 {
					fmt.Fprintf(out, " %7s", "-")
					continue
				}
				fmt.Fprintf(out, " %6.1f%%", pct(by[i]))
			}
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out)
	}
}

// short trims a kind's name to the column it has to fit in.
func short(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// reportUnreachable names the techniques no fight in this report ever casts.
//
// It is the other half of SWINGS ONLY and it is the more uncomfortable half. That
// table asks what the list is worth and answers with a win rate; this one asks
// how much of the list was in the measurement at all, and the answer is about
// half of it. bestSpell offers three doors — a heal, a sap, and the strongest
// attack worth its psyche — so a technique that weakens, stuns, poisons, burns,
// blesses or raises the dead is not weighed and found wanting, it is never seen.
//
// Which makes every row above it a partial reading rather than a wrong one, and
// the difference matters when the reader is a player asking why no way of
// playing beats swinging: on the Thief, five of nine techniques are outside the
// question this report is capable of asking.
//
// Printed as a roster rather than a count because the count is the boring half.
// The useful half is which levels a class goes without anything measurable, and
// that only reads off the names.
func reportUnreachable(out io.Writer, t *gamedata.Tables) {
	fmt.Fprintf(out, "UNREACHABLE — techniques no fight in this report ever casts\n")
	fmt.Fprintf(out, "the policy prices every kind in one currency now: hit points over the rest\n")
	fmt.Fprintf(out, "of the fight. Anything still listed here is not measured, here or above.\n\n")

	for _, class := range model.AllClasses {
		// The whole list the class will ever know, which is what a player sees
		// on the level-up screen — not what they know at some sampled level.
		top := rules.BuildCharacter(core.NewRNG(1), class, 20)
		known := t.SpellsFor(top)
		var dark []model.Spell
		for _, s := range known {
			if !rules.Castable(s) {
				dark = append(dark, s)
			}
		}
		fmt.Fprintf(out, "  %-8s %d of %d unmeasured", class, len(dark), len(known))
		if len(dark) == 0 {
			fmt.Fprintf(out, "\n")
			continue
		}
		fmt.Fprintf(out, ":\n")
		sort.Slice(dark, func(i, j int) bool { return dark[i].Level < dark[j].Level })
		for _, s := range dark {
			fmt.Fprintf(out, "    L%-3d %-9s %s\n", s.Level, s.Kind, s.Name)
		}
	}

	fmt.Fprint(out, `
This block used to list twelve. The policy could choose a heal, a sap or an
attack, so a technique that weakened, stunned, poisoned, burned or blessed was
not weighed and found wanting — it was never seen, and every class number in
this report was a floor under a heading that said "techniques used".

What is left is the one that needs a second character to point at. Standing
somebody up is worth a great deal and there is nobody to stand up: every fight
in this report is one character, so a revive has no target that is not the
caster, and a caster who needs reviving has already lost. It becomes measurable
when this report can fight a party, and not before.

`)
}

// reportSwingsOnly is the second playstyle, and it is a control rather than a
// flourish.
//
// Every heading in this report says "techniques used" and until now nothing
// checked that any were. The gate in bestAttack retires a technique the moment
// a free swing prices above it, so a class whose whole list sits under its own
// weapon plays this report identically with the list and without it — and
// every number would have read the same either way, with nothing anywhere
// saying which of the two had been measured. A player reported precisely that
// about the Fighter and there was no column in which to check them.
//
// Two columns answer it and they answer different halves. "won/died" is what
// swinging every round costs, against the competent player who casts; "casts"
// is how many rounds that competent player actually spent on the list. A zero
// there is the stronger finding of the two, because it says the difference is
// not small — it says there was no difference to measure, and the row above it
// was a swings-only run wearing the other heading.
//
// The bands are on-level and three over rather than the retreat's +3/+5, and
// the levels reach down to 1, because "techniques do nothing" is a complaint
// about the early game and the retreat table starts at 5.
func reportSwingsOnly(out io.Writer, run func(model.Class, int, int, rules.Policy) (float64, float64, float64, float64)) {
	fmt.Fprintf(out, "SWINGS ONLY — what the technique list is worth, if anything\n")
	fmt.Fprintf(out, "the same fights with no psyche spent at all: no attack, no heal, no blessing\n\n")
	fmt.Fprintf(out, "%-6s %-8s %-9s %8s %8s   %-24s %s\n",
		"level", "class", "band", "won", "died", "vs the player who casts", "casts/fight")
	fmt.Fprintln(out, strings.Repeat("-", 82))

	bands := []struct {
		label string
		delta int
	}{{"on-level", 0}, {"+3", laneStretch}}

	// The narrowest gap on a row where the competent player did cast, and the
	// levels at which the list was never opened at all. Both are printed under
	// the table because both are findings and neither is visible in any single
	// row.
	idle := map[model.Class][]int{}
	for _, level := range []int{1, 3, 5, 9, 13} {
		for _, class := range model.AllClasses {
			everCast := false
			for _, b := range bands {
				baseWon, _, baseDied, casts := run(class, level, b.delta, rules.Policy{})
				w, _, d, _ := run(class, level, b.delta, rules.Policy{NeverCast: true})
				if casts > 0 {
					everCast = true
				}
				note := fmt.Sprintf("%+.1f won, %+.1f died", w-baseWon, d-baseDied)
				// A run in which nothing was ever cast is marked rather than
				// printed as 0.0, because the two mean different things: 0.0
				// casts is a list that exists and is never worth opening, and
				// it is the whole answer to the row beside it.
				spent := fmt.Sprintf("%.2f", casts)
				if casts == 0 {
					spent = "never"
				}
				fmt.Fprintf(out, "%-6d %-8s %-9s %7.1f%% %7.1f%%   %-24s %s\n",
					level, class, b.label, w, d, note, spent)
			}
			if !everCast {
				idle[class] = append(idle[class], level)
			}
		}
		fmt.Fprintln(out)
	}

	for _, class := range model.AllClasses {
		if levels := idle[class]; len(levels) > 0 {
			fmt.Fprintf(out, "WARNING: %s never casts anything at level %s — at those levels every\n",
				class, joinInts(levels))
			fmt.Fprintf(out, "  other section of this report is measuring a swings-only player under a\n")
			fmt.Fprintf(out, "  heading that says techniques were used.\n")
		}
	}
	fmt.Fprintln(out)
}

// joinInts renders a level list for a warning line.
func joinInts(v []int) string {
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = fmt.Sprint(n)
	}
	return strings.Join(out, ", ")
}

// reportCharms is the charm slot's LANES: the measurement that decides what
// gamedata.CharmValue is allowed to believe.
//
// It exists because the balanced build was picking the last row of the charm
// file, and the reasoning behind that was a claim about the content — every
// charm gives with one hand and takes with the other, so there is no better
// one, so any pick is as good as any other. The counter still refuses to grade
// charms on exactly that basis. The premise is measurable, and it is wrong:
// one charm wins its band on both axes for essentially every class in three
// bands out of four, and the file order landed on the loser in three of four.
//
// Two axes, because a charm can be for either. Win rate on the stretch fights
// is what one is worth in a fight; fights-per-rest is what it is worth across
// an afternoon, and that column exists to answer the obvious objection to the
// first — that a single fight cannot see a charm which refills a pool. It can
// see it. It just does not find much.
func reportCharms(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
	fmt.Fprintf(out, "CHARMS — is the band a choice, and is the game picking well?\n")
	fmt.Fprintf(out, "every charm the balanced build could wear at this level, against the\n")
	fmt.Fprintf(out, "stretch fights three over and against fights-per-rest on the level\n\n")

	// spread is how far apart the best and worst of a band may come out before
	// the band has stopped being a choice and become a right answer. Three points
	// of win rate is comfortably above the run-to-run wobble at this sample size
	// and comfortably below the gaps that actually turned up, which reached 12.5.
	const spread = 3.0

	fmt.Fprintf(out, "%-6s %-30s %8s %9s %9s\n", "level", "charm", "value", "won", "per rest")
	fmt.Fprintln(out, strings.Repeat("-", 68))

	dominated := 0
	for _, level := range []int{5, 7, 9, 11, 13} {
		band := charmBand(t, level)
		if len(band) < 2 {
			continue
		}
		type row struct {
			ch   model.Charm
			won  float64
			rest float64
		}
		rows := make([]row, 0, len(band))
		for _, ch := range band {
			var wins, n, rested int
			for _, class := range model.AllClasses {
				for i := 0; i < fights; i++ {
					c := rules.BuildCharacter(g, class, level)
					t.Equip(c)
					c.Charm = ch
					mons := t.PickMonsters(g, biomeForLevel(level+3), level+3, 1)
					if len(mons) == 0 {
						continue
					}
					fresh := *c
					r := rules.SimulateFight(g, &fresh, []*model.MonsterDef{mons[0].Def},
						level+3, 60, t.SpellsFor(c))
					if r.Won {
						wins++
					}
					n++
				}
				// Fewer runs on the endurance axis: one run is a chain of
				// fights, so it costs what a dozen of the above do.
				for i := 0; i < charmRuns; i++ {
					sim := rules.BuildCharacter(g, class, level)
					t.Equip(sim)
					sim.Charm = ch
					sim.HP, sim.Psyche = sim.MaxHP, sim.MaxPsy()
					spells := t.SpellsFor(sim)
					for survived := 0; survived < 60; survived++ {
						mons := t.PickMonsters(g, biomeForLevel(level), level, 1)
						if len(mons) == 0 {
							break
						}
						r := rules.SimulateFight(g, sim, []*model.MonsterDef{mons[0].Def},
							level, 60, spells)
						if !r.Won || sim.HP <= 0 {
							break
						}
						rested++
					}
				}
			}
			if n == 0 {
				continue
			}
			rows = append(rows, row{ch, float64(wins) * 100 / float64(n),
				float64(rested) / float64(charmRuns*len(model.AllClasses))})
		}
		if len(rows) < 2 {
			continue
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].won > rows[j].won })

		// What the game actually hands out at this level.
		worn := &model.Character{Class: model.ClassFighter, Level: level}
		t.Equip(worn)

		wornWon := rows[0].won
		for _, r := range rows {
			mark := ""
			if r.ch.Name == worn.Charm.Name {
				mark, wornWon = "  <- worn", r.won
			}
			fmt.Fprintf(out, "%-6d %-30s %8.1f %8.1f%% %9.1f%s\n",
				level, r.ch.Name, gamedata.CharmValue(r.ch), r.won, r.rest, mark)
		}

		// Two questions per band: is it a choice, and is the ranking picking the
		// same thing the fights do.
		if gap := rows[0].won - rows[len(rows)-1].won; gap > spread {
			dominated++
			fmt.Fprintf(out, "       band is not a choice: %.1f points between best and worst\n", gap)
		}
		// The second question has to be asked about the *cost* of the
		// disagreement rather than about the disagreement, and the reason is
		// the first question succeeding.
		//
		// This compared argmaxes: the fights' favourite against CharmValue's
		// pick, warning on any difference at all. That was serviceable while
		// the bands had a right answer in them, because then a wrong pick cost
		// real points. Now that they trade — which is the outcome this section
		// exists to produce — the argmax over three items inside one noise
		// interval is a coin, and comparing coins warns every run forever. It
		// warned five times the first run after the bands were fixed, on gaps
		// of 0.6 to 1.5 points against a threshold of 3.0 for calling a gap at
		// all. A check that fires hardest when the content is at its best is
		// not a check, it is a smoke detector in a kitchen.
		if worst := rows[0].won - wornWon; worst > spread {
			fmt.Fprintf(out, "WARNING: the fights want %q and CharmValue picks %q, at a cost\n",
				rows[0].ch.Name, worn.Charm.Name)
			fmt.Fprintf(out, "         of %.1f points.\n", worst)
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, `A charm is meant to be a trade, and the shop counter refuses to grade one
on exactly that basis - see TestTheShelfNeverGradesACharm, which holds that
marking a charm green would be the interface lying about a system built so
that "did I get the good one" is not the only question worth asking. %d of
the bands above disagree with that premise: they have a right answer, by
more than the noise, on both axes at once.

That is a content finding rather than a fault in the picking. The ward
charms used to carry six to fourteen points of their stat where their rivals carried
one to four of theirs, so the bands are not priced in comparable units -
which is the same thing LANES found about the off arm, for the same reason.
This game gets magical at the top and the sidearm tables were written before
that was true. Until they trade, CharmValue exists so that the balanced
build is at least choosing rather than reading the last row of a file.

`, dominated)
}

// charmRuns is how many chains of fights the endurance column averages over.
const charmRuns = 120

// charmBand is what the balanced build could put in the slot at this level:
// the top band it can reach, which is one behind its gear tier.
func charmBand(t *gamedata.Tables, level int) []model.Charm {
	tier := gamedata.GearTierFor(level) - 1
	if tier < 1 {
		tier = 1
	}
	var band []model.Charm
	for _, c := range t.Charms {
		if c.Tier == tier {
			band = append(band, c)
		}
	}
	return band
}

// reportSlotValue is the diagnosis behind the verdict above.
//
// An archetype is a trade: give up a band in one slot, buy a band in another.
// That trade can only pay if a whole sidearm slot is worth about as much as one
// band of a main slot. This table is where to look when it is not — it compares
// what stepping up a band buys against what the entire off-hand or charm slot
// buys at the same tier, and no simulation is needed to read the answer off it.
func reportSlotValue(out io.Writer, t *gamedata.Tables) {
	fmt.Fprintf(out, "WHY — what one band is worth in each slot\n")
	fmt.Fprintf(out, "every archetype is a trade of bands between slots, so these are the\n")
	fmt.Fprintf(out, "exchange rates it trades at, read off a Fighter's lane\n\n")
	fmt.Fprintf(out, "%-6s %14s %14s %14s %14s %14s\n",
		"tier", "weapon step", "armour step", "shield step", "charm step", "barrier step")
	fmt.Fprintln(out, strings.Repeat("-", 81))

	best := func(tier int) (strike, def, shield, barrier int, charm float64) {
		ws, as := t.StockForClass(tier, model.ClassFighter)
		ss, cs := t.SidearmsFor(tier)
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
		// The charm the build actually wears, scored the way the build scores
		// it. This column read `cs[len(cs)-1].Bonus.Defense` — the last row of
		// the file, on the one axis charms mostly do not carry — and came out
		// 0, 0, 1, 2 for the life of the report. It was the same bug as the
		// equipper's, in the instrument that was supposed to catch it.
		if len(cs) > 0 {
			charm = gamedata.CharmValue(gamedata.BestCharm(cs))
		}
		// And the caster's arm, which is measured in a different unit: a pool
		// spent once rather than a reduction on every blow.
		for _, sh := range ss {
			if sh.Absorb > barrier {
				barrier = sh.Absorb
			}
		}
		return strike, def, shield, barrier, charm
	}

	for tier := 2; tier <= 5; tier++ {
		s0, d0, sh0, b0, ch0 := best(tier - 1)
		s1, d1, sh1, b1, ch1 := best(tier)
		fmt.Fprintf(out, "%-6d %13d+ %13d+ %13d+ %13.1f+ %13d+\n",
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

The charm column is in CharmValue's units rather than in points of
anything, because a charm does not carry one stat: the band's best is a
ward charm here, a strength charm there, and asking which is bigger
needs a rate of exchange. It read "charm def step" and measured the
Defense on the last row of charms.json until recently, which came out
0, 0, 1, 2 - the equipper's own bug, in the instrument that was meant
to catch it. Divide by roughly 1.5 to read it against the armour and
shield columns, which are in flat Defense.

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
func reportEndurance(out io.Writer, g *core.RNG, t *gamedata.Tables, runs int) {
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
func reportShapes(out io.Writer, g *core.RNG, t *gamedata.Tables, fights int) {
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

func reportProgression(out io.Writer, g *core.RNG, t *gamedata.Tables, runs int) {
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

func reportEconomy(out io.Writer, t *gamedata.Tables) {
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
func reportSupplies(out io.Writer, t *gamedata.Tables) {
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
func reportSaga(out io.Writer, t *gamedata.Tables) {
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
func reportSky(out io.Writer) {
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
func reportCompany(out io.Writer, g *core.RNG, t *gamedata.Tables) {
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
	reportCutBuys(out, g, t)
}

// reportCutBuys is the other half of the cut, and it exists because the cut
// stopped being a subtraction.
//
// A companion's share used to leave the purse and go nowhere, while they
// re-armed for free on every level-up — so there was nothing to measure and
// the section above said so in as many words: converting to coins would need a
// model of what a haul is worth, and inventing one is how a report starts
// measuring a fiction.
//
// It is not invented any more. What a fight pays is the coin award plus what
// the drops fetch at the rate a merchant actually pays, rolled off the same
// PickMonsters the game rolls, at the group sizes a company draws. And what
// keeping pace costs is what Equip asks for, which is the same definition of
// "on curve" every other section is measured against.
//
// Like SUPPLIES, this lives entirely on the buying side where SimulateFight
// cannot see it. The question it answers is one question: hire somebody at the
// bottom of a gear tier, walk them to the top of it, and can what they skimmed
// on the way pay for the next tier's kit.
func reportCutBuys(out io.Writer, g *core.RNG, t *gamedata.Tables) {
	fmt.Fprintf(out, "WHAT THE CUT BUYS — a companion's savings against the kit the curve expects\n")
	fmt.Fprintf(out, "one 13%% cut of the coin from company-sized fights, off the tables the game rolls\n\n")

	const cut = 13 // the middle of the band Recruit rolls in

	// Per-level takings first: fights to the next level, and what one cut of
	// each of them comes to.
	// The cut is a share of the coin and only of the coin — Skim is applied to
	// the purse a fight drops, and the drops themselves go whole into the
	// hero's pack. So the two channels are measured apart: what the companion
	// banks, and what the player is holding that they could hand over instead.
	banked := make([]float64, maxLevel+1)
	drops := make([]float64, maxLevel+1)
	for level := 1; level < maxLevel; level++ {
		step := rules.XPForLevel(level+1) - rules.XPForLevel(level)
		if level == 1 {
			step = rules.XPForLevel(2)
		}
		var xp, coins, loot float64
		n := 0
		for i := 0; i < 600; i++ {
			// The size a group comes in when one companion is walking behind
			// you, which is the party this section is about. Bigger groups pay
			// more and are worth more experience, so this mostly cancels in the
			// per-level column — which is itself worth knowing, and was not
			// obvious before it was measured.
			mons := t.PickMonsters(g, biomeForLevel(level), level, party.EncounterSize(g, 1, 1))
			if len(mons) == 0 {
				continue
			}
			for _, m := range mons {
				xp += float64(m.Def.XP)
				for name, k := range rules.RollLoot(g, m.Def.Loot) {
					if it, ok := t.Item(name); ok {
						loot += float64(rules.SellPrice(it.Value) * k)
					}
				}
			}
			coins += float64(rules.CoinAward(g, mons))
			n++
		}
		if n == 0 || xp == 0 {
			continue
		}
		fights := float64(step) / (xp / float64(n))
		banked[level] = fights * (coins / float64(n)) * cut / 100
		drops[level] = fights * (loot / float64(n))
	}

	fmt.Fprintf(out, "%-6s %-8s %14s %16s %9s %14s\n",
		"tier", "levels", "one cut banked", "next tier's kit", "covered", "drops kept")
	fmt.Fprintln(out, strings.Repeat("-", 76))

	worst := 999.0
	for tier := 1; tier < 5; tier++ {
		var lo, hi int
		for level := 1; level <= maxLevel; level++ {
			if gamedata.GearTierFor(level) != tier {
				continue
			}
			if lo == 0 {
				lo = level
			}
			hi = level
		}
		if lo == 0 || hi >= maxLevel {
			continue
		}
		var take, kept float64
		for level := lo; level <= hi; level++ {
			take += banked[level]
			kept += drops[level]
		}
		// A Fighter's lane, which is the dearest of the three, so this is the
		// pessimistic column rather than an average nobody plays.
		next := &model.Character{Level: hi + 1, Class: model.ClassFighter}
		t.Equip(next)
		kit := float64(gamedata.GearCost(next))
		covered := take / kit * 100
		if covered < worst {
			worst = covered
		}
		fmt.Fprintf(out, "%-6d %-8s %14.0f %16.0f %8.0f%% %14.0f\n",
			tier, fmt.Sprintf("%d-%d", lo, hi), take, kit, covered, kept)
	}

	fmt.Fprint(out, `
A companion is hired already dressed for the level they are hired at - the
fee up front is what that buys - so the question is never "can they afford
a kit from nothing". It is whether the standing charge keeps pace with the
curve moving under them, and the answer the covered column gives is: not
on its own, at any tier below the last one.

That is the finding, and it is the right shape rather than a shortfall to
tune away. The cut buys the cheap slots - a sidearm band is a quarter of a
main one, so an arm and a charm are two or three levels of skimming, and a
sword is eight. What the cut cannot reach, the pack can: the drops column
is what the same fights put in the hero's hands, it is larger than the cut
at every tier, and handing a companion the coat you have outgrown is
instant where saving for one is slow.

So the two halves are a division of labour rather than a gap. The cut
keeps a companion's sidearms current by itself and chips at the rest; the
main slots are hand-me-downs, which is why the sheet says what they are
saving for - it is a shopping list addressed to the person holding the
pack. A player who never opens it still has a companion who improves,
slowly, out of their own wages.

The covered column climbs the whole way - 8, 24, 38, 64 - and that is the
half of this table that has to hold. A companion whose share bought a
smaller fraction of the shelf every tier would be a mercenary getting
relatively poorer the longer they worked for you, which is a strange
advertisement for the trade. Rising means the arrangement is worth more
the longer it lasts, and by the top band the cut alone is most of a kit.

`)
	// The line worth shouting about is the top of the table rather than the
	// bottom: a lean early band is the design, and a cut that covered
	// everything would make the pack pointless.
	if worst > 60 {
		fmt.Fprintf(out, "NOTE: the leanest band covers %.0f%%, so the cut is now funding the\n"+
			"main slots on its own and hand-me-downs have stopped mattering.\n\n", worst)
	}
}

// reportMonsterSpread shows how the rosters are distributed by level, which is
// where holes in the content show up: a band with nothing to fight in it makes
// PickMonsters fall back to something wildly off-level.
func reportMonsterSpread(out io.Writer, t *gamedata.Tables) {
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
