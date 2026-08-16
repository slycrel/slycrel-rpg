// Package content is the writing room. It recombines the word banks in
// data/text/flavor.json into names, signs, taunts, and combat narration.
//
// The rule the whole tone hangs on: the game never comments on its own joke.
// Lines are delivered flat, as though the world genuinely works this way. That
// is what separates "over-the-top" from "trying too hard".
package content

import (
	"fmt"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/core"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/rules"
)

// Writer produces flavor text. It satisfies world.Namer.
type Writer struct {
	t *gamedata.Text
}

// New wraps a loaded text table.
func New(t *gamedata.Text) *Writer { return &Writer{t: t} }

// PlaceName invents a name appropriate to a point-of-interest kind.
func (w *Writer) PlaceName(g *core.RNG, kind string) string {
	pre := core.Pick(g, w.t.PlacePrefix)
	suf := core.Pick(g, w.t.PlaceSuffix)

	// Three name shapes, so the map does not read as one formula repeated
	// forty times: compound, possessive, and "X of Y".
	switch g.Intn(10) {
	case 0, 1, 2, 3:
		return pre + strings.ToLower(suf)
	case 4, 5, 6:
		return fmt.Sprintf("%s %s", pre, kindNoun(g, kind))
	default:
		return fmt.Sprintf("%s of %s", kindNoun(g, kind), core.Pick(g, w.t.PlaceOf))
	}
}

// kindNoun returns the common noun for a location kind, with variants so a
// dungeon is sometimes a "delve" and sometimes a "hole".
func kindNoun(g *core.RNG, kind string) string {
	opts := map[string][]string{
		"capital": {"Crown", "Seat", "Hold"},
		"town":    {"Town", "Market", "Crossing"},
		"village": {"Village", "Hamlet", "Steading"},
		"castle":  {"Keep", "Bastion", "Hold"},
		"dungeon": {"Delve", "Deeps", "Warren"},
		"cave":    {"Hollow", "Grotto", "Hole"},
		"ruin":    {"Ruin", "Remnant", "Wreck"},
		"tower":   {"Spire", "Tower", "Finger"},
		"shrine":  {"Shrine", "Rest", "Font"},
		"camp":    {"Camp", "Muster", "Fire"},
		"oddity":  {"Situation", "Anomaly", "Business"},
	}[kind]
	if len(opts) == 0 {
		opts = []string{"Place"}
	}
	return core.Pick(g, opts)
}

// PlaceTag returns the one-line description shown when you approach.
func (w *Writer) PlaceTag(g *core.RNG, kind string) string {
	if lines := w.t.PoiTagline[kind]; len(lines) > 0 {
		return core.Pick(g, lines)
	}
	return "It is there. It is a place."
}

// Rumor returns a piece of hearsay about a kind of location.
func (w *Writer) Rumor(g *core.RNG, kind string) string {
	if lines := w.t.PoiRumor[kind]; len(lines) > 0 {
		return core.Pick(g, lines)
	}
	return core.Pick(g, w.t.NpcLine)
}

// PersonName invents a name for a townsperson.
func (w *Writer) PersonName(g *core.RNG) string {
	return core.Pick(g, w.t.GivenNames)
}

// HeroName invents a default name for the player, offered at creation.
func (w *Writer) HeroName(g *core.RNG) string {
	return core.Pick(g, w.t.GivenNames)
}

// Epithet returns the title appended to the hero's name.
func (w *Writer) Epithet(g *core.RNG) string {
	return core.Pick(g, w.t.Epithets)
}

// NPCLine returns something for a townsperson to say.
func (w *Writer) NPCLine(g *core.RNG) string { return core.Pick(g, w.t.NpcLine) }

// SignText returns signage, chest engravings, and altar inscriptions.
func (w *Writer) SignText(g *core.RNG) string { return core.Pick(g, w.t.SignText) }

// Idle returns ambient narration for standing around outdoors.
func (w *Writer) Idle(g *core.RNG) string { return core.Pick(g, w.t.IdleFlavor) }

// Hit narrates a landed blow. attacker and target are already-formatted names;
// verb comes from the weapon or the monster's attack table.
func (w *Writer) Hit(g *core.RNG, attacker, verb, target string, dmg int, crit bool) string {
	bank := w.t.HitFlavor
	if crit {
		bank = w.t.CritFlavor
	}
	tmpl := core.Pick(g, bank)
	// Templates use {A} attacker, {V} verb, {T} target, {D} damage.
	r := strings.NewReplacer("{A}", attacker, "{V}", verb, "{T}", target, "{D}", fmt.Sprint(dmg))
	return r.Replace(tmpl)
}

// Miss narrates a whiff.
func (w *Writer) Miss(g *core.RNG, attacker, target string) string {
	r := strings.NewReplacer("{A}", attacker, "{T}", target)
	return r.Replace(core.Pick(g, w.t.MissFlavor))
}

// Death narrates a monster's exit, preferring the monster's own death lines.
func (w *Writer) Death(g *core.RNG, m *model.Monster) string {
	if len(m.Def.Death) > 0 {
		return strings.ReplaceAll(core.Pick(g, m.Def.Death), "{T}", m.Name)
	}
	return strings.ReplaceAll(core.Pick(g, w.t.DeathFlavor), "{T}", m.Name)
}

// Taunt returns a monster's opening line, if it has one.
func (w *Writer) Taunt(g *core.RNG, m *model.Monster) string {
	if len(m.Def.Taunt) == 0 {
		return ""
	}
	return strings.ReplaceAll(core.Pick(g, m.Def.Taunt), "{T}", m.Name)
}

// MonsterAttack builds the "the X verbs you with its Y" line for a monster.
func (w *Writer) MonsterAttack(g *core.RNG, m *model.Monster) (verb, with string) {
	verb, with = "hits", "everything it has"
	if len(m.Def.AttackVerb) > 0 {
		verb = core.Pick(g, m.Def.AttackVerb)
	}
	if len(m.Def.AttackWith) > 0 {
		with = core.Pick(g, m.Def.AttackWith)
	}
	return verb, with
}

// LevelUpLine congratulates the player, insincerely.
func (w *Writer) LevelUpLine(g *core.RNG, level int) string {
	return strings.ReplaceAll(core.Pick(g, w.t.LevelFlavor), "{L}", fmt.Sprint(level))
}

// QuestLine returns a template for a quest kind and part (ask / nag / thank).
// Unknown combinations fall back to something neutral rather than an empty
// speech bubble.
func (w *Writer) QuestLine(g *core.RNG, kind, part string) string {
	if byPart, ok := w.t.Quest[kind]; ok {
		if lines := byPart[part]; len(lines) > 0 {
			return core.Pick(g, lines)
		}
	}
	switch part {
	case "thank":
		return "\"Well. That's that, then.\""
	case "nag":
		return "\"Still outstanding, that.\""
	default:
		return "\"I need a thing done. You look like a thing-doer.\""
	}
}

// DispositionLine narrates the state of a fight, used between rounds so long
// battles have a shape rather than a stream of numbers.
func (w *Writer) DispositionLine(g *core.RNG, d rules.Disposition, target string) string {
	lines := map[rules.Disposition][]string{
		rules.DispBothStrong:       {"Everyone is fresh. Everyone is furious. This will take a while."},
		rules.DispMonHurtYouStrong: {"{T} is starting to reconsider its entire career."},
		rules.DispMonWeakYouStrong: {"{T} is held together by spite and one tendon."},
		rules.DispMonStrongYouHurt: {"You are leaking. {T} has noticed, and is delighted."},
		rules.DispBothHurt:         {"You are both breathing hard. Neither of you will mention it."},
		rules.DispMonWeakYouHurt:   {"You are both nearly finished. This is a race to fall over last."},
		rules.DispMonStrongYouWeak: {"This is going badly in the specific way you were warned about."},
		rules.DispMonHurtYouWeak:   {"One more mistake and the epitaph writes itself."},
		rules.DispBothWeak:         {"Two ruined things, still swinging. Beautiful, really."},
	}[d]
	if len(lines) == 0 {
		return ""
	}
	return strings.ReplaceAll(core.Pick(g, lines), "{T}", target)
}
