# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A procedural sci-fi landscape generator (Go + [Ebiten](https://ebitengine.org/)). A scene is drawn one element at a time, animated on screen, and is **fully determined by a single seed** — the same seed always reproduces the exact same image. There are two binaries: the windowed app (root package, `main.go`) and a headless renderer (`cmd/render`).

**Design intent: this is an always-headed GUI app — never headless.** Watching scenes build is the point, and the roadmap is *more* in-app interactivity (a scene editor: drag planets, tune gas-giant bands, lock parts of a scene). The `scifi-landscape` binary imports Ebiten, whose `internal/ui` package initializes GLFW in a package `init()` — so the binary **requires a display to even start** (`--help`/`config`/`from` all need a window server; they segfault headless). This is by design, not a bug to "fix" by making the app run headless. The `config`/`from`/`from-config` CLI subcommands are a stop-gap, not the project's direction — don't invest in headless CLI support. For batch/headless PNG rendering, `cmd/render` is the GUI-free path (it links no Ebiten). When adding a feature that's genuinely GUI-free and must run headless, put it in a binary that does **not** import `internal/app`/Ebiten (like `cmd/render`).

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

### The two contract gates (`make verify` runs both)

The change contract from `VERSIONING.md` is enforced mechanically — run `make verify` any time, and the `/release` skill runs it before tagging:

- **Behavioral freeze** — `TestGolden` (above), plus `TestGoldenCoversAllAlgorithms` (`contract_test.go`) which fails if a registered generator/renderer isn't in the golden-covered default pipeline (so no algorithm escapes the freeze).
- **Structural freeze** — `TestSchemaContract` (`contract_test.go` + `testdata/contract-schema.txt`) reflects every serialized type (entity schemas, `Globals`, `Config`) into `<Type>/<yamlKey> <goType>` lines and fails if any recorded line **disappears** (a released serialized field renamed/retyped/removed). Adding fields/schemas is fine; after an *additive* change regenerate the append-only baseline with `UPDATE_CONTRACT=1 go test ./internal/scene -run TestSchemaContract`. Regenerating to drop a line is the violation.

When a gate fails, the fix is a **new version** (`*.v1`, `…V1`), never editing the released algorithm/schema — see the release-freeze rules above. To cut a release, use the `/release` skill (verifies, then proposes + tags `vX.Y.Z`); it's tagging-only (no binaries).

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
  - `Scene.Build` (`scene.go`) is the single shared render path used by *both* binaries (so they're always pixel-identical). It builds a `Context` whose shared state comes from the globals — the **sky/ground gradients are fields of `Globals`** (derived by the director), read in `newContext`; only the ocean/land model is still rebuilt from the seed there (no renderer reads it). It then runs each element's `Generate` → `RenderList`, accumulating the returned `SceneList`.
  - `Scene.RenderList` is the renderers-only replay path: it partitions a stored scene list back to each owning element (via `Element.Schemas()`) and renders without generating. Because the gradients live in the globals it's given, **its output is seed-independent** (`TestRenderListSeedIndependent` pins this) — the seed it receives only feeds the unused ocean.
  - An **`Element`** = `Generator` (`Generate`: globals → entities, all randomness, draws nothing) + `Renderer` (`RenderList`: entities → pixels, consumes no randomness) + `Schemas()`. The 9 elements render back-to-front; **`scene.New(globals, cfg.Algorithms)` resolves the pipeline from the config's versioned generator keys** (`sky.v0`…`water.v0`) via the registry — it errors on an unknown key (`CheckAlgorithms` validates up front, e.g. in `NewController`).
  - `entity.go` holds the entity registry (schema key → factory) that lets a heterogeneous `SceneList` round-trip through YAML. `registry.go`/`director.go` hold the versioned director/generator/renderer/element registries; `RegisterElement` registers a v0 element as all three under one key. Config names algorithms by these keys.
- **`internal/config`** — generation tunables (probabilities/limits), loadable from partial YAML (gaps filled from defaults).
- **`internal/scenefile`** — a **scene file** is an ordinary PNG with the four reproducibility layers embedded as `scifi-landscape/` tEXt chunks: `seed`, `config.yaml`, `globals.yaml`, `scene-list.yaml`. Splices chunks into the encoded PNG (stdlib `image/png` can't write tEXt).
- **`internal/app`** — the Ebiten `Controller` owns the canvas and a background build goroutine; `SetReplay` injects stored globals/scene-list to drive the deeper replay modes.
- **`internal/cli`** — shared command-line plumbing: `AddSceneFlags` (POSIX `-s`/`--long`, via cobra/pflag; `--height` has no short form because `-h` is `--help`), `Resolve` (`--from` seed+config), `LoadReplay`/`LoadConfigFile`/etc. (load layers for the `from`/`from-config` subcommands), `ExtractScene` (the `config` subcommand).
- **`internal/gfx`**, **`internal/canvas`**, **`internal/seed`** — color/gradient math, the drawing surface, and seed resolution (number used directly, text hashed; `Derive` for per-element streams).

### Subcommands (the layer workflow)

`config <scene.png>` extracts the embedded layers to `scifi-<seed>.{seed.txt,config.yaml,globals.yaml,scene.yaml}`. `from <scene.png>` replays a scene file, with `--globals`/`--scene` choosing how deep to start (deepest wins). `from-config` is the inverse of `config`: it reassembles a scene from individual layer files (`--seed`/`--config`/`--globals`/`--scene`), each option skipping that layer's generation step. See `README.md` for the full UX.

### Status vs. `specs/configuration-and-replayability.md`

The spec is fully implemented: entities; the Director/Generator/Renderer split; partial/complete config; the **pipeline is built from `config.Algorithms`** (the director, generators, and renderers are resolved from versioned config keys, not hardcoded); the four PNG chunks; and replay from every layer via `from`/`from-config`. The renderer-visible shared state — the sky and ground gradients — lives in `Globals` and is recorded in `globals.yaml`, so **scene-list replay reproduces the image without the seed**.

Per the spec, **per-entity renderer granularity** (more than one renderer per scene, selected independently of generators) is explicitly deferred past v0; today each element is its own generator+renderer, and `New` drives the pipeline from the Generators list while validating the Renderers list. Two pieces of derived state remain seed-derived by design, neither affecting render-time reproducibility: (1) the **per-element generation RNG** (`deriveRng(seed, el.Name())`) — so *globals* replay (which re-runs generators) still needs the seed, which `globals.yaml` carries; and (2) the **ocean/land model** (`buildOcean` in `newContext`), which only generation reads (Cities placement) and which is already captured per-scene in the `WaterV0` entity for rendering. Future steps (per-entity renderers, promoting per-element seeds into `Globals`) happen under the freeze rules above.

## Conventions

- Match the surrounding style: heavy doc comments explaining *why* (especially reproducibility rationale), small focused functions.
- When migrating/refactoring the render path, prove pixel-identity: `TestRenderListMatchesBuild` and the golden suite must stay green.
- Each element has a `*SceneListRoundTrip` test proving its entities survive YAML and re-render identically — add one for any new element/schema.
