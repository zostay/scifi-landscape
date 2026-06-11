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

// pipelineElements is the scene's element order, used as the default Generator
// and Renderer key list. Directors default to the single scene director.
var pipelineElements = []string{
	"sky", "stars", "systemstars", "planets",
	"clouds", "mountains", "ground", "cities", "water",
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
			Directors:  []string{"scene.v0"},
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
