// Package scene composes a sci-fi landscape out of ordered elements.
//
// A scene is generated entirely from a single random seed: the same seed
// always reproduces the same settings and the same element artwork. Elements
// are rendered in sequence onto a shared canvas, and each may animate its own
// construction so the build can be watched live.
package scene

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/config"
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

	// Height is the scene-wide vantage point over the ground plane (from the globals,
	// derived by the director). The v1 ground/cities/water renderers read it to widen
	// the perspective in low mode; it is never re-derived here, so RenderList stays
	// seed-independent.
	Height HeightMode
	// Perspective carries the resolved low-mode ground-plane parameters (from the
	// globals); the v1 generators/renderers read it when Height == Low.
	Perspective Perspective

	// Ocean is the scene's resolved ocean/land model, decided up front (like the
	// gradients) so both Cities and Water can use it: Cities to place buildings
	// only on land, Water to leave islands and the coast unflooded while still
	// reflecting the city skyline drawn before it.
	Ocean *ocean

	// LandAt reports whether the below-horizon point (x, y) is land rather than
	// ocean. Without an ocean every point below the horizon is land; with one, only
	// islands and the coastline are. Land-based elements (e.g. Cities) consult it
	// so they sit on land and never on the water.
	LandAt func(x, y int) bool
}

// Element is one part of a scene (sky, ground, structures, ...): it generates
// its entities and renders them onto the canvas. Build resets the element's
// random stream, calls Generate to produce the element's scene list, then
// RenderList to draw it — so the generated entities can be recorded into a scene
// file. RenderList should return ctx.Err() promptly if the context is cancelled
// (e.g. the user requested a regenerate).
type Element interface {
	Generator
	Renderer
	// Schemas lists the entity schema keys this element produces and renders. It
	// lets Scene.RenderList partition a stored (flat) scene list back to the
	// element that owns each entity, since an element's RenderList accepts only
	// its own entity types.
	Schemas() []string
}

// Scene is an ordered collection of elements plus the globals that shape them.
// The globals carry the derived settings and the scene-wide gradients the
// renderers read, so a Scene built from a recorded globals renders identically
// without re-deriving that state from the seed.
type Scene struct {
	Globals  Globals
	Elements []Element
}

// New builds a scene for the given globals with the pipeline named by algos: each
// key in algos.Generators is resolved (in order) to its registered element, which
// renders back-to-front in that order. It errors if a generator key is not
// registered, or if any algos.Renderers key is not a registered renderer, so a
// config naming an unknown algorithm fails loudly. (For v0 each element is its own
// generator and renderer; per-entity renderer selection is a future step.)
func New(g Globals, algos config.Algorithms) (*Scene, error) {
	els, err := resolveElements(algos)
	if err != nil {
		return nil, err
	}
	return &Scene{Globals: g, Elements: els}, nil
}

// CheckAlgorithms reports whether every algorithm key in algos resolves against
// the registries. It lets a caller validate a (possibly user-supplied) config up
// front — before building globals or opening a window — and fail with a clear
// message.
func CheckAlgorithms(algos config.Algorithms) error {
	_, err := resolveElements(algos)
	return err
}

// resolveElements turns the config's generator keys into the ordered element
// pipeline and validates the renderer keys. It is the shared resolver behind New
// and CheckAlgorithms.
func resolveElements(algos config.Algorithms) ([]Element, error) {
	els := make([]Element, 0, len(algos.Generators))
	for _, key := range algos.Generators {
		el, ok := ElementByName(key)
		if !ok {
			return nil, fmt.Errorf("scene: unknown generator %q", key)
		}
		els = append(els, el)
	}
	for _, key := range algos.Renderers {
		if _, ok := RendererByName(key); !ok {
			return nil, fmt.Errorf("scene: unknown renderer %q", key)
		}
	}
	return els, nil
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
//
// Build returns the aggregate scene list — every element's generated entities,
// in render order — so a completed scene can be recorded into a scene file's
// scene-list layer.
func (sc *Scene) Build(ctx context.Context, cv *canvas.Canvas, seed int64, w, h int, onElement func(string)) (SceneList, error) {
	sctx := sc.newContext(ctx, cv, seed, w, h)

	var list SceneList
	for _, el := range sc.Elements {
		if onElement != nil {
			onElement(el.Name())
		}
		// Reset to this element's own independent stream before it generates.
		sctx.Rng = deriveRng(seed, el.Name())
		part, err := el.Generate(sctx)
		if err != nil {
			return nil, err
		}
		// RenderList consumes no randomness, so Generate-then-RenderList draws the
		// exact same pixels el.Render once did, while also yielding the entities.
		if err := el.RenderList(sctx, part); err != nil {
			return nil, err
		}
		list = append(list, part...)
	}
	return list, nil
}

// newContext builds the per-build render Context. The scene-wide gradients the
// renderers read come straight from the globals (the director derived them), so
// rendering a recorded scene list needs no seed for them. The ocean/land model is
// still resolved here from the seed — only generation reads it (Cities placement,
// Water capture); no renderer does — so it stays working state. Both Build and
// RenderList use this, so a scene rendered from its generated list sees exactly
// the shared state Build did.
func (sc *Scene) newContext(ctx context.Context, cv *canvas.Canvas, seed int64, w, h int) *Context {
	g := sc.Globals
	sctx := &Context{
		Ctx:      ctx,
		Canvas:   cv,
		Settings: g.Settings,
		Seed:     seed,
		W:        w,
		H:        h,
	}
	// Sky and ground gradients are globals (planets fade into the sky color;
	// mountains base on the ground color) — read, don't re-derive.
	sctx.SkyGradient = g.SkyGradient
	sctx.GroundGradient = g.GroundGradient
	sctx.GroundVariable = g.GroundVariable
	sctx.Height = g.Height
	sctx.Perspective = g.Perspective

	// Resolve the ocean/land model up front so Cities (drawn before Water) can keep
	// to land while Water still reflects the city skyline. For a v1 ocean (a v1 director
	// resolved a land distance) apply the same geometric coastline here, so the boundary
	// the cities consult (via LandAt) matches the one water.v1 will draw — buildings stay
	// on the land the water leaves dry. The gate must not depend on ShorePersp (which
	// only drives the waves and can be configured to 0), or the cities and water.v1 would
	// disagree. With no v1 perspective (v0) this is the original screen-space model, and
	// there is no need to build the shore map when the scene has no ocean.
	sctx.Ocean = buildOcean(deriveRng(seed, "water"), g.Settings, h)
	if sctx.Ocean.present && g.Perspective.LandDist > 0 {
		sctx.Ocean = sctx.Ocean.withPerspective(g.Perspective, w)
	}
	sctx.LandAt = sctx.Ocean.LandAt
	return sctx
}

// RenderList draws an already-generated scene list onto cv, skipping generation
// entirely: it is the renderers-only replay path, the counterpart to Build. It
// rebuilds the shared render context (sky/ground gradients, ocean) from seed and
// settings — that derived state is not captured in the scene list — then hands
// each element only the entities it owns, in pipeline order, so the image matches
// what Build produced for the same list. onElement reports progress like Build.
//
// An entity whose schema no element claims is an error, so a scene list from a
// newer build fails loudly rather than dropping entities silently.
func (sc *Scene) RenderList(ctx context.Context, cv *canvas.Canvas, seed int64, w, h int, list SceneList, onElement func(string)) error {
	sctx := sc.newContext(ctx, cv, seed, w, h)

	// Map each schema to the index of the element that owns it, then partition the
	// flat list into per-element sublists (order preserved within each element).
	owner := map[string]int{}
	for i, el := range sc.Elements {
		for _, s := range el.Schemas() {
			owner[s] = i
		}
	}
	parts := make([]SceneList, len(sc.Elements))
	for _, e := range list {
		i, ok := owner[e.EntitySchema()]
		if !ok {
			return fmt.Errorf("scene: no element renders entity schema %q", e.EntitySchema())
		}
		parts[i] = append(parts[i], e)
	}

	for i, el := range sc.Elements {
		if onElement != nil {
			onElement(el.Name())
		}
		// Renderers consume no randomness, but set the element's stream for parity
		// with Build in case a future renderer reads it.
		sctx.Rng = deriveRng(seed, el.Name())
		if err := el.RenderList(sctx, parts[i]); err != nil {
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

// instantKeyType is the unexported context-key type for the instant-render flag.
type instantKeyType struct{}

// instantKey marks a context as instant-render (no animation delays).
var instantKey instantKeyType

// WithInstant returns a context in which scene animations render with no delay.
// It affects only the pacing of the build (the sleep between animation bands),
// never the pixels produced — the final image for a given seed is byte-identical
// whether or not the build is animated. Headless renders and golden tests use it
// to skip the per-band waits that exist purely for the live, watch-it-draw UI.
func WithInstant(ctx context.Context) context.Context {
	return context.WithValue(ctx, instantKey, true)
}

// sleep pauses for d, but returns early with ctx.Err() if the context is
// cancelled. Elements use it to pace their animation without ignoring
// regenerate/quit requests. In an instant context (see WithInstant) it skips the
// wait entirely while still honoring cancellation, so the same pixels are drawn
// with no animation delay.
func sleep(ctx context.Context, d time.Duration) error {
	if instant, _ := ctx.Value(instantKey).(bool); instant {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
