package model

// Lineage is what a strain of non-human ancestry does to a person.
//
// Hirelings are the only characters that have one. Nobody at the guild will
// take a part-troll, which is exactly why one is standing outside an inn asking
// for less money than the job is worth, and why the game has somewhere to put
// abilities no player class can learn.
//
// Every entry gives with one hand and takes with the other. That is the whole
// balance argument: a lineage is a different shape of companion, not a better
// one, so hiring one is a choice about what you are short of rather than a
// straightforward upgrade.
type Lineage struct {
	Kind MonsterKind
	// Tag is how a sheet describes it: "part troll".
	Tag string
	// Note is the one line the character sheet adds under the tag.
	Note string

	// HPPct shifts maximum hit points by a percentage, because a flat bonus
	// that mattered at level one would be a rounding error at level fifteen.
	HPPct int
	// The rest are flat, since the stats themselves grow slowly.
	Strength, Dexterity, Speed, Psyche int

	// Discount is the percentage off the hiring fee. Nobody else is bidding.
	Discount int
}

// Lineages is the roster a hireling can be rolled from. Humanoid and construct
// are deliberately absent: "part human" is not a lineage, and a part-construct
// raises questions about parentage that the game is not going to answer.
var Lineages = []Lineage{
	{
		Kind: KindBeast, Tag: "part beast", Discount: 20,
		Note:     "Sheds. Hears things first.",
		Speed:    2,
		Strength: 1,
		Psyche:   -1,
	},
	{
		Kind: KindFey, Tag: "part fey", Discount: 15,
		Note:      "Cannot lie. Has found ways round it.",
		HPPct:     -10,
		Dexterity: 2,
		Psyche:    2,
	},
	{
		Kind: KindUndead, Tag: "part undead", Discount: 30,
		Note:  "Still under contract, technically.",
		HPPct: 15,
		Speed: -2,
	},
	{
		Kind: KindDemon, Tag: "part demon", Discount: 25,
		Note:      "There is an arrangement.",
		Strength:  3,
		Dexterity: -2,
	},
	{
		Kind: KindOoze, Tag: "part ooze", Discount: 35,
		Note:      "Forgiving. Ruins upholstery.",
		HPPct:     20,
		Dexterity: -2,
		Speed:     -1,
	},
	{
		Kind: KindAberrant, Tag: "part something", Discount: 25,
		Note:     "Nobody has established what.",
		Psyche:   3,
		Strength: -1,
	},
}

// LineageOf returns the descriptor for a strain of ancestry.
func LineageOf(k MonsterKind) (Lineage, bool) {
	for _, l := range Lineages {
		if l.Kind == k {
			return l, true
		}
	}
	return Lineage{}, false
}

// BloodTag is the short description for a character sheet, empty for the
// ordinary people who make up most of the roster.
func (c *Character) BloodTag() string {
	if l, ok := LineageOf(c.Blood); ok {
		return l.Tag
	}
	return ""
}
