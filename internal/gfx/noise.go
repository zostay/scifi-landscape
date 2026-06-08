package gfx

import "math"

// hash2 returns a deterministic pseudo-random value in [0,1) for the integer
// lattice point (x, y) under the given seed. It is a cheap integer bit-mix —
// good enough to seed value noise, not a cryptographic hash.
func hash2(x, y, seed int) float64 {
	h := uint32(x)*0x27d4eb2d + uint32(y)*0x165667b1 + uint32(seed)*0x9e3779b1
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return float64(h) / float64(1<<32)
}

// valueNoise is smooth 2D value noise in [0,1): random values on the integer
// lattice, smoothstep-interpolated between. Adjacent samples vary gently, which
// gives the blobby, clumped look of a natural surface rather than TV static.
func valueNoise(x, y float64, seed int) float64 {
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)
	u := fx * fx * (3 - 2*fx)
	v := fy * fy * (3 - 2*fy)

	n00 := hash2(x0, y0, seed)
	n10 := hash2(x0+1, y0, seed)
	n01 := hash2(x0, y0+1, seed)
	n11 := hash2(x0+1, y0+1, seed)

	nx0 := n00 + (n10-n00)*u
	nx1 := n01 + (n11-n01)*u
	return nx0 + (nx1-nx0)*v
}

// FBM is fractional Brownian motion: several octaves of value noise summed at
// halving amplitude and doubling frequency, normalized to [0,1]. The result has
// both broad shapes and fine grain, useful for surfaces like dirt. octaves is
// clamped to at least 1.
func FBM(x, y float64, seed, octaves int) float64 {
	if octaves < 1 {
		octaves = 1
	}
	sum, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += amp * valueNoise(x*freq, y*freq, seed+i*101)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return sum / norm
}

// RidgedFBM is fractional Brownian motion folded into sharp ridge lines: each
// octave of value noise is turned into a ridge by reflecting it about its
// midpoint (1-|2n-1|) and squaring, then the octaves are summed at halving
// amplitude and doubling frequency, normalized to [0,1]. Unlike FBM (which is
// blobby), the result has crests and creases — useful for mountain ridges and
// the rough, ridged terrain of an airless world. octaves is clamped to >= 1.
func RidgedFBM(x, y float64, seed, octaves int) float64 {
	if octaves < 1 {
		octaves = 1
	}
	sum, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		n := valueNoise(x*freq, y*freq, seed+i*131)
		r := 1 - math.Abs(2*n-1) // 0 at the extremes, 1 at the midline: a ridge
		sum += amp * r * r
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return sum / norm
}

// fade is Perlin's quintic interpolation curve 6t^5-15t^4+10t^3, which has zero
// first and second derivatives at 0 and 1 so adjacent cells join without creases.
func fade(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }

// pgrad takes a hash in [0,1) as an angle and returns the dot of that unit
// gradient with (x,y) — the per-corner contribution of gradient noise.
func pgrad(h, x, y float64) float64 {
	a := h * (2 * math.Pi)
	return math.Cos(a)*x + math.Sin(a)*y
}

// Perlin is 2D gradient (Perlin) noise: random unit gradients on the integer
// lattice, each dotted with the offset to the sample point and fade-interpolated.
// Unlike value noise it has no axis-aligned blockiness and returns roughly
// [-1, 1] (zero-mean), which makes it the better base for cloud shapes.
func Perlin(x, y float64, seed int) float64 {
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	xf := x - float64(x0)
	yf := y - float64(y0)
	u := fade(xf)
	v := fade(yf)

	n00 := pgrad(hash2(x0, y0, seed), xf, yf)
	n10 := pgrad(hash2(x0+1, y0, seed), xf-1, yf)
	n01 := pgrad(hash2(x0, y0+1, seed), xf, yf-1)
	n11 := pgrad(hash2(x0+1, y0+1, seed), xf-1, yf-1)

	return lerp(lerp(n00, n10, u), lerp(n01, n11, u), v)
}

// PerlinFBM sums octaves of Perlin noise at halving amplitude and doubling
// frequency, remapped to [0,1]. It is the smooth, organic counterpart to FBM,
// used for cloud detail and edge erosion. octaves is clamped to at least 1.
func PerlinFBM(x, y float64, seed, octaves int) float64 {
	if octaves < 1 {
		octaves = 1
	}
	sum, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += amp * Perlin(x*freq, y*freq, seed+i*131)
		norm += amp
		amp *= 0.5
		freq *= 2
	}
	return clamp01(sum/norm*0.5 + 0.5)
}

// Worley is 2D cellular (Worley) noise: one randomly-placed feature point per
// lattice cell, returning the distance to the nearest one (F1), clamped to [0,1].
// Inverted (1-Worley) it gives rounded cellular blobs — the billowy lumps of a
// cloud. The 3×3 neighbor scan guarantees the true nearest point is found.
func Worley(x, y float64, seed int) float64 {
	xi := int(math.Floor(x))
	yi := int(math.Floor(y))
	minD := math.MaxFloat64
	for gy := -1; gy <= 1; gy++ {
		for gx := -1; gx <= 1; gx++ {
			cx, cy := xi+gx, yi+gy
			fx := float64(cx) + hash2(cx, cy, seed)
			fy := float64(cy) + hash2(cx, cy, seed+9871)
			if d := math.Hypot(x-fx, y-fy); d < minD {
				minD = d
			}
		}
	}
	if minD > 1 {
		minD = 1
	}
	return minD
}
