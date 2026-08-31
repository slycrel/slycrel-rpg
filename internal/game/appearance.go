package game

import (
	"fmt"
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/world"
)

// What a hero looks like, which is now the player's business rather than a
// consequence of the class they picked.
//
// Both of these are lists of art keys, which is why they live in internal/game
// beside the audit rather than in a domain package: nothing outside the drawing
// half of the program has an opinion about which portrait is which. The
// character carries the answers as model.Sprite and model.Portrait, both of
// which already existed as overrides — hireling recruits have been setting them
// since the party landed — so the hero picking them needed no new state.

// defaultPortrait is the face a character with no portrait of their own gets.
//
// Named here rather than written out at each of the three places that need it —
// portraitOf's fallback, heroFaces' empty case, and the audit — because they
// have to agree. A fallback the audit does not check is a fallback that can go
// missing without anything saying so.
const defaultPortrait = "portrait/male/m_01"

// heroLook is one walk sheet, with the word for it.
type heroLook struct {
	Key  string // sprite prefix, e.g. "hero/thief"; heroSpriteKey appends the pose
	Name string
}

// heroLooks are the four sheets in the game, named for what the art is rather
// than for the class that used to be issued it.
//
// Decoupling the two is the point. A fighter who walks around in robes is a
// perfectly good fighter, and having the world sprite follow the class meant
// three of the four sheets were unreachable to a player who wanted the class
// the fourth one came with.
var heroLooks = []heroLook{
	{"hero/fighter", "viking"},
	{"hero/thief", "rogue"},
	{"hero/mage", "blood mage"},
	{"hero/druid", "druid"},
}

// heroFaces returns every portrait a hero may wear, in a stable order.
//
// Probed against the registry rather than listed, because a hard-coded roster
// of 68 art keys is 68 chances to name something that is not there — and the
// manifest is generated from whatever was extracted, so it moves. Anything
// missing is simply skipped, which means a portrait pack going away shortens
// the list instead of putting a magenta box in the middle of it.
//
// The numbered sheets only. The pool also holds a handful of oddly-named
// leftovers and five cultists, which are fine for a hireling drawn at random
// and wrong as the first thing a new player is asked to choose between.
func (g *Game) heroFaces() []string {
	if g.faces != nil {
		return g.faces
	}
	var out []string
	// A game with no registry is a headless one — the tests build Games without
	// assets on purpose, since opening a window is what internal/game costs.
	// It falls through to the single known-good key below.
	if g.Assets != nil {
		add := func(format string, n int) {
			for i := 1; i <= n; i++ {
				if key := fmt.Sprintf(format, i); g.Assets.Has(key) {
					out = append(out, key)
				}
			}
		}
		add("portrait/male/m_%02d", 43)
		add("portrait/female/f_%02d", 26)
	}
	if len(out) == 0 {
		// The one key the audit already insists on. A creation screen with an
		// empty face list would be a screen with nothing to draw and nothing to
		// pick, which is worse than a screen with one face on it.
		out = []string{defaultPortrait}
	}
	g.faces = out
	return out
}

// faceFor picks the portrait a named person wears, and picks the same one every
// time.
//
// Residents are not stored — a town's people are regenerated from the world
// seed whenever you walk into it — so a face cannot be assigned once and kept.
// It has to be a function of something stable, and the name is the only thing
// about a townsperson that is both stable and theirs. Hashing it means the
// baker is the same baker on your third visit, which is the whole point of
// giving them a face at all: a portrait that changed between conversations
// would make the town feel less peopled rather than more.
//
// Deliberately not saved and deliberately not in world generation. The save
// format is seed plus deltas and this derives from data already in it.
func (g *Game) faceFor(name string) string {
	faces := g.heroFaces()
	if len(faces) == 0 {
		return defaultPortrait
	}
	// FNV-1a, written out rather than imported: it is four lines and the point
	// is that it never changes, since changing it re-faces every NPC in every
	// town at once.
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return faces[h%uint32(len(faces))]
}

// roleOf is the word under a person's face: what they are, in the player's
// terms rather than the code's.
//
// One word where possible. The name is already the panel's title and the line
// is already in their mouth; this is only there so a player can place somebody
// at a glance — which is the difference between "a stranger asked me to kill
// six gulls" and "the innkeeper asked me to kill six gulls".
//
// An empty answer is fine and common. A townsperson with nothing particular
// about them gets no caption rather than a filler one, because "villager" under
// every second face is noise that teaches the player to stop reading it.
func roleOf(e *world.Entity) string {
	if e == nil {
		return ""
	}
	switch e.Kind {
	case world.EShop:
		return shopRole[e.Shop]
	case world.EInn:
		return "innkeeper"
	case world.ERecruit:
		// What they do, and what they are when that is worth saying. A
		// half-ogre sellsword is a more useful thing to have read than either
		// half on its own.
		switch {
		case e.Blood != "" && e.Class != "":
			return e.Blood + " " + strings.ToLower(e.Class)
		case e.Class != "":
			return strings.ToLower(e.Class)
		case e.Blood != "":
			return e.Blood
		}
		return "for hire"
	}
	return ""
}

// shopRole names the counter someone stands behind. Taken from the same list
// the settlement builder signs the doors with, so the word over the shop and
// the word under the face are the same word.
var shopRole = map[world.ShopKind]string{
	world.ShopSmith:      "blacksmith",
	world.ShopArmorer:    "armourer",
	world.ShopApothecary: "apothecary",
	world.ShopInn:        "innkeeper",
}
