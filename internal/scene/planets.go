package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Planets draws planets in the sky. They sit in front of the stars and suns but
// behind the ground (so a planet near the horizon is occluded by the terrain).
// A scene has a ~50% chance of any planets; when present the count is usually
// one to three but can run up to many. Each planet has a type; the first is the
// gas giant, rendered as turbulent latitudinal color bands on a shaded sphere.
type Planets struct{}

func (p *Planets) Name() string { return "planets" }

const (
	planetsAnimDuration = 900 * time.Millisecond

	planetChance    = 0.75 // chance a scene has any planets at all (zero halved)
	planetCountMean = 3.0  // exponential mean of the count when present
	planetMax       = 20

	// Per-planet band rotation, added to the global star angle. Biased toward
	// little rotation (planets stay fairly aligned) but still allowed to vary,
	// up to 90 degrees more.
	planetRotStd = 30.0

	// Atmospheric haze: planets blend toward the sky color toward the horizon.
	// Values >1 (clamped) make planets fade out fully before reaching the
	// horizon: in daylight to basically nothing, strongly at dusk, none at
	// twilight (clear dark sky).
	planetHazePow      = 2.0
	planetHazeMidday   = 1.7
	planetHazeDusk     = 1.2
	planetHazeTwilight = 0.0

	// Disc size as a fraction of scene width, biased small (squared) with a
	// rare tail up to half the scene width.
	planetMinFrac = 0.004
	planetMaxFrac = 0.50

	// When a scene has planets, the first one is often a dominant world filling
	// 20-50% of the sky width.
	planetBigFirstChance = 0.65
	planetBigMinFrac     = 0.20
	planetBigMaxFrac     = 0.50

	// Gas-giant bands.
	planetBandsMin     = 6
	planetBandsMax     = 16
	planetBandContrast = 0.12 // light/dark alternation between adjacent bands
	planetTurbScaleX   = 5.0  // turbulence cells across the disc
	planetTurbScaleLat = 8.0  // turbulence cells top-to-bottom
	planetTurbMin      = 0.04 // band waviness (fraction of latitude)
	planetTurbMax      = 0.14

	// Moons (airless, rocky). A mottled base, washed-out/gray-leaning colors,
	// optional lighter poles, and elliptical impact craters if large enough.
	moonChance      = 0.50 // share of planets that are moons rather than gas giants
	moonMottleScale = 3.5  // surface mottle cells across the disc
	moonMottleMin   = 0.25 // value mottle amplitude
	moonMottleMax   = 0.60
	moonPoleChance  = 0.60 // chance a moon has lighter poles
	moonPoleStart   = 0.60 // latitude (|.|) where polar lightening begins
	moonPoleMin     = 0.15
	moonPoleMax     = 0.45

	// Surface variation layers (beyond the fine mottle): big dark "maria" lava
	// patches (the man-in-the-moon look), and recolored patches (ice fields or
	// dusty regions of a different hue).
	moonMariaChance  = 0.65 // chance a moon has dark maria patches
	moonMariaScale   = 1.6  // big, low-frequency patches
	moonMariaDarkLo  = 0.25
	moonMariaDarkHi  = 0.55
	moonPatchChance  = 0.60 // chance a moon has recolored (ice/dusty) patches
	moonPatchScale   = 2.6  // medium patches
	moonPatchBlendLo = 0.40
	moonPatchBlendHi = 0.80

	moonCraterMinR   = 16   // smallest radius (px) that gets craters
	moonCraterMax    = 40   // cap on crater count
	moonCraterMinSz  = 0.05 // on-sphere crater radius as a fraction of the disc radius
	moonCraterMaxSz  = 0.34
	moonCraterMinFor = 0.20 // floor on the foreshortening so limb craters aren't slivers

	moonCraterFloor  = 0.28 // how much the flat interior floor darkens (shallow)
	moonCraterRelief = 0.55 // wall directional shading (gentle: shallow crater)
	moonCraterLip    = 0.50 // thin-rim highlight/shadow strength
	moonCraterJagAmp = 0.18 // roughening of the rim (fraction of crater radius)
	moonCraterJagScl = 7.0  // jagged-edge noise frequency across the disc

	// The rim lip and the inner highlight/shadow wall are both measured in
	// pixels (not crater radii), so they stay microscopic rings even on a big,
	// close planet — barely growing with the radius. The flat interior between
	// them makes craters read as vast and shallow, never deep.
	moonLipBasePx  = 0.8
	moonLipGrowPx  = 0.0015
	moonWallBasePx = 1.0
	moonWallGrowPx = 0.0015

	// Large, near moons switch to a procedural HEIGHT FIELD rendered with
	// bump-mapped lighting: ridged terrain, fine grain, and craters with central
	// peaks and smooth floors. Detail ramps in between these radii (px) so it
	// doesn't pop; below moonDetailMinR a moon keeps the cheaper flat-crater
	// shading and looks exactly as before.
	moonDetailMinR  = 55.0
	moonDetailFullR = 110.0

	// Surface terrain (outside craters): low-frequency ridged "mountains" plus a
	// high-frequency grain. The ridge frequency is a fixed cell count across the
	// disc (mountains scale with the planet); the grain frequency tracks the
	// pixel size so the grain stays fine at any on-screen size.
	moonRidgeCells = 7.0  // ridged-mountain cells across the disc
	moonRidgeAmp   = 0.10 // mountain relief (height units)
	moonRidgeOct   = 5
	moonGrainPx    = 5.0   // target grain wavelength in pixels
	moonGrainAmp   = 0.018 // grain relief (height units)

	// Bump mapping: the surface normal is tilted by the height-field gradient
	// (sampled by finite difference over moonBumpEps pixels) before lighting, so
	// terrain and craters cast self-shadows. Strength ramps with the detail level.
	moonBumpEps      = 1.3
	moonBumpStrength = 0.45

	// On the height-field path the crater floor is only gently darkened (the
	// shape now comes from the bump-mapped walls/peak, not from painting it dark).
	moonFloorDarkLarge = 0.10

	// Crater height profile (in disc-ellipse units, de=1 is the rim). A flat,
	// smooth floor inside floorEdge, walls sloping up to a raised rim ring, and a
	// central peak for craters at/above the peak size. Heights scale with the
	// crater size so big craters have proportionally more relief.
	moonCraterRimOuter   = 1.28 // footprint cutoff (terrain resumes beyond this)
	moonCraterFloorEdge  = 0.55 // de below which the floor is flat
	moonCraterDepth      = 0.55 // floor depression depth
	moonCraterRimW       = 0.09 // rim-ring width
	moonCraterRimH       = 0.30 // rim-ring height
	moonCraterPeakMinSz  = 0.16 // smallest crater size that grows a central peak
	moonCraterPeakFullSz = 0.30 // size at which the central peak is full height
	moonCraterPeakW      = 0.22 // central-peak width (de units)
	moonCraterPeakH      = 0.45 // central-peak height
	moonCraterRoughJag   = 0.28 // rim roughening on the height path (rougher edges)
)

// PlanetType selects how a planet is rendered.
type PlanetType int

const (
	GasGiant PlanetType = iota
	Moon
)

// crater is one impact crater. It is a circle on the sphere; projected to the
// disc (coords span [-1,1]) it becomes an ellipse foreshortened along the radial
// direction (rx,ry) — circular at the disc center, increasingly elongated along
// the tangent toward the limb.
type crater struct {
	cx, cy float64 // center, disc-normalized
	size   float64 // on-sphere (tangential) radius
	radial float64 // foreshortened radial semi-axis (size * foreshortening)
	rx, ry float64 // unit radial direction (disc center -> crater center)
	seed   int     // jagged-edge noise seed
}

// planet is one resolved planet. Some fields are type-specific: bands is for
// gas giants; base, poleLight, and craters are for moons.
type planet struct {
	cx, cy   int
	r        int
	typ      PlanetType
	rotation float64 // surface tilt in radians (bands / pole axis)
	turbSeed int     // surface noise seed
	turbAmp  float64 // surface noise amplitude (band warp / moon mottle)

	bands gfx.Gradient // gas giant: latitude (0=top, 1=bottom) -> color

	base      gfx.HSV  // moon: base surface color
	poleLight float64  // moon: polar lightening (0 = none)
	craters   []crater // moon: impact craters

	mariaThresh float64 // moon: dark-patch noise threshold
	mariaDark   float64 // moon: dark-patch darkening (0 = no maria)
	patchThresh float64 // moon: recolor-patch noise threshold
	patchBlend  float64 // moon: recolor-patch blend amount (0 = none)
	patchColor  gfx.HSV // moon: recolor-patch color (ice / dusty hue)
}

func (p *Planets) Render(c *Context) error {
	n := planetCount(c.Rng)
	if n == 0 {
		return nil
	}

	w, h := c.W, c.H
	planets := make([]planet, n)
	for i := range planets {
		planets[i] = makePlanet(c.Rng, w, c.Settings, i == 0)
	}

	// Precompute, per row, the sky color planets blend toward and how much
	// (haze): planets fade into the sky near the horizon in daylight/dusk.
	horizon := c.Settings.HorizonY
	th := planetTimeHaze(c.Settings.Time)
	skyRow := make([]gfx.RGB, h)
	hazeRow := make([]float64, h)
	for y := range h {
		skyRow[y] = skyColorAt(c.SkyGradient, y, horizon, h).RGB()
		// >1 values (from a strong time-of-day haze) clamp to full fade, so
		// planets vanish into the sky before reaching the horizon.
		hazeRow[y] = math.Min(th*math.Pow(math.Min(float64(y)/float64(horizon), 1), planetHazePow), 1)
	}

	lm := newLightModel(c.Settings)

	per := planetsAnimDuration / time.Duration(n)
	for i := range planets {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		pl := planets[i]
		c.Canvas.Draw(func(img *image.RGBA) {
			drawPlanet(img, w, h, pl, skyRow, hazeRow, lm)
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// planetTimeHaze is the maximum atmospheric haze for the time of day: strong in
// daylight, less at dusk, none at twilight (clear dark sky).
func planetTimeHaze(t TimeOfDay) float64 {
	switch t {
	case Dusk:
		return planetHazeDusk
	case Twilight:
		return planetHazeTwilight
	default:
		return planetHazeMidday
	}
}

// planetCount returns 0 with probability 1-planetChance; otherwise an
// exponentially distributed count (mean planetCountMean) shifted to start at 1
// and clamped to planetMax, so one-to-three is typical with a tail up to many.
func planetCount(rng *rand.Rand) int {
	if rng.Float64() >= planetChance {
		return 0
	}
	c := -planetCountMean * math.Log(1-rng.Float64())
	return min(max(1+int(c), 1), planetMax)
}

// makePlanet resolves a planet's type, size, position, and look. Planets sit in
// the sky (between the top and the horizon); large ones may dip toward the
// horizon and be partly occluded by the later ground layer. When isFirst is set
// the planet is often a dominant world filling 20-50% of the sky width.
func makePlanet(rng *rand.Rand, w int, set Settings, isFirst bool) planet {
	var frac float64
	if isFirst && rng.Float64() < planetBigFirstChance {
		frac = rnd(rng, planetBigMinFrac, planetBigMaxFrac)
	} else {
		t := math.Min(math.Abs(rng.NormFloat64())/3, 1)
		frac = planetMinFrac + (planetMaxFrac-planetMinFrac)*t*t
	}
	r := max(int(frac*float64(w)/2), 2)

	typ := GasGiant
	if rng.Float64() < moonChance {
		typ = Moon
	}

	// Surface tilt: the global star angle plus a per-planet offset of up to 90
	// degrees, biased low so planets stay fairly aligned.
	delta := math.Min(math.Abs(rng.NormFloat64())*planetRotStd, 90)
	rotation := (set.TwinkleAngle + delta) * math.Pi / 180

	p := planet{
		cx:       rng.Intn(w),
		cy:       rng.Intn(set.HorizonY + 1),
		r:        r,
		typ:      typ,
		rotation: rotation,
		turbSeed: rng.Int(),
	}
	switch typ {
	case GasGiant:
		p.bands = buildGasGiantBands(rng)
		p.turbAmp = rnd(rng, planetTurbMin, planetTurbMax)
	case Moon:
		p.base = moonBaseColor(rng)
		p.turbAmp = rnd(rng, moonMottleMin, moonMottleMax)
		if rng.Float64() < moonPoleChance {
			p.poleLight = rnd(rng, moonPoleMin, moonPoleMax)
		}
		if rng.Float64() < moonMariaChance {
			p.mariaThresh = rnd(rng, 0.45, 0.62)
			p.mariaDark = rnd(rng, moonMariaDarkLo, moonMariaDarkHi)
		}
		if rng.Float64() < moonPatchChance {
			p.patchThresh = rnd(rng, 0.50, 0.66)
			p.patchBlend = rnd(rng, moonPatchBlendLo, moonPatchBlendHi)
			p.patchColor = moonPatchColor(rng, p.base)
		}
		p.craters = makeCraters(rng, r)
	}
	return p
}

// moonBaseColor picks a moon's base surface color: any hue, but biased toward
// gray and washed-out dusty tones (low saturation).
func moonBaseColor(rng *rand.Rand) gfx.HSV {
	s := rng.Float64()
	return gfx.HSV{
		H: rng.Float64() * 360,
		S: s * s * 0.35, // squared -> mostly near-gray, occasionally dusty
		V: rnd(rng, 0.30, 0.65),
	}
}

// moonPatchColor picks the color of a moon's recolored regions: either icy
// (pale, cool, bright — ice fields) or a contrasting dusty hue (mineral spots).
func moonPatchColor(rng *rand.Rand, base gfx.HSV) gfx.HSV {
	if rng.Float64() < 0.5 {
		return gfx.HSV{H: rnd(rng, 180, 235), S: rnd(rng, 0.05, 0.25), V: rnd(rng, 0.70, 0.95)}
	}
	return gfx.HSV{H: math.Mod(base.H+rnd(rng, 60, 300), 360), S: rnd(rng, 0.25, 0.60), V: rnd(rng, 0.30, 0.60)}
}

// makeCraters scatters elliptical craters across a moon, but only if it is big
// enough to show them. Count scales with size; craters may overlap and run off
// the limb (where they read as foreshortened ellipses).
func makeCraters(rng *rand.Rand, r int) []crater {
	if r < moonCraterMinR {
		return nil
	}
	n := min(5+rng.Intn(3+r/10), moonCraterMax)
	craters := make([]crater, 0, n)
	for range n {
		// Best-candidate sampling: try a few spots and keep the one farthest
		// from existing craters, so they spread out rather than clumping into a
		// mess. Overlaps still happen occasionally (small clusters of 2-3).
		var bx, by, bdc float64
		best := -1.0
		for range 8 {
			dc := 0.93 * math.Sqrt(rng.Float64())
			ang := rng.Float64() * 2 * math.Pi
			x, y := dc*math.Cos(ang), dc*math.Sin(ang)
			nearest := math.MaxFloat64
			for _, c := range craters {
				if d := math.Hypot(x-c.cx, y-c.cy); d < nearest {
					nearest = d
				}
			}
			if nearest > best {
				best, bx, by, bdc = nearest, x, y, dc
			}
		}

		rx, ry := 1.0, 0.0
		if bdc > 1e-6 {
			rx, ry = bx/bdc, by/bdc
		}
		// Strongly bias toward small craters, more so on bigger planets (where
		// we can resolve many tiny ones), leaving medium/large as rare outliers.
		p := math.Min(3+float64(r)*0.012, 8)
		size := moonCraterMinSz + (moonCraterMaxSz-moonCraterMinSz)*math.Pow(rng.Float64(), p)
		foreshorten := math.Max(math.Sqrt(math.Max(1-bdc*bdc, 0)), moonCraterMinFor)
		craters = append(craters, crater{
			cx:     bx,
			cy:     by,
			size:   size,
			radial: size * foreshorten,
			rx:     rx,
			ry:     ry,
			seed:   rng.Int(),
		})
	}
	return craters
}

// buildGasGiantBands builds the latitude->color gradient for a gas giant. A
// scheme is chosen for the palette: similar hues (Neptune-like), moderately
// variable (Jupiter-like), or fantastic. Adjacent bands alternate lighter and
// darker so the banding reads even when hues are close.
func buildGasGiantBands(rng *rand.Rand) gfx.Gradient {
	baseHue := rng.Float64() * 360

	var hueSpread, baseSat, baseVal float64
	switch r := rng.Float64(); {
	case r < 0.45: // similar — bands of one color (e.g. Neptune's blues)
		hueSpread, baseSat, baseVal = rnd(rng, 3, 12), rnd(rng, 0.35, 0.70), rnd(rng, 0.50, 0.80)
	case r < 0.80: // variable — Jupiter-like spread
		hueSpread, baseSat, baseVal = rnd(rng, 15, 40), rnd(rng, 0.40, 0.75), rnd(rng, 0.45, 0.78)
	default: // fantastic — anything goes
		hueSpread, baseSat, baseVal = rnd(rng, 60, 170), rnd(rng, 0.50, 0.95), rnd(rng, 0.40, 0.85)
	}

	nb := planetBandsMin + rng.Intn(planetBandsMax-planetBandsMin+1)
	spacing := 1.0 / float64(nb)
	grad := make(gfx.Gradient, nb+1)
	for i := range grad {
		pos := float64(i) * spacing
		if i > 0 && i < nb {
			pos += rnd(rng, -0.4, 0.4) * spacing // uneven band widths, still ordered
		}
		alt := planetBandContrast
		if i%2 == 1 {
			alt = -planetBandContrast
		}
		grad[i] = gfx.Stop{Pos: pos, Col: gfx.HSV{
			H: baseHue + rnd(rng, -hueSpread, hueSpread),
			S: clamp01(baseSat + rnd(rng, -0.15, 0.15)),
			V: clamp01(baseVal + alt + rnd(rng, -0.07, 0.07)),
		}}
	}
	return grad
}

// lightModel is the resolved dominant-star lighting for a scene. lx,ly,lz is
// the 3D light direction (z toward the viewer); col tints the lit side; termW
// is the terminator half-width (small = sharp shadow); ambient fills the dark
// side. The in-plane part (lx,ly) doubles as the crater-wall light direction,
// already scaled by the phase so crater shadows vanish at full phase.
type lightModel struct {
	lx, ly, lz float64
	col        gfx.RGB
	termW      float64
	ambient    float64
}

func newLightModel(s Settings) lightModel {
	a := s.TwinkleAngle * math.Pi / 180
	// In-plane light direction along the equator/band axis — the same axis the
	// gas-giant bands run along — pointing up-and-to-the-left, so planets are
	// lit from the left with shadows to the right. A planet whose rotation
	// matches the star angle faces the star equator-on, so the terminator
	// crosses its bands and they're all visible; planets are lit differently
	// only as their rotation diverges from the star angle.
	dx, dy := -math.Cos(a), -math.Sin(a)
	phaseAngle := (1 - s.LightPhase) * math.Pi
	sinP, cosP := math.Sin(phaseAngle), math.Cos(phaseAngle)
	return lightModel{
		lx: dx * sinP, ly: dy * sinP, lz: cosP,
		col:     s.LightColor,
		termW:   0.45*(1-s.LightBrightness) + 0.03,
		ambient: s.LightAmbient,
	}
}

func drawPlanet(img *image.RGBA, w, h int, p planet, skyRow []gfx.RGB, hazeRow []float64, lm lightModel) {
	switch p.typ {
	case GasGiant:
		drawGasGiant(img, w, h, p, skyRow, hazeRow, lm)
	case Moon:
		drawMoon(img, w, h, p, skyRow, hazeRow, lm)
	}
}

// blendPlanetPixel finishes a planet pixel. Planets emit no light: they only
// reflect the dominant star, so they brighten the sky rather than darkening it.
// The reflected light is screened over the sky color, which means the shadowed
// side (and anything faded by atmospheric haze) simply disappears into the sky.
func blendPlanetPixel(img *image.RGBA, w, h, xx, yy int, surf gfx.HSV, nx, ny, nz, a float64, skyRow []gfx.RGB, hazeRow []float64, lm lightModel, seed int) {
	// Diffuse lighting from the dominant star. (nx,ny,nz) is the surface normal —
	// the sphere normal for gas giants and small moons, or a bump-perturbed normal
	// for large moons. illum is n·L. The terminator (illum≈0) is sharpened by termW
	// and jittered slightly by surface texture; the lit side keeps a spherical
	// (center-bright) falloff.
	illum := nx*lm.lx + ny*lm.ly + nz*lm.lz
	illum += (gfx.FBM((nx+1)*6, (ny+1)*6, seed+7, 2) - 0.5) * 0.06
	lit := smoothstep(-lm.termW, lm.termW, illum) * math.Min(0.55+0.45*math.Max(illum, 0), 1)

	// Reflected light = albedo × (ambient + direct), tinted by the star color,
	// then faded out by atmospheric haze near the horizon. In shadow with low
	// ambient this goes to ~0, so the planet vanishes into the sky there.
	base := surf.RGB()
	refl := (lm.ambient + (1-lm.ambient)*lit) * (1 - hazeRow[yy])
	rr := base.R * lm.col.R * refl
	rg := base.G * lm.col.G * refl
	rb := base.B * lm.col.B * refl

	// Screen the reflected light over the sky: the planet can only brighten it.
	sky := skyRow[yy]
	final := gfx.RGB{
		R: sky.R + rr - sky.R*rr,
		G: sky.G + rg - sky.G*rg,
		B: sky.B + rb - sky.B*rb,
	}
	blendPixel(img, w, h, xx, yy, final, a)
}

// drawGasGiant renders the banded sphere. For each disc pixel it rotates into
// band space, maps the position to a latitude (compressed toward the poles like
// a sphere), perturbs it with turbulence so the bands waver, samples the band
// gradient, and applies limb darkening so the disc reads as round. The color is
// then blended toward the sky color by the per-row haze (so low planets fade
// into the sky), and the rim is feathered.
func drawGasGiant(img *image.RGBA, w, h int, p planet, skyRow []gfx.RGB, hazeRow []float64, lm lightModel) {
	rf := float64(p.r)
	cs, sn := math.Cos(p.rotation), math.Sin(p.rotation)
	for oy := -p.r; oy <= p.r; oy++ {
		yy := p.cy + oy
		if yy < 0 || yy >= h {
			continue
		}
		ny := float64(oy) / rf
		for ox := -p.r; ox <= p.r; ox++ {
			xx := p.cx + ox
			if xx < 0 || xx >= w {
				continue
			}
			nx := float64(ox) / rf
			rr := nx*nx + ny*ny
			if rr > 1 {
				continue
			}

			// Feather the last pixel of the rim for a clean edge.
			a := 1.0
			if dist := math.Sqrt(rr) * rf; dist > rf-1 {
				a = rf - dist
			}
			if a <= 0 {
				continue
			}

			// Rotate into band space so the bands tilt with the planet.
			by := -nx*sn + ny*cs
			bx := nx*cs + ny*sn
			lat := (math.Asin(clamp(by, -1, 1)) + math.Pi/2) / math.Pi
			tb := gfx.FBM((bx+1)*planetTurbScaleX, (by+1)*planetTurbScaleLat, p.turbSeed, 3)
			latp := clamp(lat+(tb-0.5)*p.turbAmp, 0, 1)

			col := p.bands.At(latp)
			nz := math.Sqrt(math.Max(1-nx*nx-ny*ny, 0))
			blendPlanetPixel(img, w, h, xx, yy, col, nx, ny, nz, a, skyRow, hazeRow, lm, p.turbSeed)
		}
	}
}

// drawMoon renders an airless, rocky body: a mottled base color (washed-out,
// gray-leaning), optional lighter poles, and elliptical impact craters, all on
// a limb-shaded sphere. There is no atmospheric band structure.
func drawMoon(img *image.RGBA, w, h int, p planet, skyRow []gfx.RGB, hazeRow []float64, lm lightModel) {
	rf := float64(p.r)
	cs, sn := math.Cos(p.rotation), math.Sin(p.rotation)
	// Rim lip and inner wall thicknesses in disc-radius units: a thin pixel
	// count regardless of how big the planet is on screen.
	lipW := (moonLipBasePx + moonLipGrowPx*rf) / rf
	wallW := (moonWallBasePx + moonWallGrowPx*rf) / rf
	// Crater walls are lit by the dominant star's in-plane direction (already
	// scaled by phase, so shadows fade out as the planet approaches full).
	lightX, lightY := lm.lx, lm.ly

	// Large, near moons get a bump-mapped procedural height field; detail ramps
	// in with the on-screen radius. Small moons (detail==0) keep the cheaper
	// flat-crater shading and render exactly as before.
	detail := clamp((rf-moonDetailMinR)/(moonDetailFullR-moonDetailMinR), 0, 1)
	var field moonField
	var eps float64
	if detail > 0 {
		field = moonField{p: &p, cs: cs, sn: sn, ridgeScale: moonRidgeCells, grainScale: rf / moonGrainPx}
		eps = moonBumpEps / rf
	}

	for oy := -p.r; oy <= p.r; oy++ {
		yy := p.cy + oy
		if yy < 0 || yy >= h {
			continue
		}
		ny := float64(oy) / rf
		for ox := -p.r; ox <= p.r; ox++ {
			xx := p.cx + ox
			if xx < 0 || xx >= w {
				continue
			}
			nx := float64(ox) / rf
			rr := nx*nx + ny*ny
			if rr > 1 {
				continue
			}

			a := 1.0
			if dist := math.Sqrt(rr) * rf; dist > rf-1 {
				a = rf - dist
			}
			if a <= 0 {
				continue
			}

			// Rotate into surface space so poles/mottle tilt with the planet.
			bx := nx*cs + ny*sn
			by := -nx*sn + ny*cs

			surf := p.base

			// Dark maria (lava-plain) patches — the man-in-the-moon look.
			if p.mariaDark > 0 {
				mv := gfx.FBM((bx+1)*moonMariaScale, (by+1)*moonMariaScale, p.turbSeed+101, 4)
				mm := smoothstep(p.mariaThresh-0.08, p.mariaThresh+0.08, mv)
				surf.V *= 1 - p.mariaDark*mm
				surf.S *= 1 - 0.3*mm // maria read a touch greyer
			}

			// Recolored patches — ice fields or dusty regions of another hue.
			if p.patchBlend > 0 {
				pv := gfx.FBM((bx+1)*moonPatchScale+5, (by+1)*moonPatchScale, p.turbSeed+211, 3)
				pm := smoothstep(p.patchThresh-0.08, p.patchThresh+0.08, pv) * p.patchBlend
				surf = lerpHSV(surf, p.patchColor, pm)
			}

			// Mottled rock (fine texture over everything).
			m := gfx.FBM((bx+1)*moonMottleScale, (by+1)*moonMottleScale, p.turbSeed, 4)
			surf.V *= 1 + (m-0.5)*p.turbAmp
			surf.S *= 1 + (m-0.5)*0.2

			// Lighter, desaturated poles.
			if p.poleLight > 0 {
				if pole := math.Abs(by); pole > moonPoleStart {
					f := (pole - moonPoleStart) / (1 - moonPoleStart)
					surf.V += p.poleLight * f * f
					surf.S *= 1 - 0.5*f
				}
			}

			nx2, ny2, nz2 := nx, ny, math.Sqrt(math.Max(1-nx*nx-ny*ny, 0))
			if detail > 0 {
				// Bump mapping: tilt the normal by the height-field gradient
				// (finite difference in disc space), scaled down toward the limb
				// (×nz) so grazing-angle slopes don't blow up. The crater shape now
				// comes from the lit relief; the floor only gets a gentle darkening.
				h0, floorSmooth := field.heightAt(nx, ny)
				hx, _ := field.heightAt(nx+eps, ny)
				hy, _ := field.heightAt(nx, ny+eps)
				k := moonBumpStrength * detail * nz2
				px := nx - k*(hx-h0)/eps
				py := ny - k*(hy-h0)/eps
				if inv := 1 / math.Sqrt(px*px+py*py+nz2*nz2); inv > 0 {
					nx2, ny2, nz2 = px*inv, py*inv, nz2*inv
				}
				surf.V *= 1 - moonFloorDarkLarge*floorSmooth
			} else {
				// Craters: the topmost (latest) crater covering this pixel wins, so
				// a newer impact obliterates older ones it overlaps (no ghosting).
				for i := len(p.craters) - 1; i >= 0; i-- {
					if s, owns := craterAt(nx, ny, lightX, lightY, lipW, wallW, p.craters[i]); owns {
						surf.V *= s
						break
					}
				}
			}

			blendPlanetPixel(img, w, h, xx, yy, surf, nx2, ny2, nz2, a, skyRow, hazeRow, lm, p.turbSeed)
		}
	}
}

// moonField evaluates a large moon's procedural surface height at a disc point.
// Terrain (ridged mountains + fine grain) lives in rotated surface space (bx,by)
// so it tracks the planet's tilt; craters live in disc space (nx,ny) like the
// flat-crater model. The height is sampled by finite difference for bump mapping.
type moonField struct {
	p          *planet
	cs, sn     float64
	ridgeScale float64 // ridged-terrain cells across the disc (low frequency)
	grainScale float64 // grain cells across the disc (high frequency, pixel-tracking)
}

// heightAt returns the surface height at disc point (nx,ny) plus a "floor"
// smoothness mask in [0,1] (1 on a flat crater floor) the caller uses to suppress
// terrain grain and to gently darken the floor. Outside every crater the mask is
// 0 and the height is pure terrain.
func (f moonField) heightAt(nx, ny float64) (float64, float64) {
	bx := nx*f.cs + ny*f.sn
	by := -nx*f.sn + ny*f.cs
	ridge := gfx.RidgedFBM(bx*f.ridgeScale+3, by*f.ridgeScale-7, f.p.turbSeed+401, moonRidgeOct)
	grain := gfx.FBM(bx*f.grainScale, by*f.grainScale, f.p.turbSeed+503, 2) - 0.5
	terrain := ridge*moonRidgeAmp + grain*moonGrainAmp

	// Topmost crater covering the point wins, matching the flat-crater model.
	for i := len(f.p.craters) - 1; i >= 0; i-- {
		if dh, smooth, owns := craterHeight(nx, ny, f.p.craters[i]); owns {
			// Suppress terrain roughness on the smooth floor; let it run on the
			// rough rim and walls.
			return terrain*(1-smooth) + dh, smooth
		}
	}
	return terrain, 0
}

// craterHeight returns one crater's contribution to the height field at disc
// point (nx,ny): a flat, smooth floor, walls sloping up to a roughened raised rim
// ring, and a central peak for craters large enough to form one. It also returns
// a floor-smoothness mask (1 on the flat floor) and whether the point is within
// the crater footprint. Heights scale with the crater size.
func craterHeight(nx, ny float64, cr crater) (float64, float64, bool) {
	ddx, ddy := nx-cr.cx, ny-cr.cy
	// Decompose into the crater's radial (foreshortened) and tangential axes.
	dr := ddx*cr.rx + ddy*cr.ry
	dt := -ddx*cr.ry + ddy*cr.rx
	de := math.Hypot(dr/cr.radial, dt/cr.size)
	// Roughen the rim more than the flat-crater model so big craters read as
	// ragged, not clean ellipses.
	de *= 1 + moonCraterRoughJag*(gfx.FBM((nx+1)*moonCraterJagScl, (ny+1)*moonCraterJagScl, cr.seed, 3)-0.5)
	if de > moonCraterRimOuter {
		return 0, 0, false
	}

	floor := smoothstep(1, moonCraterFloorEdge, de) // 1 on the flat floor, 0 at the rim
	bowl := -moonCraterDepth * floor
	rim := moonCraterRimH * math.Exp(-sq((de-1)/moonCraterRimW))
	peak := 0.0
	if cr.size >= moonCraterPeakMinSz {
		pk := smoothstep(moonCraterPeakMinSz, moonCraterPeakFullSz, cr.size)
		peak = moonCraterPeakH * pk * math.Exp(-sq(de/moonCraterPeakW))
	}
	return (bowl + rim + peak) * cr.size, floor, true
}

// craterAt returns the brightness multiplier for one crater at disc point
// (nx,ny) and whether the crater covers (owns) that pixel. The crater is a
// foreshortened ellipse (minor axis radial, major axis tangent). It reads as a
// vast, flat-floored crater: a uniformly darker interior, a *thin* directional
// inner wall ring just inside the rim (lit away from the light, shadowed toward
// it), and a pixel-thin rim lip. Both rings are absolute (pixel) widths, so they
// stay thin even on a huge planet. The rim is roughened so it isn't a clean
// ellipse. owns is false (and the multiplier 1) outside the crater footprint.
func craterAt(nx, ny, lightX, lightY, lipW, wallW float64, cr crater) (float64, bool) {
	ddx, ddy := nx-cr.cx, ny-cr.cy
	// Decompose into the crater's radial (foreshortened) and tangential axes.
	dr := ddx*cr.rx + ddy*cr.ry
	dt := -ddx*cr.ry + ddy*cr.rx
	de := math.Hypot(dr/cr.radial, dt/cr.size)

	// Roughen the rim so the edge is jagged, not a perfect ellipse.
	de *= 1 + moonCraterJagAmp*(gfx.FBM((nx+1)*moonCraterJagScl, (ny+1)*moonCraterJagScl, cr.seed, 2)-0.5)

	dd := (de - 1) * cr.size // distance from the rim, in disc-radius units
	if dd > lipW*1.6 {
		return 1, false
	}

	// Direction outward from the crater center, for the directional relief.
	nrm := math.Hypot(ddx, ddy) + 1e-9
	ld := (ddx/nrm)*lightX + (ddy/nrm)*lightY

	floorMask := smoothstep(0, -wallW*1.5, dd)  // flat dark interior just inside the rim
	wall := math.Exp(-sq((dd + wallW) / wallW)) // thin inner-wall ring
	lip := math.Exp(-sq(dd / lipW))             // pixel-thin rim line

	shade := 1 - moonCraterFloor*floorMask // flat dark floor
	shade -= moonCraterRelief * ld * wall  // thin wall: shadowed toward light, lit away
	shade += moonCraterLip * ld * lip      // thin rim: bright toward light, dark away
	return math.Max(shade, 0.04), true
}

// smoothstep returns a smooth 0->1 ramp for x in [a,b].
func smoothstep(a, b, x float64) float64 {
	t := clamp((x-a)/(b-a), 0, 1)
	return t * t * (3 - 2*t)
}

func sq(x float64) float64 { return x * x }

// lerpHSV blends from a to b by t in RGB space (not by sweeping the hue wheel),
// so patch transitions don't run through the rainbow.
func lerpHSV(a, b gfx.HSV, t float64) gfx.HSV {
	ra, rb := a.RGB(), b.RGB()
	return gfx.RGB{
		R: ra.R + (rb.R-ra.R)*t,
		G: ra.G + (rb.G-ra.G)*t,
		B: ra.B + (rb.B-ra.B)*t,
	}.HSV()
}

func clamp(v, lo, hi float64) float64 { return math.Min(math.Max(v, lo), hi) }
func clamp01(v float64) float64       { return clamp(v, 0, 1) }
