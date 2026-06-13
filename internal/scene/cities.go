package scene

import (
	"image"
	"math"
	"sort"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Cities draws a distant city as clustered rectangular buildings sitting on the
// ground near the horizon — dark, low-saturation, similar colors, like far-off
// skyscrapers in silhouette. Not every scene has one. A city can be a small
// settlement patch or stretch the whole width (dozens of buildings up to
// thousands). Buildings are drawn back-to-front: the farthest (highest, right
// at the horizon) first, working down and closer, growing slightly as they near.
type Cities struct{}

func (c *Cities) Name() string { return "cities" }

// Schemas lists the entity schema keys the cities element owns.
func (c *Cities) Schemas() []string { return []string{SchemaCityV0} }

const (
	cityChance             = 0.45
	citiesAnimDuration     = 800 * time.Millisecond
	citiesDomeAnimDuration = 500 * time.Millisecond

	cityFullChance = 0.30 // chance the footprint spans the whole width
	cityBandLo     = 0.04 // city depth band, as a fraction of the ground height
	cityBandHi     = 0.12

	// A noisy density field shapes the footprint: irregular edges (odd shapes)
	// and dense/sparse pockets. cutoff carves gaps; candidates are sampled and
	// kept with probability equal to the local density.
	cityDensFreq = 0.006
	cityCutoffLo = 0.20
	cityCutoffHi = 0.50
	cityCandLo   = 1.0 // candidate buildings per pixel of footprint span
	cityCandHi   = 3.0
	cityMinCount = 60   // candidate floor (kept count is lower, after the density test)
	cityHazeMax  = 0.45 // farthest buildings blend this far toward the horizon sky
)

// building is one resolved building rectangle. base is the ground-contact row;
// it rises h pixels up from there and is w wide.
type building struct {
	x, base, w, h int
	col           gfx.RGB
}

// Generate resolves the scene's city into a single entity carrying every building
// (already sorted back-to-front) and any domes. It performs every city random
// draw on the element stream and has no side effects (it draws nothing), so
// identical globals always yield an identical scene list. An empty list means the
// scene has no city.
func (c *Cities) Generate(ctx *Context) (SceneList, error) {
	if ctx.Rng.Float64() >= cityChance {
		return nil, nil
	}
	w, h := ctx.W, ctx.H
	horizon := ctx.Settings.HorizonY
	groundH := h - horizon
	if groundH < 6 {
		return nil, nil // no ground to build on
	}

	// Footprint: a localized settlement or a full-width sprawl. Either way a
	// low-frequency noise field gives it irregular edges and dense/sparse
	// pockets, so the shape is varied rather than a clean rectangle.
	center, halfW := float64(w)/2, float64(w)
	if ctx.Rng.Float64() >= cityFullChance {
		halfW = rnd(ctx.Rng, 0.10, 0.50) * float64(w)
		center = float64(ctx.Rng.Intn(w))
	}
	densSeed := ctx.Rng.Int()
	cutoff := rnd(ctx.Rng, cityCutoffLo, cityCutoffHi)
	dens := make([]float64, w)
	for x := range w {
		dx := math.Abs(float64(x)-center) / halfW
		env := 1 - smoothstep(0.6, 1.0, dx) // flat core, soft (then noisy) edge
		n := gfx.FBM(float64(x)*cityDensFreq, 3, densSeed, 3)
		dens[x] = math.Max(env*n-cutoff, 0) / (1 - cutoff)
	}

	// Shallow depth band just below the horizon (the city is far off).
	band := max(int(rnd(ctx.Rng, cityBandLo, cityBandHi)*float64(groundH)), 3)

	// One dark, low-saturation palette for the whole city.
	hue := ctx.Rng.Float64() * 360
	sat := rnd(ctx.Rng, 0.05, 0.22)
	val := rnd(ctx.Rng, 0.10, 0.32)
	haze := ctx.SkyGradient.At(0).RGB() // horizon sky color, for atmospheric fade

	lo := max(int(center-halfW), 0)
	hi := min(int(center+halfW), w-1)
	span := max(hi-lo, 1)
	candidates := max(int(float64(span)*rnd(ctx.Rng, cityCandLo, cityCandHi)), cityMinCount)

	blds := make([]building, 0, candidates)
	for range candidates {
		x := lo + ctx.Rng.Intn(span+1)
		d := dens[x]
		if d <= 0 || ctx.Rng.Float64() > d {
			continue // sparse / outside the irregular footprint
		}
		base := horizon + ctx.Rng.Intn(band+1)
		// Stay on land: skip candidates that would stand in the water (when there
		// is an ocean, buildings keep to islands and the coastline). Without an
		// ocean LandAt is true everywhere, so this is a no-op.
		if ctx.LandAt != nil && !ctx.LandAt(x, base) {
			continue
		}
		depth := float64(base-horizon) / float64(band) // 0 far (horizon) .. 1 near
		// Bigger toward the viewer (depth) and in dense areas.
		scale := (1 + depth*1.2) * (0.5 + 0.9*d)
		bw := max(int(rnd(ctx.Rng, 1, 3)*scale), 1)
		bh := max(int(float64(bw)*rnd(ctx.Rng, 1.2, 4.5)), 2)

		col := gfx.HSV{H: hue, S: sat, V: clamp01(val * rnd(ctx.Rng, 0.7, 1.3))}.RGB()
		f := cityHazeMax * (1 - depth) // farther = hazier
		col = gfx.RGB{
			R: col.R + (haze.R-col.R)*f,
			G: col.G + (haze.G-col.G)*f,
			B: col.B + (haze.B-col.B)*f,
		}
		blds = append(blds, building{x: x, base: base, w: bw, h: bh, col: col})
	}
	if len(blds) == 0 {
		return nil, nil
	}

	// Some cities are domed: plan the geodesic domes (drawn after the buildings).
	domes := planDomes(ctx.Rng, blds, horizon, band, w)

	// Back-to-front: farthest (nearest the horizon) first.
	sort.Slice(blds, func(i, j int) bool { return blds[i].base < blds[j].base })

	return SceneList{cityToEntity(blds, domes)}, nil
}

// RenderList draws the city entity onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. An empty list draws nothing; a non-city entity is an
// error.
func (c *Cities) RenderList(ctx *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	blds, domes, err := entityToCity(list[0])
	if err != nil {
		return err
	}
	if len(blds) == 0 {
		return nil
	}
	w, h := ctx.W, ctx.H

	count := len(blds)
	chunk := max(count/100, 1)
	per := citiesAnimDuration / time.Duration((count+chunk-1)/chunk)
	for i0 := 0; i0 < count; i0 += chunk {
		if err := ctx.Ctx.Err(); err != nil {
			return err
		}
		i1 := min(i0+chunk, count)
		ctx.Canvas.Draw(func(img *image.RGBA) {
			for _, b := range blds[i0:i1] {
				drawBuilding(img, w, h, b)
			}
		})
		if err := sleep(ctx.Ctx, per); err != nil {
			return err
		}
	}

	// Domes go up over the finished buildings, reflecting the sky.
	if len(domes) > 0 {
		lm := newLightModel(ctx.Settings)
		per := citiesDomeAnimDuration / time.Duration(len(domes))
		for _, dm := range domes {
			if err := ctx.Ctx.Err(); err != nil {
				return err
			}
			ctx.Canvas.Draw(func(img *image.RGBA) {
				drawDome(img, w, h, dm, ctx.SkyGradient, lm)
			})
			if err := sleep(ctx.Ctx, per); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawBuilding paints one opaque rectangle rising from its base row.
func drawBuilding(img *image.RGBA, w, h int, b building) {
	r, g, bl, _ := b.col.RGBA8()
	top := max(b.base-b.h, 0)
	x1 := min(b.x+b.w, w)
	for y := top; y < b.base && y < h; y++ {
		off := img.PixOffset(max(b.x, 0), y)
		for x := max(b.x, 0); x < x1; x++ {
			img.Pix[off] = r
			img.Pix[off+1] = g
			img.Pix[off+2] = bl
			img.Pix[off+3] = 255
			off += 4
		}
	}
}
