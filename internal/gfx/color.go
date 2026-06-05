// Package gfx provides small color helpers used by the scene renderer:
// floating-point RGB/HSV colors, conversions, and gradient interpolation.
//
// Working in floating point (and especially HSV) keeps the sky-generation
// math readable: we pick hues, saturations, and values directly and only
// quantize to 8-bit at the moment we touch the canvas.
package gfx

import "math"

// RGB is a linear-ish RGB color with each channel in [0, 1].
type RGB struct {
	R, G, B float64
}

// HSV is a color in hue/saturation/value form. H is in degrees [0, 360);
// S and V are in [0, 1].
type HSV struct {
	H, S, V float64
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// wrapHue normalizes a hue into [0, 360).
func wrapHue(h float64) float64 {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	return h
}

// RGB converts an HSV color to RGB.
func (c HSV) RGB() RGB {
	h := wrapHue(c.H) / 60
	s := clamp01(c.S)
	v := clamp01(c.V)

	i := math.Floor(h)
	f := h - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	switch int(i) % 6 {
	case 0:
		return RGB{v, t, p}
	case 1:
		return RGB{q, v, p}
	case 2:
		return RGB{p, v, t}
	case 3:
		return RGB{p, q, v}
	case 4:
		return RGB{t, p, v}
	default:
		return RGB{v, p, q}
	}
}

// RGBA8 quantizes a color to 8-bit channels with a fully opaque alpha.
func (c RGB) RGBA8() (r, g, b, a uint8) {
	return to8(c.R), to8(c.G), to8(c.B), 255
}

func to8(v float64) uint8 {
	return uint8(clamp01(v)*255 + 0.5)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// HSV converts an RGB color to HSV (H in [0,360), S and V in [0,1]).
func (c RGB) HSV() HSV {
	r, g, b := clamp01(c.R), clamp01(c.G), clamp01(c.B)
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	d := mx - mn

	v := mx
	var s float64
	if mx > 0 {
		s = d / mx
	}
	var h float64
	if d > 0 {
		switch mx {
		case r:
			h = math.Mod((g-b)/d, 6)
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h *= 60
		if h < 0 {
			h += 360
		}
	}
	return HSV{H: h, S: s, V: v}
}

// Stop is one color stop in a Gradient at a normalized position in [0, 1].
type Stop struct {
	Pos float64
	Col HSV
}

// Gradient is an ordered list of HSV color stops (ascending Pos). Interpolation
// happens in RGB space (not by sweeping the hue wheel): the stop colors stay
// vivid, but transitions blend through a mix rather than running through the
// whole rainbow, which would look psychedelic.
type Gradient []Stop

// At samples the gradient at normalized position p (clamped to [0, 1]).
func (g Gradient) At(p float64) HSV {
	if len(g) == 0 {
		return HSV{}
	}
	if p <= g[0].Pos {
		return g[0].Col
	}
	last := g[len(g)-1]
	if p >= last.Pos {
		return last.Col
	}
	for i := 1; i < len(g); i++ {
		if p <= g[i].Pos {
			a, b := g[i-1], g[i]
			span := b.Pos - a.Pos
			if span <= 0 {
				return b.Col
			}
			t := (p - a.Pos) / span
			ca, cb := a.Col.RGB(), b.Col.RGB()
			return RGB{
				R: lerp(ca.R, cb.R, t),
				G: lerp(ca.G, cb.G, t),
				B: lerp(ca.B, cb.B, t),
			}.HSV()
		}
	}
	return last.Col
}
