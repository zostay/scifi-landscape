package scene

import "fmt"

// Ground entity schema. The base terrain resolves to a single entity holding the
// two noise seeds drawn from the element stream: the surface texture seed and (in
// variable color mode) the gradient-wander seed. The ground color gradient itself
// is a scene-wide global (Globals.GroundGradient / Globals.GroundVariable,
// derived by the director) and is NOT carried here — RenderList reads it from the
// Context, so it stays a scene-wide global rather than a per-element decision.
//
// This schema is FROZEN: add fields if needed, but never rename, retype, or
// repurpose an existing one — make a V1 instead. The yaml keys are the on-disk
// contract and are pinned with explicit tags so Go field renames cannot change
// the serialized form.
const SchemaGroundV0 = "ground.v0"

func init() {
	RegisterEntity(SchemaGroundV0, func() Entity { return &GroundV0{} })
}

// GroundV0 is the resolved base terrain: everything RenderList needs from the
// element's random stream to draw the dirt with no further randomness. Seed seeds
// the per-pixel fractal texture. Variable records whether the scene rolled the
// multi-color "variable" ground mode (mirrors Context.GroundVariable, the shared
// global); when true WanderSeed seeds the low-frequency gradient-lookup wander.
// WanderSeed is meaningful only when Variable is true (it is not drawn otherwise).
type GroundV0 struct {
	Seed       int  `yaml:"seed"`
	Variable   bool `yaml:"variable"`
	WanderSeed int  `yaml:"wanderSeed"`
}

func (*GroundV0) EntitySchema() string { return SchemaGroundV0 }

// groundTerrain is the internal resolved terrain produced by Generate and
// consumed by RenderList. It mirrors GroundV0 one-for-one.
type groundTerrain struct {
	seed       int
	variable   bool
	wanderSeed int
}

// groundToEntity converts the internal resolved terrain into its frozen entity
// schema. The conversion is lossless: every field RenderList reads is carried, so
// rendering from the entity reproduces the terrain exactly.
func groundToEntity(g groundTerrain) Entity {
	return &GroundV0{
		Seed:       g.seed,
		Variable:   g.variable,
		WanderSeed: g.wanderSeed,
	}
}

// entityToGround reconstructs the internal terrain from a ground entity, the
// inverse of groundToEntity. It errors if e is not a ground entity.
func entityToGround(e Entity) (groundTerrain, error) {
	v, ok := e.(*GroundV0)
	if !ok {
		return groundTerrain{}, fmt.Errorf("scene: entity %T is not ground", e)
	}
	return groundTerrain{
		seed:       v.Seed,
		variable:   v.Variable,
		wanderSeed: v.WanderSeed,
	}, nil
}
