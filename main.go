// Command scifi-landscape procedurally draws a sci-fi landscape, one element
// at a time, from a random seed. The construction is animated on screen; the
// finished scene stays up and can be saved to a PNG.
//
//	scifi-landscape                 # random seed
//	scifi-landscape -seed 12345     # reproduce a specific scene
//	scifi-landscape -time dusk      # force the time of day
//
// While running:
//
//	N / Space   new random scene
//	R           replay the current seed
//	S           save the current image to scifi-<seed>.png
//	Q / Esc     quit
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/zostay/scifi-landscape/internal/app"
	"github.com/zostay/scifi-landscape/internal/seed"
)

// resolveSeed turns the -seed flag into an int64. An empty flag picks a random
// seed; otherwise the value is resolved by internal/seed (numbers used directly,
// text hashed).
func resolveSeed(s string) int64 {
	if s == "" {
		return rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	}
	return seed.Resolve(s)
}

func main() {
	var (
		seedStr = flag.String("seed", "", "scene seed: a number, or any text (hashed); empty picks a random one")
		todStr  = flag.String("time", "", "force time of day: midday, dusk, or twilight")
		width   = flag.Int("w", 1280, "scene width in pixels")
		height  = flag.Int("h", 720, "scene height in pixels")
	)
	flag.Parse()

	s := resolveSeed(*seedStr)

	ctrl := app.NewController(*width, *height, *todStr)
	ctrl.Start(s)

	if *seedStr == "" || seed.IsNumeric(*seedStr) {
		fmt.Printf("scifi-landscape: seed %d (reproduce with -seed %d)\n", s, s)
	} else {
		fmt.Printf("scifi-landscape: seed %q → %d (reproduce with -seed %q or -seed %d)\n", *seedStr, s, *seedStr, s)
	}

	ebiten.SetWindowSize(*width, *height)
	ebiten.SetWindowTitle("scifi-landscape")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Pause the game loop while the window is in the background so it doesn't
	// churn frames (or pile up ticks to replay on return) when left unattended.
	// The scene still finishes building on its own goroutine regardless.
	ebiten.SetRunnableOnUnfocused(false)

	if err := ebiten.RunGame(app.NewGame(ctrl)); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
