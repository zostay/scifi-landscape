---
name: release
description: Cut a scifi-landscape release. Runs the full change-contract verification (reproducibility golden + additive-only schema contract + build/vet/gofmt/tests via `make verify`), then proposes a semver tag, confirms it with the user, and creates and pushes an annotated git tag. Use when the user asks to "release", "cut a release", "tag a release/version", or "ship a version". Light by design — it tags only; it does not build or publish binaries.
---

# Release

Cut a release of scifi-landscape: verify the change contract one last time, then tag the version. **Tagging only — never build or publish binaries.**

The frozen contract is the algorithm/schema versioned keys (`scene.v0`, `sky.v0`, `PlanetGasGiantV0`, …) enforced by the test suite; this skill's job is to confirm those gates pass and stamp a release tag.

Do the steps in order. **Stop and report** if any precondition or check fails — do not tag a repo that isn't clean and green.

## 1. Preconditions

- `git rev-parse --abbrev-ref HEAD` is `master` (the project's primary branch). If not, stop and tell the user.
- `git status --porcelain` is empty (clean working tree). If not, stop — the release must tag committed work only.
- In sync with the remote: `git fetch origin` then confirm `git rev-parse HEAD` == `git rev-parse origin/master`. If local is ahead, ask the user to push first; if behind/diverged, stop and report.

## 2. Verify the change contract

Run the gate and require it to pass:

```sh
make verify
```

This runs `go build`, `go vet`, a `gofmt` cleanliness check, and the full test suite — which includes:
- **`TestGolden`** — the reproducibility freeze (every seed renders byte-identical to its recorded golden).
- **`TestSchemaContract`** — the additive-only data freeze (no serialized field renamed/retyped/removed).
- **`TestGoldenCoversAllAlgorithms`** — every registered algorithm is covered by the golden suite.

If `make verify` fails, **stop** and surface the failing output. A golden or schema-contract failure means the change contract was broken (an algorithm changed behavior, or a released field changed) — that must be fixed (by adding a new versioned algorithm/schema, not editing a released one) before any release. Do not regenerate goldens or the schema contract to make the gate pass.

## 3. Choose the version (propose, then confirm)

- Find the latest release tag: `git tag -l 'v*' --sort=-v:refname | head -1`.
- Propose the next version:
  - **No tags yet** → propose **`v0.1.0`** (the first release; pre-1.0 to match the `v0` algorithm keys).
  - **Otherwise** → propose the next patch bump by default (e.g. `v0.1.0` → `v0.1.1`); mention the minor/major alternatives.
- **Ask the user to confirm or override** the version before tagging (use the question tool). Validate the chosen value is `vMAJOR.MINOR.PATCH` and is not already a tag (`git tag -l <version>` empty).

## 4. Tag and push

Create an annotated tag with a short one-line summary of what the release contains, then push just the tag:

```sh
git tag -a <version> -m "<one-line summary>"
git push origin <version>
```

## 5. Report

Tell the user the tag was created and pushed, and restate that the algorithm/schema versioned keys are now the frozen contract for this release — future changes to released algorithms or schemas must land as new versions (`*.v1`, `…V1`), enforced by `make verify`.

Keep it light: no binaries, no release notes file, no changelog generation unless the user explicitly asks.
