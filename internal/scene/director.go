package scene

import (
	"fmt"
	"math"

	"gopkg.in/yaml.v3"

	"github.com/zostay/scifi-landscape/internal/config"
	"github.com/zostay/scifi-landscape/internal/gfx"
)

// Globals are the complete, scene-wide values a Director derives from a seed and
// a configuration, before any entity is generated. Unlike a configuration,
// globals are always complete — no field is ever missing or assumed.
//
// The globals are the master seed, the scene dimensions, the derived Settings
// (time of day, horizon, twinkle, star density, and dominant-star lighting), and
// the scene-wide sky/ground gradients that renderers read. Capturing the gradients
// here (rather than re-deriving them from the seed at render time) lets a recorded
// scene list redraw the same image without the seed. The per-element generation
// streams are still derived from Seed (see deriveRng), and the ocean/land model
// remains working state rebuilt from the seed in Build (no renderer reads it — it
// is captured per-scene in the water entity). Existing fields are never changed,
// only added to, so a recorded globals keeps reproducing its scene.
type Globals struct {
	// Seed is the scene's master seed; per-element streams derive from it.
	Seed int64 `yaml:"seed"`
	// W and H are the scene dimensions in pixels.
	W int `yaml:"w"`
	H int `yaml:"h"`
	// Settings are the derived scene-wide look choices.
	Settings Settings `yaml:"settings"`
	// SkyGradient is the horizon→top sky color gradient, read by the sky, planet,
	// and city-dome renderers.
	SkyGradient gfx.Gradient `yaml:"skyGradient"`
	// GroundGradient is the horizon→foreground ground color gradient, read by the
	// ground renderer; GroundVariable reports whether the multi-color "variable"
	// ground mode was chosen (it shapes the gradient).
	GroundGradient gfx.Gradient `yaml:"groundGradient"`
	GroundVariable bool         `yaml:"groundVariable"`

	// Height is the viewer's vantage point over the ground plane (see HeightMode),
	// read by the v1 ground/cities/water renderers to widen the low-mode perspective.
	// Its zero value is High, the original look, so a globals file predating this
	// field (and the scene.v0 director, which never sets it) means "as before."
	Height HeightMode `yaml:"height"`
	// Perspective holds the resolved low-mode ground-plane parameters. They are
	// meaningful only when Height == Low: the director resolves them from the config
	// and the horizon (so the "looking up" amplification is baked in here, not in the
	// renderers), and the v1 renderers read them to widen the ground plane. In High
	// mode they stay zero and unused (the v1 renderers delegate to the v0 look). They
	// live in the globals so a recorded scene reproduces the perspective from the
	// globals alone, without the config.
	Perspective Perspective `yaml:"perspective"`

	// MountainRanges holds the resolved base parameters for the extra mountain ranges
	// (the mountainranges.v0 element): how likely and how many ranges, how far down
	// the ground their feet may sit, and the mean/σ of the height and smoothness each
	// range varies around. The director resolves these per vantage (more, deeper
	// ranges in high mode; a thin strip in low mode). Its zero value means "no extra
	// ranges", so a globals file predating this field — and the scene.v0 director,
	// which never sets it — reproduces exactly as before.
	MountainRanges MountainRangeBase `yaml:"mountainRanges"`

	// MountainRugged selects the mountains' shading style: false (the default and zero
	// value) is the soft conical, eroded-slope look layered on the horizontal banded
	// gradient; true is the alternate craggier "rugged" rock. The director rolls it per
	// scene from MountainConfig.RuggedChance. It applies to both the horizon range and
	// the extra ranges; its zero value reproduces the default for old globals.
	MountainRugged bool `yaml:"mountainRugged"`

	// Mist holds the resolved ground-mist parameters (the per-scene presence roll and
	// the fog's shape). The mountainranges.v0 element reads it; the mist still only
	// appears when the scene also has foreground ranges. Its zero value (Present false)
	// means no mist, so old globals and the scene.v0 director reproduce as before.
	Mist MistBase `yaml:"mist"`

	// Bushes holds the resolved base parameters for the scattered ground bushes (the
	// bushes.v0 element): how likely/how many, how their size grows with nearness for
	// the rolled vantage, and the shape/burial/shading ranges each bush varies within.
	// The director resolves these per vantage (many small bushes in high; fewer, larger
	// bushes in low). Its zero value (Chance 0) means no bushes, so a globals file
	// predating this field — and the scene.v0 director, which never sets it —
	// reproduces exactly as before.
	Bushes BushesBase `yaml:"bushes"`
	// BushGradient is the scene-wide bush color gradient (an independent palette, not the
	// ground), read by the bushes.v0 renderer: each bush samples one position along it for
	// its base color. The director derives it; its zero value (an empty gradient) means
	// bushes fall back to black, but the scene.v0 director never produces bushes so it is
	// unaffected.
	BushGradient gfx.Gradient `yaml:"bushGradient,omitempty"`
}

// Perspective is the resolved set of low-mode ("ground-level") ground-plane
// parameters the v1 renderers consume. GroundNearCell (px texture cell size at the
// nearest ground), GroundBias (horizon recession sharpness), and GroundGamma (the
// depth-falloff exponent, already steepened for how far the horizon sits from eye
// level) define the base terrain's perspective projection. GroundContrast scales the
// texture's light/dark. The ocean fields (ShorePersp/ShoreBias/LandDist/WaveNear/
// WaveOctaves) are documented on their struct fields below. CityBandFrac caps the
// city's depth band (as a fraction of the ground height) so the city stays pinned
// far-off near the horizon. The texture is two layers (see PerspectiveConfig):
// GroundNearCell-pixel macro blobs plus GroundDetailCell-pixel detail grain at
// GroundDetailAmt strength.
type Perspective struct {
	GroundNearCell   float64 `yaml:"groundNearCell"`
	GroundDetailCell float64 `yaml:"groundDetailCell"`
	GroundDetailAmt  float64 `yaml:"groundDetailAmt"`
	GroundBias       float64 `yaml:"groundBias"`
	GroundGamma      float64 `yaml:"groundGamma"`
	GroundContrast   float64 `yaml:"groundContrast"`
	// ShorePersp and ShoreBias bend the swell by perspective (crests bunching toward the
	// horizon); LandDist scales how far the geometric coastline sits (larger pushes it
	// toward the horizon for the ground-level look); WaveNear/WaveOctaves shape the
	// swell; CityBandFrac pins the city near the horizon. These are set in both height
	// modes (more strongly in low), so water.v1 and cities.v1 always render with them.
	ShorePersp   float64 `yaml:"shorePersp"`
	ShoreBias    float64 `yaml:"shoreBias"`
	LandDist     float64 `yaml:"landDist"`
	WaveNear     float64 `yaml:"waveNear"`
	WaveOctaves  int     `yaml:"waveOctaves"`
	CityBandFrac float64 `yaml:"cityBandFrac"`
}

// MountainRangeBase is the resolved, scene-wide base for the extra mountain ranges
// the mountainranges.v0 element draws. The director resolves the per-vantage values
// (Chance, CountMax, BaselineMaxFrac) from the config and the rolled height, and
// carries the height/smoothness base the element varies each range around. Living in
// the globals means a recorded scene reproduces the ranges without the config. The
// zero value (Chance/CountMax 0) means no extra ranges, so the v0 director — which
// never sets this — is unaffected.
type MountainRangeBase struct {
	// Chance is the probability the scene has any extra ranges at all.
	Chance float64 `yaml:"chance"`
	// CountMax is the most extra ranges to draw (resolved per vantage); the element
	// rolls a count in 1..CountMax once the Chance roll passes.
	CountMax int `yaml:"countMax"`
	// BaselineMaxFrac is how far below the horizon the nearest range's foot may sit,
	// as a fraction of the ground height (the feet spread from the horizon to here).
	BaselineMaxFrac float64 `yaml:"baselineMaxFrac"`
	// HeightMeanFrac and HeightStdFrac are the mean and σ of a range's peak height as
	// a fraction of the horizon height in pixels.
	HeightMeanFrac float64 `yaml:"heightMeanFrac"`
	HeightStdFrac  float64 `yaml:"heightStdFrac"`
	// SmoothnessMean and SmoothnessStd are the mean and σ of a range's smoothness.
	SmoothnessMean float64 `yaml:"smoothnessMean"`
	SmoothnessStd  float64 `yaml:"smoothnessStd"`
	// BaselineJitterFrac jitters each foot row (fraction of the ground height) so the
	// ranges are not perfectly evenly spaced.
	BaselineJitterFrac float64 `yaml:"baselineJitterFrac"`
}

// BushesBase is the resolved, scene-wide base for the scattered ground bushes the
// bushes.v0 element draws. The director resolves the per-vantage values (Count,
// MaxSizeFrac, SizeGamma) from the config and the rolled height, and carries the
// shape/burial/texture ranges the element varies each bush within. Living in the
// globals means a recorded scene reproduces the bushes without the config. The zero
// value (Chance 0) means no bushes, so the v0 director — which never sets this — is
// unaffected.
type BushesBase struct {
	// Chance is the probability the scene has any bushes at all.
	Chance float64 `yaml:"chance"`
	// Count is the bush count at the reference width (480px), resolved per vantage;
	// the element scales it by the actual width (not area).
	Count int `yaml:"count"`
	// MinSizeFrac is a far bush's diameter as a fraction of the scene width; MaxSizeFrac
	// is the nearest bush's diameter (resolved per vantage). SizeGamma is the depth→size
	// exponent (size grows as nearness^SizeGamma from min to max).
	MinSizeFrac float64 `yaml:"minSizeFrac"`
	MaxSizeFrac float64 `yaml:"maxSizeFrac"`
	SizeGamma   float64 `yaml:"sizeGamma"`
	// DepthBias warps the uniform depth draw (nearness = u^DepthBias): > 1 pushes bushes
	// toward the far distance, thinning the near foreground where they grow large.
	DepthBias float64 `yaml:"depthBias"`
	// SquashMin/SquashMax bound each bush's minor/major axis ratio (a squashed clump).
	SquashMin float64 `yaml:"squashMin"`
	SquashMax float64 `yaml:"squashMax"`
	// BuryMin/BuryMax bound the fraction of a bush buried below the ground contact line.
	BuryMin float64 `yaml:"buryMin"`
	BuryMax float64 `yaml:"buryMax"`
	// SizeJitter is the per-bush random size variation (± this fraction).
	SizeJitter float64 `yaml:"sizeJitter"`
	// Lumpiness is how strongly the elliptical outline is perturbed into a lopsided shape.
	Lumpiness float64 `yaml:"lumpiness"`
	// Ambient is the shadow-side fill light for the bush form-shading.
	Ambient float64 `yaml:"ambient"`
}

// MistBase is the resolved, scene-wide ground-mist state the mountainranges.v0 element
// reads. Present is the director's per-scene roll (the mist still needs foreground
// ranges to actually appear); the fractions shape the fog. Its zero value means no
// mist.
type MistBase struct {
	// Present is whether this scene rolled mist on.
	Present bool `yaml:"present"`
	// FadeUpFrac is how far the mist fades up a range's slopes, as a fraction of the sky.
	FadeUpFrac float64 `yaml:"fadeUpFrac"`
	// LowFadeFrac is the distance over which the opaque mist fades back out below the
	// mountains where none continues (e.g. over open water, and below the front range at
	// the low vantage), as a fraction of the ground height.
	LowFadeFrac float64 `yaml:"lowFadeFrac"`
	// OceanFadeFrac is how far the mist reaches over open water before fading to nothing,
	// as a fraction of the scene width.
	OceanFadeFrac float64 `yaml:"oceanFadeFrac"`
}

// Marshal serializes the globals to YAML for a scene file's globals.yaml. Globals
// are always complete, so the output fully captures the derived scene-wide state.
func (g Globals) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(g)
	if err != nil {
		return nil, fmt.Errorf("globals: %w", err)
	}
	return data, nil
}

// UnmarshalGlobals parses globals from YAML produced by Globals.Marshal.
func UnmarshalGlobals(data []byte) (Globals, error) {
	var g Globals
	if err := yaml.Unmarshal(data, &g); err != nil {
		return Globals{}, fmt.Errorf("globals: %w", err)
	}
	return g, nil
}

// Director turns a seed and a complete configuration into the scene's globals. A
// Director has no side effects: the same seed and config must always yield
// identical globals, so the globals alone (with the generators named by the
// config) can reproduce the scene list.
//
// Directors are versioned and frozen once released: a behavioral change is made
// as a new director (e.g. "scene.v1"), never by editing an existing one, so old
// seeds keep their meaning. The config names which director builds a scene.
type Director interface {
	// Name is the director's versioned registry key (e.g. "scene.v0").
	Name() string
	// Direct derives the globals for a scene of size w x h from the seed and
	// config. timeOverride, when it names a time of day, forces that time without
	// disturbing the random stream (see Settings).
	Direct(cfg config.Config, seed int64, timeOverride string, w, h int) Globals
}

// sceneDirectorV0 is the original director: it derives the scene-wide Settings
// the app has always used, now parameterized by configuration instead of
// hardcoded constants. FROZEN: do not change its draw order or math; add a
// sceneDirectorV1 for new behavior.
type sceneDirectorV0 struct{}

func (sceneDirectorV0) Name() string { return "scene.v0" }

func (sceneDirectorV0) Direct(cfg config.Config, seed int64, timeOverride string, w, h int) Globals {
	rng := deriveRng(seed, "settings")

	// The random stream is consumed in a fixed order regardless of overrides, so
	// the seed stays reproducible. See the original NewSettings for the rationale.
	tod := TimeOfDay(rng.Intn(3))

	frac := cfg.Horizon.Mean + rng.NormFloat64()*cfg.Horizon.Std
	if frac < cfg.Horizon.Min {
		frac = cfg.Horizon.Min
	}
	if frac > cfg.Horizon.Max {
		frac = cfg.Horizon.Max
	}

	twinkle := min(math.Abs(rng.NormFloat64())*cfg.Twinkle.Std, cfg.Twinkle.Max)

	density := math.Exp(min(max(rng.NormFloat64()+cfg.StarDensity.Bias, -cfg.StarDensity.Clamp), cfg.StarDensity.Clamp) * cfg.StarDensity.Std)

	lightColor := gfx.HSV{H: rng.Float64() * 360, S: rnd(rng, cfg.Lighting.ColorSatMin, cfg.Lighting.ColorSatMax), V: cfg.Lighting.ColorValue}.RGB()
	lightBright := rnd(rng, cfg.Lighting.BrightMin, cfg.Lighting.BrightMax)
	lightPhase := math.Sqrt(rng.Float64())
	lightAmbient := cfg.Lighting.AmbientBase + cfg.Lighting.AmbientScale*math.Pow(rng.Float64(), cfg.Lighting.AmbientPow)

	if override, ok := ParseTimeOfDay(timeOverride); ok {
		tod = override
	}

	// Horizon is measured from the bottom, so convert to a row from the top.
	y := min(max(int((1-frac)*float64(h)), 1), h-1)

	// Derive the scene-wide sky and ground gradients here so they are part of the
	// globals (and recorded in a scene file), rather than rebuilt from the seed at
	// render time. Each draws from its own stream off the seed, independent of the
	// "settings" stream above; these are the exact calls Scene.Build once made, so
	// the output is unchanged.
	skyGrad := buildSkyGradient(deriveRng(seed, "sky-gradient"), tod)
	gg := deriveRng(seed, "ground-gradient")
	groundVar := gg.Float64() < groundVariableChance
	groundGrad := buildGroundGradient(gg, tod, groundVar)

	return Globals{
		Seed: seed,
		W:    w,
		H:    h,
		Settings: Settings{
			Time:            tod,
			Horizon:         frac,
			HorizonY:        y,
			TwinkleAngle:    twinkle,
			StarDensity:     density,
			LightColor:      lightColor,
			LightBrightness: lightBright,
			LightPhase:      lightPhase,
			LightAmbient:    lightAmbient,
		},
		SkyGradient:    skyGrad,
		GroundGradient: groundGrad,
		GroundVariable: groundVar,
	}
}

// sceneDirectorV1 derives everything sceneDirectorV0 does and additionally sets the
// scene-wide Height global (the ground-plane vantage point). It embeds the v0
// director and calls through to it, so every field v0 derived — the Settings and
// the sky/ground gradients — is byte-identical; v1 only adds Height. The height is
// rolled from a fresh, independent random stream ("perspective"), so adding it does
// not disturb any existing per-element stream: a seed that rolls High therefore
// reproduces exactly what scene.v0 produced. FROZEN once released: add a v2 for new
// behavior.
type sceneDirectorV1 struct{ sceneDirectorV0 }

func (sceneDirectorV1) Name() string { return "scene.v1" }

func (d sceneDirectorV1) Direct(cfg config.Config, seed int64, timeOverride string, w, h int) Globals {
	g := d.sceneDirectorV0.Direct(cfg, seed, timeOverride, w, h)
	// Roll the vantage point on its own stream so existing globals are unaffected.
	g.Height = High
	if deriveRng(seed, "perspective").Float64() < cfg.Perspective.LowChance {
		g.Height = Low
	}
	// The perspective is resolved for both modes: the ground/city transforms apply
	// only at the low vantage, but the water shore/wave perspective applies in both
	// (more strongly in low), so water.v1 always has its parameters.
	g.Perspective = resolvePerspective(cfg.Perspective, g.Settings.Horizon, g.Height)
	// Resolve the extra mountain ranges' base parameters for the rolled vantage (more,
	// deeper ranges in high; a thin strip in low). This is purely additive — a fresh
	// global the mountainranges.v0 element reads — so existing per-element streams and
	// outputs are undisturbed.
	g.MountainRanges = resolveMountainRanges(cfg.Mountains, g.Height)
	// Roll the mountain shading style on its own stream so it disturbs no existing
	// per-element stream; false (soft conical) is the default and zero value.
	g.MountainRugged = deriveRng(seed, "mountain-style").Float64() < cfg.Mountains.RuggedChance
	// Roll the ground mist on its own stream; it only renders when the scene also has
	// foreground ranges (decided in the element).
	g.Mist = resolveMist(cfg.Mist, seed)
	// Resolve the bushes' base parameters for the rolled vantage (many small bushes in
	// high; fewer, larger ones in low). Purely additive — a fresh global the bushes.v0
	// element reads — so existing per-element streams and outputs are undisturbed.
	g.Bushes = resolveBushes(cfg.Bushes, g.Height)
	// Derive the scene's independent bush color gradient on its own stream, so each bush
	// can sample a base color from it. Independent stream → existing streams undisturbed.
	g.BushGradient = buildBushGradient(deriveRng(seed, "bush-gradient"), g.Settings.Time)
	return g
}

// resolveBushes turns the bushes config and the rolled height into the scene-wide base
// parameters the bushes.v0 element reads. The count, nearest-bush size, and depth→size
// exponent are chosen by vantage — a high, elevated view scatters many small bushes; a
// ground-level view places fewer but much larger bushes nearer the viewer. The
// shape/burial/texture ranges are vantage-independent; the element varies each bush
// within them.
func resolveBushes(rc config.BushesConfig, height HeightMode) BushesBase {
	count, maxSize, gamma, depthBias := rc.CountHigh, rc.MaxSizeFracHigh, rc.SizeGammaHigh, rc.DepthBiasHigh
	if height == Low {
		count, maxSize, gamma, depthBias = rc.CountLow, rc.MaxSizeFracLow, rc.SizeGammaLow, rc.DepthBiasLow
	}
	return BushesBase{
		Chance:      rc.Chance,
		Count:       count,
		MinSizeFrac: rc.MinSizeFrac,
		MaxSizeFrac: maxSize,
		SizeGamma:   gamma,
		DepthBias:   depthBias,
		SquashMin:   rc.SquashMin,
		SquashMax:   rc.SquashMax,
		BuryMin:     rc.BuryMin,
		BuryMax:     rc.BuryMax,
		SizeJitter:  rc.SizeJitter,
		Lumpiness:   rc.Lumpiness,
		Ambient:     rc.Ambient,
	}
}

// resolveMist rolls the per-scene mist presence on its own stream and carries the
// fog-shape fractions into the globals. The roll is independent of vantage; how the
// mist behaves (whole-scene vs near-the-mountains) is decided at render time from the
// height global.
func resolveMist(mc config.MistConfig, seed int64) MistBase {
	return MistBase{
		Present:       deriveRng(seed, "mist").Float64() < mc.Chance,
		FadeUpFrac:    mc.FadeUpFrac,
		LowFadeFrac:   mc.LowFadeFrac,
		OceanFadeFrac: mc.OceanFadeFrac,
	}
}

// resolveMountainRanges turns the mountain config and the rolled height into the
// scene-wide base parameters the mountainranges.v0 element reads. The count cap and
// baseline reach are chosen by vantage — a high, elevated view shows more receding
// ridgelines spread further down the ground; a ground-level view keeps a range or
// two pinned in a thin strip at the horizon. The height/smoothness base and the
// jitter are vantage-independent; the element varies each range around them.
func resolveMountainRanges(mc config.MountainConfig, height HeightMode) MountainRangeBase {
	countMax, baselineMax := mc.CountMaxHigh, mc.BaselineFracHigh
	if height == Low {
		countMax, baselineMax = mc.CountMaxLow, mc.BaselineFracLow
	}
	return MountainRangeBase{
		Chance:             mc.Chance,
		CountMax:           countMax,
		BaselineMaxFrac:    baselineMax,
		HeightMeanFrac:     mc.HeightFrac,
		HeightStdFrac:      mc.HeightStd,
		SmoothnessMean:     mc.Smoothness,
		SmoothnessStd:      mc.SmoothnessStd,
		BaselineJitterFrac: mc.BaselineJitter,
	}
}

// resolvePerspective turns the perspective config, the scene's horizon, and the
// rolled height into the concrete parameters the v1 renderers read. The ground
// stretch is amplified by how far the horizon sits below eye level (HorizonPivot): a
// horizon low on screen (lots of sky) reads as "looking up", which intensifies the
// foreshortening. The shore-perspective strength and near-wave scale are chosen by
// mode — mild in high, strong in low — so the ocean gets perspective shorelines and
// larger near waves at both vantages.
func resolvePerspective(pc config.PerspectiveConfig, horizon float64, height HeightMode) Perspective {
	pitch := 0.0
	if pc.HorizonPivot > 0 {
		pitch = clamp01((pc.HorizonPivot - horizon) / pc.HorizonPivot)
	}
	shore, waveNear, dist := pc.ShorePerspHigh, pc.WaveNearHigh, pc.LandDistHigh
	if height == Low {
		shore, waveNear, dist = pc.ShorePerspLow, pc.WaveNearLow, pc.LandDistLow
	}
	return Perspective{
		GroundNearCell:   pc.GroundNearCell,
		GroundDetailCell: pc.GroundDetailCell,
		GroundDetailAmt:  pc.GroundDetailAmt,
		GroundBias:       pc.GroundBias,
		GroundGamma:      1 + pc.HorizonGain*pitch, // looking up steepens the depth falloff
		GroundContrast:   pc.GroundContrast,
		ShorePersp:       shore,
		ShoreBias:        pc.ShoreBias,
		LandDist:         dist,
		WaveNear:         waveNear,
		WaveOctaves:      pc.WaveOctaves,
		CityBandFrac:     pc.CityBandCap,
	}
}

// directors is the registry of available directors, keyed by versioned name.
// Existing entries are frozen; new behavior is added as a new key.
var directors = map[string]Director{
	"scene.v0": sceneDirectorV0{},
	"scene.v1": sceneDirectorV1{},
}

// DirectorByName returns the registered director for a config key, or false if
// no such director exists.
func DirectorByName(name string) (Director, bool) {
	d, ok := directors[name]
	return d, ok
}

// DirectorKeys returns the registered director keys (unordered).
func DirectorKeys() []string {
	out := make([]string, 0, len(directors))
	for k := range directors {
		out = append(out, k)
	}
	return out
}

// DefaultDirector returns the director used when none is otherwise selected. It is
// the latest director (scene.v1), matching the default config, so bare/fallback use
// gets the current behavior; scene.v0 stays registered for old configs.
func DefaultDirector() Director { return sceneDirectorV1{} }
