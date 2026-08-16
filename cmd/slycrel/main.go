// Command slycrel runs the game.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/slycrel/slycrel-rpg/internal/game"
	"github.com/slycrel/slycrel-rpg/internal/gamedata"
	"github.com/slycrel/slycrel-rpg/internal/render"
)

func main() {
	seed := flag.Int64("seed", 0, "world seed; 0 picks one from the clock")
	scale := flag.Int("scale", render.Scale, "integer window scale")
	fullscreen := flag.Bool("fullscreen", false, "start fullscreen")
	audit := flag.Bool("audit", false, "report which asset keys are falling back to placeholders, then exit")
	keylog := flag.Bool("keylog", false, "trace every key the engine reports, to stderr")
	demo := flag.Bool("demo", false, "run a scripted tour, writing a frame per screen to shots/, then exit")
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano() % (1 << 40)
	}

	root, err := gamedata.FindRoot()
	if err != nil {
		log.Fatalf("slycrel: %v", err)
	}

	g, err := game.New(root, *seed)
	if err != nil {
		log.Fatalf("slycrel: %v", err)
	}

	if *audit {
		if err := g.Audit(os.Stdout); err != nil {
			log.Fatalf("slycrel: %v", err)
		}
		return
	}

	game.KeyLog = *keylog
	if *demo {
		g.StartDemo()
	}

	ebiten.SetWindowSize(render.ScreenW**scale, render.ScreenH**scale)
	ebiten.SetWindowTitle(fmt.Sprintf("Slycrel — seed %d", *seed))
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(*fullscreen)
	ebiten.SetTPS(60)

	if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
		log.Fatalf("slycrel: %v", err)
	}
}
