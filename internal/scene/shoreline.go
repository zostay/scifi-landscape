package scene

import (
	"math"
	"math/rand"
)

// shoreModel is a procedural 2D coastline map for the v1 ocean. Rather than carving
// land out of a noise field (which reads as oddly-shaped puddles), it describes the
// coast geometrically — as aerial photography would show it — in a top-down world
// plane (lateral X, distance Z from the viewer) and lets ocean.elev drape it into the
// scene through the perspective projection. Because the map is defined in world space
// and projected, a straight world coast converges toward the central vanishing point.
//
// The land is the union of:
//   - an optional mainland whose coast is a sweep of long arcs (sine terms) with the
//     odd peninsula jutting toward the viewer — either a far shore parallel to the
//     horizon (bays and headlands) or a side coast receding to the vanishing point;
//   - a few islands, each a closed lobed curve (a radius modulated by angle), giving
//     headlands and bays rather than round blobs.
//
// It is built deterministically from the ocean's land seed, so the cities (which read
// the same model via LandAt) and the water renderer always agree on where land is.
type shoreModel struct {
	hasMain bool
	side    int     // 0 = far shore parallel to the horizon; -1/+1 = land to the left/right
	baseZ   float64 // far shore: coast distance; side coast: lateral position of the coast
	arcs    []shoreArc
	penins  []peninsula
	islands []island
}

// shoreArc is one sweeping undulation of a coast (or one lobe of an island): a sine of
// the given amplitude, spatial frequency, and phase.
type shoreArc struct{ amp, freq, phase float64 }

// peninsula is a localized jut of land toward the viewer (a Gaussian pull on the coast
// line) — a headland breaking up an otherwise smooth arc.
type peninsula struct{ pos, width, reach float64 }

// island is a closed landmass centered at (x, z) with base radius r, its outline
// undulated by lobes so it reads as a headland-and-bay shape rather than a disc.
type island struct {
	x, z, r float64
	lobes   []shoreArc
}

// buildShoreModel derives the coastline map from the land seed. The draws are a fixed
// sequence, so a seed always yields the same coast.
func buildShoreModel(seed int) shoreModel {
	r := rand.New(rand.NewSource(int64(seed)))
	m := shoreModel{hasMain: r.Float64() < 0.75}

	if r.Intn(2) == 0 {
		m.side = 0 // far shore parallel to the horizon
		m.baseZ = rnd(r, 0.6, 2.0)
	} else {
		m.side = 1 // side coast receding toward the vanishing point
		if r.Float64() < 0.5 {
			m.side = -1
		}
		m.baseZ = rnd(r, -0.5, 0.5)
	}

	// Long sweeping arcs (low frequency = wide bays and headlands).
	for n := 2 + r.Intn(2); n > 0; n-- {
		m.arcs = append(m.arcs, shoreArc{
			amp:   rnd(r, 0.10, 0.40),
			freq:  rnd(r, 0.5, 2.0),
			phase: r.Float64() * 2 * math.Pi,
		})
	}
	// Occasional peninsulas.
	for n := r.Intn(3); n > 0; n-- {
		m.penins = append(m.penins, peninsula{
			pos:   rnd(r, -1.8, 1.8),
			width: rnd(r, 0.10, 0.30),
			reach: rnd(r, 0.25, 0.65),
		})
	}
	// A few lobed islands out in the water.
	for n := r.Intn(4); n > 0; n-- {
		isl := island{x: rnd(r, -1.8, 1.8), z: rnd(r, 0.08, 1.3), r: rnd(r, 0.08, 0.30)}
		for l := 2 + r.Intn(3); l > 0; l-- {
			isl.lobes = append(isl.lobes, shoreArc{
				amp:   rnd(r, 0.10, 0.50),
				freq:  float64(1 + r.Intn(4)),
				phase: r.Float64() * 2 * math.Pi,
			})
		}
		m.islands = append(m.islands, isl)
	}
	return m
}

// landSDF is the signed land field at world point (X, Z): positive on land, negative in
// open water, zero at the shoreline, with magnitude roughly the distance to the nearest
// shore (so the renderer's beach/foam bands hug the coast). Land is the union of the
// mainland and the islands, so the field is the max of their individual signed fields.
func (m shoreModel) landSDF(X, Z float64) float64 {
	sdf := math.Inf(-1)
	if m.hasMain {
		var s float64
		if m.side == 0 {
			// Far shore: land beyond the coast line Z = coast(X).
			coast := m.baseZ
			for _, a := range m.arcs {
				coast += a.amp * math.Sin(a.freq*X+a.phase)
			}
			for _, p := range m.penins {
				coast -= p.reach * gaussian(X-p.pos, p.width)
			}
			s = Z - coast
		} else {
			// Side coast: land where side*(X - coast(Z)) is positive.
			coast := m.baseZ
			for _, a := range m.arcs {
				coast += a.amp * math.Sin(a.freq*Z+a.phase)
			}
			for _, p := range m.penins {
				coast -= float64(m.side) * p.reach * gaussian(Z-p.pos, p.width)
			}
			s = float64(m.side) * (X - coast)
		}
		sdf = math.Max(sdf, s)
	}
	for _, isl := range m.islands {
		dx, dz := X-isl.x, Z-isl.z
		dist := math.Hypot(dx, dz)
		ang := math.Atan2(dz, dx)
		rr := isl.r
		for _, l := range isl.lobes {
			rr += isl.r * l.amp * math.Sin(l.freq*ang+l.phase)
		}
		sdf = math.Max(sdf, rr-dist)
	}
	return sdf
}

// gaussian is a unit-height bell of width w.
func gaussian(d, w float64) float64 {
	if w <= 0 {
		return 0
	}
	return math.Exp(-(d * d) / (2 * w * w))
}
