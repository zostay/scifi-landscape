package scene

import (
	"fmt"

	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Planet entity schemas. Gas giants and moons render very differently, so each is
// its own entity (per the design); a ring system is carried as a nested schema on
// the planet body for now. These schemas are FROZEN: add fields if needed, but
// never rename, retype, or repurpose an existing one — make a V1 instead. The
// yaml keys are the on-disk contract and are pinned with explicit tags so Go
// field renames cannot change the serialized form.
const (
	SchemaPlanetGasGiantV0 = "planet.gasgiant.v0"
	SchemaPlanetMoonV0     = "planet.moon.v0"
)

func init() {
	RegisterEntity(SchemaPlanetGasGiantV0, func() Entity { return &PlanetGasGiantV0{} })
	RegisterEntity(SchemaPlanetMoonV0, func() Entity { return &PlanetMoonV0{} })
}

// PlanetBodyV0 is the spatial/orientation schema shared by every planet entity:
// where it sits, how big it is, its surface tilt and noise seed, and an optional
// equatorial ring system.
type PlanetBodyV0 struct {
	CX       int           `yaml:"cx"`       // disc center x (pixels)
	CY       int           `yaml:"cy"`       // disc center y (pixels)
	R        int           `yaml:"r"`        // disc radius (pixels)
	Rotation float64       `yaml:"rotation"` // surface tilt (radians)
	TurbSeed int           `yaml:"turbSeed"` // surface noise seed
	TurbAmp  float64       `yaml:"turbAmp"`  // surface noise amplitude
	Ring     *RingSystemV0 `yaml:"ring,omitempty"`
}

// PlanetGasGiantV0 is a gas giant: a shaded sphere of turbulent latitudinal color
// bands.
type PlanetGasGiantV0 struct {
	Body  PlanetBodyV0 `yaml:",inline"`
	Bands gfx.Gradient `yaml:"bands"` // latitude (0=top,1=bottom) -> color
}

func (*PlanetGasGiantV0) EntitySchema() string { return SchemaPlanetGasGiantV0 }

// PlanetMoonV0 is an airless, rocky world: a mottled base color with optional
// polar lightening, dark maria, recolored patches, and impact craters.
type PlanetMoonV0 struct {
	Body        PlanetBodyV0 `yaml:",inline"`
	Base        gfx.HSV      `yaml:"base"`      // base surface color
	PoleLight   float64      `yaml:"poleLight"` // polar lightening (0 = none)
	Craters     []CraterV0   `yaml:"craters,omitempty"`
	MariaThresh float64      `yaml:"mariaThresh"` // dark-patch noise threshold
	MariaDark   float64      `yaml:"mariaDark"`   // dark-patch darkening (0 = none)
	PatchThresh float64      `yaml:"patchThresh"` // recolor-patch noise threshold
	PatchBlend  float64      `yaml:"patchBlend"`  // recolor-patch blend (0 = none)
	PatchColor  gfx.HSV      `yaml:"patchColor"`  // recolor-patch color
}

func (*PlanetMoonV0) EntitySchema() string { return SchemaPlanetMoonV0 }

// CraterV0 is one impact crater: a circle on the sphere projected to a
// foreshortened ellipse on the disc.
type CraterV0 struct {
	CX     float64 `yaml:"cx"`     // center x, disc-normalized [-1,1]
	CY     float64 `yaml:"cy"`     // center y, disc-normalized [-1,1]
	Size   float64 `yaml:"size"`   // on-sphere (tangential) radius
	Radial float64 `yaml:"radial"` // foreshortened radial semi-axis
	RX     float64 `yaml:"rx"`     // unit radial direction x
	RY     float64 `yaml:"ry"`     // unit radial direction y
	Seed   int     `yaml:"seed"`   // jagged-edge noise seed
}

// RingBandV0 is one concentric band of a ring system, spanning a radius range in
// planet-radius units (1 = the rim).
type RingBandV0 struct {
	Lo      float64 `yaml:"lo"`
	Hi      float64 `yaml:"hi"`
	Col     gfx.HSV `yaml:"col"`
	Opacity float64 `yaml:"opacity"`
}

// RingSystemV0 is a planet's equatorial ring system, projected with an opening
// tilt so it reads as an ellipse around the body.
type RingSystemV0 struct {
	Bands   []RingBandV0 `yaml:"bands"`
	Inner   float64      `yaml:"inner"`
	Outer   float64      `yaml:"outer"`
	SinTilt float64      `yaml:"sinTilt"`
	CosTilt float64      `yaml:"cosTilt"`
	Seed    int          `yaml:"seed"`
}

// planetToEntity converts an internal resolved planet into its frozen entity
// schema. The conversion is lossless: every field the renderer reads is carried,
// so rendering from the entity reproduces the planet exactly.
func planetToEntity(p planet) Entity {
	body := PlanetBodyV0{
		CX: p.cx, CY: p.cy, R: p.r,
		Rotation: p.rotation, TurbSeed: p.turbSeed, TurbAmp: p.turbAmp,
		Ring: ringsToV0(p.ring),
	}
	if p.typ == GasGiant {
		return &PlanetGasGiantV0{Body: body, Bands: p.bands}
	}
	return &PlanetMoonV0{
		Body: body, Base: p.base, PoleLight: p.poleLight,
		Craters:     cratersToV0(p.craters),
		MariaThresh: p.mariaThresh, MariaDark: p.mariaDark,
		PatchThresh: p.patchThresh, PatchBlend: p.patchBlend, PatchColor: p.patchColor,
	}
}

// entityToPlanet reconstructs the internal planet from a planet entity, the
// inverse of planetToEntity. It errors if e is not a planet entity.
func entityToPlanet(e Entity) (planet, error) {
	switch v := e.(type) {
	case *PlanetGasGiantV0:
		p := planetFromBody(v.Body)
		p.typ = GasGiant
		p.bands = v.Bands
		return p, nil
	case *PlanetMoonV0:
		p := planetFromBody(v.Body)
		p.typ = Moon
		p.base = v.Base
		p.poleLight = v.PoleLight
		p.craters = cratersFromV0(v.Craters)
		p.mariaThresh, p.mariaDark = v.MariaThresh, v.MariaDark
		p.patchThresh, p.patchBlend, p.patchColor = v.PatchThresh, v.PatchBlend, v.PatchColor
		return p, nil
	default:
		return planet{}, fmt.Errorf("scene: entity %T is not a planet", e)
	}
}

func planetFromBody(b PlanetBodyV0) planet {
	return planet{
		cx: b.CX, cy: b.CY, r: b.R,
		rotation: b.Rotation, turbSeed: b.TurbSeed, turbAmp: b.TurbAmp,
		ring: ringsFromV0(b.Ring),
	}
}

func ringsToV0(r *rings) *RingSystemV0 {
	if r == nil {
		return nil
	}
	bands := make([]RingBandV0, len(r.bands))
	for i, b := range r.bands {
		bands[i] = RingBandV0{Lo: b.lo, Hi: b.hi, Col: b.col, Opacity: b.opacity}
	}
	return &RingSystemV0{
		Bands: bands, Inner: r.inner, Outer: r.outer,
		SinTilt: r.sinTilt, CosTilt: r.cosTilt, Seed: r.seed,
	}
}

func ringsFromV0(r *RingSystemV0) *rings {
	if r == nil {
		return nil
	}
	bands := make([]ringBand, len(r.Bands))
	for i, b := range r.Bands {
		bands[i] = ringBand{lo: b.Lo, hi: b.Hi, col: b.Col, opacity: b.Opacity}
	}
	return &rings{
		bands: bands, inner: r.Inner, outer: r.Outer,
		sinTilt: r.SinTilt, cosTilt: r.CosTilt, seed: r.Seed,
	}
}

func cratersToV0(cs []crater) []CraterV0 {
	if len(cs) == 0 {
		return nil
	}
	out := make([]CraterV0, len(cs))
	for i, c := range cs {
		out[i] = CraterV0{CX: c.cx, CY: c.cy, Size: c.size, Radial: c.radial, RX: c.rx, RY: c.ry, Seed: c.seed}
	}
	return out
}

func cratersFromV0(cs []CraterV0) []crater {
	if len(cs) == 0 {
		return nil
	}
	out := make([]crater, len(cs))
	for i, c := range cs {
		out[i] = crater{cx: c.CX, cy: c.CY, size: c.Size, radial: c.Radial, rx: c.RX, ry: c.RY, seed: c.Seed}
	}
	return out
}
