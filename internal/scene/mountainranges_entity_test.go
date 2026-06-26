package scene

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
)

// testRangeBase is a generous base that reliably produces several ranges, so the
// round-trip and rendering paths are actually exercised.
var testRangeBase = MountainRangeBase{
	Chance:             1.0,
	CountMax:           5,
	BaselineMaxFrac:    0.5,
	HeightMeanFrac:     0.10,
	HeightStdFrac:      0.04,
	SmoothnessMean:     0.6,
	SmoothnessStd:      0.2,
	BaselineJitterFrac: 0.02,
}

// mountainRangesTestContext builds the minimal Context the mountainranges
// generator/renderer needs: the element stream, the settings, the ground gradient
// (Generate samples it for the range colors), and the resolved base parameters. It
// mirrors how Scene.Build wires these up. If oc is non-nil, the ocean/land model is
// attached so the coastline bounding applies.
func mountainRangesTestContext(seed int64, w, h int, base MountainRangeBase, oc *ocean) *Context {
	settings := NewSettings(seed, "", h)
	gg := deriveRng(seed, "ground-gradient")
	variable := gg.Float64() < groundVariableChance
	c := &Context{
		Ctx:            WithInstant(context.Background()),
		Canvas:         canvas.New(w, h),
		Settings:       settings,
		Seed:           seed,
		W:              w,
		H:              h,
		GroundGradient: buildGroundGradient(gg, settings.Time, variable),
		GroundVariable: variable,
		MountainRanges: base,
		Rng:            deriveRng(seed, "mountainranges"),
	}
	if oc != nil {
		c.Ocean = oc
		c.LandAt = oc.LandAt
	}
	return c
}

// mountainRangesRenderHash renders an extra-ranges scene list onto a fresh canvas and
// hashes the pixels. RenderList consumes no randomness, so the stream is irrelevant.
func mountainRangesRenderHash(t *testing.T, seed int64, w, h int, list SceneList) string {
	t.Helper()
	c := mountainRangesTestContext(seed, w, h, testRangeBase, nil)
	if err := (&MountainRanges{}).RenderList(c, list); err != nil {
		t.Fatalf("seed %d: render list: %v", seed, err)
	}
	buf := make([]byte, w*h*4)
	c.Canvas.Snapshot(buf)
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}

// TestMountainRangesSceneListRoundTrip mirrors TestMountainsSceneListRoundTrip: a
// generated scene list, serialized to YAML and read back, must (a) re-serialize to
// the same bytes and (b) render to the same pixels.
func TestMountainRangesSceneListRoundTrip(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13}
	var sawRanges bool

	for _, seed := range seeds {
		gen := mountainRangesTestContext(seed, w, h, testRangeBase, nil)
		list, err := (&MountainRanges{}).Generate(gen)
		if err != nil {
			t.Fatalf("seed %d: generate: %v", seed, err)
		}
		if len(list) == 0 {
			continue
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
			t.Errorf("seed %d: scene list not stable across YAML round-trip", seed)
		}

		if a, b := mountainRangesRenderHash(t, seed, w, h, list), mountainRangesRenderHash(t, seed, w, h, got); a != b {
			t.Errorf("seed %d: round-tripped scene list renders differently:\n got  %s\n want %s", seed, b, a)
		}

		for _, e := range list {
			mr, ok := e.(*MountainRangesV0)
			if !ok {
				t.Fatalf("seed %d: unexpected entity type %T", seed, e)
			}
			if len(mr.Ranges) > 0 {
				sawRanges = true
			}
			// The feet must be ordered far→near (ascending baseline) so the render
			// occludes correctly.
			for i := 1; i < len(mr.Ranges); i++ {
				if mr.Ranges[i].Baseline < mr.Ranges[i-1].Baseline {
					t.Errorf("seed %d: ranges not ordered far→near at %d", seed, i)
				}
			}
		}
	}
	if !sawRanges {
		t.Fatal("no extra ranges generated across all seeds")
	}
}

// TestMountainRangesZeroBaseNoRanges proves the zero-value global (what the scene.v0
// director leaves) yields no extra ranges, so old/foreign configs are unaffected.
func TestMountainRangesZeroBaseNoRanges(t *testing.T) {
	const w, h = 480, 270
	for _, seed := range []int64{42, 7, 256, 3, 100} {
		c := mountainRangesTestContext(seed, w, h, MountainRangeBase{}, nil)
		list, err := (&MountainRanges{}).Generate(c)
		if err != nil {
			t.Fatalf("seed %d: generate: %v", seed, err)
		}
		if len(list) != 0 {
			t.Errorf("seed %d: expected no ranges from a zero base, got %d entities", seed, len(list))
		}
	}
}

// TestMountainRangesCountWithinCap proves the rolled count never exceeds the resolved
// per-vantage cap, and that a zero Chance suppresses ranges entirely.
func TestMountainRangesCountWithinCap(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{1, 2, 3, 7, 11, 42, 100, 256, 1024, 31337, 5, 8, 13, 99}

	for _, cap := range []int{2, 5} {
		base := testRangeBase
		base.CountMax = cap
		var sawAny bool
		for _, seed := range seeds {
			c := mountainRangesTestContext(seed, w, h, base, nil)
			list, err := (&MountainRanges{}).Generate(c)
			if err != nil {
				t.Fatalf("seed %d: generate: %v", seed, err)
			}
			if len(list) == 0 {
				continue
			}
			mr := list[0].(*MountainRangesV0)
			n := len(mr.Ranges)
			if n < 1 || n > cap {
				t.Errorf("seed %d cap %d: count %d out of range [1,%d]", seed, cap, n, cap)
			}
			sawAny = true
		}
		if !sawAny {
			t.Fatalf("cap %d: no ranges produced across seeds", cap)
		}
	}

	// Chance 0 → never any ranges.
	base := testRangeBase
	base.Chance = 0
	for _, seed := range seeds {
		c := mountainRangesTestContext(seed, w, h, base, nil)
		list, _ := (&MountainRanges{}).Generate(c)
		if len(list) != 0 {
			t.Errorf("seed %d: chance 0 should yield no ranges", seed)
		}
	}
}

// TestMountainRangesCoastlineApplied proves that, when the scene has an ocean, the
// element applies the coastline envelope (the heightmaps differ from the no-ocean
// case) — i.e. the ranges are bounded to land rather than spanning the full width.
// The envelope math itself is unit-tested in mountains_v1_test.go.
func TestMountainRangesCoastlineApplied(t *testing.T) {
	const w, h = 480, 270
	var checked bool
	for seed := int64(1); seed <= 60 && !checked; seed++ {
		base := mountainRangesTestContext(seed, w, h, testRangeBase, nil)
		oc := buildOcean(deriveRng(seed, "water"), base.Settings, h)
		if !oc.present {
			continue
		}
		dry, _ := (&MountainRanges{}).Generate(mountainRangesTestContext(seed, w, h, testRangeBase, nil))
		wet, _ := (&MountainRanges{}).Generate(mountainRangesTestContext(seed, w, h, testRangeBase, oc))
		if len(dry) == 0 || len(wet) == 0 {
			continue
		}
		a := dry[0].(*MountainRangesV0)
		b := wet[0].(*MountainRangesV0)
		// Same seed/stream and same number of ranges, but the ocean path applies the
		// envelope (and its extra draws), so the heightmaps must differ.
		same := len(a.Ranges) == len(b.Ranges)
		if same {
			for i := range a.Ranges {
				if !floatsEqual(a.Ranges[i].Heights, b.Ranges[i].Heights) {
					same = false
					break
				}
			}
		}
		if same {
			t.Errorf("seed %d: ocean present but ranges identical to no-ocean (envelope not applied)", seed)
		}
		// And no band should be left entirely flat-to-the-edges with water under it:
		// at least one column must be suppressed to zero somewhere.
		var anyZero bool
		for _, r := range b.Ranges {
			for _, hgt := range r.Heights {
				if hgt == 0 {
					anyZero = true
				}
			}
		}
		if !anyZero {
			t.Errorf("seed %d: ocean scene has no suppressed columns in any range", seed)
		}
		checked = true
	}
	if !checked {
		t.Fatal("no ocean scene exercised across seeds")
	}
}

func floatsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestNearestTrueDistance checks the column-distance-to-nearest-land helper used to
// fade the mist out over open water.
func TestNearestTrueDistance(t *testing.T) {
	got := nearestTrueDistance([]bool{true, false, false, true, false})
	want := []int{0, 1, 1, 0, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dist[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

// TestMistBaking proves the mist is baked on only when the scene rolled it and has
// ranges, and that its per-column ocean fade is full over land and falls off over open
// water — so the renderer (which cannot read the ocean) reproduces it.
func TestMistBaking(t *testing.T) {
	const w, h = 480, 270

	// An ocean seed: build the same ocean the scene would.
	var seed int64 = -1
	for s := int64(1); s <= 80; s++ {
		oc := buildOcean(deriveRng(s, "water"), NewSettings(s, "", h), h)
		if oc.present {
			seed = s
			break
		}
	}
	if seed < 0 {
		t.Fatal("no ocean seed found")
	}
	oc := buildOcean(deriveRng(seed, "water"), NewSettings(seed, "", h), h)

	mist := MistBase{Present: true, FadeUpFrac: 0.08, LowFadeFrac: 0.25, OceanFadeFrac: 0.10}

	// Mist on + ranges + ocean → baked on with a land/water fade.
	c := mountainRangesTestContext(seed, w, h, testRangeBase, oc)
	c.Mist = mist
	list, err := (&MountainRanges{}).Generate(c)
	if err != nil || len(list) == 0 {
		t.Fatalf("generate: %v (n=%d)", err, len(list))
	}
	e := list[0].(*MountainRangesV0)
	if !e.Mist {
		t.Fatal("mist not baked on for a mist scene with ranges and ocean")
	}
	if len(e.MistOceanFade) != w {
		t.Fatalf("ocean fade length %d, want %d", len(e.MistOceanFade), w)
	}
	var full, faded int
	for _, f := range e.MistOceanFade {
		switch {
		case f >= 0.999:
			full++
		case f < 0.5:
			faded++
		}
	}
	if full == 0 || faded == 0 {
		t.Errorf("expected both land (full=%d) and open-water (faded=%d) columns", full, faded)
	}

	// Mist not rolled → off.
	c2 := mountainRangesTestContext(seed, w, h, testRangeBase, oc)
	c2.Mist = MistBase{Present: false}
	list2, _ := (&MountainRanges{}).Generate(c2)
	if e2 := list2[0].(*MountainRangesV0); e2.Mist {
		t.Error("mist baked on when not rolled")
	}
}
