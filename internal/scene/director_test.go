package scene

import (
	"testing"

	"github.com/zostay/scifi-landscape/internal/config"
)

// TestDirectDeterministic checks the no-side-effect contract: the same seed and
// config always yield identical globals.
func TestDirectDeterministic(t *testing.T) {
	d := DefaultDirector()
	cfg := config.DefaultConfig()
	a := d.Direct(cfg, 123, "", 1280, 720)
	b := d.Direct(cfg, 123, "", 1280, 720)
	if a != b {
		t.Fatalf("same seed+config gave different globals:\n %+v\n %+v", a, b)
	}
	if a.Seed != 123 || a.W != 1280 || a.H != 720 {
		t.Fatalf("globals did not carry seed/dimensions: %+v", a)
	}
}

// TestDirectConfigDrivesGlobals proves the director actually reads the config
// rather than hardcoded constants: pinning the horizon distribution to a single
// point forces every seed's horizon there.
func TestDirectConfigDrivesGlobals(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Horizon.Min, cfg.Horizon.Max, cfg.Horizon.Mean, cfg.Horizon.Std = 0.5, 0.5, 0.5, 0
	d := DefaultDirector()
	for seed := range int64(500) {
		g := d.Direct(cfg, seed, "", 1280, 720)
		if g.Settings.Horizon != 0.5 {
			t.Fatalf("seed %d: horizon %.4f, want 0.5 (config-pinned)", seed, g.Settings.Horizon)
		}
	}
	// And a different mean shifts results away from the default.
	cfg2 := config.DefaultConfig()
	cfg2.Twinkle.Max = 0 // clamp twinkle to zero for all seeds
	for seed := range int64(200) {
		if g := d.Direct(cfg2, seed, "", 1280, 720); g.Settings.TwinkleAngle != 0 {
			t.Fatalf("seed %d: twinkle %.4f, want 0 (config-clamped)", seed, g.Settings.TwinkleAngle)
		}
	}
}

// TestDirectorRegistry checks the registry resolves known directors and rejects
// unknown ones.
func TestDirectorRegistry(t *testing.T) {
	if d, ok := DirectorByName("scene.v0"); !ok || d.Name() != "scene.v0" {
		t.Fatalf("scene.v0 not resolved: %v %v", d, ok)
	}
	if _, ok := DirectorByName("scene.vNope"); ok {
		t.Fatal("unknown director resolved")
	}
}

// TestNewSettingsMatchesDirector checks the back-compat wrapper stays in lockstep
// with the director under the default config.
func TestNewSettingsMatchesDirector(t *testing.T) {
	for seed := range int64(300) {
		ns := NewSettings(seed, "", 720)
		dg := DefaultDirector().Direct(config.DefaultConfig(), seed, "", 0, 720).Settings
		if ns != dg {
			t.Fatalf("seed %d: NewSettings != director:\n %+v\n %+v", seed, ns, dg)
		}
	}
}
