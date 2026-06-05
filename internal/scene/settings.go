package scene

import (
	"math/rand"
	"strings"
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
}

// NewSettings derives the global settings from rng for a scene of height h.
//
// To keep the seed reproducible, the random stream is consumed in a fixed
// order regardless of overrides: we always draw the time-of-day and horizon
// values, then apply any override on top. timeOverride is an optional
// time-of-day name; an empty/unknown value means "use the seed's choice".
func NewSettings(rng *rand.Rand, timeOverride string, h int) Settings {
	tod := TimeOfDay(rng.Intn(3))

	frac := horizonMean + rng.NormFloat64()*horizonStd
	if frac < horizonMin {
		frac = horizonMin
	}
	if frac > horizonMax {
		frac = horizonMax
	}

	if override, ok := ParseTimeOfDay(timeOverride); ok {
		tod = override
	}

	// Horizon is measured from the bottom, so convert to a row from the top.
	y := min(max(int((1-frac)*float64(h)), 1), h-1)

	return Settings{
		Time:     tod,
		Horizon:  frac,
		HorizonY: y,
	}
}
