package app

import (
	"context"
	"os"
	"sync"

	"github.com/zostay/scifi-landscape/internal/canvas"
	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/scene"
	"github.com/zostay/scifi-landscape/internal/scenefile"
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
	config       config.Config
	canvas       *canvas.Canvas

	mu     sync.Mutex
	status Status
	// globals are the derived scene-wide values of the current scene, kept so a
	// saved scene file can embed them. Replaced on each new generation.
	globals scene.Globals
	cancel  context.CancelFunc
	done    chan struct{} // closed when the current run goroutine exits
}

// NewController creates a controller for a w x h scene. cfg is the (complete)
// configuration that shapes every scene; it is locked for the controller's
// lifetime, as the app's configuration does not change after start. timeOverride
// forces the time of day when non-empty (see scene.ParseTimeOfDay); otherwise it
// is derived from each seed.
func NewController(w, h int, timeOverride string, cfg config.Config) *Controller {
	return &Controller{
		W:            w,
		H:            h,
		timeOverride: timeOverride,
		config:       cfg,
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

	// The director turns seed + config into the scene-wide globals.
	globals := c.director().Direct(c.config, seed, c.timeOverride, c.W, c.H)
	settings := globals.Settings

	c.mu.Lock()
	c.globals = globals
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

// director resolves the director named by the config, falling back to the default
// if the configured key is unknown.
func (c *Controller) director() scene.Director {
	if dirs := c.config.Algorithms.Directors; len(dirs) > 0 {
		if d, ok := scene.DirectorByName(dirs[0]); ok {
			return d
		}
	}
	return scene.DefaultDirector()
}

// SaveSceneFile writes the current scene to name as a scene file: the rendered
// PNG plus the embedded seed, config, and globals, so the scene can be reproduced
// later. (The scene list is omitted until the element pipeline is fully migrated
// to entities; see scene.SceneTexts.)
func (c *Controller) SaveSceneFile(name string) error {
	c.mu.Lock()
	seed := c.status.Seed
	cfg := c.config
	g := c.globals
	c.mu.Unlock()

	texts, err := scene.SceneTexts(seed, cfg, &g, nil)
	if err != nil {
		return err
	}
	img := c.canvas.SnapshotImage()

	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := scenefile.Write(f, img, texts); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (c *Controller) setCurrent(name string) {
	c.mu.Lock()
	c.status.Current = name
	c.mu.Unlock()
}
