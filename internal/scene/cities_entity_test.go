package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// citiesTestContext builds the minimal Context a Cities generator/renderer needs
// for a seed: the cities random stream, the settings, the sky gradient (which the
// dome renderer samples for reflections), and the ocean-derived LandAt predicate
// (which Generate consults to keep buildings on land). It mirrors how Scene.Build
// wires these up, rebuilding the shared globals from the same derived streams so
// both renders see identical globals.
func citiesTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	oc := buildOcean(deriveRng(seed, "water"), settings, h)
	return &Context{
		Ctx:         WithInstant(context.Background()),
		Canvas:      canvas.New(w, h),
		Settings:    settings,
		Seed:        seed,
		W:           w,
		H:           h,
		SkyGradient: buildSkyGradient(deriveRng(seed, "sky-gradient"), settings.Time),
		Ocean:       oc,
		LandAt:      oc.LandAt,
		Rng:         deriveRng(seed, "cities"),
	}
}

// cityRenderListHash renders a city scene list onto a fresh canvas and hashes the
// pixels. RenderList consumes no randomness, so the stream in the context is
// irrelevant here.
func cityRenderListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := citiesTestContext(seed, w, h)
	if err := (&Cities{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestCitiesSceneListRoundTrip proves the cities reproducibility path end-to-end:
// a generated city scene list, serialized to YAML and read back, must (a)
// re-serialize to the same bytes and (b) render to the same pixels.
func TestCitiesSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13, 17, 23, 99, 12345}
	var sawCity, sawDome bool

	for _, seed := range seeds {
		gen := citiesTestContext(seed, w, h)
		list, err := (&Cities{}).Generate(gen)
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
		if a, b := cityRenderListHash(t, seed, w, h, list), cityRenderListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			city, ok := e.(*CityV0)
			if !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
			if len(city.Buildings) > 0 {
				sawCity = true
			}
			if len(city.Domes) > 0 {
				sawDome = true
			}
		}
	}

	// Coverage sanity: the chosen seeds should produce at least one city (so the
	// schema is actually exercised) and ideally a domed one.
	if !sawCity {
		t.Fatal("no city generated across all seeds; pick seeds that produce a city")
	}
	if !sawDome {
		t.Log("note: no domed city in the seed set (dome schema not exercised)")
	}
}
