package app

import (
	"context"
	"sync"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/scene"
)

// Status is an immutable snapshot of the current generation, read by the UI.
type Status struct {
	Seed         int64
	Time         scene.TimeOfDay
	Horizon      float64
	TwinkleAngle float64
	StarDensity  float64
	Current      string // element being rendered, or "" when idle
	Done         bool
}

// Controller owns the canvas and the background goroutine that builds a scene.
// It can be restarted with a new (or the same) seed, cancelling any in-flight
// generation. All exported methods are safe to call from the UI goroutine.
type Controller struct {
	W, H         int
	timeOverride string
	canvas       *canvas.Canvas

	mu     sync.Mutex
	status Status
	cancel context.CancelFunc
	done   chan struct{} // closed when the current run goroutine exits
}

// NewController creates a controller for a w x h scene. timeOverride forces the
// time of day when non-empty (see scene.ParseTimeOfDay); otherwise it is
// derived from each seed.
func NewController(w, h int, timeOverride string) *Controller {
	return &Controller{
		W:            w,
		H:            h,
		timeOverride: timeOverride,
		canvas:       canvas.New(w, h),
	}
}

// Canvas returns the shared drawing surface.
func (c *Controller) Canvas() *canvas.Canvas { return c.canvas }

// Status returns a snapshot of the current generation state.
func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// Start cancels any running generation and begins a new one with seed.
func (c *Controller) Start(seed int64) {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	prevDone := c.done
	c.mu.Unlock()

	// Wait for the previous run to fully stop before reusing the canvas.
	if prevDone != nil {
		<-prevDone
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	c.mu.Lock()
	c.cancel = cancel
	c.done = done
	c.status = Status{Seed: seed}
	c.mu.Unlock()

	go c.run(ctx, seed, done)
}

func (c *Controller) run(ctx context.Context, seed int64, done chan struct{}) {
	defer close(done)

	settings := scene.NewSettings(seed, c.timeOverride, c.H)

	c.mu.Lock()
	c.status.Time = settings.Time
	c.status.Horizon = settings.Horizon
	c.status.TwinkleAngle = settings.TwinkleAngle
	c.status.StarDensity = settings.StarDensity
	c.mu.Unlock()

	c.canvas.Clear(blackRGBA)

	sc := scene.New(settings)
	if err := sc.Build(ctx, c.canvas, seed, c.W, c.H, c.setCurrent); err != nil {
		return // cancelled
	}

	c.mu.Lock()
	c.status.Current = ""
	c.status.Done = true
	c.mu.Unlock()
}

func (c *Controller) setCurrent(name string) {
	c.mu.Lock()
	c.status.Current = name
	c.mu.Unlock()
}
