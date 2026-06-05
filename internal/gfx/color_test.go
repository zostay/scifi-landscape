package gfx

import (
	"math"
	"testing"
)

func TestHSVToRGBPrimaries(t *testing.T) {
	cases := []struct {
		hsv  HSV
		want RGB
	}{
		{HSV{0, 1, 1}, RGB{1, 0, 0}},     // red
		{HSV{120, 1, 1}, RGB{0, 1, 0}},   // green
		{HSV{240, 1, 1}, RGB{0, 0, 1}},   // blue
		{HSV{0, 0, 1}, RGB{1, 1, 1}},     // white
		{HSV{0, 0, 0}, RGB{0, 0, 0}},     // black
		{HSV{360, 1, 1}, RGB{1, 0, 0}},   // hue wraps
		{HSV{-120, 1, 1}, RGB{0, 0, 1}},  // negative hue wraps
	}
	for _, c := range cases {
		got := c.hsv.RGB()
		if !close(got.R, c.want.R) || !close(got.G, c.want.G) || !close(got.B, c.want.B) {
			t.Errorf("%v.RGB() = %v, want %v", c.hsv, got, c.want)
		}
	}
}

func TestGradientClampAndEndpoints(t *testing.T) {
	g := Gradient{
		{Pos: 0, Col: HSV{0, 0, 0}},
		{Pos: 1, Col: HSV{0, 0, 1}},
	}
	if v := g.At(-5).V; v != 0 {
		t.Errorf("At(-5).V = %v, want 0 (clamped to first stop)", v)
	}
	if v := g.At(5).V; v != 1 {
		t.Errorf("At(5).V = %v, want 1 (clamped to last stop)", v)
	}
	if v := g.At(0.5).V; !close(v, 0.5) {
		t.Errorf("At(0.5).V = %v, want 0.5 (midpoint lerp)", v)
	}
}

func TestLerpHueShortestArc(t *testing.T) {
	// 350 -> 10 should cross 0 (the short way), landing near 0 at the midpoint.
	got := lerpHue(350, 10, 0.5)
	if d := math.Min(got, 360-got); d > 1 {
		t.Errorf("lerpHue(350,10,0.5) = %v, want ~0/360 (shortest arc through 0)", got)
	}
}

func close(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
