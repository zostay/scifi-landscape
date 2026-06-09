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
	"github.com/zostay/scifi-landscape/internal/seed"
)

// Context carries everything an element needs to render itself. Each element gets
// its own independent random stream (Rng), derived from the master Seed and the
// element's name, so the whole scene is deterministic for a seed while elements
// stay isolated from one another's random draws.
type Context struct {
	Ctx    context.Context
	Canvas *canvas.Canvas
	// Rng is the random stream for the element currently rendering. Build swaps it
	// to a fresh, independent stream (derived from Seed and the element's name)
	// before each element renders, so one element's draws never affect another's.
	Rng      *rand.Rand
	Settings Settings
	// Seed is the scene's master seed; per-part streams are derived from it.
	Seed int64
	W, H int

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

// Build renders every element of the scene onto cv in order. Each element draws
// from its own independent random stream, derived from the master seed and the
// element's name, so adding a new element or changing how an existing one consumes
// randomness never shifts another element's output — a seed keeps its meaning as
// the codebase evolves. onElement, if non-nil, is called with each element's name
// just before it renders (used to report progress). It returns ctx.Err() if
// generation is cancelled mid-build.
//
// This is the single shared rendering path used by both the live UI and the
// headless renderer, so they always produce identical output for a given seed.
func (sc *Scene) Build(ctx context.Context, cv *canvas.Canvas, seed int64, w, h int, onElement func(string)) error {
	sctx := &Context{
		Ctx:      ctx,
		Canvas:   cv,
		Settings: sc.Settings,
		Seed:     seed,
		W:        w,
		H:        h,
	}
	// Build the sky and ground gradients up front so every element can share them
	// (planets fade into the sky color; mountains base on the ground color). Each
	// gets its own derived stream so they stay independent of each other and of
	// the elements.
	sctx.SkyGradient = buildSkyGradient(deriveRng(seed, "sky-gradient"), sc.Settings.Time)
	gg := deriveRng(seed, "ground-gradient")
	sctx.GroundVariable = gg.Float64() < groundVariableChance
	sctx.GroundGradient = buildGroundGradient(gg, sc.Settings.Time, sctx.GroundVariable)

	for _, el := range sc.Elements {
		if onElement != nil {
			onElement(el.Name())
		}
		// Reset to this element's own independent stream before it renders.
		sctx.Rng = deriveRng(seed, el.Name())
		if err := el.Render(sctx); err != nil {
			return err
		}
	}
	return nil
}

// deriveRng returns the independent random stream for one named part of a scene,
// seeded deterministically from the master seed and the name (see seed.Derive).
func deriveRng(master int64, key string) *rand.Rand {
	return rand.New(rand.NewSource(seed.Derive(master, key)))
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
