package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Stars scatters points of light across the sky. It renders nothing at midday.
// At twilight stars are bright over the whole scene; at dusk they fade toward
// the bottom (into the still-bright horizon) via alpha. Most stars are single
// pixels, a few are tiny discs, and a rare few are discs with twinkle spikes
// whose shared angle comes from the global settings.
type Stars struct{}

func (s *Stars) Name() string { return "stars" }

const (
	starsAnimDuration = 900 * time.Millisecond
	starsAnimBatches  = 80

	// earthlikeArea is how many pixels share one star at earthlike density.
	earthlikeArea = 1400.0
)

// star is one resolved star ready to draw.
type star struct {
	x, y   int
	col    gfx.RGB
	alpha  float64 // brightness * time-of-day fade
	radius int     // 0 = single pixel
	spikes bool    // draw twinkle cross
	spike  int     // spike length in pixels (when spikes)
}

// Generate resolves the scene's star field into entities. It performs every star
// random draw on the element stream and has no side effects (it draws nothing),
// so identical globals always yield an identical scene list. An empty list means
// the scene has no stars (midday, or no stars survived the brightness floor).
func (s *Stars) Generate(c *Context) (SceneList, error) {
	if c.Settings.Time == Midday {
		return nil, nil // no stars in daylight
	}
	stars := s.generate(c.Rng, c.W, c.H, c.Settings)
	if len(stars) == 0 {
		return nil, nil
	}
	list := make(SceneList, len(stars))
	for i, st := range stars {
		list[i] = starToEntity(st)
	}
	return list, nil
}

// RenderList draws the star entities onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. The shared twinkle direction comes from the scene's
// global twinkle angle. Entities that are not stars are an error.
func (s *Stars) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}

	w, h := c.W, c.H
	stars := make([]star, len(list))
	for i, e := range list {
		st, err := entityToStar(e)
		if err != nil {
			return err
		}
		stars[i] = st
	}

	// Shared twinkle direction for every star in the scene.
	rad := c.Settings.TwinkleAngle * math.Pi / 180
	dx, dy := math.Cos(rad), math.Sin(rad)

	// Draw in batches so the field twinkles in rather than appearing at once.
	batch := max(len(stars)/starsAnimBatches, 1)
	per := starsAnimDuration / time.Duration((len(stars)+batch-1)/batch)

	for i0 := 0; i0 < len(stars); i0 += batch {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		i1 := min(i0+batch, len(stars))
		c.Canvas.Draw(func(img *image.RGBA) {
			for _, st := range stars[i0:i1] {
				drawStar(img, w, h, st, dx, dy)
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// generate builds the star list for the scene. Position, color, brightness, and
// shape all come from rng, so the field is reproducible from the seed.
func (s *Stars) generate(rng *rand.Rand, w, h int, set Settings) []star {
	count := starCount(w, h, set.StarDensity)
	stars := make([]star, 0, count)

	for range count {
		x := rng.Intn(w)
		y := rng.Intn(h)

		// Brightness biased dim (most stars faint, a few bright). The low floor
		// allows small, dim stars at night.
		bright := 0.06 + 0.94*math.Pow(rng.Float64(), 1.9)

		// Dusk fades stars toward the bottom; twilight is uniform.
		fade := 1.0
		if set.Time == Dusk {
			fade = 1 - float64(y)/float64(h)
		}
		alpha := bright * fade
		if alpha < 0.02 {
			continue // too faint to see; skip the work
		}

		st := star{x: x, y: y, col: starColor(rng), alpha: alpha}

		switch r := rng.Float64(); {
		case r < 0.82:
			st.radius = 0 // single pixel — the common case
		case r < 0.97:
			st.radius = 1 // tiny disc
		default:
			st.radius = 1 + rng.Intn(2) // 1-2 px disc...
			st.spikes = true            // ...with a twinkle cross (rare)
			st.spike = 3 + rng.Intn(2)  // 3-4 px spikes
		}
		stars = append(stars, st)
	}
	return stars
}

// starCount converts a density multiplier into a star count for the scene,
// capped to keep dense clusters from becoming absurd.
func starCount(w, h int, density float64) int {
	n := int(float64(w*h) / earthlikeArea * density)
	return min(max(n, 0), w*h/20)
}

// starColor picks a stellar tint: mostly near-white (low saturation), split
// between cool blue-white and warm yellow/orange/red. Value is full; apparent
// brightness is applied via alpha when drawing.
func starColor(rng *rand.Rand) gfx.RGB {
	sat := rng.Float64()
	sat = sat * sat * 0.6 // bias toward 0 (white)

	var hue float64
	if rng.Float64() < 0.55 {
		hue = rnd(rng, 200, 240) // blue-white
	} else {
		hue = rnd(rng, 0, 60) // yellow -> orange -> red
	}
	return gfx.HSV{H: hue, S: sat, V: 1.0}.RGB()
}

// drawStar renders one star onto img. dx,dy is the shared twinkle direction.
//
// Stars are emissive, so they are drawn additively (brighten-only): a star can
// only add light to the sky behind it, never darken it. This keeps a
// saturated-but-bright star color from compositing into a dull, dark dot when
// it sits over a brighter part of the gradient.
func drawStar(img *image.RGBA, w, h int, st star, dx, dy float64) {
	if st.radius == 0 {
		addPixel(img, w, h, st.x, st.y, st.col, st.alpha)
	} else {
		// Filled disc, intensity tapering slightly toward the edge.
		r2 := st.radius * st.radius
		for oy := -st.radius; oy <= st.radius; oy++ {
			for ox := -st.radius; ox <= st.radius; ox++ {
				d2 := ox*ox + oy*oy
				if d2 > r2 {
					continue
				}
				a := st.alpha * (1 - 0.4*float64(d2)/float64(r2+1))
				addPixel(img, w, h, st.x+ox, st.y+oy, st.col, a)
			}
		}
	}

	if !st.spikes {
		return
	}
	// Four spokes along the shared twinkle axes, fading toward the tips.
	for t := 1; t <= st.spike; t++ {
		a := st.alpha * (1 - float64(t)/float64(st.spike+1))
		fx, fy := dx*float64(t), dy*float64(t)
		px, py := -dy*float64(t), dx*float64(t) // perpendicular axis
		addPixel(img, w, h, st.x+round(fx), st.y+round(fy), st.col, a)
		addPixel(img, w, h, st.x-round(fx), st.y-round(fy), st.col, a)
		addPixel(img, w, h, st.x+round(px), st.y+round(py), st.col, a)
		addPixel(img, w, h, st.x-round(px), st.y-round(py), st.col, a)
	}
}

// blendPixel alpha-blends color c over the existing pixel at (x, y).
func blendPixel(img *image.RGBA, w, h, x, y int, c gfx.RGB, a float64) {
	if a <= 0 || x < 0 || y < 0 || x >= w || y >= h {
		return
	}
	if a > 1 {
		a = 1
	}
	off := img.PixOffset(x, y)
	out := gfx.RGB{
		R: c.R*a + float64(img.Pix[off])/255*(1-a),
		G: c.G*a + float64(img.Pix[off+1])/255*(1-a),
		B: c.B*a + float64(img.Pix[off+2])/255*(1-a),
	}
	r, g, b, _ := out.RGBA8()
	img.Pix[off] = r
	img.Pix[off+1] = g
	img.Pix[off+2] = b
	img.Pix[off+3] = 255
}

func round(v float64) int { return int(math.Round(v)) }
