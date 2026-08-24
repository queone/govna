# govna Architecture

## Purpose

Provide the Go implementation of govna while preserving the externally observable behavior of the frozen `govna-rust` reference unless an intentional difference is approved.

## System Summary

The repository is in its pre-implementation state. Product components, package boundaries, runtime flow, storage, and integrations remain unset until the parity AC defines the required behavior and the Director settles any architectural choices.

## Current Platform

- Go

## Major Components

- Governance and release scaffolding only.
- Product components not yet selected.

## Core Files

- `AGENTS.md`: base governance contract
- `plan.md`: prioritized roadmap and approved direction
- `build.sh`: self-contained build / release-prep / release script (Bash 3.2+, no external tools)
- `govna/development-cycle.md`: workflow from roadmap through release
- `govna/ac-template.md`: acceptance-criteria template for new work
- `govna/build-release.md`: build, test, and release rules

## Data And Control Flow

No product control flow exists yet.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

## Architecture Notes

- Treat `govna-rust` as the behavioral reference until validated Go parity.
- Record approved intentional differences in the owning AC, documentation, and tests.
- Defer package layout, dependencies, and implementation strategy to the parity AC.

## Conventions

- Update this document when architecture or major workflow changes materially.
- Keep implementation detail in code and stable architecture here.
