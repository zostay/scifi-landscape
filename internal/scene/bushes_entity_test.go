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

// bushesTestContext builds the minimal Context a Bushes generator/renderer needs for a
// seed: the bushes random stream, the settings, the scene's bush gradient (which
// RenderList samples for each bush's base color), the resolved bushes base parameters,
// and an all-land LandAt so generation places bushes freely. It forces the low vantage
// so the bushes are large enough to exercise the drawing path.
func bushesTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	return &Context{
		Ctx:          WithInstant(context.Background()),
		Canvas:       canvas.New(w, h),
		Settings:     settings,
		Seed:         seed,
		W:            w,
		H:            h,
		BushGradient: buildBushGradient(deriveRng(seed, "bush-gradient"), settings.Time),
		Bushes:       resolveBushes(config.DefaultConfig().Bushes, Low),
		LandAt:       func(x, y int) bool { return true },
		Rng:          deriveRng(seed, "bushes"),
	}
}

// bushesRenderListHash renders a bushes scene list onto a fresh canvas and hashes the
// pixels. RenderList consumes no randomness, so the stream in the context is irrelevant.
func bushesRenderListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := bushesTestContext(seed, w, h)
	if err := (&Bushes{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestBushesSceneListRoundTrip mirrors the other element round-trip tests: a generated
// bushes scene list, serialized to YAML and read back, must (a) re-serialize to the same
// bytes and (b) render to the same pixels. This proves the "config + scene-list → render"
// reproducibility path for bushes.
func TestBushesSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13}
	var sawNonEmpty bool

	for _, seed := range seeds {
		gen := bushesTestContext(seed, w, h)
		list, err := (&Bushes{}).Generate(gen)
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

		if a, b := bushesRenderListHash(t, seed, w, h, list), bushesRenderListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			sawNonEmpty = true
			if _, ok := e.(*BushesV0); !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
		}
	}

	if !sawNonEmpty {
		t.Fatal("no bushes generated across all seeds; the chance roll or count may be misconfigured")
	}
}
