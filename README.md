# scifi-landscape

A procedural sci-fi landscape generator. Scenes are drawn one element at a
time — the construction is animated on screen so you can watch it build — and
every scene is fully determined by a single random **seed**, so any scene can
be reproduced exactly just by knowing its seed. The finished image stays on
screen and can be saved to a PNG.

This is a modern reimagining of an old 256-color graphics experiment.

## Running

```sh
go run .                 # random seed, 1280x720 window
go run . -seed 12345     # reproduce a specific scene
go run . -seed mars      # any text works too (hashed to a seed)
go run . -time dusk      # force the time of day (midday | dusk | twilight)
go run . -w 1920 -h 1080 # custom size
```

A seed can be **a number or any text**. A plain integer (within int64 range) is
used directly; anything else — a word, a phrase, a too-big number — is hashed to
a stable seed, so `-seed mars` always yields the same scene. The resolved seed
is printed to the terminal on startup and shown on the on-screen HUD.

### Controls

| Key       | Action                                    |
|-----------|-------------------------------------------|
| `N` / `Space` | Generate a new random scene           |
| `R`       | Replay the current seed (re-animate)      |
| `E`       | Enter a seed (type a number or text, `Enter` to apply, `Esc` to cancel) |
| `S`       | Save the current image to `scifi-<seed>.png` |
| `Q` / `Esc` | Quit                                    |

### Headless rendering

To render straight to a PNG without opening a window (useful for batches):

```sh
go run ./cmd/render -seed 12345 -o scene.png
go run ./cmd/render -seed mars -time twilight -w 1920 -h 1080
```

The interactive app and the headless renderer share the exact same element
pipeline, so a given seed produces an identical image either way.

## How it works

A scene is a sequence of **elements** drawn back-to-front onto a shared canvas.
Global **settings** are chosen first and shape how every element is drawn. All
randomness comes from one seeded `math/rand` source consumed in a fixed order,
which is what makes scenes reproducible.

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
    impacts over old.

  Each planet's surface is tilted: the global star
  angle plus a per-planet rotation of up to 90° (biased small, so planets stay
  fairly aligned). Planets also fade toward the sky color near the horizon
  (atmospheric haze) — strongly at dusk and, in daylight, fading to basically
  nothing before reaching the horizon — so low planets blend into the sky while
  still hiding the stars and suns behind them, and the sky itself is never
  dimmed. (At twilight there is no haze, so planets stay crisp.) Planets emit no
  light: their reflected light is screened over the sky, so they only ever
  brighten it, and the shadowed side simply fades into the sky color.

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
  the farthest fading into the horizon haze.

- **Water** — an ocean filling the foreground (only some scenes), drawn last. It
  reflects the scene above the horizon — sky, suns, planets, mountains, and the
  city skyline — by mirroring the already-drawn pixels down across the horizon
  with wave-ripple distortion (calm and mirror-like near the horizon, choppier
  toward the viewer) and a water tint that grows, and darkens, with distance
  from the horizon.

## Code layout

```
main.go              interactive entry point (flags, window)
cmd/render/          headless PNG renderer
internal/seed/       resolve a number-or-text seed to an int64
internal/gfx/        RGB/HSV color + gradient interpolation + fractal noise
internal/canvas/     concurrency-safe RGBA drawing surface
internal/scene/      settings, the Element interface, the Sky/Stars/SystemStars/Planets/Mountains/Ground/Cities/Water elements
internal/app/        Ebiten front-end + generation controller
```

New scene elements implement `scene.Element` and are added to the pipeline in
`scene.New`.
