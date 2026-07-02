package scene

import (
	"image"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// MountainRanges draws the extra mountain ranges: receding ridgelines that fill the
// midground below the horizon, behind the city. Their base parameters come from the
// globals (resolved per vantage by the director) and each range varies a little from
// that base by a normal distribution, so a scene reads as several layered ridges of
// differing height and jaggedness. Every range is bounded to the land at its own
// depth (the same coastline envelope the horizon range uses) and its foot bulge is
// clipped at the shoreline, so no range sits in water. The ranges render last — after
// the ocean — so a coastal range occludes the sea behind it rather than being painted
// over by the water; the foot is shoreline-clipped so it does not spill onto nearer
// water in turn. Colors are chosen exactly like the horizon range.
//
// This is a v1-era element (it reads the v1 globals); it draws from its own
// "mountainranges" stream, so it never disturbs another element's randomness.
type MountainRanges struct{}

func (m *MountainRanges) Name() string { return "mountainranges" }

// Schemas lists the entity schema keys this element owns.
func (m *MountainRanges) Schemas() []string { return []string{SchemaMountainRangesV0} }

const (
	mountainRangesAnimDuration = 600 * time.Millisecond
	// A range needs at least this many rows of ground below the horizon to bother
	// drawing into (a sliver of ground has no room for receding ridges).
	mountainRangesMinGround = 8
	// Nearer ranges are taller (perspective): a range's height and altitude scale grow
	// by 1 + rangeNearHeightGain·depth, where depth is 0 at the horizon and ~1 at the
	// bottom edge — so the foreground reads as big near mountains receding to small far
	// ridges rather than a uniform field of bumps.
	rangeNearHeightGain = 3.0

	// The mist settles in slowly over this total time (split across its bands), drawn
	// from each band's opaque core out to its transparent edges in this many steps.
	mistAnimDuration = 1200 * time.Millisecond
	mistAnimSteps    = 24
)

// Generate resolves the scene's extra ranges into a single entity. It reads the
// resolved base parameters from the globals (Context.MountainRanges), rolls a count
// for the vantage, and varies each range's foot depth, height, and smoothness around
// the base. All randomness is drawn here, in a fixed order, on the element stream; it
// draws nothing, so identical globals always yield an identical scene list. An empty
// list means the scene has no extra ranges (zero-value globals, a failed chance roll,
// or no room).
func (m *MountainRanges) Generate(c *Context) (SceneList, error) {
	bands, sc, ok := resolveMountainRangeBands(c.Rng, c)
	if !ok {
		return nil, nil
	}
	return SceneList{mountainRangesToEntity(bands, sc)}, nil
}

// resolveMountainRangeBands is the shared, pure resolver behind the mountainranges.v0
// element: from a random stream and the scene context (the resolved base parameters,
// settings, ocean, and ground gradient) it produces the ordered (far→near) ranges plus
// the scene-level data (water tint, mist flag). It draws all of the element's randomness
// here, in a fixed order, on the stream it is given; it draws nothing onto the canvas.
// ok is false (and the slices nil) when the scene has no extra ranges — no ranges
// configured, no room, or a failed chance roll.
//
// The element's Generate calls it on its own "mountainranges" stream; newContext calls
// it on an independently derived copy of that same stream (to build the bush floor), so
// both see byte-identical ranges without either disturbing the other.
func resolveMountainRangeBands(rng *rand.Rand, c *Context) ([]mountainRangeBand, rangesScene, bool) {
	mr := c.MountainRanges
	if mr.Chance <= 0 || mr.CountMax <= 0 {
		return nil, rangesScene{}, false // no extra ranges configured (e.g. the v0 director)
	}
	horizon := c.Settings.HorizonY
	w, h := c.W, c.H
	groundH := h - horizon
	if horizon < 4 || groundH < mountainRangesMinGround {
		return nil, rangesScene{}, false // no sky to rise into, or no ground to recede across
	}
	if rng.Float64() >= mr.Chance {
		return nil, rangesScene{}, false // this scene has no extra ranges
	}

	n := 1 + rng.Intn(mr.CountMax)
	sky := float64(horizon)
	// Color is normalized by the largest possible range (as the horizon range is), so
	// short ranges stay ground-colored and only tall ones reach the white peak.
	maxAlt := mountainHeightMax * sky

	hasOcean := c.Ocean != nil && c.Ocean.present
	bands := make([]mountainRangeBand, 0, n)
	for i := range n {
		// Feet spread from the horizon (far) down to BaselineMaxFrac of the ground
		// (near), one step per range, jittered so the spacing is uneven. From a high
		// vantage BaselineMaxFrac can exceed 1, putting the nearest feet at — or just
		// below — the bottom edge, so the foot may sit off-screen while the peak still
		// rises into view; cap it where even a full-height peak would no longer show.
		frac := mr.BaselineMaxFrac*float64(i+1)/float64(n) + rng.NormFloat64()*mr.BaselineJitterFrac
		baseline := horizon + int(math.Round(clamp(frac, 0, 2)*float64(groundH)))
		baseline = clampInt(baseline, horizon+1, h-1+int(maxAlt))

		// Nearer ranges (larger depth below the horizon) are taller, and their altitude
		// color scale grows with them so a near range still spans base→peak the same way.
		depth := clamp(float64(baseline-horizon)/float64(groundH), 0, 2)
		heightScale := 1 + rangeNearHeightGain*depth
		bandMaxAlt := maxAlt * heightScale

		smoothness := clamp01(mr.SmoothnessMean + rng.NormFloat64()*mr.SmoothnessStd)
		heightPx := math.Min(math.Max(mr.HeightMeanFrac+rng.NormFloat64()*mr.HeightStdFrac, 0), mountainHeightMax) * sky * heightScale
		hmap := mountainHeights(rng, w, smoothness, heightPx)
		// Bound the range to the land at its own depth so no part of its foot stands in
		// water; with no ocean the whole ground is land and the range spans the width.
		if hasOcean {
			applyCoastEnvelope(rng, hmap, c.Ocean, baseline, w)
		}
		grad := buildMountainGradient(rng, c.GroundGradient)
		texSeed := rng.Int()

		// Per-column foot-bulge depth, clipped at the shoreline so the foot never swells
		// into nearer water (the renderer draws after the ocean, so an unclipped bulge
		// would paint over the sea). Also record the waterline (shore) where the foot
		// meets water, so the renderer can reflect the range there. Both are baked because
		// RenderList must stay seed-independent and cannot read the ocean model.
		bulges := make([]float64, w)
		shore := make([]int, w)
		bulgeSeed := texSeed + bulgeSeedOffset
		searchExtra := int(reflectShoreExtraFrac * sky)
		minReflect := minReflectFrac * sky // only peaks this tall cast a reflection
		for x := range w {
			d := footBulgeDepth(hmap[x], bandMaxAlt, x, bulgeSeed)
			if d > 0 && hasOcean {
				footRow := baseline + int(math.Ceil(d))
				searchTo := min(footRow+searchExtra, h-1)
				for y := baseline + 1; y <= searchTo; y++ {
					if !c.LandAt(x, y) { // water in front of the foot
						if hmap[x] >= minReflect { // a visible peak: reflect it (no dashes)
							shore[x] = y
						}
						if y <= footRow { // the foot would cross water: clip it back
							d = math.Max(float64(y-1-baseline), 0)
						}
						break
					}
				}
			}
			bulges[x] = d
		}

		bands = append(bands, mountainRangeBand{
			baseline: baseline,
			heights:  hmap,
			grad:     grad,
			texSeed:  texSeed,
			maxAlt:   bandMaxAlt,
			bulges:   bulges,
			shore:    shore,
		})
	}
	// Draw back-to-front: the highest foot (nearest the horizon) first, the lowest
	// (nearest the viewer) last, so a nearer ridge occludes the one behind it. The
	// jitter can reorder the feet, so sort rather than trust the loop order.
	sort.SliceStable(bands, func(i, j int) bool { return bands[i].baseline < bands[j].baseline })

	sc := rangesScene{}
	if hasOcean {
		sc.water = c.Ocean.color
	}
	// Ground mist appears only when this scene both rolled mist on (globals) and has
	// foreground ranges. Its horizontal extent is derived per range at render time from
	// each range's own (coastline-bounded) footprint — nothing ocean-specific to bake.
	if c.Mist.Present && len(bands) > 0 {
		sc.mist = true
	}
	return bands, sc, true
}

// mistBandFade is the per-column horizontal factor for one range's mist band: 1 over
// every column where the range actually drew a mountain (its coastline-bounded
// footprint), falling off to 0 with horizontal distance beyond it, so the mist extends
// a little past the range and then fades away — over the open ocean, or anywhere the
// range has no mountains. fadeDist is the falloff distance in columns. A range with no
// mountains (e.g. only open water at its depth) yields all zeros, so no band is drawn.
func mistBandFade(heights []float64, fadeDist float64) []float64 {
	w := len(heights)
	mask := make([]bool, w)
	for x := range heights {
		mask[x] = heights[x] > 0
	}
	dist := nearestTrueDistance(mask)
	fade := make([]float64, w)
	for x := range w {
		if mask[x] {
			fade[x] = 1
			continue
		}
		if fadeDist > 0 {
			fade[x] = clamp(1-float64(dist[x])/fadeDist, 0, 1)
		}
	}
	return fade
}

// nearestTrueDistance returns, for each index, the distance (in columns) to the
// nearest true entry in mask (0 at a true entry). A mask with no true entries yields a
// large distance everywhere.
func nearestTrueDistance(mask []bool) []int {
	n := len(mask)
	const big = 1 << 30
	out := make([]int, n)
	d := big
	for x := range n { // forward: distance to the nearest true on the left
		if mask[x] {
			d = 0
		} else if d < big {
			d++
		}
		out[x] = d
	}
	d = big
	for x := n - 1; x >= 0; x-- { // backward: take the nearer side
		if mask[x] {
			d = 0
		} else if d < big {
			d++
		}
		if d < out[x] {
			out[x] = d
		}
	}
	return out
}

// RenderList draws the extra-range entity onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. Each range animates column-by-column, far range first.
func (m *MountainRanges) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	bands, sc, err := entityToMountainRanges(list[0])
	if err != nil {
		return err
	}
	if len(bands) == 0 {
		return nil
	}
	w, h := c.W, c.H
	shade := mountainShader(c.MountainRugged)
	// One slope window for every range, from the scene's BASE altitude scale — NOT each
	// band's height-scaled maxAlt, which would widen the window on the taller near ranges
	// until it spanned several peaks and the left/right shading washed out (see
	// slopeWindow). The heightmaps all share the same horizontal peak structure.
	slopeWin := slopeWindow(mountainHeightMax * float64(c.Settings.HorizonY))

	// Occlusion floor: a rear range's lower contour (foot + bulge) must never dip below
	// the lower contour of any range in front of it, or a farther ridge would appear
	// beneath a nearer one. clipFloor[i][x] is the lowest row range i may draw to — the
	// minimum lower edge over every range nearer than i (capped at the bottom edge). The
	// nearest range (last, largest baseline) is unconstrained. bands are ordered
	// far→near, so a single back-to-front suffix-min builds it.
	clipFloor := make([][]int, len(bands))
	running := make([]int, w)
	for x := range running {
		running[x] = h - 1 // the nearest range may draw all the way to the bottom edge
	}
	for i := len(bands) - 1; i >= 0; i-- {
		floor := make([]int, w)
		copy(floor, running)
		clipFloor[i] = floor
		b := bands[i]
		for x := range w {
			lower := b.baseline + int(math.Ceil(bandBulge(b, x)))
			if lower < running[x] {
				running[x] = lower
			}
		}
	}

	batch := max(w/mountainsAnimCols, 1)
	// Split the animation budget across the ranges so the whole set draws in about the
	// same time regardless of count.
	per := mountainRangesAnimDuration / time.Duration(len(bands)*((w+batch-1)/batch))
	if per <= 0 {
		per = time.Millisecond
	}

	// The shade reads the slope of the un-stretched shape: divide it by how much taller
	// than the base scale this range is drawn (perspective), so a near range shades like
	// the far horizon range rather than saturating into vertical light/dark.
	baseMaxAlt := mountainHeightMax * float64(c.Settings.HorizonY)

	// Ground mist: an atmospheric-haze band drawn after each range (starting with the
	// horizon range), opaque from a range's foot down to the next range's foot and
	// fading up over its slopes, so peaks emerge from the fog (see drawMistBand). The
	// last range's band reaches the scene bottom from a high vantage, or fades back out
	// just below the range from a low one.
	horizon := c.Settings.HorizonY
	var mistColor gfx.RGB
	var mountainFloor []int
	fadeUpH, landFadeH := 0, 0
	fadeDist := 0.0
	// Each mist band animates over an equal share of the total mist time (one band per
	// range plus the horizon range's band).
	mistDur := mistAnimDuration / time.Duration(len(bands)+1)
	if sc.mist {
		mistColor = c.SkyGradient.At(0).RGB()
		fadeUpH = max(int(c.Mist.FadeUpFrac*float64(horizon)), 1)
		landFadeH = max(int(c.Mist.LowFadeFrac*float64(h-horizon)), 1)
		fadeDist = c.Mist.OceanFadeFrac * float64(w)
		// The deepest row any range reaches at each column (across all ranges), so the
		// opaque mist holds down to the mountains and then fades out below them where
		// there is none — e.g. over the open ocean beneath an island — instead of running
		// solid to the next range / the bottom. Dilated horizontally by the fade distance
		// so small ridge valleys do not punch holes in the fog.
		mountainFloor = mistMountainFloor(bands, w, h, horizon, int(fadeDist))
		// The horizon range (already painted, its foot at the horizon) gets the first
		// band, opaque down to the first extra range's foot. It has no heightmap of its
		// own here, so it borrows the nearest extra range's footprint for its extent.
		if err := drawMistBand(c, mistColor, mistBandFade(bands[0].heights, fadeDist), mountainFloor, horizon, bands[0].baseline, landFadeH, fadeUpH, mistDur); err != nil {
			return err
		}
	}

	for i, b := range bands {
		floor := clipFloor[i]
		heightScale := 1.0
		if baseMaxAlt > 0 {
			heightScale = b.maxAlt / baseMaxAlt
		}
		for x0 := 0; x0 < w; x0 += batch {
			if err := c.Ctx.Err(); err != nil {
				return err
			}
			x1 := min(x0+batch, w)
			c.Canvas.Draw(func(img *image.RGBA) {
				for x := x0; x < x1; x++ {
					// The shaded peak plus the (shoreline-clipped) foot bulge — a 3D, sloped
					// silhouette swelling downward near peaks (see drawShadedRangeColumn) —
					// with the foot clipped to floor[x] so it never shows below a nearer range,
					// then its reflection mirrored into the water at shore[x].
					drawShadedRangeColumn(img, w, h, x, b.baseline, b.heights, bandBulge(b, x), b.maxAlt, b.grad, b.texSeed, slopeWin, heightScale, shade, floor[x], bandShore(b, x), sc.water)
				}
			})
			if err := sleep(c.Ctx, per); err != nil {
				return err
			}
		}
		if sc.mist {
			if err := c.Ctx.Err(); err != nil {
				return err
			}
			// The band would fill opaque down to the next range's foot (or, for the front
			// range, the bottom of the scene). The per-column mountain floor then pulls it
			// up wherever no mountain actually reaches that far — so over land it stays
			// solid to the next range / the bottom, and over ocean it fades out under the
			// mountains.
			opaqueEnd := h - 1
			if i < len(bands)-1 {
				opaqueEnd = bands[i+1].baseline
			}
			if err := drawMistBand(c, mistColor, mistBandFade(b.heights, fadeDist), mountainFloor, b.baseline, opaqueEnd, landFadeH, fadeUpH, mistDur); err != nil {
				return err
			}
		}
	}
	return nil
}

// mistMountainFloor returns, per column, the deepest row any range reaches there (its
// foot, baseline + bulge), across all ranges — clamped to the bottom edge and to no
// shallower than the horizon. Columns no range covers stay at the horizon. The result
// is then dilated horizontally by `dilate` columns (a max filter) so a narrow ridge
// valley — or a column just off the edge of a range — still counts as covered, letting
// the fog hold together over the mountains and reach a little past them.
func mistMountainFloor(bands []mountainRangeBand, w, h, horizon, dilate int) []int {
	raw := make([]int, w)
	for x := range raw {
		raw[x] = horizon
	}
	for _, b := range bands {
		for x := range w {
			if b.heights[x] > 0 {
				if foot := b.baseline + int(math.Ceil(bandBulge(b, x))); foot > raw[x] {
					raw[x] = foot
				}
			}
		}
	}
	for x := range raw {
		if raw[x] > h-1 {
			raw[x] = h - 1
		}
	}
	if dilate <= 0 {
		return raw
	}
	out := make([]int, w)
	for x := range w {
		lo, hi := max(x-dilate, 0), min(x+dilate, w-1)
		m := raw[x]
		for i := lo; i <= hi; i++ {
			if raw[i] > m {
				m = raw[i]
			}
		}
		out[x] = m
	}
	return out
}

// drawMistBand paints one horizontal band of ground mist (color, an atmospheric haze)
// over the canvas. Per column it is opaque from base down to that column's opaque floor
// — the lesser of opaqueEnd and the mountain floor there — fading to nothing over the
// next landFadeH rows, and fading up over fadeUpH rows above base so a range's peaks
// emerge from the fog. So the mist holds solid only where a mountain actually reaches,
// and dissolves below the mountains over open water. Each column is also scaled by the
// per-range horizontal fade (see mistBandFade) so the band hugs that range's footprint.
// It animates the band in over dur, drawing its rows most-opaque first so the fog fills
// from its solid core out to its transparent edges. Returns ctx.Err() if cancelled.
func drawMistBand(c *Context, color gfx.RGB, fade []float64, mountainFloor []int, base, opaqueEnd, landFadeH, fadeUpH int, dur time.Duration) error {
	w, h := c.W, c.H
	// Per-column opaque floor and the row each column's fade reaches.
	floorY := make([]int, w)
	top, bottom := max(base-fadeUpH, 0), base
	for x := range w {
		fl := min(opaqueEnd, max(base, columnMountainFloor(mountainFloor, x)))
		floorY[x] = fl
		if fl+landFadeH > bottom {
			bottom = fl + landFadeH
		}
	}
	bottom = min(bottom, h-1)
	if bottom < top {
		return nil
	}
	// op is the mist opacity at (x, y): the vertical profile (fade up, opaque to the
	// column's floor, fade down) scaled by the column's horizontal fade.
	op := func(x, y int) float64 {
		var a float64
		switch fl := floorY[x]; {
		case y < base:
			if fadeUpH > 0 {
				a = clamp(float64(y-(base-fadeUpH))/float64(fadeUpH), 0, 1)
			}
		case y <= fl:
			a = 1
		case landFadeH > 0 && y <= fl+landFadeH:
			a = clamp(1-float64(y-fl)/float64(landFadeH), 0, 1)
		}
		if x < len(fade) {
			a *= fade[x]
		}
		return a
	}

	type mistRow struct {
		y     int
		maxOp float64
	}
	rows := make([]mistRow, 0, bottom-top+1)
	for y := top; y <= bottom; y++ {
		var mo float64
		for x := range w {
			if o := op(x, y); o > mo {
				mo = o
			}
		}
		if mo > 0 {
			rows = append(rows, mistRow{y, mo})
		}
	}
	if len(rows) == 0 {
		return nil
	}
	// Most-opaque rows first, transparent edges last (stable, so equal rows keep order).
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].maxOp > rows[j].maxOp })

	batchN := max(len(rows)/mistAnimSteps, 1)
	steps := (len(rows) + batchN - 1) / batchN
	per := dur / time.Duration(max(steps, 1))
	if per <= 0 {
		per = time.Millisecond
	}
	for i0 := 0; i0 < len(rows); i0 += batchN {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		i1 := min(i0+batchN, len(rows))
		c.Canvas.Draw(func(img *image.RGBA) {
			for _, r := range rows[i0:i1] {
				for x := range w {
					if a := op(x, r.y); a > 0 {
						blendPixel(img, w, h, x, r.y, color, a)
					}
				}
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// columnMountainFloor reads the mountain floor at column x, treating a missing slice
// (e.g. a no-mist path) as "unbounded" so the caller's opaqueEnd governs.
func columnMountainFloor(mountainFloor []int, x int) int {
	if x < len(mountainFloor) {
		return mountainFloor[x]
	}
	return 1 << 30
}

// bandBulge is the foot-bulge depth (px) for column x of a band: the baked, shoreline-
// clipped value when present, else the unclipped foot contour (a scene list predating
// the baked field).
func bandBulge(b mountainRangeBand, x int) float64 {
	if x < len(b.bulges) {
		return b.bulges[x]
	}
	return footBulgeDepth(b.heights[x], b.maxAlt, x, b.texSeed+bulgeSeedOffset)
}

// bandShore is the waterline row for column x of a band (0 = no water in front of the
// foot, so no reflection there). 0 for a band whose shore was never baked.
func bandShore(b mountainRangeBand, x int) int {
	if x < len(b.shore) {
		return b.shore[x]
	}
	return 0
}
