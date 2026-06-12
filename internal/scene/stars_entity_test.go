package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// starsTestContext builds the minimal Context a Stars generator/renderer needs
// for a seed: the stars random stream and the settings (which carry the time of
// day, star density, and twinkle angle the field is built and drawn from). Stars
// read no shared globals (no gradients or ocean), so none are wired. It mirrors
// how Scene.Build sets up the per-element stream.
func starsTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	return &Context{
		Ctx:      WithInstant(context.Background()),
		Canvas:   canvas.New(w, h),
		Settings: settings,
		Seed:     seed,
		W:        w,
		H:        h,
		Rng:      deriveRng(seed, "stars"),
	}
}

// renderStarsListHash renders a star scene list onto a fresh canvas and hashes
// the pixels. RenderList consumes no randomness, so the stream in the context is
// irrelevant here.
func renderStarsListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := starsTestContext(seed, w, h)
	if err := (&Stars{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestStarsSceneListRoundTrip is the milestone's keystone for stars: a generated
// star scene list, serialized to YAML and read back, must (a) re-serialize to the
// same bytes and (b) render to the same pixels. This proves the "config +
// scene-list → render" reproducibility path end-to-end for the star field.
func TestStarsSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13}
	var sawNonEmpty, sawDisc, sawSpikes bool

	for _, seed := range seeds {
		gen := starsTestContext(seed, w, h)
		list, err := (&Stars{}).Generate(gen)
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
		if a, b := renderStarsListHash(t, seed, w, h, list), renderStarsListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			v, ok := e.(*StarV0)
			if !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
			sawNonEmpty = true
			if v.Radius > 0 {
				sawDisc = true
			}
			if v.Spikes {
				sawSpikes = true
			}
		}
	}

	// Coverage sanity: the chosen seeds should produce a non-empty field that
	// exercises the disc and twinkle-spike shapes, so the round-trip actually
	// tests them.
	if !sawNonEmpty {
		t.Fatal("no stars generated across all seeds; pick seeds/settings that produce stars")
	}
	if !sawDisc {
		t.Log("note: no disc star in the seed set (radius>0 shape not exercised)")
	}
	if !sawSpikes {
		t.Log("note: no twinkle-spike star in the seed set (spikes not exercised)")
	}
}
