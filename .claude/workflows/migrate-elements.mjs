export const meta = {
  name: 'migrate-elements',
  description: 'Migrate each remaining scene element to the generator/renderer/entity model, proofing each byte-identical',
  whenToUse: 'After the Planets proof: roll the config/generator/renderer/entity split out to the other elements.',
  phases: [
    { title: 'Migrate', detail: 'one agent per element does the gen/renderer/entity split + self-verify + commit' },
    { title: 'Proof', detail: 'an independent adversarial agent confirms byte-identity was not faked' },
  ],
}

// Sequential, simplest-first. Every element edits internal/scene and builds on the
// previous commit, so they CANNOT run in parallel — this is a sequential dynamic
// loop, not a fan-out. The golden suite is the ground-truth byte-identity gate.
const ELEMENTS = [
  { el: 'stars', note: 'a field of individual stars — most like the Planets list pattern' },
  { el: 'systemstars', note: 'the system sun(s) — a small list, like Planets' },
  { el: 'mountains', note: 'a horizon heightmap resolved from its own rng, then drawn' },
  { el: 'ground', note: 'base terrain; note the ground gradient is a shared global built in Scene.Build (Context.GroundGradient/GroundVariable) — do NOT move it' },
  { el: 'sky', note: 'the sky gradient is a shared global built in Scene.Build (Context.SkyGradient); Sky.Render may consume little or no rng of its own — handle that case' },
  { el: 'clouds', note: 'COMPLEX: two independent layer types, intricate per-cloud rng; extract all rng into the entity(ies)' },
  { el: 'cities', note: 'COMPLEX: also covers domes.go (domes are part of the cities element, planned from the cities stream after buildings); land-confined via Context.LandAt' },
  { el: 'water', note: 'COMPLEX: the ocean/land model is a shared global built in Scene.Build (Context.Ocean/LandAt) — do NOT move it; Water.Render reflects already-drawn pixels and draws little/no rng' },
]

const COAUTHOR = 'Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>'

const MIG_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['element', 'success', 'goldenGreen', 'commit', 'entitySchemas', 'roundTripTest', 'notes'],
  properties: {
    element: { type: 'string' },
    success: { type: 'boolean', description: 'true only if build+golden+round-trip all green AND committed' },
    skipped: { type: 'boolean', description: 'true if the element was already migrated on entry' },
    goldenGreen: { type: 'boolean', description: 'TestGolden passed byte-identical without regenerating goldens' },
    commit: { type: 'string', description: 'commit sha of the migration, or "" if not committed' },
    entitySchemas: { type: 'array', items: { type: 'string' }, description: 'versioned entity schema keys added, e.g. ["star.v0"]' },
    roundTripTest: { type: 'string', description: 'name of the scene-list round-trip test added' },
    filesChanged: { type: 'array', items: { type: 'string' } },
    notes: { type: 'string', description: 'what was done, or what blocked a failure' },
  },
}

const PROOF_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['element', 'verified', 'goldenGreen', 'goldenFileUntouched', 'roundTripVerifiesPixels', 'drawMathUnchanged', 'issues'],
  properties: {
    element: { type: 'string' },
    verified: { type: 'boolean', description: 'true only if EVERY check passed' },
    goldenGreen: { type: 'boolean' },
    goldenFileUntouched: { type: 'boolean', description: 'testdata/golden.txt was NOT modified/committed (no cheating)' },
    roundTripVerifiesPixels: { type: 'boolean', description: 'the round-trip test compares re-rendered PIXELS, not just structs' },
    drawMathUnchanged: { type: 'boolean', description: 'drawing math only moved, not altered; rng order preserved' },
    registered: { type: 'boolean', description: 'element registered as both generator and renderer under <el>.v0' },
    issues: { type: 'array', items: { type: 'string' } },
  },
}

function migratePrompt(el, note) {
  return `You are migrating the \`${el}\` scene element to the generator/renderer/entity architecture in the Go repo github.com/zostay/scifi-landscape, EXACTLY following the pattern already established for the Planets element. Reproducibility is SACRED: the rendered output for every seed MUST stay byte-identical. The golden suite is the arbiter.

Element note: ${note}

ENTRY GUARD — do this first:
- Run \`go build ./...\` (ignore harmless cgo/metal "deprecated" warnings). If the build is ALREADY broken before you touch anything, STOP: return success=false, notes="tree broken on entry". Do not try to fix unrelated breakage.
- Check whether \`${el}\` is already migrated (does internal/scene/${el}_entity.go exist, or is "${el}.v0" already registered as a generator?). If already migrated, return success=true, skipped=true, and stop.

READ THESE FIRST to learn the exact pattern (do not skip):
- internal/scene/planets.go — Render = Generate + RenderList; Generate does ALL rng on c.Rng then returns entities (no drawing, no sleeps); RenderList converts entities back and does ALL drawing + any animation sleeps (no rng).
- internal/scene/planets_entity.go — versioned entity schema(s) <Name>V0 with explicit yaml tags on EVERY field, init() RegisterEntity, lossless internal<->entity conversion.
- internal/scene/registry.go — init() registers "planets.v0" as Generator and Renderer.
- internal/scene/entity.go — Entity, SceneList, Generator, Renderer, MarshalSceneList/UnmarshalSceneList.
- internal/scene/planets_entity_test.go — TestPlanetsSceneListRoundTrip and planetsTestContext.
- internal/scene/scene.go — how Scene.Build wires Context (Settings, SkyGradient, GroundGradient/GroundVariable, Ocean/LandAt, Rng per element). Shared globals built in Build (gradients, ocean) are NOT generated by the element — the renderer reads them from Context as today. Do NOT move them into the entity.
- VERSIONING.md — freeze rules and .v0 naming.

DO THE MIGRATION for internal/scene/${el}.go:
1. Read ${el}.go fully (and any helper files it owns, e.g. domes.go for cities). Identify EVERY c.Rng/rng draw and the resolved state it produces. That resolved state is your entity (one entity, or a list if the element produces many things). State the element reads from Context shared globals stays in Context.
2. New file internal/scene/${el}_entity.go: define versioned, YAML-serializable schema(s) named <Name>V0 with explicit yaml tags on every field. RegisterEntity them in an init(). Provide lossless conversion both ways. Use gfx types (HSV/RGB/Gradient) directly — they serialize fine.
3. Split Render(c) into Generate(c *Context) (SceneList, error) [ALL rng, no drawing] and RenderList(c *Context, list SceneList) error [all drawing + sleeps, no rng], and make Render = Generate then RenderList. CRITICAL: preserve the EXACT rng draw order. If ${el} currently interleaves rng with drawing, move all rng draws into Generate (buffer the decisions into the entity) in the same order, and draw from the buffer in RenderList.
4. Register the element as Generator and Renderer under key "${el}.v0" (add to registry.go init() alongside planets.v0, or an init() in ${el}.go).
5. New file internal/scene/${el}_entity_test.go: add Test<Name>SceneListRoundTrip mirroring TestPlanetsSceneListRoundTrip. For several seeds: Generate -> MarshalSceneList -> UnmarshalSceneList -> re-render via RenderList; assert (a) re-marshaled bytes are byte-stable AND (b) re-rendered PIXELS are byte-identical (sha256 of canvas.Snapshot) to rendering the original list. Build the test Context like planetsTestContext, wiring EVERY Context field ${el} reads — rebuild shared globals the same way Scene.Build does (e.g. buildSkyGradient, buildGroundGradient, buildOcean) using the same derived streams, so both renders see identical globals. (If ${el} produces no entities for some seeds, that is fine; still verify pixels match.)

VERIFY — ALL must pass before you commit:
- \`gofmt -w\` your new/changed files.
- \`go build ./...\` clean.
- \`go test ./internal/scene -run TestGolden\` PASSES. This is the byte-identity proof.
- HARD RULE: do NOT run UPDATE_GOLDEN and do NOT modify internal/scene/testdata/golden.txt. If TestGolden fails, your split changed output — FIX your code until it is green. Regenerating goldens is forbidden and counts as failure.
- \`go test ./internal/scene -run <your round-trip test>\` passes.
- \`go vet ./internal/scene\` clean.

ON SUCCESS: \`git add -A && git commit\` with subject "Migrate ${el} element to generator/renderer/entity model" and a body ending with the trailer:
${COAUTHOR}
Return success=true, the commit sha, goldenGreen=true, the entity schema keys, and the round-trip test name.

ON FAILURE (cannot reach golden-green): do NOT commit. Discard ONLY your own uncommitted changes to the files you touched using \`git stash\` or \`git checkout -- <specific files you created/edited>\` (NEVER \`git reset --hard\` or \`git checkout .\` — there may be other work in the tree). Leave the tree at the last good commit. Return success=false with notes explaining exactly what blocked byte-identity.

Return the structured result.`
}

function proofPrompt(el) {
  return `Independently and ADVERSARIALLY verify the migration of the \`${el}\` scene element to the generator/renderer/entity model. Assume nothing; your job is to catch any reproducibility break or shortcut (especially faked byte-identity). You are READ/VERIFY ONLY — do not modify code (you may run builds/tests).

Run these checks and report each:
1. \`go build ./...\` is clean (ignore cgo/metal "deprecated" warnings).
2. NO GOLDEN CHEATING: confirm internal/scene/testdata/golden.txt was NOT regenerated. Run \`git show --stat HEAD\` for the latest commit (the ${el} migration) and confirm testdata/golden.txt is NOT among the changed files. Also \`git log --oneline -5 -- internal/scene/testdata/golden.txt\` to confirm the goldens were not touched by this migration. If golden.txt was modified/committed, that is a CRITICAL failure → verified=false.
3. \`go test ./internal/scene -run TestGolden\` passes (whole seed matrix byte-identical).
4. A *SceneListRoundTrip test covering \`${el}\` exists and passes. Read it and confirm it genuinely (a) round-trips the scene list through YAML and (b) compares re-rendered PIXELS (a hash of canvas pixels), not merely struct/byte equality of the YAML. A test that only checks struct equality does NOT prove rendering reproducibility → roundTripVerifiesPixels=false.
5. Inspect \`git show HEAD\` (the migration diff): confirm the element's DRAWING math was only moved, not altered; the rng draw order is preserved; Render still equals Generate+RenderList; Generate does no drawing and RenderList does no rng; the entity schema is named <Name>V0 with explicit yaml tags and is registered via RegisterEntity; and "${el}.v0" is registered as BOTH a generator and a renderer.
6. \`go vet ./internal/scene\` is clean.

Set verified=true ONLY if every check passes. Otherwise verified=false with specific, actionable issues. Return the structured result.`
}

const results = []
for (let i = 0; i < ELEMENTS.length; i++) {
  const { el, note } = ELEMENTS[i]
  log(`(${i + 1}/${ELEMENTS.length}) migrating ${el}`)

  const mig = await agent(migratePrompt(el, note), {
    label: `migrate:${el}`, phase: 'Migrate', schema: MIG_SCHEMA,
  })

  if (!mig || !mig.success) {
    log(`✗ ${el}: migration failed${mig ? ' — ' + mig.notes : ' (agent died)'}`)
    results.push({ el, mig, proof: null, ok: false })
    // The migrator reverts its own changes on failure, leaving the tree at the
    // last good commit; continue so independent later elements still get a turn.
    continue
  }
  if (mig.skipped) {
    log(`• ${el}: already migrated, skipping`)
    results.push({ el, mig, proof: null, ok: true, skipped: true })
    continue
  }

  const proof = await agent(proofPrompt(el), {
    label: `proof:${el}`, phase: 'Proof', schema: PROOF_SCHEMA,
  })

  const ok = !!(proof && proof.verified)
  log(ok ? `✓ ${el}: migrated and proofed (${mig.commit.slice(0, 8)})`
        : `✗ ${el}: PROOF FAILED — ${proof ? proof.issues.join('; ') : 'agent died'}`)
  results.push({ el, mig, proof, ok })
}

const done = results.filter(r => r.ok)
const failed = results.filter(r => !r.ok)
return {
  migrated: done.map(r => r.el),
  failed: failed.map(r => ({ el: r.el, why: r.proof ? r.proof.issues : (r.mig ? r.mig.notes : 'agent died') })),
  detail: results.map(r => ({
    el: r.el, ok: r.ok, skipped: r.skipped || false,
    commit: r.mig && r.mig.commit, schemas: r.mig && r.mig.entitySchemas,
    proof: r.proof && { verified: r.proof.verified, goldenGreen: r.proof.goldenGreen, goldenFileUntouched: r.proof.goldenFileUntouched, roundTripVerifiesPixels: r.proof.roundTripVerifiesPixels, drawMathUnchanged: r.proof.drawMathUnchanged },
  })),
}
