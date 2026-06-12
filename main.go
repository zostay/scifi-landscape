// Command scifi-landscape procedurally draws a sci-fi landscape, one element
// at a time, from a random seed. The construction is animated on screen; the
// finished scene stays up and can be saved to a PNG.
//
//	scifi-landscape                     # random seed
//	scifi-landscape -s 12345            # reproduce a specific scene
//	scifi-landscape -t dusk             # force the time of day
//	scifi-landscape -c my.yaml          # tune generation with a config file
//	scifi-landscape -f scene.png        # reproduce a saved scene file (seed + config)
//	scifi-landscape config scene.png    # extract a scene file's embedded layers to files
//
// Saving (S) writes a scene file: a PNG with the seed, config, globals, and
// scene list embedded, so the scene can be reproduced later with --from.
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
	"os"
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
	cmd.AddCommand(configCmd())

	if err := cmd.Execute(); err != nil {
		log.Fatalln("scifi-landscape:", err)
	}
}

// configCmd builds the "config" subcommand, which extracts the reproducibility
// layers embedded in a PNG scene file to scifi-<seed>.* files. Config, globals,
// and the scene list are written by default; the seed is opt-in. Each output is
// toggled by its own flag.
func configCmd() *cobra.Command {
	var seedF, configF, globalsF, sceneF bool
	cmd := &cobra.Command{
		Use:   "config <scene.png>",
		Short: "Extract a PNG scene file's embedded layers to scifi-<seed>.* files",
		Long: `Extract the reproducibility layers embedded in a PNG scene file, writing each
to a file named after the scene's seed:

  scifi-<seed>.seed.txt      the seed
  scifi-<seed>.config.yaml   the config
  scifi-<seed>.globals.yaml  the derived globals
  scifi-<seed>.scene.yaml    the generated scene list

Config, globals, and the scene list are written by default; the seed is not.
Toggle any output with its flag, e.g. --seed to add the seed file or
--config=false to skip the config.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			want := map[string]bool{
				"seed":    seedF,
				"config":  configF,
				"globals": globalsF,
				"scene":   sceneF,
			}
			files, missing, err := cli.ExtractScene(args[0], want)
			if err != nil {
				return err
			}
			for _, m := range missing {
				fmt.Fprintf(os.Stderr, "scifi-landscape: scene file has no %s layer; skipping\n", m)
			}
			for _, f := range files {
				if err := os.WriteFile(f.File, []byte(f.Content), 0o644); err != nil {
					return err
				}
				fmt.Println(f.File)
			}
			return nil
		},
	}
	fl := cmd.Flags()
	fl.BoolVarP(&seedF, "seed", "s", false, "write scifi-<seed>.seed.txt")
	fl.BoolVar(&configF, "config", true, "write scifi-<seed>.config.yaml")
	fl.BoolVar(&globalsF, "globals", true, "write scifi-<seed>.globals.yaml")
	fl.BoolVar(&sceneF, "scene", true, "write scifi-<seed>.scene.yaml")
	return cmd
}
