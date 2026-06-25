package scene

import (
	"image"
	"math"
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
)

// Generate resolves the scene's extra ranges into a single entity. It reads the
// resolved base parameters from the globals (Context.MountainRanges), rolls a count
// for the vantage, and varies each range's foot depth, height, and smoothness around
// the base. All randomness is drawn here, in a fixed order, on the element stream; it
// draws nothing, so identical globals always yield an identical scene list. An empty
// list means the scene has no extra ranges (zero-value globals, a failed chance roll,
// or no room).
func (m *MountainRanges) Generate(c *Context) (SceneList, error) {
	mr := c.MountainRanges
	if mr.Chance <= 0 || mr.CountMax <= 0 {
		return nil, nil // no extra ranges configured (e.g. the v0 director)
	}
	horizon := c.Settings.HorizonY
	w, h := c.W, c.H
	groundH := h - horizon
	if horizon < 4 || groundH < mountainRangesMinGround {
		return nil, nil // no sky to rise into, or no ground to recede across
	}
	if c.Rng.Float64() >= mr.Chance {
		return nil, nil // this scene has no extra ranges
	}

	n := 1 + c.Rng.Intn(mr.CountMax)
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
		frac := mr.BaselineMaxFrac*float64(i+1)/float64(n) + c.Rng.NormFloat64()*mr.BaselineJitterFrac
		baseline := horizon + int(math.Round(clamp(frac, 0, 2)*float64(groundH)))
		baseline = clampInt(baseline, horizon+1, h-1+int(maxAlt))

		// Nearer ranges (larger depth below the horizon) are taller, and their altitude
		// color scale grows with them so a near range still spans base→peak the same way.
		depth := clamp(float64(baseline-horizon)/float64(groundH), 0, 2)
		heightScale := 1 + rangeNearHeightGain*depth
		bandMaxAlt := maxAlt * heightScale

		smoothness := clamp01(mr.SmoothnessMean + c.Rng.NormFloat64()*mr.SmoothnessStd)
		heightPx := math.Min(math.Max(mr.HeightMeanFrac+c.Rng.NormFloat64()*mr.HeightStdFrac, 0), mountainHeightMax) * sky * heightScale
		hmap := mountainHeights(c.Rng, w, smoothness, heightPx)
		// Bound the range to the land at its own depth so no part of its foot stands in
		// water; with no ocean the whole ground is land and the range spans the width.
		if hasOcean {
			applyCoastEnvelope(c.Rng, hmap, c.Ocean, baseline, w)
		}
		grad := buildMountainGradient(c.Rng, c.GroundGradient)
		texSeed := c.Rng.Int()

		// Per-column foot-bulge depth, clipped at the shoreline so the foot never swells
		// into nearer water (the renderer draws after the ocean, so an unclipped bulge
		// would paint over the sea). Also record the waterline (shore) where the foot
		// meets water, so the renderer can reflect the range there. Both are baked because
		// RenderList must stay seed-independent and cannot read the ocean model.
		bulges := make([]float64, w)
		shore := make([]int, w)
		bulgeSeed := texSeed + bulgeSeedOffset
		searchExtra := int(reflectShoreExtraFrac * sky)
		for x := range w {
			d := footBulgeDepth(hmap[x], bandMaxAlt, x, bulgeSeed)
			if d > 0 && hasOcean {
				footRow := baseline + int(math.Ceil(d))
				searchTo := min(footRow+searchExtra, h-1)
				for y := baseline + 1; y <= searchTo; y++ {
					if !c.LandAt(x, y) { // water in front of the foot
						shore[x] = y
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

	var water gfx.RGB
	if hasOcean {
		water = c.Ocean.color
	}
	return SceneList{mountainRangesToEntity(bands, water)}, nil
}

// RenderList draws the extra-range entity onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. Each range animates column-by-column, far range first.
func (m *MountainRanges) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	bands, water, err := entityToMountainRanges(list[0])
	if err != nil {
		return err
	}
	if len(bands) == 0 {
		return nil
	}
	w, h := c.W, c.H
	shade := mountainShader(c.MountainRugged)

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

	for i, b := range bands {
		floor := clipFloor[i]
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
					drawShadedRangeColumn(img, w, h, x, b.baseline, b.heights, bandBulge(b, x), b.maxAlt, b.grad, b.texSeed, shade, floor[x], bandShore(b, x), water)
				}
			})
			if err := sleep(c.Ctx, per); err != nil {
				return err
			}
		}
	}
	return nil
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
