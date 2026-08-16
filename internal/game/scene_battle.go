package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
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
const stepTicks = 30

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

	cam       render.Camera
	floaters  []floater
	hurt      []int // per-monster hit flash timer
	partyHurt map[*model.Character]int

	// guarding is who braced this round. Per member rather than a single flag,
	// because a companion deciding to cover up must not also halve what the
	// hero takes. It is not an Effect: bracing is a stance held for one round,
	// not a condition with a duration.
	guarding map[*model.Character]bool

	// result is 0 running, 1 victory, 2 defeat, 3 fled, 4 the hero went down
	// but the company did not.
	result   int
	round    int
	introRun bool
}

func newBattleScene(g *Game, mons []*model.Monster, where string) *battleScene {
	b := &battleScene{
		under:     g.Top(),
		mons:      mons,
		where:     where,
		party:     g.Party(),
		log:       ui.NewLog(60),
		hurt:      make([]int, len(mons)),
		partyHurt: map[*model.Character]int{},
		guarding:  map[*model.Character]bool{},
	}
	// Nobody carries anything in from the last fight.
	for _, c := range b.party {
		c.Active = nil
	}
	b.cam = render.Camera{}
	b.setRootMenu(g)

	names := mons[0].Name
	if len(mons) > 1 {
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
			b.log.AddColor(render.ColGold, "%s %s for %d.", t.Kind.Verb(), m.Name, t.Damage)
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
			c.HP = core.Max(0, c.HP-t.Damage)
			b.partyHurt[c] = 10
			fx, fy := b.memberFloat(c)
			b.addFloater(fx, fy, fmt.Sprintf("-%d", t.Damage), render.ColBlood)
			b.log.AddColor(render.ColBlood, "%s %s for %d.", t.Kind.Verb(), c.Name, t.Damage)
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
	// The root menu is text-only; icons return when a list of things appears.
	b.menu.Icons = nil
	spells := g.Data.SpellsFor(g.Player)
	b.menu.SetItems([]ui.MenuItem{
		{Label: "Attack", Detail: g.Player.Weapon.Name},
		{Label: "Technique", Detail: fmt.Sprintf("%d SP", g.Player.Psyche), Disabled: len(spells) == 0},
		{Label: "Item", Detail: fmt.Sprintf("%d", len(g.Player.Bag)), Disabled: len(g.Player.Bag) == 0},
		{Label: "Defend", Detail: "brace"},
		{Label: "Flee", Detail: "sensible"},
	})
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

	switch b.mode {
	case modeBusy:
		b.updateBusy(g)
	case modeDone:
		if g.Accept() || Cancel() {
			g.Pop()
			b.onPopped(g)
		}
	default:
		b.updateMenus(g)
	}
	return nil
}

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

func (b *battleScene) chooseRoot(g *Game) {
	switch b.menu.Index {
	case 0: // Attack
		b.beginTargeting(modeRoot)
	case 1: // Technique
		spells := g.Data.SpellsFor(g.Player)
		fallen := b.anyoneDown()
		items := make([]ui.MenuItem, 0, len(spells)+1)
		for _, s := range spells {
			// Greying out a revive while everybody is upright is the same
			// courtesy as greying out a technique you cannot pay for: the menu
			// should not offer a move that does nothing.
			off := s.Cost > g.Player.Psyche ||
				(s.Kind == model.SpellRevive && !fallen)
			items = append(items, ui.MenuItem{
				Label: s.Name, Detail: fmt.Sprintf("%d SP", s.Cost), Icon: s.Icon,
				Disabled: off, Data: s,
			})
		}
		if len(items) == 0 {
			return
		}
		b.menu.Icons = g.Assets
		b.menu.SetItems(items)
		// Icon rows are taller, so fewer fit the command panel.
		b.menu.Visible = 3
		b.mode = modeSpell
	case 2: // Item
		fallen := b.anyoneDown()
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
	case 3: // Defend
		b.runRound(g, func(g *Game) {
			b.guarding[g.Player] = true
			b.log.Add("%s sets their feet and waits for it.", g.Player.Name)
		})
	case 4: // Flee
		b.runRound(g, func(g *Game) { b.attemptFlee(g) })
	}
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
		if rules.Initiative(g.RNG, member.Speed, fastest) {
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
		b.log.Add("%s", g.Write.Miss(g.RNG, p.Name, m.Name))
		return
	}
	dmg, crit := sw.Damage, sw.Crit

	b.damageMonster(g, idx, dmg)
	if crit {
		g.Sound.Play("fight/crit")
		b.cam.Shake(3)
	} else {
		g.Sound.Play("fight/hit")
	}
	b.log.Add("%s", g.Write.Hit(g.RNG, p.Name, p.Weapon.Verb, m.Name, dmg, crit))
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
	if s.Cost > p.Psyche {
		b.log.Add("%s reaches for it and finds nothing there.", p.Name)
		return
	}
	p.Psyche -= s.Cost
	switch s.Kind {
	case model.SpellHeal, model.SpellRevive:
		g.Sound.Play("fight/heal")
	default:
		g.Sound.Play("fight/spell")
	}
	if s.Cast != "" {
		b.log.AddColor(render.ColMagic, "%s", fmt.Sprintf(s.Cast, p.Name))
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
		switch s.Kind {
		case model.SpellDamage:
			d := rules.SpellDamage(g.RNG, p, s)
			b.damageMonster(g, i, d)
			b.log.Add("%s takes %d.", m.Name, d)
		case model.SpellDrain:
			d := rules.SpellDamage(g.RNG, p, s)
			b.damageMonster(g, i, d)
			healed := p.Heal(d / 2)
			b.log.Add("%s takes %d; %s recovers %d of it.", m.Name, d, p.Name, healed)
			b.addFloater(hx, hy, fmt.Sprintf("+%d", healed), render.ColHeal)
		case model.SpellWeaken:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectWeaken, Power: s.Power, Rounds: model.Forever,
			})
			b.log.Add("%s hits noticeably softer now.", m.Name)
		case model.SpellStun:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectStun, Power: 1, Rounds: 1,
			})
			b.log.Add("%s loses track of the fight entirely.", m.Name)
		case model.SpellPoison:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectPoison, Power: s.Power, Rounds: 4,
			})
			b.log.Add("%s has been given something it cannot metabolise.", m.Name)
		case model.SpellBurn:
			m.Active = rules.Apply(m.Active, model.Effect{
				Kind: model.EffectBurn, Power: s.Power, Rounds: 3,
			})
			b.log.Add("%s is on fire, and has noticed.", m.Name)
		}
	}

	if s.Target == model.TargetAll {
		for _, i := range b.living() {
			apply(i)
		}
		b.cam.Shake(2)
		return
	}
	if idx < 0 || idx >= len(b.mons) || b.mons[idx].Dead {
		idx = b.firstLiving()
	}
	if idx >= 0 {
		apply(idx)
	}
}

// useItem spends one item out of the hero's pack on whoever it was aimed at.
// The pack is always the hero's; the target need not be.
func (b *battleScene) useItem(g *Game, idx int, t *model.Character) {
	it, ok := g.Player.TakeItem(idx)
	if !ok {
		return
	}
	if t == nil {
		t = g.Player
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
	if t != g.Player {
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
		b.log.Add("%s waves the %s at the problem. It does not help.", g.Player.Name, it.Name)
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
	slowest := g.Player.Speed
	for _, c := range b.livingParty() {
		if c.Speed < slowest {
			slowest = c.Speed
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
		b.log.AddColor(render.ColInkDim, "%s is still working out what happened.", m.Name)
		return
	}

	switch rules.ChooseMonsterAction(g.RNG, m) {
	case rules.MonFlee:
		m.Dead = true // it leaves the fight; no experience for a runner
		b.log.AddColor(render.ColInkDim, "%s decides this is not its problem and goes.", m.Name)
		return
	case rules.MonDefend:
		b.log.AddColor(render.ColInkDim, "%s hides behind %s.", m.Name, m.Def.DefendWith)
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
	if b.guarding[tgt] {
		dmg = rules.Defending(dmg)
	}

	verb, with := g.Write.MonsterAttack(g.RNG, m)
	g.Sound.Play("fight/monster")
	if dmg == 0 {
		b.log.Add("%s %s at %s with %s. %s %s it.",
			m.Name, verb, tgt.Name, with, tgt.Armor.Name, tgt.Armor.Verb)
		return
	}
	tgt.HP = core.Max(0, tgt.HP-dmg)
	g.Sound.Play("fight/hurt")
	b.partyHurt[tgt] = 14
	b.cam.Shake(float64(dmg) / 6)
	fx, fy := b.memberFloat(tgt)
	b.addFloater(fx, fy, fmt.Sprintf("-%d", dmg), render.ColBlood)
	b.log.AddColor(render.ColBlood, "%s %s %s with %s for %d.",
		m.Name, verb, tgt.Name, with, dmg)

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
	b.addFloater(monSlotX(idx, len(b.mons)), 88, fmt.Sprintf("-%d", dmg), render.ColGold)
	if m.HP == 0 && !m.Dead {
		m.Dead = true
		g.Sound.Play("fight/die")
		g.noteQuestProgress(g.Quests.OnMonsterKilled(m.Def.ID))
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
	// depends on whether anybody is left to do something about it: with a
	// companion still standing you get carried to a town and charged for it,
	// and alone it is still the end. That is the whole argument for a party —
	// not that they hit things, but that somebody is there to pick you up.
	if g.Player.HP <= 0 {
		b.result = 2
		if len(b.livingParty()) > 0 {
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
	for _, m := range b.mons {
		if m.HP <= 0 { // a monster that fled has HP left and earns nothing
			killed = append(killed, m)
		}
	}
	if len(killed) == 0 {
		return
	}

	xp := rules.XPAward(killed)
	coins := rules.CoinAward(g.RNG, killed)

	// Experience is not divided. Every member banks the full award and levels
	// on their own curve, which is what keeps a hireling taken on at level 6
	// still useful at level 12 without any catch-up machinery — and it leaves
	// the hero's progression exactly where the balance pass tuned it, because
	// splitting XP would silently re-tune the whole curve.
	//
	// The party is paid for out of the purse instead: each companion skims a
	// standing cut of every haul, which is a cost you feel at the shop counter
	// rather than one that slows down levelling.
	var skimmed int64
	for _, c := range b.party {
		c.TotalXP += xp
		if c == p {
			c.SpendXP += xp
			continue
		}
		skimmed += rules.Skim(coins, c.Cut)
	}

	p.Coins += coins - skimmed
	g.Sound.Play("world/coins")
	if skimmed > 0 {
		b.log.AddColor(render.ColGold, "%d experience. %d coins, less %d in cuts.",
			xp, coins, skimmed)
	} else {
		b.log.AddColor(render.ColGold, "%d experience. %d coins.", xp, coins)
	}

	for _, m := range killed {
		for name, n := range rules.RollLoot(g.RNG, m.Def.Loot) {
			it, ok := g.Data.Item(name)
			if !ok {
				continue
			}
			it.Count = n
			p.AddItem(it)
			g.Sound.Play("world/loot")
			b.log.Add("Picked up %s x%d.", name, n)
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
		// They re-arm themselves out of the cut they have been taking, which is
		// what the cut is for. Without this a companion taken on early would
		// still be swinging a table leg twenty levels later, and the standing
		// charge on every haul would buy the player nothing at all.
		was := c.Weapon.Name
		g.Data.Equip(c)
		if c.Weapon.Name != was {
			b.log.AddColor(render.ColHeal, "%s is now level %d, and has spent their cut on a %s.",
				c.Name, c.Level, c.Weapon.Name)
			continue
		}
		b.log.AddColor(render.ColHeal, "%s is now level %d, and mentions it.", c.Name, c.Level)
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
		b.log.AddColor(render.ColBlood, "You die. Press Z.")
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
	}
	// Copy the battle transcript into the world log so it survives the pop.
	g.sinceFight = 0
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
	case 2:
		for len(g.stack) > 0 {
			g.Pop()
		}
		g.quit = false
		g.Push(newTitleScene(g))
	case 4:
		g.rescueToTown()
	}
}

// --- layout helpers -------------------------------------------------------

func monSlotX(i, n int) float64 {
	if n <= 0 {
		n = 1
	}
	return render.ScreenW / float64(n+1) * float64(i+1)
}

// Where the party panel sits, and what one member's row looks like inside it.
const (
	partyPanelX = 8.0
	partyPanelY = 206.0
	partyPanelW = 188.0
	partyPanelH = 58.0
)

// drawAllyCursor frames the party row the cursor is on. It borrows the gold
// frame the monster cursor uses, so "this is the thing you are about to act on"
// looks the same whichever side of the fight it is on.
func (b *battleScene) drawAllyCursor(g *Game, dst *ebiten.Image) {
	if b.allyPick < 0 || b.allyPick >= len(b.party) {
		return
	}
	// A solo hero never opens this cursor, so the row geometry is the one the
	// party panel uses rather than the large single-portrait layout.
	ry := partyPanelY + 4 + float64(b.allyPick)*partyRowH
	render.Frame(dst, partyPanelX+2, ry-1, partyPanelW-4, partyRowH, render.ColGold)
	if (g.Tick()/12)%2 == 0 {
		render.Text(dst, ">", partyPanelX-4, ry+1, render.ColGold)
	}
}

// memberFloat is where a damage or healing number pops for a party member: over
// their own row, so with three of them on screen it is never a guess who just
// took the hit. A solo hero keeps the original spot beside their portrait.
func (b *battleScene) memberFloat(c *model.Character) (float64, float64) {
	if len(b.party) <= 1 {
		return 66, 200
	}
	for i, m := range b.party {
		if m == c {
			return partyPanelX + 118, partyPanelY + 4 + float64(i)*partyRowH
		}
	}
	return 66, 200
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

	// Monsters across the top.
	slotW := render.ScreenW / float64(len(b.mons)+1)
	for i, m := range b.mons {
		cx := monSlotX(i, len(b.mons)) + ox
		top := 18.0 + oy
		boxW := core.ClampF(slotW-20, 56, 108)

		tint := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
		switch {
		case m.Dead:
			tint = color.RGBA{0x50, 0x40, 0x50, 0x90}
		case b.hurt[i] > 0 && (b.hurt[i]/3)%2 == 0:
			tint = color.RGBA{0xFF, 0x90, 0x90, 0xFF}
		}
		sprite := g.Assets.Get(m.Def.Sprite)
		render.ScreenFit(dst, sprite, 0, cx-boxW/2, top, boxW, 82, tint)

		// Name plate and health.
		nameCol := render.ColInk
		if m.Dead {
			nameCol = render.ColInkFaint
		}
		if b.mode == modeTarget && b.target == i && !m.Dead {
			render.Frame(dst, cx-boxW/2-2, top-2, boxW+4, 86, render.ColGold)
			if (g.Tick()/12)%2 == 0 {
				render.TextCenter(dst, "v", cx, top-13, render.ColGold)
			}
		}
		if !m.Dead {
			ui.Bar(dst, cx-boxW/2, top+84, boxW, 5, m.HPFrac(), render.ColBlood)
			drawEffectPips(dst, cx-boxW/2, top+90, m.Active)
		}
		// Names run long and slots are only a third of the screen, so the
		// plate is truncated rather than allowed to collide with its neighbour.
		render.TextCenter(dst, render.Trunc(m.Name, slotW-6), cx, top+94, nameCol)
	}

	// Transcript.
	ui.TitledPanel(dst, "", 8, 136, render.ScreenW-16, 64)
	b.log.Draw(dst, 16, 142, 4)

	// The company.
	g.drawPartyPanel(dst, partyPanelX, partyPanelY, partyPanelW, partyPanelH, b.partyHurt)
	if b.mode == modeAllyPick {
		b.drawAllyCursor(g, dst)
	}

	// Command panel.
	title := map[battleMode]string{
		modeRoot: "", modeSpell: "technique", modeItem: "pack",
		modeTarget: "target", modeAllyPick: "on whom", modeBusy: "", modeDone: "",
	}[b.mode]
	ui.TitledPanel(dst, title, 204, 206, render.ScreenW-212, 58)
	switch b.mode {
	case modeRoot, modeSpell, modeItem:
		b.menu.Draw(dst, 218, 212, render.ScreenW-238)
	case modeTarget:
		render.Text(dst, "Left / Right to choose.", 214, 218, render.ColInk)
		render.Text(dst, "Z commits. X reconsiders.", 214, 232, render.ColInkDim)
	case modeAllyPick:
		render.Text(dst, "Up / Down to choose.", 214, 218, render.ColInk)
		render.Text(dst, "Z commits. X reconsiders.", 214, 232, render.ColInkDim)
	case modeBusy:
		render.Text(dst, "...", 214, 224, render.ColInkDim)
	case modeDone:
		render.Text(dst, "Press Z.", 214, 224, render.ColGold)
	}

	for _, f := range b.floaters {
		alpha := uint8(core.Clamp(f.life*6, 0, 255))
		c := f.col
		c.A = alpha
		render.TextCenter(dst, f.text, f.x, f.y, c)
	}
}
