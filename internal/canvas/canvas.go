// Package canvas provides a concurrency-safe RGBA drawing surface.
//
// The scene is built on a background goroutine while the UI goroutine reads
// the current pixels every frame to display the construction in progress, so
// all access is guarded by a single RWMutex.
package canvas

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"sync"
)

// Canvas is a fixed-size RGBA image safe for concurrent draw/read access.
type Canvas struct {
	mu      sync.RWMutex
	img     *image.RGBA
	version uint64 // bumped on every mutation, so readers can skip unchanged frames
	W       int
	H       int
}

// New returns a w x h canvas initialized to opaque black.
func New(w, h int) *Canvas {
	c := &Canvas{
		img: image.NewRGBA(image.Rect(0, 0, w, h)),
		W:   w,
		H:   h,
	}
	c.Clear(color.RGBA{0, 0, 0, 255})
	return c
}

// Clear fills the entire canvas with a single color.
func (c *Canvas) Clear(col color.RGBA) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pix := c.img.Pix
	for i := 0; i < len(pix); i += 4 {
		pix[i] = col.R
		pix[i+1] = col.G
		pix[i+2] = col.B
		pix[i+3] = col.A
	}
	c.version++
}

// Draw runs fn with exclusive access to the underlying image. Callers should
// keep the work inside fn bounded (e.g. one animation band) so the UI
// goroutine is not starved of read access.
func (c *Canvas) Draw(fn func(img *image.RGBA)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c.img)
	c.version++
}

// Version returns a counter that increases on every Clear/Draw. The UI can
// compare it across frames to avoid re-snapshotting and re-uploading the canvas
// when nothing has changed (e.g. once a scene has finished building).
func (c *Canvas) Version() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.version
}

// Snapshot copies the current pixels into dst, which must be exactly
// W*H*4 bytes. It is used to feed the display each frame.
func (c *Canvas) Snapshot(dst []byte) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copy(dst, c.img.Pix)
}

// SnapshotImage returns a copy of the current canvas as an *image.RGBA. The copy
// is independent of the canvas, so it stays stable while the canvas keeps
// mutating — used to hand a stable image to the scene-file writer.
func (c *Canvas) SnapshotImage() *image.RGBA {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := image.NewRGBA(c.img.Bounds())
	copy(out.Pix, c.img.Pix)
	return out
}

// SavePNG writes the current canvas to the named file as a PNG.
func (c *Canvas) SavePNG(name string) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := c.encode(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (c *Canvas) encode(w io.Writer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return png.Encode(w, c.img)
}
