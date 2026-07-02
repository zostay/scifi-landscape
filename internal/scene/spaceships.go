package scene

import (
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Spaceships hangs flying craft in the sky. Each ship is built procedurally from a tight
// cluster of overlaid, shaded shapes — ovals, triangles, rectangles, and rounded-corner
// squares — assembled along a shared long axis so the group reads as one hull. A small
// ship uses a few shapes; a large one uses many. One side of the ship is chosen as the
// "ear", and a bank of drive plumes flares from it: each plume is a tapered triangle whose
// centerline glows white at the base, fades to a bright color, and dissolves to
// transparent at the tip and along its edges.
//
// Ships render just after the clouds — they are in the sky, in front of the celestial
// bodies and clouds, but behind the horizon terrain and water, so a low-flying ship is
// occluded by the mountains and ground. To start, every scene has exactly one ship (the
// config's Count), so the rendering can be developed against a predictable scene.
//
// This is a v1-era element (it reads the v1 globals); it draws from its own "spaceships"
// stream, so it never disturbs another element's randomness.
type Spaceships struct{}

func (r *Spaceships) Name() string { return "spaceships" }

// Schemas lists the entity schema keys this element owns.
func (r *Spaceships) Schemas() []string { return []string{SchemaSpaceshipV0} }

const (
	shipsAnimDuration = 600 * time.Millisecond
	// shipSuperSample is the per-pixel supersampling grid for the hull shapes, so their
	// rotated silhouettes antialias cleanly against the sky.
	shipSuperSample = 3
	// shipNozzleShade darkens the nozzle relative to the hull, so the drive nozzles read as
	// scorched metal set apart from the bright hull panels.
	shipNozzleShade = 0.5

	// Surface bump map. The plating detail is lit as a height field: shipBumpStrength scales
	// how far the height gradient tilts the surface normal, and the light direction (screen-
	// left, up, and out) is shared by every shape so the relief reads consistently. The panel
	// and rivet lattices are sized in PIXELS (not in the shape's normalized space), so a small
	// greeble and a big hull get plating at the same physical scale rather than the same count.
	shipBumpStrength = 1.1
	shipBumpLightX   = 0.45
	shipBumpLightY   = -0.50
	shipBumpLightZ   = 0.74
	shipPanelPx      = 15.0 // panel cells ~15px across
	shipRivetPx      = 22.0 // rivet/port lattice ~22px
	shipGrainAmp     = 0.08 // fine surface grain (kept subtle so seams/rivets read cleanly)
)

// shipPart is one internal overlaid hull shape (mirrors ShipPartV0). It lives in ship-
// local pixel space with the origin at the ship center.
type shipPart struct {
	kind   int
	dx, dy float64
	hw, hh float64
	theta  float64
	shade  float64
	corner int
	cut    float64
	detail int
}

// shipPlume is one internal drive plume (mirrors ShipPlumeV0), in ship-local pixel space.
type shipPlume struct {
	ox, oy     float64
	dirX, dirY float64
	length     float64
	halfWidth  float64
	col        gfx.HSV
}

// shipNozzle is one internal drive nozzle (mirrors ShipNozzleV0), in ship-local pixel space.
type shipNozzle struct {
	nx, ny     float64
	ax, ay     float64
	length     float64
	narrowHalf float64
	wideHalf   float64
	shade      float64
}

// ship is the internal resolved craft produced by Generate and consumed by RenderList.
type ship struct {
	cx, cy   int
	hull     gfx.HSV
	ambient  float64
	parts    []shipPart
	greebles []shipPart
	plumes   []shipPlume
	nozzles  []shipNozzle
}

// Generate resolves the scene's spaceships into one entity per ship. It reads the resolved
// base parameters from the globals (Context.Spaceships), rolls the per-scene ship count from
// a normal distribution, then rolls each ship's position, size, hull color, overlaid parts,
// and drive plumes on the element stream, in a fixed draw order, so identical globals always
// yield an identical scene list. It draws nothing. An empty list means no ships (a zero
// count roll, the zero-value v0 globals, or no sky room).
func (r *Spaceships) Generate(c *Context) (SceneList, error) {
	sb := c.Spaceships
	if sb.CountMax <= 0 {
		return nil, nil // ships disabled (e.g. the v0 director's zero-value globals)
	}
	w := c.W
	horizon := c.Settings.HorizonY
	if w < 16 || horizon < 8 {
		return nil, nil // not enough sky to fly in
	}

	// Ship count ~ N(CountMean, CountStd), rounded and clamped to [0, CountMax]: most scenes
	// get a ship or two, larger fleets are increasingly rare. Rolled first, so the count draw
	// is a fixed point in the stream regardless of how many ships follow.
	count := max(0, min(sb.CountMax, int(math.Round(sb.CountMean+c.Rng.NormFloat64()*sb.CountStd))))

	var list SceneList
	for range count {
		list = append(list, shipToEntity(generateShip(c.Rng, sb, w, horizon)))
	}
	return list, nil
}

// generateShip rolls one ship's full layout. All randomness is drawn here in a fixed order
// (position, hull, body, extra parts, then plumes), on the passed stream, so a ship is
// fully reproducible from its scene-list entity with no render-time randomness.
func generateShip(rng *rand.Rand, sb SpaceshipsBase, w, horizon int) ship {
	// Overall size: a length in pixels between the min and max fraction of the width, with
	// a matching hull aspect ratio. sizeT (0 small … 1 large) drives the part count.
	length := rnd(rng, sb.MinSizeFrac, sb.MaxSizeFrac) * float64(w)
	span := (sb.MaxSizeFrac - sb.MinSizeFrac) * float64(w)
	sizeT := 0.0
	if span > 0 {
		sizeT = clamp01((length - sb.MinSizeFrac*float64(w)) / span)
	}
	aspect := rnd(rng, sb.AspectMin, sb.AspectMax)
	height := length * aspect
	halfL, halfH := length/2, height/2

	// Long axis: nearly horizontal with a slight tilt, so the ship reads as banking.
	orient := rnd(rng, -0.28, 0.28)
	cosO, sinO := math.Cos(orient), math.Sin(orient)

	// Position: center the ship in the configured sky band, kept fully on screen.
	top := int(sb.SkyTopFrac * float64(horizon))
	bot := int((1 - sb.SkyBotFrac) * float64(horizon))
	cy := placeInBand(rng, top, bot, int(math.Ceil(halfH))+1, horizon)
	cx := placeInBand(rng, 0, w, int(math.Ceil(halfL))+1, w)

	// Hull: a metallic, mostly-desaturated color.
	hull := gfx.HSV{H: rng.Float64() * 360, S: rnd(rng, 0.05, 0.30), V: rnd(rng, 0.55, 0.82)}

	s := ship{cx: cx, cy: cy, hull: hull, ambient: sb.Ambient}

	// The central body: a large oval spanning most of the hull, drawn first (backmost). Its
	// half-extents anchor greeble placement (on and inside the body outline).
	bodyHW := halfL * rnd(rng, 0.58, 0.78)
	bodyHH := halfH * rnd(rng, 0.72, 1.0)
	s.parts = append(s.parts, shipPart{
		kind:   ShipShapeOval,
		hw:     bodyHW,
		hh:     bodyHH,
		theta:  orient,
		shade:  1.0,
		detail: rng.Int(),
	})

	// Extra parts: a count that grows with size, scattered along the long axis in a tight
	// cluster (small perpendicular spread), each a random shape, size, tilt, and shade.
	n := max(sb.MinParts+int(math.Round(sizeT*float64(sb.MaxParts-sb.MinParts))), 1)
	for i := 1; i < n; i++ {
		along := rnd(rng, -0.52, 0.52) * length // position along the axis
		perp := rnd(rng, -0.32, 0.32) * height  // small offset off the axis
		dx := along*cosO - perp*sinO
		dy := along*sinO + perp*cosO
		part := shipPart{
			kind:   rng.Intn(4),
			dx:     dx,
			dy:     dy,
			hw:     rnd(rng, 0.10, 0.30) * length,
			hh:     rnd(rng, 0.10, 0.42) * height,
			theta:  orient + rnd(rng, -0.5, 0.5),
			shade:  rnd(rng, 0.72, 1.18),
			corner: rng.Intn(4),
			cut:    rnd(rng, 0.30, 0.72),
			detail: rng.Int(),
		}
		s.parts = append(s.parts, part)
	}

	// The ear: one long-axis end is the rear the drives fire from. Its outward direction is
	// ±the long axis; plumes spread along the rear edge (perpendicular to the axis).
	earSign := 1.0
	if rng.Float64() < 0.5 {
		earSign = -1.0
	}
	earX, earY := earSign*cosO, earSign*sinO // outward (away from the ship)
	perpX, perpY := -earY, earX              // along the rear edge
	earDist := halfL * rnd(rng, 0.70, 0.95)

	plumeHue := rng.Float64() * 360 // all of a ship's plumes share one bright color
	k := max(sb.MinPlumes+rng.Intn(sb.MaxPlumes-sb.MinPlumes+1), 1)
	for j := range k {
		// Spread the plumes evenly across the rear edge, centered on the axis.
		spread := 0.0
		if k > 1 {
			spread = ((float64(j)+0.5)/float64(k))*2 - 1 // in (-1, 1)
		}
		off := spread * halfH * rnd(rng, 0.40, 0.75)
		ox := earX*earDist + perpX*off
		oy := earY*earDist + perpY*off
		// A little fan so the bank of plumes splays out slightly.
		fan := rnd(rng, -0.12, 0.12)
		cf, sf := math.Cos(fan), math.Sin(fan)
		dirX := earX*cf - earY*sf
		dirY := earX*sf + earY*cf
		halfWidth := sb.PlumeWidthFrac * halfH * rnd(rng, 0.55, 0.95)
		s.plumes = append(s.plumes, shipPlume{
			ox: ox, oy: oy, dirX: dirX, dirY: dirY,
			length:    sb.PlumeLenFrac * length * rnd(rng, 0.70, 1.0),
			halfWidth: halfWidth,
			col:       gfx.HSV{H: plumeHue, S: rnd(rng, 0.70, 1.0), V: 1.0},
		})
		// A nozzle bridges this plume to the hull: its narrow end sits at the plume base
		// (matching the plume width) and its wide end runs back into the ship along the
		// plume axis, so the plume reads as firing out of the nozzle's throat.
		s.nozzles = append(s.nozzles, shipNozzle{
			nx: ox, ny: oy, ax: -dirX, ay: -dirY,
			length:     sb.NozzleLenFrac * length,
			narrowHalf: halfWidth,
			wideHalf:   halfWidth * sb.NozzleFlare,
			shade:      shipNozzleShade,
		})
	}

	// Greebles: a second layer of small shapes over the hull, count scaling with ship size.
	// Even-index greebles straddle the body outline (complicating the silhouette); the rest
	// sit inside it (interior detail). Each is placed relative to the central body ellipse,
	// then rotated into ship-local space.
	gc := sb.MinGreebles + int(math.Round(sizeT*float64(sb.MaxGreebles-sb.MinGreebles)))
	for i := range gc {
		ang := rng.Float64() * 2 * math.Pi
		rr := rnd(rng, 0.15, 0.85) // interior: somewhere inside the body
		if i%2 == 0 {
			rr = rnd(rng, 0.90, 1.05) // edge: on the rim, so it half-protrudes
		}
		bx := rr * bodyHW * math.Cos(ang)
		by := rr * bodyHH * math.Sin(ang)
		size := rnd(rng, sb.GreebleSizeMin, sb.GreebleSizeMax) * height
		s.greebles = append(s.greebles, shipPart{
			kind:   rng.Intn(4),
			dx:     bx*cosO - by*sinO,
			dy:     bx*sinO + by*cosO,
			hw:     size * rnd(rng, 0.50, 1.10),
			hh:     size * rnd(rng, 0.40, 0.90),
			theta:  orient + rng.Float64()*math.Pi,
			shade:  rnd(rng, 0.70, 1.20),
			corner: rng.Intn(4),
			cut:    rnd(rng, 0.30, 0.72),
			detail: rng.Int(),
		})
	}
	return s
}

// placeInBand returns a center coordinate uniformly within [lo, hi) but pulled in by pad
// on each side so the ship stays fully within [0, extent). If the padded band collapses,
// it centers on the band midpoint.
func placeInBand(rng *rand.Rand, lo, hi, pad, extent int) int {
	a, b := lo+pad, hi-pad
	if a > extent-1-pad {
		a = extent - 1 - pad
	}
	if b > extent-pad {
		b = extent - pad
	}
	if a >= b {
		return (lo + hi) / 2
	}
	return a + rng.Intn(b-a)
}

// RenderList draws the ships onto the canvas. It is the only step that touches the image
// and it consumes no randomness, so the same scene list always draws the same pixels.
// Ships animate one at a time; within a ship the plumes are drawn first (behind the hull),
// then the overlaid parts back→front.
func (r *Spaceships) RenderList(c *Context, list SceneList) error {
	if len(list) == 0 {
		return nil
	}
	w, h := c.W, c.H
	per := shipsAnimDuration / time.Duration(len(list))
	if per <= 0 {
		per = time.Millisecond
	}
	for _, e := range list {
		if err := c.Ctx.Err(); err != nil {
			return err
		}
		s, err := entityToShip(e)
		if err != nil {
			return err
		}
		c.Canvas.Draw(func(img *image.RGBA) {
			drawShip(img, w, h, s)
		})
		if err := sleep(c.Ctx, per); err != nil {
			return err
		}
	}
	return nil
}

// drawShip renders one ship back→front: its drive plumes first (behind), then the drive
// nozzles one layer above the plumes (so each nozzle's narrow end covers the plume base
// and the plume reads as emerging from it), then the overlaid hull parts on top (so the
// hull occludes the nozzles' wide ends where they tuck into the ship). It consumes no
// randomness.
func drawShip(img *image.RGBA, w, h int, s ship) {
	for _, pl := range s.plumes {
		drawPlume(img, w, h, s.cx, s.cy, pl)
	}
	for _, nz := range s.nozzles {
		drawNozzle(img, w, h, s.cx, s.cy, s.hull, s.ambient, nz)
	}
	for _, p := range s.parts {
		drawShipPart(img, w, h, s.cx, s.cy, s.hull, s.ambient, p)
	}
	for _, g := range s.greebles {
		drawShipPart(img, w, h, s.cx, s.cy, s.hull, s.ambient, g)
	}
}

// drawShipPart rasterizes one hull shape: a rotated silhouette (per its Kind) filled with
// the hull color, top-lit for a simple metallic form and modulated by the part's own shade
// so overlapping panels read apart. Edges are antialiased by supersampling.
func drawShipPart(img *image.RGBA, w, h int, cx, cy int, hull gfx.HSV, ambient float64, p shipPart) {
	if p.hw <= 0 || p.hh <= 0 {
		return
	}
	ccx := float64(cx) + p.dx
	ccy := float64(cy) + p.dy
	cosT, sinT := math.Cos(p.theta), math.Sin(p.theta)

	// Screen-space bounding box of the rotated shape, padded a pixel.
	ext := math.Hypot(p.hw, p.hh) + 1
	x0 := int(math.Floor(ccx - ext))
	x1 := int(math.Ceil(ccx + ext))
	y0 := int(math.Floor(ccy - ext))
	y1 := int(math.Ceil(ccy + ext))

	step := 1.0 / float64(shipSuperSample)
	base := step / 2

	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			var covSum, litSum float64
			for sy := range shipSuperSample {
				for sx := range shipSuperSample {
					px := float64(x) + base + float64(sx)*step
					py := float64(y) + base + float64(sy)*step
					dx := px - ccx
					dy := py - ccy
					// Into the part's local frame (undo the rotation) and normalize.
					lu := (dx*cosT + dy*sinT) / p.hw
					lv := (-dx*sinT + dy*cosT) / p.hh
					if !shapeContains(p, lu, lv) {
						continue
					}
					covSum++
					// Top-lit: the local-up edge (lv = -1) is brightest, the bottom darkest.
					litSum += clamp01(0.5 - 0.5*lv)
				}
			}
			if covSum <= 0 {
				continue
			}
			cov := covSum / float64(shipSuperSample*shipSuperSample)
			lit := litSum / covSum
			// Surface bump map at the pixel center: panel seams, blocks, rivets, and grain
			// modulate the shade so the flat shape reads as detailed ship plating.
			clu := (float64(x) + 0.5 - ccx)
			clv := (float64(y) + 0.5 - ccy)
			lu := (clu*cosT + clv*sinT) / p.hw
			lv := (-clu*sinT + clv*cosT) / p.hh
			detail := shipDetailFactor(lu, lv, p.hw, p.hh, p.detail)
			shade := (ambient + (1-ambient)*lit) * detail
			col := gfx.HSV{H: hull.H, S: hull.S, V: clamp01(hull.V * p.shade * shade)}
			blendPixel(img, w, h, x, y, col.RGB(), cov)
		}
	}
}

// drawNozzle rasterizes one drive nozzle: a trapezoid running from a narrow end (at the
// plume base) to a wider end (into the hull) along its axis, filled with the hull color
// darkened by the nozzle's shade and top-lit like the hull parts. Edges are antialiased by
// supersampling. It consumes no randomness.
func drawNozzle(img *image.RGBA, w, h int, cx, cy int, hull gfx.HSV, ambient float64, nz shipNozzle) {
	if nz.length <= 0 || (nz.narrowHalf <= 0 && nz.wideHalf <= 0) {
		return
	}
	nxp := float64(cx) + nz.nx
	nyp := float64(cy) + nz.ny
	ax, ay := nz.ax, nz.ay // unit axis, narrow → wide
	px, py := -ay, ax      // perpendicular (across the trapezoid)

	// Screen-space bounding box over the four corners, padded a pixel.
	wx, wy := nxp+ax*nz.length, nyp+ay*nz.length // wide-end center
	corners := [4][2]float64{
		{nxp + px*nz.narrowHalf, nyp + py*nz.narrowHalf},
		{nxp - px*nz.narrowHalf, nyp - py*nz.narrowHalf},
		{wx + px*nz.wideHalf, wy + py*nz.wideHalf},
		{wx - px*nz.wideHalf, wy - py*nz.wideHalf},
	}
	x0, y0 := corners[0][0], corners[0][1]
	x1, y1 := x0, y0
	for _, c := range corners {
		x0, y0 = math.Min(x0, c[0]), math.Min(y0, c[1])
		x1, y1 = math.Max(x1, c[0]), math.Max(y1, c[1])
	}
	ix0, iy0 := int(math.Floor(x0))-1, int(math.Floor(y0))-1
	ix1, iy1 := int(math.Ceil(x1))+1, int(math.Ceil(y1))+1

	// Top-lit: the side of the trapezoid higher on screen is brightest. The perpendicular's
	// screen-y component (== ax) tells us which cross direction points down-screen.
	var upSign float64
	if ax > 0 {
		upSign = 1
	} else if ax < 0 {
		upSign = -1
	}

	step := 1.0 / float64(shipSuperSample)
	base := step / 2
	for y := iy0; y <= iy1; y++ {
		for x := ix0; x <= ix1; x++ {
			var covSum, litSum float64
			for sy := range shipSuperSample {
				for sx := range shipSuperSample {
					qx := float64(x) + base + float64(sx)*step
					qy := float64(y) + base + float64(sy)*step
					rx, ry := qx-nxp, qy-nyp
					s := rx*ax + ry*ay // along the axis
					if s < 0 || s > nz.length {
						continue
					}
					halfAt := nz.narrowHalf + (nz.wideHalf-nz.narrowHalf)*(s/nz.length)
					if halfAt <= 0 {
						continue
					}
					cross := rx*px + ry*py
					if math.Abs(cross) > halfAt {
						continue
					}
					covSum++
					litSum += clamp01(0.5 - 0.5*(cross/halfAt)*upSign)
				}
			}
			if covSum <= 0 {
				continue
			}
			cov := covSum / float64(shipSuperSample*shipSuperSample)
			lit := litSum / covSum
			shade := ambient + (1-ambient)*lit
			col := gfx.HSV{H: hull.H, S: hull.S, V: clamp01(hull.V * nz.shade * shade)}
			blendPixel(img, w, h, x, y, col.RGB(), cov)
		}
	}
}

// shipDetailFactor returns the surface-detail brightness multiplier at ship-local shape
// coordinates (u, v) for a shape whose pixel half-extents are (hw, hh) and which is seeded
// by seed. It combines a flat per-panel albedo (blocks of slightly varied tone) with bump-
// map lighting of a height field (recessed panel seams, raised rivets / recessed ports, and
// fine grain), so a plain filled shape reads as paneled, riveted ship plating. The lattices
// are sized in pixels via (hw, hh), so detail stays a consistent physical scale across
// shapes. It is a pure function of its inputs — it draws no randomness — so the render stays
// reproducible.
func shipDetailFactor(u, v, hw, hh float64, seed int) float64 {
	// Flat panel tone: quantize into a pixel-sized grid of cells, each a different brightness.
	nx := math.Max(hw/shipPanelPx, 1)
	ny := math.Max(hh/shipPanelPx, 1)
	cx := math.Floor(u * nx)
	cy := math.Floor(v * ny)
	tone := 1 + (hashCell(int(cx), int(cy), seed)-0.5)*0.22

	// Bump lighting from the height field's local gradient (central finite differences). The
	// step is a fixed pixel distance (converted into u/v space) so the slopes read the same
	// at any shape size.
	eu := shipBumpEps / math.Max(hw, 1)
	ev := shipBumpEps / math.Max(hh, 1)
	dhdu := (shipBumpHeight(u+eu, v, hw, hh, seed) - shipBumpHeight(u-eu, v, hw, hh, seed)) / (2 * eu)
	dhdv := (shipBumpHeight(u, v+ev, hw, hh, seed) - shipBumpHeight(u, v-ev, hw, hh, seed)) / (2 * ev)
	nX := -dhdu * shipBumpStrength
	nY := -dhdv * shipBumpStrength
	nZ := 1.0
	inv := 1 / math.Sqrt(nX*nX+nY*nY+nZ*nZ)
	ndl := (nX*shipBumpLightX + nY*shipBumpLightY + nZ*shipBumpLightZ) * inv
	f := ndl / shipBumpLightZ // 1 on a flat face; > 1 facing the light, < 1 facing away

	return clamp(tone*f, 0.55, 1.6)
}

// shipBumpEps is the finite-difference step (in pixels) used to sample the height field's
// gradient, converted into normalized u/v space per shape by shipDetailFactor.
const shipBumpEps = 0.9

// shipBumpHeight is the ship-plating height field sampled in shape-local coordinates for a
// shape of pixel half-extents (hw, hh): a pixel-sized grid of recessed panel seams (lines),
// scattered raised rivets and recessed ports on a coarser lattice (circles), and a little
// fine grain. shipDetailFactor lights its gradient to fake surface relief. Heights are in
// normalized units; the gradient is what matters.
func shipBumpHeight(u, v, hw, hh float64, seed int) float64 {
	h := 0.0

	// Panel seams: grooves along the borders of the pixel-sized panel grid (lines).
	nx := math.Max(hw/shipPanelPx, 1)
	ny := math.Max(hh/shipPanelPx, 1)
	gu := u*nx - math.Floor(u*nx)
	gv := v*ny - math.Floor(v*ny)
	d := math.Min(math.Min(gu, 1-gu), math.Min(gv, 1-gv)) // fraction to the nearest seam
	const seamW = 0.18
	if d < seamW {
		h -= (1 - d/seamW) * 0.7 // recessed toward the seam
	}

	// Rivets and ports on a coarser pixel-sized lattice, jittered within each cell (circles).
	rnu := math.Max(hw/shipRivetPx, 1)
	rnv := math.Max(hh/shipRivetPx, 1)
	ru, rv := u*rnu, v*rnv
	rcu, rcv := math.Floor(ru), math.Floor(rv)
	present := hashCell(int(rcu), int(rcv), seed*131+7)
	if present > 0.45 {
		ju := 0.28 + 0.44*hashCell(int(rcu), int(rcv), seed*131+8)
		jv := 0.28 + 0.44*hashCell(int(rcu), int(rcv), seed*131+9)
		dd := math.Hypot((ru-rcu)-ju, (rv-rcv)-jv)
		const rad = 0.2
		if dd < rad {
			t := 1 - dd/rad
			if present > 0.80 {
				h -= t * 0.9 // a recessed port
			} else {
				h += t * 0.8 // a raised rivet / boss
			}
		}
	}

	// Fine surface grain, sampled in pixel space so it is a consistent scale (kept subtle).
	h += (gfx.FBM(u*hw/6, v*hh/6, seed, 3) - 0.5) * shipGrainAmp
	return h
}

// hashCell is a deterministic hash of an integer cell (i, j) and a seed to [0, 1). It gives
// per-cell panel tones and rivet placement without touching any random stream.
func hashCell(i, j, seed int) float64 {
	x := uint32(i)*374761393 + uint32(j)*668265263 + uint32(seed)*2246822519
	x = (x ^ (x >> 13)) * 3266489917
	x ^= x >> 16
	return float64(x) / float64(^uint32(0))
}

// shapeContains reports whether the ship-local normalized point (u, v) — each already in
// [-1, 1] at the shape's half-extents — lies inside the part's silhouette.
func shapeContains(p shipPart, u, v float64) bool {
	switch p.kind {
	case ShipShapeOval:
		return u*u+v*v <= 1
	case ShipShapeTriangle:
		// Isosceles triangle: apex at (0, -1), base edge at v = 1; width grows with v.
		if v < -1 || v > 1 {
			return false
		}
		return math.Abs(u) <= (v+1)/2
	case ShipShapeRect:
		return math.Abs(u) <= 1 && math.Abs(v) <= 1
	case ShipShapeRoundedRect:
		if math.Abs(u) > 1 || math.Abs(v) > 1 {
			return false
		}
		// Round one corner: in signed coords where the chosen corner is (+1, +1), any point
		// inside the r-square nearest that corner must lie within radius r of its inner point.
		sx, sy := 1.0, 1.0
		switch p.corner {
		case 0:
			sx, sy = -1, -1
		case 1:
			sx, sy = 1, -1
		case 2:
			sx, sy = 1, 1
		default:
			sx, sy = -1, 1
		}
		su, sv := sx*u, sy*v
		r := p.cut
		if su > 1-r && sv > 1-r {
			ddx, ddy := su-(1-r), sv-(1-r)
			return ddx*ddx+ddy*ddy <= r*r
		}
		return true
	default:
		return false
	}
}

// drawPlume renders one drive plume as an additive glow: a triangle tapering from a base
// half-width at the ship to a point at the tip. The centerline is white near the base,
// fades through the plume's bright color, and dissolves to transparent at the tip and
// toward the edges. Additive blending lets the bright core read as white over the sky.
func drawPlume(img *image.RGBA, w, h int, cx, cy int, pl shipPlume) {
	if pl.length <= 0 || pl.halfWidth <= 0 {
		return
	}
	bx := float64(cx) + pl.ox
	by := float64(cy) + pl.oy
	dx, dy := pl.dirX, pl.dirY // unit direction (away from the ship)
	px, py := -dy, dx          // perpendicular

	col := pl.col.RGB()
	white := gfx.RGB{R: 1, G: 1, B: 1}

	// Bounding box over the triangle's three corners.
	tipX, tipY := bx+dx*pl.length, by+dy*pl.length
	c1x, c1y := bx+px*pl.halfWidth, by+py*pl.halfWidth
	c2x, c2y := bx-px*pl.halfWidth, by-py*pl.halfWidth
	x0 := int(math.Floor(math.Min(math.Min(tipX, c1x), c2x))) - 1
	x1 := int(math.Ceil(math.Max(math.Max(tipX, c1x), c2x))) + 1
	y0 := int(math.Floor(math.Min(math.Min(tipY, c1y), c2y))) - 1
	y1 := int(math.Ceil(math.Max(math.Max(tipY, c1y), c2y))) + 1

	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			rx := float64(x) - bx
			ry := float64(y) - by
			s := rx*dx + ry*dy // distance along the plume
			if s < 0 || s > pl.length {
				continue
			}
			sn := s / pl.length
			wAt := pl.halfWidth * (1 - sn)
			if wAt <= 0 {
				continue
			}
			cross := math.Abs(rx*px+ry*py) / wAt
			if cross > 1 {
				continue
			}
			centerF := 1 - cross // 1 on the centerline, 0 at the triangle edge
			// White is concentrated at the base and along the centerline; it gives way to the
			// bright color moving out along the plume and toward the edges.
			whiteMix := clamp01(1-sn*1.6) * centerF
			glow := gfx.RGB{
				R: col.R + (white.R-col.R)*whiteMix,
				G: col.G + (white.G-col.G)*whiteMix,
				B: col.B + (white.B-col.B)*whiteMix,
			}
			// Fade to transparent toward the tip and toward the edges.
			alpha := (1 - sn) * (0.30 + 0.70*centerF)
			addPixel(img, w, h, x, y, glow, alpha)
		}
	}
}
