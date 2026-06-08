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
