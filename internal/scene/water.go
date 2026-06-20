package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Water turns the foreground below the horizon into an ocean that reflects the
// scene above the horizon — sky, suns, planets, mountains, and the city skyline.
// Not every scene has it. It samples the already-drawn pixels above the horizon,
// mirrors them down with wave-ripple distortion (calm and mirror-like near the
// horizon, choppier and more water-colored toward the viewer), and tints them
// with a water color. The ocean is not solid: where the land elevation (see the
// ocean model, resolved up front in Build) clears sea level the ground shows
// through as an island or coastline, ringed by a beach and surf — and the city,
// drawn before the water, keeps to that land.
type Water struct{}

func (w *Water) Name() string { return "water" }

// Schemas lists the entity schema keys the water element owns.
func (w *Water) Schemas() []string { return []string{SchemaWaterV0} }

const (
	waterChance       = 0.40
	waterAnimDuration = 900 * time.Millisecond

	waterWaveMin  = 1.0  // ripple amplitude (px) at the horizon
	waterWaveFrac = 0.06 // ripple amplitude toward the viewer, as a fraction of foreground height
	waterFreqY    = 0.11 // wave frequency down the image (horizontal crests)
	waterFreqX    = 0.01 // slow variation across the image

	// Fresnel-ish: near the horizon water is mirror-like; toward the viewer it
	// shows more of its own color and darkens.
	waterTintHorizon    = 0.15
	waterTintForeground = 0.62
	waterDarkForeground = 0.35

	// Islands: some of the ocean can be land. A noise field gives a land elevation;
	// where it clears the (per-scene) sea level the ground shows through as an
	// island, ringed by a beach and surf. A coastal bias raises the elevation
	// toward the horizon, so distant land/coastline (at the feet of the mountains)
	// is common while the near water stays open with only scattered islands.
	islandFreqX      = 0.004 // island cells across the width (broad land masses)
	islandFreqY      = 2.6   // island cells over the foreground depth
	islandOctaves    = 4     // fractal octaves for island/coast shape
	islandSeaLevelLo = 0.54
	islandSeaLevelHi = 0.66
	islandCoastMax   = 0.38  // strongest horizon-ward land bias (0 = open ocean)
	islandBeachBand  = 0.035 // elevation above sea level painted as beach
	islandBeachAmt   = 0.45  // how strongly the beach tints the shore ground
	islandFoamBand   = 0.03  // elevation below sea level painted as surf foam
	islandFoamAmt    = 0.40  // how strongly the surf lightens the water
	islandFoamLift   = 0.60  // how far the foam color is lifted toward white
)

// Generate resolves the scene's ocean into a single entity. The ocean/land model
// is a shared global decided up front in Scene.Build (Context.Ocean, via
// buildOcean on the "water" stream) so Cities — drawn before Water — can keep to
// land while Water reflects the city skyline. Generate therefore draws NO
// randomness of its own: it simply captures that resolved global into the entity.
// A scene with no ocean (Context.Ocean nil or not present) yields an empty list.
func (wt *Water) Generate(c *Context) (SceneList, error) {
	oc := c.Ocean
	if oc == nil || !oc.present {
		return nil, nil
	}
	return SceneList{oceanToEntity(oc)}, nil
}

// RenderList draws the water entity onto the canvas: it mirrors the already-drawn
// pixels above the horizon down into a rippled, island-dotted sea. It is the only
// step that touches the image and it consumes no randomness, so the same scene
// list always draws the same pixels. An entity that is not water is an error.
func (wt *Water) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	oc, err := entityToOcean(list[0])
	if err != nil {
		return err
	}
	// The v0 water uses a linear depth ramp (depthPow 1) and the plain wave
	// amplitude (waveScale 1). water.v1 reuses renderOcean with a sub-linear depth
	// warp and a larger wave scale for its low (ground-level) mode.
	return renderOcean(c, oc, 1.0, 1.0)
}

// renderOcean mirrors the scene above the horizon down into a rippled, island-
// dotted sea. depthPow warps the foreground depth used for the wave/tint/darken
// ramps: 1.0 is the linear v0 ramp; a value below 1 spends more of the visible
// range near the horizon (a wider, calmer mirror band easing to choppier
// foreground), so the water reads as stretching further toward the viewer. waveScale
// multiplies the ripple amplitude. It is the shared drawing core behind both the v0
// water (depthPow 1, waveScale 1) and water.v1's low mode, and consumes no
// randomness.
func renderOcean(c *Context, oc *ocean, depthPow, waveScale float64) error {
	w, h := c.W, c.H
	horizon, groundH := oc.horizon, oc.groundH
	wcol := oc.color
	seed := oc.waveSeed
	waveMax := math.Max(waterWaveFrac*float64(groundH), 2)
	// Surf foam: the water color lifted toward white.
	foam := gfx.RGB{R: wcol.R + (1-wcol.R)*islandFoamLift, G: wcol.G + (1-wcol.G)*islandFoamLift, B: wcol.B + (1-wcol.B)*islandFoamLift}

	bandH := max(groundH/80, 1)
	per := waterAnimDuration / time.Duration((groundH+bandH-1)/bandH)

	for y0 := horizon + 1; y0 < h; y0 += bandH {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y1 := min(y0+bandH, h)
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				d := float64(y-horizon) / float64(groundH) // 0 at the horizon, 1 at the bottom
				dw := d
				if depthPow != 1 {
					dw = math.Pow(d, depthPow)
				}
				amp := (waterWaveMin + (waveMax-waterWaveMin)*dw) * waveScale
				tint := waterTintHorizon + (waterTintForeground-waterTintHorizon)*dw
				dark := 1 - waterDarkForeground*dw
				for x := range w {
					e := oc.elev(x, y)
					if e > oc.seaLevel {
						// Land (island or coast): leave the ground showing, but tint a
						// beach at the shoreline.
						if beach := smoothstep(oc.seaLevel+islandBeachBand, oc.seaLevel, e); beach > 0 {
							blendPixel(img, w, h, x, y, oc.sand, beach*islandBeachAmt)
						}
						continue
					}

					// Ripple displacement (mostly per-row, for horizontal crests).
					dx := (gfx.FBM(float64(y)*waterFreqY, float64(x)*waterFreqX, seed, 3) - 0.5) * 2 * amp
					dy := (gfx.FBM(float64(y)*waterFreqY*0.7+10, float64(x)*waterFreqX, seed+5, 2) - 0.5) * amp
					sx := clampInt(x+int(dx), 0, w-1)
					sy := clampInt(2*horizon-y+int(dy), 0, horizon-1) // mirror across the horizon

					off := img.PixOffset(sx, sy)
					rr := float64(img.Pix[off]) / 255
					gg := float64(img.Pix[off+1]) / 255
					bb := float64(img.Pix[off+2]) / 255

					out := gfx.RGB{
						R: (rr + (wcol.R-rr)*tint) * dark,
						G: (gg + (wcol.G-gg)*tint) * dark,
						B: (bb + (wcol.B-bb)*tint) * dark,
					}
					// Surf foam where the water laps just below the shoreline.
					if surf := smoothstep(oc.seaLevel-islandFoamBand, oc.seaLevel, e); surf > 0 {
						f := surf * islandFoamAmt
						out = gfx.RGB{R: out.R + (foam.R-out.R)*f, G: out.G + (foam.G-out.G)*f, B: out.B + (foam.B-out.B)*f}
					}
					r8, g8, b8, _ := out.RGBA8()
					o := img.PixOffset(x, y)
					img.Pix[o] = r8
					img.Pix[o+1] = g8
					img.Pix[o+2] = b8
					img.Pix[o+3] = 255
				}
			}
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// ocean is a scene's resolved ocean/land model. When present, the below-horizon
// foreground is water except where the land elevation clears sea level — islands,
// and (biased toward the horizon) a coastline at the feet of the mountains.
type ocean struct {
	present  bool
	horizon  int
	groundH  int
	color    gfx.RGB
	sand     gfx.RGB
	waveSeed int
	landSeed int
	seaLevel float64
	coast    float64
}

// buildOcean decides whether a scene has an ocean and, if so, its color, waves,
// and island/coast shape. Drawing order is fixed so a seed reproduces the same
// ocean.
func buildOcean(rng *rand.Rand, s Settings, h int) *ocean {
	horizon := s.HorizonY
	o := &ocean{horizon: horizon, groundH: h - horizon}
	if rng.Float64() >= waterChance || horizon >= h-2 || horizon < 1 {
		return o // no ocean: the whole foreground stays land
	}
	o.present = true

	// Water color: usually blue/teal, occasionally something alien.
	hue := rnd(rng, 180, 245)
	if rng.Float64() < 0.2 {
		hue = rng.Float64() * 360
	}
	o.color = gfx.HSV{H: hue, S: rnd(rng, 0.30, 0.70), V: rnd(rng, 0.20, 0.50)}.RGB()
	o.waveSeed = rng.Int()
	o.landSeed = rng.Int()
	o.seaLevel = rnd(rng, islandSeaLevelLo, islandSeaLevelHi)
	o.coast = rnd(rng, 0, islandCoastMax)
	o.sand = gfx.HSV{H: rnd(rng, 35, 50), S: rnd(rng, 0.25, 0.45), V: rnd(rng, 0.60, 0.82)}.RGB()
	return o
}

// elev is the land elevation at a below-horizon point: fractal noise plus a bias
// that rises toward the horizon, so distant coastline is common while the near
// water stays open with scattered islands.
func (o *ocean) elev(x, y int) float64 {
	d := float64(y-o.horizon) / float64(o.groundH)
	return gfx.FBM(float64(x)*islandFreqX, d*islandFreqY, o.landSeed, islandOctaves) + o.coast*(1-d)
}

// LandAt reports whether (x, y) is land. Above the horizon, and everywhere when
// there is no ocean, it is land; below the horizon with an ocean, land is where
// the elevation clears sea level.
func (o *ocean) LandAt(x, y int) bool {
	if !o.present || y <= o.horizon {
		return true
	}
	return o.elev(x, y) > o.seaLevel
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
