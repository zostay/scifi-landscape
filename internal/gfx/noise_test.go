package gfx

import (
	"math"
	"testing"
)

func TestFBMInRangeAndDeterministic(t *testing.T) {
	for i := range 5000 {
		x := float64(i%97) * 0.3
		y := float64(i%53) * 0.7
		v := FBM(x, y, 42, 4)
		if v < 0 || v > 1 {
			t.Fatalf("FBM(%g,%g) = %g out of [0,1]", x, y, v)
		}
		if v2 := FBM(x, y, 42, 4); v != v2 {
			t.Fatalf("FBM not deterministic: %g vs %g", v, v2)
		}
	}
}

// Value noise should be continuous: a tiny step in position yields a tiny step
// in value (no static-like jumps).
func TestValueNoiseContinuous(t *testing.T) {
	const eps = 0.001
	for i := range 1000 {
		x := float64(i) * 0.13
		y := float64(i) * 0.07
		a := valueNoise(x, y, 7)
		b := valueNoise(x+eps, y, 7)
		if math.Abs(a-b) > 0.05 {
			t.Fatalf("value noise jumped %g over a %g step at (%g,%g)", math.Abs(a-b), eps, x, y)
		}
	}
}

func TestFBMSeedsDiffer(t *testing.T) {
	if FBM(1.5, 2.5, 1, 4) == FBM(1.5, 2.5, 2, 4) {
		t.Error("different seeds produced identical FBM")
	}
}
