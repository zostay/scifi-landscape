package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Extra-mountain-range entity schema. The receding ridgelines below the horizon
// resolve to a single entity holding an ordered (far→near) list of ranges; each
// range carries the same data a horizon range does (per-column heightmap, base→peak
// gradient, surface texture seed, altitude normalization) plus the foot row it
// stands on. This schema is FROZEN: add fields if needed, but never rename, retype,
// or repurpose an existing one — make a V1 instead. The yaml keys are the on-disk
// contract and are pinned with explicit tags.
const SchemaMountainRangesV0 = "mountainranges.v0"

func init() {
	RegisterEntity(SchemaMountainRangesV0, func() Entity { return &MountainRangesV0{} })
}

// MountainRangesV0 is the resolved set of extra mountain ranges, drawn back-to-front
// (the slice is ordered far→near, so later ranges occlude earlier ones). WaterColor is
// the scene's ocean tint (zero when there is no ocean), used to tint each range's
// reflection where its foot meets water.
type MountainRangesV0 struct {
	Ranges     []MountainRangeBandV0 `yaml:"ranges"`
	WaterColor gfx.RGB               `yaml:"waterColor,omitempty"`
}

func (*MountainRangesV0) EntitySchema() string { return SchemaMountainRangesV0 }

// MountainRangeBandV0 is one resolved range: everything RenderList needs to draw the
// ridge with no further randomness. Baseline is the foot row it rises from (a row
// below the horizon); Heights is the per-column ridge height in pixels; Gradient maps
// absolute altitude (0 at the foot, 1 at MaxAlt) to color; TexSeed seeds the per-pixel
// value mottle; MaxAlt is the altitude (pixels) the gradient's top is normalized to.
type MountainRangeBandV0 struct {
	Baseline int          `yaml:"baseline"`
	Heights  []float64    `yaml:"heights"`
	Gradient gfx.Gradient `yaml:"gradient"`
	TexSeed  int          `yaml:"texSeed"`
	MaxAlt   float64      `yaml:"maxAlt"`
	// Bulges is the per-column foot-bulge depth in pixels (the "negative contour" below
	// the baseline), already clipped at the shoreline so the foot never swells into
	// nearer water. Baking it here (rather than recomputing it from the ocean at render
	// time) keeps RenderList seed-independent. One entry per scene column.
	Bulges []float64 `yaml:"bulges"`
	// Shore is the per-column waterline row where this range's foot meets water (0 = no
	// water in front of the foot, so no reflection there). The renderer mirrors the
	// range down across this row into the water to draw its reflection. Baked here for
	// the same seed-independence reason as Bulges.
	Shore []int `yaml:"shore,omitempty"`
}

// mountainRangeBand is the internal resolved range produced by Generate and consumed
// by RenderList. It mirrors MountainRangeBandV0 one-for-one.
type mountainRangeBand struct {
	baseline int
	heights  []float64
	grad     gfx.Gradient
	texSeed  int
	maxAlt   float64
	bulges   []float64
	shore    []int
}

// mountainRangesToEntity converts the internal resolved ranges (and the scene water
// color) into their frozen entity schema. The conversion is lossless: every field
// RenderList reads is carried.
func mountainRangesToEntity(bands []mountainRangeBand, water gfx.RGB) Entity {
	out := &MountainRangesV0{Ranges: make([]MountainRangeBandV0, len(bands)), WaterColor: water}
	for i, b := range bands {
		out.Ranges[i] = MountainRangeBandV0{
			Baseline: b.baseline,
			Heights:  b.heights,
			Gradient: b.grad,
			TexSeed:  b.texSeed,
			MaxAlt:   b.maxAlt,
			Bulges:   b.bulges,
			Shore:    b.shore,
		}
	}
	return out
}

// entityToMountainRanges reconstructs the internal ranges and the scene water color
// from an entity, the inverse of mountainRangesToEntity. It errors if e is not a
// mountain-ranges entity.
func entityToMountainRanges(e Entity) ([]mountainRangeBand, gfx.RGB, error) {
	v, ok := e.(*MountainRangesV0)
	if !ok {
		return nil, gfx.RGB{}, fmt.Errorf("scene: entity %T is not extra mountain ranges", e)
	}
	bands := make([]mountainRangeBand, len(v.Ranges))
	for i, b := range v.Ranges {
		bands[i] = mountainRangeBand{
			baseline: b.Baseline,
			heights:  b.Heights,
			grad:     b.Gradient,
			texSeed:  b.TexSeed,
			maxAlt:   b.MaxAlt,
			bulges:   b.Bulges,
			shore:    b.Shore,
		}
	}
	return bands, v.WaterColor, nil
}
