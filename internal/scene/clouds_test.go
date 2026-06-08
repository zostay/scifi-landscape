package scene

import (
	"math/rand"
	"testing"
)

// lowCloudLayerCount must stay in [1, cloudLowMaxLayers] and favor a single
// layer (the sky is kept sparse).
func TestLowCloudLayerCount(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var single, total int
	const n = 100000
	for range n {
		c := lowCloudLayerCount(rng)
		if c < 1 || c > cloudLowMaxLayers {
			t.Fatalf("lowCloudLayerCount = %d, out of [1,%d]", c, cloudLowMaxLayers)
		}
		if c == 1 {
			single++
		}
		total += c
	}
	if frac := float64(single) / n; frac < 0.45 {
		t.Errorf("only %.0f%% of scenes are single-layer, want a majority", frac*100)
	}
}

// makeCloud should produce a usable cloud: a sane bounding box, a flat bottom
// clamped above the horizon, and a height field that is positive somewhere inside
// (the cloud isn't eroded away to nothing) yet negative out at the edges.
func TestMakeCloudShape(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	const horizon = 500
	for range 1000 {
		cx := rng.Float64() * 1280
		baseY := rnd(rng, 100, 480)
		cw := rnd(rng, 60, 320)
		lit, shadow := cloudColorsLow(rng, Midday)
		cd := makeCloud(rng, cx, baseY, cw, horizon, lit, shadow)

		if cd.maxX <= cd.minX || cd.ch <= 0 || cd.cw <= 0 {
			t.Fatalf("degenerate cloud: minX=%d maxX=%d cw=%.1f ch=%.1f", cd.minX, cd.maxX, cd.cw, cd.ch)
		}
		if cd.baseY > horizon-1 {
			t.Fatalf("flat bottom %.1f below the horizon %d", cd.baseY, horizon)
		}
		// Bottom-center should be solid cloud; a point well outside should not be.
		if h := cloudHeight(cd, cd.cx, cd.baseY-0.1*cd.ch); h <= 0 {
			t.Fatalf("cloud center height %.3f, want > 0", h)
		}
		if h := cloudHeight(cd, cd.cx+cd.cw, cd.baseY-2*cd.ch); h > 0 {
			t.Fatalf("height %.3f well outside the cloud, want <= 0", h)
		}
	}
}
