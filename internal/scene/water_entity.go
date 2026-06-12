package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Water entity schema. The ocean resolves to a single entity holding the
// already-decided ocean/land model the renderer needs: its color and sand, the
// wave and land noise seeds, the sea level and coastal bias, and the horizon/
// ground geometry. The ocean itself is a shared global built up front in
// Scene.Build (Context.Ocean, via buildOcean on the "water" stream) so that
// Cities — drawn before Water — can keep to land while Water still reflects the
// city skyline. Water.Generate therefore draws NO randomness of its own: it
// simply captures the resolved global into this entity for the renderer to read.
// A scene with no ocean produces no Water entity at all.
//
// This schema is FROZEN: add fields if needed, but never rename, retype, or
// repurpose an existing one — make a V1 instead. The yaml keys are the on-disk
// contract and are pinned with explicit tags so Go field renames cannot change
// the serialized form.
const SchemaWaterV0 = "water.v0"

func init() {
	RegisterEntity(SchemaWaterV0, func() Entity { return &WaterV0{} })
}

// WaterV0 is the resolved ocean: everything RenderList needs to mirror the scene
// above the horizon into a rippled, island-dotted sea with no further randomness.
// It carries the full ocean/land model (the shared global decided in Scene.Build)
// so the renderer can recompute land elevation, waves, beaches, and surf exactly
// as before. An entity exists only when the scene actually has an ocean (the
// model's present flag), so there is no present field here.
type WaterV0 struct {
	Horizon  int     `yaml:"horizon"`  // horizon row (pixels)
	GroundH  int     `yaml:"groundH"`  // foreground height below the horizon (pixels)
	Color    gfx.RGB `yaml:"color"`    // water tint color
	Sand     gfx.RGB `yaml:"sand"`     // beach/shore color
	WaveSeed int     `yaml:"waveSeed"` // ripple-displacement noise seed
	LandSeed int     `yaml:"landSeed"` // island/coast elevation noise seed
	SeaLevel float64 `yaml:"seaLevel"` // elevation above which land shows through
	Coast    float64 `yaml:"coast"`    // horizon-ward land bias (0 = open ocean)
}

func (*WaterV0) EntitySchema() string { return SchemaWaterV0 }

// oceanToEntity converts the internal resolved ocean into its frozen entity
// schema. The conversion is lossless: every field RenderList reads is carried, so
// rendering from the entity reproduces the ocean exactly. The present flag is not
// carried because an entity is emitted only for a present ocean.
func oceanToEntity(o *ocean) Entity {
	return &WaterV0{
		Horizon:  o.horizon,
		GroundH:  o.groundH,
		Color:    o.color,
		Sand:     o.sand,
		WaveSeed: o.waveSeed,
		LandSeed: o.landSeed,
		SeaLevel: o.seaLevel,
		Coast:    o.coast,
	}
}

// entityToOcean reconstructs the internal ocean from a water entity, the inverse
// of oceanToEntity. The reconstructed ocean is always present (entities are
// emitted only for present oceans). It errors if e is not a water entity.
func entityToOcean(e Entity) (*ocean, error) {
	v, ok := e.(*WaterV0)
	if !ok {
		return nil, fmt.Errorf("scene: entity %T is not water", e)
	}
	return &ocean{
		present:  true,
		horizon:  v.Horizon,
		groundH:  v.GroundH,
		color:    v.Color,
		sand:     v.Sand,
		waveSeed: v.WaveSeed,
		landSeed: v.LandSeed,
		seaLevel: v.SeaLevel,
		coast:    v.Coast,
	}, nil
}
