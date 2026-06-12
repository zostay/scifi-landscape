package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// mountainsTestContext builds the minimal Context a Mountains generator/renderer
// needs for a seed: the mountains random stream, the settings, and the ground
// gradient (which Generate samples for the base mountain color). It mirrors how
// Scene.Build wires these up — in particular it rolls groundVariable on its own
// derived stream before building the ground gradient, exactly as Build does, so
// both generation and rendering see the identical shared globals.
func mountainsTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	gg := deriveRng(seed, "ground-gradient")
	variable := gg.Float64() < groundVariableChance
	return &Context{
		Ctx:            WithInstant(context.Background()),
		Canvas:         canvas.New(w, h),
		Settings:       settings,
		Seed:           seed,
		W:              w,
		H:              h,
		GroundGradient: buildGroundGradient(gg, settings.Time, variable),
		GroundVariable: variable,
		Rng:            deriveRng(seed, "mountains"),
	}
}

// mountainsRenderListHash renders a mountain scene list onto a fresh canvas and
// hashes the pixels. RenderList consumes no randomness, so the stream in the
// context is irrelevant here.
func mountainsRenderListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := mountainsTestContext(seed, w, h)
	if err := (&Mountains{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestMountainsSceneListRoundTrip mirrors TestPlanetsSceneListRoundTrip: a
// generated mountain scene list, serialized to YAML and read back, must (a)
// re-serialize to the same bytes and (b) render to the same pixels. This proves
// the "config + scene-list → render" reproducibility path for mountains.
func TestMountainsSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13}
	var sawNonEmpty bool

	for _, seed := range seeds {
		gen := mountainsTestContext(seed, w, h)
		list, err := (&Mountains{}).Generate(gen)
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

		// The reconstructed list must render identically to the original.
		if a, b := mountainsRenderListHash(t, seed, w, h, list), mountainsRenderListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			sawNonEmpty = true
			if _, ok := e.(*MountainsV0); !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
		}
	}

	// Coverage sanity: the chosen seeds should produce a mountain range, so the
	// round-trip actually exercises the schema.
	if !sawNonEmpty {
		t.Fatal("no mountains generated across all seeds; pick seeds with a tall enough horizon")
	}
}
