package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// planetsTestContext builds the minimal Context a Planets generator/renderer
// needs for a seed: the planets random stream, the settings, and the sky
// gradient (which RenderList samples for haze). It mirrors how Scene.Build wires
// these up.
func planetsTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	return &Context{
		Ctx:         WithInstant(context.Background()),
		Canvas:      canvas.New(w, h),
		Settings:    settings,
		Seed:        seed,
		W:           w,
		H:           h,
		SkyGradient: buildSkyGradient(deriveRng(seed, "sky-gradient"), settings.Time),
		Rng:         deriveRng(seed, "planets"),
	}
}

// renderListHash renders a planet scene list onto a fresh canvas and hashes the
// pixels. RenderList consumes no randomness, so the stream in the context is
// irrelevant here.
func renderListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := planetsTestContext(seed, w, h)
	if err := (&Planets{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestPlanetsSceneListRoundTrip is the milestone's keystone: a generated planet
// scene list, serialized to YAML and read back, must (a) re-serialize to the
// same bytes and (b) render to the same pixels. This is the "config +
// scene-list → render" reproducibility path proven end-to-end for planets.
func TestPlanetsSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13}
	var sawGas, sawMoon, sawRing, sawCrater, sawNonEmpty bool

	for _, seed := range seeds {
		gen := planetsTestContext(seed, w, h)
		list, err := (&Planets{}).Generate(gen)
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
		if a, b := renderListHash(t, seed, w, h, list), renderListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			sawNonEmpty = true
			switch v := e.(type) {
			case *PlanetGasGiantV0:
				sawGas = true
				if v.Body.Ring != nil {
					sawRing = true
				}
			case *PlanetMoonV0:
				sawMoon = true
				if v.Body.Ring != nil {
					sawRing = true
				}
				if len(v.Craters) > 0 {
					sawCrater = true
				}
			default:
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
		}
	}

	// Coverage sanity: the chosen seeds should exercise both planet schemas (and
	// ideally rings and craters), so the round-trip actually tests them.
	if !sawNonEmpty {
		t.Fatal("no planets generated across all seeds; pick seeds that produce planets")
	}
	if !sawGas || !sawMoon {
		t.Errorf("did not exercise both planet schemas: gasGiant=%v moon=%v", sawGas, sawMoon)
	}
	if !sawRing {
		t.Log("note: no ringed planet in the seed set (ring schema not exercised)")
	}
	if !sawCrater {
		t.Log("note: no cratered moon in the seed set (crater schema not exercised)")
	}
}

// TestUnmarshalSceneListUnknownSchema checks that an unknown schema key is a hard
// error, so a scene file from a newer build fails loudly rather than silently
// dropping entities.
func TestUnmarshalSceneListUnknownSchema(t *testing.T) {
	_, err := UnmarshalSceneList([]byte("- schema: planet.unknown.v9\n  data: {}\n"))
	if err == nil {
		t.Fatal("expected error for unknown schema")
	}
}
