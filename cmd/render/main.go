// Command render builds a scene headlessly (no window) and writes it to a PNG.
// It runs the exact same element pipeline as the interactive app, so a seed
// renders identically here and on screen.
//
//	go run ./cmd/render -seed 12345 -o scene.png
//	go run ./cmd/render -seed 7 -time twilight -w 1920 -h 1080
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/scene"
	"github.com/zostay/scifi-landscape/internal/seed"
)

func main() {
	var (
		seedStr = flag.String("seed", "", "scene seed: a number, or any text (hashed); empty picks a random one")
		todStr  = flag.String("time", "", "force time of day: midday, dusk, or twilight")
		width   = flag.Int("w", 1280, "scene width in pixels")
		height  = flag.Int("h", 720, "scene height in pixels")
		out     = flag.String("o", "", "output PNG path (default scifi-<seed>.png)")
	)
	flag.Parse()

	var s int64
	if *seedStr == "" {
		s = rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
	} else {
		s = seed.Resolve(*seedStr)
	}

	rng := rand.New(rand.NewSource(s))
	settings := scene.NewSettings(rng, *todStr, *height)
	cv := canvas.New(*width, *height)

	sc := scene.New(settings)
	if err := sc.Build(context.Background(), cv, rng, *width, *height, nil); err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}

	name := *out
	if name == "" {
		name = fmt.Sprintf("scifi-%d.png", s)
	}
	if err := cv.SavePNG(name); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}
	label := ""
	if *seedStr != "" && !seed.IsNumeric(*seedStr) {
		label = fmt.Sprintf("%q → ", *seedStr)
	}
	fmt.Printf("seed %s%d  %s  horizon %.0f%%  ->  %s\n", label, s, settings.Time, settings.Horizon*100, name)
}
