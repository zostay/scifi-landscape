package scene

import (
	"image"
	"math"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Water turns the foreground (everything below the horizon) into an ocean that
// reflects the scene above the horizon — sky, suns, planets, mountains, and the
// city skyline. Not every scene has it. Drawn last, it samples the already-drawn
// pixels above the horizon, mirrors them down with wave-ripple distortion (calm
// and mirror-like near the horizon, choppier and more water-colored toward the
// viewer), and tints them with a water color.
type Water struct{}

func (w *Water) Name() string { return "water" }

const (
	waterChance       = 0.40
	waterAnimDuration = 900 * time.Millisecond

	waterWaveMin  = 1.0  // ripple amplitude (px) at the horizon
	waterWaveFrac = 0.06 // ripple amplitude toward the viewer, as a fraction of foreground height
	waterFreqY    = 0.11 // wave frequency down the image (horizontal crests)
	waterFreqX    = 0.01 // slow variation across the image

	// Fresnel-ish: near the horizon water is mirror-like; toward the viewer it
	// shows more of its own color and darkens.
	waterTintHorizon    = 0.15
	waterTintForeground = 0.62
	waterDarkForeground = 0.35
)

func (wt *Water) Render(c *Context) error {
	if c.Rng.Float64() >= waterChance {
		return nil
	}
	w, h := c.W, c.H
	horizon := c.Settings.HorizonY
	if horizon >= h-2 || horizon < 1 {
		return nil
	}
	groundH := h - horizon

	// Water color: usually blue/teal, occasionally something alien.
	hue := rnd(c.Rng, 180, 245)
	if c.Rng.Float64() < 0.2 {
		hue = c.Rng.Float64() * 360
	}
	wcol := gfx.HSV{H: hue, S: rnd(c.Rng, 0.30, 0.70), V: rnd(c.Rng, 0.20, 0.50)}.RGB()
	seed := c.Rng.Int()
	waveMax := math.Max(waterWaveFrac*float64(groundH), 2)

	bandH := max(groundH/80, 1)
	per := waterAnimDuration / time.Duration((groundH+bandH-1)/bandH)

	for y0 := horizon + 1; y0 < h; y0 += bandH {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y1 := min(y0+bandH, h)
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				d := float64(y-horizon) / float64(groundH) // 0 at the horizon, 1 at the bottom
				amp := waterWaveMin + (waveMax-waterWaveMin)*d
				tint := waterTintHorizon + (waterTintForeground-waterTintHorizon)*d
				dark := 1 - waterDarkForeground*d
				for x := range w {
					// Ripple displacement (mostly per-row, for horizontal crests).
					dx := (gfx.FBM(float64(y)*waterFreqY, float64(x)*waterFreqX, seed, 3) - 0.5) * 2 * amp
					dy := (gfx.FBM(float64(y)*waterFreqY*0.7+10, float64(x)*waterFreqX, seed+5, 2) - 0.5) * amp
					sx := clampInt(x+int(dx), 0, w-1)
					sy := clampInt(2*horizon-y+int(dy), 0, horizon-1) // mirror across the horizon

					off := img.PixOffset(sx, sy)
					rr := float64(img.Pix[off]) / 255
					gg := float64(img.Pix[off+1]) / 255
					bb := float64(img.Pix[off+2]) / 255

					out := gfx.RGB{
						R: (rr + (wcol.R-rr)*tint) * dark,
						G: (gg + (wcol.G-gg)*tint) * dark,
						B: (bb + (wcol.B-bb)*tint) * dark,
					}
					r8, g8, b8, _ := out.RGBA8()
					o := img.PixOffset(x, y)
					img.Pix[o] = r8
					img.Pix[o+1] = g8
					img.Pix[o+2] = b8
					img.Pix[o+3] = 255
				}
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
