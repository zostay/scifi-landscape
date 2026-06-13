package scene

import (
	"fmt"
	"math"

	"gopkg.in/yaml.v3"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Globals are the complete, scene-wide values a Director derives from a seed and
// a configuration, before any entity is generated. Unlike a configuration,
// globals are always complete — no field is ever missing or assumed.
//
// The globals are the master seed, the scene dimensions, the derived Settings
// (time of day, horizon, twinkle, star density, and dominant-star lighting), and
// the scene-wide sky/ground gradients that renderers read. Capturing the gradients
// here (rather than re-deriving them from the seed at render time) lets a recorded
// scene list redraw the same image without the seed. The per-element generation
// streams are still derived from Seed (see deriveRng), and the ocean/land model
// remains working state rebuilt from the seed in Build (no renderer reads it — it
// is captured per-scene in the water entity). Existing fields are never changed,
// only added to, so a recorded globals keeps reproducing its scene.
type Globals struct {
	// Seed is the scene's master seed; per-element streams derive from it.
	Seed int64 `yaml:"seed"`
	// W and H are the scene dimensions in pixels.
	W int `yaml:"w"`
	H int `yaml:"h"`
	// Settings are the derived scene-wide look choices.
	Settings Settings `yaml:"settings"`
	// SkyGradient is the horizon→top sky color gradient, read by the sky, planet,
	// and city-dome renderers.
	SkyGradient gfx.Gradient `yaml:"skyGradient"`
	// GroundGradient is the horizon→foreground ground color gradient, read by the
	// ground renderer; GroundVariable reports whether the multi-color "variable"
	// ground mode was chosen (it shapes the gradient).
	GroundGradient gfx.Gradient `yaml:"groundGradient"`
	GroundVariable bool         `yaml:"groundVariable"`
}

// Marshal serializes the globals to YAML for a scene file's globals.yaml. Globals
// are always complete, so the output fully captures the derived scene-wide state.
func (g Globals) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("globals: %w", err)
	}
	return data, nil
}

// UnmarshalGlobals parses globals from YAML produced by Globals.Marshal.
func UnmarshalGlobals(data []byte) (Globals, error) {
	var g Globals
	if err := yaml.Unmarshal(data, &g); err != nil {
		return Globals{}, fmt.Errorf("globals: %w", err)
	}
	return g, nil
}

// Director turns a seed and a complete configuration into the scene's globals. A
// Director has no side effects: the same seed and config must always yield
// identical globals, so the globals alone (with the generators named by the
// config) can reproduce the scene list.
//
// Directors are versioned and frozen once released: a behavioral change is made
// as a new director (e.g. "scene.v1"), never by editing an existing one, so old
// seeds keep their meaning. The config names which director builds a scene.
type Director interface {
	// Name is the director's versioned registry key (e.g. "scene.v0").
	Name() string
	// Direct derives the globals for a scene of size w x h from the seed and
	// config. timeOverride, when it names a time of day, forces that time without
	// disturbing the random stream (see Settings).
	Direct(cfg config.Config, seed int64, timeOverride string, w, h int) Globals
}

// sceneDirectorV0 is the original director: it derives the scene-wide Settings
// the app has always used, now parameterized by configuration instead of
// hardcoded constants. FROZEN: do not change its draw order or math; add a
// sceneDirectorV1 for new behavior.
type sceneDirectorV0 struct{}

func (sceneDirectorV0) Name() string { return "scene.v0" }

func (sceneDirectorV0) Direct(cfg config.Config, seed int64, timeOverride string, w, h int) Globals {
	rng := deriveRng(seed, "settings")

	// The random stream is consumed in a fixed order regardless of overrides, so
	// the seed stays reproducible. See the original NewSettings for the rationale.
	tod := TimeOfDay(rng.Intn(3))

	frac := cfg.Horizon.Mean + rng.NormFloat64()*cfg.Horizon.Std
	if frac < cfg.Horizon.Min {
		frac = cfg.Horizon.Min
	}
	if frac > cfg.Horizon.Max {
		frac = cfg.Horizon.Max
	}

	twinkle := min(math.Abs(rng.NormFloat64())*cfg.Twinkle.Std, cfg.Twinkle.Max)

	density := math.Exp(min(max(rng.NormFloat64()+cfg.StarDensity.Bias, -cfg.StarDensity.Clamp), cfg.StarDensity.Clamp) * cfg.StarDensity.Std)

	lightColor := gfx.HSV{H: rng.Float64() * 360, S: rnd(rng, cfg.Lighting.ColorSatMin, cfg.Lighting.ColorSatMax), V: cfg.Lighting.ColorValue}.RGB()
	lightBright := rnd(rng, cfg.Lighting.BrightMin, cfg.Lighting.BrightMax)
	lightPhase := math.Sqrt(rng.Float64())
	lightAmbient := cfg.Lighting.AmbientBase + cfg.Lighting.AmbientScale*math.Pow(rng.Float64(), cfg.Lighting.AmbientPow)

	if override, ok := ParseTimeOfDay(timeOverride); ok {
		tod = override
	}

	// Horizon is measured from the bottom, so convert to a row from the top.
	y := min(max(int((1-frac)*float64(h)), 1), h-1)

	// Derive the scene-wide sky and ground gradients here so they are part of the
	// globals (and recorded in a scene file), rather than rebuilt from the seed at
	// render time. Each draws from its own stream off the seed, independent of the
	// "settings" stream above; these are the exact calls Scene.Build once made, so
	// the output is unchanged.
	skyGrad := buildSkyGradient(deriveRng(seed, "sky-gradient"), tod)
	gg := deriveRng(seed, "ground-gradient")
	groundVar := gg.Float64() < groundVariableChance
	groundGrad := buildGroundGradient(gg, tod, groundVar)

	return Globals{
		Seed: seed,
		W:    w,
		H:    h,
		Settings: Settings{
			Time:            tod,
			Horizon:         frac,
			HorizonY:        y,
			TwinkleAngle:    twinkle,
			StarDensity:     density,
			LightColor:      lightColor,
			LightBrightness: lightBright,
			LightPhase:      lightPhase,
			LightAmbient:    lightAmbient,
		},
		SkyGradient:    skyGrad,
		GroundGradient: groundGrad,
		GroundVariable: groundVar,
	}
}

// directors is the registry of available directors, keyed by versioned name.
// Existing entries are frozen; new behavior is added as a new key.
var directors = map[string]Director{
	"scene.v0": sceneDirectorV0{},
}

// DirectorByName returns the registered director for a config key, or false if
// no such director exists.
func DirectorByName(name string) (Director, bool) {
	d, ok := directors[name]
	return d, ok
}

// DefaultDirector returns the director used when none is otherwise selected.
func DefaultDirector() Director { return sceneDirectorV0{} }
