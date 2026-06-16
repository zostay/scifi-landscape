# Versioning & Reproducibility Contract

Seed reproducibility is the sacred invariant of scifi-landscape: a given seed (and
configuration) must always produce the same scene, **even as the app keeps
evolving**. This document is the contract that makes that possible. It applies
from the first released version onward.

See `specs/configuration-and-replayability.md` for the full design rationale.

## The pipeline

A scene is built in layers, each a pure function of the previous:

```
seed + config  ──Director──▶  globals  ──Generators──▶  scene list  ──Renderers──▶  image
```

- **Config** (`internal/config`) — the tunable constants (probabilities, limits).
  May be partial on load (missing values filled from `DefaultConfig`); always
  complete when written.
- **Director** (`internal/scene/director.go`) — `seed + config → globals`. No side
  effects; deterministic.
- **Globals** (`Globals`) — the complete, scene-wide derived values. Never partial.
- **Generators** — `globals → entities`. No side effects (draw nothing).
- **Entities** (`Entity`) — versioned, serializable schema instances; the scene
  list.
- **Renderers** — `scene list → image`. The only thing that draws; consume no
  randomness.

A scene file (`internal/scenefile`) is a PNG carrying `scifi-landscape/seed`,
`config.yaml`, `globals.yaml`, and `scene-list.yaml`, so a scene can be reproduced
from any layer. A config's `algorithms` section names the director, generator, and
renderer **versioned keys** (`scene.v0`, `sky.v0` … `water.v0`); `scene.New` builds
the pipeline by resolving the generator keys (in order) against the registries, so
the keys recorded in `config.yaml` are the on-disk contract that selects the code.

## The freeze rules (from the first release on)

1. **Algorithms are frozen once released.** Do not change the behavior of an
   existing Director, Generator, or Renderer. To change behavior, add a new
   versioned implementation (`scene.v0` → `scene.v1`, `planets.v0` → `planets.v1`)
   and register it under a new key. Configs select algorithms by key (the config's
   `algorithms` lists), so old configs keep running the old code.

2. **Entity schemas are forward-mutable only.** You may *add* fields to an existing
   schema (so long as a zero value means "as before"). You may never rename,
   retype, or repurpose an existing field. A breaking change is a new schema
   (`PlanetGasGiantV0` → `PlanetGasGiantV1`). The yaml keys are the on-disk
   contract and are pinned with explicit tags.

3. **Random-stream keys never change.** Each element draws from
   `seed.Derive(master, name)` where `name` is the element's `Name()`. Changing a
   name re-maps every existing seed for that element, so names are frozen. (Note:
   the stream key is separate from the versioned algorithm key — an algorithm can
   go to `planets.v1` while still drawing from the `"planets"` stream.)

4. **Within an algorithm, the random draw order is frozen.** Adding/removing/
   reordering draws in a released algorithm changes its output for every seed. Add
   new draws only at the end, or in a new version.

5. **Few dependencies in the deterministic core.** Directors, Generators, and
   Renderers should avoid dependencies that could change the output across
   versions of those dependencies. Serialization (YAML) lives at the boundary, not
   in the algorithms.

### The only exception

Bug fixes for crashes, security, or pathological performance may touch frozen
code — but should be done in a way that does not change normal-case output. Prefer
guarding the failure over altering the algorithm.

## Guards

The contract is enforced mechanically by the test suite — run `make verify` (or
`go test ./internal/scene`) any time, and always before a release:

- **Behavioral freeze — `golden_test.go` + `testdata/golden.txt`.** The backstop:
  it fails if any seed in its matrix renders even one pixel differently, so no
  released algorithm can change its output unnoticed. `TestGoldenCoversAllAlgorithms`
  (`contract_test.go`) guards that *every* registered generator/renderer is in the
  golden-covered default pipeline, so nothing escapes this net. Regenerate goldens
  only for an intentional, reviewed output change:

  ```
  UPDATE_GOLDEN=1 go test ./internal/scene -run TestGolden
  ```

- **Structural freeze — `contract_test.go` (`TestSchemaContract`) +
  `testdata/contract-schema.txt`.** Reflects every serialized type (entity schemas,
  `Globals`, `Config`) into a `<Type>/<yamlKey> <goType>` signature and fails if any
  recorded line disappears — i.e. a released serialized field was renamed, retyped,
  or removed. Adding fields/schemas is allowed; after such an additive change,
  regenerate the (append-only) baseline:

  ```
  UPDATE_CONTRACT=1 go test ./internal/scene -run TestSchemaContract
  ```

  Regenerating to *drop* a line is the violation a reviewer catches in the diff.

- **Naming — `registry_test.go`.** Entity schema keys and algorithm keys must be
  versioned (`.v<n>`), every renderer must have a matching generator, and the
  default config's pipeline must resolve against the registries.

The `/release` skill (`.claude/skills/release/SKILL.md`) runs `make verify` as the
final gate, then creates and pushes the annotated version tag.

## Migration status

**All elements are migrated.** Sky, Stars, SystemStars, Planets, Clouds,
Mountains, Ground, Cities, and Water each have a versioned generator + renderer
(`<el>.v0`) and entity schema(s), registered in `registry.go` / each element's
`init`. `Scene.Build` drives each element as `Generate` (all randomness) followed
by `RenderList` (only drawing), accumulating every element's entities into the
scene list it returns; the golden suite confirms the build is byte-identical, and
each element has a `*SceneListRoundTrip` test proving its entities survive YAML
and re-render to the same pixels.

The scene-wide sky and ground gradients are **globals**: the director derives
them (`SkyGradient`, `GroundGradient`, `GroundVariable` on `Globals`) and they are
recorded in `globals.yaml`. Renderers read them from the `Context`, which
`Scene.newContext` populates straight from the globals — so a recorded scene list
redraws the same image without re-deriving anything from the seed. The ocean/land
model is the one shared value still built from the seed in `newContext`, but only
generation reads it (Cities placement); for rendering it is captured per-scene in
the `water` entity. Both the headless renderer and the live app (on a completed
build) record all four layers — seed + config + globals + scene list.

Replaying from each layer is exposed by the `scifi-landscape from` subcommand:
the default re-derives everything from seed + config; `--globals` uses the stored
globals (skipping the director); and `--scene` renders the stored scene list
directly (`Scene.RenderList`, skipping the generators). Because the gradients now
live in the globals, `--scene` rendering is **seed-independent** — its output
depends only on the globals and scene list. Two tests pin this:
`TestRenderListMatchesBuild` (`RenderList(Build's list) == Build` byte-for-byte,
including a YAML round-trip) and `TestRenderListSeedIndependent` (the same image
even when `RenderList` is handed a different seed).
