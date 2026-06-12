package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// systemStarsTestContext builds the minimal Context a SystemStars
// generator/renderer needs for a seed: the system-stars random stream and the
// settings (which carry the time of day, horizon, and twinkle angle the suns are
// built and drawn from). SystemStars reads no shared globals (no gradients or
// ocean), so none are wired. It mirrors how Scene.Build sets up the per-element
// stream, keyed by the element's Name().
func systemStarsTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	return &Context{
		Ctx:      WithInstant(context.Background()),
		Canvas:   canvas.New(w, h),
		Settings: settings,
		Seed:     seed,
		W:        w,
		H:        h,
		Rng:      deriveRng(seed, (&SystemStars{}).Name()),
	}
}

// renderSystemStarsListHash renders a system-star scene list onto a fresh canvas
// and hashes the pixels. RenderList consumes no randomness, so the stream in the
// context is irrelevant here.
func renderSystemStarsListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := systemStarsTestContext(seed, w, h)
	if err := (&SystemStars{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestSystemStarsSceneListRoundTrip is the milestone's keystone for the system
// suns: a generated system-star scene list, serialized to YAML and read back,
// must (a) re-serialize to the same bytes and (b) render to the same pixels. This
// proves the "config + scene-list → render" reproducibility path end-to-end for
// the system's local star(s).
func TestSystemStarsSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13,
		17, 19, 23, 29, 37, 41, 43, 47, 53, 59, 61, 67, 71, 73, 79, 83}
	var sawNonEmpty, sawPlus, sawDim bool

	for _, seed := range seeds {
		gen := systemStarsTestContext(seed, w, h)
		list, err := (&SystemStars{}).Generate(gen)
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
		if a, b := renderSystemStarsListHash(t, seed, w, h, list), renderSystemStarsListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			v, ok := e.(*SystemStarV0)
			if !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
			sawNonEmpty = true
			if v.Plus {
				sawPlus = true
			}
			if v.Bright < 1 {
				sawDim = true
			}
		}
	}

	// Coverage sanity: the chosen seeds should produce at least one sun, so the
	// round-trip actually tests the schema.
	if !sawNonEmpty {
		t.Fatal("no system suns generated across all seeds; pick seeds that produce suns")
	}
	if !sawPlus {
		t.Log("note: no twinkle-cross sun in the seed set (plus shape not exercised)")
	}
	if !sawDim {
		t.Log("note: no dim night sun in the seed set (bright<1 not exercised)")
	}
}
