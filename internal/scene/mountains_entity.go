package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Mountain entity schema. The horizon range resolves to a single entity holding
// the per-column ridge heightmap, the base->peak color gradient, the surface
// texture seed, and the altitude normalization used to shade by absolute height.
// This schema is FROZEN: add fields if needed, but never rename, retype, or
// repurpose an existing one — make a V1 instead. The yaml keys are the on-disk
// contract and are pinned with explicit tags so Go field renames cannot change
// the serialized form.
const SchemaMountainsV0 = "mountains.v0"

func init() {
	RegisterEntity(SchemaMountainsV0, func() Entity { return &MountainsV0{} })
}

// MountainsV0 is the resolved horizon mountain range: everything RenderList needs
// to draw the ridge with no further randomness. Heights is the per-column ridge
// height in pixels (one entry per scene column); Gradient maps absolute altitude
// (0 at the horizon, 1 at MaxAlt) to color; TexSeed seeds the per-pixel value
// mottle; MaxAlt is the altitude (pixels) the gradient's top is normalized to.
type MountainsV0 struct {
	Heights  []float64    `yaml:"heights"`
	Gradient gfx.Gradient `yaml:"gradient"`
	TexSeed  int          `yaml:"texSeed"`
	MaxAlt   float64      `yaml:"maxAlt"`
}

func (*MountainsV0) EntitySchema() string { return SchemaMountainsV0 }

// mountainRange is the internal resolved range produced by Generate and consumed
// by RenderList. It mirrors MountainsV0 one-for-one.
type mountainRange struct {
	heights []float64
	grad    gfx.Gradient
	texSeed int
	maxAlt  float64
}

// mountainsToEntity converts the internal resolved range into its frozen entity
// schema. The conversion is lossless: every field RenderList reads is carried, so
// rendering from the entity reproduces the range exactly.
func mountainsToEntity(m mountainRange) Entity {
	return &MountainsV0{
		Heights:  m.heights,
		Gradient: m.grad,
		TexSeed:  m.texSeed,
		MaxAlt:   m.maxAlt,
	}
}

// entityToMountains reconstructs the internal range from a mountains entity, the
// inverse of mountainsToEntity. It errors if e is not a mountains entity.
func entityToMountains(e Entity) (mountainRange, error) {
	v, ok := e.(*MountainsV0)
	if !ok {
		return mountainRange{}, fmt.Errorf("scene: entity %T is not a mountain range", e)
	}
	return mountainRange{
		heights: v.Heights,
		grad:    v.Gradient,
		texSeed: v.TexSeed,
		maxAlt:  v.MaxAlt,
	}, nil
}
