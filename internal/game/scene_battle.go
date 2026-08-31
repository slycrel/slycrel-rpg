package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/saga"
	"github.com/slycrel/slycrel-rpg/internal/save"
	"github.com/slycrel/slycrel-rpg/internal/thread"
	"github.com/slycrel/slycrel-rpg/internal/ui"
)

// battleMode is which part of the battle interface has the cursor.
type battleMode int

const (
	modeRoot     battleMode = iota // Attack / Technique / Item / Flee
	modeTarget                     // choosing which monster
	modeAllyPick                   // choosing which of your own people
	modeSpell                      // choosing a technique
	modeItem                       // choosing an item
	modeBusy                       // playing back queued actions
	modeDone                       // battle over, waiting for a key
)

// stepTicks is how long each queued action's message stays up before the next
// one plays. Long enough to read, short enough that a three-monster round does
// not become a coffee break.
//
// It was 30 and fixed, and 30 turned out to be the fast end of the range
// rather than the middle of it: the attacks read as a little bit fast. The old
// value is still there, as the setting called fast; the default is the rung
// below it. See paceTicks.
var stepTicks = paceSteady

// floater is a damage number rising off a combatant.
type floater struct {
	x, y float64
	text string
	life int
	col  color.RGBA
}

// battleScene is the turn-based encounter screen.
type battleScene struct {
	under Scene
	mons  []*model.Monster
	where string

	// party is the company as it stood when the fight started, hero first.
	// It is snapshotted rather than read from the game each round so that
	// indices into the panel and the per-member state below stay valid; you
	// cannot hire anybody halfway through a fight anyway.
	party []*model.Character

	log  *ui.Log
	menu ui.Menu
	mode battleMode
	// back is the mode to return to when targeting is cancelled.
	back battleMode

	// blurb is the technique popover: what the highlighted move actually does,
	// toggled with left or right. The command panel is fifty-eight pixels and
	// holds three rows, so there has never been anywhere to put this — and a
	// technique that charges psyche and will not say what for is the failure
	// this project keeps finding. It opens over the transcript, which is the
	// one panel nobody is reading while they choose.
	blurb bool

	target      int
	pendingCast model.Spell
	pendingItem int
	// allyPick is the cursor over your own party, and pendingFall records
	// whether it is picking somebody to help or somebody to stand back up —
	// which are opposite lists, and choosing the wrong one is how a revive
	// ends up offered to the only person who does not need it.
	allyPick    int
	pendingFall bool

	queue []func(*Game)
	timer int

	// What the fight has cost each member so far, which is what it hands part
	// of back at the end. Counted rather than read off the health bar: a
	// potion drunk mid-fight would otherwise erase the damage that earned the
	// refund, and the simulator counts it the same way.
	spentHP     map[*model.Character]int
	spentPsyche map[*model.Character]int

	cam       render.Camera
	floaters  []floater
	hurt      []int // per-monster hit flash timer
	partyHurt map[*model.Character]int

	// guarding is who braced this round. Per member rather than a single flag,
	// because a companion deciding to cover up must not also halve what the
	// hero takes. It is not an Effect: bracing is a stance held for one round,
	// not a condition with a duration.
	guarding map[*model.Character]bool

	// feintFailed is set when a false retreat is not bought, and makes the
	// answer land harder for the rest of the round. Cleared when the round is.
	feintFailed bool

	// fade counts the ticks since the hero went down, which is the only thing
	// the scene does afterwards. Zero means nobody has died.
	fade int

	// bursts are the combat effects currently playing. Screen-positioned and
	// self-retiring; nothing else in the scene reads them.
	bursts []burst
	// swings counts weapon blows, only so the slash art cycles.
	swings int

	// result is 0 running, 1 victory, 2 defeat, 3 fled, 4 the hero went down
	// but the company did not.
	result   int
	round    int
	introRun bool
}

func newBattleScene(g *Game, enc gamedata.Encounter, where string) *battleScene {
	mons := enc.Monsters
	b := &battleScene{
		under:       g.Top(),
		mons:        mons,
		where:       where,
		party:       g.Party(),
		log:         ui.NewLog(60),
		hurt:        make([]int, len(mons)),
		partyHurt:   map[*model.Character]int{},
		spentHP:     map[*model.Character]int{},
		spentPsyche: map[*model.Character]int{},
		guarding:    map[*model.Character]bool{},
	}
	// Nobody carries anything in from the last fight — and then whatever is on
	// the off arm goes up, before anybody has swung at anything.
	for _, c := range b.party {
		c.Active = nil
		if n := rules.Raise(c); n > 0 {
			b.log.AddColor(render.ColMagic, "%s's %s settles into place. %d.",
				c.Name, c.Shield.Name, n)
		}
	}
	b.cam = render.Camera{}
	b.setRootMenu(g)

	// What kind of fight this is, said out loud. A composition cannot be read
	// off three portraits in the second before a player chooses, and the shape
	// is the whole of what the encounter work added — so it goes in the one
	// line the transcript already opens with.
	names := mons[0].Name
	if line := enc.Line(); line != "" {
		names = line
	} else if len(mons) > 1 {
		names = fmt.Sprintf("%d of them", len(mons))
	}
	g.Sound.Play("fight/start")
	b.log.AddColor(render.ColGold, "Out of the %s: %s.", where, names)
	if t := g.Write.Taunt(g.RNG, mons[0]); t != "" {
		b.log.Add("%s", t)
	}
	return b
}

// partyFrac is the company's health as one number, for the between-rounds
// narration. It pools hit points across everyone including the fallen, so "you
// are both nearly finished" is a read on the party rather than on the hero
// alone — a hero at full health flanked by two people on the floor is not
// winning, and the line should not say so.
func (b *battleScene) partyFrac() float64 {
	var hp, max int
	for _, c := range b.party {
		hp += core.Max(0, c.HP)
		max += c.MaxHP
	}
	if max <= 0 {
		return 0
	}
	return core.ClampF(float64(hp)/float64(max), 0, 1)
}

// tickEffects runs the lingering conditions on both sides: damage first, then
// the clocks, then a line for anything that has just worn off.
//
// It runs at the end of the round rather than the start so that something
// applied this round does not immediately tick, which would make a one-round
// effect land twice and a three-round one feel like four.
func (b *battleScene) tickEffects(g *Game) {
	for i, m := range b.mons {
		if m.Dead {
			m.Active = nil
			continue
		}
		for _, t := range rules.TickDamage(g.RNG, m.Active) {
			// The line before the blow: damageMonster writes the death notice
			// when it finishes something off, and a transcript that announces
			// the corpse before the poison that made it reads backwards.
			b.log.AddColor(render.ColGold, "%s %s for %d.", t.Kind.Verb(), m.Short(), t.Damage)
			b.damageMonster(g, i, t.Damage)
			if m.Dead {
				break // the rest of what is wrong with it no longer matters
			}
		}
		m.Active, _ = rules.Advance(m.Active)
	}

	for _, c := range b.party {
		if !c.Alive() {
			c.Active = nil
			continue
		}
		for _, t := range rules.TickDamage(g.RNG, c.Active) {
			d, soaked := t.Damage, 0
			c.Active, d, soaked = rules.Soak(c.Active, d)
			if soaked > 0 {
				// Said out loud even when it stops the whole tick. A barrier
				// quietly eaten by poison is a pip the player watches vanish
				// with nothing to explain it.
				b.log.AddColor(render.ColMagic, "%s's %s takes the %s.",
					c.Name, c.Shield.Name, t.Kind)
			}
			if d == 0 {
				continue
			}
			c.HP = core.Max(0, c.HP-d)
			b.spentHP[c] += d
			b.partyHurt[c] = 10
			fx, fy := b.memberFloat(c)
			b.addFloater(fx, fy, fmt.Sprintf("-%d", d), render.ColBlood)
			b.log.AddColor(render.ColBlood, "%s %s for %d.", t.Kind.Verb(), c.Name, d)
			// Somebody carrying two conditions has both roll every round, so
			// without stopping here the second one would tick a body that is
			// already on the floor and announce it going down a second time.
			if !c.Alive() {
				if c != g.Player {
					g.Sound.Play("fight/die")
					b.log.AddColor(render.ColBlood, "%s", g.Write.AllyDown(g.RNG, c.Name))
				}
				break
			}
		}
		var expired []model.EffectKind
		c.Active, expired = rules.Advance(c.Active)
		for _, k := range expired {
			if k.Harmful() {
				b.log.AddColor(render.ColInkDim, "%s stops %s.", c.Name, wearingOff(k))
			}
		}
	}
}

// sufferingList names what somebody has just been cured of, in plain English
// rather than as a comma-separated dump of internal kind names.
func sufferingList(kinds []model.EffectKind) string {
	words := make([]string, 0, len(kinds))
	for _, k := range kinds {
		words = append(words, suffering(k))
	}
	switch len(words) {
	case 0:
		return "afflicted"
	case 1:
		return words[0]
	case 2:
		return words[0] + " or " + words[1]
	default:
		return strings.Join(words[:len(words)-1], ", ") + " or " + words[len(words)-1]
	}
}

// suffering is the adjective for a condition.
func suffering(k model.EffectKind) string {
	switch k {
	case model.EffectPoison:
		return "poisoned"
	case model.EffectBurn:
		return "on fire"
	case model.EffectStun:
		return "confused"
	case model.EffectWeaken:
		return "weakened"
	}
	return "afflicted"
}

// wearingOff phrases a condition ending, from the sufferer's point of view.
func wearingOff(k model.EffectKind) string {
	switch k {
	case model.EffectPoison:
		return "being poisoned, which they mention"
	case model.EffectBurn:
		return "burning, eventually"
	case model.EffectStun:
		return "staring at the middle distance"
	}
	return "suffering from it"
}

// anyoneDown reports whether the company has somebody on the floor, which is
// what makes a revive worth offering.
func (b *battleScene) anyoneDown() bool {
	for _, c := range b.party {
		if !c.Alive() {
			return true
		}
	}
	return false
}

// livingParty returns the members of this fight's company still standing.
func (b *battleScene) livingParty() []*model.Character {
	var out []*model.Character
	for _, c := range b.party {
		if c.Alive() {
			out = append(out, c)
		}
	}
	return out
}

func (b *battleScene) setRootMenu(g *Game) {
	// Backing out of the technique list closes what was open over it.
	b.blurb = false
	// The root menu is text-only; icons return when a list of things appears.
	b.menu.Icons = nil
	spells := g.Data.SpellsFor(g.Player)
	// The attack row names the weapon rather than the word "Attack".
	//
	// It said "Attack" with the weapon in the detail column, which is the wrong
	// way round twice over: the player knows the first row is the attack, and
	// what they actually want off it is which of the two things in their pack
	// they are currently holding. It also could not survive the detail column
	// being measured first — a thirty-four-character weapon name would have
	// squeezed the label out entirely.
	//
	// A rod keeps a word in the detail, because a row reading "Ashen Rod of
	// Mild Threat" and nothing else does not say that swinging it is a spell.
	attack := ui.MenuItem{Label: g.Player.Weapon.Titled(), Data: cmdAttack}
	if g.Player.Casting() {
		attack.Detail = "bolt"
	}
	items := []ui.MenuItem{
		attack,
		{Label: "Technique", Detail: fmt.Sprintf("%d SP", g.Player.Psyche),
			Disabled: len(spells) == 0, Data: cmdTechnique},
		{Label: "Item", Detail: fmt.Sprintf("%d", len(g.Player.Bag)),
			Disabled: len(g.Player.Bag) == 0, Data: cmdItem},
		{Label: "Defend", Detail: "brace", Data: cmdDefend},
		{Label: "Flee", Detail: "sensible", Data: cmdFlee},
	}
	// The thief's way out of a retreat paying nothing. It sits under Flee
	// because that is what it pretends to be, and it only appears for somebody
	// who can actually do it — a greyed row would be advertising a class
	// ability to two classes that will never have it.
	if rules.CanFeint(g.Player) {
		items = append(items, ui.MenuItem{
			Label: "False retreat", Detail: fmt.Sprintf("%.0f%%",
				rules.FeintChance(g.Player, b.fastestFoe())*100),
			Data: cmdFeint,
		})
	}
	b.menu.SetItems(items)
	b.menu.Index = 0
	b.mode = modeRoot
}

// living returns the indices of monsters still standing.
func (b *battleScene) living() []int {
	var out []int
	for i, m := range b.mons {
		if !m.Dead {
			out = append(out, i)
		}
	}
	return out
}

func (b *battleScene) firstLiving() int {
	if l := b.living(); len(l) > 0 {
		return l[0]
	}
	return -1
}

func (b *battleScene) Update(g *Game) error {
	b.cam.Update()
	b.retireBursts(g)
	for i := range b.hurt {
		if b.hurt[i] > 0 {
			b.hurt[i]--
		}
	}
	for c, n := range b.partyHurt {
		if n > 0 {
			b.partyHurt[c] = n - 1
		}
	}
	for i := 0; i < len(b.floaters); {
		b.floaters[i].life--
		b.floaters[i].y -= 0.35
		if b.floaters[i].life <= 0 {
			b.floaters = append(b.floaters[:i], b.floaters[i+1:]...)
			continue
		}
		i++
	}

	// Going dark. Every key is ignored while it happens, including the one that
	// would have dismissed the fight — a player mashing Z through the last
	// round should not skip the only moment the game takes for itself.
	if b.fade > 0 {
		b.fade++
		if b.fade > deathFade {
			g.Pop()
			b.onPopped(g)
		}
		return nil
	}

	switch b.mode {
	case modeBusy:
		b.updateBusy(g)
	case modeDone:
		// The fight is over and the panel is a report. Any key gets on with it.
		if g.Dismiss() {
			g.Pop()
			b.onPopped(g)
		}
	default:
		b.updateMenus(g)
	}
	return nil
}

// deathFade is how long the screen takes to go out, in ticks.
//
// A hundred and five is a shade under two seconds: long enough to read as the
// world closing rather than as a transition, short enough that somebody who has
// died four times running is not sitting through a ceremony.
const deathFade = 105

func (b *battleScene) updateBusy(g *Game) {
	if b.timer > 0 {
		b.timer--
		return
	}
	if len(b.queue) > 0 {
		step := b.queue[0]
		b.queue = b.queue[1:]
		step(g)
		b.timer = stepTicks
		return
	}
	// Conditions bite at the end of the round, before the ending is checked, so
	// a poison can be what finishes a fight rather than only ever softening
	// one. Then the clocks run down.
	b.tickEffects(g)
	if b.checkEnd(g) {
		return
	}
	clear(b.guarding)
	b.feintFailed = false
	b.round++
	if b.round%3 == 0 {
		if i := b.firstLiving(); i >= 0 {
			d := rules.GetDisposition(b.partyFrac(), b.mons[i].HPFrac())
			if line := g.Write.DispositionLine(g.RNG, d, b.mons[i].Name); line != "" {
				b.log.AddColor(render.ColInkDim, "%s", line)
			}
		}
	}
	b.setRootMenu(g)
}

func (b *battleScene) updateMenus(g *Game) {
	if d, ok := MenuDir(); ok {
		switch b.mode {
		case modeTarget:
			l := b.living()
			if len(l) > 0 {
				switch d {
				case core.DirLeft:
					b.target = prevIn(l, b.target)
					g.Sound.Play("ui/move")
				case core.DirRight:
					b.target = nextIn(l, b.target)
					g.Sound.Play("ui/move")
				case core.DirUp, core.DirDown:
					// A row at a time, now that the field is a grid rather than
					// a line. Up and down did nothing at all when the monsters
					// were a single row across the top, which was honest then
					// and reads as a broken key in front of a two-by-two.
					step := monCols(len(b.mons))
					if d == core.DirUp {
						step = -step
					}
					b.target = b.nearestLiving(b.target + step)
					g.Sound.Play("ui/move")
				}
			}
		case modeAllyPick:
			// The party is a vertical list, so this cursor runs up and down
			// where the monster cursor runs left and right.
			l := b.allyChoices()
			if len(l) > 0 {
				switch d {
				case core.DirUp:
					b.allyPick = prevIn(l, b.allyPick)
					g.Sound.Play("ui/move")
				case core.DirDown:
					b.allyPick = nextIn(l, b.allyPick)
					g.Sound.Play("ui/move")
				}
			}
		default:
			switch d {
			case core.DirDown:
				b.menu.Move(1)
				g.Sound.Play("ui/move")
			case core.DirUp:
				b.menu.Move(-1)
				g.Sound.Play("ui/move")
			case core.DirLeft, core.DirRight:
				// Only the technique list has anything to explain. Left and
				// right do nothing in the other menus and this is the one
				// place they were free to mean something.
				if b.mode == modeSpell {
					b.blurb = !b.blurb
					g.Sound.Play("ui/page")
				}
			}
		}
	}

	if g.Back() {
		switch b.mode {
		case modeSpell, modeItem:
			b.setRootMenu(g)
		case modeTarget, modeAllyPick:
			if b.back == modeRoot {
				b.setRootMenu(g)
			} else {
				b.mode = b.back
			}
		}
		return
	}
	if !g.Accept() {
		return
	}

	switch b.mode {
	case modeRoot:
		b.chooseRoot(g)
	case modeSpell:
		b.chooseSpell(g)
	case modeItem:
		b.chooseItem(g)
	case modeTarget:
		b.confirmTarget(g)
	case modeAllyPick:
		b.confirmAlly(g)
	}
}

// command tags the top-level rows, so what a row *does* travels with the row
// rather than with its position.
//
// It dispatched on the index, which is the bug the pause menu has already had
// once: the labels moved and the switch did not, and Save silently became Load.
// This menu was one row away from the same failure — the attack row's label is
// the weapon's name now, so it is no longer even readable as a constant, and
// False retreat already appears and disappears under everything else.
type command int

const (
	cmdAttack command = iota
	cmdTechnique
	cmdItem
	cmdDefend
	cmdFlee
	cmdFeint
)

// selectCommand puts the cursor on a top-level row by what it does. The tour
// drives this menu and used to do it by index, which is the same fragility the
// dispatch below just stopped relying on.
func (b *battleScene) selectCommand(cmd command) bool {
	for i, it := range b.menu.Items {
		if c, ok := it.Data.(command); ok && c == cmd {
			b.menu.Index = i
			return true
		}
	}
	return false
}

func (b *battleScene) chooseRoot(g *Game) {
	it, ok := b.menu.Selected()
	if !ok {
		return
	}
	cmd, ok := it.Data.(command)
	if !ok {
		return
	}
	switch cmd {
	case cmdAttack:
		b.beginTargeting(modeRoot)
	case cmdTechnique:
		spells := g.Data.SpellsFor(g.Player)
		fallen := b.anyoneDown()
		items := make([]ui.MenuItem, 0, len(spells)+1)
		for _, s := range spells {
			// Greying out a revive while everybody is upright is the same
			// courtesy as greying out a technique you cannot pay for: the menu
			// should not offer a move that does nothing.
			// Quoted at what this caster pays rather than what the table
			// says, because a Fighter's surcharge that only shows up as a
			// failed cast is a rule nobody can plan around.
			cost := rules.PsycheCost(g.Player, s)
			off := cost > g.Player.Psyche ||
				(s.Kind == model.SpellRevive && !fallen)
			detail, tint := fmt.Sprintf("%d SP", cost), color.Color(nil)
			// A technique with two sides quotes both, in the one column whose
			// colour survives the cursor landing on it. There is room for about
			// nine characters after a long name, which is why this is a signed
			// number rather than a sentence — and the transcript says it in
			// words the first time anybody casts one.
			switch s.Kind {
			case model.SpellSap:
				detail, tint = fmt.Sprintf("%d SP  +%d", cost, s.Power), render.ColBetter
			case model.SpellPact:
				detail, tint = fmt.Sprintf("%d SP  -%d", cost, rules.PactCost(s)), render.ColWorse
			}
			items = append(items, ui.MenuItem{
				Label: s.Name, Detail: detail, DetailTint: tint, Icon: s.Icon,
				Disabled: off, Data: s,
			})
		}
		if len(items) == 0 {
			return
		}
		b.menu.Icons = g.Assets
		// From the top, not from wherever the root menu's cursor happened to
		// be. One Menu serves both, and SetItems preserves the index — so
		// opening Techniques from row one landed on the *second* technique, and
		// Item from row two on the second item, for no reason anybody chose.
		b.menu.Index = 0
		b.menu.SetItems(items)
		// Icon rows are taller, so fewer fit the command panel.
		b.menu.Visible = 3
		// Open on whatever was cast last, if it is still castable. A fight is
		// mostly the same two or three moves in some order, and scrolling past
		// the same four techniques every round to reach the one being used is
		// the sort of friction that is invisible until somebody counts the
		// keypresses. Visible is set first: Select scrolls against it.
		for i, it := range items {
			if s, ok := it.Data.(model.Spell); ok && s.ID == g.LastSpell {
				b.menu.Select(i)
				break
			}
		}
		b.mode = modeSpell
	case cmdItem:
		fallen := b.anyoneDown()
		b.menu.Index = 0
		items := make([]ui.MenuItem, 0, len(g.Player.Bag))
		for i, it := range g.Player.Bag {
			off := it.Kind == model.ItemTrinket ||
				(it.Kind.WantsTheFallen() && !fallen)
			items = append(items, ui.MenuItem{
				Label: it.Name, Detail: fmt.Sprintf("x%d", it.Count), Icon: it.Icon,
				Disabled: off, Data: i,
			})
		}
		if len(items) == 0 {
			return
		}
		b.menu.Icons = g.Assets
		b.menu.SetItems(items)
		b.menu.Visible = 3
		b.mode = modeItem
	case cmdDefend:
		b.runRound(g, func(g *Game) {
			b.guarding[g.Player] = true
			b.log.Add("%s sets their feet and waits for it.", g.Player.Name)
		})
	case cmdFlee:
		b.runRound(g, func(g *Game) { b.attemptFlee(g) })
	case cmdFeint:
		b.runRound(g, func(g *Game) { b.attemptFeint(g) })
	}
}

// fastestFoe reports the speed of the quickest thing still standing, which is
// what both running and pretending to run are judged against.
func (b *battleScene) fastestFoe() int {
	fastest := 0
	for _, i := range b.living() {
		if b.mons[i].Speed > fastest {
			fastest = b.mons[i].Speed
		}
	}
	return fastest
}

// attemptFeint sells the retreat, or fails to.
//
// The reward is a blow against something that has turned its back; the price of
// failing is the round plus a harder answer, which is what keeps this a gamble
// rather than simply the thief's best attack.
func (b *battleScene) attemptFeint(g *Game) {
	live := b.living()
	if len(live) == 0 {
		return
	}
	idx := live[0]
	m := b.mons[idx]

	if !g.RNG.Chance(rules.FeintChance(g.Player, b.fastestFoe())) {
		b.feintFailed = true
		g.Sound.Play("fight/miss")
		b.log.Add("%s turns to run. Nobody buys it.", g.Player.Name)
		return
	}
	g.Sound.Play("fight/crit")
	d := rules.FeintDamage(g.RNG, g.Player, m)
	b.damageMonster(g, idx, d)
	b.log.AddColor(render.ColGold, "%s breaks, %s follows, and %s was never leaving. %d.",
		g.Player.Name, m.Short(), g.Player.Name, d)
}

// chooseSpell routes a technique to whichever cursor it needs.
//
// A technique aimed at your own side asks for a party member; one aimed at the
// monsters asks for a monster; one that hits everybody, or that only ever
// applies to the caster, asks for nothing and goes straight off.
func (b *battleScene) chooseSpell(g *Game) {
	it, ok := b.menu.Selected()
	if !ok || it.Disabled {
		return
	}
	s := it.Data.(model.Spell)
	b.pendingCast = s

	if s.Target == model.TargetOne {
		if s.Kind.Side() == model.SideParty {
			b.beginAllyPick(g, modeSpell, s.Kind == model.SpellRevive)
			return
		}
		b.beginTargeting(modeSpell)
		return
	}
	b.runRound(g, func(g *Game) {
		b.castSpell(g, cast{by: g.Player, spell: s, foe: -1, ally: g.Player})
	})
}

func (b *battleScene) chooseItem(g *Game) {
	it, ok := b.menu.Selected()
	if !ok || it.Disabled {
		return
	}
	idx := it.Data.(int)
	if idx >= len(g.Player.Bag) {
		return
	}
	b.pendingItem = idx

	// Anything applied to a person needs one chosen once there is more than
	// one of you. Handing a companion a potion is most of why the party has a
	// pack at all.
	if kind := g.Player.Bag[idx].Kind; kind.UsedOnSomeone() {
		b.beginAllyPick(g, modeItem, kind.WantsTheFallen())
		return
	}
	b.runRound(g, func(g *Game) { b.useItem(g, idx, g.Player) })
}

// allyChoices lists the party indices the ally cursor may land on: the fallen
// when something is being used to stand somebody up, everyone still standing
// otherwise.
func (b *battleScene) allyChoices() []int {
	var out []int
	for i, c := range b.party {
		if c.Alive() != b.pendingFall {
			out = append(out, i)
		}
	}
	return out
}

// beginAllyPick opens the cursor over your own party, or skips it when there is
// only one person it could possibly mean.
//
// Skipping matters more than it looks: a solo hero would otherwise have to
// confirm "yes, me" every time they drank a potion, and the party feature would
// have made the common case worse.
func (b *battleScene) beginAllyPick(g *Game, from battleMode, fallen bool) {
	b.back = from
	b.pendingFall = fallen

	choices := b.allyChoices()
	switch len(choices) {
	case 0:
		// Nothing to aim it at — a revive with nobody down. Say so rather than
		// silently eating the turn.
		g.Sound.Play("ui/deny")
		b.log.AddColor(render.ColInkDim, "Nobody here needs that yet.")
		b.setRootMenu(g)
		return
	case 1:
		b.allyPick = choices[0]
		b.commitAlly(g)
		return
	}
	if !contains(choices, b.allyPick) {
		b.allyPick = choices[0]
	}
	b.mode = modeAllyPick
}

func (b *battleScene) confirmAlly(g *Game) { b.commitAlly(g) }

// commitAlly turns the chosen party member into a queued action.
func (b *battleScene) commitAlly(g *Game) {
	if b.allyPick < 0 || b.allyPick >= len(b.party) {
		return
	}
	target := b.party[b.allyPick]
	if b.back == modeItem {
		idx := b.pendingItem
		b.runRound(g, func(g *Game) { b.useItem(g, idx, target) })
		return
	}
	s := b.pendingCast
	b.runRound(g, func(g *Game) {
		b.castSpell(g, cast{by: g.Player, spell: s, foe: -1, ally: target})
	})
}

func contains(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (b *battleScene) beginTargeting(from battleMode) {
	b.back = from
	if l := b.living(); len(l) > 0 {
		found := false
		for _, i := range l {
			if i == b.target {
				found = true
				break
			}
		}
		if !found {
			b.target = l[0]
		}
	}
	b.mode = modeTarget
}

func (b *battleScene) confirmTarget(g *Game) {
	tgt := b.target
	if b.back == modeSpell {
		s := b.pendingCast
		b.runRound(g, func(g *Game) {
			b.castSpell(g, cast{by: g.Player, spell: s, foe: tgt})
		})
		return
	}
	b.runRound(g, func(g *Game) { b.playerAttack(g, g.Player, tgt) })
}

// cast is one queued casting: who is doing it, what, and at whom. It exists so
// that adding a second target side did not turn castSpell into a function with
// two nullable index parameters that callers had to remember the rules for.
type cast struct {
	by    *model.Character
	spell model.Spell
	foe   int              // index into b.mons, or -1
	ally  *model.Character // party-side target, or nil
}

// runRound queues the whole round — the player's chosen action, each
// companion's own decision, and every monster's response — then hands the scene
// over to the playback loop.
//
// Initiative is rolled per party member against the fastest monster, so a quick
// thief can act before the pack and a mage in plate can end up going last. For a
// solo hero this is exactly the single roll it always was.
func (b *battleScene) runRound(g *Game, playerAction func(*Game)) {
	b.menu.Visible = 0
	fastest := 0
	for _, i := range b.living() {
		if s := b.mons[i].Speed; s > fastest {
			fastest = s
		}
	}

	var before, after []func(*Game)
	for _, c := range b.livingParty() {
		member := c
		act := playerAction
		if member != g.Player {
			act = func(g *Game) { b.allyTurn(g, member) }
		}
		// Anybody who loses initiative may be dead by the time their step comes
		// up, since the monsters go in between. Without this the hero could be
		// killed during the monster phase and still swing, drink a potion, or
		// flee afterwards — the companions were already guarded inside
		// allyTurn, and the hero was the one who was not.
		queued := func(g *Game) {
			if !member.Alive() || b.result != 0 {
				b.timer = 0
				return
			}
			act(g)
		}
		if rules.Initiative(g.RNG, member.Spd(), fastest) {
			before = append(before, queued)
		} else {
			after = append(after, queued)
		}
	}

	var monSteps []func(*Game)
	for _, i := range b.living() {
		idx := i
		monSteps = append(monSteps, func(g *Game) { b.monsterTurn(g, idx) })
	}

	b.queue = b.queue[:0]
	b.queue = append(b.queue, before...)
	b.queue = append(b.queue, monSteps...)
	b.queue = append(b.queue, after...)
	b.mode = modeBusy
	b.timer = 0
}

// allyTurn plays one companion's move. They are never commanded: the policy in
// rules picks, and the screen narrates it.
func (b *battleScene) allyTurn(g *Game, c *model.Character) {
	move := rules.ChooseAllyMove(g.RNG, c, g.Data.SpellsFor(c), b.party)
	switch move.Kind {
	case rules.AllyCast:
		b.castSpell(g, cast{
			by: c, spell: move.Spell, foe: b.weakestLiving(), ally: move.Ally,
		})
	case rules.AllyUse:
		b.useItemFrom(g, c, move.Item, c)
	case rules.AllyGuard:
		b.guarding[c] = true
		b.log.AddColor(render.ColInkDim, "%s gets behind %s and stays there.", c.Name, c.Armor.Name)
	default:
		b.playerAttack(g, c, b.weakestLiving())
	}
}

// weakestLiving is the monster a companion goes for: the one closest to
// falling over. Finishing something off removes an attacker from the round,
// which is worth more than spreading damage around evenly.
func (b *battleScene) weakestLiving() int {
	best := -1
	for _, i := range b.living() {
		if best < 0 || b.mons[i].HP < b.mons[best].HP {
			best = i
		}
	}
	return best
}

// playerAttack resolves a weapon swing by any party member, not only the hero.
func (b *battleScene) playerAttack(g *Game, p *model.Character, idx int) {
	if idx < 0 || idx >= len(b.mons) || b.mons[idx].Dead {
		if l := b.firstLiving(); l >= 0 {
			idx = l
		} else {
			return
		}
	}
	m := b.mons[idx]
	str, dex := b.buffsFor(g, p)

	sw := rules.PlayerAttack(g.RNG, p, m, str, dex)
	if sw.Miss {
		g.Sound.Play("fight/miss")
		b.log.Add("%s", g.Write.Miss(g.RNG, p.Name, m.Short()))
		return
	}
	dmg, crit := sw.Damage, sw.Crit

	b.damageMonster(g, idx, dmg)
	// The swing lands where it lands. Played here rather than inside
	// damageMonster because that is also where a spell's damage goes, and a
	// fireball should not come with a sword slash behind it.
	//
	// A rod's free attack is a bolt, so it gets the bolt's burst. It costs no
	// psyche and is not on the technique list, which means the transcript and
	// this burst are the only places the game can say that this class's
	// ordinary round is magic. The wording is the weapon's own verb, as it is
	// for everybody else — a focus is authored with "spark at" and "overrule"
	// where a mace is authored with "clobber", because flavour is data and a
	// verb hard-coded here is a verb the content files cannot revise.
	if sw.Magic {
		b.playOnMonster(g, idx, "vfx/bolt")
	} else {
		b.playOnMonster(g, idx, b.nextSlash())
	}
	if crit {
		g.Sound.Play("fight/crit")
		b.cam.Shake(3)
	} else {
		g.Sound.Play("fight/hit")
	}
	b.log.Add("%s", g.Write.Hit(g.RNG, p.Name, p.Weapon.Verb, m.Short(), dmg, crit))
}

// buffsFor returns what the conditions riding on a member are worth to a blow.
// Blessings and potions both land in the same list now, so the hero's bottles
// and a companion's encouragement are added up the same way.
func (b *battleScene) buffsFor(_ *Game, c *model.Character) (str, dex int) {
	return rules.OffenseMod(c.Active), rules.DexterityMod(c.Active)
}

// castSpell resolves one technique. Which side it lands on comes from the
// kind, and how many of that side from the target — so a heal can never
// accidentally be aimed at a monster and a stun can never be aimed at a friend.
func (b *battleScene) castSpell(g *Game, c cast) {
	p, s := c.by, c.spell
	cost := rules.PsycheCost(p, s)
	if cost > p.Psyche {
		b.log.Add("%s reaches for it and finds nothing there.", p.Name)
		return
	}
	p.Psyche -= cost
	b.spentPsyche[p] += cost
	// Remembered only once it is actually paid for, so a technique the player
	// selected and then backed out of at the targeting step does not become
	// the one the menu opens on next round.
	if p == g.Player {
		g.LastSpell = s.ID
	}
	switch s.Kind {
	case model.SpellHeal, model.SpellRevive:
		g.Sound.Play("fight/heal")
	default:
		g.Sound.Play("fight/spell")
	}
	if s.Cast != "" {
		b.log.AddColor(render.ColMagic, "%s", castLine(s, p.Name))
	}

	if s.Kind.Side() == model.SideParty {
		b.castOnParty(g, c)
		return
	}
	b.castOnFoes(g, c)
}

// castOnParty applies the techniques that help: heals, blessings, and standing
// somebody back up.
func (b *battleScene) castOnParty(g *Game, c cast) {
	p, s := c.by, c.spell

	targets := []*model.Character{c.ally}
	switch {
	case s.Target == model.TargetSelf:
		targets = []*model.Character{p}
	case s.Target == model.TargetAll:
		// A blessing over the whole party reaches everyone upright; a revive
		// over the whole party reaches everyone who is not.
		targets = nil
		for _, m := range b.party {
			if m.Alive() != (s.Kind == model.SpellRevive) {
				targets = append(targets, m)
			}
		}
	case c.ally == nil:
		targets = []*model.Character{p}
	}

	for _, t := range targets {
		if t == nil {
			continue
		}
		fx, fy := b.memberFloat(t)
		b.playOnAlly(g, t, vfxForSpell(s))
		switch s.Kind {
		case model.SpellHeal:
			if !t.Alive() {
				b.log.Add("%s is past the point where that would help.", t.Name)
				continue
			}
			healed := t.Heal(rules.SpellDamage(g.RNG, p, s))
			b.log.AddColor(render.ColHeal, "%s closes up %d worth of %s.",
				p.Name, healed, damageOn(p, t))
			b.addFloater(fx, fy, fmt.Sprintf("+%d", healed), render.ColHeal)
		case model.SpellBless:
			if !t.Alive() {
				b.log.Add("%s is in no condition to be encouraged.", t.Name)
				continue
			}
			t.Active = rules.Apply(t.Active, model.Effect{
				Kind: model.EffectBless, Power: s.Power, Rounds: model.Forever,
			})
			b.log.AddColor(render.ColMagic, "%s hits harder for the rest of this.", t.Name)
		case model.SpellRevive:
			if t.Alive() {
				b.log.Add("%s is already standing, and says so.", t.Name)
				continue
			}
			b.standUp(g, t, rules.ReviveAmount(t, s.Power))
			b.addFloater(fx, fy, fmt.Sprintf("+%d", t.HP), render.ColHeal)
		}
	}
}

// castLine fills a technique's flavour line in. One placeholder, the same one
// the rest of the writing uses, and no format verbs anywhere near it.
func castLine(s model.Spell, caster string) string {
	return strings.ReplaceAll(s.Cast, "{A}", caster)
}

// drawBlurb puts what the highlighted technique does over the transcript.
//
// Over the transcript rather than beside the list, because the command panel is
// holding the list itself and every other pixel on this screen already belongs
// to something. The transcript is the one panel a player is not reading while
// they are choosing what to do — and now that the two sit side by side along
// the bottom, the explanation appears directly beside the row it explains.
func (b *battleScene) drawBlurb(g *Game, dst *ebiten.Image) {
	it, ok := b.menu.Selected()
	if !ok {
		return
	}
	s, ok := it.Data.(model.Spell)
	if !ok {
		return
	}
	// A solid ground first. ui.Panel is deliberately a little translucent, which
	// is right for a box over the world and wrong for one over the transcript:
	// three lines of combat log reading through three lines of explanation is
	// two things and neither of them legible.
	render.Rect(dst, barSideX, battleBarY, barSideW, battleBarH, color.RGBA{0x14, 0x10, 0x1C, 0xFF})
	ui.TitledPanel(dst, render.Trunc(s.Name, barSideW-24), barSideX, battleBarY, barSideW, battleBarH)
	y := battleBarY + 9
	for _, ln := range techniqueBlurb(g.Player, s, barSideW-22) {
		if y > battleBarY+battleBarH-12 {
			break
		}
		render.Text(dst, ln, barSideX+10, y, render.ColInk)
		y += render.LineH
	}
}

// damageOn phrases whose injuries are being closed, so a self-heal reads as
// "their own" rather than repeating the caster's name twice in one sentence.
func damageOn(caster, target *model.Character) string {
	if caster == target {
		return "their own damage"
	}
	return target.Name + "'s damage"
}

// standUp puts a fallen party member back on their feet.
func (b *battleScene) standUp(g *Game, c *model.Character, hp int) {
	c.HP = core.Clamp(hp, 1, c.MaxHP)
	b.partyHurt[c] = 0
	g.Sound.Play("fight/levelup")
	b.log.AddColor(render.ColHeal, "%s", g.Write.Revived(g.RNG, c.Name))
}

// castOnFoes applies the techniques that do not.
func (b *battleScene) castOnFoes(g *Game, c cast) {
	p, s, idx := c.by, c.spell, c.foe
	hx, hy := b.memberFloat(p)

	apply := func(i int) {
		m := b.mons[i]
		// One burst per target, before the arithmetic. A technique aimed at
		// everything should look like it reached everything, which is the one
		// thing the transcript is worst at saying — three lines of "takes 6"
		// read as three separate events.
		b.playOnMonster(g, i, vfxForSpell(s))
		switch s.Kind {
		case model.SpellDamage:
			// Ward is what the target has instead of armour against this, and
			// it is applied per target rather than to the roll: a fireball that
			// hits three things is resisted three times, once by each of them.
			d := rules.AfterWard(rules.SpellDamage(g.RNG, p, s), m.Ward)
			b.damageMonster(g, i, d)
			b.log.Add("%s takes %d.", m.Short(), d)
		case model.SpellDrain:
			d := rules.AfterWard(rules.SpellDamage(g.RNG, p, s), m.Ward)
			b.damageMonster(g, i, d)
			healed := p.Heal(d / 2)
			b.log.Add("%s takes %d; %s recovers %d of it.", m.Short(), d, p.Name, healed)
			b.addFloater(hx, hy, fmt.Sprintf("+%d", healed), render.ColHeal)
		case model.SpellWeaken:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectWeaken, Power: s.Power, Rounds: model.Forever,
			})
			b.log.Add("%s hits noticeably softer now.", m.Short())
		case model.SpellStun:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectStun, Power: 1, Rounds: 1,
			})
			b.log.Add("%s loses track of the fight entirely.", m.Short())
		case model.SpellPoison:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectPoison, Power: s.Power, Rounds: 4,
			})
			b.log.Add("%s has been given something it cannot metabolise.", m.Short())
		case model.SpellBurn:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectBurn, Power: s.Power, Rounds: 3,
			})
			b.log.Add("%s is on fire, and has noticed.", m.Short())
		case model.SpellSap:
			// Half the exchange. The other half lands on the caster once,
			// below, rather than once per target — otherwise pointing it at
			// three things would be three blessings, and a technique whose
			// value scales with how outnumbered you are is the wrong shape for
			// the one that is meant to even a fight up.
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectWeaken, Power: s.Power, Rounds: model.Forever,
			})
			b.log.Add("%s has less of whatever that was.", m.Short())
		case model.SpellPact:
			d := rules.AfterWard(rules.SpellDamage(g.RNG, p, s), m.Ward)
			b.damageMonster(g, i, d)
			b.log.Add("%s takes %d.", m.Name, d)
		}
	}

	if s.Target == model.TargetAll {
		for _, i := range b.living() {
			apply(i)
		}
		b.cam.Shake(2)
	} else {
		if idx < 0 || idx >= len(b.mons) || b.mons[idx].Dead {
			idx = b.firstLiving()
		}
		if idx >= 0 {
			apply(idx)
		}
	}
	b.settleUpWith(g, p, s, hx, hy)
}

// settleUpWith puts the caster's own half of a two-sided technique on them,
// once, after everything it did to the other side.
//
// Both directions exist and they are the point of the pair: a sap takes what it
// took off them and gives it to you, and a pact hits far above its band and
// leaves you wearing the difference. Doing it here rather than inside the
// per-target loop is what stops "aimed at three things" from being three times
// the bargain.
func (b *battleScene) settleUpWith(g *Game, p *model.Character, s model.Spell, hx, hy float64) {
	switch s.Kind {
	case model.SpellSap:
		p.Active = rules.Apply(p.Active, model.Effect{
			Kind: model.EffectBless, Power: s.Power, Rounds: model.Forever,
		})
		b.playOnAlly(g, p, "vfx/cross")
		b.addFloater(hx, hy, "+", render.ColMagic)
		b.log.AddColor(render.ColMagic, "%s is holding it now, and hits harder for it.", p.Name)
	case model.SpellPact:
		p.Active = rules.Apply(p.Active, model.Effect{
			Kind: model.EffectWeaken, Power: rules.PactCost(s), Rounds: model.Forever,
		})
		b.playOnAlly(g, p, "vfx/wind")
		b.log.AddColor(render.ColBlood, "%s will be feeling that for the rest of this.", p.Name)
	}
}

// useItem spends one item out of the hero's pack on whoever it was aimed at.
func (b *battleScene) useItem(g *Game, idx int, t *model.Character) {
	b.useItemFrom(g, g.Player, idx, t)
}

// useItemFrom spends one item out of a named pack.
//
// The holder and the target are separate because both directions happen: the
// hero hands a companion a potion out of his own bag, and a companion drinks
// one out of theirs without asking anybody.
func (b *battleScene) useItemFrom(g *Game, holder *model.Character, idx int, t *model.Character) {
	if holder == nil {
		holder = g.Player
	}
	it, ok := holder.TakeItem(idx)
	if !ok {
		return
	}
	if t == nil {
		t = holder
	}
	// You aim at somebody standing and the monsters get to them first. Only a
	// revive works on the fallen; without this, Character.Heal would clamp a
	// corpse up from zero and every healing potion in the game would quietly be
	// a resurrection, which is the one thing the revive items are for.
	if !t.Alive() && !it.Kind.WantsTheFallen() {
		b.log.Add("%s is past the point where the %s would help.", t.Name, it.Name)
		return
	}

	who := "%s downs the %s"
	if t != holder {
		who = "%s is handed the %s"
	}
	fx, fy := b.memberFloat(t)

	switch it.Kind {
	case model.ItemHeal:
		healed := t.Heal(it.Power)
		g.Sound.Play("fight/heal")
		b.log.AddColor(render.ColHeal, who+". %d back.", t.Name, it.Name, healed)
		b.addFloater(fx, fy, fmt.Sprintf("+%d", healed), render.ColHeal)
	case model.ItemPsyche:
		before := t.Psyche
		t.Psyche = core.Clamp(t.Psyche+it.Power, 0, t.MaxPsyche)
		b.log.AddColor(render.ColMagic, "%s recovers %d psyche.", t.Name, t.Psyche-before)
	case model.ItemRevive:
		if t.Alive() {
			b.log.Add("%s is upright, and finds the gesture insulting.", t.Name)
			return
		}
		b.standUp(g, t, rules.ReviveAmount(t, it.Power))
		b.addFloater(fx, fy, fmt.Sprintf("+%d", t.HP), render.ColHeal)
	case model.ItemCure:
		var removed []model.EffectKind
		t.Active, removed = rules.Cleanse(t.Active)
		if len(removed) == 0 {
			b.log.Add("%s has nothing in them worth removing. Yet.", t.Name)
			return
		}
		g.Sound.Play("fight/heal")
		b.log.AddColor(render.ColHeal, "%s is no longer %s.", t.Name, sufferingList(removed))
	case model.ItemBuff:
		// The bottle says what it does, and it does it to whoever drank it.
		kind := it.Effect
		if kind == "" {
			kind = model.EffectBless
		}
		t.Active = rules.Apply(t.Active, model.Effect{
			Kind: kind, Power: it.Power, Rounds: it.Rounds,
		})
		if kind == model.EffectQuicken {
			b.log.Add("%s feels quicker, and slightly wrong about it.", t.Name)
		} else {
			b.log.Add("%s feels stronger and considerably angrier.", t.Name)
		}
	default:
		b.log.Add("%s waves the %s at the problem. It does not help.", holder.Name, it.Name)
	}
}

func (b *battleScene) attemptFlee(g *Game) {
	fastest := 0
	for _, i := range b.living() {
		if s := b.mons[i].Speed; s > fastest {
			fastest = s
		}
	}
	// A company runs at the pace of whoever is slowest. Rolling on the hero's
	// speed would mean a fast thief could outrun a wolf while dragging two
	// people who plainly cannot.
	slowest := g.Player.Spd()
	for _, c := range b.livingParty() {
		if c.Spd() < slowest {
			slowest = c.Spd()
		}
	}
	if g.RNG.Chance(rules.FleeChance(slowest, fastest)) {
		b.result = 3
		g.Sound.Play("fight/flee")
		b.log.AddColor(render.ColGold, "%s leaves with what dignity remains. Which is none.", g.Player.Name)
		b.queue = nil
		b.finish(g)
		return
	}
	b.log.Add("%s turns to run and immediately reconsiders.", g.Player.Name)
}

func (b *battleScene) monsterTurn(g *Game, idx int) {
	m := b.mons[idx]
	if m.Dead || b.result != 0 {
		b.timer = 0
		return
	}
	if rules.Has(m.Active, model.EffectStun) {
		m.Active = rules.Remove(m.Active, model.EffectStun)
		b.log.AddColor(render.ColInkDim, "%s is still working out what happened.", m.Short())
		return
	}

	switch rules.ChooseMonsterAction(g.RNG, m, len(b.living()) == 1) {
	case rules.MonFlee:
		// It leaves the fight, and it leaves its purse. What it does not leave
		// is the whole reward — see awardSpoils.
		m.Dead = true
		m.Fled = true
		b.log.AddColor(render.ColInkDim, "%s decides this is not its problem and goes.", m.Short())
		return
	case rules.MonDefend:
		b.log.AddColor(render.ColInkDim, "%s hides behind %s.", m.Short(), m.Def.DefendWith)
		return
	}

	// A monster swings at whoever is in front of it, which is anyone still
	// upright. Spreading the incoming damage across the party is most of what
	// makes a companion worth the fee: they are an extra sword and, more
	// usefully, an extra place for a claw to land.
	living := b.livingParty()
	if len(living) == 0 {
		return
	}
	tgt := core.Pick(g.RNG, living)

	dmg := rules.MonsterDamage(g.RNG, tgt, m) + rules.OffenseMod(m.Active)
	if dmg < 0 {
		dmg = 0
	}
	// A retreat nobody bought leaves the player facing the wrong way.
	if b.feintFailed && tgt == g.Player {
		dmg = rules.FeintPunish(dmg)
	}
	if b.guarding[tgt] {
		dmg = rules.Defending(dmg)
	}

	verb, with := g.Write.MonsterAttack(g.RNG, m)
	g.Sound.Play("fight/monster")
	if dmg == 0 {
		b.log.Add("%s %s at %s with %s. %s %s it.",
			m.Short(), verb, tgt.Name, with, tgt.Armor.Name, tgt.Armor.Verb)
		return
	}
	// The barrier takes what it can first, and says so — a number that lands
	// somewhere other than the health bar is a number the player will read as a
	// bug in their own arithmetic unless the transcript accounts for it.
	var soaked int
	tgt.Active, dmg, soaked = rules.Soak(tgt.Active, dmg)
	if soaked > 0 {
		fx, fy := b.memberFloat(tgt)
		b.addFloater(fx, fy, fmt.Sprintf("-%d", soaked), render.ColMagic)
		if rules.Power(tgt.Active, model.EffectBarrier) == 0 {
			b.log.AddColor(render.ColMagic, "%s's %s takes %d and gives out.",
				tgt.Name, tgt.Shield.Name, soaked)
		} else {
			b.log.AddColor(render.ColMagic, "%s's %s takes %d of it.",
				tgt.Name, tgt.Shield.Name, soaked)
		}
		if dmg == 0 {
			g.Sound.Play("fight/hurt")
			return
		}
	}
	tgt.HP = core.Max(0, tgt.HP-dmg)
	b.spentHP[tgt] += dmg
	g.Sound.Play("fight/hurt")
	// A claw landing on your own row, small. The party panel is where a player
	// watches their own health, so this is the half of the fight that most
	// needed somewhere to point: the transcript names who was hit, and by the
	// time it is read the number has already moved.
	b.playOnAlly(g, tgt, b.nextSlash())
	b.partyHurt[tgt] = 14
	b.cam.Shake(float64(dmg) / 6)
	fx, fy := b.memberFloat(tgt)
	b.addFloater(fx, fy, fmt.Sprintf("-%d", dmg), render.ColBlood)
	b.log.AddColor(render.ColBlood, "%s %s %s with %s for %d.",
		m.Short(), verb, tgt.Name, with, dmg)

	// Some things leave more than a wound. A spider's bite is worth more than
	// its damage roll suggests, which is what makes the roster's stat lines
	// stop being the whole story about which monster you would rather meet.
	if e, ok := rules.RollAffliction(g.RNG, m.Def.Inflicts); ok && tgt.Alive() {
		tgt.Active = rules.Apply(tgt.Active, e)
		b.log.AddColor(render.ColBlood, "%s", g.Write.Afflicted(g.RNG, string(e.Kind), tgt.Name))
	}

	// A companion who runs out of hit points is out of the fight, not dead.
	// Permanently losing someone you paid for would make hiring anyone a bet
	// rather than a purchase, and nobody would take it twice.
	if !tgt.Alive() && tgt != g.Player {
		g.Sound.Play("fight/die")
		b.log.AddColor(render.ColBlood, "%s", g.Write.AllyDown(g.RNG, tgt.Name))
	}
}

func (b *battleScene) damageMonster(g *Game, idx, dmg int) {
	m := b.mons[idx]
	m.HP = core.Max(0, m.HP-dmg)
	b.hurt[idx] = 12
	b.addFloater(monSlotX(idx, len(b.mons)), monSlotY(idx, len(b.mons)),
		fmt.Sprintf("-%d", dmg), render.ColGold)
	if m.HP == 0 && !m.Dead {
		m.Dead = true
		g.Sound.Play("fight/die")
		g.noteQuestProgress(g.Quests.OnMonsterKilled(m.Def.ID))
		g.advanceThreads(thread.Event{Kind: thread.Kills, Monster: m.Def.ID})
		g.advanceSagas(saga.Event{Kind: saga.Hunt, Monster: m.Def.ID})
		b.log.AddColor(render.ColGold, "%s", g.Write.Death(g.RNG, m))
	}
}

func (b *battleScene) addFloater(x, y float64, text string, c color.RGBA) {
	b.floaters = append(b.floaters, floater{x: x, y: y, text: text, life: 46, col: c})
}

// checkEnd resolves victory or defeat, returning true when the battle is over.
func (b *battleScene) checkEnd(g *Game) bool {
	if b.result != 0 {
		return true
	}
	// The hero falling ends the fight either way. What it does to the run
	// depends on whether anybody was hired: with a company you are carried to a
	// town and charged for it, and alone you wake up at your last rest with
	// everything since undone. That is the whole argument for a party — not
	// that they hit things, but that somebody is there to pick you up.
	//
	// Any hireling counts, standing or not. A companion who runs out of hit
	// points is out of the fight and not dead — reviveFallen puts them back on
	// their feet the moment it stops — so a company wiped alongside the hero
	// still walks out of the room, and being offered a reload for a death
	// somebody was there for was the game contradicting its own fiction.
	if g.Player.HP <= 0 {
		b.result = 2
		if len(g.Allies) > 0 {
			b.result = 4
		}
		b.finish(g)
		return true
	}
	if len(b.living()) == 0 {
		b.result = 1
		b.awardSpoils(g)
		b.finish(g)
		return true
	}
	return false
}

func (b *battleScene) awardSpoils(g *Game) {
	p := g.Player
	killed := make([]*model.Monster, 0, len(b.mons))
	routed := make([]*model.Monster, 0, len(b.mons))
	for _, m := range b.mons {
		switch {
		case m.Fled:
			routed = append(routed, m)
		case m.HP <= 0:
			killed = append(killed, m)
		}
	}
	if len(killed)+len(routed) == 0 {
		return
	}

	// Driving something off pays. It used to pay nothing, which meant a fight
	// could end with the player having done ninety percent of the work on every
	// creature in it and walking away with an empty purse — and at levels one
	// to three, where an encounter is one or two creatures, that was most of
	// what "the monsters keep running away" felt like.
	//
	// The coins come across whole, because something running is not stopping to
	// collect anything. The experience is halved, because you did not finish
	// it. The gear roll below stays on kills alone: a runner still has whatever
	// it was wearing, since it is wearing it as it goes.
	xp := rules.XPAward(killed) + rules.RoutedXP(rules.XPAward(routed))
	coins := rules.CoinAward(g.RNG, killed) + rules.CoinAward(g.RNG, routed)

	// Something occasionally has gear on it.
	//
	// Rarer than a chest by a wide margin — a chest is a thing you went and
	// opened, and a fight is something that happened to you — but a fight was
	// paying only in coin and experience, so the only route to a named piece of
	// equipment was to go indoors and find furniture. It is held rather than
	// offered here: an Ask over a battle screen fights the battle for input,
	// and the spoils have not finished being read out.
	if b.result == 1 && g.RNG.Chance(0.07) {
		// Banded off what was actually fought, so the reward tracks the risk
		// rather than where the fight happened to start. Runners count for the
		// band even though they drop nothing themselves: the danger of the
		// encounter is what was in the room, not what was still in it at the
		// end.
		lv := 1
		for _, m := range append(append([]*model.Monster{}, killed...), routed...) {
			if m.Def.Level > lv {
				lv = m.Def.Level
			}
		}
		tier := core.Clamp(1+lv/3, 1, 5)
		if f, ok := g.rollAffixedGearOfTier(tier); ok {
			g.pendingFind = &f
		}
	}

	// Experience is not divided. Every member banks the full award and levels
	// on their own curve, which is what keeps a hireling taken on at level 6
	// still useful at level 12 without any catch-up machinery — and it leaves
	// the hero's progression exactly where the balance pass tuned it, because
	// splitting XP would silently re-tune the whole curve.
	//
	// The party is paid for out of the purse instead: each companion skims a
	// standing cut of every haul, which is a cost you feel at the shop counter
	// rather than one that slows down levelling.
	//
	// The cut goes into their own purse rather than out of the game. It used to
	// simply leave, and what it bought was a companion who re-armed for free
	// every time they levelled — the same arrangement with the arithmetic
	// hidden, and no way for a player to tell whether the standing charge on
	// everything they found was expensive. They spend it themselves now, in
	// towns, on one piece at a time. See Tables.Shop.
	var skimmed int64
	for _, c := range b.party {
		c.TotalXP += xp
		if c == p {
			c.SpendXP += xp
			continue
		}
		take := rules.Skim(coins, c.Cut)
		c.Coins += take
		skimmed += take
	}

	p.Coins += coins - skimmed
	g.Sound.Play("world/coins")
	if skimmed > 0 {
		b.log.AddColor(render.ColGold, "%d experience. %d coins, less %d in cuts.",
			xp, coins, skimmed)
	} else {
		b.log.AddColor(render.ColGold, "%d experience. %d coins.", xp, coins)
	}
	// Say that the ones which ran still paid, or the player reads the smaller
	// number as the game having taken something off them.
	if len(routed) > 0 {
		b.log.AddColor(render.ColInkDim, "%s dropped %s running. Half credit for the ones that got away.",
			plural(len(routed), "One", "Some"), plural(len(routed), "its purse", "their purses"))
	}

	// The bodies. A thief also works the ones that got away, which is the whole
	// of its share in a fight that ends with something bolting: everybody gets
	// the purse a runner drops, and the thief gets what it was holding.
	looted := killed
	if rules.Pickpocket(p) && len(routed) > 0 {
		looted = append(append([]*model.Monster{}, killed...), routed...)
	}
	for _, m := range looted {
		for name, n := range rules.RollLoot(g.RNG, m.Def.Loot) {
			it, ok := g.Data.Item(name)
			if !ok {
				continue
			}
			it.Count = n
			p.AddItem(it)
			g.Sound.Play("world/loot")
			if m.Fled {
				b.log.Add("Lifted %s on its way past.", itemLine(it))
			} else {
				b.log.Add("Picked up %s.", itemLine(it))
			}
		}
	}

	// Level-ups, hero loudly and companions in one line, so a three-strong
	// party does not bury the transcript in congratulations.
	for rules.PendingLevels(p) > 0 {
		rules.LevelUp(g.RNG, p)
		g.Sound.Play("fight/levelup")
		b.log.AddColor(render.ColHeal, "%s", g.Write.LevelUpLine(g.RNG, p.Level))
	}
	for _, c := range b.party {
		if c == p || rules.PendingLevels(c) == 0 {
			continue
		}
		for rules.PendingLevels(c) > 0 {
			rules.LevelUp(g.RNG, c)
		}
		// No free re-arm. This used to hand a companion the whole on-curve kit
		// for their new level on the spot, out of nowhere, while the cut they
		// had been taking went nowhere at all — two halves of one idea that
		// never met. What levelling does now is move the bar: the curve asks
		// for better gear, so they want better gear, and the next town is
		// where the money they have been skimming answers that.
		b.log.AddColor(render.ColHeal, "%s is now level %d, and mentions it.", c.Name, c.Level)
		if w, ok := g.Data.Wants(c); ok {
			b.log.AddColor(render.ColInkDim, "%s has started saving for %s.",
				c.Name, w.Gear.Titled())
		}
	}
	g.Quests.SyncFetch(p.Bag)
}

func (b *battleScene) finish(g *Game) {
	b.mode = modeDone
	switch b.result {
	case 1:
		g.Sound.Play("fight/victory")
		// The old magician chips in now and then, not every single time; a
		// catchphrase stops being funny the fourth time you hear it.
		if g.RNG.Chance(0.25) {
			g.Sound.Play("vo/victory")
		}
		b.log.AddColor(render.ColGold, "Victory. Press Z.")
	case 2:
		g.Sound.Play("fight/defeat")
		g.Sound.Play("vo/death")
		b.log.AddColor(render.ColBlood, "You die.")
		// No "press Z". Dying is the one ending the player does not get to
		// acknowledge: the screen goes rather than waiting to be dismissed,
		// which is most of the difference between an outcome and a dialog box.
		b.fade = 1
	case 3:
		b.log.AddColor(render.ColGold, "Escaped. Press Z.")
	case 4:
		g.Sound.Play("fight/defeat")
		b.log.AddColor(render.ColBlood, "%s goes down. Somebody is still up. Press Z.", g.Player.Name)
	}
	// Anyone who fell gets up once the fighting stops — including the hero,
	// whose company is about to carry him somewhere with a roof.
	if b.result != 2 {
		b.reviveFallen(g)
		b.catchBreath(g)
	}
	// A fight the company came out of, which is what a backstory beat counts.
	// Dying is not a shared anecdote.
	if b.result == 1 && len(g.Allies) > 0 {
		g.advanceThreads(thread.Event{Kind: thread.Fights, N: 1})
	}
	// Copy the battle transcript into the world log so it survives the pop.
	g.sinceFight = 0
}

// catchBreath hands everybody still standing part of what the fight took off
// them. Won or run from: a retreat already pays nothing, and charging it the
// full price of the fight on top of that is what made leaving the answer
// nobody picks.
//
// It is announced rather than applied quietly. The whole point of it is that
// the player can feel the encounter costing less than it looks like it costs,
// and a number that moves while nobody is saying why is a number they will
// assume is a bug in their own reading of the bar.
func (b *battleScene) catchBreath(g *Game) {
	for _, c := range b.party {
		hp, sp := rules.CatchBreath(c, b.spentHP[c], b.spentPsyche[c])
		if hp == 0 && sp == 0 {
			continue
		}
		switch {
		case sp == 0:
			b.log.AddColor(render.ColHeal, "%s gets their breath back. +%d HP.", c.Name, hp)
		case hp == 0:
			b.log.AddColor(render.ColHeal, "%s gets their breath back. +%d SP.", c.Name, sp)
		default:
			b.log.AddColor(render.ColHeal, "%s gets their breath back. +%d HP, +%d SP.", c.Name, hp, sp)
		}
	}
}

// reviveFallen stands the downed companions back up on one hit point once the
// fighting stops. They cost a rest rather than a funeral, so the party leaves a
// bad fight intact but in no condition to walk into another one.
func (b *battleScene) reviveFallen(g *Game) {
	for _, c := range b.party {
		if c == g.Player || c.Alive() {
			continue
		}
		c.HP = 1
		b.log.AddColor(render.ColHeal, "%s", g.Write.AllyUp(g.RNG, c.Name))
	}
}

// Pop handling: on a rescue the company relocates the run, and on a real defeat
// the run is over.
func (b *battleScene) onPopped(g *Game) {
	switch b.result {
	case 1:
		if f := g.pendingFind; f != nil {
			g.pendingFind = nil
			// A creature was carrying it, so it was not under anything.
			g.takeFind(*f, "It was carrying")
		}
	case 2:
		g.offerRewind()
	case 4:
		g.rescueToTown()
	}
}

// offerRewind puts the run back to the last time the player was safe, if they
// want it and there is anything to go back to.
//
// The offer is on a black screen the battle faded into, so the question arrives
// after the run has visibly ended rather than over the corpse.
//
// What it costs is the thing worth being explicit about. It used to rewind one
// fight, which is barely a cost — the run resumed from a step already taken.
// The checkpoint is a bed now, so going back costs everything since the last
// one, and the box says how long that is. A player choosing between an hour
// replayed and a run ended should be choosing with the number in front of them.
func (g *Game) offerRewind() {
	toTitle := func(g *Game) {
		for len(g.stack) > 0 {
			g.Pop()
		}
		g.quit = false
		g.Push(newTitleScene(g))
	}
	// The newest save of *this* character, in whatever slot it is, rather than
	// the autosave specifically. A player who saved by hand ten minutes ago and
	// last slept an hour ago should be offered the ten minutes — and the
	// autosave outlives the run that wrote it, so reaching for it by name could
	// hand somebody back a character who is not theirs.
	sl, ok := save.LatestForRun(g.Root, g.Seed, g.Player.Name+" "+g.Player.Epithet)
	if !ok {
		toTitle(g)
		return
	}

	// The black stays under the question. Popped by whichever answer wins:
	// loading rebuilds the stack from nothing, and letting it stand empties it.
	g.Push(&deathScene{})

	when := humanAge(sl.Saved)
	if when == "" {
		when = "some time ago"
	}
	g.Ask("", "There is a version of this where you were still asleep.\n\n"+
		"The last time you were safe was "+when+". Everything since would "+
		"have to happen again.",
		[]string{"Go back to it", "Let it stand"},
		func(g *Game, choice int) {
			if choice != 0 {
				toTitle(g)
				return
			}
			if err := g.LoadFrom(sl.Name); err != nil {
				g.Log.AddColor(render.ColBlood, "The moment would not come back: %v", err)
				toTitle(g)
				return
			}
			g.Sound.Play("world/enter")
			g.Log.AddColor(render.ColGold, "You wake up, and decide against all of it.")
		})
}

// --- layout helpers -------------------------------------------------------

// The battle screen, in one place.
//
// It used to be monsters in a row across the top, a transcript across the
// middle, and the company and the command list sharing the bottom — which put
// the two things you compare, your people and their people, at opposite ends of
// the screen with a wall of text between them. Deciding who to hit meant
// looking up, and deciding whether you could afford to meant looking down.
//
// So: your side down the left, theirs on the right, and the two things that are
// words rather than pictures along the bottom. It is the arrangement every
// console RPG of a certain age arrived at, and for the reason they arrived at
// it — a fight is a comparison, and a comparison wants both halves side by side.
const (
	// The left column: the company, one above another.
	partyPanelX = 6.0
	partyPanelY = 6.0
	partyPanelW = 140.0
	partyPanelH = 176.0

	// The right field: whatever is in front of you.
	foeFieldX = 152.0
	foeFieldY = 6.0
	foeFieldW = render.ScreenW - foeFieldX - 6
	foeFieldH = 176.0

	// The bottom strip: what just happened, all the way across.
	battleBarY = 186.0
	battleBarH = render.ScreenH - battleBarY - 6
	logPanelX  = 6.0
	logPanelW  = render.ScreenW - 12.0

	// The command panel overdraws the left end of it rather than sitting
	// beside it.
	//
	// Two boxes down there meant the transcript was permanently two thirds of
	// a screen wide, including while it was being written to — which is the one
	// moment anybody reads it. As an overlay it only takes the room while there
	// is a command to give: when the round is resolving there is nothing to
	// press, so there is no panel, and the transcript has the whole bar.
	cmdPanelX = 6.0
	cmdPanelW = 210.0
	// What is left of the bar beside the command panel, for the transcript to
	// indent into and for the technique popover to open in.
	barSideX = cmdPanelX + cmdPanelW + 4
	barSideW = render.ScreenW - 6 - barSideX
)

// commandsUp reports whether the command panel is covering the left of the bar.
//
// Only the modes that have something to press. Busy and done have "..." and
// "Press Z" respectively, and neither is worth a box over the transcript at the
// exact moment the transcript is saying what happened.
func (b *battleScene) commandsUp() bool {
	switch b.mode {
	case modeRoot, modeSpell, modeItem, modeTarget, modeAllyPick:
		return true
	}
	return false
}

// monSlot is the cell one creature occupies in the right-hand field: the centre
// to hang it on, and how much room it has.
//
// Up to three across and as many rows as that needs. Four is the most an
// encounter is meant to send (party.MaxFoes) and a pack can push it to six, so
// the grid has to hold six without any of them overlapping — which a single row
// of six at this width cannot.
func monSlot(i, n int) (cx, cy, w, h float64) {
	if n < 1 {
		n = 1
	}
	// Three across is the widest row, but four goes two-and-two rather than
	// three-and-one: four is the most an ordinary encounter sends, and a square
	// of them reads as a group where a row with one straggler under it reads as
	// a mistake.
	cols := monCols(n)
	rows := (n + cols - 1) / cols
	w = foeFieldW / float64(cols)
	h = foeFieldH / float64(rows)
	// The last row is centred on what is left rather than left-aligned, so five
	// creatures read as a group and not as a full shelf with a gap in it.
	inRow := core.Min(cols, n-(i/cols)*cols)
	col := i % cols
	rowW := w * float64(inRow)
	cx = foeFieldX + (foeFieldW-rowW)/2 + w*(float64(col)+0.5)
	cy = foeFieldY + h*(float64(i/cols)+0.5)
	return cx, cy, w, h
}

// monCols is how many creatures stand across one row of the field.
//
// Three across is the widest row, but four goes two-and-two rather than
// three-and-one: four is the most an ordinary encounter sends, and a square of
// them reads as a group where a row with one straggler under it reads as a
// mistake.
func monCols(n int) int {
	if n == 4 {
		return 2
	}
	return core.Max(1, core.Min(n, 3))
}

// monSlotX is the horizontal centre alone, for the callers that only want to
// know where to put a number.
func monSlotX(i, n int) float64 {
	cx, _, _, _ := monSlot(i, n)
	return cx
}

// monSlotY is the vertical centre alone.
func monSlotY(i, n int) float64 {
	_, cy, _, _ := monSlot(i, n)
	return cy
}

// monNameLines is how many lines a creature's name plate may take under its
// portrait: the species, and then what sort of one it is.
//
// Three when the field is a single row and two when it is stacked, and both
// numbers exist to fit the *whole* name rather than as much of it as happened
// to go in. It was two and one, which was enough for the species alone, and a
// captured frame showed what that actually looked like: five creatures labelled
// "Goblin Middle" and "Overfamiliar", each the first row of a wrapped name with
// the rest silently dropped and nothing on screen to say so. That is the
// transcript's own bug — an entry shown in part, reading as a different and
// shorter thing — moved under a portrait.
//
// Four when the field is a single row, three when it is stacked, and the
// difference is what the vertical room will pay for rather than a preference.
// A single row hangs its portraits in 176 pixels, so the fourth line is free —
// the portrait stays at its 96-pixel cap either way. A stacked row has 88, and
// the third line is what takes the portrait down to 36; a fourth would take the
// two rows past the field and into each other.
//
// Four is not arbitrary either. It is what the widest names in the roster
// actually need at three columns: "Living Armour" is two lines on its own and
// "Two People Inside" is another two. At three lines the test that measures
// this said so, which is the whole reason it exists.
func monNameLines(n int) int {
	if (n+monCols(n)-1)/monCols(n) == 1 {
		return 4
	}
	return 3
}

// monBelow is the height of everything under the portrait: the health meter,
// the condition pips, and the name.
func monBelow(n int) float64 { return 14 + render.LineH*float64(monNameLines(n)) }

// monBox is the portrait rectangle inside a cell, and the baseline the name and
// meter hang off.
func monBox(i, n int) (x, top, w, h float64) {
	cx, cy, cw, ch := monSlot(i, n)
	below := monBelow(n)
	w = core.ClampF(cw-16, 40, 104)
	h = core.ClampF(ch-4-below, 36, 96)
	// Never wider than it is tall. The art is square and ScreenFit keeps it
	// that way, so a box 91 across and 36 down is a 36-pixel creature with
	// twenty-seven pixels of empty frame on either side of it — which reads as
	// a portrait that failed to load rather than as a small portrait. The name
	// under it still gets the whole slot width, so squaring the box costs the
	// label nothing.
	if w > h {
		w = h
	}
	// The portrait and everything under it are centred as one block, so a lone
	// creature sits in the middle of the field rather than hanging from the top
	// of it.
	top = cy - (h+below)/2
	return cx - w/2, top, w, h
}

// partyRowY is the top of one member's row in the left column.
//
// Centred on the panel rather than stacked from the top: a solo hero in a
// column built for three should be in the middle of it, not floating at the
// ceiling with two rows of nothing underneath.
func partyRowY(i, n int) float64 {
	if n < 1 {
		n = 1
	}
	block := float64(n) * partyRowH
	return partyPanelY + (partyPanelH-block)/2 + float64(i)*partyRowH
}

// drawNamePlate writes a creature's name under its portrait: the species, then
// the epithet in dimmer ink, wrapped into whatever lines the layout affords.
//
// The head is served first and the epithet gets the remainder, because the
// species is the half that answers "which one do I hit" and the epithet is the
// half that answers "what am I looking at". A name that needs every line leaves
// nothing for its epithet, which is correct: those are the whole-phrase names,
// where the phrase *is* the epithet.
//
// Nothing is ever dropped silently. A line that will not fit is truncated,
// which at least reads as cut; a line quietly not drawn reads as the whole
// name, and that is how "Goblin Middle Manager" appeared on screen as "Goblin
// Middle" for as long as it did.
func (b *battleScene) drawNamePlate(dst *ebiten.Image, cx, y, slotW float64,
	head, tail string, col color.Color, dead bool) {

	budget := monNameLines(len(b.mons))
	lines := render.Wrap(head, slotW-8)
	if len(lines) > budget {
		lines = lines[:budget]
		// The last line carries the cut rather than the name simply stopping.
		lines[budget-1] = render.Trunc(lines[budget-1]+" "+wrapTail(head, lines), slotW-6)
	}
	headLines := len(lines)

	// The epithet is dimmer than the species, and stays dim when the cursor
	// lands on the slot: the gold species is already the whole of "this one",
	// and a second gold line would compete with it for the same job.
	sub := render.ColInkDim
	if dead {
		sub = render.ColInkFaint
	}
	var tailLines []string
	if tail != "" && headLines < budget {
		tailLines = render.Wrap(tail, slotW-8)
		if room := budget - headLines; len(tailLines) > room {
			tailLines = tailLines[:room]
			tailLines[room-1] = render.Trunc(tailLines[room-1]+" "+wrapTail(tail, tailLines), slotW-6)
		}
	}

	for j, ln := range lines {
		render.TextCenter(dst, render.Trunc(ln, slotW-6), cx, y+float64(j)*render.LineH, col)
	}
	for j, ln := range tailLines {
		render.TextCenter(dst, render.Trunc(ln, slotW-6), cx,
			y+float64(headLines+j)*render.LineH, sub)
	}
}

// wrapTail is whatever of a phrase the kept lines did not carry, so a truncated
// last line ends in the beginning of the words it is cutting rather than
// stopping cleanly at a word boundary and reading as complete.
func wrapTail(full string, kept []string) string {
	used := 0
	for _, ln := range kept {
		used += len(ln) + 1
	}
	if used >= len(full) {
		return ""
	}
	return full[used:]
}

// monsterName splits a creature's name into what it is and what sort of one.
//
// A thin wrapper on model.SplitName, which is where the rule lives now: the
// writing needs the same split, and two copies of "find the comma, keep the
// group letter with the species" is two copies to disagree.
func monsterName(full string) (head, tail string) { return model.SplitName(full) }

// drawAllyCursor frames the party row the cursor is on. It borrows the gold
// frame the monster cursor uses, so "this is the thing you are about to act on"
// looks the same whichever side of the fight it is on.
func (b *battleScene) drawAllyCursor(g *Game, dst *ebiten.Image) {
	if b.allyPick < 0 || b.allyPick >= len(b.party) {
		return
	}
	ry := partyRowY(b.allyPick, len(b.party))
	render.Frame(dst, partyPanelX+2, ry, partyPanelW-4, partyRowH-4, render.ColGold)
	if (g.Tick()/12)%2 == 0 {
		render.Text(dst, ">", partyPanelX-3, ry+14, render.ColGold)
	}
}

// memberFloat is where a damage or healing number pops for a party member: over
// their own row, so with three of them on screen it is never a guess who just
// took the hit. A solo hero keeps the original spot beside their portrait.
func (b *battleScene) memberFloat(c *model.Character) (float64, float64) {
	for i, m := range b.party {
		if m == c {
			return partyPanelX + partyPanelW/2, partyRowY(i, len(b.party)) + 8
		}
	}
	return partyPanelX + partyPanelW/2, partyPanelY + partyPanelH/2
}

// nearestLiving snaps a raw slot index to the closest creature still standing,
// so a row-step onto a corpse or off the end of the grid still lands somewhere
// the player can hit.
func (b *battleScene) nearestLiving(want int) int {
	l := b.living()
	if len(l) == 0 {
		return b.target
	}
	best, bestD := l[0], 1<<30
	for _, i := range l {
		if d := core.Abs(i - want); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

func nextIn(list []int, cur int) int {
	for i, v := range list {
		if v == cur {
			return list[(i+1)%len(list)]
		}
	}
	return list[0]
}

func prevIn(list []int, cur int) int {
	for i, v := range list {
		if v == cur {
			return list[(i-1+len(list))%len(list)]
		}
	}
	return list[0]
}

// --- drawing --------------------------------------------------------------

func (b *battleScene) Draw(g *Game, dst *ebiten.Image) {
	// A dim, desaturated version of wherever the fight started.
	if b.under != nil {
		b.under.Draw(g, dst)
	}
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH, color.RGBA{0x0A, 0x06, 0x10, 0xE0})

	ox, oy := b.cam.Offset()

	// Them, down the right.
	for i, m := range b.mons {
		bx, top, boxW, boxH := monBox(i, len(b.mons))
		_, _, slotW, _ := monSlot(i, len(b.mons))
		cx := bx + boxW/2 + ox
		top += oy

		tint := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
		switch {
		case m.Dead:
			tint = color.RGBA{0x50, 0x40, 0x50, 0x90}
		case b.hurt[i] > 0 && (b.hurt[i]/3)%2 == 0:
			tint = color.RGBA{0xFF, 0x90, 0x90, 0xFF}
		}
		// The frame goes down first and carries the state, so a dead thing is
		// a dim picture in a dim frame rather than a dim picture on the same
		// bright field as its living neighbours. Three portraits on a black
		// screen with nothing between them was the one place the interface
		// stopped saying where anything ended.
		var edge color.Color
		switch {
		case m.Dead:
			edge = render.ColInkFaint
		case b.hurt[i] > 0:
			edge = render.ColBlood
		}
		ui.Slot(dst, cx-boxW/2-2, top-2, boxW+4, boxH+4, edge)

		sprite := g.Assets.Get(m.Def.Sprite)
		render.ScreenFit(dst, sprite, 0, cx-boxW/2, top, boxW, boxH, tint)

		// Name plate and health.
		nameCol := render.ColInk
		if m.Dead {
			nameCol = render.ColInkFaint
		}
		if b.mode == modeTarget && b.target == i && !m.Dead {
			// Redrawn in gold over the resting frame rather than instead of
			// it, so the slot never changes size when it is picked.
			render.Frame(dst, cx-boxW/2-2, top-2, boxW+4, boxH+4, render.ColGold)
			if (g.Tick()/12)%2 == 0 {
				render.TextCenter(dst, "v", cx, top-13, render.ColGold)
			}
			nameCol = render.ColGold
		}
		if !m.Dead {
			ui.Bar(dst, cx-boxW/2, top+boxH+2, boxW, 4, m.HPFrac(), render.ColBlood)
			drawEffectPips(dst, cx-boxW/2, top+boxH+8, m.Active)
		}
		// What it is, and then what sort of one — both, under every portrait.
		//
		// The epithet used to wait for the target cursor, on the grounds that
		// it was the half that would not fit. It is the half that is funny, and
		// a joke the player has to point at to read is a joke that lands once
		// per fight instead of four times. It is dim rather than absent, so the
		// species still reads first and the plate still answers "what is that"
		// before it answers "what sort".
		head, tail := monsterName(m.Name)
		b.drawNamePlate(dst, cx, top+boxH+12, slotW, head, tail, nameCol, m.Dead)
	}

	// Effects over the portraits, under everything with a number on it.
	b.drawBursts(g, dst, false)

	// Us, down the left.
	g.drawPartyPanel(dst, partyPanelX, partyPanelY, partyPanelW, partyPanelH, b.partyHurt)
	if b.mode == modeAllyPick {
		b.drawAllyCursor(g, dst)
	}

	// The transcript, across the whole bottom.
	//
	// Its heading carries the popover's hint, because the panel being
	// advertised is the space the explanation opens in.
	logTitle := ""
	if b.mode == modeSpell && !b.blurb {
		logTitle = "left/right explains one"
	}
	ui.TitledPanel(dst, logTitle, logPanelX, battleBarY, logPanelW, battleBarH)
	// Indented past the command panel while there is one, so the lines are
	// whole sentences starting where you can see them rather than the tails of
	// sentences beginning underneath a box.
	lx, lw := logPanelX+8, logPanelW-18
	if b.commandsUp() {
		lx, lw = barSideX+8, barSideW-14
	}
	b.log.DrawWrapped(dst, lx, battleBarY+6, lw, 5)

	// The effects that land on the party panel, which has just been drawn over
	// where they are.
	b.drawBursts(g, dst, true)

	// The technique popover, over the transcript. Drawn before the command
	// panel so the list it belongs to stays on top of it.
	if b.mode == modeSpell && b.blurb {
		b.drawBlurb(g, dst)
	}

	// Command panel.
	title := map[battleMode]string{
		modeRoot: "", modeSpell: "technique", modeItem: "pack",
		modeTarget: "target", modeAllyPick: "on whom", modeBusy: "", modeDone: "",
	}[b.mode]
	if !b.commandsUp() {
		// Nothing to press. Say so at the end of the bar rather than in a box
		// over it — a panel here would cover the line it is reacting to.
		if b.mode == modeDone && b.fade == 0 {
			render.TextRight(dst, "Press Z", render.ScreenW-14,
				battleBarY+battleBarH-14, render.ColGold)
		}
		b.drawFloaters(dst)
		b.drawFade(dst)
		return
	}

	// A solid ground under it first. ui.Panel is a little translucent, which is
	// right over the world and wrong over four lines of transcript: the two
	// read through each other and neither is legible.
	render.Rect(dst, cmdPanelX, battleBarY, cmdPanelW, battleBarH, color.RGBA{0x14, 0x10, 0x1C, 0xFF})
	ui.TitledPanel(dst, title, cmdPanelX, battleBarY, cmdPanelW, battleBarH)
	const tx = cmdPanelX + 10
	switch b.mode {
	case modeRoot, modeSpell, modeItem:
		b.menu.Draw(dst, cmdPanelX+12, battleBarY+6, cmdPanelW-24)
	case modeTarget:
		// The epithet, here, where there is room for it. This is the moment it
		// is worth reading: the player is looking at four portraits deciding
		// which one to hit, and "Territorial" is the entire answer to what sort
		// of crab this is.
		if b.target >= 0 && b.target < len(b.mons) {
			head, tail := monsterName(b.mons[b.target].Name)
			render.Text(dst, render.Trunc(head, cmdPanelW-24), tx, battleBarY+8, render.ColGold)
			if tail != "" {
				for i, ln := range render.Wrap(tail, cmdPanelW-24) {
					if i > 1 {
						break
					}
					render.Text(dst, ln, tx, battleBarY+22+float64(i)*render.LineH, render.ColInk)
				}
			}
		}
		render.Text(dst, "Arrows choose - Z commits", tx, battleBarY+52, render.ColInkFaint)
	case modeAllyPick:
		render.Text(dst, "Up / Down to choose.", tx, battleBarY+16, render.ColInk)
		render.Text(dst, "Z commits. X reconsiders.", tx, battleBarY+30, render.ColInkDim)
	}

	b.drawFloaters(dst)
	b.drawFade(dst)
}

// drawFloaters paints the damage and healing numbers rising off whoever earned
// them. Split out of Draw so the mode with no command panel can end early and
// still finish the frame the same way.
func (b *battleScene) drawFloaters(dst *ebiten.Image) {
	for _, f := range b.floaters {
		alpha := uint8(core.Clamp(f.life*6, 0, 255))
		c := f.col
		c.A = alpha
		render.TextCenter(dst, f.text, f.x, f.y, c)
	}
}

// drawFade is the lights going out, over everything including the interface.
func (b *battleScene) drawFade(dst *ebiten.Image) {
	if b.fade <= 0 {
		return
	}
	// Linear, after a cubic ease turned out to spend the first second doing
	// nothing visible and then slam shut — which is not a fade over two
	// seconds, it is a delay followed by a cut.
	f := core.ClampF(float64(b.fade)/deathFade, 0, 1)
	render.Rect(dst, 0, 0, render.ScreenW, render.ScreenH,
		color.RGBA{0, 0, 0, uint8(f * 255)})
}
