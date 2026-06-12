// Command scifi-landscape procedurally draws a sci-fi landscape, one element
// at a time, from a random seed. The construction is animated on screen; the
// finished scene stays up and can be saved to a PNG.
//
//	scifi-landscape                     # random seed
//	scifi-landscape -s 12345            # reproduce a specific scene
//	scifi-landscape -t dusk             # force the time of day
//	scifi-landscape -c my.yaml          # tune generation with a config file
//	scifi-landscape -f scene.png        # reproduce a saved scene file (seed + config)
//
// Saving (S) writes a scene file: a PNG with the seed, config, and globals
// embedded, so the scene can be reproduced later with --from.
//
// While running:
//
//	N / Space   new random scene
//	R           replay the current seed
//	S           save the current image to scifi-<seed>.png
//	Q / Esc     quit
package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/spf13/cobra"

	"github.com/zostay/scifi-landscape/internal/app"
	"github.com/zostay/scifi-landscape/internal/cli"
	"github.com/zostay/scifi-landscape/internal/seed"
)

// resolveSeed turns the --seed flag into an int64. An empty flag picks a random
// seed; otherwise the value is resolved by internal/seed (numbers used directly,
// text hashed).
func resolveSeed(s string) int64 {
	if s == "" {
		return rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	}
	return seed.Resolve(s)
}

func run(flags *cli.SceneFlags) error {
	cfg, seedSrc, err := cli.Resolve(flags.Config, flags.From, flags.Seed)
	if err != nil {
		return err
	}

	s := resolveSeed(seedSrc)

	ctrl := app.NewController(flags.Width, flags.Height, flags.Time, cfg)
	ctrl.Start(s)

	if seedSrc == "" || seed.IsNumeric(seedSrc) {
		fmt.Printf("scifi-landscape: seed %d (reproduce with -s %d)\n", s, s)
	} else {
		fmt.Printf("scifi-landscape: seed %q → %d (reproduce with -s %q or -s %d)\n", seedSrc, s, seedSrc, s)
	}

	ebiten.SetWindowSize(flags.Width, flags.Height)
	ebiten.SetWindowTitle("scifi-landscape")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Pause the game loop while the window is in the background so it doesn't
	// churn frames (or pile up ticks to replay on return) when left unattended.
	// The scene still finishes building on its own goroutine regardless.
	ebiten.SetRunnableOnUnfocused(false)

	return ebiten.RunGame(app.NewGame(ctrl))
}

func main() {
	var flags *cli.SceneFlags
	cmd := &cobra.Command{
		Use:           "scifi-landscape",
		Short:         "Procedurally draw an animated sci-fi landscape from a seed",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(flags)
		},
	}
	flags = cli.AddSceneFlags(cmd)

	if err := cmd.Execute(); err != nil {
		log.Fatalln("scifi-landscape:", err)
	}
}
