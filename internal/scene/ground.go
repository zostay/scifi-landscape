package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Ground is the base terrain layer filling the scene below the horizon. It is a
// vertical gradient with fractal noise baked in so the surface reads as dirt.
// It is built in horizontal bands that are narrow at the horizon and grow
// taller toward the foreground, giving a sense of receding distance. The base
// layer is always drawn (every time of day); later layers will detail it.
type Ground struct{}

func (g *Ground) Name() string { return "ground" }

const (
	groundAnimDuration = 1000 * time.Millisecond

	// Band heights start small at the horizon and grow geometrically toward the
	// foreground, capped so the nearest bands don't become enormous.
	groundMinBand = 1.0
	groundGrowth  = 1.18
	groundMaxBand = 60

	// Noise baked into the gradient. Amplitude grows toward the foreground so
	// near dirt looks coarser/more textured than hazy distance.
	groundOctaves  = 4
	groundValueAmp = 0.45 // value (light/dark) variation
	groundSatAmp   = 0.20 // saturation variation
	groundHueAmp   = 6.0  // hue wobble in degrees

	// Horizontal noise frequency is constant; the vertical sample coordinate is
	// warped (see groundDepthWarp) so the texture is strongly compressed near
	// the horizon — thin, very stretched streaks — easing to near-isotropic
	// dirt toward the foreground.
	groundFreqX      = 0.04
	groundFreqY      = 0.05
	groundStretch    = 7.0 // vertical frequency multiplier at the horizon
	groundStretchPow = 2.0 // how quickly the stretch eases off downward

	// Color mode. In normal mode the ground is one base hue with light/dark and
	// saturation variation. In variable mode the depth gradient runs through
	// several random colors, and a low-frequency noise wanders the gradient
	// lookup so color patches drift back and forth instead of transitioning
	// cleanly — an alien, non-uniform landscape.
	groundVariableChance = 0.45
	groundVarMinColors   = 2
	groundVarMaxColors   = 5
	groundWanderAmp      = 0.28  // how far (in gradient space) patches wander
	groundWanderFreqX    = 0.012 // low horizontal frequency -> broad patches
	groundWanderVScale   = 0.5   // vertical patch frequency, relative to texture
)

// Render generates the scene's base terrain and draws it. It is the Element-level
// entry point used by the build pipeline, and is exactly Generate followed by
// RenderList — generation (all the random draws) cleanly separated from rendering
// (all the drawing), bridged by the ground entity schema.
func (g *Ground) Render(c *Context) error {
	list, err := g.Generate(c)
	if err != nil {
		return err
	}
	return g.RenderList(c, list)
}

// Generate resolves the scene's base terrain into a single entity. It performs
// every ground random draw on the element stream and has no side effects (it
// draws nothing), so identical globals always yield an identical scene list. When
// there is no room for ground below the horizon it returns an empty list and
// draws no randomness, exactly as the original short-circuit did. The ground
// color gradient and variable flag are shared globals read from the Context (not
// generated here), so only the texture/wander seeds are resolved.
func (g *Ground) Generate(c *Context) (SceneList, error) {
	horizon := c.Settings.HorizonY
	h := c.H
	if horizon >= h-1 {
		return nil, nil // no room for ground
	}

	terr := groundTerrain{variable: c.GroundVariable}
	terr.seed = c.Rng.Int()
	if terr.variable {
		terr.wanderSeed = c.Rng.Int()
	}
	return SceneList{groundToEntity(terr)}, nil
}

// RenderList draws the ground entity onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. It reads the shared ground gradient/variable flag from
// the Context (the scene-wide globals built in Scene.Build). An entity that is not
// ground is an error.
func (g *Ground) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	terr, err := entityToGround(list[0])
	if err != nil {
		return err
	}

	horizon := c.Settings.HorizonY
	w, h := c.W, c.H

	variable := c.GroundVariable
	grad := c.GroundGradient
	seed := terr.seed
	wanderSeed := terr.wanderSeed
	span := float64(h - horizon) // ground height in pixels
	vy := groundDepthWarp(h-horizon, span)

	bands := groundBands(horizon, h)
	per := groundAnimDuration / time.Duration(len(bands))

	for _, b := range bands {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y0, y1 := b[0], b[1]
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				t := float64(y-horizon) / span // 0 at horizon, 1 at foreground
				amp := groundValueAmp * (0.5 + 0.8*t)
				for x := range w {
					// In variable mode, wander where in the color gradient this
					// pixel samples, so patches drift back and forth.
					ct := t
					if variable {
						wn := gfx.FBM(float64(x)*groundWanderFreqX, vy[y-horizon]*groundWanderVScale, wanderSeed, 3)
						ct = min(max(t+(wn-0.5)*groundWanderAmp, 0), 1)
					}
					base := grad.At(ct)

					n := gfx.FBM(float64(x)*groundFreqX, vy[y-horizon], seed, groundOctaves)
					d := n - 0.5 // centered noise
					col := gfx.HSV{
						H: base.H + d*groundHueAmp,
						S: base.S * (1 + d*groundSatAmp),
						V: base.V * (1 + d*amp),
					}.RGB()
					r, gg, bb, _ := col.RGBA8()
					off := img.PixOffset(x, y)
					img.Pix[off] = r
					img.Pix[off+1] = gg
					img.Pix[off+2] = bb
					img.Pix[off+3] = 255
				}
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// groundDepthWarp returns, per ground row, the vertical coordinate at which to
// sample the noise. Each row advances the coordinate by groundFreqY times a
// stretch factor that is largest at the horizon (groundStretch) and eases to 1
// toward the foreground. Because the coordinate climbs fast near the horizon,
// noise features there are squeezed into thin, very stretched streaks; lower
// down the steps shrink and the dirt looks natural. rows is the ground height
// in pixels; span is the same value as a float.
func groundDepthWarp(rows int, span float64) []float64 {
	vy := make([]float64, rows)
	acc := 0.0
	for i := range vy {
		tb := float64(i) / span
		m := 1 + (groundStretch-1)*math.Pow(1-tb, groundStretchPow)
		acc += groundFreqY * m
		vy[i] = acc
	}
	return vy
}

// groundBands returns [y0,y1) row ranges from the horizon to the bottom, with
// heights growing geometrically toward the foreground (capped).
func groundBands(horizon, h int) [][2]int {
	var bands [][2]int
	y := horizon
	bh := groundMinBand
	for y < h {
		height := min(max(int(math.Round(bh)), 1), groundMaxBand)
		y1 := min(y+height, h)
		bands = append(bands, [2]int{y, y1})
		y = y1
		bh *= groundGrowth
	}
	return bands
}

// buildGroundGradient builds the horizon->foreground color gradient. In normal
// mode it is one base hue (any color) that is hazier/lighter at the distant
// horizon and darker/more saturated in the foreground. In variable mode it runs
// through a random number of fully random colors for an alien surface. Values
// are scaled by how much light the time of day gives.
func buildGroundGradient(rng *rand.Rand, t TimeOfDay, variable bool) gfx.Gradient {
	b := groundBrightness(t)
	if variable {
		return variableGroundGradient(rng, b)
	}

	hue := rng.Float64() * 360 // any color
	return gfx.Gradient{
		{Pos: 0.0, Col: gfx.HSV{H: hue, S: rnd(rng, 0.15, 0.35), V: rnd(rng, 0.45, 0.68) * b}},
		{Pos: 1.0, Col: gfx.HSV{H: hue + rnd(rng, -8, 8), S: rnd(rng, 0.40, 0.70), V: rnd(rng, 0.20, 0.38) * b}},
	}
}

// variableGroundGradient builds a gradient of 2-5 random color stops from the
// horizon (pos 0) to the foreground (pos 1). Interior stops are evenly spaced
// with bounded jitter (so they stay ordered), giving uneven color band widths.
func variableGroundGradient(rng *rand.Rand, brightness float64) gfx.Gradient {
	n := groundVarMinColors + rng.Intn(groundVarMaxColors-groundVarMinColors+1)
	spacing := 1.0 / float64(n-1)

	grad := make(gfx.Gradient, n)
	for i := range grad {
		pos := float64(i) * spacing
		if i > 0 && i < n-1 {
			pos += rnd(rng, -0.4, 0.4) * spacing // keep within neighbors
		}
		grad[i] = gfx.Stop{
			Pos: pos,
			Col: gfx.HSV{
				H: rng.Float64() * 360,
				S: rnd(rng, 0.40, 0.90),
				V: rnd(rng, 0.40, 0.75) * brightness,
			},
		}
	}
	return grad
}

// groundBrightness scales ground lighting with the time of day.
func groundBrightness(t TimeOfDay) float64 {
	switch t {
	case Dusk:
		return 0.62
	case Twilight:
		return 0.32
	default:
		return 1.0
	}
}
