# govna

Planned Go successor to [`govna-rust`](https://github.com/queone/govna-rust), the frozen Rust implementation preserved as the behavioral reference.

## Why

Govna changes frequently. Go's faster edit-build-test cycle is expected to improve iteration speed while the Rust repository supplies a stable reference for externally observable behavior.

The dependency-free Go implementation provides top-level dispatch, deterministic rendering, `apply`, non-mutating audit, non-destructive removal assessment, and complete product tooling for CODE and DOC repositories. Apply preserves settled consumer-owned files, merges registered governance boundaries, emits an adoption AC, and optionally initializes Git on `main`. Audit validates durable state, classifies drift, and emits a guarded routing AC only for actionable results. Removal classifies delete, keep, and review routes into a guarded AC without executing them. The canonical build discovers and validates utilities before installation and runs ordinary pre- and post-change validation during Go release prep. Legacy `governa/` content is intentionally ignored. [`govna/parity.md`](govna/parity.md) defines the frozen-reference contract and approved differences.

## Product Tooling

Run `./build.sh` for the canonical full validation and installation path. Go builds emit no validation token. `./build.sh prep vX.Y.Z "message"` runs canonical validation before and after its mutations, then prints the release command without executing it. Validation-token and baseline-refresh behavior remains specific to rendered Rust tooling.

## Audit

Run `govna audit` from an adopted CODE or DOC Git worktree to compare its governed files with embedded canon. Audit validates metadata, baseline, and optional `govna/preserve.txt` state; classifies current-canon and evidenced target-only paths; and emits one guarded audit AC only when work is actionable. Use `--json` for the deterministic machine report. Audit does not apply routing decisions or mutate existing governed content.

## Removal

Run `govna rm` from an adopted CODE or DOC Git worktree to emit a Director-reviewed removal AC. Byte-equal pure canon is proposed for deletion, mixed or edited canon requires routing, and expected-divergence, preserved, and target-only paths stay out of scope. The command never performs a routed deletion and removes no control state itself.

## Governance

This repo is governed by an explicit session-entry contract for AI coding agents — see [`govna/operator-contract-rationale.md`](govna/operator-contract-rationale.md) for the design reasoning and [`AGENTS.md`](AGENTS.md) for the operational rules.

## AC Workflow

Here, "AC" names both the acceptance-criteria document—the change blueprint—and the governed change it tracks from Draft through Package.

Use the standalone action vocabulary `Draft → Audit → Refine → Implement → Ratify → Package` for an active AC. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation. Accept lowercase forms for the phase actions and `package`, `pack`, or `prep` for Package. Ordinary coding phrases such as `build`, `prepare the build`, and `package the binary` do not advance the workflow.
