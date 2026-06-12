// Command render builds a scene headlessly (no window) and writes it to a scene
// file: a PNG with the seed, config, and globals embedded so it can be reproduced
// later. It runs the exact same element pipeline as the interactive app, so a
// seed renders identically here and on screen.
//
//	go run ./cmd/render -s 12345 -o scene.png
//	go run ./cmd/render -s 7 -t twilight -w 1920 --height 1080
//	go run ./cmd/render -c my.yaml -s 7 -o tuned.png
//	go run ./cmd/render -f scene.png -o copy.png   # reproduce a saved scene
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/cli"
	"github.com/zostay/scifi-landscape/internal/scene"
	"github.com/zostay/scifi-landscape/internal/scenefile"
	"github.com/zostay/scifi-landscape/internal/seed"
)

func render(flags *cli.SceneFlags, out string) error {
	cfg, seedSrc, err := cli.Resolve(flags.Config, flags.From, flags.Seed)
	if err != nil {
		return err
	}

	var s int64
	if seedSrc == "" {
		s = rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	} else {
		s = seed.Resolve(seedSrc)
	}

	// The director turns seed + config into the scene-wide globals.
	dir := scene.DefaultDirector()
	if dirs := cfg.Algorithms.Directors; len(dirs) > 0 {
		if d, ok := scene.DirectorByName(dirs[0]); ok {
			dir = d
		}
	}
	globals := dir.Direct(cfg, s, flags.Time, flags.Width, flags.Height)

	cv := canvas.New(flags.Width, flags.Height)
	sc := scene.New(globals.Settings)
	// Headless: render instantly (no animation delay); the pixels are identical.
	if err := sc.Build(scene.WithInstant(context.Background()), cv, s, flags.Width, flags.Height, nil); err != nil {
		return err
	}

	name := out
	if name == "" {
		name = fmt.Sprintf("scifi-%d.png", s)
	}
	texts, err := scene.SceneTexts(s, cfg, &globals, nil)
	if err != nil {
		return err
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := scenefile.Write(f, cv.SnapshotImage(), texts); err != nil {
		f.Close()
		return fmt.Errorf("save: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	label := ""
	if seedSrc != "" && !seed.IsNumeric(seedSrc) {
		label = fmt.Sprintf("%q → ", seedSrc)
	}
	fmt.Printf("seed %s%d  %s  horizon %.0f%%  ->  %s\n", label, s, globals.Settings.Time, globals.Settings.Horizon*100, name)
	return nil
}

func main() {
	var (
		flags *cli.SceneFlags
		out   string
	)
	cmd := &cobra.Command{
		Use:           "render",
		Short:         "Build a scene headlessly and write it to a scene file (PNG)",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return render(flags, out)
		},
	}
	flags = cli.AddSceneFlags(cmd)
	cmd.Flags().StringVarP(&out, "output", "o", "", "output scene-file path (default scifi-<seed>.png)")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
}
