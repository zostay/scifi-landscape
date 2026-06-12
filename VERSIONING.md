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
from any layer.

## The freeze rules (from the first release on)

1. **Algorithms are frozen once released.** Do not change the behavior of an
   existing Director, Generator, or Renderer. To change behavior, add a new
   versioned implementation (`scene.v0` → `scene.v1`, `planets.v0` → `planets.v1`)
   and register it under a new key. Configs select algorithms by key, so old
   configs keep running the old code.

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

`internal/scene/registry_test.go` enforces parts of this mechanically: entity
schema keys and algorithm keys must be versioned (`.v<n>`), every renderer must
have a matching generator, and the default config's director must resolve. The
golden suite (`golden_test.go` + `testdata/golden.txt`) is the backstop: it fails
if any seed in its matrix renders even one pixel differently. Regenerate goldens
only for an intentional, reviewed output change:

```
UPDATE_GOLDEN=1 go test ./internal/scene -run TestGolden
```

## Migration status

**All elements are migrated.** Sky, Stars, SystemStars, Planets, Clouds,
Mountains, Ground, Cities, and Water each have a versioned generator + renderer
(`<el>.v0`) and entity schema(s), registered in `registry.go` / each element's
`init`. `Scene.Build` drives each element as `Generate` (all randomness) followed
by `RenderList` (only drawing), accumulating every element's entities into the
scene list it returns; the golden suite confirms the build is byte-identical, and
each element has a `*SceneListRoundTrip` test proving its entities survive YAML
and re-render to the same pixels.

For `sky` and `water` the per-scene content (the sky gradient, the ocean/land
model) is a shared *global* built in `Scene.Build`, not drawn from the element's
own random stream — so their entities are thin (a marker / the ocean params) and
their renderers read the shared global from the `Context`. Promoting those derived
globals into `globals.yaml` (so a scene file's scene-list layer is fully
self-contained without re-deriving from the seed) is the remaining follow-on. Both
the headless renderer and the live app (on a completed build) now record all four
layers — seed + config + globals + scene list — so a scene reproduces from any of
them.
