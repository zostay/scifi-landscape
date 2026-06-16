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
		{HSV{0, 1, 1}, RGB{1, 0, 0}},    // red
		{HSV{120, 1, 1}, RGB{0, 1, 0}},  // green
		{HSV{240, 1, 1}, RGB{0, 0, 1}},  // blue
		{HSV{0, 0, 1}, RGB{1, 1, 1}},    // white
		{HSV{0, 0, 0}, RGB{0, 0, 0}},    // black
		{HSV{360, 1, 1}, RGB{1, 0, 0}},  // hue wraps
		{HSV{-120, 1, 1}, RGB{0, 0, 1}}, // negative hue wraps
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

// Gradients interpolate in RGB, so a stop between two distant hues blends
// through a desaturated mix rather than sweeping the hue wheel (no rainbow).
func TestGradientBlendsThroughMixNotRainbow(t *testing.T) {
	g := Gradient{
		{Pos: 0, Col: HSV{60, 1, 1}},  // yellow
		{Pos: 1, Col: HSV{240, 1, 1}}, // blue
	}
	mid := g.At(0.5)
	// The RGB midpoint of yellow and blue is a grayish mix (low saturation),
	// never a vivid intermediate hue like green or magenta.
	if mid.S > 0.5 {
		t.Errorf("midpoint saturation %.2f too high; expected a desaturated mix", mid.S)
	}
}

func TestRGBHSVRoundTrip(t *testing.T) {
	for _, h := range []float64{0, 45, 120, 210, 300} {
		in := HSV{H: h, S: 0.8, V: 0.7}
		got := in.RGB().HSV()
		if !close(got.S, 0.8) || !close(got.V, 0.7) {
			t.Errorf("round trip S/V: in %v got %v", in, got)
		}
	}
}

func close(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
