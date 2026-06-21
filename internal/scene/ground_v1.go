package scene

import (
	"image"
	"math"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Ground1 is the v1 base terrain. It embeds the v0 Ground and behaves identically
// at the high vantage (High mode delegates straight through, so a high scene is
// byte-identical to v0). In low (ground-level) mode it redraws the same terrain
// entity (generation is unchanged) through a perspective projection that fans the
// texture out horizontally toward the viewer and compresses it toward a central
// vanishing point on the horizon — the at-ground-level look.
//
// FROZEN once released: it keeps the "ground" stream key and the GroundV0 schema of
// its embedded v0; add a Ground2 for new behavior.
type Ground1 struct{ Ground }

// Generate is unchanged from v0: the same terrain seeds are drawn regardless of
// vantage point, so the low look is purely a render-time transform.
func (g *Ground1) Generate(c *Context) (SceneList, error) {
	return g.Ground.Generate(c)
}

// RenderList draws the terrain. In High mode it delegates to the v0 ground (byte-
// identical). In Low mode it draws the same entity with the perspective sampler.
func (g *Ground1) RenderList(c *Context, list SceneList) error {
	if c.Height != Low {
		return g.Ground.RenderList(c, list)
	}
	if len(list) == 0 {
		return nil
	}
	terr, err := entityToGround(list[0])
	if err != nil {
		return err
	}
	return renderGroundLow(c, terr, c.Perspective)
}

// groundLOD returns the level-of-detail for a row whose base value-noise samples at
// f cycles per pixel on screen. It drops the octaves whose frequency (f doubling per
// octave) would exceed the Nyquist limit, and returns a fade that ramps the texture
// amplitude from full at Nyquist down to zero once even the base octave aliases — so
// the far, compressed ground turns to smooth haze instead of moiré. maxOct caps the
// octave count (the un-aliased near ground gets all of them).
func groundLOD(f float64, maxOct int) (oct int, fade float64) {
	const nyquist = 0.5
	oct = maxOct
	for oct > 1 && f*math.Pow(2, float64(oct-1)) > nyquist {
		oct--
	}
	fade = 1.0
	if f > nyquist {
		fade = clamp01(1 - (f-nyquist)/nyquist) // 1 at Nyquist, 0 at twice Nyquist
	}
	return oct, fade
}

// lowGroundOctaves is the octave count of the low-mode terrain noise. It is higher
// than the v0 ground's groundOctaves so the near ground keeps fine grain layered on
// top of the large base blobs (GroundNearCell): the coarse octaves persist far toward
// the horizon while the per-row level-of-detail drops the fine ones with distance.
const lowGroundOctaves = 7

// renderGroundLow draws a ground terrain in the low (ground-level) vantage as a flat
// plane seen at a grazing angle. Each row is given a perspective depth — 1 at the
// nearest (bottom) row, growing hyperbolically toward the horizon (~1/row), so the
// distance shrinks away fast. The noise is sampled in world space with BOTH axes
// scaled by that same depth and measured from the central vanishing point, so the
// dirt stays isotropic (round blobs, not streaks or curves) and converges naturally:
// near ground reads as detailed dirt at p.GroundNearCell-pixel grain, the far field
// crushes into a thin band. p.GroundContrast scales the light/dark; a per-row
// level-of-detail drops octaves and fades the texture to smooth where the compressed
// distance would otherwise alias into moiré. It consumes no randomness.
func renderGroundLow(c *Context, terr groundTerrain, p Perspective) error {
	horizon := c.Settings.HorizonY
	w, h := c.W, c.H
	rows := h - horizon
	if rows < 1 {
		return nil
	}
	span := float64(rows)
	cx := float64(w) / 2

	variable := c.GroundVariable
	grad := c.GroundGradient
	seed := terr.seed
	wanderSeed := terr.wanderSeed

	nearCell := p.GroundNearCell
	if nearCell <= 0 {
		nearCell = 6
	}
	bias := p.GroundBias
	if bias <= 0 {
		bias = 1
	}
	// Per-row depth, normalized so the bottom row is 1 and the horizon row is large.
	denom := (span - 1 + bias) // depth at i = rows-1 is 1
	depth := make([]float64, rows)
	for i := range depth {
		d := denom / (float64(i) + bias) // 1 at the bottom, grows toward the horizon
		if p.GroundGamma != 1 && p.GroundGamma > 0 {
			d = math.Pow(d, p.GroundGamma)
		}
		depth[i] = d
	}
	const wanderRatioX = groundWanderFreqX / groundFreqX // wander samples at a broader scale

	// The detail layer samples this many times finer than the macro layer; its seed
	// is offset well past the macro octave seeds so the two layers are uncorrelated.
	detailCell := p.GroundDetailCell
	if detailCell <= 0 {
		detailCell = nearCell / 8
	}
	detailRatio := nearCell / detailCell
	detailSeed := seed + 1000003

	bands := groundBands(horizon, h)
	per := groundAnimDuration / time.Duration(len(bands))

	for _, b := range bands {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		y0, y1 := b[0], b[1]
		c.Canvas.Draw(func(img *image.RGBA) {
			for y := y0; y < y1; y++ {
				row := y - horizon
				t := float64(row) / span
				amp := groundValueAmp * (0.5 + 0.8*t) * p.GroundContrast
				dz := depth[row]
				// World noise coordinates: nz is the depth (vertical), nx the lateral
				// offset from the vanishing point, both scaled by depth so the texture
				// is isotropic. nx = off*dz/nearCell, nz = dz*span/nearCell.
				nz := dz * span / nearCell
				fx := dz / nearCell // macro horizontal on-screen frequency (per pixel)
				// vertical on-screen frequency: how fast depth changes between rows.
				fy := fx
				if row+1 < rows {
					fy = (depth[row] - depth[row+1]) * span / nearCell
				} else if row > 0 {
					fy = (depth[row-1] - depth[row]) * span / nearCell
				}
				fLOD := math.Max(fx, fy)
				// Macro layer (big blobs) and the finer detail layer each get their own
				// level-of-detail: the detail layer samples detailRatio× finer, so it
				// aliases — and is faded out — sooner with distance, while the macro
				// structure carries on toward the horizon.
				octM, fadeM := groundLOD(fLOD, lowGroundOctaves)
				octD, fadeD := groundLOD(fLOD*detailRatio, lowGroundOctaves)
				octW, fadeW := groundLOD(fLOD*math.Max(wanderRatioX, groundWanderVScale), 3)
				for x := range w {
					off := float64(x) - cx
					nx := off * dz / nearCell
					ct := t
					if variable {
						wn := gfx.FBM(nx*wanderRatioX, nz*groundWanderVScale, wanderSeed, octW)
						ct = min(max(t+(wn-0.5)*groundWanderAmp*fadeW, 0), 1)
					}
					base := grad.At(ct)

					nM := gfx.FBM(nx, nz, seed, octM)
					nD := gfx.FBM(nx*detailRatio, nz*detailRatio, detailSeed, octD)
					dd := (nM-0.5)*fadeM + (nD-0.5)*p.GroundDetailAmt*fadeD
					col := gfx.HSV{
						H: base.H + dd*groundHueAmp,
						S: base.S * (1 + dd*groundSatAmp),
						V: base.V * (1 + dd*amp),
					}.RGB()
					r, gg, bb, _ := col.RGBA8()
					o := img.PixOffset(x, y)
					img.Pix[o] = r
					img.Pix[o+1] = gg
					img.Pix[o+2] = bb
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
