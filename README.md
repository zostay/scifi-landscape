# scifi-landscape

A procedural sci-fi landscape generator. Scenes are drawn one element at a
time — the construction is animated on screen so you can watch it build — and
every scene is fully determined by a single random **seed**, so any scene can
be reproduced exactly just by knowing its seed. The finished image stays on
screen and can be saved to a PNG.

This is a modern reimagining of an old 256-color graphics experiment.

## Running

```sh
go run .                      # random seed, 1280x720 window
go run . -s 12345             # reproduce a specific scene
go run . -s mars              # any text works too (hashed to a seed)
go run . -t dusk              # force the time of day (midday | dusk | twilight)
go run . -w 1920 --height 1080 # custom size
go run . -c my.yaml           # tune generation with a config file (see Configuration)
go run . -f scene.png         # reproduce a saved scene file (its seed + config)
```

Flags use POSIX `-s`/`--long` style; run `go run . --help` for the full list.
Each has a long form too (`--seed`, `--time`, `--width`, `--height`, `--config`,
`--from`); `--height` has no short form because `-h` is `--help`.

On macOS, ebiten's Metal driver is noisy: it triggers a wall of cgo deprecation
warnings at build time, and prints benign `[CAMetalLayer nextDrawable] returning
nil` lines to stderr at runtime. The `Makefile` quiets both — it sets
`CGO_CFLAGS=-Wno-deprecated-declarations` for the build warnings and filters the
Metal log lines out of `make run`'s stderr (real errors still pass through). So
prefer it for day-to-day use (pass flags via `ARGS`):

```sh
make run                      # go run . , warnings suppressed
make run ARGS="-s mars -t dusk"
make render ARGS="-s 7 -o scene.png"
make build                    # build the ./scifi-landscape binary
make test
```

A seed can be **a number or any text**. A plain integer (within int64 range) is
used directly; anything else — a word, a phrase, a too-big number — is hashed to
a stable seed, so `-s mars` always yields the same scene. The resolved seed
is printed to the terminal on startup and shown on the on-screen HUD.

### Controls

| Key       | Action                                    |
|-----------|-------------------------------------------|
| `N` / `Space` | Generate a new random scene           |
| `R`       | Replay the current seed (re-animate)      |
| `E`       | Enter a seed (type a number or text, `Enter` to apply, `Esc` to cancel) |
| `S`       | Save the current scene to `scifi-<seed>.png` (a **scene file** — see below) |
| `Q` / `Esc` | Quit                                    |

### Headless rendering

To render straight to a PNG without opening a window (useful for batches):

```sh
go run ./cmd/render -s 12345 -o scene.png
go run ./cmd/render -s mars -t twilight -w 1920 --height 1080
go run ./cmd/render -c my.yaml -s 7 -o tuned.png
go run ./cmd/render -f scene.png -o copy.png    # reproduce a saved scene
```

The interactive app and the headless renderer share the exact same element
pipeline, so a given seed produces an identical image either way.

## Configuration & scene files

The constants that shape generation — probabilities and limits like the horizon
distribution, the star-density bias, and the dominant-star lighting ranges — live
in a **configuration** rather than being hardcoded. Pass a YAML file with `--config`
to tune them. The file may be **partial**: only the values you set are overridden,
and everything else is filled from the built-in defaults. For example, to pin the
horizon halfway up:

```yaml
horizon:
  min: 0.5
  max: 0.5
  mean: 0.5
  std: 0
```

Saving (in the app or with `cmd/render`) writes a **scene file**: an ordinary PNG
that any viewer can open, with the data needed to reproduce the scene embedded as
PNG text chunks under the `scifi-landscape/` prefix — the **seed**, the complete
**config**, the derived **globals**, and the generated **scene list** (as YAML).
Pass a scene file back with `--from` to reproduce it: the embedded seed and config
are loaded (an explicit `--seed` or `--config` still takes precedence), so the
scene regenerates pixel-for-pixel. This makes a saved image self-describing — it
carries its own recipe.

To pull those embedded layers back out as files — to inspect them or reuse the
config with `--config` — use the `config` subcommand. It writes each requested
layer to a file named after the scene's seed: `scifi-<seed>.config.yaml`,
`scifi-<seed>.globals.yaml`, `scifi-<seed>.scene.yaml`, and `scifi-<seed>.seed.txt`.
Config, globals, and the scene list are written by default; the seed is opt-in.
Each output has its own toggle flag:

```sh
go run . config scene.png                 # config + globals + scene list
go run . config scene.png --seed          # also write the seed file
go run . config scene.png --config=false  # skip the config; write the rest
```

`VERSIONING.md` describes the reproducibility contract that keeps old seeds and old
scene files rendering the same as the generator evolves.

## How it works

A scene is a sequence of **elements** drawn back-to-front onto a shared canvas.
Global **settings** are chosen first and shape how every element is drawn. The
scene seed makes everything reproducible, but rather than one shared random
stream, the settings and **each element draw from their own independent stream**,
derived from the seed and the part's name (`seed.Derive`). Because the streams
are independent, adding a new element — or changing how an existing one uses
randomness — never shifts any other element's output, so a seed keeps its meaning
as the project grows.

### Global settings (so far)

- **Time of day** — `midday`, `dusk`, or `twilight`. Drives the sky palette.
- **Dominant-star light** — how planets are lit: the star's **color** (tints the
  lit side), **brightness** (terminator harshness — high gives a sharp shadow
  line, low a soft fade), **phase** (full = fully sunlit with a spherical
  highlight, new = visible only by ambient light), and **ambient** (fill light in
  shadow — low leaves shadows black, high keeps features visible). The light's
  angle is the star/twinkle angle below.
- **Horizon point** — between 20% and 50% of the scene height *measured from
  the bottom* (the ground's share), normally distributed around ~35%. So the
  sky always fills 50–80% of the scene — there's never more ground than sky.
  The sky gradient is anchored at this line.
- **Twinkle angle** — the shared orientation of star twinkle spikes, 0–90°,
  biased hard toward 0° (90° is rarest). Every twinkling star uses this angle.
- **Star density** — a log-normal multiplier on the earthlike star count,
  biased so a richer-than-earthlike sky (~1.7×) is typical, still ranging from a
  near-empty sky to a dense cluster.

### Elements (so far)

- **Sky** — a vertical gradient filling the scene, anchored at the horizon
  (brightest) and darkening toward the top:
  - *Midday*: any hue, light/desaturated at the horizon → dark/saturated up top
    (e.g. gray-blue→deep-blue for an Earth-like sky, pink→deep-red for Mars).
  - *Dusk*: a wild multi-stop journey that starts warm and bright, runs through
    a range of hues (yellow→orange→red, yellow→green→blue, ...), and trends to
    near-black at the top.
  - *Twilight*: mostly black, with a dim glow near the horizon fading out,
    sometimes picking up a faint second color higher up.

  Below the horizon the sky is mirrored and dimmed, ready to be reused as a
  water reflection by a later element (otherwise it is overwritten).

- **Stars** — points of light scattered over the sky (none at midday). Each has
  its own color (mostly near-white, split between blue-white and warm tints) and
  brightness, which ranges from faint and dim to bright. Most are single pixels;
  a few are tiny discs; a rare few are discs with twinkle spikes drawn at the
  global twinkle angle. At twilight stars cover the whole scene; at dusk they
  fade toward the bottom via alpha. How many appear is set by the star-density
  setting, which is biased toward rich, dense fields.

- **System stars** — the local sun(s) of the system. There are 0–5, usually 0
  or 1, with higher counts much rarer toward dusk and night. Each has its own
  color and a size biased small (about like Earth's sun) with a rare tail up to
  20% of the sky width. A soft circular glow brightens the sky around each
  before its disc is drawn (white-hot core fading to the sun's color); small
  ones get a twinkle cross at the global angle. At dusk the suns sit low — near
  the horizon, but wandering up to about a quarter of the sky and sometimes just
  under the horizon; at twilight a few small, dim suns may appear, scattered in
  the night sky like faint distant stars.

- **Planets** — planets in the sky, in front of the stars and suns but behind
  the ground (so one near the horizon is occluded by the terrain). A scene has a
  ~75% chance of any planets; when present the count is usually a few but can run
  up to 20, with multiples common. Sizes range from a few-pixel dot to half the
  scene width (biased small), though the first planet is often a dominant world
  filling 20–50% of the sky. Each planet has a type:
  - **Gas giant** — turbulent latitudinal color bands on a limb-shaded sphere,
    with palettes from similar hues (Neptune-like), to moderately variable
    (Jupiter-like), to fantastic.
  - **Moon** — an airless rocky body in washed-out, gray-leaning dusty colors
    (any hue possible). Its surface is built from several layers: fine mottle,
    large dark "maria" lava patches (the man-in-the-moon look), recolored ice or
    dusty patches of another hue, often lighter poles, and — if large enough — a
    spread-out scattering of impact craters (mostly small, with the occasional
    larger one). Craters are
    circular on the sphere — so they appear round near the disc center and
    foreshorten into tangent-aligned ellipses toward the limb. Each is a vast,
    flat-floored crater: a uniformly darker floor, a thin (near-pixel-width)
    inner highlight/shadow ring and rim lip lit from the planet's rotation
    direction. Overlapping craters obliterate the ones beneath them, like fresh
    impacts over old. When a moon is **large on screen** it switches to a far more
    detailed, bump-mapped surface: a procedural height field of ridged
    "mountains" and fine grain that self-shadows under the dominant star, and
    richer craters — roughened, raised rims, central peaks in the bigger ones, and
    smooth (rather than just dark) floors. The detail fades in with size, so small
    moons stay simple and only big, near worlds pay for the extra texture.

  Each planet's surface is tilted: the global star
  angle plus a per-planet rotation of up to 90° (biased small, so planets stay
  fairly aligned). Planets also fade toward the sky color near the horizon
  (atmospheric haze) — strongly at dusk and, in daylight, fading to basically
  nothing before reaching the horizon — so low planets blend into the sky while
  still hiding the stars and suns behind them, and the sky itself is never
  dimmed. (At twilight there is no haze, so planets stay crisp.) Planets emit no
  light: their reflected light is screened over the sky, so they only ever
  brighten it, and the shadowed side simply fades into the sky color.

- **Clouds** — atmospheric cloud layers in the sky (in front of the stars, suns,
  and planets, but behind the horizon terrain, so the water reflects them). The
  sky is kept mostly clear — each kind of layer appears independently and is
  biased thin/sparse so the clouds never wall off the view. Two kinds:
  - *High gauzy layer*: a single, mostly-transparent sheet of fractal-noise puffs
    and ripples covering the whole sky, stretched hard toward the horizon (like
    the ground) so it recedes into thin distant bands.
  - *Low nimbus clouds*: discrete, flat-bottomed, billowy clouds, drawn in up to
    three depth layers — small clouds hugging the horizon drawn first, larger
    clouds riding higher drawn last over them. Each cloud is a procedural height
    field — a flat-based envelope lumped up by fractal "cauliflower" billows
    (multi-octave cellular noise) and finer fractal detail, with the edges eroded
    to a ragged outline — that is then bump-mapped and lit with the same
    dominant-sun model as the planets (the light direction comes from the twinkle
    angle and the lit side is tinted by the star's color), so the billow tops
    catch the sun and the crevices and the base fall into shadow. The base is
    nearly flat but not a hard straight cut — it rounds gently up at the rounded
    tips and carries a shallow scallop. The billow detail scales with on-screen size, so a
    big near cloud keeps the small-bulb cauliflower texture of a distant one
    instead of dissolving into a few large lobes.

  Coloring follows the time of day: pale and near-white by day, bright and warm
  at dusk, dark silhouettes at night.

- **Mountains** — a range along the horizon (only some scenes have one), rising
  into the sky in front of the planets. Shaped by a **height** (averaging ~10% of
  the sky, occasionally up to 50%) and a **smoothness**: high smoothness gives a
  few key points and gentle curves with little noise, low smoothness gives many
  points and heavy noise for jagged peaks. The combinations span rolling hills,
  Rockies/Alps, low buttes, and jagged airless ridges. Colored by a gradient from
  a ground-like base up to light/white, normalized by absolute altitude so only
  genuinely tall peaks turn white (snow caps).

- **Ground** — the base terrain below the horizon, always drawn (drawn last, so
  the suns and planets set behind it). Any color, in one of two modes:
  - *Normal*: one base hue, roughly uniform, with light/dark and saturation
    variation — hazier/lighter at the distant horizon, darker/more saturated in
    the foreground.
  - *Variable*: the depth gradient runs through a random number (2–5) of random
    colors, and a low-frequency noise wanders the gradient lookup so color
    patches drift back and forth instead of transitioning cleanly — an alien,
    non-uniform surface.

  Brightness scales with the time of day. Fractal value-noise texture is baked
  in so it reads as a surface, coarser toward the foreground, and the noise is
  warped so it's strongly stretched near the horizon (thin streaks) and relaxes
  toward the foreground. It is built in horizontal bands that are narrow at the
  horizon and grow taller toward the foreground, giving a sense of distance.

- **Cities** — a distant city on the ground near the horizon (only some scenes),
  drawn as clustered rectangular buildings in dark, low-saturation, similar
  colors — far-off skyscrapers in silhouette. A city can be a small settlement
  patch or stretch the whole width (dozens of buildings up to thousands). A
  noisy density field gives it an irregular shape — odd outlines with dense
  cores and sparse stretches and gaps — and buildings run taller and wider where
  the city is dense. They are drawn back-to-front: the farthest (highest, at the
  horizon) first, working down and closer, growing slightly as they near, with
  the farthest fading into the horizon haze. When there is an ocean the city
  keeps to **land** — its islands and coastline — instead of standing in the
  water (it consults the shared land mask, and is drawn before the water so it
  still reflects). Some cities are **domed**: one to a few geodesic glass
  hemispheres go up over clusters of buildings. Each dome is a subdivided
  icosahedron projected onto a sphere — so its triangle struts shrink and
  foreshorten believably toward the rim — over a semi-transparent shell that
  reflects the sky (brighter and glassier at the grazing rim, with a sun glint)
  while the buildings show through inside.

- **Water** — an ocean below the horizon (only some scenes). It reflects the
  scene above the horizon — sky, suns, planets, mountains, and the city skyline —
  by mirroring the already-drawn pixels down across the horizon with wave-ripple
  distortion (calm and mirror-like near the horizon, choppier toward the viewer)
  and a water tint that grows, and darkens, with distance from the horizon. The
  ocean is not solid ground: a noise **land elevation**, biased upward toward the
  horizon, decides where land clears sea level, so the foreground is open water
  with scattered **islands** plus a distant **coastline** at the feet of the
  mountains. Land shows the ground through the water and is ringed by a beach and
  surf. The ocean/land model is resolved up front (in `Scene.Build`) and shared,
  so both the city (land placement) and the water (what to flood) agree on it.

## Code layout

```
main.go              interactive entry point (flags, window)
cmd/render/          headless scene-file renderer
internal/seed/       resolve a number-or-text seed to an int64
internal/gfx/        RGB/HSV color + gradient interpolation + fractal noise
internal/canvas/     concurrency-safe RGBA drawing surface
internal/config/     the tunable Config (probabilities/limits), partial→complete merge, YAML
internal/scene/      directors, globals, entities, generators, renderers; the Sky/Stars/SystemStars/Planets/Clouds/Mountains/Ground/Cities/Water elements
internal/scenefile/  read/write scene files (PNG + embedded seed/config/globals/scene-list)
internal/cli/        shared config + seed resolution for both front-ends
internal/app/        Ebiten front-end + generation controller
```

A scene is built in layers — `seed + config` → **director** → **globals** →
**generators** → **entities** (the scene list) → **renderers** → image — so each
stage can be recorded and replayed independently. Every element is migrated to this
model: each implements `scene.Element` (added to the pipeline in `scene.New`) and
also has a versioned **generator**, **renderer**, and **entity schema**, so its
random choices are resolved into serializable entities that the renderer draws
from. See `VERSIONING.md` for the freeze contract that keeps it all reproducible.
