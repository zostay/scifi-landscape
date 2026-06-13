package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Clouds draws atmospheric cloud layers in the sky. It sits in front of the
// stars, suns, and planets (clouds are in our own atmosphere, much nearer than
// anything celestial) but behind the horizon terrain (mountains, ground, cities)
// and the water, so the ocean reflects whatever clouds are above the horizon.
//
// Two kinds of layer are drawn, and a scene may have either, both, or neither —
// the sky is deliberately kept mostly clear (thin/sparse coverage is favored so
// the clouds never wall off the sky):
//
//   - A HIGH gauzy layer: a single sheet of mostly-transparent puffs/ripples
//     covering the whole sky, built like the ground base layer (fractal noise,
//     stretched hard toward the horizon for a sense of distance).
//   - LOW nimbus clouds: discrete, flat-bottomed, puff-ball-topped clouds drawn
//     in a few depth layers. Far layers sit just above the horizon with small
//     clouds and are drawn first; nearer layers sit higher with larger clouds and
//     are drawn last (over the farther ones).
type Clouds struct{}

func (cl *Clouds) Name() string { return "clouds" }

// Schemas lists the entity schema keys the clouds element owns.
func (cl *Clouds) Schemas() []string {
	return []string{SchemaCloudsHighV0, SchemaCloudLowV0}
}

const (
	cloudsAnimDuration = 1000 * time.Millisecond

	// Each kind of layer appears independently; both biased so the sky is clear
	// as often as not.
	cloudHighChance = 0.45
	cloudLowChance  = 0.45

	// High gauzy layer. Coverage is sparse: only noise above a (per-scene) high
	// threshold shows, and even then at a low maximum opacity, so the layer reads
	// as thin haze/ripples rather than overcast.
	cloudHighOctaves   = 4
	cloudHighFreqX     = 0.006 // broad puffs across the sky
	cloudHighFreqY     = 0.010
	cloudHighStretch   = 9.0 // vertical frequency multiplier at the horizon
	cloudHighStretchPw = 2.2 // how fast the stretch eases toward the zenith
	cloudHighThreshLo  = 0.46
	cloudHighThreshHi  = 0.60
	cloudHighSoft      = 0.30 // coverage ramp width above the threshold
	cloudHighAlphaLo   = 0.18
	cloudHighAlphaHi   = 0.38

	// Low nimbus clouds. Each cloud is a HEIGHT FIELD over its bounding box: a
	// smooth flat-based envelope, lumped up by fractal inverted-Worley billows
	// (multi-octave cauliflower, see cloudBillows) and Perlin-FBM detail, then
	// eroded at the edges so the silhouette is ragged. The field is bump-mapped and
	// lit with the same dominant-sun model the planets use (light direction from
	// the twinkle angle, tinted by LightColor).
	cloudLowMaxLayers  = 3
	cloudHeightRatio   = 0.45 // cloud height as a fraction of its width (wider than tall)
	cloudBottomFeather = 5.0  // flat-bottom softening, in pixels
	cloudLowAlpha      = 0.96 // low clouds are nearly opaque (sparsity, not haze)

	// The flat base isn't a straight line: it is nearly flat across the middle and
	// rounds gently up only near the edges (a smoothstep eases the rise in and out
	// so the tips are rounded, not sharp), with a shallow low-frequency scallop so
	// it never reads as a hard horizontal cut.
	cloudBaseArcEdge  = 0.07 // edge rise as a fraction of cloud height (small: nearly flat)
	cloudBaseArcFlat  = 0.45 // fraction of the half-width that stays flat before rounding up
	cloudBaseArcNoise = 0.04 // scallop depth as a fraction of cloud height
	cloudBaseArcCells = 2.5  // scallop arcs across the cloud width

	// Cauliflower billows: fractal (multi-octave) inverted Worley. The base bulb
	// size is capped in PIXELS (so a big cloud gets more bulbs, not bigger ones —
	// the straight cell joins that show when one octave is stretched over a large
	// cloud), and bigger clouds get MORE octaves, layering progressively smaller
	// metaballs to build up the cauliflower texture.
	cloudWorleyBaseCells = 4.0   // billow cells across a small cloud's width
	cloudMaxBulbPx       = 42.0  // largest bulb diameter (px); big clouds add cells to keep this
	cloudBillowBaseOct   = 2     // octaves for a small cloud
	cloudBillowMaxOct    = 5     // octave cap
	cloudBillowRefW      = 150.0 // width at which octaves start ramping up with size
	cloudBillowGain      = 0.55  // amplitude falloff per finer octave

	cloudFBMCells    = 2.5  // Perlin-FBM base cells across the cloud width
	cloudFBMOct      = 5    // Perlin-FBM octaves (fine erosion detail)
	cloudBillowAmp   = 0.60 // how much the Worley billows lump the surface
	cloudFBMAmp      = 0.40 // how much Perlin-FBM ruffles/erodes the surface
	cloudErosion     = 0.22 // baseline erosion: thins and ragged-edges the cloud
	cloudNoiseGate   = 0.50 // envelope depth over which lumps fade in (kills floaters)
	cloudHeightDecay = 1.0  // how fast the envelope thins toward the top
	cloudEdgeSoft    = 0.16 // coverage ramp width at the silhouette
	cloudBaseTaper   = 0.40 // lower fraction over which relief ramps up from the flat base

	cloudBumpStrength = 0.5  // surface-normal tilt from the height field (× cloud height)
	cloudLightZ       = 0.6  // sun elevation toward the viewer (out of screen)
	cloudTermW        = 0.45 // soft terminator half-width (clouds, not hard like planets)
	cloudTintAmt      = 0.6  // how strongly the dominant star's color tints the lit side
)

// highClouds is the resolved high gauzy sheet: all the random draws for the
// whole-sky layer baked into plain values, so the drawing pass consumes no
// randomness. It is the internal mirror of CloudsHighV0.
type highClouds struct {
	seed     int
	thresh   float64
	maxAlpha float64
	col      gfx.HSV
}

// Generate resolves the scene's cloud layers into entities. It performs every
// cloud random draw on the element stream, in the same order the original
// interleaved drawing did, and has no side effects (it draws nothing), so
// identical globals always yield an identical scene list. The high gauzy sheet,
// when present, is the first entity; each nimbus cloud follows in draw order
// (farthest first). An empty list means a clear sky.
func (cl *Clouds) Generate(c *Context) (SceneList, error) {
	horizon := c.Settings.HorizonY
	if horizon < 8 {
		return nil, nil // essentially no sky to put clouds in
	}
	rng := c.Rng

	// The two layer rolls come first, before either layer's own draws, so the
	// stream order is preserved exactly.
	hasHigh := rng.Float64() < cloudHighChance
	hasLow := rng.Float64() < cloudLowChance

	var list SceneList
	if hasHigh {
		list = append(list, highToEntity(generateHighClouds(c)))
	}
	if hasLow {
		for _, cd := range generateLowClouds(c, horizon) {
			list = append(list, cloudToEntity(cd))
		}
	}
	return list, nil
}

// RenderList draws the cloud entities onto the canvas. It is the only step that
// touches the image and it consumes no randomness, so the same scene list always
// draws the same pixels. The high sheet (if present) is drawn first, then each
// nimbus cloud in list order (farthest first). Entities that are not clouds are
// an error.
func (cl *Clouds) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	horizon := c.Settings.HorizonY

	// Determine which layers are present so the animation budget is split exactly
	// as the original did (hasHigh && hasLow halves each layer's share).
	var high *CloudsHighV0
	var lows []*CloudLowV0
	for _, e := range list {
		switch v := e.(type) {
		case *CloudsHighV0:
			high = v
		case *CloudLowV0:
			lows = append(lows, v)
		default:
			return errNotCloud(e)
		}
	}

	hasHigh, hasLow := high != nil, len(lows) > 0
	share := cloudsAnimDuration
	if hasHigh && hasLow {
		share /= 2
	}

	if hasHigh {
		if err := renderHighClouds(c, horizon, share, entityToHigh(high)); err != nil {
			return err
		}
	}
	if hasLow {
		clouds := make([]cloud, len(lows))
		for i, e := range lows {
			clouds[i] = entityToCloud(e)
		}
		if err := renderLowClouds(c, horizon, share, clouds); err != nil {
			return err
		}
	}
	return nil
}

// generateHighClouds resolves the high gauzy sheet's random parameters in the
// exact draw order the original interleaved version used: noise seed, coverage
// threshold, peak alpha, then the time-of-day color.
func generateHighClouds(c *Context) highClouds {
	rng := c.Rng
	return highClouds{
		seed:     rng.Int(),
		thresh:   rnd(rng, cloudHighThreshLo, cloudHighThreshHi),
		maxAlpha: rnd(rng, cloudHighAlphaLo, cloudHighAlphaHi),
		col:      cloudColorHigh(rng, c.Settings.Time),
	}
}

// renderHighClouds draws the resolved high gauzy sheet: fractal noise over the
// whole sky, thresholded to sparse coverage and stretched toward the horizon,
// alpha-blended over the sky so it only thins the view rather than walling it
// off. It consumes no randomness.
func renderHighClouds(c *Context, horizon int, budget time.Duration, h highClouds) error {
	w := c.W
	seed, thresh, maxAlpha, col := h.seed, h.thresh, h.maxAlpha, h.col

	// Per-row vertical sample coordinate, stretched hard near the horizon (so
	// features there squeeze into thin, far-looking ripples) and relaxing toward
	// the zenith — the same trick the ground uses for receding distance.
	vy := cloudDepthWarp(horizon)

	const bands = 80
	bandH := max(horizon/bands, 1)
	per := budget / time.Duration((horizon+bandH-1)/bandH)

	for y0 := 0; y0 < horizon; y0 += bandH {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y1 := min(y0+bandH, horizon)
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				for x := range w {
					n := gfx.FBM(float64(x)*cloudHighFreqX, vy[y], seed, cloudHighOctaves)
					cov := smoothstep(thresh, thresh+cloudHighSoft, n)
					if cov <= 0 {
						continue
					}
					// Denser cores read a touch brighter than wispy edges.
					cc := col
					cc.V *= 0.9 + 0.2*cov
					blendPixel(img, w, c.H, x, y, cc.RGB(), cov*maxAlpha)
				}
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// cloudDepthWarp returns the vertical noise coordinate for each sky row [0,
// horizon]. The step between rows grows toward the horizon (cloudHighStretch), so
// the noise climbs fast there and its features are compressed into thin
// horizontal streaks — distant cloud bands — easing to rounder puffs overhead.
func cloudDepthWarp(horizon int) []float64 {
	vy := make([]float64, horizon+1)
	acc := 0.0
	hf := float64(max(horizon, 1))
	for y := 0; y <= horizon; y++ {
		tb := float64(y) / hf // 0 at the top, 1 at the horizon
		m := 1 + (cloudHighStretch-1)*math.Pow(tb, cloudHighStretchPw)
		acc += cloudHighFreqY * m
		vy[y] = acc
	}
	return vy
}

// cloud is one resolved nimbus cloud: a flat-based height field over a bounding
// box, shaded between a lit and a shadow color. cx/baseY/cw/ch place and size it;
// seed drives its Worley + Perlin noise.
type cloud struct {
	cx, baseY   float64 // horizontal center and flat-bottom row
	cw, ch      float64 // width and height
	minX, maxX  int     // bounding box (rows run minY..baseY)
	minY        int
	lit, shadow gfx.HSV
	seed        int
}

// generateLowClouds builds the depth layers of nimbus clouds, flattened into a
// single farthest-first draw order (the layers are nested only so far/near sizing
// can be computed; the returned slice is the order they are drawn in). This is
// the only random part of the low layer; it preserves the original draw order
// exactly: per layer the cloud count, then per cloud cx, baseY, width, and the
// makeCloud draws (height, then seed).
func generateLowClouds(c *Context, horizon int) []cloud {
	w := c.W
	rng := c.Rng
	sky := float64(horizon)

	nLayers := lowCloudLayerCount(rng)
	lit, shadow := cloudColorsLow(rng, c.Settings.Time)

	var clouds []cloud
	for i := range nLayers {
		t := 0.3
		if nLayers > 1 {
			t = float64(i) / float64(nLayers-1) // 0 = farthest, 1 = nearest
		}
		// Far layers hug the horizon with small clouds; near layers ride higher
		// with larger clouds.
		baseY := sky - (0.04+0.46*t)*sky
		width := (0.07 + 0.20*t) * float64(w)
		nClouds := max(int(math.Round(rnd(rng, 2, 5)-2*t)), 1) // fewer, bigger clouds when near

		for range nClouds {
			cx := rng.Float64() * float64(w)
			by := baseY + rnd(rng, -0.03, 0.03)*sky
			cw := width * rnd(rng, 0.75, 1.3)
			clouds = append(clouds, makeCloud(rng, cx, by, cw, horizon, lit, shadow))
		}
	}
	return clouds
}

// renderLowClouds draws the resolved nimbus clouds from the farthest (lowest,
// smallest, first) to the nearest (highest, largest, last), so nearer clouds
// overlap farther ones. It consumes no randomness. All clouds in a scene share
// one light direction, derived (like the planets) from the twinkle angle and
// tinted by the dominant star's color — these come from Settings, not the random
// stream, so they are recomputed here.
func renderLowClouds(c *Context, horizon int, budget time.Duration, clouds []cloud) error {
	w := c.W
	ambient := cloudAmbient(c.Settings.Time)

	// Dominant-sun light direction, same convention as the planets: in-plane it
	// points up-and-to-the-left from the twinkle angle (so clouds are lit on the
	// same side as the planets), plus a fixed elevation toward the viewer.
	a := c.Settings.TwinkleAngle * math.Pi / 180
	lx, ly, lz := -math.Cos(a), -math.Sin(a), cloudLightZ
	li := 1 / math.Sqrt(lx*lx+ly*ly+lz*lz)
	light := [3]float64{lx * li, ly * li, lz * li}
	tint := c.Settings.LightColor

	per := budget / time.Duration(max(len(clouds), 1))

	for _, cd := range clouds {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		c.Canvas.Draw(func(img *image.RGBA) {
			drawCloud(img, w, c.H, horizon, cd, light, tint, ambient)
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// lowCloudLayerCount picks the number of nimbus depth layers, biased toward one
// (kept sparse).
func lowCloudLayerCount(rng *rand.Rand) int {
	switch r := rng.Float64(); {
	case r > 0.85:
		return min(3, cloudLowMaxLayers)
	case r > 0.50:
		return min(2, cloudLowMaxLayers)
	default:
		return 1
	}
}

// makeCloud resolves one flat-bottomed nimbus cloud: its placement, size, colors,
// noise seed, and bounding box. The actual shape is a procedural height field
// evaluated at draw time (see cloudHeight); here we only fix the parameters.
func makeCloud(rng *rand.Rand, cx, baseY, cw float64, horizon int, lit, shadow gfx.HSV) cloud {
	ch := cw * cloudHeightRatio * rnd(rng, 0.8, 1.2)
	half := cw / 2
	by := math.Min(baseY, float64(horizon-1))
	// A little margin past the envelope for billows that bulge beyond it.
	mx := 0.18 * cw
	return cloud{
		cx:     cx,
		baseY:  by,
		cw:     cw,
		ch:     ch,
		minX:   int(math.Floor(cx - half - mx)),
		maxX:   int(math.Ceil(cx + half + mx)),
		minY:   int(math.Floor(by - ch - 0.18*ch)),
		lit:    lit,
		shadow: shadow,
		seed:   rng.Int(),
	}
}

// cloudHeight is the cloud's procedural height field at world point (px,py): a
// smooth flat-based envelope (widest at the bottom, thinning to the top), lumped
// up by inverted Worley billows and Perlin-FBM detail, with a baseline erosion
// that thins the body and ragged-edges the silhouette. The value is the cloud's
// relief/thickness (>0 inside the cloud); it is the field both the silhouette
// (iso-level 0) and the bump-mapped lighting are derived from.
func cloudHeight(cd cloud, px, py float64) float64 {
	u := (px - cd.cx) / (cd.cw * 0.5)           // horizontal, ~[-1,1]
	vy := (cloudLocalBase(cd, px) - py) / cd.ch // 0 at the (arced) base, 1 at the top
	// Flat-based envelope: a downward parabola in x, thinning with height.
	env := (1 - u*u) - vy*cloudHeightDecay

	ff := cloudFBMCells / cd.cw
	billow := cloudBillows(px, py, cd.cw, cd.seed)                // fractal cauliflower lumps
	detail := gfx.PerlinFBM(px*ff, py*ff, cd.seed+7, cloudFBMOct) // organic erosion

	// Gate the lumps by the envelope: full inside the body, fading to nothing just
	// outside it, so a billow can ruffle the edge but never spawn a detached blob
	// floating above the cloud.
	noise := cloudBillowAmp*(billow-0.5) + cloudFBMAmp*(detail-0.5)
	gate := smoothstep(-cloudNoiseGate, 0.15, env)
	return env + noise*gate - cloudErosion
}

// cloudBillows is fractal inverted-Worley "cauliflower" noise in [0,1]. Each
// octave is 1 - F1² (rounded billow domes, no cones/points); octaves stack
// progressively smaller bulbs onto the larger ones. The base bulb size is capped
// in pixels so a wide cloud gets more bulbs (not bigger ones), and a wide cloud
// also gets more octaves — so near/large clouds keep the small-bulb cauliflower
// look of distant ones instead of dissolving into a few big metaballs with
// straight joins.
func cloudBillows(px, py, cw float64, seed int) float64 {
	cells := math.Max(cloudWorleyBaseCells, cw/cloudMaxBulbPx)
	base := cells / cw // base frequency (cells per pixel)
	oct := cloudBillowBaseOct
	if cw > cloudBillowRefW {
		oct += int(math.Log2(cw / cloudBillowRefW))
	}
	oct = min(oct, cloudBillowMaxOct)

	sum, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for i := 0; i < oct; i++ {
		f1 := gfx.Worley(px*base*freq, py*base*freq, seed+i*149)
		sum += amp * (1 - f1*f1)
		norm += amp
		amp *= cloudBillowGain
		freq *= 2
	}
	return sum / norm
}

// cloudLocalBase returns the cloud's flat-bottom row at column px. The base bows
// up toward the edges (a broad stretched arc) and carries a shallow low-frequency
// scallop, so the bottom is a soft set of arcs rather than a hard straight cut —
// kept shallow (a fraction of the cloud height) so it still reads as flat. The
// base only ever rises from baseY, so the cloud never reaches below it.
func cloudLocalBase(cd cloud, px float64) float64 {
	u := math.Abs((px - cd.cx) / (cd.cw * 0.5))
	// Flat across the middle, then a gentle rounded rise toward the edges. The
	// smoothstep eases the rise both where it starts and at the very tip, so the
	// arc tightens and rounds off at the edges instead of shooting up sharply.
	arc := cloudBaseArcEdge * cd.ch * smoothstep(cloudBaseArcFlat, 1.0, u)
	n := gfx.Perlin(px*cloudBaseArcCells/cd.cw, 11.7, cd.seed+53) // [-1,1]
	arc += cloudBaseArcNoise * cd.ch * (0.5 + 0.5*n)
	return cd.baseY - arc
}

// drawCloud rasterizes one nimbus cloud. It first bakes the height field over the
// cloud's bounding box, then makes a second pass that derives, per pixel: the
// silhouette (height iso-level 0, feathered, cut flat along the base), a
// bump-mapped surface normal from the field gradient (the relief is faded in from
// the flat base so the base reads as a thin shadowed lip, not a lit wall), and
// dominant-sun diffuse lighting like the planets — n·L with a soft terminator,
// the lit side tinted by the star color, the shadow filled by ambient.
func drawCloud(img *image.RGBA, w, h, horizon int, cd cloud, light [3]float64, tint gfx.RGB, ambient float64) {
	y0 := max(cd.minY, 0)
	y1 := min(int(cd.baseY)+1, horizon)
	x0 := max(cd.minX, 0)
	x1 := min(cd.maxX, w)
	bw, bh := x1-x0, y1-y0
	if bw <= 0 || bh <= 0 {
		return
	}

	// Bake the height field once so neighbor lookups (for the normal) are free.
	field := make([]float64, bw*bh)
	for j := range bh {
		py := float64(y0+j) + 0.5
		row := j * bw
		for i := range bw {
			field[row+i] = cloudHeight(cd, float64(x0+i)+0.5, py)
		}
	}
	at := func(i, j int) float64 {
		i = clampInt(i, 0, bw-1)
		j = clampInt(j, 0, bh-1)
		return field[j*bw+i]
	}

	// The height field's features span many pixels (more for a bigger cloud), so
	// the per-pixel gradient is tiny; scale the bump by the cloud width to get a
	// resolution-independent surface relief.
	bump := cloudBumpStrength * cd.ch
	// Per-column flat-bottom row (an arced base, not a straight cut).
	lbCol := make([]float64, bw)
	for i := range bw {
		lbCol[i] = cloudLocalBase(cd, float64(x0+i)+0.5)
	}
	vbAt := func(lb float64, y int) float64 {
		return smoothstep(0, cloudBaseTaper, (lb-float64(y))/cd.ch)
	}

	lx, ly, lz := light[0], light[1], light[2]
	for j := range bh {
		y := y0 + j
		for i := range bw {
			lb := lbCol[i]
			// Flat-bottom mask: full above the (arced) base, fading out just below it.
			bottom := smoothstep(lb, lb-cloudBottomFeather, float64(y))
			if bottom <= 0 {
				continue
			}
			d := field[j*bw+i]
			cov := smoothstep(0, cloudEdgeSoft, d) * bottom
			if cov <= 0 {
				continue
			}

			// Relief ramps in from the flat base so the base is a thin,
			// downward-facing (shadowed) lip while the billows above carry the volume.
			vbase, vbU, vbD := vbAt(lb, y), vbAt(lb, y-2), vbAt(lb, y+2)

			// Surface normal from the (base-faded) height field. The gradient is
			// taken over a 2px step, which low-passes the field so residual Worley
			// creases don't show as facets. z points toward the viewer.
			zl := math.Max(at(i-2, j), 0) * vbase
			zr := math.Max(at(i+2, j), 0) * vbase
			zu := math.Max(at(i, j-2), 0) * vbU
			zd := math.Max(at(i, j+2), 0) * vbD
			nx := -(zr - zl) * 0.25 * bump
			ny := -(zd - zu) * 0.25 * bump
			nz := 1.0
			ni := 1 / math.Sqrt(nx*nx+ny*ny+nz*nz)

			// Dominant-sun diffuse, planet-style: n·L through a soft terminator plus
			// a gentle spherical falloff, filled by ambient on the shadow side.
			illum := (nx*lx + ny*ly + nz*lz) * ni
			lit := smoothstep(-cloudTermW, cloudTermW, illum) * math.Min(0.6+0.4*math.Max(illum, 0), 1)
			bright := ambient + (1-ambient)*lit

			col := lerpHSV(cd.shadow, cd.lit, bright).RGB()
			// Tint the lit contribution by the star color (the shadow/ambient part
			// stays neutral), matching how planets pick up the dominant star's hue.
			// Softened so a strongly-colored sun only gently colors the clouds.
			t := lit * cloudTintAmt
			col.R *= 1 - t*(1-tint.R)
			col.G *= 1 - t*(1-tint.G)
			col.B *= 1 - t*(1-tint.B)
			blendPixel(img, w, h, x0+i, y, col, cov*cloudLowAlpha)
		}
	}
}

// cloudColorHigh picks the high gauzy layer's color for the time of day: pale and
// near-white by day, bright and warm at dusk, dim and cool at night.
func cloudColorHigh(rng *rand.Rand, t TimeOfDay) gfx.HSV {
	switch t {
	case Dusk:
		return gfx.HSV{H: rnd(rng, 12, 50), S: rnd(rng, 0.40, 0.70), V: rnd(rng, 0.85, 1.0)}
	case Twilight:
		return gfx.HSV{H: rnd(rng, 200, 255), S: rnd(rng, 0.12, 0.32), V: rnd(rng, 0.10, 0.22)}
	default:
		return gfx.HSV{H: rnd(rng, 195, 225), S: rnd(rng, 0.04, 0.12), V: rnd(rng, 0.92, 1.0)}
	}
}

// cloudColorsLow picks a nimbus cloud's lit-top and shadowed-underside colors for
// the time of day. By day the puffs are white with grey bellies; at dusk warm,
// bright tops drop to cool, dim undersides; at night they are dark shadows.
func cloudColorsLow(rng *rand.Rand, t TimeOfDay) (lit, shadow gfx.HSV) {
	switch t {
	case Dusk:
		hue := rnd(rng, 15, 45)
		lit = gfx.HSV{H: hue, S: rnd(rng, 0.45, 0.75), V: rnd(rng, 0.92, 1.0)}
		shadow = gfx.HSV{H: math.Mod(hue+rnd(rng, 235, 300), 360), S: rnd(rng, 0.35, 0.60), V: rnd(rng, 0.28, 0.46)}
	case Twilight:
		hue := rnd(rng, 205, 255)
		lit = gfx.HSV{H: hue, S: rnd(rng, 0.15, 0.35), V: rnd(rng, 0.16, 0.28)}
		shadow = gfx.HSV{H: hue, S: rnd(rng, 0.20, 0.40), V: rnd(rng, 0.04, 0.10)}
	default:
		hue := rnd(rng, 200, 225)
		lit = gfx.HSV{H: hue, S: rnd(rng, 0.0, 0.05), V: rnd(rng, 0.96, 1.0)}
		shadow = gfx.HSV{H: hue, S: rnd(rng, 0.04, 0.11), V: rnd(rng, 0.62, 0.80)}
	}
	return lit, shadow
}

// cloudAmbient is the shadow-side fill light for nimbus clouds: brightest by day
// so even shaded puffs stay light, lowest at night so clouds read as silhouettes.
func cloudAmbient(t TimeOfDay) float64 {
	switch t {
	case Dusk:
		return 0.40
	case Twilight:
		return 0.28
	default:
		return 0.45
	}
}
