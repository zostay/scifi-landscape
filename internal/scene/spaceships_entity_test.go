package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/config"
)

// spaceshipsTestContext builds the minimal Context a Spaceships generator/renderer needs
// for a seed: the spaceships random stream, the settings (for the horizon/sky band), and
// the resolved spaceships base parameters. RenderList consumes no randomness, so the
// stream is irrelevant to it.
func spaceshipsTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	return &Context{
		Ctx:        WithInstant(context.Background()),
		Canvas:     canvas.New(w, h),
		Settings:   settings,
		Seed:       seed,
		W:          w,
		H:          h,
		Spaceships: resolveSpaceships(config.DefaultConfig().Spaceships),
		Rng:        deriveRng(seed, "spaceships"),
	}
}

// spaceshipsRenderListHash renders a spaceships scene list onto a fresh canvas and hashes
// the pixels. RenderList consumes no randomness, so the stream in the context is irrelevant.
func spaceshipsRenderListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := spaceshipsTestContext(seed, w, h)
	if err := (&Spaceships{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestSpaceshipsSceneListRoundTrip mirrors the other element round-trip tests: a generated
// spaceships scene list, serialized to YAML and read back, must (a) re-serialize to the
// same bytes and (b) render to the same pixels. This proves the "config + scene-list →
// render" reproducibility path for spaceships.
func TestSpaceshipsSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13}
	var sawShip bool

	for _, seed := range seeds {
		gen := spaceshipsTestContext(seed, w, h)
		list, err := (&Spaceships{}).Generate(gen)
		if err != nil {
			t.Fatalf("seed %d: generate: %v", seed, err)
		}

		data, err := MarshalSceneList(list)
		if err != nil {
			t.Fatalf("seed %d: marshal: %v", seed, err)
		}
		got, err := UnmarshalSceneList(data)
		if err != nil {
			t.Fatalf("seed %d: unmarshal: %v", seed, err)
		}
		data2, err := MarshalSceneList(got)
		if err != nil {
			t.Fatalf("seed %d: re-marshal: %v", seed, err)
		}
		if !bytes.Equal(data, data2) {
			t.Errorf("seed %d: scene list not stable across YAML round-trip\n--- first ---\n%s\n--- second ---\n%s", seed, data, data2)
		}

		if a, b := spaceshipsRenderListHash(t, seed, w, h, list), spaceshipsRenderListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			sawShip = true
			if _, ok := e.(*SpaceshipV0); !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
		}
	}

	if !sawShip {
		t.Fatal("no spaceships generated across all seeds; the count may be misconfigured")
	}
}
