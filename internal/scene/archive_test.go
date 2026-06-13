package scene

import (
	"reflect"
	"testing"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/scenefile"
)

// TestSceneTextsAndLoadRoundTrip checks the save/load helpers agree: texts built
// by SceneTexts parse back to the same seed, config, and globals via
// LoadSceneTexts. The scene list is omitted here (nil), so it must read as absent.
func TestSceneTextsAndLoadRoundTrip(t *testing.T) {
	const seed = int64(99)
	cfg := config.DefaultConfig()
	g := DefaultDirector().Direct(cfg, seed, "", 800, 600)

	texts, err := SceneTexts(seed, cfg, &g, nil)
	if err != nil {
		t.Fatalf("SceneTexts: %v", err)
	}
	if _, ok := texts[scenefile.KeySceneList]; ok {
		t.Error("scene list should be omitted when nil")
	}

	ls, err := LoadSceneTexts(texts)
	if err != nil {
		t.Fatalf("LoadSceneTexts: %v", err)
	}
	if !ls.HasSeed || ls.Seed != seed {
		t.Errorf("seed = %d (has=%v), want %d", ls.Seed, ls.HasSeed, seed)
	}
	if !ls.HasConfig || !reflect.DeepEqual(ls.Config, cfg) {
		t.Errorf("config did not round-trip")
	}
	if !ls.HasGlobals || !reflect.DeepEqual(ls.Globals, g) {
		t.Errorf("globals did not round-trip:\n got %+v\nwant %+v", ls.Globals, g)
	}
	if ls.HasSceneList {
		t.Error("scene list reported present but was omitted")
	}
}

// TestLoadSceneTextsFillsConfigDefaults checks the missing-layer behavior: a scene
// file with only a seed yields a complete (default) config and no globals/list.
func TestLoadSceneTextsFillsConfigDefaults(t *testing.T) {
	ls, err := LoadSceneTexts(map[string]string{scenefile.KeySeed: "7"})
	if err != nil {
		t.Fatalf("LoadSceneTexts: %v", err)
	}
	if !ls.HasSeed || ls.Seed != 7 {
		t.Errorf("seed = %d (has=%v), want 7", ls.Seed, ls.HasSeed)
	}
	if ls.HasConfig {
		t.Error("HasConfig should be false when no config chunk is present")
	}
	if !reflect.DeepEqual(ls.Config, config.DefaultConfig()) {
		t.Error("Config should be filled from defaults when absent")
	}
	if ls.HasGlobals || ls.HasSceneList {
		t.Error("globals/scene-list should be absent")
	}
}

// TestLoadSceneTextsPartialConfig checks that a partial config chunk is completed
// from defaults on load.
func TestLoadSceneTextsPartialConfig(t *testing.T) {
	texts := map[string]string{scenefile.KeyConfig: "horizon:\n  mean: 0.49\n"}
	ls, err := LoadSceneTexts(texts)
	if err != nil {
		t.Fatalf("LoadSceneTexts: %v", err)
	}
	if !ls.HasConfig {
		t.Error("HasConfig should be true when a config chunk is present")
	}
	if ls.Config.Horizon.Mean != 0.49 {
		t.Errorf("horizon.mean = %v, want 0.49 (from partial config)", ls.Config.Horizon.Mean)
	}
	if ls.Config.Horizon.Min != config.DefaultConfig().Horizon.Min {
		t.Error("partial config did not inherit default horizon.min")
	}
}

// TestLoadSceneTextsBadSeed reports an error for a non-numeric seed chunk.
func TestLoadSceneTextsBadSeed(t *testing.T) {
	if _, err := LoadSceneTexts(map[string]string{scenefile.KeySeed: "not-a-number"}); err == nil {
		t.Fatal("expected error for non-numeric seed")
	}
}
