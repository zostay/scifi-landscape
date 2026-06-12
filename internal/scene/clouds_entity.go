package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Cloud entity schemas. The two cloud layers render very differently — the high
// gauzy sheet is one whole-sky pass while the low nimbus clouds are discrete,
// individually-placed bodies — so each is its own entity (per the design). A
// scene carries at most one high-layer entity (when the high layer rolls in) and
// one low-cloud entity per resolved nimbus cloud, in draw order. These schemas
// are FROZEN: add fields if needed, but never rename, retype, or repurpose an
// existing one — make a V1 instead. The yaml keys are the on-disk contract and
// are pinned with explicit tags so Go field renames cannot change the serialized
// form.
const (
	SchemaCloudsHighV0 = "clouds.high.v0"
	SchemaCloudLowV0   = "clouds.low.v0"
)

func init() {
	RegisterEntity(SchemaCloudsHighV0, func() Entity { return &CloudsHighV0{} })
	RegisterEntity(SchemaCloudLowV0, func() Entity { return &CloudLowV0{} })
}

// CloudsHighV0 is the resolved high gauzy sheet: a single whole-sky pass of
// fractal noise, thresholded to sparse coverage and tinted for the time of day.
// All of its randomness is baked into these four fields, so the renderer redraws
// it deterministically.
type CloudsHighV0 struct {
	Seed     int     `yaml:"seed"`     // FBM noise seed
	Thresh   float64 `yaml:"thresh"`   // coverage threshold (noise below this is clear sky)
	MaxAlpha float64 `yaml:"maxAlpha"` // peak opacity of the layer
	Col      gfx.HSV `yaml:"col"`      // sheet color for the time of day
}

func (*CloudsHighV0) EntitySchema() string { return SchemaCloudsHighV0 }

// CloudLowV0 is one resolved nimbus cloud: a flat-based procedural height field
// over a bounding box, shaded between a lit and a shadow color. Position, size,
// bounding box, colors, and the noise seed are all baked here; the renderer
// evaluates the height field and lighting from them with no further randomness.
type CloudLowV0 struct {
	CX     float64 `yaml:"cx"`     // horizontal center (pixels)
	BaseY  float64 `yaml:"baseY"`  // flat-bottom row (pixels)
	CW     float64 `yaml:"cw"`     // width (pixels)
	CH     float64 `yaml:"ch"`     // height (pixels)
	MinX   int     `yaml:"minX"`   // bounding box left
	MaxX   int     `yaml:"maxX"`   // bounding box right
	MinY   int     `yaml:"minY"`   // bounding box top (rows run minY..baseY)
	Lit    gfx.HSV `yaml:"lit"`    // lit-top color
	Shadow gfx.HSV `yaml:"shadow"` // shadowed-underside color
	Seed   int     `yaml:"seed"`   // Worley + Perlin noise seed
}

func (*CloudLowV0) EntitySchema() string { return SchemaCloudLowV0 }

// highToEntity converts a resolved high-cloud sheet into its frozen entity
// schema. The conversion is lossless: every field the renderer reads is carried.
func highToEntity(h highClouds) Entity {
	return &CloudsHighV0{Seed: h.seed, Thresh: h.thresh, MaxAlpha: h.maxAlpha, Col: h.col}
}

// entityToHigh reconstructs the internal high-cloud sheet from its entity, the
// inverse of highToEntity.
func entityToHigh(e *CloudsHighV0) highClouds {
	return highClouds{seed: e.Seed, thresh: e.Thresh, maxAlpha: e.MaxAlpha, col: e.Col}
}

// cloudToEntity converts an internal resolved nimbus cloud into its frozen entity
// schema. The conversion is lossless.
func cloudToEntity(c cloud) Entity {
	return &CloudLowV0{
		CX: c.cx, BaseY: c.baseY, CW: c.cw, CH: c.ch,
		MinX: c.minX, MaxX: c.maxX, MinY: c.minY,
		Lit: c.lit, Shadow: c.shadow, Seed: c.seed,
	}
}

// entityToCloud reconstructs the internal nimbus cloud from its entity, the
// inverse of cloudToEntity.
func entityToCloud(e *CloudLowV0) cloud {
	return cloud{
		cx: e.CX, baseY: e.BaseY, cw: e.CW, ch: e.CH,
		minX: e.MinX, maxX: e.MaxX, minY: e.MinY,
		lit: e.Lit, shadow: e.Shadow, seed: e.Seed,
	}
}

// entityToCloudsErr is a small typed-dispatch helper used by RenderList to reject
// non-cloud entities clearly.
func errNotCloud(e Entity) error {
	return fmt.Errorf("scene: entity %T is not a cloud", e)
}
