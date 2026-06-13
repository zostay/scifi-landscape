package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// SystemStars draws the local sun(s) of the system — the bright stars belonging
// to this world. They appear only in daylight and at dusk (never twilight).
// There are 0-5 of them, usually 0 or 1, with higher counts much rarer at dusk.
// Each has its own color and size (mostly small, sun-like, up to 20% of the sky
// width). A soft circular glow brightens the sky around each one before the
// disc is drawn; small ones also get a twinkle cross at the global angle. At
// dusk the suns sit on or near the horizon, like a setting sun.
type SystemStars struct{}

func (s *SystemStars) Name() string { return "system stars" }

// Schemas lists the entity schema keys the system-stars element owns.
func (s *SystemStars) Schemas() []string { return []string{SchemaSystemStarV0} }

const (
	sysAnimPerStar = 180 * time.Millisecond

	sysMaxCount      = 5
	sysSigmaMidday   = 1.35 // |normal|*sigma, rounded -> count; ~27% chance of 2+ suns
	sysSigmaDusk     = 0.67 // smaller at dusk so multiple suns stay much rarer
	sysSigmaTwilight = 0.50 // night: the system's sun(s) appear small and dim, if at all

	sysMinFrac         = 0.010 // smallest sun diameter as a fraction of sky width
	sysMaxFrac         = 0.20  // largest sun diameter (20% of the sky width)
	sysTwilightMaxFrac = 0.030 // night suns are small

	// Night suns are dim: brightness scales their glow and disc.
	sysDimMin = 0.30
	sysDimMax = 0.55

	sysPlusFrac   = 0.025 // suns smaller than this get a twinkle cross
	sysGlowMul    = 6.0   // glow extends to this multiple of the disc radius
	sysGlowPeak   = 0.65  // strongest sky brightening, just outside the disc
	sysCoreWhite  = 0.25  // how far the flat disc color is lifted toward white
	sysFeather    = 0.90  // disc is solid until this fraction of the radius
	sysSpikePadPx = 3     // twinkle spikes reach this far past the disc edge
)

// sun is one resolved system star.
type sun struct {
	cx, cy int
	r      int     // disc radius in pixels
	col    gfx.RGB // base (edge) color; the core is whiter
	plus   bool    // draw a twinkle cross
	bright float64 // scales glow and disc; <1 for dim night suns
}

// Generate resolves the scene's system suns into entities. It performs every sun
// random draw on the element stream and has no side effects (it draws nothing),
// so identical globals always yield an identical scene list. An empty list means
// the scene has no system suns.
func (s *SystemStars) Generate(c *Context) (SceneList, error) {
	n := systemStarCount(c.Rng, c.Settings.Time)
	if n == 0 {
		return nil, nil
	}

	w, h := c.W, c.H
	list := make(SceneList, n)
	for i := range list {
		list[i] = sunToEntity(makeSun(c.Rng, w, h, c.Settings))
	}
	return list, nil
}

// RenderList draws the system-sun entities onto the canvas. It is the only step
// that touches the image and it consumes no randomness, so the same scene list
// always draws the same pixels. The shared twinkle direction comes from the
// scene's global twinkle angle. Entities that are not system suns are an error.
func (s *SystemStars) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}

	w, h := c.W, c.H
	suns := make([]sun, len(list))
	for i, e := range list {
		su, err := entityToSun(e)
		if err != nil {
			return err
		}
		suns[i] = su
	}

	rad := c.Settings.TwinkleAngle * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)

	for i := range suns {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		su := suns[i]
		c.Canvas.Draw(func(img *image.RGBA) {
			drawGlow(img, w, h, su)    // brighten the sky around the sun
			drawSunDisc(img, w, h, su) // the disc itself, white-hot core
			if su.plus {
				drawSunSpikes(img, w, h, su, dx, dy)
			}
		})
		if err := sleep(c.Ctx, sysAnimPerStar); err != nil {
			return err
		}
	}
	return nil
}

// systemStarCount draws the number of suns: |normal|*sigma rounded and clamped
// to [0, 5]. Sigma falls from daylight to dusk to twilight, so multiple suns
// get progressively rarer; at twilight the few that appear are small and dim.
func systemStarCount(rng *rand.Rand, t TimeOfDay) int {
	sigma := sysSigmaMidday
	switch t {
	case Dusk:
		sigma = sysSigmaDusk
	case Twilight:
		sigma = sysSigmaTwilight
	}
	n := int(math.Round(math.Abs(rng.NormFloat64()) * sigma))
	return min(max(n, 0), sysMaxCount)
}

// makeSun resolves one sun's size, color, position, and brightness. Size is
// biased small (most look about like Earth's sun) with a long tail to 20% of
// the sky width. At twilight the sun is small and dim, scattered in the night
// sky like a faint distant star.
func makeSun(rng *rand.Rand, w, h int, set Settings) sun {
	var frac, bright float64
	if set.Time == Twilight {
		frac = rnd(rng, sysMinFrac, sysTwilightMaxFrac)
		bright = rnd(rng, sysDimMin, sysDimMax)
	} else {
		// t in [0,1], biased toward 0; squared so most suns are small.
		t := math.Min(math.Abs(rng.NormFloat64())/3, 1)
		frac = sysMinFrac + (sysMaxFrac-sysMinFrac)*t*t
		bright = 1.0
	}
	r := max(int(frac*float64(w)/2), 2)

	col := gfx.HSV{H: rng.Float64() * 360, S: rnd(rng, 0.25, 0.8), V: 1.0}.RGB()

	cx := rng.Intn(w)
	var cy int
	if set.Time == Dusk {
		// Low in the sky: biased near the horizon, but wandering up to about a
		// quarter of the sky, and sometimes dipping just under the horizon.
		sky := float64(set.HorizonY)
		frac := rng.Float64()*rng.Float64()*0.33 - 0.04 // [-0.04, 0.29), biased low
		cy = set.HorizonY - int(frac*sky)
	} else {
		// Up in the sky, between the top margin and just shy of the horizon.
		top := int(0.05 * float64(h))
		bot := max(int(0.9*float64(set.HorizonY)), top+1)
		cy = top + rng.Intn(bot-top)
	}

	return sun{cx: cx, cy: cy, r: r, col: col, plus: frac < sysPlusFrac, bright: bright}
}

// drawGlow adds a soft radial brightening of the sun's color around the disc.
func drawGlow(img *image.RGBA, w, h int, s sun) {
	gr := int(float64(s.r) * sysGlowMul)
	if gr < 1 {
		return
	}
	for oy := -gr; oy <= gr; oy++ {
		for ox := -gr; ox <= gr; ox++ {
			d := math.Sqrt(float64(ox*ox+oy*oy)) / float64(gr)
			if d >= 1 {
				continue
			}
			// Gentle falloff (exponent < 2) so the glow spreads wide.
			inten := math.Pow(1-d, 1.5) * sysGlowPeak * s.bright
			addPixel(img, w, h, s.cx+ox, s.cy+oy, s.col, inten)
		}
	}
}

// drawSunDisc draws a flat, blindingly bright disc: a single solid color (the
// sun's color lifted slightly toward white) across the whole disc, with just a
// quick fade at the very edge. Suns read as uniformly blinding, not centrally
// bright.
func drawSunDisc(img *image.RGBA, w, h int, s sun) {
	// Lift toward white (less so when dim), then scale the whole disc by
	// brightness so night suns are dim.
	lift := sysCoreWhite * s.bright
	fill := gfx.RGB{
		R: (s.col.R + (1-s.col.R)*lift) * s.bright,
		G: (s.col.G + (1-s.col.G)*lift) * s.bright,
		B: (s.col.B + (1-s.col.B)*lift) * s.bright,
	}
	r2 := s.r * s.r
	for oy := -s.r; oy <= s.r; oy++ {
		for ox := -s.r; ox <= s.r; ox++ {
			if ox*ox+oy*oy > r2 {
				continue
			}
			a := 1.0
			if d := math.Sqrt(float64(ox*ox+oy*oy)) / float64(s.r); d > sysFeather {
				a = (1 - d) / (1 - sysFeather)
			}
			blendPixel(img, w, h, s.cx+ox, s.cy+oy, fill, a)
		}
	}
}

// drawSunSpikes draws a four-spoke twinkle cross for small suns, reaching just
// past the disc and fading toward the tips, along the shared twinkle axes.
func drawSunSpikes(img *image.RGBA, w, h int, s sun, dx, dy float64) {
	end := s.r + s.r + sysSpikePadPx
	for t := s.r; t <= end; t++ {
		a := 0.85 * s.bright * (1 - float64(t-s.r)/float64(end-s.r+1))
		fx, fy := dx*float64(t), dy*float64(t)
		px, py := -dy*float64(t), dx*float64(t)
		blendPixel(img, w, h, s.cx+round(fx), s.cy+round(fy), s.col, a)
		blendPixel(img, w, h, s.cx-round(fx), s.cy-round(fy), s.col, a)
		blendPixel(img, w, h, s.cx+round(px), s.cy+round(py), s.col, a)
		blendPixel(img, w, h, s.cx-round(px), s.cy-round(py), s.col, a)
	}
}

// addPixel adds color c at the given intensity to the existing pixel (a screen-
// like brightening used for the glow), clamping each channel.
func addPixel(img *image.RGBA, w, h, x, y int, c gfx.RGB, inten float64) {
	if inten <= 0 || x < 0 || y < 0 || x >= w || y >= h {
		return
	}
	off := img.PixOffset(x, y)
	img.Pix[off] = addChannel(img.Pix[off], c.R*inten)
	img.Pix[off+1] = addChannel(img.Pix[off+1], c.G*inten)
	img.Pix[off+2] = addChannel(img.Pix[off+2], c.B*inten)
	img.Pix[off+3] = 255
}

func addChannel(b uint8, add float64) uint8 {
	v := float64(b)/255 + add
	if v > 1 {
		v = 1
	}
	return uint8(v*255 + 0.5)
}
