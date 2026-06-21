package scene

import (
	"testing"
)

// TestSignedCoastDistance checks the per-column signed distance to the coastline:
// positive into land, negative into water, ~0 at the boundary, and a large signed
// magnitude for an all-one-kind row (so an all-land horizon stays covered and an
// all-water horizon stays bare).
func TestSignedCoastDistance(t *testing.T) {
	land := []bool{false, false, true, true, true, false}
	sd := signedCoastDistance(land)
	for i, v := range sd {
		if land[i] && v < 0 {
			t.Errorf("col %d is land but distance %v is negative", i, v)
		}
		if !land[i] && v > 0 {
			t.Errorf("col %d is water but distance %v is positive", i, v)
		}
	}
	// The columns straddling the two boundaries (1|2 and 4|5) should be the closest
	// to a coast (smallest magnitude).
	if abs(sd[1]) > 1 || abs(sd[2]) > 1 || abs(sd[4]) > 1 || abs(sd[5]) > 1 {
		t.Errorf("boundary columns should be ~0 from a coast, got %v", sd)
	}

	allLand := signedCoastDistance([]bool{true, true, true})
	for i, v := range allLand {
		if v < 2 {
			t.Errorf("all-land col %d should be far inside land, got %v", i, v)
		}
	}
	allWater := signedCoastDistance([]bool{false, false, false})
	for i, v := range allWater {
		if v > -2 {
			t.Errorf("all-water col %d should be far inside water, got %v", i, v)
		}
	}
}

// TestMountainsV1NoOceanMatchesV0 proves the no-ocean path is byte-identical to v0:
// with no ocean in the context, Mountains1 draws no extra randomness and applies no
// envelope, so it yields the exact same heightmap as Mountains.
func TestMountainsV1NoOceanMatchesV0(t *testing.T) {
	const w, h = 480, 270
	for _, seed := range []int64{42, 7, 256, 3, 100, 1024, 31337, 11} {
		v0ctx := mountainsTestContext(seed, w, h) // Ocean is nil
		v0list, err := (&Mountains{}).Generate(v0ctx)
		if err != nil {
			t.Fatalf("seed %d: v0 generate: %v", seed, err)
		}
		v1ctx := mountainsTestContext(seed, w, h) // Ocean is nil
		v1list, err := (&Mountains1{}).Generate(v1ctx)
		if err != nil {
			t.Fatalf("seed %d: v1 generate: %v", seed, err)
		}
		if len(v0list) != len(v1list) {
			t.Fatalf("seed %d: list length %d != %d", seed, len(v1list), len(v0list))
		}
		if len(v0list) == 0 {
			continue
		}
		a, _ := entityToMountains(v0list[0])
		b, _ := entityToMountains(v1list[0])
		if len(a.heights) != len(b.heights) {
			t.Fatalf("seed %d: heights length differs", seed)
		}
		for x := range a.heights {
			if a.heights[x] != b.heights[x] {
				t.Fatalf("seed %d: no-ocean v1 differs from v0 at col %d: %v != %v", seed, x, b.heights[x], a.heights[x])
			}
		}
	}
}

// TestMountainsV1CoastEnvelope proves the ocean path only ever lowers the ridge
// (the envelope is in [0,1]) and that it actually fires: across seeds with an ocean
// whose horizon has open water, at least one scene must come out shorter than v0
// (its feet brought down at the coast).
func TestMountainsV1CoastEnvelope(t *testing.T) {
	const w, h = 480, 270
	seeds := []int64{42, 7, 256, 3, 100, 1024, 31337, 11, 2, 5, 8, 13, 99, 77, 21, 64}
	var sawSuppression, sawOcean bool

	for _, seed := range seeds {
		base := mountainsTestContext(seed, w, h)
		oc := buildOcean(deriveRng(seed, "water"), base.Settings, h)
		if !oc.present {
			continue
		}
		sawOcean = true

		v0list, err := (&Mountains{}).Generate(mountainsTestContext(seed, w, h))
		if err != nil {
			t.Fatalf("seed %d: v0 generate: %v", seed, err)
		}
		v1ctx := mountainsTestContext(seed, w, h)
		v1ctx.Ocean = oc
		v1ctx.LandAt = oc.LandAt
		v1list, err := (&Mountains1{}).Generate(v1ctx)
		if err != nil {
			t.Fatalf("seed %d: v1 generate: %v", seed, err)
		}
		if len(v0list) == 0 || len(v1list) == 0 {
			continue
		}
		a, _ := entityToMountains(v0list[0])
		b, _ := entityToMountains(v1list[0])

		var sumV0, sumV1 float64
		for x := range a.heights {
			// The envelope only ever lowers a column.
			if b.heights[x] > a.heights[x]+1e-9 {
				t.Errorf("seed %d col %d: v1 height %v exceeds v0 %v", seed, x, b.heights[x], a.heights[x])
			}
			sumV0 += a.heights[x]
			sumV1 += b.heights[x]
		}
		if sumV1 < sumV0-1e-6 {
			sawSuppression = true
		}
	}

	if !sawOcean {
		t.Fatal("no ocean produced across seeds; pick seeds that roll an ocean")
	}
	if !sawSuppression {
		t.Fatal("coast envelope never lowered the range across ocean seeds; the feature is not firing")
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
