# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A procedural sci-fi landscape generator (Go + [Ebiten](https://ebitengine.org/)). A scene is drawn one element at a time, animated on screen, and is **fully determined by a single seed** — the same seed always reproduces the exact same image. There are two binaries: the windowed app (root package, `main.go`) and a headless renderer (`cmd/render`).

## Commands

Use the **Makefile**, not bare `go`, for anything that touches the GUI. It sets `CGO_CFLAGS=-Wno-deprecated-declarations` (silences ebiten's macOS Metal cgo deprecation warnings) and filters benign `[CAMetalLayer nextDrawable]` runtime noise out of `make run`'s stderr.

- `make run ARGS="-s 7 -t dusk"` — windowed app; flags pass via `ARGS` (e.g. `ARGS="config scene.png"`)
- `make render ARGS="-s 7 -o scene.png"` — headless render to PNG
- `make build` — build the `scifi-landscape` binary
- `make test` / `make vet` / `make fmt`
- Single test: `go test ./internal/scene -run TestGolden` (export `CGO_CFLAGS=-Wno-deprecated-declarations` first if running `go` directly, or it spews warnings)

## The one rule that governs everything: seed reproducibility

**A given seed must always produce the exact same scene, forever.** This is the non-negotiable invariant. `internal/scene/golden_test.go` is the safety net: it renders a matrix of seeds/sizes headlessly, hashes the raw RGBA pixels, and compares against `internal/scene/testdata/golden.txt`. **Any refactor must keep this passing** — a golden diff means you changed output.

- `UPDATE_GOLDEN=1 go test ./internal/scene -run TestGolden` regenerates `golden.txt`. Pre-release this is for deliberate output changes (review the diff). **Post-release, a diff to an *existing* golden case is a freeze violation** — existing seeds must not move; golden.txt only *grows* (new cases / new versions), never changes existing rows. Never regenerate to "make the test pass."

### The release freeze — Directors, Generators, Renderers, entity schemas

**The first release is imminent; after it these rules are enforced, so treat them as binding now.** The full rationale is `VERSIONING.md`; the operational rules to apply when touching the deterministic core (`internal/scene`):

- **Never edit a released Director, Generator, or Renderer.** To change what one does, add a *new versioned implementation* under a new key (`scene.v0` → `scene.v1`, `planets.v0` → `planets.v1`) and register it; leave the old one untouched and still registered. Configs name algorithms by key, so old configs/scenes keep running old code. Prefer reusing the old logic via Go's type system (embed/compose) over copy-paste — but do not modify it. Registries: directors in `director.go` (`directors` map); generators/renderers in `registry.go` (`init`).
- **Entity schemas are forward-mutable only.** You may *add* a field whose zero value means "as before"; never rename, retype, or repurpose an existing field. A breaking change is a new schema struct **and** key (`PlanetGasGiantV0` → `PlanetGasGiantV1`), registered alongside the old via `RegisterEntity`. The `yaml:"…"` tags are the on-disk contract — pin them explicitly and never change them.
- **Random-stream keys are frozen.** Each element draws from `seed.Derive(master, el.Name())`; renaming `Name()` re-maps every existing seed. The stream key is *separate* from the versioned algorithm key — `planets.v1` still draws from the `"planets"` stream.
- **Draw order within a released algorithm is frozen.** Adding, removing, or reordering random draws shifts output for every seed. Add new draws only at the end, or in a new version.
- **Keep the deterministic core dependency-light.** Anything that could drift across dependency versions can change output; keep serialization (YAML) at the boundary, not inside the algorithms.
- **Only exception:** fixing a crash / security / severe-perf bug — and even then prefer a fix that leaves normal-case output unchanged.

The golden suite is the arbiter: if bumping a version (or any edit) shows an *unintended* `golden.txt` diff, you broke a freeze rule.

## Architecture

The pipeline is a chain of pure functions, each layer reproducible from the previous:

```
seed + config ──Director──▶ globals ──Generators──▶ scene list (entities) ──Renderers──▶ image
```

- **`internal/scene`** is the deterministic core. Key pieces:
  - `Scene.Build` (`scene.go`) is the single shared render path used by *both* binaries (so they're always pixel-identical). It builds a `Context` with seed-derived shared state (sky/ground gradients, ocean — see `newContext`), then for each element runs `Generate` → `RenderList`, accumulating the returned `SceneList`.
  - `Scene.RenderList` is the renderers-only replay path: it partitions a stored scene list back to each owning element (via `Element.Schemas()`) and renders without generating. It still rebuilds the shared gradients/ocean from the seed (those aren't captured in the scene list yet — the known follow-on).
  - An **`Element`** = `Generator` (`Generate`: globals → entities, all randomness, draws nothing) + `Renderer` (`RenderList`: entities → pixels, consumes no randomness) + `Schemas()`. The 9 elements (sky, stars, systemstars, planets, clouds, mountains, ground, cities, water) render back-to-front in the order set by `scene.New`.
  - `entity.go` holds the entity registry (schema key → factory) that lets a heterogeneous `SceneList` round-trip through YAML. `director.go`/`registry.go` hold the versioned director/generator/renderer registries selected by config keys.
- **`internal/config`** — generation tunables (probabilities/limits), loadable from partial YAML (gaps filled from defaults).
- **`internal/scenefile`** — a **scene file** is an ordinary PNG with the four reproducibility layers embedded as `scifi-landscape/` tEXt chunks: `seed`, `config.yaml`, `globals.yaml`, `scene-list.yaml`. Splices chunks into the encoded PNG (stdlib `image/png` can't write tEXt).
- **`internal/app`** — the Ebiten `Controller` owns the canvas and a background build goroutine; `SetReplay` injects stored globals/scene-list to drive the deeper replay modes.
- **`internal/cli`** — shared command-line plumbing: `AddSceneFlags` (POSIX `-s`/`--long`, via cobra/pflag; `--height` has no short form because `-h` is `--help`), `Resolve` (`--from` seed+config), `LoadReplay`/`LoadConfigFile`/etc. (load layers for the `from`/`from-config` subcommands), `ExtractScene` (the `config` subcommand).
- **`internal/gfx`**, **`internal/canvas`**, **`internal/seed`** — color/gradient math, the drawing surface, and seed resolution (number used directly, text hashed; `Derive` for per-element streams).

### Subcommands (the layer workflow)

`config <scene.png>` extracts the embedded layers to `scifi-<seed>.{seed.txt,config.yaml,globals.yaml,scene.yaml}`. `from <scene.png>` replays a scene file, with `--globals`/`--scene` choosing how deep to start (deepest wins). `from-config` is the inverse of `config`: it reassembles a scene from individual layer files (`--seed`/`--config`/`--globals`/`--scene`), each option skipping that layer's generation step. See `README.md` for the full UX.

### Known gap vs. `specs/configuration-and-replayability.md`

The spec is otherwise fully implemented (entities, the Director/Generator/Renderer split, partial/complete config, the four PNG chunks, and replay from every layer via `from`/`from-config`). The one unfinished piece: `Globals` (`director.go`) holds only `Seed/W/H/Settings`. The spec's globals are meant to be *all* pre-render derived state, but the **sky/ground gradients, ocean model, and per-element derived seeds are still re-derived from the master seed** in `Scene.newContext`/`deriveRng` rather than captured in `globals.yaml`. So replaying from `--globals`/`--scene` is **not yet seed-independent** — it still threads the master seed through to rebuild that shared state. Completing this means promoting those values into `Globals` as **new fields** (forward-compatible) or a new globals version — under the same freeze rules above.

## Conventions

- Match the surrounding style: heavy doc comments explaining *why* (especially reproducibility rationale), small focused functions.
- When migrating/refactoring the render path, prove pixel-identity: `TestRenderListMatchesBuild` and the golden suite must stay green.
- Each element has a `*SceneListRoundTrip` test proving its entities survive YAML and re-render identically — add one for any new element/schema.
