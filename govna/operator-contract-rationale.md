# Operator Contract Rationale

This explanatory document records why the Operator contract exists. `AGENTS.md` alone defines operational rules and wins every conflict.

## Session-Entry Purpose

Session Entry tells a general-purpose agent that constrained repository rules apply before substantive work. It initializes contract identity, substantive-action scope, gates, precedence, and an observable checkpoint without restating the full contract. Audit catches residual drift.

## LLM-Agent Behavior Assumptions

The design assumes similarity-weighted retrieval, stronger compliance with imperative wording, and a modest primacy benefit for role framing. `AGENTS.md` therefore stays imperative and near the action; this document serves human onboarding without diluting that signal.

## The `Govna contract loaded.` Checkpoint

`Govna contract loaded.` is a human-visible readiness signal emitted only after internalizing `AGENTS.md` and before the first substantive governed action. Its narrow trigger keeps it meaningful; it detects rather than prevents contract skipping.

## Audit Verification

`govna audit` complements session framing by detecting canon incoherence, consumer adoption drift, and local-rule decay across sessions and repositories.

## Why Effective Implementation Scope Is Bounded

Effective implementation scope avoids repeating settled Director decisions for directly broken, deterministic fallout. It preserves behavior and intent, requires one valid outcome, records every use, and returns to Refine wherever product, scope, security, destructive, publication, release, dependency, migration, architecture, or competing-outcome judgment begins.

## Why Contract Integrity Reporting Is Evidence-Triggered

Evidence-triggered reporting distinguishes contract defects from implementation defects without inviting ambient opinion. Classification routes consumer-local, canon, or unclear findings but never grants editing authority. Blocking findings stop unsafe or decision-bearing work; unchanged acknowledged findings stay silent; authorized corrections land only in their owning governance document.

## Why Ratify Auto-Corrects Implementation-Only Findings

Ratify auto-corrects only defects already decided by the settled contract. Out Of Scope and Director-owned categories preserve the safety boundary, while the bounded retry loop prevents indefinite self-correction.

## Canon Versus Local Flexibility

Canon fixes shared roles, workflow, approvals, discipline, and review behavior. Consumers own non-conflicting `## Project Rules`, additional local governance documents, tooling, build scripts, and CI. Propose disputed canon upstream instead of creating permanent local drift.
