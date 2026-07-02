package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Spaceship entity schema. Each flying craft resolves to one entity holding everything
// the renderer needs with no further randomness: the ship's screen position, its base
// hull color and shadow-fill, the ordered list of overlaid hull shapes (drawn back→front),
// and the drive plumes flaring from its ear side. Colors are stored directly (unlike the
// bushes, which sample a scene gradient) because each ship rolls its own hull and plume
// hues.
//
// This schema is FROZEN: add fields if needed, but never rename, retype, or repurpose an
// existing one — make a V1 instead. The yaml keys are the on-disk contract and are pinned
// with explicit tags.
const SchemaSpaceshipV0 = "spaceship.v0"

func init() {
	RegisterEntity(SchemaSpaceshipV0, func() Entity { return &SpaceshipV0{} })
}

// Hull-shape kinds. These integer codes are part of the frozen on-disk contract: the
// meaning of an existing value never changes; new shapes get new codes appended.
const (
	ShipShapeOval        = 0 // an ellipse
	ShipShapeTriangle    = 1 // an isosceles triangle pointing along local -Y (before rotation)
	ShipShapeRect        = 2 // an axis-aligned (then rotated) rectangle
	ShipShapeRoundedRect = 3 // a rectangle with one circular corner cut-out
)

// SpaceshipV0 is one fully-resolved flying craft. CX,CY is the ship's center on screen;
// Hull is the base hull color and Ambient the shadow-side fill for its top-lit form
// shading. Parts are the overlaid hull shapes in back→front draw order; Plumes are the
// drive plumes, drawn behind the hull so they flare out from under the ear side; Nozzles
// are the trapezoids that connect each plume to the hull, drawn one layer above the plumes
// (so the plume reads as emerging from the nozzle's narrow end) but below the hull parts.
// Greebles are a second layer of small detail shapes drawn over the hull parts: some
// straddle the outline (complicating the silhouette), some sit inside it (interior detail).
type SpaceshipV0 struct {
	CX       int            `yaml:"cx"`
	CY       int            `yaml:"cy"`
	Hull     gfx.HSV        `yaml:"hull"`
	Ambient  float64        `yaml:"ambient"`
	Parts    []ShipPartV0   `yaml:"parts"`
	Plumes   []ShipPlumeV0  `yaml:"plumes"`
	Nozzles  []ShipNozzleV0 `yaml:"nozzles"`
	Greebles []ShipPartV0   `yaml:"greebles"`
}

func (*SpaceshipV0) EntitySchema() string { return SchemaSpaceshipV0 }

// ShipPartV0 is one overlaid hull shape in ship-local pixel space (origin at the ship
// center). Kind selects the silhouette (see the ShipShape* codes); DX,DY is the shape's
// center offset from the ship center; HW,HH are its half-extents; Theta is its rotation
// in radians; Shade multiplies the hull value so panels vary in brightness; Corner (only
// for a rounded-corner square) selects which corner is cut (0..3 for the four corners)
// and Cut is the cut radius as a fraction of the smaller half-extent. Detail seeds the
// procedural surface bump map (panel seams, blocks, rivets, and fine grain) the renderer
// overlays on the shape.
type ShipPartV0 struct {
	Kind   int     `yaml:"kind"`
	DX     float64 `yaml:"dx"`
	DY     float64 `yaml:"dy"`
	HW     float64 `yaml:"hw"`
	HH     float64 `yaml:"hh"`
	Theta  float64 `yaml:"theta"`
	Shade  float64 `yaml:"shade"`
	Corner int     `yaml:"corner"`
	Cut    float64 `yaml:"cut"`
	Detail int     `yaml:"detail"`
}

// ShipPlumeV0 is one drive plume in ship-local pixel space. OX,OY is the plume base at
// the ear side of the ship; DirX,DirY is the unit direction it flares (away from the
// ship); Len is its length and HalfWidth its base half-width (it tapers to a point at the
// tip). Col is the plume's bright color; the centerline fades from white (at the base)
// through Col and out to transparent at the tip and along the edges.
type ShipPlumeV0 struct {
	OX        float64 `yaml:"ox"`
	OY        float64 `yaml:"oy"`
	DirX      float64 `yaml:"dirX"`
	DirY      float64 `yaml:"dirY"`
	Len       float64 `yaml:"len"`
	HalfWidth float64 `yaml:"halfWidth"`
	Col       gfx.HSV `yaml:"col"`
}

// ShipNozzleV0 is one drive nozzle in ship-local pixel space: a trapezoid whose narrow
// end sits at the plume base (aligned to the plume's width) and whose wide end tucks back
// into the hull, connecting the plume to the ship. NX,NY is the narrow-end center; AX,AY
// is the unit axis pointing from the narrow end toward the wide (hull) end; Len is the
// trapezoid's length along that axis; NarrowHalf and WideHalf are the half-widths at the
// two ends; Shade multiplies the hull value (nozzles render darker than the hull).
type ShipNozzleV0 struct {
	NX         float64 `yaml:"nx"`
	NY         float64 `yaml:"ny"`
	AX         float64 `yaml:"ax"`
	AY         float64 `yaml:"ay"`
	Len        float64 `yaml:"len"`
	NarrowHalf float64 `yaml:"narrowHalf"`
	WideHalf   float64 `yaml:"wideHalf"`
	Shade      float64 `yaml:"shade"`
}

// partToEntity/entityToPart convert one hull shape between its internal and schema forms.
// Both hull parts and greebles are ShipPartV0, so they share this conversion.
func partToEntity(p shipPart) ShipPartV0 {
	return ShipPartV0{
		Kind: p.kind, DX: p.dx, DY: p.dy, HW: p.hw, HH: p.hh,
		Theta: p.theta, Shade: p.shade, Corner: p.corner, Cut: p.cut, Detail: p.detail,
	}
}

func entityToPart(p ShipPartV0) shipPart {
	return shipPart{
		kind: p.Kind, dx: p.DX, dy: p.DY, hw: p.HW, hh: p.HH,
		theta: p.Theta, shade: p.Shade, corner: p.Corner, cut: p.Cut, detail: p.Detail,
	}
}

// shipToEntity converts an internal resolved ship into its frozen entity schema. The
// conversion is lossless: every field RenderList reads is carried.
func shipToEntity(s ship) Entity {
	out := &SpaceshipV0{
		CX:       s.cx,
		CY:       s.cy,
		Hull:     s.hull,
		Ambient:  s.ambient,
		Parts:    make([]ShipPartV0, len(s.parts)),
		Plumes:   make([]ShipPlumeV0, len(s.plumes)),
		Nozzles:  make([]ShipNozzleV0, len(s.nozzles)),
		Greebles: make([]ShipPartV0, len(s.greebles)),
	}
	for i, p := range s.parts {
		out.Parts[i] = partToEntity(p)
	}
	for i, g := range s.greebles {
		out.Greebles[i] = partToEntity(g)
	}
	for i, pl := range s.plumes {
		out.Plumes[i] = ShipPlumeV0{
			OX: pl.ox, OY: pl.oy, DirX: pl.dirX, DirY: pl.dirY,
			Len: pl.length, HalfWidth: pl.halfWidth, Col: pl.col,
		}
	}
	for i, nz := range s.nozzles {
		out.Nozzles[i] = ShipNozzleV0{
			NX: nz.nx, NY: nz.ny, AX: nz.ax, AY: nz.ay,
			Len: nz.length, NarrowHalf: nz.narrowHalf, WideHalf: nz.wideHalf, Shade: nz.shade,
		}
	}
	return out
}

// entityToShip reconstructs the internal ship from an entity, the inverse of shipToEntity.
// It errors if e is not a spaceship entity.
func entityToShip(e Entity) (ship, error) {
	v, ok := e.(*SpaceshipV0)
	if !ok {
		return ship{}, fmt.Errorf("scene: entity %T is not a spaceship", e)
	}
	s := ship{cx: v.CX, cy: v.CY, hull: v.Hull, ambient: v.Ambient}
	s.parts = make([]shipPart, len(v.Parts))
	for i, p := range v.Parts {
		s.parts[i] = entityToPart(p)
	}
	s.greebles = make([]shipPart, len(v.Greebles))
	for i, g := range v.Greebles {
		s.greebles[i] = entityToPart(g)
	}
	s.plumes = make([]shipPlume, len(v.Plumes))
	for i, pl := range v.Plumes {
		s.plumes[i] = shipPlume{
			ox: pl.OX, oy: pl.OY, dirX: pl.DirX, dirY: pl.DirY,
			length: pl.Len, halfWidth: pl.HalfWidth, col: pl.Col,
		}
	}
	s.nozzles = make([]shipNozzle, len(v.Nozzles))
	for i, nz := range v.Nozzles {
		s.nozzles[i] = shipNozzle{
			nx: nz.NX, ny: nz.NY, ax: nz.AX, ay: nz.AY,
			length: nz.Len, narrowHalf: nz.NarrowHalf, wideHalf: nz.WideHalf, shade: nz.Shade,
		}
	}
	return s, nil
}
