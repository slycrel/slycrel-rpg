package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
)

// Combat effects: the six-frame bursts that play where a blow lands.
//
// The battle screen said everything in words. A swing produced a line in the
// transcript and a portrait that flashed red for a few frames, which is enough
// to follow and not enough to watch — and a turn-based fight is almost entirely
// watching. The one thing a burst adds that the transcript cannot is *where*:
// with three monsters on screen, "Bosk hits the Spider" is a sentence you have
// to read, and a fireball landing on the middle portrait is not.
//
// Deliberately not a new mechanic. Nothing here is read by rules, nothing
// changes a number, and the entire system can be deleted without the game
// playing differently. It is the fight saying out loud what it was already
// doing quietly.

// vfxFrames is how many frames every sheet in the pack has, and vfxEvery is how
// long each one holds. Six frames at four ticks is four tenths of a second —
// short enough that two exchanges do not queue up behind each other, long
// enough to read as an event rather than a flicker.
const (
	vfxFrames = 6
	vfxEvery  = 4
)

// burst is one effect playing at a place on screen.
//
// Positions are screen coordinates rather than a monster index, because a burst
// outlives what it was aimed at: the whole point of an explosion on a corpse is
// that the corpse is a corpse now. Storing the index would mean re-deriving a
// slot for something that may have stopped being in the line-up.
type burst struct {
	key   string
	x, y  float64 // centre
	size  float64 // drawn width and height
	start int     // tick it began on
	// late puts the burst after the panels rather than before them.
	//
	// A monster's effect belongs under the transcript and the party panel,
	// because those carry numbers and nothing should cover a number. One landing
	// on a party member has the opposite problem: it plays *on* the party panel,
	// so drawing it in the same pass as the monsters' put it underneath the
	// thing it was aimed at and it was invisible.
	late bool
}

// done reports whether a burst has run out of frames.
func (b burst) done(tick int) bool {
	return (tick-b.start)/vfxEvery >= vfxFrames
}

// spellVFX is the effect a technique plays, by kind.
//
// A default per kind rather than per spell, with data able to override it (see
// model.Spell.VFX). Twenty-seven techniques would be twenty-seven authored art
// keys to keep in step with a manifest, and the ones that would actually differ
// from their kind are a handful — so the table carries the rule and the data
// carries the exceptions, which is the same split the rest of the content
// follows.
var spellVFX = map[model.SpellKind]string{
	// The explosion rather than the lightning, which was the first choice and
	// the wrong one: ElectricExplosion is four thin cyan strokes on a
	// transparent field with two blank frames at the front, so the commonest
	// technique in the game played nothing anybody could see. Checked off a
	// frame, which is the only way to check art.
	model.SpellDamage: "vfx/boom",
	model.SpellHeal:   "vfx/holy",
	model.SpellDrain:  "vfx/drain",
	model.SpellWeaken: "vfx/wind",
	model.SpellStun:   "vfx/shock",
	model.SpellBless:  "vfx/cross",
	model.SpellRevive: "vfx/wings",
	model.SpellPoison: "vfx/poison",
	model.SpellBurn:   "vfx/burn",
	model.SpellSap:    "vfx/drain",
	model.SpellPact:   "vfx/boom",
}

// vfxForSpell picks the art for a technique: whatever it names, or its kind's.
func vfxForSpell(s model.Spell) string {
	if s.VFX != "" {
		return s.VFX
	}
	if key, ok := spellVFX[s.Kind]; ok {
		return key
	}
	return "vfx/boom"
}

// slashKeys are the three weapon-swing sheets, cycled so a fight does not play
// the same picture nine times running.
var slashKeys = []string{"vfx/slash_a", "vfx/slash_b", "vfx/slash_c"}

// play queues a burst centred on a point.
func (b *battleScene) play(g *Game, key string, x, y, size float64, late bool) {
	if key == "" {
		return
	}
	b.bursts = append(b.bursts, burst{
		key: key, x: x, y: y, size: size, start: g.Tick(), late: late,
	})
}

// playOnMonster centres a burst on a monster's portrait slot.
//
// The slot is recomputed here rather than passed in because every caller that
// lands a hit already knows the index and none of them know the layout, and the
// layout is one line in one place.
func (b *battleScene) playOnMonster(g *Game, idx int, key string) {
	if idx < 0 || idx >= len(b.mons) {
		return
	}
	ox, oy := b.cam.Offset()
	// Creature in, slot out: a burst belongs over the place on the field the
	// player is looking at, and with staggering that is the queue rather than
	// the body.
	slot := b.slotOf(idx)
	cx, cy, _, _ := monSlot(slot, len(b.slots))
	b.play(g, key, cx+ox, cy+oy, 72, false)
}

// playOnAlly centres a burst on a party member's row in the panel.
//
// Smaller than a monster's, because a row is a portrait and three meters and a
// seventy-pixel explosion over it would cover the numbers the player is
// actually reading. Forty is enough to see and small enough to see past.
func (b *battleScene) playOnAlly(g *Game, c *model.Character, key string) {
	for i, m := range b.party {
		if m != c {
			continue
		}
		b.play(g, key, partyPanelX+24, partyRowY(i, len(b.party))+20, 40, true)
		return
	}
}

// drawBursts paints one pass of effects. Called twice a frame: once over the
// portraits and under the panels, once over everything.
func (b *battleScene) drawBursts(g *Game, dst *ebiten.Image, late bool) {
	for _, e := range b.bursts {
		if e.late != late || e.done(g.Tick()) {
			continue
		}
		sp := g.Assets.Get(e.key)
		if sp == nil || sp.Count() == 0 {
			continue
		}
		frame := (g.Tick() - e.start) / vfxEvery
		render.ScreenFit(dst, sp, frame, e.x-e.size/2, e.y-e.size/2, e.size, e.size, nil)
	}
}

// retireBursts drops the ones that have run out.
//
// In Update rather than in Draw, which is where it started. Draw runs once per
// displayed frame and Update once per tick, and they are not the same count —
// but more to the point, a draw that quietly rewrites the scene's state is a
// draw nobody can call twice, and this one is called twice.
func (b *battleScene) retireBursts(g *Game) {
	live := b.bursts[:0]
	for _, e := range b.bursts {
		if !e.done(g.Tick()) {
			live = append(live, e)
		}
	}
	b.bursts = live
}

// vfxKeys is every effect the game can play, for the audit.
//
// Built from the same tables the game reads rather than listed again, so a key
// that is added to one and not the other cannot exist. That is the rule the
// quest generator and the thread caster both follow — never name something that
// might not be there — and art is where it bites hardest, since a missing sheet
// is a magenta box in the middle of a fight.
func vfxKeys(spells []model.Spell) []string {
	seen := map[string]bool{}
	var out []string
	add := func(k string) {
		if k != "" && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, k := range slashKeys {
		add(k)
	}
	for _, k := range spellVFX {
		add(k)
	}
	add("vfx/boom")
	// The free bolt off a focus weapon, which belongs to no technique and so
	// appears in neither table above.
	add("vfx/bolt")
	for _, s := range spells {
		add(s.VFX)
	}
	return out
}

// nextSlash returns the next swing sheet in the cycle.
//
// A counter rather than a roll, and deliberately not off g.RNG. Drawing from
// the shared generator to decide something purely cosmetic would move every
// damage roll after it, so a build with effects and a build without would play
// the same seed differently — for a decision the player cannot see the result
// of. It is the same reason casting a backstory forks its own stream.
//
// Cycling also does the job better than rolling would: three sheets picked at
// random repeat about a third of the time, and the whole point of having three
// is that consecutive swings do not look identical.
func (b *battleScene) nextSlash() string {
	b.swings++
	return slashKeys[b.swings%len(slashKeys)]
}
