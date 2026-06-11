package scenefile

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"
)

// testImage builds a small image with varied pixels so encode/decode is
// meaningfully exercised.
func testImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 16, 12))
	for y := range 12 {
		for x := range 16 {
			img.Set(x, y, color.RGBA{uint8(x * 16), uint8(y * 20), uint8(x + y), 255})
		}
	}
	return img
}

func samePixels(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	bnd := a.Bounds()
	for y := bnd.Min.Y; y < bnd.Max.Y; y++ {
		for x := bnd.Min.X; x < bnd.Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}

// TestRoundTrip is the core guarantee: a scene file's text chunks and pixels both
// survive write → read unchanged, and the result is still a valid PNG.
func TestRoundTrip(t *testing.T) {
	img := testImage()
	texts := map[string]string{
		KeySeed:      "1234567890",
		KeyConfig:    "horizon:\n  mean: 0.35\n",
		KeyGlobals:   "seed: 1234567890\nsettings:\n  time: dusk\n",
		KeySceneList: "- schema: planet.moon.v0\n  data: {}\n",
	}

	var buf bytes.Buffer
	if err := Write(&buf, img, texts); err != nil {
		t.Fatalf("write: %v", err)
	}

	gotImg, gotTexts, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !reflect.DeepEqual(gotTexts, texts) {
		t.Errorf("texts changed:\n got %#v\nwant %#v", gotTexts, texts)
	}
	if !samePixels(img, gotImg) {
		t.Error("pixels changed across scene-file round-trip")
	}
	// The scene file must still be a valid PNG to any standard decoder.
	if _, err := png.Decode(bytes.NewReader(buf.Bytes())); err != nil {
		t.Errorf("scene file is not a valid PNG: %v", err)
	}
}

// TestEmptyValuesSkipped checks that empty-valued entries are not written, so a
// partially-populated scene (e.g. seed only) does not emit blank chunks.
func TestEmptyValuesSkipped(t *testing.T) {
	var buf bytes.Buffer
	texts := map[string]string{KeySeed: "42", KeyConfig: "", KeyGlobals: ""}
	if err := Write(&buf, testImage(), texts); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadTexts(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 || got[KeySeed] != "42" {
		t.Fatalf("expected only seed chunk, got %#v", got)
	}
}

// TestPlainPNGHasNoTexts checks that an ordinary PNG (no scene-file chunks) reads
// back with an empty text map and still decodes.
func TestPlainPNGHasNoTexts(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := ReadTexts(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no texts, got %#v", got)
	}
}

// TestWriteDeterministic checks that the same image and texts produce identical
// scene-file bytes, so a recorded scene reproduces byte-for-byte.
func TestWriteDeterministic(t *testing.T) {
	img := testImage()
	texts := map[string]string{KeySceneList: "a: 1\n", KeySeed: "7", KeyConfig: "b: 2\n"}
	var b1, b2 bytes.Buffer
	if err := Write(&b1, img, texts); err != nil {
		t.Fatal(err)
	}
	if err := Write(&b2, img, texts); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1.Bytes(), b2.Bytes()) {
		t.Fatal("scene-file output is not deterministic")
	}
}

// TestKnownKeysWrittenInOrder checks the canonical chunk order (seed, config,
// globals, scene-list) regardless of map iteration order.
func TestKnownKeysWrittenInOrder(t *testing.T) {
	texts := map[string]string{KeySceneList: "d", KeyGlobals: "c", KeyConfig: "b", KeySeed: "a"}
	got := writeOrder(texts)
	want := []string{KeySeed, KeyConfig, KeyGlobals, KeySceneList}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("write order = %v, want %v", got, want)
	}
}

// TestRejectNonPNG checks that non-PNG input is rejected rather than mis-parsed.
func TestRejectNonPNG(t *testing.T) {
	if _, err := ReadTexts(bytes.NewReader([]byte("not a png"))); err == nil {
		t.Fatal("expected error for non-PNG input")
	}
}
