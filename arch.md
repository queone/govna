# govna Architecture

## Purpose

Provide the Go implementation of govna while preserving the externally observable behavior of the frozen `govna-rust` reference unless an intentional difference is approved.

## System Summary

The S1 command foundation, S2 embedded renderer, S3 apply flow, S4 audit engine, S5 removal assessment, and S6 product tooling are implemented as a dependency-free Go module. [`govna/parity.md`](govna/parity.md) defines the completed behavioral boundary.

## Current Platform

- Go

## Major Components

- `cmd/govna`: command dispatch, version and usage output, terminal-color gating, and operational render, apply, audit, and removal handlers.
- `internal/canon`: embedded canon assets, substitutions, overlay composition, and deterministic baseline generation.
- `internal/render`: render argument handling, cwd inference, validation, and filesystem emission.
- `internal/repository`: shared repository identity and source-checkout resolution.
- `internal/apply`: fresh/existing adoption, protected writes, boundary merging, symlink handling, and optional Git initialization.
- `internal/audit`: strict durable-state parsing, ordered canon classification, bounded target-only evidence, deterministic reports, and non-mutating audit orchestration.
- `internal/remove`: deterministic delete, keep, and review classification with no-follow target traversal and non-destructive removal-AC emission.
- `internal/buildtest`: isolated product-tooling fixtures, traceability, and normalized output contracts.
- `internal/emission`: monotonic AC numbering and guarded stem/version-keyed stub reuse with body-hash edit detection.
- Governance, release scaffolding, and the behavioral-parity contract.

## Core Files

- `AGENTS.md`: base governance contract
- `cmd/govna/main.go`: executable entry point and S1 command runner
- `govna/parity.md`: frozen-reference behavioral contract and traceability matrix
- `govna/parity-index.txt`: deterministic frozen-reference test index
- `govna/parity-check.sh`: parity-contract generator and mechanical verifier
- `plan.md`: prioritized roadmap and approved direction
- `build.sh`: self-contained build / release-prep / release script (Bash 3.2+, no external tools)
- `govna/development-cycle.md`: workflow from roadmap through release
- `govna/ac-template.md`: acceptance-criteria template for new work
- `govna/build-release.md`: build, test, and release rules

## Data And Control Flow

`main` detects stderr terminal capability and passes it with environment lookup and output writers to the command runner. The runner routes render requests into `internal/render`, which resolves cwd identity and asks `internal/canon` for a path-sorted in-memory file set. The renderer writes that set without pre-cleaning, applies deterministic modes, installs the baseline, and recreates the `CLAUDE.md` symlink. Removal remains a temporary handler until S5 replaces it.

Apply resolves the cwd through `internal/repository`, renders the same canon set, then writes all files in new mode or preserves and boundary-merges settled paths in existing mode. `internal/emission` writes one ordinary adoption AC. Apply never reads or changes legacy `governa/` content. Git initialization is an optional final step and uses `main` only.

Audit resolves and validates an adopted Git worktree, parses metadata plus baseline and preserve control state, and compares canon-owned regions in byte order. It uses baseline hashes for ordinary drift, bounded Git history only for first-baseline migration, and merged baseline, tombstone, all-stack cross-flavor, or divergent-reference evidence for target-only paths. Clean audits allocate and write nothing. Actionable audits write or reuse only one unedited canon-version-keyed AC stub; JSON uses the same report model.

Removal resolves the same repository identity and strict preserve state, then compares existing current-canon files and traverses target-only entries without following symlinks. It sorts ordinary deletion, keep, and review routes; places preserve control-state removal last; and writes or byte-identically reuses one guarded removal AC. It executes no route and excludes only its eligible emitted stub from target-only classification.

The canonical Go build discovers regular command entry points, validates their literal versions, compiles into invocation-owned external storage, validates compiled output, and only then replaces safe install destinations. Go release prep runs ordinary canonical validation before and after mutation and prints rather than executes the release command. Validation-token and baseline-refresh behavior remains specific to Rust tooling.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

## Architecture Notes

- Preserve `govna-rust` as the frozen behavioral reference for regression comparison.
- Record approved intentional differences in the owning AC, documentation, and tests.
- Keep S1 on the standard library; defer later package layout, dependencies, and implementation strategy to each stage's owning implementation AC.

## Conventions

- Update this document when architecture or major workflow changes materially.
- Keep implementation detail in code and stable architecture here.
