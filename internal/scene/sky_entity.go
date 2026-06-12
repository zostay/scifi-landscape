package scene

import "fmt"

// Sky entity schema. The sky is the bottom-most element: a vertical gradient
// that fills the whole scene. Its color gradient (Context.SkyGradient) is a
// shared global built once in Scene.Build and is NOT carried here — RenderList
// reads it from the Context exactly as today, so it stays a scene-wide global
// rather than a per-element decision. The sky element itself draws no randomness
// of its own (every color comes from the shared gradient), so the resolved
// element state is empty: a single marker entity recording that the scene has a
// sky to draw.
//
// This schema is FROZEN: add fields if needed, but never rename, retype, or
// repurpose an existing one — make a V1 instead. The yaml keys are the on-disk
// contract and are pinned with explicit tags so Go field renames cannot change
// the serialized form.
const SchemaSkyV0 = "sky.v0"

func init() {
	RegisterEntity(SchemaSkyV0, func() Entity { return &SkyV0{} })
}

// SkyV0 is the resolved sky: a marker entity. The sky element consumes no
// randomness of its own — its colors are read entirely from the shared
// Context.SkyGradient global at render time — so there is no per-element state to
// carry. The entity records only that a sky is present so the scene list and the
// renderer agree on what to draw.
type SkyV0 struct{}

func (*SkyV0) EntitySchema() string { return SchemaSkyV0 }

// skyMarker is the internal resolved sky produced by Generate and consumed by
// RenderList. It mirrors SkyV0: empty, since the sky carries no element-generated
// state (the gradient is a shared global on the Context).
type skyMarker struct{}

// skyToEntity converts the internal resolved sky into its frozen entity schema.
// The conversion is lossless: the sky carries no element state, so the entity is
// a bare marker and rendering from it reproduces the sky exactly (from the shared
// Context.SkyGradient).
func skyToEntity(skyMarker) Entity {
	return &SkyV0{}
}

// entityToSky reconstructs the internal sky from a sky entity, the inverse of
// skyToEntity. It errors if e is not a sky entity.
func entityToSky(e Entity) (skyMarker, error) {
	if _, ok := e.(*SkyV0); !ok {
		return skyMarker{}, fmt.Errorf("scene: entity %T is not sky", e)
	}
	return skyMarker{}, nil
}
