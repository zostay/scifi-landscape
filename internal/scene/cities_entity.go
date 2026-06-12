package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// City entity schema. A city is a single resolved entity carrying every building
// rectangle (already sorted back-to-front for drawing) and any geodesic domes
// planned over it. All the random draws that shape the city — footprint, palette,
// per-building geometry, and dome placement — are baked into this data, so the
// renderer reproduces the city exactly without consuming any randomness.
//
// This schema is FROZEN: add fields if needed, but never rename, retype, or
// repurpose an existing one — make a V1 instead. The yaml keys are the on-disk
// contract and are pinned with explicit tags.
const SchemaCityV0 = "city.v0"

func init() {
	RegisterEntity(SchemaCityV0, func() Entity { return &CityV0{} })
}

// CityV0 is one resolved city: its buildings (drawn back-to-front, in order) and
// any domes that cap it (drawn after the buildings).
type CityV0 struct {
	Buildings []BuildingV0 `yaml:"buildings"`
	Domes     []DomeV0     `yaml:"domes,omitempty"`
}

func (*CityV0) EntitySchema() string { return SchemaCityV0 }

// BuildingV0 is one resolved building rectangle. Base is the ground-contact row;
// the building rises H pixels up from there and is W wide, anchored at X.
type BuildingV0 struct {
	X    int     `yaml:"x"`    // left edge (pixels)
	Base int     `yaml:"base"` // ground-contact row (pixels)
	W    int     `yaml:"w"`    // width (pixels)
	H    int     `yaml:"h"`    // height (pixels)
	Col  gfx.RGB `yaml:"col"`  // resolved (hazed) color
}

// DomeV0 is one geodesic glass dome over part of a city: it sits flat on the
// ground at (CX, BaseRow) and bulges up with radius R.
type DomeV0 struct {
	CX      int `yaml:"cx"`      // center x (pixels)
	BaseRow int `yaml:"baseRow"` // ground-contact row (pixels)
	R       int `yaml:"r"`       // dome radius (pixels)
}

// cityToEntity converts the internal resolved buildings and domes into the frozen
// city entity. The conversion is lossless: every field the renderer reads is
// carried, so rendering from the entity reproduces the city exactly.
func cityToEntity(blds []building, domes []dome) *CityV0 {
	out := &CityV0{Buildings: make([]BuildingV0, len(blds))}
	for i, b := range blds {
		out.Buildings[i] = BuildingV0{X: b.x, Base: b.base, W: b.w, H: b.h, Col: b.col}
	}
	if len(domes) > 0 {
		out.Domes = make([]DomeV0, len(domes))
		for i, d := range domes {
			out.Domes[i] = DomeV0{CX: d.cx, BaseRow: d.baseRow, R: d.r}
		}
	}
	return out
}

// entityToCity reconstructs the internal buildings and domes from a city entity,
// the inverse of cityToEntity. It errors if e is not a city entity.
func entityToCity(e Entity) ([]building, []dome, error) {
	v, ok := e.(*CityV0)
	if !ok {
		return nil, nil, fmt.Errorf("scene: entity %T is not a city", e)
	}
	blds := make([]building, len(v.Buildings))
	for i, b := range v.Buildings {
		blds[i] = building{x: b.X, base: b.Base, w: b.W, h: b.H, col: b.Col}
	}
	var domes []dome
	if len(v.Domes) > 0 {
		domes = make([]dome, len(v.Domes))
		for i, d := range v.Domes {
			domes[i] = dome{cx: d.CX, baseRow: d.BaseRow, r: d.R}
		}
	}
	return blds, domes, nil
}
