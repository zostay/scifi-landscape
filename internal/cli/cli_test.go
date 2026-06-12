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

func TestExtractConfig(t *testing.T) {
	const want = "horizon:\n  min: 0.5\n"
	path := writeScene(t, map[string]string{scenefile.KeyConfig: want})

	got, err := ExtractConfig(path)
	if err != nil {
		t.Fatalf("ExtractConfig: %v", err)
	}
	if got != want {
		t.Errorf("ExtractConfig = %q, want %q", got, want)
	}
}

func TestExtractConfigNoConfig(t *testing.T) {
	// A scene file with a seed but no config chunk (e.g. an older scene file).
	path := writeScene(t, map[string]string{scenefile.KeySeed: "12345"})

	_, err := ExtractConfig(path)
	if err == nil {
		t.Fatal("ExtractConfig: want error for scene file without config, got nil")
	}
	if !strings.Contains(err.Error(), "no embedded config") {
		t.Errorf("ExtractConfig error = %q, want it to mention %q", err, "no embedded config")
	}
}

func TestExtractConfigNotPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notpng.png")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ExtractConfig(path); err == nil {
		t.Fatal("ExtractConfig: want error for non-PNG input, got nil")
	}
}
