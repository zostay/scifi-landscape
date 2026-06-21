package scene

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/config"
)

// buildHash builds a scene for cfg+seed headlessly and returns the SHA-256 of its
// raw RGBA pixels. It resolves the director from the config like the binaries do.
func buildHash(t *testing.T, cfg config.Config, seed int64, w, h int) [32]byte {
	t.Helper()
	dir := DefaultDirector()
	if dirs := cfg.Algorithms.Directors; len(dirs) > 0 {
		if d, ok := DirectorByName(dirs[0]); ok {
			dir = d
		}
	}
	g := dir.Direct(cfg, seed, "", w, h)
	cv := canvas.New(w, h)
	sc, err := New(g, cfg.Algorithms)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := sc.Build(WithInstant(context.Background()), cv, seed, w, h, nil); err != nil {
		t.Fatalf("build: %v", err)
	}
	buf := make([]byte, w*h*4)
	cv.Snapshot(buf)
	return sha256.Sum256(buf)
}

// perspectiveSeeds spreads across the element-presence space (ocean vs none, city
// vs none, varied horizons).
var perspectiveSeeds = []int64{1, 2, 3, 7, 11, 42, 100, 256, 1024, -5}

// groundPipeline returns a config whose pipeline is the shared elements plus the
// given ground algorithm and director, and no cities or water — so it isolates the
// ground's output (the ocean/cities are excluded). It is used to prove ground.v1's
// high vantage is byte-identical to ground.v0.
func groundPipeline(director, ground string) config.Config {
	cfg := config.DefaultConfig()
	cfg.Perspective.LowChance = 0.0 // force high where the director is scene.v1
	cfg.Algorithms.Directors = []string{director}
	gens := []string{"sky.v0", "stars.v0", "systemstars.v0", "planets.v0", "clouds.v0", "mountains.v0", ground}
	cfg.Algorithms.Generators = gens
	cfg.Algorithms.Renderers = append([]string(nil), gens...)
	return cfg
}

// TestGroundHighMatchesV0 is the freeze guard for the ground element: ground.v1 in
// its high vantage must be byte-identical to ground.v0, so a high-rolling seed's
// ground is unchanged. (cities.v1 and water.v1 deliberately differ from v0 in high
// too — they gain perspective shorelines and waves — so they are excluded here and
// frozen by their own golden cases instead.)
func TestGroundHighMatchesV0(t *testing.T) {
	v0 := groundPipeline("scene.v0", "ground.v0")
	v1 := groundPipeline("scene.v1", "ground.v1")
	for _, s := range perspectiveSeeds {
		if buildHash(t, v1, s, 480, 270) != buildHash(t, v0, s, 480, 270) {
			t.Errorf("seed %d: ground.v1 high differs from ground.v0 — high mode must be byte-identical", s)
		}
	}
}

// TestPerspectiveLowDiffersFromHigh proves the low vantage actually transforms the
// image: across the seed spread, at least some scenes must change between high and
// low (every scene has a ground plane to widen).
func TestPerspectiveLowDiffersFromHigh(t *testing.T) {
	low := forcedHeightConfig(1.0)
	high := forcedHeightConfig(0.0)
	diffs := 0
	for _, s := range perspectiveSeeds {
		if buildHash(t, low, s, 480, 270) != buildHash(t, high, s, 480, 270) {
			diffs++
		}
	}
	if diffs == 0 {
		t.Fatal("low vantage never changed any scene; the perspective transform did nothing")
	}
}

// TestSceneV1DerivesHeight checks the scene.v1 director: it rolls the height on its
// own stream (forced via LowChance) and resolves the low-mode perspective, while
// leaving every Settings field identical to what scene.v0 derives.
func TestSceneV1DerivesHeight(t *testing.T) {
	d, ok := DirectorByName("scene.v1")
	if !ok {
		t.Fatal("scene.v1 not registered")
	}
	lowCfg := config.DefaultConfig()
	lowCfg.Perspective.LowChance = 1.0
	highCfg := config.DefaultConfig()
	highCfg.Perspective.LowChance = 0.0

	for _, s := range perspectiveSeeds {
		gl := d.Direct(lowCfg, s, "", 480, 270)
		if gl.Height != Low {
			t.Errorf("seed %d: lowChance=1 gave Height %v, want low", s, gl.Height)
		}
		if gl.Perspective.GroundNearCell <= 0 || gl.Perspective.GroundGamma <= 0 {
			t.Errorf("seed %d: low mode did not resolve the ground perspective: %+v", s, gl.Perspective)
		}
		// Low mode uses the strong shore perspective; high mode the mild one. Both are
		// resolved (water gets perspective at both vantages).
		if gl.Perspective.ShorePersp != lowCfg.Perspective.ShorePerspLow {
			t.Errorf("seed %d: low ShorePersp = %v, want %v", s, gl.Perspective.ShorePersp, lowCfg.Perspective.ShorePerspLow)
		}

		gh := d.Direct(highCfg, s, "", 480, 270)
		if gh.Height != High {
			t.Errorf("seed %d: lowChance=0 gave Height %v, want high", s, gh.Height)
		}
		if gh.Perspective.ShorePersp != highCfg.Perspective.ShorePerspHigh {
			t.Errorf("seed %d: high ShorePersp = %v, want %v", s, gh.Perspective.ShorePersp, highCfg.Perspective.ShorePerspHigh)
		}

		// The Settings (and gradients) must be exactly what v0 derives, regardless of
		// the rolled height — only the new fields are added.
		v0g := sceneDirectorV0{}.Direct(config.DefaultConfig(), s, "", 480, 270)
		if gl.Settings != v0g.Settings || gh.Settings != v0g.Settings {
			t.Errorf("seed %d: scene.v1 Settings diverged from scene.v0", s)
		}
	}
}

// TestGlobalsHeightRoundTrip checks the new globals fields survive YAML, so a
// recorded scene reproduces its vantage point without the seed.
func TestGlobalsHeightRoundTrip(t *testing.T) {
	d, _ := DirectorByName("scene.v1")
	lowCfg := config.DefaultConfig()
	lowCfg.Perspective.LowChance = 1.0
	g := d.Direct(lowCfg, 42, "", 480, 270)

	data, err := g.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	g2, err := UnmarshalGlobals(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if g2.Height != Low {
		t.Errorf("height did not round-trip: got %v", g2.Height)
	}
	if g2.Perspective != g.Perspective {
		t.Errorf("perspective did not round-trip:\n got  %+v\n want %+v", g2.Perspective, g.Perspective)
	}
}

// TestParseHeight checks the height-name parser used by the YAML codec.
func TestParseHeight(t *testing.T) {
	cases := map[string]struct {
		want HeightMode
		ok   bool
	}{
		"high": {High, true}, "low": {Low, true}, "LOW": {Low, true},
		"": {High, false}, "middle": {High, false},
	}
	for in, exp := range cases {
		got, ok := ParseHeight(in)
		if got != exp.want || ok != exp.ok {
			t.Errorf("ParseHeight(%q) = (%v, %v), want (%v, %v)", in, got, ok, exp.want, exp.ok)
		}
	}
	if High.String() != "high" || Low.String() != "low" {
		t.Errorf("HeightMode.String mismatch: %q %q", High.String(), Low.String())
	}
}
