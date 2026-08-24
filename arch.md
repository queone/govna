# govna Architecture

## Purpose

Provide the Go implementation of govna while preserving the externally observable behavior of the frozen `govna-rust` reference unless an intentional difference is approved.

## System Summary

The S1 command foundation and S2 embedded renderer are implemented as a dependency-free Go module. [`govna/parity.md`](govna/parity.md) defines behavioral boundaries and stages S1 through S6.

## Current Platform

- Go

## Major Components

- `cmd/govna`: command dispatch, version and usage output, terminal-color gating, frozen render and audit help, and deferred operational handlers.
- `internal/canon`: embedded canon assets, substitutions, overlay composition, and deterministic baseline generation.
- `internal/render`: render argument handling, cwd inference, validation, and filesystem emission.
- `internal/repository`: shared repository identity and source-checkout resolution.
- `internal/apply`: fresh/existing adoption, protected writes, boundary merging, symlink handling, and optional Git initialization.
- `internal/emission`: monotonic adoption-AC numbering across files and Git history.
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

`main` detects stderr terminal capability and passes it with environment lookup and output writers to the command runner. The runner routes render requests into `internal/render`, which resolves cwd identity and asks `internal/canon` for a path-sorted in-memory file set. The renderer writes that set without pre-cleaning, applies deterministic modes, installs the baseline, and recreates the `CLAUDE.md` symlink. Audit and removal remain temporary handlers until S4 and S5 replace them.

Apply resolves the cwd through `internal/repository`, renders the same canon set, then writes all files in new mode or preserves and boundary-merges settled paths in existing mode. `internal/emission` writes one ordinary adoption AC. Apply never reads or changes legacy `governa/` content. Git initialization is an optional final step and uses `main` only.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

## Architecture Notes

- Treat `govna-rust` as the behavioral reference until validated Go parity.
- Record approved intentional differences in the owning AC, documentation, and tests.
- Keep S1 on the standard library; defer later package layout, dependencies, and implementation strategy to each stage's owning implementation AC.

## Conventions

- Update this document when architecture or major workflow changes materially.
- Keep implementation detail in code and stable architecture here.
