package game

import (
	"fmt"

	"github.com/slycrel/slycrel-rpg/internal/model"
	"github.com/slycrel/slycrel-rpg/internal/render"
	"github.com/slycrel/slycrel-rpg/internal/rules"
	"github.com/slycrel/slycrel-rpg/internal/sky"
)

// makeCamp spends a bedroll: half of both pools back for everybody, a chunk of
// the clock, and a roll on whether anything wanders in.
//
// What it deliberately does not do is the other three things an inn does. It
// does not fill the pools, it does not wake you at dawn, and it does not write
// a checkpoint. Those are what the bed is for, and they are why one is still
// worth paying for at level fourteen — the camp buys the walk, not the safety.
func (g *Game) makeCamp(it model.Item) {
	// Not in a town. There is a bed forty feet away and an innkeeper who would
	// like a word about the fire.
	if g.Local != nil && g.Local.POI.Kind.Settlement() {
		g.Sound.Play("ui/deny")
		g.Say("", "You could. There is a bed about forty feet away, and a landlord "+
			"who has already seen you looking at the floor.")
		return
	}

	indoors := g.Local != nil
	danger := 0
	weather := g.weatherAt(g.Walk.Tile)
	switch {
	case g.Local != nil:
		danger = g.Local.POI.Level
		weather = g.weatherHere()
	default:
		danger = g.World.RegionLevel(g.Walk.Tile)
	}
	prowl := sky.Prowl(g.Clock.Phase(), weather)

	disturbed := rules.CampDisturbed(g.RNG, prowl, danger, indoors, it.Power)
	share := 1.0
	if disturbed {
		share = rules.DisturbedShare
	}

	var hp, psyche int
	for _, c := range g.Party() {
		h, p := rules.MakeCamp(c, share)
		hp += h
		psyche += p
	}
	g.Clock.Tick(rules.CampSteps)
	g.Sound.Play("world/enter")

	back := fmt.Sprintf("%d hit points and %d psyche back across the company.", hp, psyche)
	if !disturbed {
		g.Log.AddColor(render.ColHeal, "You make camp. %s", back)
		g.Say("", fmt.Sprintf("%s\n\nYou get a fire going, eat something that was "+
			"technically food, and sleep in shifts nobody keeps to.\n\n%s",
			it.Name, back))
		return
	}

	// Something found you. The rest is mostly lost and the fight starts on the
	// spot — which is the whole reason the roll exists: where and when you lie
	// down is a decision rather than a formality.
	g.Log.AddColor(render.ColBlood, "Something walks into the camp.")
	g.SayThen("", fmt.Sprintf("%s\n\nYou are woken by the sound of something "+
		"that has already worked out how many of you there are.\n\n%s",
		it.Name, back), func(g *Game) {
		biome := g.World.At(g.Walk.Tile.X, g.Walk.Tile.Y).Biome()
		level := g.encounterLevel(g.Walk.Tile)
		if g.Local != nil {
			biome, level = g.Local.Biome, g.Local.POI.Level
		}
		mons := g.Data.PickMonsters(g.RNG, biome, level, g.encounterSize(1+g.RNG.Intn(2)))
		if len(mons) == 0 {
			return
		}
		g.sinceFight = 0
		g.Push(newBattleScene(g, mons, "the camp"))
	})
}
