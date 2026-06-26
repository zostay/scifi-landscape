// Package config holds the tunable constants that shape a scene.
//
// Configuration is one of the two fundamental inputs to scene generation (the
// other is the seed). It captures the probabilities and limits that used to live
// as hardcoded constants in the scene code, so they can be recorded alongside a
// rendered image and adjusted for future scenes.
//
// A Config may be complete or partial. A partial config — say, a file on disk in
// which the user set only the values they care about — is merged over the
// built-in defaults by Load, so the system always works with a complete config.
// Whatever is written back out is always complete, so a recorded config
// reproduces its scene exactly.
//
// This package deliberately depends only on the standard library and the YAML
// codec: it is pure data with no dependency on the scene-generation algorithms,
// so it can be serialized, embedded in a PNG, and round-tripped without dragging
// the rendering stack along.
package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Config is the complete set of tunable constants for a scene. Sections mirror
// the const groups they replace in the scene code. As elements are migrated to
// be configuration-driven, new sections are appended here; existing sections and
// fields are never renamed, retyped, or given new meaning, so an old recorded
// config keeps reproducing its scene (see the versioning contract in the spec).
type Config struct {
	// Algorithms names the Directors, Generators, and Renderers used to build the
	// scene, in pipeline order, by their versioned registry keys.
	Algorithms Algorithms `yaml:"algorithms"`

	// Horizon, Twinkle, StarDensity, and Lighting parameterize how the scene-wide
	// globals are derived from the seed (see the Director). They correspond to the
	// matching const groups in the scene's settings code.
	Horizon     HorizonConfig  `yaml:"horizon"`
	Twinkle     TwinkleConfig  `yaml:"twinkle"`
	StarDensity DensityConfig  `yaml:"starDensity"`
	Lighting    LightingConfig `yaml:"lighting"`

	// Perspective parameterizes the scene-wide "height" vantage point: how often a
	// scene is rendered at ground level, and how strongly that widens the ground
	// plane. It is read by the scene.v1 director (to roll the height global) and by
	// the v1 ground/cities/water algorithms (to shape the low-mode look). Zero-value
	// callers (the v0 director) ignore it entirely.
	Perspective PerspectiveConfig `yaml:"perspective"`

	// Mountains parameterizes the extra mountain ranges (the mountainranges.v0
	// element) that recede below the horizon behind the city: how likely/how many
	// per vantage, how far down the ground they reach, and the base height/smoothness
	// each range varies around. It is read by the scene.v1 director (to resolve the
	// per-vantage base parameters into the globals). Zero-value callers (the v0
	// director) leave it empty, which means "no extra ranges".
	Mountains MountainConfig `yaml:"mountains"`

	// Mist parameterizes the ground mist that can settle among the extra mountain
	// ranges (drawn by the mountainranges.v0 element). It is read by the scene.v1
	// director to roll the per-scene mist on or off and resolve its shape. Zero-value
	// callers (the v0 director) leave it empty, which means "no mist".
	Mist MistConfig `yaml:"mist"`
}

// Algorithms lists the versioned registry keys of the algorithms that build a
// scene. Directors turn seed+config into globals; Generators turn globals into
// the scene list of entities; Renderers draw the scene list. The lists are in
// pipeline order.
type Algorithms struct {
	Directors  []string `yaml:"directors"`
	Generators []string `yaml:"generators"`
	Renderers  []string `yaml:"renderers"`
}

// HorizonConfig bounds the horizon height as a fraction of scene height measured
// from the bottom (the ground's share). The value is drawn from a normal
// distribution (Mean, Std) clamped to [Min, Max].
type HorizonConfig struct {
	Min  float64 `yaml:"min"`
	Max  float64 `yaml:"max"`
	Mean float64 `yaml:"mean"`
	Std  float64 `yaml:"std"`
}

// TwinkleConfig parameterizes the shared star-twinkle/light angle, in degrees,
// drawn as |normal|*Std biased toward 0 and clamped to Max.
type TwinkleConfig struct {
	Max float64 `yaml:"max"`
	Std float64 `yaml:"std"`
}

// DensityConfig parameterizes the log-normal star-density multiplier: a normal
// shifted by Bias and clamped to ±Clamp in log space, then scaled by Std.
type DensityConfig struct {
	Std   float64 `yaml:"std"`
	Bias  float64 `yaml:"bias"`
	Clamp float64 `yaml:"clamp"`
}

// LightingConfig parameterizes the dominant-star lighting applied to planets.
// The light color is a near-white tint: a random hue at full Value and a
// saturation in [ColorSatMin, ColorSatMax]. Brightness (terminator harshness) is
// drawn from [BrightMin, BrightMax]. Ambient fill is AmbientBase +
// AmbientScale*rng^AmbientPow, biased low so shadows usually fall dark.
type LightingConfig struct {
	ColorSatMin  float64 `yaml:"colorSatMin"`
	ColorSatMax  float64 `yaml:"colorSatMax"`
	ColorValue   float64 `yaml:"colorValue"`
	BrightMin    float64 `yaml:"brightMin"`
	BrightMax    float64 `yaml:"brightMax"`
	AmbientBase  float64 `yaml:"ambientBase"`
	AmbientScale float64 `yaml:"ambientScale"`
	AmbientPow   float64 `yaml:"ambientPow"`
}

// PerspectiveConfig parameterizes the scene's vantage point over the ground plane.
//
// LowChance is the probability (in [0,1]) that the scene.v1 director rolls the
// "low" (ground-level) height rather than "high" (the original elevated look). It
// doubles as a deterministic switch: 1.0 forces every scene low, 0.0 forces high.
//
// The remaining fields shape the low-mode look (high mode is byte-identical to the
// v0 algorithms and ignores them all):
//
//   - GroundNearCell / GroundDetailCell / GroundDetailAmt / GroundBias /
//     GroundContrast / HorizonGain / HorizonPivot — the base terrain's perspective
//     projection in low mode. The ground is textured as a flat plane seen at a grazing
//     angle: each row is given a depth that grows hyperbolically toward the horizon
//     (so the distance shrinks away fast), and the noise is sampled in world space —
//     both axes scaled by that depth — so the dirt stays isotropic (real blobs, not
//     streaks) and recedes to a central vanishing point. The texture is two layers:
//     a macro layer of GroundNearCell-pixel blobs (the large terrain structure, which
//     carries detail toward the horizon) plus a finer detail layer of
//     GroundDetailCell-pixel dirt grain at GroundDetailAmt relative strength (crisp up
//     close, faded out with distance where it would alias). GroundBias sets how sharply
//     the horizon recedes (smaller = the far field crushes into a thinner band).
//     GroundContrast multiplies the combined light/dark variation. The depth falloff
//     is steepened by (1 + HorizonGain*pitch), where pitch rises from 0 when the
//     horizon sits at HorizonPivot (eye level) toward 1 as the horizon drops down-
//     screen (more sky, i.e. the viewer is looking up).
//   - ShorePerspHigh / ShorePerspLow / ShoreBias — how strongly the swell is bent by
//     perspective: the wave crests bunch into a calm mirror near the horizon and open
//     into large swells toward the viewer. ShorePerspHigh applies in high mode,
//     ShorePerspLow in low; ShoreBias sets how sharply the wave world-depth recedes.
//   - LandDistHigh / LandDistLow — how far the coast sits. The shoreline is a geometric
//     coastline map (see internal/scene/shoreline.go), not a noise field: a mainland
//     whose coast is a sweep of long arcs with the odd peninsula, plus lobed islands,
//     drawn in a top-down world plane and draped through the perspective so straight
//     coasts converge toward the vanishing point and beaches hug the shore. LandDist
//     scales world distance: high mode (1.0 — "seeing more") keeps the coast where the
//     map places it and lets land spread down the view; low mode (larger — "on the
//     ground, seeing less") pushes the coast toward the horizon for an open-water
//     foreground. The cities read the same map, so buildings stay on the land the water
//     leaves dry.
//   - WaveNearHigh / WaveNearLow / WaveOctaves — the swell. The wave amplitude grows
//     toward the viewer up to WaveNear× the base near the bottom (WaveNearHigh in high
//     mode, WaveNearLow in low — so near waves get much larger), and the ripple is
//     summed over WaveOctaves scales (long swells carrying short chop) sampled in
//     perspective world space, like the ground's layered noise.
//   - CityBandCap — caps the city's depth band (as a fraction of the ground height)
//     in low mode, so the city stays pinned far-off near the horizon instead of
//     marching down into the stretched foreground.
type PerspectiveConfig struct {
	LowChance float64 `yaml:"lowChance"`

	GroundNearCell   float64 `yaml:"groundNearCell"`
	GroundDetailCell float64 `yaml:"groundDetailCell"`
	GroundDetailAmt  float64 `yaml:"groundDetailAmt"`
	GroundBias       float64 `yaml:"groundBias"`
	GroundContrast   float64 `yaml:"groundContrast"`
	HorizonPivot     float64 `yaml:"horizonPivot"`
	HorizonGain      float64 `yaml:"horizonGain"`

	ShorePerspHigh float64 `yaml:"shorePerspHigh"`
	ShorePerspLow  float64 `yaml:"shorePerspLow"`
	ShoreBias      float64 `yaml:"shoreBias"`
	LandDistHigh   float64 `yaml:"landDistHigh"`
	LandDistLow    float64 `yaml:"landDistLow"`
	WaveNearHigh   float64 `yaml:"waveNearHigh"`
	WaveNearLow    float64 `yaml:"waveNearLow"`
	WaveOctaves    int     `yaml:"waveOctaves"`

	CityBandCap float64 `yaml:"cityBandCap"`
}

// MountainConfig parameterizes the extra mountain ranges (mountainranges.v0): the
// receding ridgelines that fill the midground below the horizon range, behind the
// city. The scene.v1 director resolves the per-vantage values (the *High/*Low
// pairs and count caps) into the globals; the element then rolls a count and
// varies each range a little around the base height/smoothness.
//
//   - Chance — probability a scene has any extra ranges at all.
//   - CountMaxHigh / CountMaxLow — the most extra ranges drawn from each vantage
//     (a high, elevated view shows more receding ridgelines than a ground-level one).
//   - BaselineFracHigh / BaselineFracLow — how far below the horizon the nearest
//     range's foot may sit, as a fraction of the ground height. High spreads the
//     ranges down the ground; low keeps them in a thin strip at the horizon.
//   - HeightFrac / HeightStd — mean and standard deviation of a range's peak height,
//     as a fraction of the horizon height in pixels (kept modest so the extra ranges
//     read as background behind the taller horizon range).
//   - Smoothness / SmoothnessStd — mean and standard deviation of ridge smoothness
//     (high = few gentle key points, low = many jagged ones; see mountainHeights).
//   - BaselineJitter — per-range jitter of the foot row, as a fraction of the ground
//     height, so the ranges are not perfectly evenly spaced.
type MountainConfig struct {
	Chance           float64 `yaml:"chance"`
	CountMaxHigh     int     `yaml:"countMaxHigh"`
	CountMaxLow      int     `yaml:"countMaxLow"`
	BaselineFracHigh float64 `yaml:"baselineFracHigh"`
	BaselineFracLow  float64 `yaml:"baselineFracLow"`
	HeightFrac       float64 `yaml:"heightFrac"`
	HeightStd        float64 `yaml:"heightStd"`
	Smoothness       float64 `yaml:"smoothness"`
	SmoothnessStd    float64 `yaml:"smoothnessStd"`
	BaselineJitter   float64 `yaml:"baselineJitter"`

	// RuggedChance is the probability (in [0,1]) that a scene renders its mountains in
	// the alternate "rugged" style — a craggier, more broken-rock look — instead of the
	// default conical (soft, eroded-slope) shading. 0 disables it; 1 forces it. The
	// scene.v1 director rolls the per-scene style from it, applied to both the horizon
	// range and the extra ranges.
	RuggedChance float64 `yaml:"ruggedChance"`
}

// MistConfig parameterizes the ground mist that can settle among the extra mountain
// ranges. Mist is an atmospheric-haze-colored fog drawn after each range: a band that
// is fully opaque from a range's foot down to the next range's clip line and fades up
// over the range's lower slopes, so peaks emerge from the fog. From a high vantage the
// opaque band reaches the bottom of the scene (the ground vanishes into haze); from a
// low vantage it fades out below the nearest range so the near ground shows. Where
// there is an ocean the mist fades out across the open water away from the coast.
//
//   - Chance — probability the mist is present (it still needs foreground mountains).
//   - FadeUpFrac — how far the mist fades up a range's slopes, as a fraction of the sky.
//   - LowFadeFrac — at the low vantage, the distance below the nearest range over which
//     the opaque mist fades back out, as a fraction of the ground height.
//   - OceanFadeFrac — how far the mist reaches over open water before fading to nothing,
//     as a fraction of the scene width.
type MistConfig struct {
	Chance        float64 `yaml:"chance"`
	FadeUpFrac    float64 `yaml:"fadeUpFrac"`
	LowFadeFrac   float64 `yaml:"lowFadeFrac"`
	OceanFadeFrac float64 `yaml:"oceanFadeFrac"`
}

// pipelineElements is the scene's element order as versioned algorithm keys,
// used as the default Generator and Renderer key list (these resolve against the
// scene package's generator/renderer registries). Directors default to the single
// scene director. The keys are part of the on-disk config contract.
var pipelineElements = []string{
	"sky.v0", "stars.v0", "systemstars.v0", "planets.v0",
	"clouds.v0", "mountains.v1", "ground.v1", "cities.v1", "water.v1", "mountainranges.v0",
}

// DefaultConfig returns the complete built-in configuration. Its values mirror
// the constants the scene code has always used, so a scene generated with the
// default config is identical to one generated before configuration existed.
// Each call returns a fresh value (with its own slices) so callers may mutate the
// result freely.
func DefaultConfig() Config {
	gens := append([]string(nil), pipelineElements...)
	rends := append([]string(nil), pipelineElements...)
	return Config{
		Algorithms: Algorithms{
			Directors:  []string{"scene.v1"},
			Generators: gens,
			Renderers:  rends,
		},
		Horizon:     HorizonConfig{Min: 0.20, Max: 0.50, Mean: 0.35, Std: 0.06},
		Twinkle:     TwinkleConfig{Max: 90.0, Std: 24.0},
		StarDensity: DensityConfig{Std: 0.9, Bias: 0.95, Clamp: 3.0},
		Lighting: LightingConfig{
			ColorSatMin: 0.0, ColorSatMax: 0.35, ColorValue: 1.0,
			BrightMin: 0.40, BrightMax: 1.0,
			AmbientBase: 0.02, AmbientScale: 0.38, AmbientPow: 2.0,
		},
		Perspective: PerspectiveConfig{
			LowChance:        0.5,
			GroundNearCell:   140.0, // ~140px macro blobs at the nearest ground: large terrain structure
			GroundDetailCell: 16.0,  // ~16px fine dirt grain layered on the blobs up close
			GroundDetailAmt:  0.6,   // detail-layer strength relative to the macro layer
			GroundBias:       1.2,   // horizon recession sharpness (smaller crushes the far field into a thinner band)
			GroundContrast:   1.4,   // texture light/dark strength for the near dirt
			HorizonPivot:     0.50,  // horizon at vertical middle reads as eye level
			HorizonGain:      1.0,   // looking up (low horizon) steepens the depth falloff
			ShorePerspHigh:   0.2,   // gentle swell perspective from the elevated view
			ShorePerspLow:    1.0,   // full swell perspective at ground level
			ShoreBias:        0.18,  // wave world-depth recession sharpness
			LandDistHigh:     1.0,   // high (elevated): the coast sits where the map places it
			LandDistLow:      3.0,   // low (on the ground): the coast is pushed toward the horizon, open water in front
			WaveNearHigh:     3.0,   // near waves ~3× the base in high mode
			WaveNearLow:      8.0,   // near waves much larger at ground level
			WaveOctaves:      4,     // long swells carrying shorter chop
			CityBandCap:      0.02,  // keep the city pinned almost on the horizon itself
		},
		Mountains: MountainConfig{
			Chance:           0.6,   // most scenes get a few receding ridgelines
			CountMaxHigh:     10,    // a deep stack of receding ridges from the elevated view
			CountMaxLow:      2,     // at most a range or two at ground level
			BaselineFracHigh: 1.10,  // the nearest feet reach the bottom edge (and a touch below)
			BaselineFracLow:  0.04,  // a thin strip near the horizon at ground level
			HeightFrac:       0.06,  // mean peak ~6% of the horizon height (below the horizon range)
			HeightStd:        0.008, // ranges stay close to the standard height
			Smoothness:       0.6,   // gently rolling on average
			SmoothnessStd:    0.05,  // only a slight spread in jaggedness
			BaselineJitter:   0.008, // feet spaced near-evenly, just off the grid
			RuggedChance:     0.15,  // mostly soft conical; the occasional craggier range
		},
		Mist: MistConfig{
			Chance:        1.0,  // always, whenever a scene has foreground mountains
			FadeUpFrac:    0.08, // the mist fades up ~8% of the sky over each range's slopes
			LowFadeFrac:   0.25, // low view: opaque mist fades out over ~25% of the ground
			OceanFadeFrac: 0.10, // mist reaches ~10% of the width over open water, then gone
		},
	}
}

// Load merges a (possibly partial) YAML config over the defaults and returns the
// resulting complete config. Any field absent from data keeps its default value,
// so a user can record only the values they care about. An empty input yields the
// default config. It is an error if data is not valid YAML for a Config.
func Load(data []byte) (Config, error) {
	c := DefaultConfig()
	if len(data) == 0 {
		return c, nil
	}
	// yaml.v3 decodes into the existing struct, overwriting only the fields the
	// document actually contains and leaving the rest at their default values —
	// this is exactly the partial-over-default merge we want.
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return c, nil
}

// Marshal serializes the (complete) config to YAML. The system always writes a
// complete config so the result reproduces its scene exactly.
func (c Config) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return data, nil
}
