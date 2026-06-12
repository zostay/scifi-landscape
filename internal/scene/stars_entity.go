package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Star entity schema. Each resolved star in the field is one entity instance:
// the generator does all the random draws (position, color, brightness, shape)
// and records them here, and the renderer draws purely from these values. The
// schema is FROZEN: add fields if needed, but never rename, retype, or repurpose
// an existing one — make a V1 instead. The yaml keys are the on-disk contract and
// are pinned with explicit tags so Go field renames cannot change the serialized
// form.
const SchemaStarV0 = "star.v0"

func init() {
	RegisterEntity(SchemaStarV0, func() Entity { return &StarV0{} })
}

// StarV0 is one resolved star ready to draw: where it sits, its color, its
// time-of-day-faded brightness, and its shape (single pixel, tiny disc, or a disc
// with twinkle spikes). The shared twinkle direction is not stored here — it is
// derived by the renderer from the scene's global twinkle angle, the same as in
// the pre-split code.
type StarV0 struct {
	X      int     `yaml:"x"`      // pixel x
	Y      int     `yaml:"y"`      // pixel y
	Col    gfx.RGB `yaml:"col"`    // stellar tint
	Alpha  float64 `yaml:"alpha"`  // brightness * time-of-day fade
	Radius int     `yaml:"radius"` // 0 = single pixel
	Spikes bool    `yaml:"spikes"` // draw twinkle cross
	Spike  int     `yaml:"spike"`  // spike length in pixels (when spikes)
}

func (*StarV0) EntitySchema() string { return SchemaStarV0 }

// starToEntity converts an internal resolved star into its frozen entity schema.
// The conversion is lossless: every field the renderer reads is carried, so
// rendering from the entity reproduces the star exactly.
func starToEntity(s star) Entity {
	return &StarV0{
		X:      s.x,
		Y:      s.y,
		Col:    s.col,
		Alpha:  s.alpha,
		Radius: s.radius,
		Spikes: s.spikes,
		Spike:  s.spike,
	}
}

// entityToStar reconstructs the internal star from a star entity, the inverse of
// starToEntity. It errors if e is not a star entity.
func entityToStar(e Entity) (star, error) {
	v, ok := e.(*StarV0)
	if !ok {
		return star{}, fmt.Errorf("scene: entity %T is not a star", e)
	}
	return star{
		x:      v.X,
		y:      v.Y,
		col:    v.Col,
		alpha:  v.Alpha,
		radius: v.Radius,
		spikes: v.Spikes,
		spike:  v.Spike,
	}, nil
}
