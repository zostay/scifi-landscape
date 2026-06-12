package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// cloudsTestContext builds the minimal Context a Clouds generator/renderer needs
// for a seed: the clouds random stream plus the settings (which supply the
// horizon, time of day, twinkle angle, and light color the layers read). Clouds
// read no shared globals (no sky gradient or ocean), so nothing else is wired.
// It mirrors how Scene.Build wires these up.
func cloudsTestContext(seed int64, w, h int) *Context {
	settings := NewSettings(seed, "", h)
	return &Context{
		Ctx:      WithInstant(context.Background()),
		Canvas:   canvas.New(w, h),
		Settings: settings,
		Seed:     seed,
		W:        w,
		H:        h,
		Rng:      deriveRng(seed, "clouds"),
	}
}

// renderCloudListHash renders a cloud scene list onto a fresh canvas and hashes
// the pixels. RenderList consumes no randomness, so the stream in the context is
// irrelevant here.
func renderCloudListHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := cloudsTestContext(seed, w, h)
	if err := (&Clouds{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestCloudsSceneListRoundTrip proves the cloud generator/renderer split is
// reproducible end-to-end: a generated cloud scene list, serialized to YAML and
// read back, must (a) re-serialize to the same bytes and (b) render to the same
// pixels. Seeds are chosen to exercise both the high gauzy sheet and the low
// nimbus clouds.
func TestCloudsSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13, 17, 19, 23, 29}
	var sawHigh, sawLow, sawNonEmpty bool

	for _, seed := range seeds {
		gen := cloudsTestContext(seed, w, h)
		list, err := (&Clouds{}).Generate(gen)
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
		if a, b := renderCloudListHash(t, seed, w, h, list), renderCloudListHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			sawNonEmpty = true
			switch e.(type) {
			case *CloudsHighV0:
				sawHigh = true
			case *CloudLowV0:
				sawLow = true
			default:
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
		}
	}

	// Coverage sanity: the chosen seeds should exercise both cloud schemas, so the
	// round-trip actually tests them.
	if !sawNonEmpty {
		t.Fatal("no clouds generated across all seeds; pick seeds that produce clouds")
	}
	if !sawHigh {
		t.Error("did not exercise the high gauzy cloud schema")
	}
	if !sawLow {
		t.Error("did not exercise the low nimbus cloud schema")
	}
}
