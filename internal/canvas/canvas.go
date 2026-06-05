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
	mu  sync.RWMutex
	img *image.RGBA
	W   int
	H   int
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
}

// Draw runs fn with exclusive access to the underlying image. Callers should
// keep the work inside fn bounded (e.g. one animation band) so the UI
// goroutine is not starved of read access.
func (c *Canvas) Draw(fn func(img *image.RGBA)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c.img)
}

// Snapshot copies the current pixels into dst, which must be exactly
// W*H*4 bytes. It is used to feed the display each frame.
func (c *Canvas) Snapshot(dst []byte) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copy(dst, c.img.Pix)
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
