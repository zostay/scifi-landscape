package scene

import (
	"image"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Bushes scatters lopsided, squashed clumps of foliage across the ground, drawn far→near
// in front of the mountains. Each bush is anchored on the land below the horizon and is
// colored from the scene's own bush gradient (an independent global, NOT the ground):
// each bush picks one position along that gradient for its base color, then carries a
// soft mottle and speckle on top. Its size grows with nearness: in the low (ground-level)
// vantage a far bush is a few pixels and a near one can span a quarter of the scene width
// (large enough to occlude part of the view); in the high vantage bushes stay small
// everywhere. The outline is a rotated, squashed ellipse perturbed into a lopsided,
// soft-edged clump and cut off at the bottom at the ground contact line, so the bush
// reads as a rounded shrub rooted in the ground. The form is shaded as a bulging clump
// lit from the scene's light direction — the same shading angle the mountains use (and
// the alternate rugged style when the scene rolled it).
//
// Bushes appear only in front of the nearest mountain range at each column and below any
// ground mist: the generator consults the per-column bush floor that newContext derives
// from the same ranges the mountainranges element draws. They also root in soil, not on
// the sand, so none is placed on the shore's beach band. Bushes render LAST (frontmost),
// so a near clump can occlude the ground, water, and mountains behind it.
//
// This is a v1-era element (it reads the v1 globals); it draws from its own "bushes"
// stream, so it never disturbs another element's randomness.
type Bushes struct{}

func (r *Bushes) Name() string { return "bushes" }

// Schemas lists the entity schema keys this element owns.
func (r *Bushes) Schemas() []string { return []string{SchemaBushesV0} }

const (
	bushesAnimDuration = 700 * time.Millisecond
	// bushesReferenceWidth is the width (480px) at which a BushesBase.Count is expressed;
	// the generator scales the count by the actual width (NOT area). Bush sizes are
	// fractions of the width, so a near bush covers the same share of the frame at any
	// resolution — scaling the count by area would multiply those big near bushes on a
	// larger canvas and pile up the foreground. Width-scaling keeps the look stable.
	bushesReferenceWidth = 480.0
	// bushesMinGround is the smallest ground band (rows below the horizon) worth placing
	// bushes into.
	bushesMinGround = 4
	// bushesPlaceAttempts is how many placement tries per wanted bush before giving up,
	// so a heavily occluded scene (most of the ground behind ranges/over water) still
	// terminates rather than looping forever.
	bushesPlaceAttempts = 8

	// Surface look. The base color is the gradient sample, slightly desaturated, with a
	// broad value mottle plus sparse light and dark speckles for leafy texture.
	bushesBaseSat    = 0.82
	bushesMottleFreq = 0.09
	bushesMottleAmp  = 0.16
	bushesSpeckFreq  = 0.55
	bushesSpeckLight = 0.55 // brightness added by a light speckle
	bushesSpeckDark  = 0.45 // darkening from a dark speckle
	bushesSpeckHi    = 0.72 // noise above this is a light speckle
	bushesSpeckLo    = 0.28 // noise below this is a dark speckle
	// bushesLumpFreq scales the outline-perturbation noise sampled in the bush's local
	// pixel space; a few cycles across the bush give broad lobes rather than fuzz.
	bushesLumpFreq = 0.7
	bushesLumpOct  = 4
	// bushesTexSpeckOffset decorrelates the speckle field from the mottle field.
	bushesTexSpeckOffset = 9173
)

// Generate resolves the scene's bushes into a single entity. It reads the resolved base
// parameters from the globals (Context.Bushes), rolls a count for the vantage scaled by
// area, and places each bush — rejecting anchors that fall in water, on the shore's beach,
// or behind the nearest mountain range / under the mist (via Context.BushFloor) — varying its depth, size,
// squash, angle, burial, gradient color position, and seeds around the base. All
// randomness is drawn here, in a fixed order, on the element stream; it draws nothing, so
// identical globals (and the same bush floor) always yield an identical scene list. An
// empty list means no bushes (zero-value globals, a failed chance roll, or no room).
func (r *Bushes) Generate(c *Context) (SceneList, error) {
	bb := c.Bushes
	if bb.Chance <= 0 || bb.Count <= 0 {
		return nil, nil // no bushes configured (e.g. the v0 director)
	}
	horizon := c.Settings.HorizonY
	w, h := c.W, c.H
	groundH := h - horizon
	if horizon < 1 || groundH < bushesMinGround {
		return nil, nil // no ground to scatter bushes across
	}
	if c.Rng.Float64() >= bb.Chance {
		return nil, nil // this scene has no bushes
	}

	// The configured count is for the reference width; scale by the actual width (not
	// area) so the foreground does not pile up on a larger canvas (see the constant).
	want := int(math.Round(float64(bb.Count) * float64(w) / bushesReferenceWidth))
	if want <= 0 {
		return nil, nil
	}

	bushes := make([]bush, 0, want)
	for range want {
		for range bushesPlaceAttempts {
			// Anchor: a column and a depth fraction (0 at the horizon, 1 at the bottom).
			// These two draws happen on every attempt, accepted or not, so the stream stays
			// deterministic regardless of how many tries a bush takes.
			x := c.Rng.Intn(w)
			// Nearness drawn uniformly then biased toward the far distance (u^DepthBias):
			// near bushes grow large, so a value > 1 thins the foreground rather than
			// crowding it with big overlapping clumps. One draw, so the stream is unchanged.
			u := c.Rng.Float64()
			if bb.DepthBias > 0 {
				u = math.Pow(u, bb.DepthBias)
			}
			ay := horizon + int(u*float64(groundH))
			if ay >= h {
				ay = h - 1
			}
			// Reject anchors behind the nearest range or under the mist at this column.
			floor := horizon + 1
			if x < len(c.BushFloor) {
				floor = c.BushFloor[x]
			}
			if ay < floor || ay >= h {
				continue
			}
			// Bushes grow on land, never in open water.
			if c.LandAt != nil && !c.LandAt(x, ay) {
				continue
			}
			// And they root in soil, not on the sand: reject anchors within the shore's
			// beach band — the same perspective-widened band water.v1 paints (see
			// beachBandV1) — so a clump never sits on the beach, which broadens toward the
			// viewer in the low vantage.
			if oc := c.Ocean; oc != nil && oc.present {
				d := float64(ay-horizon) / float64(groundH)
				if oc.elev(x, ay) <= oc.seaLevel+beachBandV1(d, clamp01(c.Perspective.ShorePersp)) {
					continue
				}
			}

			// Accepted: roll this bush's shape and color. Draw order is fixed (jitter,
			// squash, angle, burial, color position, shape seed, texture seed) so the scene
			// list is reproducible.
			t := float64(ay-horizon) / float64(groundH) // 0 far, 1 near
			frac := bb.MinSizeFrac + (bb.MaxSizeFrac-bb.MinSizeFrac)*math.Pow(clamp01(t), bb.SizeGamma)
			jitter := 1 + (c.Rng.Float64()*2-1)*bb.SizeJitter
			diam := frac * float64(w) * jitter
			a := math.Max(diam/2, 1)
			squash := rnd(c.Rng, bb.SquashMin, bb.SquashMax)
			b := math.Max(a*squash, 0.8)
			theta := c.Rng.Float64() * math.Pi // any angle (an ellipse is π-periodic)
			bury := rnd(c.Rng, bb.BuryMin, bb.BuryMax)
			colorPos := c.Rng.Float64() // where along the scene's bush gradient this bush sits
			shapeSeed := c.Rng.Int()
			texSeed := c.Rng.Int()

			bushes = append(bushes, bush{
				x: x, y: ay, a: a, b: b, theta: theta, bury: bury, colorPos: colorPos,
				shapeSeed: shapeSeed, texSeed: texSeed,
			})
			break
		}
	}
	if len(bushes) == 0 {
		return nil, nil
	}

	// Draw far→near (smallest anchor row, nearest the horizon, first) so a nearer bush
	// occludes the ones behind it. Ties break by x for a stable order.
	sort.SliceStable(bushes, func(i, j int) bool {
		if bushes[i].y != bushes[j].y {
			return bushes[i].y < bushes[j].y
		}
		return bushes[i].x < bushes[j].x
	})

	return SceneList{bushesToEntity(bushes, bushesScene{ambient: bb.Ambient, lumpiness: bb.Lumpiness})}, nil
}

// RenderList draws the bushes entity onto the canvas. It is the only step that touches
// the image and it consumes no randomness, so the same scene list always draws the same
// pixels. It reads the scene's bush gradient (for each bush's base color) and the
// mountain shading style from the Context. Bushes animate one at a time, far→near.
func (r *Bushes) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	bushes, sc, err := entityToBushes(list[0])
	if err != nil {
		return err
	}
	if len(bushes) == 0 {
		return nil
	}
	w, h := c.W, c.H
	lx, ly, lz := bushLight(c.MountainRugged)

	// Spread the animation budget across the bushes so the whole field draws in about the
	// same time regardless of count.
	per := bushesAnimDuration / time.Duration(len(bushes))
	if per <= 0 {
		per = time.Millisecond
	}

	for _, bs := range bushes {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		base := c.BushGradient.At(bs.colorPos) // this bush's color from the scene's bush gradient
		c.Canvas.Draw(func(img *image.RGBA) {
			drawBush(img, w, h, bs, base, sc, lx, ly, lz)
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// bushLight returns the unit light direction for the bush form-shading, following the
// scene's mountain shading angle: the rugged style lights from the upper-left (like the
// mountains' rugged facets), the default conical style from the right and slightly above
// (like the mountains' conical hillshade, which brightens the right-facing side). y is
// screen-down, z points out of the screen toward the viewer.
func bushLight(rugged bool) (x, y, z float64) {
	if rugged {
		return -0.45, -0.45, 0.77
	}
	// Mostly from the right, a little from above: the right side of each bush lights up
	// and the left falls into shadow, matching the conical mountains.
	v := normalize3([3]float64{0.62, -0.30, 0.72})
	return v[0], v[1], v[2]
}

// drawBush renders one bush: a rotated, squashed, lopsided ellipse cut off at the ground
// contact line (bs.y) and form-shaded as a bulging clump with a leafy speckle. base is
// the color the bush sampled from the scene's bush gradient. It consumes no randomness.
func drawBush(img *image.RGBA, w, h int, bs bush, base gfx.HSV, sc bushesScene, lx, ly, lz float64) {
	a, b := bs.a, bs.b
	if a <= 0 || b <= 0 {
		return
	}
	cosT, sinT := math.Cos(bs.theta), math.Sin(bs.theta)

	// Vertical and horizontal half-extents of the rotated ellipse (before lumpiness),
	// padded a touch for the lobes. Center the bush above the contact line so a `bury`
	// fraction of its height falls below it (the buried, hidden part).
	pad := 1 + sc.lumpiness
	vExt := math.Sqrt(sq(a*sinT)+sq(b*cosT)) * pad
	hExt := math.Sqrt(sq(a*cosT)+sq(b*sinT)) * pad
	cx := float64(bs.x)
	cy := float64(bs.y) - vExt*(1-2*bs.bury)

	x0 := int(math.Floor(cx - hExt))
	x1 := int(math.Ceil(cx + hExt))
	y0 := int(math.Floor(cy - vExt))
	y1 := bs.y - 1 // never below the ground contact line (the rest is buried)
	if y1 >= h {
		y1 = h - 1
	}
	if y0 < 0 {
		y0 = 0
	}

	// Edge feather measured in normalized-radius units: one screen pixel is about
	// 1/min(a,b) in that space, so a ~1px soft edge antialiases the silhouette.
	edge := 1.5 / math.Max(math.Min(a, b), 1)

	for y := y0; y <= y1; y++ {
		dy := float64(y) - cy
		for x := x0; x <= x1; x++ {
			dx := float64(x) - cx
			// Into the bush's local frame (undo the rotation).
			lxp := dx*cosT + dy*sinT
			lyp := -dx*sinT + dy*cosT
			nx := lxp / a
			ny := lyp / b
			rho2 := nx*nx + ny*ny
			// Lopsided outline: perturb the unit boundary by a low-frequency noise sampled
			// in local space, so the silhouette lumps in and out instead of being a clean
			// ellipse.
			lump := sc.lumpiness * (gfx.FBM(lxp*bushesLumpFreq, lyp*bushesLumpFreq, bs.shapeSeed, bushesLumpOct) - 0.5)
			boundary := 1 + lump
			if boundary < 0.2 {
				boundary = 0.2
			}
			rho := math.Sqrt(rho2)
			cov := clamp01((boundary - rho) / edge)
			if cov <= 0 {
				continue
			}

			// Form-shading: treat the bush as a bulging ellipsoid. The local normalized
			// normal is (nx, ny, zl); rotate its lateral part back to screen space and light
			// it from the scene direction.
			zl := math.Sqrt(math.Max(1-rho2, 0))
			snx := nx*cosT - ny*sinT
			sny := nx*sinT + ny*cosT
			inv := 1 / math.Sqrt(snx*snx+sny*sny+zl*zl+1e-6)
			ndl := (snx*lx + sny*ly + zl*lz) * inv
			shade := sc.ambient + (1-sc.ambient)*clamp01(ndl)

			// Leafy texture: a broad value mottle plus sparse light/dark speckles.
			m := (gfx.FBM(lxp*bushesMottleFreq, lyp*bushesMottleFreq, bs.texSeed, 4) - 0.5) * bushesMottleAmp
			grain := 0.0
			sp := gfx.FBM(lxp*bushesSpeckFreq, lyp*bushesSpeckFreq, bs.texSeed+bushesTexSpeckOffset, 2)
			if sp > bushesSpeckHi {
				grain = (sp - bushesSpeckHi) / (1 - bushesSpeckHi) * bushesSpeckLight
			} else if sp < bushesSpeckLo {
				grain = -(bushesSpeckLo - sp) / bushesSpeckLo * bushesSpeckDark
			}

			col := gfx.HSV{
				H: base.H,
				S: base.S * bushesBaseSat,
				V: base.V * (1 + m) * shade * (1 + grain),
			}
			blendPixel(img, w, h, x, y, col.RGB(), cov)
		}
	}
}

// Bush coloring. Each scene gets its own independent bush gradient (a global derived by
// the director), and every bush samples one position along it. The base hue is fully
// random (any color — these are alien bushes, not necessarily green), and the palette
// comes in two flavors rolled per scene: a "mono" run of one hue from dark to light (a
// coherent clump color the bushes vary in shade across), or a "multi" gradient whose
// stops are independent random hues, so a scene's bushes span several vivid, contrasting
// colors. Values still climb dark→light so each bush keeps form-shading headroom; they
// are scaled by how much light the time of day gives (reusing the ground brightness).
const (
	bushMultiHueChance = 0.4  // chance a scene's bushes span several random hues, not one
	bushHueDrift       = 30.0 // per-stop hue wobble around the base in the mono palette
)

// buildBushGradient builds the scene-wide horizon-independent bush color gradient. It
// rolls a fully random base hue and (per scene) either a mono palette — that hue, dark→
// light, with a little per-stop drift — or a multi-hue palette whose every stop is an
// independent random hue, for wildly varied alien foliage. Bushes sample a position along
// it for their base color. It draws several values from rng on its own stream.
func buildBushGradient(rng *rand.Rand, t TimeOfDay) gfx.Gradient {
	br := groundBrightness(t)
	multi := rng.Float64() < bushMultiHueChance
	base := rng.Float64() * 360
	hue := func() float64 {
		if multi {
			return rng.Float64() * 360 // each stop its own color
		}
		return base + rnd(rng, -bushHueDrift, bushHueDrift) // one color, slight drift
	}
	return gfx.Gradient{
		{Pos: 0.0, Col: gfx.HSV{H: hue(), S: rnd(rng, 0.35, 1.0), V: rnd(rng, 0.14, 0.30) * br}},
		{Pos: 0.5, Col: gfx.HSV{H: hue(), S: rnd(rng, 0.35, 1.0), V: rnd(rng, 0.30, 0.48) * br}},
		{Pos: 1.0, Col: gfx.HSV{H: hue(), S: rnd(rng, 0.30, 0.95), V: rnd(rng, 0.48, 0.66) * br}},
	}
}
