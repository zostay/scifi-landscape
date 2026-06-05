package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Sky is the bottom-most element: a vertical gradient that fills the whole
// scene. The gradient is anchored at the horizon (brightest there) and darkens
// toward the top. Below the horizon it is mirrored and dimmed, so later
// elements can reuse it as a reflected sky on water; usually it is overwritten.
type Sky struct{}

func (s *Sky) Name() string { return "sky" }

// skyAnimDuration is the wall-clock time the sky takes to wipe in.
const skyAnimDuration = 1100 * time.Millisecond

func (s *Sky) Render(c *Context) error {
	grad := buildSkyGradient(c.Rng, c.Settings.Time)

	w, h := c.W, c.H
	horizon := c.Settings.HorizonY

	// Precompute one color per row: the sky is uniform horizontally.
	rows := make([]gfx.RGB, h)
	for y := range h {
		var col gfx.HSV
		if y <= horizon {
			// 0 at the horizon, 1 at the top of the scene.
			pos := float64(horizon-y) / float64(horizon)
			col = grad.At(pos)
		} else {
			// Mirror the gradient downward and dim it for a reflection.
			mirror := float64(y-horizon) / float64(h-horizon)
			col = grad.At(mirror)
			col.V *= 0.55
			col.S *= 0.9
		}
		rows[y] = col.RGB()
	}

	// Wipe the gradient in top-to-bottom in bands so the build is visible.
	const bands = 100
	bandH := max(h/bands, 1)
	perBand := skyAnimDuration / time.Duration((h+bandH-1)/bandH)

	for y0 := 0; y0 < h; y0 += bandH {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y1 := min(y0+bandH, h)
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				r, g, b, a := rows[y].RGBA8()
				off := img.PixOffset(0, y)
				for range w {
					img.Pix[off] = r
					img.Pix[off+1] = g
					img.Pix[off+2] = b
					img.Pix[off+3] = a
					off += 4
				}
			}
		})
		if err := sleep(c.Ctx, perBand); err != nil {
			return err
		}
	}
	return nil
}

// buildSkyGradient produces the horizon->top color stops for the given time of
// day. Position 0 is the horizon (brightest); position 1 is the top of the sky.
func buildSkyGradient(rng *rand.Rand, t TimeOfDay) gfx.Gradient {
	switch t {
	case Dusk:
		return duskGradient(rng)
	case Twilight:
		return twilightGradient(rng)
	default:
		return middayGradient(rng)
	}
}

func rnd(rng *rand.Rand, lo, hi float64) float64 {
	return lo + rng.Float64()*(hi-lo)
}

// middayGradient: any hue, running from a lighter/less-saturated color at the
// horizon to a darker, fully-saturated color at the top.
func middayGradient(rng *rand.Rand) gfx.Gradient {
	hue := rng.Float64() * 360
	drift := rnd(rng, -40, 40) // slight hue shift toward the top

	return gfx.Gradient{
		{Pos: 0.0, Col: gfx.HSV{H: hue, S: rnd(rng, 0.18, 0.36), V: rnd(rng, 0.88, 0.98)}},
		{Pos: 1.0, Col: gfx.HSV{H: hue + drift, S: rnd(rng, 0.80, 1.0), V: rnd(rng, 0.32, 0.54)}},
	}
}

// duskPatterns are warm-to-X hue journeys (in degrees), each ending dark.
var duskPatterns = [][]float64{
	{52, 28, 5},    // yellow -> orange -> red
	{52, 290, 325}, // yellow -> purple -> pink
	{55, 120, 225}, // yellow -> green -> blue
	{50, 15, 320},  // yellow -> red -> magenta
	{48, 200, 255}, // yellow -> teal -> indigo
}

// duskGradient: a wild, multi-stop sky that starts warm and bright at the
// horizon, runs through a range of hues, and trends to low value (near black)
// at the top.
func duskGradient(rng *rand.Rand) gfx.Gradient {
	hues := append([]float64(nil), duskPatterns[rng.Intn(len(duskPatterns))]...)
	for i := range hues {
		hues[i] += rnd(rng, -12, 12)
	}
	n := len(hues)

	grad := gfx.Gradient{
		{Pos: 0.0, Col: gfx.HSV{H: hues[0], S: rnd(rng, 0.85, 1.0), V: rnd(rng, 0.90, 0.98)}},
	}
	for i := 1; i < n; i++ {
		pos := 0.18 + 0.55*float64(i)/float64(n-1)
		v := 0.75 - 0.18*float64(i)
		if v < 0.15 {
			v = 0.15
		}
		grad = append(grad, gfx.Stop{Pos: pos, Col: gfx.HSV{H: hues[i], S: rnd(rng, 0.85, 1.0), V: v}})
	}

	// Usually fade to black at the very top; occasionally keep a faint glow.
	topV := 0.0
	if rng.Float64() < 0.25 {
		topV = rnd(rng, 0.05, 0.13)
	}
	grad = append(grad, gfx.Stop{Pos: 1.0, Col: gfx.HSV{H: hues[n-1], S: 0.9, V: topV}})
	return grad
}

// twilightGradient: mostly dark. A dim color near the horizon fades to black,
// then either stays black or picks up a faint second color toward the top.
func twilightGradient(rng *rand.Rand) gfx.Gradient {
	hue := rng.Float64() * 360
	grad := gfx.Gradient{
		{Pos: 0.0, Col: gfx.HSV{H: hue, S: rnd(rng, 0.40, 0.85), V: rnd(rng, 0.14, 0.36)}},
	}

	fadeEnd := rnd(rng, 0.45, 0.75)
	grad = append(grad, gfx.Stop{Pos: fadeEnd, Col: gfx.HSV{H: hue, S: rnd(rng, 0.3, 0.7), V: 0.02}})

	if rng.Float64() < 0.5 {
		h2 := math.Mod(hue+rnd(rng, 60, 300), 360)
		grad = append(grad, gfx.Stop{Pos: 1.0, Col: gfx.HSV{H: h2, S: rnd(rng, 0.5, 0.9), V: rnd(rng, 0.05, 0.17)}})
	} else {
		grad = append(grad, gfx.Stop{Pos: 1.0, Col: gfx.HSV{H: hue, S: 0.3, V: 0.01}})
	}
	return grad
}
