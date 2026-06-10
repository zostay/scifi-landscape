package scene

import (
	"math"
	"math/rand"
	"testing"
)

// buildGeodesic must produce edges that all lie on the unit sphere's upper
// hemisphere (the dome), so the projection sits cleanly on the ground.
func TestBuildGeodesicUpperHemisphere(t *testing.T) {
	for _, freq := range []int{2, 3, 4} {
		edges := buildGeodesic(freq)
		if len(edges) == 0 {
			t.Fatalf("freq %d produced no edges", freq)
		}
		for _, e := range edges {
			for _, p := range e {
				if p[1] < -1e-6 {
					t.Fatalf("freq %d: edge endpoint below the equator: %v", freq, p)
				}
				if l := math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2]); math.Abs(l-1) > 1e-6 {
					t.Fatalf("freq %d: endpoint not on unit sphere (len %.4f)", freq, l)
				}
			}
		}
	}
}

// planDomes must keep within the dome cap and place every dome over the city's
// horizontal extent (centered on a real building).
func TestPlanDomesWithinCity(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	blds := []building{
		{x: 100, base: 210, w: 3, h: 8},
		{x: 140, base: 214, w: 4, h: 12},
		{x: 180, base: 212, w: 2, h: 6},
	}
	minX, maxX := 100, 184
	got := 0
	for range 200 {
		domes := planDomes(rng, blds, 200, 20, 1280)
		if len(domes) > domeMaxCount {
			t.Fatalf("got %d domes, max %d", len(domes), domeMaxCount)
		}
		for _, d := range domes {
			got++
			if d.cx < minX || d.cx > maxX {
				t.Fatalf("dome center %d outside city [%d,%d]", d.cx, minX, maxX)
			}
			if d.r < domeMinR {
				t.Fatalf("dome radius %d below min %d", d.r, domeMinR)
			}
		}
	}
	if got == 0 {
		t.Fatal("no domes ever planned over 200 tries")
	}
}
