package scene

import (
	"image"
	"math/rand"
	"time"
)

// Mountains1 is the v1 mountain range. It embeds the v0 Mountains for its stream
// key ("mountains") and entity schema (MountainsV0), and draws the same ridge with
// two adjustments. It lifts a near-flat ridge to a minimum height so the horizon is
// never bare (enforceMinRidge). And when the scene has an ocean it brings the range's
// feet down to the horizon at the coastline instead of running edge to edge: columns
// whose far horizon is open water are tapered to nothing, so the range plants itself
// on the land and leaves open sea (or, over offshore land, disconnected island ridges)
// along the coast. The coast it reads is the SAME perspective-mapped ocean the water
// renderer draws, so the feet land exactly where the water meets the sky.
//
// Its Generate draws no randomness of its own beyond v0's (the floor and envelope are
// deterministic post-processing). Its RenderList, however, is its own: it shades the
// ridge with a slope hillshade and a facet field (see drawMountainColumnShaded) so the
// silhouette reads as sloped rock rather than a flat fill — the same form-shading the
// extra ranges use.
//
// FROZEN once released: it keeps the "mountains" stream key and the MountainsV0
// schema of its embedded v0; add a Mountains2 for new behavior.
type Mountains1 struct{ Mountains }

const (
	// Coastline response, as fractions of the scene width so the look is
	// resolution-independent. coastBias shifts the taper a little to either side of
	// the true coast (negative pulls the feet back onto the land, leaving a bare
	// coastal strip; positive lets the range spill a little over the water).
	// coastFeather is the half-width over which a foot slopes down to the horizon.
	coastBiasFracMin    = -0.04
	coastBiasFracMax    = 0.05
	coastFeatherFracMin = 0.02
	coastFeatherFracMax = 0.07

	// coastProbe is how many rows below the horizon to sample land/water. The first
	// rows below the horizon are the far distance, so this reads the coastline as it
	// meets the sky. A small band (not a single row) avoids a one-pixel sliver of
	// beach/foam flipping a whole column.
	coastProbe = 2
)

// minRidgeFrac is the smallest a horizon ridge may be, as a fraction of the sky. The
// v0 height is |normal|·scale, which bottoms out near zero — a near-flat ridge reads
// as "no mountains on the horizon", which looks especially wrong when the scene also
// has extra ranges. v1 lifts a too-short ridge to this minimum so there is always a
// visible horizon range (an ocean's coastline can still bring it down — see Generate).
const minRidgeFrac = 0.06

// Generate draws the v0 ridge, lifts it to a minimum height so the horizon is never
// bare (enforceMinRidge), then — when the scene has an ocean — brings it down to the
// coastline (applyCoastEnvelope), so a non-ocean scene always shows a ridge while an
// ocean scene may still leave the horizon open over water. It draws no randomness of
// its own beyond v0's (the floor and envelope are deterministic post-processing on the
// stored Heights), so replay stays seed-independent.
func (m *Mountains1) Generate(c *Context) (SceneList, error) {
	list, err := m.Mountains.Generate(c)
	if err != nil || len(list) == 0 {
		return list, err
	}
	mr, err := entityToMountains(list[0])
	if err != nil {
		return nil, err
	}
	enforceMinRidge(mr.heights, c.Settings.HorizonY)
	if oc := c.Ocean; oc != nil && oc.present {
		applyCoastEnvelope(c.Rng, mr.heights, oc, c.Settings.HorizonY, c.W)
	}
	return SceneList{mountainsToEntity(mr)}, nil
}

// enforceMinRidge scales a horizon ridge up to a minimum peak height (minRidgeFrac of
// the sky) when it would otherwise be near-flat, preserving its shape so the result is
// an ordinary-looking ridge rather than a flat bar. A ridge already at or above the
// minimum is untouched. It mutates heights in place.
func enforceMinRidge(heights []float64, horizon int) {
	minPx := minRidgeFrac * float64(horizon)
	var hmax float64
	for _, v := range heights {
		if v > hmax {
			hmax = v
		}
	}
	if hmax <= 0 || hmax >= minPx {
		return
	}
	s := minPx / hmax
	for i := range heights {
		heights[i] *= s
	}
}

// RenderList draws the horizon range with the v1 form-shading: each column is shaded
// by its lateral slope (a directional hillshade) and a vertical facet field, so the
// ridge reads as sloped, faceted rock instead of a flat altitude fill. It mirrors the
// v0 animation (column batches over the same duration) but swaps the flat-mottle
// column drawer for the shaded one. It consumes no randomness, so replay is
// seed-independent.
func (m *Mountains1) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	mr, err := entityToMountains(list[0])
	if err != nil {
		return err
	}
	w, h := c.W, c.H
	horizon := c.Settings.HorizonY
	shade := mountainShader(c.MountainRugged)

	batch := max(w/mountainsAnimCols, 1)
	per := mountainsAnimDuration / time.Duration((w+batch-1)/batch)

	for x0 := 0; x0 < w; x0 += batch {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		x1 := min(x0+batch, w)
		c.Canvas.Draw(func(img *image.RGBA) {
			for x := x0; x < x1; x++ {
				drawMountainColumnShaded(img, w, h, x, horizon, mr.heights, mr.maxAlt, mr.grad, mr.texSeed, shade)
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// applyCoastEnvelope tapers the ridge heights to zero over the columns whose far
// horizon is open water, so the range comes down to the horizon at the coastline.
// It classifies each column as land or water just below the horizon, builds a
// signed column-distance to the nearest coast (positive into land, negative into
// water), and scales each column by a smoothstep of that distance — offset by a
// random lateral bias and softened over a random feather width. It mutates heights
// in place and draws two values from rng.
func applyCoastEnvelope(rng *rand.Rand, heights []float64, oc *ocean, horizon, w int) {
	if w == 0 {
		return
	}
	// Classify the far horizon per column from the perspective-mapped ocean the
	// water renderer uses, so the feet land where the water touches the sky.
	land := make([]bool, w)
	for x := range w {
		l := false
		for k := 1; k <= coastProbe; k++ {
			if oc.LandAt(x, horizon+k) {
				l = true
				break
			}
		}
		land[x] = l
	}
	sd := signedCoastDistance(land)

	bias := rnd(rng, coastBiasFracMin, coastBiasFracMax) * float64(w)
	feather := rnd(rng, coastFeatherFracMin, coastFeatherFracMax) * float64(w)
	for x := range heights {
		// env: 0 well out over water, 1 well into land, with the transition centered
		// on the coast and shifted by bias.
		env := smoothstep(-feather, feather, sd[x]+bias)
		heights[x] *= env
	}
}

// signedCoastDistance returns, for each column, the distance in columns to the
// nearest land/water boundary: positive where the column is land, negative where
// it is water. An all-land (or all-water) row has no boundary, so every column is
// reported as +len (or -len) — far inside its kind — which leaves an all-land
// horizon stretching across and an all-water horizon mountain-free.
func signedCoastDistance(land []bool) []float64 {
	n := len(land)
	out := make([]float64, n)
	// Forward pass: distance to the nearest boundary seen so far on the left.
	const big = 1 << 30
	dist := big
	for x := range n {
		if x > 0 && land[x] != land[x-1] {
			dist = 0 // a boundary sits between x-1 and x; x is 0.5 from it, round to 0
		} else if dist < big {
			dist++
		}
		out[x] = float64(dist)
	}
	// Backward pass: take the smaller distance from either side.
	dist = big
	for x := n - 1; x >= 0; x-- {
		if x < n-1 && land[x] != land[x+1] {
			dist = 0
		} else if dist < big {
			dist++
		}
		if float64(dist) < out[x] {
			out[x] = float64(dist)
		}
	}
	// Sign by land/water; a row with no boundary keeps a large magnitude.
	for x := range n {
		if out[x] >= big {
			out[x] = float64(n)
		}
		if !land[x] {
			out[x] = -out[x]
		}
	}
	return out
}
