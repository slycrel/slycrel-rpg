package rules

import "github.com/slycrel/slycrel-rpg/internal/model"

// Reputation is two numbers, not one, and that is the whole idea.
//
// Fame is what the deeds are worth. Renown is how well the face is known. They
// are earned by different things and they come apart, which is where the
// interesting states are:
//
//   - deeds known, face not — the story travels and the man in the room does
//     not. Nobody marks up their prices for you because nobody has placed you.
//   - face known, deeds thin — recognised everywhere and worth nothing once
//     anybody checks. Every counter in the realm has your measure.
//
// One bar cannot say either of those. It can only say "more" or "less", and a
// world that reacts to a single number reacts the same way to a hero and to a
// loudmouth with a good tailor.
//
// Shame is the other end of Fame rather than a third axis: deeds you would
// rather were not travelling.

// notable is where a number stops being noise. Fame arrives one per level and
// one per errand, so this is a handful of both — roughly the point where a
// player has done enough of something for a town to have an opinion about it.
const notable = 6

// Standing is how a town reads somebody.
type Standing int

const (
	// Unknown: nobody has heard of you, which is its own kind of useful.
	Unknown Standing = iota
	// Rumoured: the deeds travel and the face does not.
	Rumoured
	// Celebrated: both, and everyone has an opinion.
	Celebrated
	// Recognised: the face is everywhere and there is nothing behind it.
	Recognised
	// Notorious: what travels is the part you would rather did not.
	Notorious
)

// Read works out how the world takes a character.
//
// Shame is checked first and against the deeds rather than against a fixed
// number: a scoundrel with nothing to their name is a nobody, and a scoundrel
// with a string of finished errands is a complicated person. What makes
// somebody notorious is the balance tipping, not the total.
func Read(c *model.Character) Standing {
	if c == nil {
		return Unknown
	}
	deeds := c.Fame - c.Shame
	if c.Shame >= notable/2 && c.Shame > c.Fame {
		return Notorious
	}
	switch {
	case deeds >= notable && c.Renown >= notable:
		return Celebrated
	case deeds >= notable:
		return Rumoured
	case c.Renown >= notable:
		return Recognised
	}
	return Unknown
}

// Name is what to call a standing on a character sheet.
func (s Standing) Name() string {
	switch s {
	case Rumoured:
		return "a rumour"
	case Celebrated:
		return "a name"
	case Recognised:
		return "a face"
	case Notorious:
		return "notorious"
	}
	return "nobody"
}

// Key is the data-table name for a standing, for looking up what a townsperson
// says to somebody like you.
func (s Standing) Key() string {
	switch s {
	case Rumoured:
		return "rumoured"
	case Celebrated:
		return "celebrated"
	case Recognised:
		return "recognised"
	case Notorious:
		return "notorious"
	}
	return ""
}

// Sheet is the one line a character sheet gets, naming the corner and saying
// what it means in the same breath.
//
// Name and Note used to be two separate things on that screen — a row reading
// "They call you: a rumour" and a faint line underneath explaining it — which
// is two lines to say one thing on a panel that had run out of room for the
// spent ledger. Written together they are shorter than either arrangement and
// read better, because the name is the subject of the sentence rather than the
// answer to a form field.
//
// Kept under about forty characters: the sheet gives this line 230 pixels, and
// a line that has to be truncated stops at the interesting half.
func (s Standing) Sheet() string {
	switch s {
	case Rumoured:
		return "A rumour: the stories travel, not you."
	case Celebrated:
		return "A name, and known for something."
	case Recognised:
		return "A face. Nobody says what for."
	case Notorious:
		return "Notorious. The wrong half travels."
	}
	return "Nobody has heard of you. Yet."
}

// PriceMultiplier is what a shopkeeper charges you, relative to the sticker.
//
// It reads Renown rather than Fame, which is the point of splitting them. A
// counter marks up the person it recognises, and recognising somebody is not
// the same as thinking well of them: the loudmouth pays most, the legend
// nobody has placed pays the sticker price, and the celebrity pays for the
// privilege of being one.
func (s Standing) PriceMultiplier() float64 {
	switch s {
	case Recognised:
		return 1.15
	case Celebrated:
		return 1.10
	case Notorious:
		return 1.25
	}
	return 1
}

// HireMultiplier is what somebody outside an inn asks to come along.
//
// The mirror of prices, and deliberately so: what a shopkeeper wants from you
// and what a mercenary wants from you are different questions with different
// answers, so a standing that costs at one counter can pay at the other.
func (s Standing) HireMultiplier() float64 {
	switch s {
	case Celebrated:
		return 0.80 // people would like to have been there
	case Rumoured:
		return 0.90
	case Notorious:
		return 1.30 // hazard pay
	}
	return 1
}
