package scene

import (
	"bytes"
	"image"
	"reflect"
	"strconv"
	"testing"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/scenefile"
)

// TestSceneFilePayloadRoundTrip is the Phase 5 capstone: the four reproducibility
// layers (seed, config, globals, scene list) assembled from real scene types are
// embedded in a scene file, read back, and parsed — and every layer reconstructs
// to the same value (and the scene list re-renders to the same pixels). This is
// the data half of "reproduce a scene from a scene file"; the orchestration that
// picks which layer to start from is wired in the app/CLI.
func TestSceneFilePayloadRoundTrip(t *testing.T) {
	const w, h = 480, 270
	const seed = int64(42)

	cfg := config.DefaultConfig()
	globals := DefaultDirector().Direct(cfg, seed, "", w, h)

	gen := planetsTestContext(seed, w, h)
	list, err := (&Planets{}).Generate(gen)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("seed 42 produced no planets; pick another seed")
	}
	// Render the scene list so the scene file carries a real picture.
	if err := (&Planets{}).RenderList(gen, list); err != nil {
		t.Fatalf("render: %v", err)
	}
	pix := make([]byte, w*h*4)
	gen.Canvas.Snapshot(pix)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, pix)

	cfgY, err := cfg.Marshal()
	if err != nil {
		t.Fatalf("config marshal: %v", err)
	}
	gY, err := globals.Marshal()
	if err != nil {
		t.Fatalf("globals marshal: %v", err)
	}
	slY, err := MarshalSceneList(list)
	if err != nil {
		t.Fatalf("scene list marshal: %v", err)
	}
	texts := map[string]string{
		scenefile.KeySeed:      strconv.FormatInt(seed, 10),
		scenefile.KeyConfig:    string(cfgY),
		scenefile.KeyGlobals:   string(gY),
		scenefile.KeySceneList: string(slY),
	}

	var buf bytes.Buffer
	if err := scenefile.Write(&buf, img, texts); err != nil {
		t.Fatalf("scene file write: %v", err)
	}
	got, err := scenefile.ReadTexts(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("scene file read: %v", err)
	}

	// Seed layer.
	if s, err := strconv.ParseInt(got[scenefile.KeySeed], 10, 64); err != nil || s != seed {
		t.Errorf("seed layer = %q (%v), want %d", got[scenefile.KeySeed], err, seed)
	}
	// Config layer.
	cfg2, err := config.Load([]byte(got[scenefile.KeyConfig]))
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if !reflect.DeepEqual(cfg, cfg2) {
		t.Errorf("config layer changed across scene file")
	}
	// Globals layer.
	g2, err := UnmarshalGlobals([]byte(got[scenefile.KeyGlobals]))
	if err != nil {
		t.Fatalf("globals load: %v", err)
	}
	if g2 != globals {
		t.Errorf("globals layer changed:\n got %+v\nwant %+v", g2, globals)
	}
	// Scene-list layer: must re-render to the same pixels.
	list2, err := UnmarshalSceneList([]byte(got[scenefile.KeySceneList]))
	if err != nil {
		t.Fatalf("scene list load: %v", err)
	}
	if a, b := renderListHash(t, seed, w, h, list), renderListHash(t, seed, w, h, list2); a != b {
		t.Errorf("scene list layer re-renders differently:\n got  %s\n want %s", b, a)
	}
}
