package scene

import (
	"math"
	"strings"

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

// Horizon bounds: the horizon sits between 20% and 50% of the way *up from the
// bottom* of the scene, biased toward ~35% via a normal distribution. This is
// the ground's share of the height, so the sky always fills 50-80% — there is
// always at least as much sky as ground.
const (
	horizonMin  = 0.20
	horizonMax  = 0.50
	horizonMean = 0.35
	horizonStd  = 0.06
)

// Twinkle angle: the orientation of star twinkle spikes, shared by every star
// in a scene. It runs 0-90 degrees, biased hard toward 0 (upright spikes), with
// 90 the rarest. twinkleStd scales |normal|, so 90 sits ~3.75 sigma out.
const (
	twinkleMax = 90.0
	twinkleStd = 24.0
)

// Star density: a multiplier on the "earthlike" star count, drawn log-normally.
// densityStd scales a normal in log space; densityBias shifts its mean up so
// richer-than-earthlike skies are the norm (the typical multiplier is
// exp(densityBias*densityStd) ≈ 2.3×); densityClamp bounds the exponent so the
// tails still reach a near-empty sky or a dense cluster without going absurd.
const (
	densityStd   = 0.9
	densityBias  = 0.95 // typical ~2.3x earthlike; denser fields are common
	densityClamp = 3.0
)

// Settings holds the global, scene-wide choices made up front. Every value is
// derived from the seed (via rng) unless explicitly overridden.
type Settings struct {
	Time TimeOfDay

	// Horizon is the horizon height as a fraction of the scene measured from
	// the bottom [0.20, 0.50] — i.e. the ground's share of the height.
	Horizon float64
	// HorizonY is the horizon's pixel row from the top of the image. Because
	// Horizon is measured from the bottom, HorizonY = height * (1 - Horizon).
	HorizonY int

	// TwinkleAngle is the shared orientation of star twinkle spikes, in degrees
	// [0, 90].
	TwinkleAngle float64
	// StarDensity multiplies the earthlike star count: ~1.0 is earthlike, well
	// below 1 is a near-empty sky, well above 1 is a dense cluster.
	StarDensity float64

	// Dominant-star lighting for planets. The light's screen angle is the same
	// TwinkleAngle above.
	//
	// LightColor tints the lit side of planets. LightBrightness is the
	// terminator harshness: high makes a sharp shadow line, low a soft fade.
	// LightPhase is how much of the planets is lit: 1 is full (fully sunlit), 0
	// is new (visible only by ambient light). LightAmbient is fill light in the
	// shadowed part: 0 leaves shadows black, higher keeps features visible.
	LightColor      gfx.RGB
	LightBrightness float64
	LightPhase      float64
	LightAmbient    float64
}

// NewSettings derives the global settings for a scene of height h from its own
// independent random stream (derived from seed), so the settings stay stable as
// elements are added or changed.
//
// To keep the seed reproducible, the random stream is consumed in a fixed
// order regardless of overrides: we always draw the time-of-day and horizon
// values, then apply any override on top. timeOverride is an optional
// time-of-day name; an empty/unknown value means "use the seed's choice".
func NewSettings(seed int64, timeOverride string, h int) Settings {
	rng := deriveRng(seed, "settings")
	tod := TimeOfDay(rng.Intn(3))

	frac := horizonMean + rng.NormFloat64()*horizonStd
	if frac < horizonMin {
		frac = horizonMin
	}
	if frac > horizonMax {
		frac = horizonMax
	}

	// Twinkle angle: |normal| biased to 0, clamped to 90.
	twinkle := min(math.Abs(rng.NormFloat64())*twinkleStd, twinkleMax)

	// Star density: log-normal, biased above earthlike so denser skies are
	// the norm; still clamped so the tails stay sane.
	density := math.Exp(min(max(rng.NormFloat64()+densityBias, -densityClamp), densityClamp) * densityStd)

	// Dominant-star lighting. Color is a near-white tint (full value, low
	// saturation). Phase biased toward full so planets are usually well lit, but
	// reaches new. Ambient biased low so shadows are usually dark.
	lightColor := gfx.HSV{H: rng.Float64() * 360, S: rnd(rng, 0, 0.35), V: 1}.RGB()
	lightBright := rnd(rng, 0.40, 1.0)
	lightPhase := math.Sqrt(rng.Float64())
	// Ambient biased low (squared) so shadows usually fall dark, with the
	// occasional brighter fill.
	lightAmbient := 0.02 + 0.38*math.Pow(rng.Float64(), 2)

	if override, ok := ParseTimeOfDay(timeOverride); ok {
		tod = override
	}

	// Horizon is measured from the bottom, so convert to a row from the top.
	y := min(max(int((1-frac)*float64(h)), 1), h-1)

	return Settings{
		Time:            tod,
		Horizon:         frac,
		HorizonY:        y,
		TwinkleAngle:    twinkle,
		StarDensity:     density,
		LightColor:      lightColor,
		LightBrightness: lightBright,
		LightPhase:      lightPhase,
		LightAmbient:    lightAmbient,
	}
}
