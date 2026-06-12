package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// System-star entity schema. Each resolved sun (the system's local star(s)) is
// one entity instance: the generator does all the random draws (count, size,
// color, position, brightness, twinkle) and records them here, and the renderer
// draws purely from these values. The schema is FROZEN: add fields if needed,
// but never rename, retype, or repurpose an existing one — make a V1 instead.
// The yaml keys are the on-disk contract and are pinned with explicit tags so Go
// field renames cannot change the serialized form.
const SchemaSystemStarV0 = "systemstar.v0"

func init() {
	RegisterEntity(SchemaSystemStarV0, func() Entity { return &SystemStarV0{} })
}

// SystemStarV0 is one resolved system sun ready to draw: where it sits, its disc
// radius and base (edge) color, whether it grows a twinkle cross, and a
// brightness scale (<1 for dim night suns). The shared twinkle direction is not
// stored here — it is derived by the renderer from the scene's global twinkle
// angle, the same as in the pre-split code.
type SystemStarV0 struct {
	CX     int     `yaml:"cx"`     // disc center x (pixels)
	CY     int     `yaml:"cy"`     // disc center y (pixels)
	R      int     `yaml:"r"`      // disc radius (pixels)
	Col    gfx.RGB `yaml:"col"`    // base (edge) color; the core is whiter
	Plus   bool    `yaml:"plus"`   // draw a twinkle cross
	Bright float64 `yaml:"bright"` // scales glow and disc; <1 for dim night suns
}

func (*SystemStarV0) EntitySchema() string { return SchemaSystemStarV0 }

// sunToEntity converts an internal resolved sun into its frozen entity schema.
// The conversion is lossless: every field the renderer reads is carried, so
// rendering from the entity reproduces the sun exactly.
func sunToEntity(s sun) Entity {
	return &SystemStarV0{
		CX:     s.cx,
		CY:     s.cy,
		R:      s.r,
		Col:    s.col,
		Plus:   s.plus,
		Bright: s.bright,
	}
}

// entityToSun reconstructs the internal sun from a system-star entity, the
// inverse of sunToEntity. It errors if e is not a system-star entity.
func entityToSun(e Entity) (sun, error) {
	v, ok := e.(*SystemStarV0)
	if !ok {
		return sun{}, fmt.Errorf("scene: entity %T is not a system star", e)
	}
	return sun{
		cx:     v.CX,
		cy:     v.CY,
		r:      v.R,
		col:    v.Col,
		plus:   v.Plus,
		bright: v.Bright,
	}, nil
}
