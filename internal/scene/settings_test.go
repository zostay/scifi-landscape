package scene

import (
	"math"
	"testing"
)

var exp = math.Exp

func TestNewSettingsHorizonInRange(t *testing.T) {
	for seed := range int64(2000) {
		s := NewSettings(seed, "", 720)
		if s.Horizon < horizonMin-1e-9 || s.Horizon > horizonMax+1e-9 {
			t.Fatalf("seed %d: horizon %.3f out of [%.2f,%.2f]", seed, s.Horizon, horizonMin, horizonMax)
		}
		if s.HorizonY < 1 || s.HorizonY > 719 {
			t.Fatalf("seed %d: HorizonY %d out of bounds", seed, s.HorizonY)
		}
	}
}

func TestNewSettingsStarParamsInRange(t *testing.T) {
	for seed := range int64(2000) {
		s := NewSettings(seed, "", 720)
		if s.TwinkleAngle < 0 || s.TwinkleAngle > twinkleMax {
			t.Fatalf("seed %d: twinkle %.2f out of [0,%.0f]", seed, s.TwinkleAngle, twinkleMax)
		}
		// Density is log-normal with the exponent clamped to ±densityClamp.
		lo := exp(-densityClamp * densityStd)
		hi := exp(densityClamp * densityStd)
		if s.StarDensity < lo-1e-9 || s.StarDensity > hi+1e-9 {
			t.Fatalf("seed %d: density %.3f out of [%.3f,%.3f]", seed, s.StarDensity, lo, hi)
		}
	}
}

func TestNewSettingsDeterministic(t *testing.T) {
	a := NewSettings(123, "", 720)
	b := NewSettings(123, "", 720)
	if a != b {
		t.Fatalf("same seed gave different settings: %+v vs %+v", a, b)
	}
}

// TestTimeOverrideKeepsHorizonStable verifies that forcing the time of day does
// not shift the random stream, so the horizon (and everything downstream) stays
// reproducible regardless of the override.
func TestTimeOverrideKeepsHorizonStable(t *testing.T) {
	base := NewSettings(55, "", 720)
	for _, name := range []string{"midday", "dusk", "twilight"} {
		s := NewSettings(55, name, 720)
		if s.Horizon != base.Horizon {
			t.Errorf("override %q changed horizon: %.4f vs %.4f", name, s.Horizon, base.Horizon)
		}
		if want, _ := ParseTimeOfDay(name); s.Time != want {
			t.Errorf("override %q gave time %v, want %v", name, s.Time, want)
		}
	}
}

func TestParseTimeOfDay(t *testing.T) {
	if _, ok := ParseTimeOfDay(""); ok {
		t.Error("empty string should not parse")
	}
	if tod, ok := ParseTimeOfDay("DUSK"); !ok || tod != Dusk {
		t.Errorf("DUSK parsed as (%v,%v)", tod, ok)
	}
}
