// Package scene composes a sci-fi landscape out of ordered elements.
//
// A scene is generated entirely from a single random seed: the same seed
// always reproduces the same settings and the same element artwork. Elements
// are rendered in sequence onto a shared canvas, and each may animate its own
// construction so the build can be watched live.
package scene

import (
	"context"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Context carries everything an element needs to render itself. The same
// *rand.Rand is threaded through every element so the whole scene is
// deterministic in seed order.
type Context struct {
	Ctx      context.Context
	Canvas   *canvas.Canvas
	Rng      *rand.Rand
	Settings Settings
	W, H     int

	// SkyGradient is the scene's sky color gradient (horizon -> top), built once
	// up front so elements other than the sky (e.g. planets fading into the sky
	// near the horizon) can sample the same colors the sky was drawn with.
	SkyGradient gfx.Gradient

	// GroundGradient is the scene's ground color gradient (horizon -> foreground),
	// also built once up front and shared, so mountains can base their color on
	// the same palette as the ground. GroundVariable reports whether the ground
	// uses the multi-color "variable" mode.
	GroundGradient gfx.Gradient
	GroundVariable bool
}

// Element is one piece of a scene (sky, ground, structures, ...). Render draws
// the element onto the canvas and should return ctx.Err() promptly if the
// context is cancelled (e.g. the user requested a regenerate).
type Element interface {
	Name() string
	Render(c *Context) error
}

// Scene is an ordered collection of elements plus the settings that shape them.
type Scene struct {
	Settings Settings
	Elements []Element
}

// New builds the element pipeline for the given settings. As the project grows
// this is where element selection, exclusions, and ordering will live; for now
// it is just the sky.
func New(s Settings) *Scene {
	return &Scene{
		Settings: s,
		Elements: []Element{
			&Sky{},
			&Stars{},
			&SystemStars{},
			&Planets{},
			&Clouds{},
			&Mountains{},
			&Ground{},
			&Cities{},
			&Water{},
		},
	}
}

// Build renders every element of the scene onto cv in order, threading rng
// through each one. onElement, if non-nil, is called with each element's name
// just before it renders (used to report progress). It returns ctx.Err() if
// generation is cancelled mid-build.
//
// This is the single shared rendering path used by both the live UI and the
// headless renderer, so they always produce identical output for a given seed.
func (sc *Scene) Build(ctx context.Context, cv *canvas.Canvas, rng *rand.Rand, w, h int, onElement func(string)) error {
	sctx := &Context{
		Ctx:      ctx,
		Canvas:   cv,
		Rng:      rng,
		Settings: sc.Settings,
		W:        w,
		H:        h,
	}
	// Build the sky and ground gradients up front so every element can share
	// them (planets fade into the sky color; mountains base on the ground color).
	sctx.SkyGradient = buildSkyGradient(rng, sc.Settings.Time)
	sctx.GroundVariable = rng.Float64() < groundVariableChance
	sctx.GroundGradient = buildGroundGradient(rng, sc.Settings.Time, sctx.GroundVariable)

	for _, el := range sc.Elements {
		if onElement != nil {
			onElement(el.Name())
		}
		if err := el.Render(sctx); err != nil {
			return err
		}
	}
	return nil
}

// sleep pauses for d, but returns early with ctx.Err() if the context is
// cancelled. Elements use it to pace their animation without ignoring
// regenerate/quit requests.
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
