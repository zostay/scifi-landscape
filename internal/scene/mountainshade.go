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
// so it captures the mountain-scale tilt rather than per-pixel noise. Callers pass the
// scene's BASE altitude scale (mountainHeightMax·sky), not a range's height-scaled
// maxAlt — every range's heightmap shares the same horizontal peak structure, so they
// must all use the same window.
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
// isotropic mottle, so the silhouette reads as three-dimensional rock. slopeWin is the
// lateral-slope window (px), passed in rather than derived from maxAlt: every range's
// heightmap shares the same horizontal peak structure, so the window must be the same
// for all of them — deriving it from a range's height-scaled maxAlt would widen it for
// the taller near ranges until it spanned several peaks and the per-peak left/right
// shading washed out. It needs the whole heightmap to take the lateral slope, and
// consumes no randomness.
func drawMountainColumnShaded(img *image.RGBA, w, h, x, baseline int, heights []float64, maxAlt float64, grad gfx.Gradient, texSeed, slopeWin int, shade mountainShadeFunc) {
	hcol := heights[x]
	slope := broadRidgeSlope(heights, x, slopeWin)
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

// drawShadedRangeColumn draws one column of an extra mountain range: the shaded peak
// (drawMountainColumnShaded) plus the foot bulge below the baseline (the foot "negative
// contour"; see footBulgeDepth). The bulge is the foot color, darkened toward the bottom
// and form-shaded, so the swelling foot reads as a rounded, sloped body rather than a
// flat edge. The foot is clipped to floor (the lowest row this range may occupy) so it
// never shows below a nearer range. Finally, where the foot meets water (shore > 0), the
// column is reflected into the water tinted with the water color. It consumes no
// randomness.
func drawShadedRangeColumn(img *image.RGBA, w, h, x, baseline int, heights []float64, dcol, maxAlt float64, grad gfx.Gradient, texSeed, slopeWin int, shade mountainShadeFunc, floor, shore int, water gfx.RGB) {
	drawMountainColumnShaded(img, w, h, x, baseline, heights, maxAlt, grad, texSeed, slopeWin, shade)

	if dcol > 0 {
		slope := broadRidgeSlope(heights, x, slopeWin)
		base := grad.At(0) // foot color (darkest end of the range gradient)
		bottom := baseline + int(math.Ceil(dcol))
		for y := baseline; y <= bottom && y < h && y <= floor; y++ {
			if y < 0 {
				continue
			}
			depth := float64(y - baseline)     // 0 at the foot row, increasing downward
			cov := clamp(dcol-depth+0.5, 0, 1) // 1 inside, feathered at the lower edge
			if cov <= 0 {
				continue
			}
			col := base
			col.V *= (1 - rangeBulgeShade*clamp(depth/dcol, 0, 1)) * shade(x, y, slope, texSeed)
			blendPixel(img, w, h, x, y, col.RGB(), cov)
		}
	}

	if shore > 0 {
		drawRangeReflection(img, w, h, x, baseline, heights, dcol, shore, water)
	}
}

const (
	// Reflection: where a range's foot meets water (within reflectShoreExtraFrac of the
	// sky below the foot), the column is mirrored across the waterline into the water,
	// sampling the just-drawn mountain and tinting it toward the water color — more
	// tinted, darker, and fainter with depth, mimicking the water's own fresnel.
	reflectShoreExtraFrac = 0.05
	reflectAlpha          = 0.5  // reflection opacity at the waterline
	reflectFade           = 0.55 // how much the opacity drops by the deepest reflected row
	reflectTintMin        = 0.30 // water tint at the waterline
	reflectTintMax        = 0.70 // water tint at the deepest reflected row
	reflectDark           = 0.30 // darkening at the deepest reflected row
)

// drawRangeReflection mirrors a range column across its waterline (shore) into the
// water below, sampling the mountain pixels just drawn above the line and tinting them
// toward the water color. The mirror spans the drawn silhouette (peak + foot); above
// the peak is sky, which the water already reflects, so the mirror stops there. It
// consumes no randomness.
func drawRangeReflection(img *image.RGBA, w, h, x, baseline int, heights []float64, dcol float64, shore int, water gfx.RGB) {
	top := baseline - int(math.Ceil(heights[x])) - 1 // peak top; above it is sky
	footBottom := baseline + int(math.Ceil(dcol))    // lowest drawn mountain row
	span := float64(shore - top)                     // mirror extent (mountain above the line)
	if span <= 0 {
		return
	}
	for yr := shore + 1; yr < h; yr++ {
		ys := 2*shore - yr // source row above the waterline
		if ys < top {
			break // mirrored past the peak into sky
		}
		if ys > footBottom {
			continue // the small land gap between the foot and the shore: no mountain to mirror
		}
		off := img.PixOffset(x, ys)
		sr := float64(img.Pix[off]) / 255
		sg := float64(img.Pix[off+1]) / 255
		sb := float64(img.Pix[off+2]) / 255

		frac := clamp(float64(yr-shore)/span, 0, 1)
		tint := reflectTintMin + (reflectTintMax-reflectTintMin)*frac
		dark := 1 - reflectDark*frac
		out := gfx.RGB{
			R: (sr + (water.R-sr)*tint) * dark,
			G: (sg + (water.G-sg)*tint) * dark,
			B: (sb + (water.B-sb)*tint) * dark,
		}
		blendPixel(img, w, h, x, yr, out, reflectAlpha*(1-reflectFade*frac))
	}
}

const (
	// The foot bulge is a "negative contour" below the baseline that gives the range a
	// 3D body. The foot is almost FLAT — a thin near-constant base — with the variation
	// coming from occasional outward "arms": a sparse, independent low-frequency noise
	// (thresholded, so it is nothing most of the way and flares into a wide bump here and
	// there). The peak height is linked only very weakly, so the underside never mirrors
	// the skyline. The foot fades out only at the range's true lateral ends (over a couple
	// of pixels of peak height), not in the valleys between peaks. All depths are fractions
	// of maxAlt, so the foot tracks resolution and the range's altitude scale.
	bulgeEdgePx       = 2.5   // peak height (px) over which the foot fades in at the very edges
	bulgeFlatFrac     = 0.012 // the thin, near-constant flat base depth (× maxAlt)
	bulgeArmFreq      = 0.012 // low frequency → wide, occasional arms
	bulgeArmOct       = 2     //
	bulgeArmThreshold = 0.60  // only noise above this flares into an arm (keeps arms sparse)
	bulgeArmFrac      = 0.075 // how far an arm extends the foot outward (× maxAlt)
	bulgeWeakFrac     = 0.03  // the weak, very small tie to peak height (× hcol)
	bulgeSeedOffset   = 86711 // decorrelate the foot contour from the peak/texture noise

	rangeBulgeShade = 0.25 // darken the underside toward its lowest point (shadowed foot)
)

// footBulgeDepth is how far (px) the foot bulges below the baseline for a column whose
// peak rises hcol px, given the range's altitude scale and a per-range seed. It is a
// thin flat base plus an occasional outward "arm" (sparse, independent noise) plus a
// very weak nudge from the peak height — so the underside reads as a nearly flat foot
// with the odd arm flowing outward, never a mirror of the skyline. It fades to nothing
// only at the range's lateral ends. Zero peak → no foot.
func footBulgeDepth(hcol, maxAlt float64, x, bulgeSeed int) float64 {
	if hcol <= 0 || maxAlt <= 0 {
		return 0
	}
	edge := smoothstep(0, bulgeEdgePx, hcol) // fade in only at the true lateral edge

	flat := bulgeFlatFrac * maxAlt

	// Occasional outward arm: a sparse, independent bump that is zero until the noise
	// clears a threshold, then rises into a wide flare — uncorrelated with the skyline.
	n := gfx.FBM(float64(x)*bulgeArmFreq, 0, bulgeSeed, bulgeArmOct)
	arm := 0.0
	if n > bulgeArmThreshold {
		arm = (n - bulgeArmThreshold) / (1 - bulgeArmThreshold) * bulgeArmFrac * maxAlt
	}

	weak := bulgeWeakFrac * hcol // very small, very weak tie to the peak

	return edge * (flat + arm + weak)
}
