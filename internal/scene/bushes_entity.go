package scene

import "fmt"

// Bushes entity schema. The scattered ground bushes resolve to a single entity holding
// an ordered (far→near) list of bushes; each bush carries its ground-contact anchor,
// its squashed-ellipse axes and rotation, how far it is buried, the position it sampled
// along the scene's bush gradient for its base color, and the seeds for its lopsided
// outline and texture. The base color is NOT stored directly: the renderer samples the
// scene's bush gradient (a global) at the bush's ColorPos, and the form-shading style
// follows the scene's mountain shading — both read from the render context.
//
// This schema is FROZEN: add fields if needed, but never rename, retype, or repurpose
// an existing one — make a V1 instead. The yaml keys are the on-disk contract and are
// pinned with explicit tags.
const SchemaBushesV0 = "bushes.v0"

func init() {
	RegisterEntity(SchemaBushesV0, func() Entity { return &BushesV0{} })
}

// BushesV0 is the resolved set of ground bushes, ordered far→near so a nearer bush
// occludes the ones behind it when drawn in order. Ambient is the shadow-side fill
// light for the form-shading (snapshotted from the globals so the render is
// self-contained); Lumpiness is how strongly each outline is perturbed from a plain
// ellipse.
type BushesV0 struct {
	Bushes    []BushV0 `yaml:"bushes"`
	Ambient   float64  `yaml:"ambient"`
	Lumpiness float64  `yaml:"lumpiness"`
}

func (*BushesV0) EntitySchema() string { return SchemaBushesV0 }

// BushV0 is one resolved bush: everything RenderList needs to draw it with no further
// randomness. X,Y is the ground-contact anchor (the buried base sits here); A and B are
// the semi-major and semi-minor axes in pixels (A ≥ B — a squashed clump); Theta is the
// outline's rotation in radians; Bury is the fraction of the bush's height below the
// contact line; ColorPos is the position in [0,1] this bush sampled along the scene's
// bush gradient for its base color; ShapeSeed seeds the lopsided outline perturbation;
// TexSeed seeds the foliage mottle and speckle.
type BushV0 struct {
	X         int     `yaml:"x"`
	Y         int     `yaml:"y"`
	A         float64 `yaml:"a"`
	B         float64 `yaml:"b"`
	Theta     float64 `yaml:"theta"`
	Bury      float64 `yaml:"bury"`
	ColorPos  float64 `yaml:"colorPos"`
	ShapeSeed int     `yaml:"shapeSeed"`
	TexSeed   int     `yaml:"texSeed"`
}

// bush is the internal resolved bush produced by Generate and consumed by RenderList.
// It mirrors BushV0 one-for-one.
type bush struct {
	x, y      int
	a, b      float64
	theta     float64
	bury      float64
	colorPos  float64
	shapeSeed int
	texSeed   int
}

// bushesScene is the scene-level (non-per-bush) data carried with the bushes.
type bushesScene struct {
	ambient   float64
	lumpiness float64
}

// bushesToEntity converts the internal resolved bushes and scene-level data into their
// frozen entity schema. The conversion is lossless: every field RenderList reads is
// carried.
func bushesToEntity(bushes []bush, sc bushesScene) Entity {
	out := &BushesV0{
		Bushes:    make([]BushV0, len(bushes)),
		Ambient:   sc.ambient,
		Lumpiness: sc.lumpiness,
	}
	for i, b := range bushes {
		out.Bushes[i] = BushV0{
			X:         b.x,
			Y:         b.y,
			A:         b.a,
			B:         b.b,
			Theta:     b.theta,
			Bury:      b.bury,
			ColorPos:  b.colorPos,
			ShapeSeed: b.shapeSeed,
			TexSeed:   b.texSeed,
		}
	}
	return out
}

// entityToBushes reconstructs the internal bushes and scene-level data from an entity,
// the inverse of bushesToEntity. It errors if e is not a bushes entity.
func entityToBushes(e Entity) ([]bush, bushesScene, error) {
	v, ok := e.(*BushesV0)
	if !ok {
		return nil, bushesScene{}, fmt.Errorf("scene: entity %T is not bushes", e)
	}
	bushes := make([]bush, len(v.Bushes))
	for i, b := range v.Bushes {
		bushes[i] = bush{
			x:         b.X,
			y:         b.Y,
			a:         b.A,
			b:         b.B,
			theta:     b.Theta,
			bury:      b.Bury,
			colorPos:  b.ColorPos,
			shapeSeed: b.ShapeSeed,
			texSeed:   b.TexSeed,
		}
	}
	return bushes, bushesScene{ambient: v.Ambient, lumpiness: v.Lumpiness}, nil
}
