package config

import (
	"reflect"
	"testing"
)

// TestDefaultRoundTrips checks that a complete config survives marshal → load
// unchanged, so a recorded config reproduces its scene exactly.
func TestDefaultRoundTrips(t *testing.T) {
	def := DefaultConfig()
	data, err := def.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := Load(data)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(def, got) {
		t.Fatalf("round-trip changed config:\n got %+v\nwant %+v", got, def)
	}
}

// TestEmptyLoadsDefaults checks that empty input yields the default config (a
// missing config is filled in entirely from defaults).
func TestEmptyLoadsDefaults(t *testing.T) {
	got, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, DefaultConfig()) {
		t.Fatalf("empty load did not yield defaults: %+v", got)
	}
}

// TestPartialMergeKeepsDefaults is the core partial-config behavior: a document
// that sets only a couple of fields must override exactly those, leaving every
// other field — including siblings within the same section — at its default.
func TestPartialMergeKeepsDefaults(t *testing.T) {
	partial := []byte("horizon:\n  mean: 0.42\nlighting:\n  brightMin: 0.7\n")
	got, err := Load(partial)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	def := DefaultConfig()

	if got.Horizon.Mean != 0.42 {
		t.Errorf("horizon.mean = %v, want 0.42 (overridden)", got.Horizon.Mean)
	}
	// Siblings within the overridden sections must keep their defaults.
	if got.Horizon.Min != def.Horizon.Min || got.Horizon.Max != def.Horizon.Max || got.Horizon.Std != def.Horizon.Std {
		t.Errorf("horizon siblings changed: %+v", got.Horizon)
	}
	if got.Lighting.BrightMin != 0.7 {
		t.Errorf("lighting.brightMin = %v, want 0.7", got.Lighting.BrightMin)
	}
	if got.Lighting.BrightMax != def.Lighting.BrightMax || got.Lighting.AmbientBase != def.Lighting.AmbientBase {
		t.Errorf("lighting siblings changed: %+v", got.Lighting)
	}
	// Untouched sections are wholly default.
	if !reflect.DeepEqual(got.Twinkle, def.Twinkle) || !reflect.DeepEqual(got.StarDensity, def.StarDensity) {
		t.Errorf("untouched sections changed")
	}
	if !reflect.DeepEqual(got.Algorithms, def.Algorithms) {
		t.Errorf("algorithms changed: %+v", got.Algorithms)
	}
}

// TestDefaultConfigIsolated checks that DefaultConfig returns independent copies:
// mutating one result's slices must not affect another.
func TestDefaultConfigIsolated(t *testing.T) {
	a := DefaultConfig()
	b := DefaultConfig()
	a.Algorithms.Generators[0] = "tampered"
	if b.Algorithms.Generators[0] == "tampered" {
		t.Fatal("DefaultConfig shares slice state between calls")
	}
}

// TestInvalidYAML reports an error rather than silently returning defaults.
func TestInvalidYAML(t *testing.T) {
	if _, err := Load([]byte("horizon: [this is not a mapping]")); err == nil {
		t.Fatal("expected error for malformed config")
	}
}
