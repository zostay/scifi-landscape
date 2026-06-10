package scene

import (
	"image"
	"math"
	"math/rand"
	"sync"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// A dome is a geodesic glass hemisphere over part of a city. It sits flat on the
// ground at (cx, baseRow) and bulges up with radius r. It is drawn as a
// semi-transparent shell that reflects the sky (so the buildings inside show
// through), brightening at the grazing rim, overlaid with the projected struts of
// a geodesic (subdivided-icosahedron) frame.
type dome struct {
	cx, baseRow, r int
}

const (
	domeChance   = 0.45 // chance a city (when present) is domed
	domeMaxCount = 3    // up to this many domes over one city

	// Dome radius as a fraction of the scene width (clamped to cover the tallest
	// building so nothing pokes through the top).
	domeRFracLo = 0.035
	domeRFracHi = 0.085
	domeMinR    = 10

	// Glass shell: nearly clear over the center (buildings show through), more
	// opaque and reflective at the grazing rim (Fresnel).
	domeBaseAlpha = 0.20
	domeRimAlpha  = 0.68
	domeFresPow   = 2.6
	domeRimWhite  = 0.60 // how far the rim reflection washes toward white
	domeSpecPow   = 28.0 // sun-glint tightness
	domeSpecAmt   = 0.7  // sun-glint strength

	// Geodesic struts: the near side bright, the far side dimmer (seen through the
	// glass). The far side must stay clearly visible — near the apex some
	// front-facing faces tilt just past z=0 and read as "back", so too-faint a
	// value leaves an apparent hole in the struts up top.
	domeStrutFront = 0.45
	domeStrutBack  = 0.28
	domeFreqSmall  = 2
	domeFreqMedium = 3
	domeFreqLarge  = 4
)

// planDomes decides whether a city is domed and, if so, places the domes over
// clusters of its buildings. It draws from rng (after the buildings), so a city's
// buildings are unaffected by whether it ends up domed.
func planDomes(rng *rand.Rand, blds []building, horizon, band, w int) []dome {
	if len(blds) == 0 || rng.Float64() >= domeChance {
		return nil
	}
	// Tallest building, so every dome is at least tall enough to enclose one.
	maxH := 0
	for _, b := range blds {
		if b.h > maxH {
			maxH = b.h
		}
	}
	baseRow := horizon + band // sit on the front edge of the city band
	minCover := band + maxH + 4

	n := 1 + rng.Intn(domeMaxCount)
	domes := make([]dome, 0, n)
	for range n {
		// Center on a random building — dense districts hold more buildings, so
		// they attract more domes.
		cx := blds[rng.Intn(len(blds))].x
		r := max(int(rnd(rng, domeRFracLo, domeRFracHi)*float64(w)), domeMinR, minCover)
		domes = append(domes, dome{cx: cx, baseRow: baseRow, r: r})
	}
	return domes
}

// drawDome rasterizes one dome: the reflective glass shell, then the geodesic
// strut frame projected onto it.
func drawDome(img *image.RGBA, w, h int, d dome, sky gfx.Gradient, lm lightModel) {
	R := float64(d.r)
	cx, baseRow := float64(d.cx), float64(d.baseRow)
	// Sun direction for the glint, in dome space (X right, Y up, Z toward viewer).
	sx, sy, sz := lm.lx, -lm.ly, lm.lz
	if sl := math.Sqrt(sx*sx + sy*sy + sz*sz); sl > 0 {
		sx, sy, sz = sx/sl, sy/sl, sz/sl
	}

	for oy := -d.r; oy <= 0; oy++ {
		y := d.baseRow + oy
		if y < 0 || y >= h {
			continue
		}
		Y := -float64(oy) / R // 0 at the base, 1 at the apex
		for ox := -d.r; ox <= d.r; ox++ {
			x := d.cx + ox
			if x < 0 || x >= w {
				continue
			}
			X := float64(ox) / R
			rr := X*X + Y*Y
			if rr > 1 {
				continue
			}
			cover := math.Min((1-math.Sqrt(rr))*R, 1) // feather the last rim pixel
			if cover <= 0 {
				continue
			}
			Z := math.Sqrt(1 - rr)

			// Reflected sky: the dome shows more sky overhead at the top, fading to
			// the horizon color near the base.
			refl := sky.At(clamp01(0.15 + 0.85*Y)).RGB()
			// Fresnel: the grazing rim is more reflective — brighter and more opaque.
			fres := math.Pow(1-Z, domeFresPow)
			refl = gfx.RGB{
				R: refl.R + (1-refl.R)*fres*domeRimWhite,
				G: refl.G + (1-refl.G)*fres*domeRimWhite,
				B: refl.B + (1-refl.B)*fres*domeRimWhite,
			}
			// Sun glint off the glass.
			if spec := X*sx + Y*sy + Z*sz; spec > 0 {
				s := math.Pow(spec, domeSpecPow) * domeSpecAmt
				refl = gfx.RGB{R: refl.R + (1-refl.R)*s, G: refl.G + (1-refl.G)*s, B: refl.B + (1-refl.B)*s}
			}
			a := (domeBaseAlpha + (domeRimAlpha-domeBaseAlpha)*fres) * cover
			blendPixel(img, w, h, x, y, refl, a)
		}
	}

	// Geodesic struts, projected from the unit hemisphere onto the dome. The
	// alpha fades smoothly with depth (near side bright, far side dimmer) rather
	// than switching abruptly at z=0 — a hard switch leaves an apparent hole near
	// the apex, where front-facing faces tilt just past z=0.
	frame := gfx.RGB{R: 0.9, G: 0.94, B: 1.0}
	for _, e := range geodesicFor(domeFreqFor(d.r)) {
		a, b := e[0], e[1]
		midz := (a[2] + b[2]) * 0.5
		alpha := domeStrutBack + (domeStrutFront-domeStrutBack)*smoothstep(-0.45, 0.45, midz)
		ax, ay := cx+a[0]*R, baseRow-a[1]*R
		bx, by := cx+b[0]*R, baseRow-b[1]*R
		drawLine(img, w, h, ax, ay, bx, by, frame, alpha)
	}
}

// domeFreqFor picks the geodesic subdivision frequency for a dome of radius r, so
// larger domes get finer triangles (and tiny distant ones stay simple).
func domeFreqFor(r int) int {
	switch {
	case r >= 120:
		return domeFreqLarge
	case r >= 55:
		return domeFreqMedium
	default:
		return domeFreqSmall
	}
}

// drawLine blends a thin line between two floating-point endpoints.
func drawLine(img *image.RGBA, w, h int, x0, y0, x1, y1 float64, col gfx.RGB, a float64) {
	dx, dy := x1-x0, y1-y0
	steps := math.Max(math.Abs(dx), math.Abs(dy))
	if steps < 1 {
		blendPixel(img, w, h, round(x0), round(y0), col, a)
		return
	}
	for i := 0.0; i <= steps; i++ {
		t := i / steps
		blendPixel(img, w, h, round(x0+dx*t), round(y0+dy*t), col, a)
	}
}

// geodesic edges are cached per subdivision frequency (the geometry is the same
// for every dome of that frequency, only the projection differs).
var (
	geodesicMu    sync.Mutex
	geodesicCache = map[int][][2][3]float64{}
)

func geodesicFor(freq int) [][2][3]float64 {
	geodesicMu.Lock()
	defer geodesicMu.Unlock()
	if e, ok := geodesicCache[freq]; ok {
		return e
	}
	e := buildGeodesic(freq)
	geodesicCache[freq] = e
	return e
}

// buildGeodesic subdivides each face of an icosahedron into freq² triangles,
// projects the vertices onto the unit sphere, and returns the unique edges of the
// upper hemisphere (Y >= 0) — the wireframe of a geodesic dome.
func buildGeodesic(freq int) [][2][3]float64 {
	t := (1 + math.Sqrt(5)) / 2
	verts := [][3]float64{
		{-1, t, 0}, {1, t, 0}, {-1, -t, 0}, {1, -t, 0},
		{0, -1, t}, {0, 1, t}, {0, -1, -t}, {0, 1, -t},
		{t, 0, -1}, {t, 0, 1}, {-t, 0, -1}, {-t, 0, 1},
	}
	for i := range verts {
		verts[i] = normalize3(verts[i])
	}
	faces := [][3]int{
		{0, 11, 5}, {0, 5, 1}, {0, 1, 7}, {0, 7, 10}, {0, 10, 11},
		{1, 5, 9}, {5, 11, 4}, {11, 10, 2}, {10, 7, 6}, {7, 1, 8},
		{3, 9, 4}, {3, 4, 2}, {3, 2, 6}, {3, 6, 8}, {3, 8, 9},
		{4, 9, 5}, {2, 4, 11}, {6, 2, 10}, {8, 6, 7}, {9, 8, 1},
	}

	// Stable, collision-free vertex ids from quantized coordinates, so shared
	// vertices map to one id and each edge is deduped exactly once.
	vid := map[[3]int64]int{}
	id := func(p [3]float64) int {
		k := [3]int64{
			int64(math.Round(p[0] * 1e5)),
			int64(math.Round(p[1] * 1e5)),
			int64(math.Round(p[2] * 1e5)),
		}
		i, ok := vid[k]
		if !ok {
			i = len(vid)
			vid[k] = i
		}
		return i
	}
	seen := map[[2]int]bool{}
	var edges [][2][3]float64
	addEdge := func(p, q [3]float64) {
		if p[1] < -1e-6 || q[1] < -1e-6 {
			return // upper hemisphere only
		}
		a, b := id(p), id(q)
		if a > b {
			a, b = b, a
		}
		k := [2]int{a, b}
		if seen[k] {
			return
		}
		seen[k] = true
		edges = append(edges, [2][3]float64{p, q})
	}

	ff := float64(freq)
	for _, f := range faces {
		A, B, C := verts[f[0]], verts[f[1]], verts[f[2]]
		pt := func(i, j int) [3]float64 {
			fi, fj := float64(i), float64(j)
			return normalize3([3]float64{
				(A[0]*(ff-fi-fj) + B[0]*fi + C[0]*fj) / ff,
				(A[1]*(ff-fi-fj) + B[1]*fi + C[1]*fj) / ff,
				(A[2]*(ff-fi-fj) + B[2]*fi + C[2]*fj) / ff,
			})
		}
		for i := range freq {
			for j := range freq - i {
				p00, p10, p01 := pt(i, j), pt(i+1, j), pt(i, j+1)
				addEdge(p00, p10)
				addEdge(p00, p01)
				addEdge(p10, p01)
			}
		}
	}
	return edges
}

func normalize3(v [3]float64) [3]float64 {
	l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if l == 0 {
		return v
	}
	return [3]float64{v[0] / l, v[1] / l, v[2] / l}
}
