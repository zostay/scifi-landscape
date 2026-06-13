package scene

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/config"
)

// hashScene renders pixels and returns their SHA-256.
func hashScene(t *testing.T, cv *canvas.Canvas, w, h int) [32]byte {
	t.Helper()
	buf := make([]byte, w*h*4)
	cv.Snapshot(buf)
	return sha256.Sum256(buf)
}

// globalsFor builds the default-config globals for a seed/size, as the app and
// renderer do.
func globalsFor(seed int64, tod string, w, h int) Globals {
	return DefaultDirector().Direct(config.DefaultConfig(), seed, tod, w, h)
}

// TestRenderListMatchesBuild is the scene-list replay guarantee: rendering a
// scene's recorded entity list (Scene.RenderList, the renderers-only path the
// `from --scene` mode uses) reproduces the exact image Scene.Build drew while
// generating that list. It spans a spread of seeds and sizes so element
// presence/absence (ocean, planets, clouds, cities) is exercised.
func TestRenderListMatchesBuild(t *testing.T) {
	type tc struct {
		seed int64
		w, h int
	}
	cases := []tc{
		{1, 480, 270}, {2, 480, 270}, {7, 480, 270}, {42, 480, 270},
		{256, 480, 270}, {-5, 480, 270}, {31337, 480, 270},
		{42, 1280, 720},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("s%d_%dx%d", c.seed, c.w, c.h), func(t *testing.T) {
			globals := globalsFor(c.seed, "", c.w, c.h)
			ctx := WithInstant(context.Background())

			// Build generates + renders, capturing the scene list.
			cvBuild := canvas.New(c.w, c.h)
			list, err := New(globals).Build(ctx, cvBuild, c.seed, c.w, c.h, nil)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}

			// Round-trip the list through YAML, mirroring exactly what `from --scene`
			// does (a scene file's recorded list is reloaded, not the in-memory one).
			y, err := MarshalSceneList(list)
			if err != nil {
				t.Fatalf("MarshalSceneList: %v", err)
			}
			reloaded, err := UnmarshalSceneList(y)
			if err != nil {
				t.Fatalf("UnmarshalSceneList: %v", err)
			}

			// RenderList replays only the renderers from that list on a fresh canvas.
			cvReplay := canvas.New(c.w, c.h)
			if err := New(globals).RenderList(ctx, cvReplay, c.seed, c.w, c.h, reloaded, nil); err != nil {
				t.Fatalf("RenderList: %v", err)
			}

			if hashScene(t, cvBuild, c.w, c.h) != hashScene(t, cvReplay, c.w, c.h) {
				t.Errorf("RenderList image differs from Build image for seed %d at %dx%d", c.seed, c.w, c.h)
			}
		})
	}
}

// unknownEntity is a stand-in entity whose schema no element owns.
type unknownEntity struct{}

func (unknownEntity) EntitySchema() string { return "does.not.exist.v0" }

// TestRenderListUnknownSchema confirms a scene list carrying an entity no element
// renders fails loudly rather than silently dropping it — so a scene file from a
// newer build can't be half-rendered.
func TestRenderListUnknownSchema(t *testing.T) {
	cv := canvas.New(480, 270)
	list := SceneList{unknownEntity{}}
	err := New(globalsFor(1, "", 480, 270)).RenderList(WithInstant(context.Background()), cv, 1, 480, 270, list, nil)
	if err == nil {
		t.Fatal("RenderList: want error for unknown entity schema, got nil")
	}
}

// TestRenderListSeedIndependent proves the payoff of promoting the gradients into
// globals: a recorded globals + scene list reproduce the image with NO dependence
// on the master seed. It renders the captured list through RenderList with a wildly
// different seed and asserts the pixels are byte-identical to the original Build.
// (The only remaining seed use in the render path is the ocean, which no renderer
// reads.) Before the gradients lived in globals this failed — the gradients would
// be re-derived from the wrong seed.
func TestRenderListSeedIndependent(t *testing.T) {
	const w, h int = 480, 270
	for _, seed := range []int64{7, 42, 256, -5} {
		t.Run(fmt.Sprintf("s%d", seed), func(t *testing.T) {
			globals := globalsFor(seed, "", w, h)
			ctx := WithInstant(context.Background())

			cvBuild := canvas.New(w, h)
			list, err := New(globals).Build(ctx, cvBuild, seed, w, h, nil)
			if err != nil {
				t.Fatalf("Build: %v", err)
			}

			// Replay the same globals + list, but hand RenderList a different seed.
			cvReplay := canvas.New(w, h)
			wrongSeed := seed + 123456789
			if err := New(globals).RenderList(ctx, cvReplay, wrongSeed, w, h, list, nil); err != nil {
				t.Fatalf("RenderList: %v", err)
			}

			if hashScene(t, cvBuild, w, h) != hashScene(t, cvReplay, w, h) {
				t.Errorf("seed %d: RenderList output changed with a different seed — render path still depends on the seed", seed)
			}
		})
	}
}
