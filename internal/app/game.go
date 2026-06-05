package app

import (
	"fmt"
	"image/color"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/zostay/scifi-landscape/internal/seed"
)

// maxSeedLen caps typed seed input to keep the HUD readable. A seed may be any
// text; numbers are used directly and anything else is hashed (see internal/seed).
const maxSeedLen = 40

var blackRGBA = color.RGBA{0, 0, 0, 255}

// Game is the Ebiten front-end. It mirrors the controller's canvas to the
// screen every frame and translates key presses into controller actions.
type Game struct {
	ctrl    *Controller
	display *ebiten.Image
	buf     []byte

	// newSeed returns a fresh random seed for regeneration. Injectable for
	// testing; defaults to a time-based source.
	newSeed func() int64

	// editing is true while the user is typing a seed; input holds the digits
	// entered so far.
	editing bool
	input   string

	notice      string
	noticeUntil time.Time
}

// NewGame wraps a controller in an Ebiten game.
func NewGame(ctrl *Controller) *Game {
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &Game{
		ctrl:    ctrl,
		display: ebiten.NewImage(ctrl.W, ctrl.H),
		buf:     make([]byte, ctrl.W*ctrl.H*4),
		newSeed: func() int64 { return src.Int63() },
	}
}

func (g *Game) Update() error {
	if g.editing {
		g.updateSeedEntry()
		return nil
	}

	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape), inpututil.IsKeyJustPressed(ebiten.KeyQ):
		return ebiten.Termination

	case inpututil.IsKeyJustPressed(ebiten.KeyS):
		g.save()

	case inpututil.IsKeyJustPressed(ebiten.KeyN), inpututil.IsKeyJustPressed(ebiten.KeySpace):
		seed := g.newSeed()
		g.ctrl.Start(seed)
		g.flash(fmt.Sprintf("new scene — seed %d", seed))

	case inpututil.IsKeyJustPressed(ebiten.KeyR):
		seed := g.ctrl.Status().Seed
		g.ctrl.Start(seed)
		g.flash(fmt.Sprintf("replaying seed %d", seed))

	case inpututil.IsKeyJustPressed(ebiten.KeyE):
		g.editing = true
		g.input = ""
	}
	return nil
}

// updateSeedEntry handles keystrokes while the user is typing a seed: any
// printable character is appended, Backspace deletes, Enter commits, and Escape
// cancels.
func (g *Game) updateSeedEntry() {
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= ' ' && len([]rune(g.input)) < maxSeedLen {
			g.input += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && g.input != "" {
		r := []rune(g.input)
		g.input = string(r[:len(r)-1])
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		g.commitSeed()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.editing = false
		g.input = ""
		g.flash("seed entry cancelled")
	}
}

// commitSeed resolves the typed text to a seed and starts a new scene. Numeric
// text is used directly; other text is hashed (see internal/seed).
func (g *Game) commitSeed() {
	in := g.input
	g.editing = false
	g.input = ""

	if in == "" {
		g.flash("seed entry cancelled (empty)")
		return
	}
	n := seed.Resolve(in)
	g.ctrl.Start(n)
	if seed.IsNumeric(in) {
		g.flash(fmt.Sprintf("set seed %d", n))
	} else {
		g.flash(fmt.Sprintf("seed %q → %d", in, n))
	}
}

func (g *Game) save() {
	st := g.ctrl.Status()
	name := fmt.Sprintf("scifi-%d.png", st.Seed)
	if err := g.ctrl.Canvas().SavePNG(name); err != nil {
		g.flash("save failed: " + err.Error())
		return
	}
	g.flash("saved " + name)
}

func (g *Game) flash(msg string) {
	g.notice = msg
	g.noticeUntil = time.Now().Add(3 * time.Second)
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.ctrl.Canvas().Snapshot(g.buf)
	g.display.WritePixels(g.buf)
	screen.DrawImage(g.display, nil)
	g.drawHUD(screen)
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	st := g.ctrl.Status()

	state := "building: " + st.Current
	if st.Done {
		state = "done"
	}
	lines := []string{
		fmt.Sprintf("seed %d   %s   horizon %.0f%%", st.Seed, st.Time, st.Horizon*100),
		state,
		"[N]ew  [R]eplay  [E]nter seed  [S]ave PNG  [Q]uit",
	}
	switch {
	case g.editing:
		lines = append(lines, fmt.Sprintf("seed> %s_   [Enter] apply  [Esc] cancel", g.input))
	case g.notice != "" && time.Now().Before(g.noticeUntil):
		lines = append(lines, g.notice)
	}

	// Translucent backdrop sized to the widest line so text stays legible.
	const pad, lineH, glyphW = 6, 16, 6
	maxLen := 0
	for _, ln := range lines {
		if len(ln) > maxLen {
			maxLen = len(ln)
		}
	}
	boxW := float32(maxLen*glyphW + pad*2)
	boxH := float32(lineH*len(lines) + pad*2)
	vector.FillRect(screen, 0, 0, boxW, boxH, color.RGBA{0, 0, 0, 150}, false)
	for i, ln := range lines {
		ebitenutil.DebugPrintAt(screen, ln, pad, pad+i*lineH)
	}
}

// Layout fixes the logical screen to the canvas size so WritePixels lines up
// 1:1 with the display image.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.ctrl.W, g.ctrl.H
}
