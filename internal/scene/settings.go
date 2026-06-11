package scene

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/gfx"
)

// TimeOfDay is a global scene setting that drives how every element is colored.
type TimeOfDay int

const (
	Midday TimeOfDay = iota
	Dusk
	Twilight
)

func (t TimeOfDay) String() string {
	switch t {
	case Midday:
		return "midday"
	case Dusk:
		return "dusk"
	case Twilight:
		return "twilight"
	default:
		return "unknown"
	}
}

// MarshalYAML serializes a time of day as its lowercase name, so globals.yaml
// reads "midday"/"dusk"/"twilight" rather than an opaque integer.
func (t TimeOfDay) MarshalYAML() (any, error) { return t.String(), nil }

// UnmarshalYAML parses a time of day from its name, rejecting unknown values so a
// malformed globals file fails loudly.
func (t *TimeOfDay) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	tod, ok := ParseTimeOfDay(s)
	if !ok {
		return fmt.Errorf("scene: unknown time of day %q", s)
	}
	*t = tod
	return nil
}

// ParseTimeOfDay parses a time-of-day name. ok is false for unrecognized input
// (including the empty string), which callers treat as "choose from the seed".
func ParseTimeOfDay(s string) (t TimeOfDay, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "midday", "midnoon", "noon", "day":
		return Midday, true
	case "dusk", "sunset":
		return Dusk, true
	case "twilight", "night":
		return Twilight, true
	default:
		return Midday, false
	}
}

// The constants that used to live here — the horizon distribution, the twinkle
// angle, and the star-density multiplier — now live in config (HorizonConfig,
// TwinkleConfig, DensityConfig) and are applied by the scene director (see
// director.go). config.DefaultConfig holds the original values and documents
// their rationale.

// Settings holds the global, scene-wide choices made up front. Every value is
// derived from the seed (via rng) unless explicitly overridden. The yaml tags
// pin its serialized form (it is recorded in a scene file's globals.yaml).
type Settings struct {
	Time TimeOfDay `yaml:"time"`

	// Horizon is the horizon height as a fraction of the scene measured from
	// the bottom [0.20, 0.50] — i.e. the ground's share of the height.
	Horizon float64 `yaml:"horizon"`
	// HorizonY is the horizon's pixel row from the top of the image. Because
	// Horizon is measured from the bottom, HorizonY = height * (1 - Horizon).
	HorizonY int `yaml:"horizonY"`

	// TwinkleAngle is the shared orientation of star twinkle spikes, in degrees
	// [0, 90].
	TwinkleAngle float64 `yaml:"twinkleAngle"`
	// StarDensity multiplies the earthlike star count: ~1.0 is earthlike, well
	// below 1 is a near-empty sky, well above 1 is a dense cluster.
	StarDensity float64 `yaml:"starDensity"`

	// Dominant-star lighting for planets. The light's screen angle is the same
	// TwinkleAngle above.
	//
	// LightColor tints the lit side of planets. LightBrightness is the
	// terminator harshness: high makes a sharp shadow line, low a soft fade.
	// LightPhase is how much of the planets is lit: 1 is full (fully sunlit), 0
	// is new (visible only by ambient light). LightAmbient is fill light in the
	// shadowed part: 0 leaves shadows black, higher keeps features visible.
	LightColor      gfx.RGB `yaml:"lightColor"`
	LightBrightness float64 `yaml:"lightBrightness"`
	LightPhase      float64 `yaml:"lightPhase"`
	LightAmbient    float64 `yaml:"lightAmbient"`
}

// NewSettings derives the global settings for a scene of height h using the
// default configuration. It is a convenience wrapper over the scene director (see
// director.go), preserved for callers and tests that only need the Settings and
// the default tuning; the constants that used to live here now live in
// config.DefaultConfig. timeOverride is an optional time-of-day name; an
// empty/unknown value means "use the seed's choice".
func NewSettings(seed int64, timeOverride string, h int) Settings {
	return DefaultDirector().Direct(config.DefaultConfig(), seed, timeOverride, 0, h).Settings
}
