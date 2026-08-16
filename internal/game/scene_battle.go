package game

import (
	"fmt"
	"image/color"

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
	modeRoot   battleMode = iota // Attack / Technique / Item / Flee
	modeTarget                   // choosing which monster
	modeSpell                    // choosing a technique
	modeItem                     // choosing an item
	modeBusy                     // playing back queued actions
	modeDone                     // battle over, waiting for a key
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

	log  *ui.Log
	menu ui.Menu
	mode battleMode
	// back is the mode to return to when targeting is cancelled.
	back battleMode

	target      int
	pendingCast model.Spell
	pendingItem int

	queue []func(*Game)
	timer int

	cam      render.Camera
	floaters []floater
	hurt     []int // per-monster hit flash timer
	heroHurt int

	defending bool
	// weakened tracks temporary offense reductions, keyed by monster index.
	weakened []int
	stunned  []bool
	// buffs applied by items, cleared when the battle ends.
	buffStr, buffDex int

	result   int // 0 running, 1 victory, 2 defeat, 3 fled
	round    int
	introRun bool
}

func newBattleScene(g *Game, mons []*model.Monster, where string) *battleScene {
	b := &battleScene{
		under:    g.Top(),
		mons:     mons,
		where:    where,
		log:      ui.NewLog(60),
		hurt:     make([]int, len(mons)),
		weakened: make([]int, len(mons)),
		stunned:  make([]bool, len(mons)),
	}
	b.cam = render.Camera{}
	b.setRootMenu(g)

	names := mons[0].Name
	if len(mons) > 1 {
		names = fmt.Sprintf("%d of them", len(mons))
	}
	b.log.AddColor(render.ColGold, "Out of the %s: %s.", where, names)
	if t := g.Write.Taunt(g.RNG, mons[0]); t != "" {
		b.log.Add("%s", t)
	}
	return b
}

func (b *battleScene) setRootMenu(g *Game) {
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
	if b.heroHurt > 0 {
		b.heroHurt--
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
		if Confirm() || Cancel() {
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
	// Round over: check for an ending, otherwise hand control back.
	if b.checkEnd(g) {
		return
	}
	b.defending = false
	b.round++
	if b.round%3 == 0 {
		if i := b.firstLiving(); i >= 0 {
			d := rules.GetDisposition(g.Player.HPFrac(), b.mons[i].HPFrac())
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
				case core.DirRight:
					b.target = nextIn(l, b.target)
				}
			}
		default:
			switch d {
			case core.DirDown:
				b.menu.Move(1)
			case core.DirUp:
				b.menu.Move(-1)
			}
		}
	}

	if Cancel() {
		switch b.mode {
		case modeSpell, modeItem:
			b.setRootMenu(g)
		case modeTarget:
			if b.back == modeRoot {
				b.setRootMenu(g)
			} else {
				b.mode = b.back
			}
		}
		return
	}
	if !Confirm() {
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
	}
}

func (b *battleScene) chooseRoot(g *Game) {
	switch b.menu.Index {
	case 0: // Attack
		b.beginTargeting(modeRoot)
	case 1: // Technique
		spells := g.Data.SpellsFor(g.Player)
		items := make([]ui.MenuItem, 0, len(spells)+1)
		for _, s := range spells {
			items = append(items, ui.MenuItem{
				Label: s.Name, Detail: fmt.Sprintf("%d SP", s.Cost),
				Disabled: s.Cost > g.Player.Psyche, Data: s,
			})
		}
		if len(items) == 0 {
			return
		}
		b.menu.SetItems(items)
		b.menu.Visible = 4
		b.mode = modeSpell
	case 2: // Item
		items := make([]ui.MenuItem, 0, len(g.Player.Bag))
		for i, it := range g.Player.Bag {
			items = append(items, ui.MenuItem{
				Label: it.Name, Detail: fmt.Sprintf("x%d", it.Count),
				Disabled: it.Kind == model.ItemTrinket, Data: i,
			})
		}
		if len(items) == 0 {
			return
		}
		b.menu.SetItems(items)
		b.menu.Visible = 4
		b.mode = modeItem
	case 3: // Defend
		b.runRound(g, func(g *Game) {
			b.defending = true
			b.log.Add("%s sets their feet and waits for it.", g.Player.Name)
		})
	case 4: // Flee
		b.runRound(g, func(g *Game) { b.attemptFlee(g) })
	}
}

func (b *battleScene) chooseSpell(g *Game) {
	it, ok := b.menu.Selected()
	if !ok || it.Disabled {
		return
	}
	s := it.Data.(model.Spell)
	b.pendingCast = s
	if s.Target == model.TargetOne {
		b.beginTargeting(modeSpell)
		return
	}
	b.runRound(g, func(g *Game) { b.castSpell(g, s, -1) })
}

func (b *battleScene) chooseItem(g *Game) {
	it, ok := b.menu.Selected()
	if !ok || it.Disabled {
		return
	}
	idx := it.Data.(int)
	b.pendingItem = idx
	b.runRound(g, func(g *Game) { b.useItem(g, idx) })
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
		b.runRound(g, func(g *Game) { b.castSpell(g, s, tgt) })
		return
	}
	b.runRound(g, func(g *Game) { b.playerAttack(g, tgt) })
}

// runRound queues the player's action and every monster's response, ordered by
// initiative, then hands the scene over to the playback loop.
func (b *battleScene) runRound(g *Game, playerAction func(*Game)) {
	b.menu.Visible = 0
	fastest := 0
	for _, i := range b.living() {
		if s := b.mons[i].Def.Speed; s > fastest {
			fastest = s
		}
	}
	playerFirst := rules.Initiative(g.RNG, g.Player.Speed, fastest)

	var monSteps []func(*Game)
	for _, i := range b.living() {
		idx := i
		monSteps = append(monSteps, func(g *Game) { b.monsterTurn(g, idx) })
	}

	b.queue = b.queue[:0]
	if playerFirst {
		b.queue = append(b.queue, playerAction)
		b.queue = append(b.queue, monSteps...)
	} else {
		b.queue = append(b.queue, monSteps...)
		b.queue = append(b.queue, playerAction)
	}
	b.mode = modeBusy
	b.timer = 0
}

func (b *battleScene) playerAttack(g *Game, idx int) {
	if idx < 0 || idx >= len(b.mons) || b.mons[idx].Dead {
		if l := b.firstLiving(); l >= 0 {
			idx = l
		} else {
			return
		}
	}
	m := b.mons[idx]
	p := g.Player

	// Miss chance from the speed/dexterity gap, floored and capped so neither
	// side ever becomes untouchable.
	missChance := core.ClampF(0.06+float64(m.Def.Speed-p.Dexterity-b.buffDex)*0.012, 0.03, 0.32)
	if g.RNG.Chance(missChance) {
		b.log.Add("%s", g.Write.Miss(g.RNG, p.Name, m.Name))
		return
	}

	crit := g.RNG.Chance(0.07 + float64(p.Dexterity)/400)
	dmg := rules.PlayerDamage(g.RNG, p, m)
	dmg += b.buffStr
	if crit {
		dmg = dmg*3/2 + 2
	}
	b.damageMonster(g, idx, dmg)
	b.log.Add("%s", g.Write.Hit(g.RNG, p.Name, p.Weapon.Verb, m.Name, dmg, crit))
	if crit {
		b.cam.Shake(3)
	}
}

func (b *battleScene) castSpell(g *Game, s model.Spell, idx int) {
	p := g.Player
	if s.Cost > p.Psyche {
		b.log.Add("%s reaches for it and finds nothing there.", p.Name)
		return
	}
	p.Psyche -= s.Cost
	if s.Cast != "" {
		b.log.AddColor(render.ColMagic, "%s", fmt.Sprintf(s.Cast, p.Name))
	}

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
			b.addFloater(heroSlotX(), 200, fmt.Sprintf("+%d", healed), render.ColHeal)
		case model.SpellWeaken:
			b.weakened[i] += s.Power
			b.log.Add("%s hits noticeably softer now.", m.Name)
		case model.SpellStun:
			b.stunned[i] = true
			b.log.Add("%s loses track of the fight entirely.", m.Name)
		}
	}

	switch s.Kind {
	case model.SpellHeal:
		healed := p.Heal(rules.SpellDamage(g.RNG, p, s))
		b.log.AddColor(render.ColHeal, "%s closes up %d worth of damage.", p.Name, healed)
		b.addFloater(heroSlotX(), 200, fmt.Sprintf("+%d", healed), render.ColHeal)
	default:
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
	}
}

func (b *battleScene) useItem(g *Game, idx int) {
	p := g.Player
	it, ok := p.TakeItem(idx)
	if !ok {
		return
	}
	switch it.Kind {
	case model.ItemHeal:
		healed := p.Heal(it.Power)
		b.log.AddColor(render.ColHeal, "%s downs the %s. %d back.", p.Name, it.Name, healed)
		b.addFloater(heroSlotX(), 200, fmt.Sprintf("+%d", healed), render.ColHeal)
	case model.ItemPsyche:
		before := p.Psyche
		p.Psyche = core.Clamp(p.Psyche+it.Power, 0, p.MaxPsyche)
		b.log.AddColor(render.ColMagic, "%s recovers %d psyche.", p.Name, p.Psyche-before)
	case model.ItemBuff:
		if it.Name == "Suspicious Pollen" {
			b.buffDex += it.Power
			b.log.Add("%s feels quicker, and slightly wrong about it.", p.Name)
		} else {
			b.buffStr += it.Power
			b.log.Add("%s feels stronger and considerably angrier.", p.Name)
		}
	default:
		b.log.Add("%s waves the %s at the problem. It does not help.", p.Name, it.Name)
	}
}

func (b *battleScene) attemptFlee(g *Game) {
	fastest := 0
	for _, i := range b.living() {
		if s := b.mons[i].Def.Speed; s > fastest {
			fastest = s
		}
	}
	if g.RNG.Chance(rules.FleeChance(g.Player.Speed, fastest)) {
		b.result = 3
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
	if b.stunned[idx] {
		b.stunned[idx] = false
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

	dmg := rules.MonsterDamage(g.RNG, g.Player, m)
	dmg -= b.weakened[idx]
	if dmg < 0 {
		dmg = 0
	}
	if b.defending {
		dmg = rules.Defending(dmg)
	}

	verb, with := g.Write.MonsterAttack(g.RNG, m)
	if dmg == 0 {
		b.log.Add("%s %s at %s with %s. %s %s it.",
			m.Name, verb, g.Player.Name, with, g.Player.Armor.Name, g.Player.Armor.Verb)
		return
	}
	g.Player.HP = core.Max(0, g.Player.HP-dmg)
	b.heroHurt = 14
	b.cam.Shake(float64(dmg) / 6)
	b.addFloater(heroSlotX(), 200, fmt.Sprintf("-%d", dmg), render.ColBlood)
	b.log.AddColor(render.ColBlood, "%s %s %s with %s for %d.",
		m.Name, verb, g.Player.Name, with, dmg)
}

func (b *battleScene) damageMonster(g *Game, idx, dmg int) {
	m := b.mons[idx]
	m.HP = core.Max(0, m.HP-dmg)
	b.hurt[idx] = 12
	b.addFloater(monSlotX(idx, len(b.mons)), 88, fmt.Sprintf("-%d", dmg), render.ColGold)
	if m.HP == 0 && !m.Dead {
		m.Dead = true
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
	if g.Player.HP <= 0 {
		b.result = 2
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
	p.TotalXP += xp
	p.SpendXP += xp
	p.Coins += coins
	b.log.AddColor(render.ColGold, "%d experience. %d coins.", xp, coins)

	for _, m := range killed {
		for name, n := range rules.RollLoot(g.RNG, m.Def.Loot) {
			it, ok := g.Data.Item(name)
			if !ok {
				continue
			}
			it.Count = n
			p.AddItem(it)
			b.log.Add("Picked up %s x%d.", name, n)
		}
	}

	for rules.PendingLevels(p) > 0 {
		rules.LevelUp(g.RNG, p)
		b.log.AddColor(render.ColHeal, "%s", g.Write.LevelUpLine(g.RNG, p.Level))
	}
}

func (b *battleScene) finish(g *Game) {
	b.mode = modeDone
	switch b.result {
	case 1:
		b.log.AddColor(render.ColGold, "Victory. Press Z.")
	case 2:
		b.log.AddColor(render.ColBlood, "You die. Press Z.")
	case 3:
		b.log.AddColor(render.ColGold, "Escaped. Press Z.")
	}
	// Copy the battle transcript into the world log so it survives the pop.
	g.sinceFight = 0
}

// Pop handling: on defeat, drop the run and return to the title.
func (b *battleScene) onPopped(g *Game) {
	if b.result == 2 {
		for len(g.stack) > 0 {
			g.Pop()
		}
		g.quit = false
		g.Push(newTitleScene(g))
	}
}

// --- layout helpers -------------------------------------------------------

func monSlotX(i, n int) float64 {
	if n <= 0 {
		n = 1
	}
	return render.ScreenW / float64(n+1) * float64(i+1)
}

func heroSlotX() float64 { return 66 }

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
		}
		// Names run long and slots are only a third of the screen, so the
		// plate is truncated rather than allowed to collide with its neighbour.
		render.TextCenter(dst, render.Trunc(m.Name, slotW-6), cx, top+94, nameCol)
	}

	// Transcript.
	ui.TitledPanel(dst, "", 8, 136, render.ScreenW-16, 64)
	b.log.Draw(dst, 16, 142, 4)

	// Hero panel.
	ui.TitledPanel(dst, render.Trunc(g.Player.Name, 120), 8, 206, 188, 58)
	portrait := g.Assets.Get("portrait/male/m_01")
	tint := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	if b.heroHurt > 0 && (b.heroHurt/3)%2 == 0 {
		tint = color.RGBA{0xFF, 0x80, 0x80, 0xFF}
	}
	render.ScreenFit(dst, portrait, 0, 12, 210, 46, 46, tint)
	ui.StatBars(dst, 64, 212, 122, g.Player.HP, g.Player.MaxHP, g.Player.Psyche, g.Player.MaxPsyche)

	// Command panel.
	title := map[battleMode]string{
		modeRoot: "", modeSpell: "technique", modeItem: "pack",
		modeTarget: "target", modeBusy: "", modeDone: "",
	}[b.mode]
	ui.TitledPanel(dst, title, 204, 206, render.ScreenW-212, 58)
	switch b.mode {
	case modeRoot, modeSpell, modeItem:
		b.menu.Visible = 4
		b.menu.Draw(dst, 218, 212, render.ScreenW-238)
	case modeTarget:
		render.Text(dst, "Left / Right to choose.", 214, 218, render.ColInk)
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
