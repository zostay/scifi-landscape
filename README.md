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
- **Horizon point** — between 20% and 50% of the scene height *measured from
  the bottom* (the ground's share), normally distributed around ~35%. So the
  sky always fills 50–80% of the scene — there's never more ground than sky.
  The sky gradient is anchored at this line.

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

## Code layout

```
main.go              interactive entry point (flags, window)
cmd/render/          headless PNG renderer
internal/seed/       resolve a number-or-text seed to an int64
internal/gfx/        RGB/HSV color + gradient interpolation
internal/canvas/     concurrency-safe RGBA drawing surface
internal/scene/      settings, the Element interface, the Sky element
internal/app/        Ebiten front-end + generation controller
```

New scene elements implement `scene.Element` and are added to the pipeline in
`scene.New`.
