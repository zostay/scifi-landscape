package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Mountains draws a mountain range along the horizon (every scene has one).
// A height and a smoothness shape it: smoothness picks how many key points the
// ridge has (high = few, gentle curves; low = many, plus heavy noise for jagged
// peaks), and height scales the peaks. The combinations span rolling hills,
// Rockies/Alps, low buttes, and jagged airless ridges. The range is colored by
// a gradient from a ground-like base up to light/white, normalized by absolute
// altitude so only genuinely tall peaks turn white (snow caps).
type Mountains struct{}

func (m *Mountains) Name() string { return "mountains" }

// Schemas lists the entity schema keys the mountains element owns.
func (m *Mountains) Schemas() []string { return []string{SchemaMountainsV0} }

const (
	mountainsAnimDuration = 700 * time.Millisecond
	mountainsAnimCols     = 90 // animation column-batches

	// Peak height as a fraction of the sky: |normal|*scale, so it averages ~10%
	// and rarely reaches the 50% cap.
	mountainHeightScale = 0.13
	mountainHeightMax   = 0.50

	mountainMinPoints = 4  // key points at high smoothness
	mountainMaxPoints = 44 // key points at low smoothness

	mountainNoiseFreq = 0.035
	mountainNoiseOct  = 4
	mountainNoiseAmp  = 0.55 // jaggedness (× height) at zero smoothness
	mountainTexFreq   = 0.06
	mountainTexAmp    = 0.10 // surface value mottle
)

// Generate resolves the scene's mountain range into a single entity. It performs
// every mountain random draw on the element stream, in the original order, and
// has no side effects (it draws nothing), so identical globals always yield an
// identical scene list. An empty list means there is no sky to rise into.
//
// Mountains are always drawn when there is room. (Each element has its own random
// stream, so there is nothing to keep in sync by drawing a presence roll first.)
func (m *Mountains) Generate(c *Context) (SceneList, error) {
	w := c.W
	horizon := c.Settings.HorizonY
	if horizon < 4 {
		return nil, nil // no sky to rise into
	}
	sky := float64(horizon)
	// Coloring is normalized by the largest possible mountain, so short ranges
	// stay ground-colored and only tall ones reach the white top of the gradient.
	maxAlt := mountainHeightMax * sky

	smoothness := c.Rng.Float64()
	heightPx := math.Min(math.Abs(c.Rng.NormFloat64())*mountainHeightScale, mountainHeightMax) * sky
	hmap := mountainHeights(c.Rng, w, smoothness, heightPx)
	grad := buildMountainGradient(c.Rng, c.GroundGradient)
	texSeed := c.Rng.Int()

	return SceneList{mountainsToEntity(mountainRange{
		heights: hmap,
		grad:    grad,
		texSeed: texSeed,
		maxAlt:  maxAlt,
	})}, nil
}

// RenderList draws the mountain entity onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. An empty list draws nothing; a non-mountain entity is an
// error.
func (m *Mountains) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	mr, err := entityToMountains(list[0])
	if err != nil {
		return err
	}

	w, h := c.W, c.H
	horizon := c.Settings.HorizonY
	hmap := mr.heights
	grad := mr.grad
	texSeed := mr.texSeed
	maxAlt := mr.maxAlt

	batch := max(w/mountainsAnimCols, 1)
	per := mountainsAnimDuration / time.Duration((w+batch-1)/batch)

	for x0 := 0; x0 < w; x0 += batch {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		x1 := min(x0+batch, w)
		c.Canvas.Draw(func(img *image.RGBA) {
			for x := x0; x < x1; x++ {
				drawMountainColumn(img, w, h, x, horizon, hmap[x], maxAlt, grad, texSeed)
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// mountainHeights builds the per-column ridge height (in pixels) from key points
// interpolated with smoothstep, plus noise. Smoothness sets the point count and
// the noise amplitude: high smoothness gives few points and gentle curves, low
// gives many points and a jagged, noisy ridge.
func mountainHeights(rng *rand.Rand, w int, smoothness, heightPx float64) []float64 {
	np := max(int(math.Round(mountainMinPoints+(1-smoothness)*(mountainMaxPoints-mountainMinPoints))), 1)
	pts := make([]float64, np+1)
	for i := range pts {
		pts[i] = rng.Float64() // peak height fraction at each key point
	}
	noiseSeed := rng.Int()
	noiseAmp := (1 - smoothness) * mountainNoiseAmp * heightPx

	seg := 1.0 / float64(len(pts)-1)
	hmap := make([]float64, w)
	for x := range w {
		u := float64(x) / float64(max(w-1, 1))
		i := min(int(u/seg), len(pts)-2)
		t := (u - float64(i)*seg) / seg
		t = t * t * (3 - 2*t) // smoothstep between key points
		base := (pts[i] + (pts[i+1]-pts[i])*t) * heightPx
		n := (gfx.FBM(float64(x)*mountainNoiseFreq, 7, noiseSeed, mountainNoiseOct) - 0.5) * 2 * noiseAmp
		hmap[x] = math.Max(base+n, 0)
	}
	return hmap
}

// drawMountainColumn fills one column of the range from the horizon up to its
// peak, anti-aliasing the silhouette's top edge and shading by absolute altitude.
func drawMountainColumn(img *image.RGBA, w, h, x, horizon int, hcol, maxAlt float64, grad gfx.Gradient, texSeed int) {
	top := horizon - int(math.Ceil(hcol)) - 1
	for y := horizon - 1; y >= top && y >= 0; y-- {
		alt := float64(horizon - y)
		cov := clamp(hcol-alt+0.5, 0, 1) // coverage: 1 inside, feathered at the top edge
		if cov <= 0 {
			continue
		}
		col := grad.At(clamp(alt/maxAlt, 0, 1))
		col.V *= 1 + (gfx.FBM(float64(x)*mountainTexFreq, float64(y)*mountainTexFreq, texSeed, 3)-0.5)*mountainTexAmp
		blendPixel(img, w, h, x, y, col.RGB(), cov)
	}
}

// buildMountainGradient builds the base->peak color gradient: a ground-like base
// (from the shared ground gradient, a bit darker), through a lighter rock mid
// tone, up to a near-white peak (snow / lit ridges).
func buildMountainGradient(rng *rand.Rand, ground gfx.Gradient) gfx.Gradient {
	gb := ground.At(0) // ground color at the horizon
	bottom := gfx.HSV{H: gb.H, S: gb.S * 0.9, V: gb.V * 0.7}
	mid := gfx.HSV{H: gb.H + rnd(rng, -25, 25), S: gb.S * rnd(rng, 0.4, 0.7), V: clamp01(gb.V*0.85 + 0.2)}
	top := gfx.HSV{H: gb.H, S: rnd(rng, 0.02, 0.12), V: rnd(rng, 0.90, 1.0)}
	return gfx.Gradient{
		{Pos: 0, Col: bottom},
		{Pos: rnd(rng, 0.5, 0.7), Col: mid},
		{Pos: 1, Col: top},
	}
}
