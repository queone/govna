# govna

Planned Go successor to [`govna-rust`](https://github.com/queone/govna-rust), the frozen Rust implementation preserved as the behavioral reference.

## Why

Govna changes frequently. Go's faster edit-build-test cycle is expected to improve iteration speed while the Rust repository supplies a stable reference for externally observable behavior.

The dependency-free Go command foundation implements top-level dispatch, usage, version output, Rust-compatible terminal color gating, and frozen help for the reserved render and audit routes. Operational command behavior remains staged work. [`govna/parity.md`](govna/parity.md) defines the contract against the frozen Rust reference, including approved successor-owned identity substitutions. S2 embedded canon and rendering are next.

## Governance

This repo is governed by an explicit session-entry contract for AI coding agents — see [`govna/operator-contract-rationale.md`](govna/operator-contract-rationale.md) for the design reasoning and [`AGENTS.md`](AGENTS.md) for the operational rules.

## AC Workflow

Here, "AC" names both the acceptance-criteria document—the change blueprint—and the governed change it tracks from Draft through Package.

Use the standalone action vocabulary `Draft → Audit → Refine → Implement → Ratify → Package` for an active AC. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation. Accept lowercase forms for the phase actions and `package`, `pack`, or `prep` for Package. Ordinary coding phrases such as `build`, `prepare the build`, and `package the binary` do not advance the workflow.
