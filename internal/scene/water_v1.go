package scene

import (
	"image"
	"math"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Water1 is the v1 ocean. It embeds the v0 Water for its stream key and entity
// schema, but unlike ground.v1/cities.v1 it does NOT delegate to v0 in high mode:
// the ocean gets the perspective treatment at both vantages (mildly in high, fully
// in low), because the v0 sea — flat, screen-space shorelines and uniform ripple —
// reads wrong under perspective. Generation is unchanged (the ocean model is the
// shared global captured into the entity); the look is entirely a render transform.
//
// FROZEN once released: it keeps the "water" stream key and the WaterV0 schema of
// its embedded v0; add a Water2 for new behavior.
type Water1 struct{ Water }

// Generate is unchanged from v0: the ocean model is the shared global captured into
// the entity regardless of vantage point.
func (w *Water1) Generate(c *Context) (SceneList, error) {
	return w.Water.Generate(c)
}

// RenderList draws the ocean with perspective shorelines and a layered, perspective
// swell. It applies in both height modes — the strength comes from the resolved
// globals (Context.Perspective), which the director set mildly for high and fully
// for low.
func (w *Water1) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	oc, err := entityToOcean(list[0])
	if err != nil {
		return err
	}
	return renderOceanV1(c, oc, c.Perspective)
}

// beachPerspGain sets how much the sand band widens toward the viewer under full shore
// perspective: at the bottom of a low-vantage scene the beach spans (1+beachPerspGain)×
// its horizon width in elevation, so the shore broadens into the foreground the way
// perspective foreshortens the near ground. It is scaled by the shore-perspective
// strength, so the elevated (high) view keeps a nearly uniform shore ribbon.
const beachPerspGain = 3.0

// beachBandV1 returns the elevation width painted as beach at screen depth d (0 at the
// horizon, 1 at the bottom) under shore-perspective strength s (0..1). It widens linearly
// toward the viewer (see beachPerspGain), so the near shore reads as a broad beach and
// the far shore as a thin ribbon. The bush generator uses the same band to keep clumps
// off the sand, so the exclusion matches the beach water.v1 actually draws.
func beachBandV1(d, s float64) float64 {
	return islandBeachBand * (1 + beachPerspGain*clamp01(s)*clamp01(d))
}

// renderOceanV1 mirrors the scene above the horizon into a perspective sea. The shore
// boundary is bent by perspective via the ocean's perspective mapping — set here from
// the same globals newContext used for the cities' LandAt, so buildings stay on the
// land the water leaves dry. The swell is a perspective, multi-octave ripple: crests
// bunch into a calm mirror near the horizon and open into large swells toward the
// viewer (amplitude growing to p.WaveNear× the base), with a per-row level-of-detail
// that drops octaves where the compressed distance would alias. It consumes no
// randomness.
func renderOceanV1(c *Context, oc *ocean, p Perspective) error {
	oc = oc.withPerspective(p, c.W) // shore boundary matches the cities' LandAt
	w, h := c.W, c.H
	horizon, groundH := oc.horizon, oc.groundH
	if groundH < 1 {
		return nil
	}
	cx := float64(w) / 2
	span := float64(groundH)
	wcol := oc.color
	seed := oc.waveSeed
	baseWave := math.Max(waterWaveFrac*span, 2)
	waveNear := math.Max(p.WaveNear, 1)
	octaves := max(p.WaveOctaves, 1)
	s := clamp01(p.ShorePersp)
	b := oc.perspBias
	foam := gfx.RGB{R: wcol.R + (1-wcol.R)*islandFoamLift, G: wcol.G + (1-wcol.G)*islandFoamLift, B: wcol.B + (1-wcol.B)*islandFoamLift}

	// Per-row precompute: the perspective wave phase (a vertical noise coordinate that
	// bunches crests near the horizon and spreads them near the viewer), the lateral
	// world-frequency, the on-screen LOD frequency, and the amplitude/tint/darken ramps.
	rows := h - (horizon + 1)
	if rows < 1 {
		return nil
	}
	aEnd := (1 + b) * math.Log((1+b)/b)
	phase := make([]float64, rows)
	zEff := make([]float64, rows)
	amp := make([]float64, rows)
	for i := range phase {
		y := horizon + 1 + i
		d := float64(y-horizon) / span // 0 at horizon, 1 at bottom
		a := (1 + b) * math.Log((d+b)/b)
		vp := (1-s)*d + s*a/aEnd // 0..1, compressed toward the horizon
		phase[i] = vp * span * waterFreqY
		z := (1 + b) / (d + b)
		zEff[i] = 1 + s*(z-1)
		// Amplitude grows toward the viewer; bigger near, up to waveNear× the base.
		amp[i] = waterWaveMin + (baseWave*waveNear-waterWaveMin)*d*d
	}

	// The ground fills the horizon row, so an open-water column would otherwise show
	// a one-pixel land line between the sea and the sky. Draw the waterline on the
	// horizon row itself wherever the far water reaches it, so the ocean meets the
	// sky. It is the calm, distant water: the sky just above the horizon, water-
	// tinted at the horizon strength, with no ripple (the mirror is glassy there).
	// Land/coast columns keep their ground at the horizon (elev clears sea level).
	if horizon >= 1 {
		c.Canvas.Draw(func(img *image.RGBA) {
			for x := range w {
				if oc.elev(x, horizon) > oc.seaLevel {
					continue
				}
				off := img.PixOffset(x, horizon-1) // reflect the sky just above the horizon
				rr := float64(img.Pix[off]) / 255
				gg := float64(img.Pix[off+1]) / 255
				bb := float64(img.Pix[off+2]) / 255
				out := gfx.RGB{
					R: rr + (wcol.R-rr)*waterTintHorizon,
					G: gg + (wcol.G-gg)*waterTintHorizon,
					B: bb + (wcol.B-bb)*waterTintHorizon,
				}
				r8, g8, b8, _ := out.RGBA8()
				o := img.PixOffset(x, horizon)
				img.Pix[o] = r8
				img.Pix[o+1] = g8
				img.Pix[o+2] = b8
				img.Pix[o+3] = 255
			}
		})
	}

	bandH := max(groundH/80, 1)
	per := waterAnimDuration / time.Duration((groundH+bandH-1)/bandH)

	for y0 := horizon + 1; y0 < h; y0 += bandH {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y1 := min(y0+bandH, h)
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				row := y - (horizon + 1)
				d := float64(y-horizon) / span
				tint := waterTintHorizon + (waterTintForeground-waterTintHorizon)*d
				dark := 1 - waterDarkForeground*d
				ph := phase[row]
				ze := zEff[row]
				a := amp[row]
				// Level-of-detail from the on-screen wave frequency: the vertical crest
				// rate (how fast the phase advances per row) and the lateral rate.
				fCrest := 0.0
				if row+1 < rows {
					fCrest = math.Abs(phase[row+1] - phase[row])
				} else if row > 0 {
					fCrest = math.Abs(phase[row] - phase[row-1])
				}
				fLat := ze * waterFreqX
				oct, fade := groundLOD(math.Max(fCrest, fLat), octaves)
				aw := a * fade
				for x := range w {
					e := oc.elev(x, y)
					if e > oc.seaLevel {
						// The sand band widens toward the viewer, so the shore broadens into
						// the foreground under perspective (see beachBandV1).
						if beach := smoothstep(oc.seaLevel+beachBandV1(d, s), oc.seaLevel, e); beach > 0 {
							blendPixel(img, w, h, x, y, oc.sand, beach*islandBeachAmt)
						}
						continue
					}

					lateral := (float64(x) - cx) * ze * waterFreqX
					dx := (gfx.FBM(ph, lateral, seed, oct) - 0.5) * 2 * aw
					dy := (gfx.FBM(ph*0.7+10, lateral, seed+5, max(oct-1, 1)) - 0.5) * aw
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
