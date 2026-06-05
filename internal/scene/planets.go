package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Planets draws planets in the sky. They sit in front of the stars and suns but
// behind the ground (so a planet near the horizon is occluded by the terrain).
// A scene has a ~50% chance of any planets; when present the count is usually
// one to three but can run up to many. Each planet has a type; the first is the
// gas giant, rendered as turbulent latitudinal color bands on a shaded sphere.
type Planets struct{}

func (p *Planets) Name() string { return "planets" }

const (
	planetsAnimDuration = 900 * time.Millisecond

	planetChance    = 0.75 // chance a scene has any planets at all (zero halved)
	planetCountMean = 3.0  // exponential mean of the count when present
	planetMax       = 20

	// Per-planet band rotation, added to the global star angle. Biased toward
	// little rotation, but high rotation is common, up to 90 degrees more.
	planetRotStd = 55.0

	// Atmospheric haze: planets blend toward the sky color toward the horizon in
	// daylight/dusk (and barely at twilight), so low planets fade into the sky.
	planetHazePow      = 2.0
	planetHazeMidday   = 0.85
	planetHazeDusk     = 0.72
	planetHazeTwilight = 0.0

	// Disc size as a fraction of scene width, biased small (squared) with a
	// rare tail up to half the scene width.
	planetMinFrac = 0.004
	planetMaxFrac = 0.50

	// Gas-giant bands.
	planetBandsMin     = 6
	planetBandsMax     = 16
	planetBandContrast = 0.12 // light/dark alternation between adjacent bands
	planetTurbScaleX   = 5.0  // turbulence cells across the disc
	planetTurbScaleLat = 8.0  // turbulence cells top-to-bottom
	planetTurbMin      = 0.04 // band waviness (fraction of latitude)
	planetTurbMax      = 0.14
	planetLimbMin      = 0.42 // brightness at the limb vs the center (sphere shading)
)

// PlanetType selects how a planet is rendered.
type PlanetType int

const (
	GasGiant PlanetType = iota
)

// planet is one resolved planet.
type planet struct {
	cx, cy   int
	r        int
	typ      PlanetType
	bands    gfx.Gradient // latitude (0=top, 1=bottom) -> color
	turbSeed int
	turbAmp  float64
	rotation float64 // band tilt in radians
}

func (p *Planets) Render(c *Context) error {
	n := planetCount(c.Rng)
	if n == 0 {
		return nil
	}

	w, h := c.W, c.H
	planets := make([]planet, n)
	for i := range planets {
		planets[i] = makePlanet(c.Rng, w, c.Settings)
	}

	// Precompute, per row, the sky color planets blend toward and how much
	// (haze): planets fade into the sky near the horizon in daylight/dusk.
	horizon := c.Settings.HorizonY
	th := planetTimeHaze(c.Settings.Time)
	skyRow := make([]gfx.RGB, h)
	hazeRow := make([]float64, h)
	for y := range h {
		skyRow[y] = skyColorAt(c.SkyGradient, y, horizon, h).RGB()
		hazeRow[y] = th * math.Pow(math.Min(float64(y)/float64(horizon), 1), planetHazePow)
	}

	per := planetsAnimDuration / time.Duration(n)
	for i := range planets {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		pl := planets[i]
		c.Canvas.Draw(func(img *image.RGBA) {
			drawPlanet(img, w, h, pl, skyRow, hazeRow)
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// planetTimeHaze is the maximum atmospheric haze for the time of day: strong in
// daylight, less at dusk, none at twilight (clear dark sky).
func planetTimeHaze(t TimeOfDay) float64 {
	switch t {
	case Dusk:
		return planetHazeDusk
	case Twilight:
		return planetHazeTwilight
	default:
		return planetHazeMidday
	}
}

// planetCount returns 0 with probability 1-planetChance; otherwise an
// exponentially distributed count (mean planetCountMean) shifted to start at 1
// and clamped to planetMax, so one-to-three is typical with a tail up to many.
func planetCount(rng *rand.Rand) int {
	if rng.Float64() >= planetChance {
		return 0
	}
	c := -planetCountMean * math.Log(1-rng.Float64())
	return min(max(1+int(c), 1), planetMax)
}

// makePlanet resolves a planet's type, size, position, and look. Planets sit in
// the sky (between the top and the horizon); large ones may dip toward the
// horizon and be partly occluded by the later ground layer.
func makePlanet(rng *rand.Rand, w int, set Settings) planet {
	t := math.Min(math.Abs(rng.NormFloat64())/3, 1)
	frac := planetMinFrac + (planetMaxFrac-planetMinFrac)*t*t
	r := max(int(frac*float64(w)/2), 2)

	// Band tilt: the global star angle plus a per-planet offset of up to 90
	// degrees, biased low but with high rotations common.
	delta := math.Min(math.Abs(rng.NormFloat64())*planetRotStd, 90)
	rotation := (set.TwinkleAngle + delta) * math.Pi / 180

	return planet{
		cx:       rng.Intn(w),
		cy:       rng.Intn(set.HorizonY + 1),
		r:        r,
		typ:      GasGiant,
		bands:    buildGasGiantBands(rng),
		turbSeed: rng.Int(),
		turbAmp:  rnd(rng, planetTurbMin, planetTurbMax),
		rotation: rotation,
	}
}

// buildGasGiantBands builds the latitude->color gradient for a gas giant. A
// scheme is chosen for the palette: similar hues (Neptune-like), moderately
// variable (Jupiter-like), or fantastic. Adjacent bands alternate lighter and
// darker so the banding reads even when hues are close.
func buildGasGiantBands(rng *rand.Rand) gfx.Gradient {
	baseHue := rng.Float64() * 360

	var hueSpread, baseSat, baseVal float64
	switch r := rng.Float64(); {
	case r < 0.45: // similar — bands of one color (e.g. Neptune's blues)
		hueSpread, baseSat, baseVal = rnd(rng, 3, 12), rnd(rng, 0.35, 0.70), rnd(rng, 0.50, 0.80)
	case r < 0.80: // variable — Jupiter-like spread
		hueSpread, baseSat, baseVal = rnd(rng, 15, 40), rnd(rng, 0.40, 0.75), rnd(rng, 0.45, 0.78)
	default: // fantastic — anything goes
		hueSpread, baseSat, baseVal = rnd(rng, 60, 170), rnd(rng, 0.50, 0.95), rnd(rng, 0.40, 0.85)
	}

	nb := planetBandsMin + rng.Intn(planetBandsMax-planetBandsMin+1)
	spacing := 1.0 / float64(nb)
	grad := make(gfx.Gradient, nb+1)
	for i := range grad {
		pos := float64(i) * spacing
		if i > 0 && i < nb {
			pos += rnd(rng, -0.4, 0.4) * spacing // uneven band widths, still ordered
		}
		alt := planetBandContrast
		if i%2 == 1 {
			alt = -planetBandContrast
		}
		grad[i] = gfx.Stop{Pos: pos, Col: gfx.HSV{
			H: baseHue + rnd(rng, -hueSpread, hueSpread),
			S: clamp01(baseSat + rnd(rng, -0.15, 0.15)),
			V: clamp01(baseVal + alt + rnd(rng, -0.07, 0.07)),
		}}
	}
	return grad
}

func drawPlanet(img *image.RGBA, w, h int, p planet, skyRow []gfx.RGB, hazeRow []float64) {
	switch p.typ {
	case GasGiant:
		drawGasGiant(img, w, h, p, skyRow, hazeRow)
	}
}

// drawGasGiant renders the banded sphere. For each disc pixel it rotates into
// band space, maps the position to a latitude (compressed toward the poles like
// a sphere), perturbs it with turbulence so the bands waver, samples the band
// gradient, and applies limb darkening so the disc reads as round. The color is
// then blended toward the sky color by the per-row haze (so low planets fade
// into the sky), and the rim is feathered.
func drawGasGiant(img *image.RGBA, w, h int, p planet, skyRow []gfx.RGB, hazeRow []float64) {
	rf := float64(p.r)
	cs, sn := math.Cos(p.rotation), math.Sin(p.rotation)
	for oy := -p.r; oy <= p.r; oy++ {
		yy := p.cy + oy
		if yy < 0 || yy >= h {
			continue
		}
		ny := float64(oy) / rf
		for ox := -p.r; ox <= p.r; ox++ {
			xx := p.cx + ox
			if xx < 0 || xx >= w {
				continue
			}
			nx := float64(ox) / rf
			rr := nx*nx + ny*ny
			if rr > 1 {
				continue
			}

			// Feather the last pixel of the rim for a clean edge.
			a := 1.0
			if dist := math.Sqrt(rr) * rf; dist > rf-1 {
				a = rf - dist
			}
			if a <= 0 {
				continue
			}

			// Rotate into band space so the bands tilt with the planet.
			by := -nx*sn + ny*cs
			bx := nx*cs + ny*sn
			lat := (math.Asin(clamp(by, -1, 1)) + math.Pi/2) / math.Pi
			tb := gfx.FBM((bx+1)*planetTurbScaleX, (by+1)*planetTurbScaleLat, p.turbSeed, 3)
			latp := clamp(lat+(tb-0.5)*p.turbAmp, 0, 1)

			col := p.bands.At(latp)
			z := math.Sqrt(math.Max(1-rr, 0)) // sphere normal toward viewer
			col.V *= planetLimbMin + (1-planetLimbMin)*z

			// Fade toward the sky color near the horizon (atmospheric haze).
			pr := col.RGB()
			sky := skyRow[yy]
			hz := hazeRow[yy]
			final := gfx.RGB{
				R: pr.R + (sky.R-pr.R)*hz,
				G: pr.G + (sky.G-pr.G)*hz,
				B: pr.B + (sky.B-pr.B)*hz,
			}
			blendPixel(img, w, h, xx, yy, final, a)
		}
	}
}

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }
func clamp01(v float64) float64       { return clamp(v, 0, 1) }
