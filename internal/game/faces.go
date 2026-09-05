package game

import (
	"strings"

	"github.com/slycrel/slycrel-rpg/internal/quest"
	"github.com/slycrel/slycrel-rpg/internal/world"
)

// Who gets which face.
//
// A portrait used to be one of two things: the hero's, chosen at creation, or a
// hireling's, drawn at random from a pool of ten. Everybody else had no face at
// all, so the question never came up. Now that a conversation shows one it does,
// and "any of the seventy-six" is the wrong answer — the pack is mostly
// adventurers, and hashing a name across all of it puts a helmeted knight
// behind the apothecary's counter.
//
// So faces are pooled by what somebody is. Three rules hold the whole thing
// together:
//
//   - **A face belongs to a person, not to a conversation.** It is hashed from
//     the name, so the baker is the same baker on your third visit. Anything
//     that varies per conversation would make a town feel less peopled rather
//     than more, which is the opposite of the point.
//   - **The pool is chosen by something that cannot change about them.** A
//     vendor's counter, a recruit's trade, and — see questFaceKind — the sort of
//     errand a person has, all of which are fixed the moment the town is
//     generated.
//   - **Nothing is saved.** Towns are regenerated from the world seed, so every
//     input here is already in the save file.
//
// The cultist portraits are in no pool at all. They have hoods and, in one case,
// glowing eyes, and a cultist behind the counter is the game telling a joke
// about its own asset pack.

// facePool is a set of portrait keys somebody may wear.
type facePool []string

// Adventurers: armoured, helmeted, hooded for travel. Wrong for anybody who
// stands behind a counter and right for the only people in this game who
// carry a sword for money.
//
// This is the pool that matters most. A hireling's face follows them out of
// the hiring conversation into the party panel and every battle after it, so
// it is the portrait the player looks at for hours.
var faceRecruit = facePool{
	"portrait/male/m_05", "portrait/male/m_07", "portrait/male/m_14",
	"portrait/male/m_23", "portrait/male/m_24", "portrait/male/m_25",
	"portrait/male/m_27", "portrait/male/m_31", "portrait/male/m_35",
	"portrait/male/m_36", "portrait/female/f_10", "portrait/female/f_20",
}

// The counters. Each is chosen for the trade rather than at random: a smith is
// built like one, an apothecary looks like they know something you do not, and
// an innkeeper looks pleased you came in.
var (
	faceSmith = facePool{
		"portrait/male/m_09", "portrait/male/m_19", "portrait/male/m_20",
		"portrait/male/m_38", "portrait/male/m_43", "portrait/female/f_05",
		"portrait/female/f_12",
	}
	faceArmourer = facePool{
		"portrait/male/m_02", "portrait/male/m_22", "portrait/male/m_37",
		"portrait/male/m_41", "portrait/female/f_17", "portrait/female/f_25",
	}
	faceApothecary = facePool{
		"portrait/male/m_04", "portrait/male/m_10", "portrait/male/m_11",
		"portrait/male/m_16", "portrait/female/f_03", "portrait/female/f_06",
		"portrait/female/f_26",
	}
	faceInnkeeper = facePool{
		"portrait/male/m_13", "portrait/male/m_21", "portrait/male/m_30",
		"portrait/male/m_34", "portrait/female/f_09", "portrait/female/f_13",
		"portrait/female/f_22",
	}
)

// vendorFaces maps a counter to its pool.
var vendorFaces = map[world.ShopKind]facePool{
	world.ShopSmith:      faceSmith,
	world.ShopArmorer:    faceArmourer,
	world.ShopApothecary: faceApothecary,
	world.ShopInn:        faceInnkeeper,
}

// The errand-givers, by what they want, because what somebody wants is the
// most interesting thing about them at the moment they ask.
//
// A cull is asked for by somebody it has happened to; a fetch by somebody with
// a use for the thing; a delve by somebody frightened enough to send a stranger
// into a hole; a delivery by somebody with business elsewhere. The pools lean
// that way rather than being decorative: the face is doing the work the caption
// under it deliberately does not, since "villager" over every second person is
// noise.
var questFaces = map[quest.Kind]facePool{
	quest.Cull: {
		"portrait/male/m_19", "portrait/male/m_41", "portrait/male/m_43",
		"portrait/female/f_05", "portrait/female/f_07",
	},
	quest.Fetch: {
		"portrait/male/m_12", "portrait/male/m_15", "portrait/male/m_28",
		"portrait/female/f_15", "portrait/female/f_22",
	},
	// Hoods, for the ones who will not be coming with you.
	quest.Delve: {
		"portrait/male/m_08", "portrait/male/m_17", "portrait/male/m_26",
		"portrait/female/f_01", "portrait/female/f_04",
	},
	quest.Deliver: {
		"portrait/male/m_03", "portrait/male/m_10", "portrait/male/m_16",
		"portrait/female/f_10", "portrait/female/f_16",
	},
}

// faceTown is everybody else: the people who live here and have a line about
// the weather.
//
// Plain faces on purpose. This is the largest pool because most people in a
// town are not selling you anything, and it is the one an oddity's residents
// draw from as well — the joke there is that nobody in the frame is in on it,
// and a resident who looked like they knew would be somebody in on it.
var faceTown = facePool{
	"portrait/male/m_01", "portrait/male/m_02", "portrait/male/m_03",
	"portrait/male/m_12", "portrait/male/m_13", "portrait/male/m_15",
	"portrait/male/m_19", "portrait/male/m_20", "portrait/male/m_21",
	"portrait/male/m_22", "portrait/male/m_28", "portrait/male/m_29",
	"portrait/male/m_30", "portrait/male/m_33", "portrait/male/m_34",
	"portrait/male/m_37", "portrait/male/m_38", "portrait/male/m_39",
	"portrait/male/m_40", "portrait/male/m_41", "portrait/male/m_43",
	"portrait/female/f_02", "portrait/female/f_05", "portrait/female/f_07",
	"portrait/female/f_08", "portrait/female/f_09", "portrait/female/f_11",
	"portrait/female/f_12", "portrait/female/f_13", "portrait/female/f_14",
	"portrait/female/f_15", "portrait/female/f_16", "portrait/female/f_19",
	"portrait/female/f_21", "portrait/female/f_22", "portrait/female/f_23",
	"portrait/female/f_24", "portrait/female/f_25",
}

// pick returns the face somebody with this name wears, out of this pool.
//
// FNV-1a, written out rather than imported: it is four lines, and the point of
// it is that it never changes. Changing the hash re-faces every person in every
// town at once.
func (p facePool) pick(name string) string {
	if len(p) == 0 {
		return defaultPortrait
	}
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return p[h%uint32(len(p))]
}

// faceOf is the face for whoever is speaking.
//
// The pool is decided here and only here, so the offer, the nag and the turn-in
// cannot disagree about what somebody looks like — which is exactly how the
// hireling bug worked, where the person you agreed terms with was not the
// person who joined.
func (g *Game) faceOf(e *world.Entity) string {
	if e == nil {
		return defaultPortrait
	}
	switch e.Kind {
	case world.EShop:
		if pool, ok := vendorFaces[e.Shop]; ok {
			return g.have(pool.pick(e.Name))
		}
	case world.EInn:
		return g.have(faceInnkeeper.pick(e.Name))
	case world.ERecruit:
		return g.have(faceRecruit.pick(e.Name))
	case world.ENPC:
		// Somebody with an errand looks like somebody with that errand, and
		// keeps looking like it before, during and after you run it.
		if k, ok := g.questFaceKind(e); ok {
			if pool, ok := questFaces[k]; ok {
				return g.have(pool.pick(e.Name))
			}
		}
	}
	return g.have(faceTown.pick(e.Name))
}

// have falls back when a pool names a portrait this build does not have.
//
// The manifest is generated from whatever was extracted, so it moves. A pool is
// a hand-written list of keys and every one of them is a chance to name
// something that is not there; the audit reports it, and in the meantime a
// known-good face is better than a magenta box in the middle of a conversation.
func (g *Game) have(key string) string {
	if g.Assets != nil && !g.Assets.Has(key) {
		return defaultPortrait
	}
	return key
}

// faceKeys is every portrait any pool can produce, for the audit.
func faceKeys() []string {
	var out []string
	add := func(p facePool) { out = append(out, p...) }
	add(faceRecruit)
	add(faceTown)
	for _, p := range vendorFaces {
		add(p)
	}
	for _, p := range questFaces {
		add(p)
	}
	return out
}

// questFaceKind is the sort of errand a person has, decided when the town was
// generated rather than when you talk to them.
//
// wantsToAsk already works this way — whether somebody has an errand is a hash
// of where they are standing and the settlement's seed — and this is the same
// trick applied one step further, to what the errand is. Without it the kind
// comes off the run RNG at the moment of the first conversation, which would
// mean a face that appears when they hand you a job and disappears when you
// finish it.
//
// It is what the face uses whether or not a quest currently exists, and
// deliberately not read back off the quest. A settlement that cannot support
// the preferred kind produces a different one, and in that case a face that
// stays put is worth more than a face that matches the job.
func (g *Game) questFaceKind(e *world.Entity) (quest.Kind, bool) {
	if g.Local == nil || g.Local.POI == nil {
		return "", false
	}
	if !g.wantsToAsk(e, g.currentPOIIndex()) {
		return "", false
	}
	kinds := []quest.Kind{quest.Cull, quest.Fetch, quest.Delve, quest.Deliver}
	i := int(personHash(e.Name, g.Local.POI.Seed, 0x4A5C) * float64(len(kinds)))
	if i >= len(kinds) {
		i = len(kinds) - 1
	}
	return kinds[i], true
}

// captionPool is the set of things somebody of this sort might be captioned as.
//
// A pool rather than one string, and picked by the same name hash the faces
// use, so two smiths in two towns are captioned differently and the same smith
// is captioned the same way every time you walk in. Voice and stability at once:
// the caption is a fact about a person, not a line the game says about a job.
type captionPool []string

func (p captionPool) pick(name string) string {
	if len(p) == 0 {
		return ""
	}
	var h uint32 = 2166136261
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	// Salted differently from the face hash. Sharing it would tie caption to
	// portrait, so every smith with that face would carry that caption and the
	// two pools would collapse into one list of pairs.
	h ^= 0x5CA1
	h *= 16777619
	return p[h%uint32(len(p))]
}

// roleOf is the caption under a person's face: what they are, in the player's
// terms rather than the code's.
//
// Short by construction — the caption sits under a 76px portrait and wraps to
// two lines of about twelve characters, so anything over twenty-two characters
// does not fit. The name is already the panel's title and the line is already
// in their mouth; this only exists so a player can place somebody at a glance.
//
// Empty is fine and common. An ordinary townsperson gets no caption rather than
// a filler one, because "villager" under every second face is noise that
// teaches the player to stop reading it — and the face is already saying what
// they are, since the pools are chosen by what somebody is.
func (g *Game) roleOf(e *world.Entity) string {
	if e == nil {
		return ""
	}
	switch e.Kind {
	case world.EShop:
		return vendorCaptions[e.Shop].pick(e.Name)
	case world.EInn:
		return capInnkeeper.pick(e.Name)
	case world.ERecruit:
		// Derived rather than written, and deliberately so: blood and class
		// decide what this person may wield and wear, so the caption is load
		// bearing here in a way it is nowhere else. A joke in this slot would
		// cost the player the one fact they need before paying.
		switch {
		case e.Blood != "" && e.Class != "":
			return e.Blood + " " + strings.ToLower(e.Class)
		case e.Class != "":
			return strings.ToLower(e.Class)
		case e.Blood != "":
			return e.Blood
		}
		return "for hire"
	case world.EResident:
		return capResident.pick(e.Name)
	case world.ENPC:
		if k, ok := g.questFaceKind(e); ok {
			return questCaptions[k].pick(e.Name)
		}
	}
	return ""
}

// Somebody in their own house, which is the one thing the caption has to say.
//
// A player who has just opened a stranger's door should be told they are
// looking at the person who lives here rather than at another townsperson who
// happens to be standing indoors — that is the difference between a room and a
// street with a roof on it. The trade goes first, as everywhere: "at home"
// leads and the rest rides on it.
var capResident = captionPool{
	"at home, mid-sentence",
	"at home, was not expecting anyone",
	"at home, hand still on the door",
	"at home, something on the stove",
	"at home, wearing the indoor face",
	"at home, and it shows",
}

// The counters.
//
// Six each, drawn from three passes at the voice — flat occupational, quiet
// grievance, and something very slightly wrong that nobody in the world finds
// remarkable. Mixed on purpose: a town where every caption is the same joke is
// a town telling one joke six times.
//
// The rule they all follow is the game's: never comment on the joke, and never
// let it eat the information. A player who cannot tell the smith from the
// innkeeper has lost something the plain word gave them, so the trade is always
// the first thing said and the rest rides on top of it.
var (
	capSmith = captionPool{
		"smith, no eyebrows",
		"smith, deaf in one ear",
		"smith, never burns",
		"smith, apprentice fled",
		"smith, no windows",
		"smith, counts fingers",
	}
	capArmourer = captionPool{
		"armourer, dents show",
		"armourer, own dents",
		"armourer, sizes by eye",
		"armourer, straps fray",
		"armourer, no returns",
		"armourer, one good ear",
	}
	capApothecary = captionPool{
		"apothecary, don't ask",
		"apothecary, no labels",
		"apothecary, sells both",
		"apothecary, went sour",
		"apothecary, tastes it",
		"apothecary, guesses",
	}
	capInnkeeper = captionPool{
		"innkeeper, watered ale",
		"innkeeper, extra rooms",
		"innkeeper, keeps a tab",
		"innkeeper, rooms damp",
		"innkeeper, spare key",
		"innkeeper, no candles",
	}
)

var vendorCaptions = map[world.ShopKind]captionPool{
	world.ShopSmith:      capSmith,
	world.ShopArmorer:    capArmourer,
	world.ShopApothecary: capApothecary,
	world.ShopInn:        capInnkeeper,
}

// The errand-givers, who had no caption at all before this: there was nothing
// true to derive, since "villager" is not what somebody with a problem is.
//
// Each names a trade the errand makes sense coming from, so the caption is
// doing two jobs — placing the person, and saying why this is their problem.
// A cull comes from somebody it has happened to; a delve from somebody who is
// emphatically not coming with you.
var questCaptions = map[quest.Kind]captionPool{
	quest.Cull: {
		"farmer, losing sheep",
		"widow, took the flock",
		"cooper, taking names",
		"herder, it's personal",
		"miller, rats again",
		"beekeeper, remembers",
	},
	quest.Fetch: {
		"witch, won't explain",
		"alchemist, buys scales",
		"trader, has a use",
		"scholar, needs proof",
		"trapper, no questions",
		"chandler, keeps count",
	},
	quest.Delve: {
		"elder, won't go down",
		"priest, won't enter",
		"surveyor, drew the map",
		"clerk, not paid enough",
		"warden, minds the gate",
		"monk, prefers daylight",
	},
	quest.Deliver: {
		"trader, bad knee",
		"courier, overbooked",
		"merchant, closing shop",
		"widow, can't travel",
		"clerk, already leaving",
		"steward, leaves soon",
	},
}
