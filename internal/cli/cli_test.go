package cli

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zostay/scifi-landscape/internal/scenefile"
)

// writeScene writes a 1x1 PNG scene file carrying the given text chunks and
// returns its path.
func writeScene(t *testing.T, texts map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scene.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := scenefile.Write(f, img, texts); err != nil {
		t.Fatal(err)
	}
	return path
}

// allLayers selects every extractable layer.
var allLayers = map[string]bool{"seed": true, "config": true, "globals": true, "scene": true}

func TestExtractScene(t *testing.T) {
	path := writeScene(t, map[string]string{
		scenefile.KeySeed:      "12345",
		scenefile.KeyConfig:    "horizon:\n  min: 0.5\n",
		scenefile.KeyGlobals:   "time: dusk\n",
		scenefile.KeySceneList: "- schema: sky.v0\n",
	})

	files, missing, err := ExtractScene(path, allLayers)
	if err != nil {
		t.Fatalf("ExtractScene: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
	// Files come back in canonical order with seed-named filenames and verbatim
	// content.
	want := []ExtractedLayer{
		{"seed", "scifi-12345.seed.txt", "12345"},
		{"config", "scifi-12345.config.yaml", "horizon:\n  min: 0.5\n"},
		{"globals", "scifi-12345.globals.yaml", "time: dusk\n"},
		{"scene", "scifi-12345.scene.yaml", "- schema: sky.v0\n"},
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %+v", len(files), len(want), files)
	}
	for i, f := range files {
		if f != want[i] {
			t.Errorf("file[%d] = %+v, want %+v", i, f, want[i])
		}
	}
}

func TestExtractSceneSelectsAndReportsMissing(t *testing.T) {
	// Only seed and config are embedded; the request asks for config and scene.
	path := writeScene(t, map[string]string{
		scenefile.KeySeed:   "7",
		scenefile.KeyConfig: "x: 1\n",
	})

	files, missing, err := ExtractScene(path, map[string]bool{"config": true, "scene": true})
	if err != nil {
		t.Fatalf("ExtractScene: %v", err)
	}
	if len(files) != 1 || files[0].File != "scifi-7.config.yaml" {
		t.Errorf("files = %+v, want only scifi-7.config.yaml", files)
	}
	if len(missing) != 1 || missing[0] != "scene" {
		t.Errorf("missing = %v, want [scene]", missing)
	}
}

func TestExtractSceneNoSeed(t *testing.T) {
	// Without a seed there is no way to name the outputs, so it is an error.
	path := writeScene(t, map[string]string{scenefile.KeyConfig: "x: 1\n"})

	if _, _, err := ExtractScene(path, allLayers); err == nil {
		t.Fatal("ExtractScene: want error for scene file without seed, got nil")
	} else if !strings.Contains(err.Error(), "no embedded seed") {
		t.Errorf("error = %q, want it to mention %q", err, "no embedded seed")
	}
}

func TestExtractSceneNotPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notpng.png")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := ExtractScene(path, allLayers); err == nil {
		t.Fatal("ExtractScene: want error for non-PNG input, got nil")
	}
}
