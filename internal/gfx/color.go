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

// lerpHue interpolates between two hues along the shorter arc of the color
// wheel, which keeps transitions between adjacent gradient stops looking
// natural (e.g. green->cyan->blue rather than the long way around).
func lerpHue(a, b, t float64) float64 {
	a = wrapHue(a)
	b = wrapHue(b)
	d := math.Mod(b-a+540, 360) - 180 // shortest signed delta in (-180, 180]
	return wrapHue(a + d*t)
}

// Stop is one color stop in a Gradient at a normalized position in [0, 1].
type Stop struct {
	Pos float64
	Col HSV
}

// Gradient is an ordered list of HSV color stops (ascending Pos). Interpolation
// happens in HSV space so the path travels through intermediate hues, which is
// what we want for dusk skies that run yellow -> orange -> red and the like.
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
			return HSV{
				H: lerpHue(a.Col.H, b.Col.H, t),
				S: lerp(a.Col.S, b.Col.S, t),
				V: lerp(a.Col.V, b.Col.V, t),
			}
		}
	}
	return last.Col
}
