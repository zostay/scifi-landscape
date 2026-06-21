package scene

import (
	"math/rand"
)

// Mountains1 is the v1 mountain range. It embeds the v0 Mountains for its stream
// key ("mountains") and entity schema (MountainsV0), and draws the exact same
// ridge — but when the scene has an ocean it brings the range's feet down to the
// horizon at the coastline instead of running edge to edge. Columns whose far
// horizon is open water are tapered to nothing, so the range plants itself on the
// land and leaves open sea (or, over offshore land, disconnected island ridges)
// along the coast. The coast it reads is the SAME perspective-mapped ocean the
// water renderer draws, so the feet land exactly where the water meets the sky.
//
// With no ocean it is byte-identical to v0: it draws no extra randomness and
// applies no envelope, so land-only scenes are unchanged.
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

// Generate draws the same ridge as v0, then — when the scene has an ocean — brings
// the range down to the horizon at the coastline (see applyCoastEnvelope). The
// extra random draws happen only on the ocean path and only after every v0 draw,
// so a scene with no ocean reproduces v0 exactly. The envelope is baked into the
// stored Heights, so the renderer is unchanged and replay stays seed-independent.
func (m *Mountains1) Generate(c *Context) (SceneList, error) {
	list, err := m.Mountains.Generate(c)
	if err != nil || len(list) == 0 {
		return list, err
	}
	oc := c.Ocean
	if oc == nil || !oc.present {
		return list, nil // no coast to respond to: identical to v0
	}
	mr, err := entityToMountains(list[0])
	if err != nil {
		return nil, err
	}
	applyCoastEnvelope(c.Rng, mr.heights, oc, c.Settings.HorizonY, c.W)
	return SceneList{mountainsToEntity(mr)}, nil
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
