package scene

import (
	"image"
	"math"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Mountain form-shading. A filled mountain silhouette colored only by altitude reads
// as flat "foam"; these helpers add the cues that make it read as three-dimensional
// rock. They modulate only the value (brightness) of each pixel — hue and saturation
// stay on the range's gradient — and are used by the v1-era mountain renderers
// (mountains.v1 and mountainranges.v0). The released mountains.v0 keeps its original
// flat mottle and is untouched.
//
// There are two styles, chosen per scene (Globals.MountainRugged):
//   - CONICAL (the default): the original horizontal banded gradient and isotropic
//     mottle, plus a smooth, broad lateral hillshade that lights one side of each peak
//     and shadows the other — so the ridge reads as softly eroded, rounded slopes
//     (Rockies-like) with a sense of scale and depth, not sharp cliffs.
//   - RUGGED (an alternate, craggier look): the conical base plus a hillshaded fractal
//     relief, breaking the faces into more textured, broken rock.
const (
	// Shared: the lateral hillshade is taken over a broad window so it tracks the
	// mountain-scale tilt (the cone's facing) rather than pixel jaggedness, which would
	// read as stripes. The window scales with the peak height. Light comes from the
	// right: a flank descending to the right brightens, one rising to the right darkens.
	mountainShadeBase   = 1.00
	conicalSlopeWinFrac = 0.14 // slope window half-width as a fraction of maxAlt
	conicalAmp          = 0.42 // light/shadow swing across a peak's soft sides

	// Rugged (alt) adds a screen-space hillshade of an isotropic fractal relief on top
	// of a gentler conical base, reading as craggy, broken rock. Light from upper-left.
	ruggedFormAmp   = 0.22
	ruggedFacetFreq = 0.05 // broad, terrain-scale facets
	ruggedFacetOct  = 5
	ruggedRelief    = 7.0 // how steeply the noise gradient tilts the facet normals
	ruggedFacetAmp  = 0.55
	ruggedLightX    = -0.45
	ruggedLightY    = -0.45
	ruggedLightZ    = 0.77 // (X,Y,Z) is unit length; Z is the flat-ground reference

	// facetSeedOffset decorrelates the facet field from any other use of texSeed.
	facetSeedOffset = 7919
)

// mountainShadeFunc is a per-pixel brightness multiplier for a mountain column: given
// the pixel and the column's broad lateral ridge slope, it returns the value scale
// that gives the silhouette its form. The two styles below satisfy it.
type mountainShadeFunc func(x, y int, slope float64, texSeed int) float64

// mountainShader picks the shading style for a scene: the craggier rugged shading when
// rugged is set, otherwise the default soft conical shading.
func mountainShader(rugged bool) mountainShadeFunc {
	if rugged {
		return mountainRuggedShade
	}
	return mountainConicalShade
}

// slopeWindow is the half-width (px) over which the lateral hillshade slope is taken,
// so it captures the mountain-scale tilt rather than per-pixel noise.
func slopeWindow(maxAlt float64) int {
	return max(int(conicalSlopeWinFrac*maxAlt), 2)
}

// broadRidgeSlope is the lateral slope of the ridge at column x measured over a window
// of half-width win (clamped at the edges): the broad, mountain-scale tilt that gives
// each peak its soft conical sides.
func broadRidgeSlope(heights []float64, x, win int) float64 {
	l, r := x-win, x+win
	if l < 0 {
		l = 0
	}
	if r >= len(heights) {
		r = len(heights) - 1
	}
	if r <= l {
		return 0
	}
	return (heights[r] - heights[l]) / float64(r-l)
}

// mountainMottle is the original subtle isotropic surface texture (the previous
// gradient's mottle), kept under both styles so faces are not dead flat.
func mountainMottle(x, y, texSeed int) float64 {
	return 1 + (gfx.FBM(float64(x)*mountainTexFreq, float64(y)*mountainTexFreq, texSeed, 3)-0.5)*mountainTexAmp
}

// mountainConicalShade is the default style: a smooth, broad lateral hillshade giving
// each peak a soft lit/shadow side (conical, eroded — no sharp facets) times the
// subtle surface mottle. The horizontal banded gradient (applied by the caller) shows
// through.
func mountainConicalShade(x, y int, slope float64, texSeed int) float64 {
	unit := slope / math.Sqrt(1+slope*slope) // sin of the broad lateral tilt, in [-1, 1]
	hill := mountainShadeBase - conicalAmp*unit
	return hill * mountainMottle(x, y, texSeed)
}

// mountainRuggedShade is the alternate style: the conical base (a gentler lateral
// hillshade) plus a hillshaded fractal relief, so the faces break into craggier,
// more textured rock.
func mountainRuggedShade(x, y int, slope float64, texSeed int) float64 {
	unit := slope / math.Sqrt(1+slope*slope)
	form := mountainShadeBase - ruggedFormAmp*unit
	return form * mountainMottle(x, y, texSeed) * ruggedFacetShade(x, y, texSeed)
}

// ruggedFacetShade hillshades an isotropic fractal relief in screen space: it takes
// the noise's screen gradient as a surface tilt, lights it from the upper-left, and
// centers flat ground at 1, so a filled silhouette breaks into craggy rock faces.
func ruggedFacetShade(x, y, texSeed int) float64 {
	s := texSeed + facetSeedOffset
	c := gfx.FBM(float64(x)*ruggedFacetFreq, float64(y)*ruggedFacetFreq, s, ruggedFacetOct)
	ex := gfx.FBM(float64(x+1)*ruggedFacetFreq, float64(y)*ruggedFacetFreq, s, ruggedFacetOct)
	ey := gfx.FBM(float64(x)*ruggedFacetFreq, float64(y+1)*ruggedFacetFreq, s, ruggedFacetOct)
	gx := (ex - c) * ruggedRelief
	gy := (ey - c) * ruggedRelief
	inv := 1 / math.Sqrt(gx*gx+gy*gy+1)
	lit := (-gx*ruggedLightX - gy*ruggedLightY + ruggedLightZ) * inv
	return 1 + ruggedFacetAmp*(lit-ruggedLightZ) // flat ground (gx=gy=0) → 1
}

// drawMountainColumnShaded draws one column of a mountain ridge from the foot up to
// its peak, like the frozen drawMountainColumn, but shades each pixel with the given
// style (a broad lateral hillshade plus the style's texture) instead of the flat
// isotropic mottle, so the silhouette reads as three-dimensional rock. It needs the
// whole heightmap to take the lateral slope. It consumes no randomness.
func drawMountainColumnShaded(img *image.RGBA, w, h, x, baseline int, heights []float64, maxAlt float64, grad gfx.Gradient, texSeed int, shade mountainShadeFunc) {
	hcol := heights[x]
	slope := broadRidgeSlope(heights, x, slopeWindow(maxAlt))
	top := baseline - int(math.Ceil(hcol)) - 1
	// Start at the foot, but never above the bottom row, so a range whose foot sits
	// below the bottom edge (high vantage) still draws the part of its peak in view.
	for y := min(baseline-1, h-1); y >= top && y >= 0; y-- {
		alt := float64(baseline - y)
		cov := clamp(hcol-alt+0.5, 0, 1) // coverage: 1 inside, feathered at the top edge
		if cov <= 0 {
			continue
		}
		col := grad.At(clamp(alt/maxAlt, 0, 1))
		col.V *= shade(x, y, slope, texSeed)
		blendPixel(img, w, h, x, y, col.RGB(), cov)
	}
}
