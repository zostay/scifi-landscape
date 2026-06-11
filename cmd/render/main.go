// Command render builds a scene headlessly (no window) and writes it to a scene
// file: a PNG with the seed, config, and globals embedded so it can be reproduced
// later. It runs the exact same element pipeline as the interactive app, so a
// seed renders identically here and on screen.
//
//	go run ./cmd/render -seed 12345 -o scene.png
//	go run ./cmd/render -seed 7 -time twilight -w 1920 -h 1080
//	go run ./cmd/render -config my.yaml -seed 7 -o tuned.png
//	go run ./cmd/render -from scene.png -o copy.png   # reproduce a saved scene
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/cli"
	"github.com/zostay/scifi-landscape/internal/scene"
	"github.com/zostay/scifi-landscape/internal/scenefile"
	"github.com/zostay/scifi-landscape/internal/seed"
)

func main() {
	var (
		seedStr    = flag.String("seed", "", "scene seed: a number, or any text (hashed); empty picks a random one")
		todStr     = flag.String("time", "", "force time of day: midday, dusk, or twilight")
		width      = flag.Int("w", 1280, "scene width in pixels")
		height     = flag.Int("h", 720, "scene height in pixels")
		out        = flag.String("o", "", "output scene-file path (default scifi-<seed>.png)")
		configPath = flag.String("config", "", "YAML config file (partial or complete) to tune generation")
		fromPath   = flag.String("from", "", "reproduce from a scene file (PNG): supplies seed and config unless overridden")
	)
	flag.Parse()

	cfg, seedSrc, err := cli.Resolve(*configPath, *fromPath, *seedStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
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
	globals := dir.Direct(cfg, s, *todStr, *width, *height)

	cv := canvas.New(*width, *height)
	sc := scene.New(globals.Settings)
	// Headless: render instantly (no animation delay); the pixels are identical.
	if err := sc.Build(scene.WithInstant(context.Background()), cv, s, *width, *height, nil); err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}

	name := *out
	if name == "" {
		name = fmt.Sprintf("scifi-%d.png", s)
	}
	texts, err := scene.SceneTexts(s, cfg, &globals, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
	f, err := os.Create(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
	if err := scenefile.Write(f, cv.SnapshotImage(), texts); err != nil {
		f.Close()
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}

	label := ""
	if seedSrc != "" && !seed.IsNumeric(seedSrc) {
		label = fmt.Sprintf("%q → ", seedSrc)
	}
	fmt.Printf("seed %s%d  %s  horizon %.0f%%  ->  %s\n", label, s, globals.Settings.Time, globals.Settings.Horizon*100, name)
}
