# AC13 Investigate redundant dependency recompilation in the Rust build.sh template

## Summary

Investigation (infra, template-code-only if a safe fix is found). A single `./build.sh` run in any Rust CODE consumer visibly recompiles the same dependency graph three times — once each for `cargo clippy --all-targets --all-features` (`build.sh:612-618`), `cargo test --all-targets --all-features` (`build.sh:624-629`), and `cargo build --release` (`build.sh:635-639`) — despite all three sharing one `--target-dir`. Originally raised as `rkit` AC38; an Audit there confirmed (via direct diff) the affected block is byte-identical to `templates/overlays/code/stacks/rust/build.sh.tmpl` and to govna's own root `build.sh` — pure canon, not rkit-local — so the investigation and any fix belong here instead. This AC investigates whether the recompilation is reducible without weakening what each step verifies, and either lands a fix or records why it cannot be done.

## In Scope

### Investigation

- Determine, for each of the three `cargo` invocations above (and their `_build_scoped_phases`-path equivalents at `build.sh:697-728`; govna's own root `build.sh` and the Rust stack template are byte-identical, so either serves as the reference copy), which Cargo unit-graph identity differs (profile, feature set, `--target-dir` join, `RUSTFLAGS`, or target selection) and confirm which recompiles are inherent to Cargo's per-profile/per-feature-set artifact isolation versus incidental (e.g. differing `--all-features` vs plain flags, differing `--verbose` paths, environment-variable drift between steps).
- Preferred evidence method (stable-Rust-compatible, verified during Audit): run `./build.sh` and diff the `Compiling`-line counts across its own Clippy/Test/Build phase output, in invocation order, plus one repeat-invocation control (rerun the same phase's cargo command again against the same target-dir) to confirm same-command artifacts are stable/cacheable in isolation. Audit already ran this against govna's own repo: Clippy 23 compiles (cold), Test 37 more despite the shared `--target-dir`, Release build 48 more, and a clippy repeat immediately after clippy showed 0 — confirming the phenomenon is real and reproducible without extra tooling. `cargo build -Z unstable-options --unit-graph` is a secondary option only if a nightly toolchain happens to be available — govna's own tooling contract (`code-stacks.md` Rust section) requires only stable Cargo/rustfmt/Clippy, so do not assume nightly is present.
- Run any direct `cargo`/`rustc` diagnostic commands per `AGENTS.md`'s `### Build Verification` carve-out (scoped, target-dir outside the repo) — prefer observing `./build.sh`'s own output where possible, per the preferred method above.
- Evaluate concrete mitigations if any incidental divergence is found — e.g. `sccache`/`cargo build --timings`-driven profile alignment, sharing one invocation via `cargo hack`, a recommended `Cargo.toml` `dev`/`test` profile-alignment pattern (Cargo.toml is consumer-owned, not canon-templated — see Files to modify), or reordering steps so the release build reuses `test`-profile artifacts where Cargo's caching rules allow it.
- Record findings directly in this AC's `## Migration findings`-style notes section (added during Refine/Implement) rather than a separate document.

### Files to modify (conditional — only if a safe fix is identified)

- `templates/overlays/code/stacks/rust/build.sh.tmpl` — adjust cargo invocation flags/ordering only if doing so does not reduce clippy/test/release coverage; propagates to every Rust CODE consumer via their next `govna apply`/drift-scan cycle.
- `build.sh` (govna's own root copy) — mirror the same fix, keeping it byte-identical to the template per this repo's own dogfooding convention.
- `govna/code-stacks.md` `## Rust` — document a recommended `Cargo.toml` profile-alignment pattern only if that is the identified mitigation; Cargo.toml itself is consumer-owned and not canon-templated, so this is guidance, not enforcement.

## Out Of Scope

- Any change that causes `cargo clippy` or `cargo test` to run with fewer features/targets than `--all-targets --all-features` today — that would weaken existing verification, not just speed it up.
- Any change to the release binaries' build flags or optimization profile.
- Introducing a new build-acceleration dependency (e.g. `sccache`, `cargo-nextest`) unless explicitly approved by the Director as a separate follow-on decision; this AC may recommend one but not add it silently.
- Editing any consumer repo's local `Cargo.toml` directly (e.g. `rkit`'s) — this AC's output is a canon template change and/or documented guidance; consumers adopt it through their normal drift-scan/apply cycle.

## Investigation Findings

All evidence gathered via `./build.sh` output and, where a controlled comparison needed a different phase order than `./build.sh` runs, direct `cargo` invocations against an isolated `mktemp`-created target-dir outside the repo (per `AGENTS.md`'s diagnostic carve-out) — stable Rust throughout, no nightly tooling used.

**Baseline (current `build.sh` order: Clippy → Test → Release build), fresh target-dir:**

| Phase | `Compiling` lines |
|---|---|
| `cargo clippy --all-targets --all-features` (cold) | 23 |
| `cargo test --all-targets --all-features` | +37 |
| `cargo build --release` | +48 |
| **Total** | **108** |

A `cargo clippy` repeat immediately after the first clippy run, same target-dir, produced 0 `Compiling` lines — same-command artifacts are fully stable/cacheable in isolation. The redundancy is specifically cross-command, not a caching bug (e.g. no flakiness from timestamps or lockfile churn).

**Control (reversed order: Test → Clippy → Release build), fresh target-dir:**

| Phase | `Compiling` lines |
|---|---|
| `cargo test --all-targets --all-features` (cold) | 48 |
| `cargo clippy --all-targets --all-features` | +10 |
| `cargo build --release` | +48 |
| **Total** | **106** |

**Conclusion: inherent, not incidental. No fix applied.**

The clippy↔test pair shows real but *asymmetric* sharing (clippy-after-test only needs 10 more; test-after-clippy needs 37 more) — consistent with `cargo clippy` wrapping compilation through `clippy-driver` (`RUSTC_WORKSPACE_WRAPPER`), which changes Cargo's fingerprint for workspace-member units relative to a plain `rustc` build, and clippy performing check-oriented analysis that doesn't fully substitute for `cargo test`'s codegen needs in the clippy→test direction. But the practical question is total work, not direction: **106 vs. 108 total compiles is a ~2% difference — noise, not a meaningful win.** Reordering doesn't reduce total recompilation; it just relocates which phase pays for it. The Test↔Release pair shows zero sharing in either arrangement (48 compiles every time), consistent with Cargo never sharing `dev`/`test`-profile and `release`-profile artifacts — a hard Cargo boundary, not something any flag or ordering change can cross.

No combination of the three phases (Clippy, Test, Release build) can be made to share a meaningfully larger fraction of the compile graph without either weakening what a phase checks (Out Of Scope) or introducing a build-acceleration dependency like `sccache` (Out Of Scope without separate Director approval). `templates/overlays/code/stacks/rust/build.sh.tmpl` and govna's own root `build.sh` are unchanged — no fix to apply. `govna/code-stacks.md` is also unchanged: there's no `Cargo.toml` profile-alignment pattern to recommend, since the redundancy isn't a profile-configuration mismatch a consumer could tune away — it's Cargo's own check/build and dev/release artifact-separation model.

If this becomes a real pain point later, the durable fix is `sccache` (compiler-cache, not a `build.sh`-logic change) — a separate, explicit Director decision per this AC's Out Of Scope, not something this investigation should default into.

## Acceptance Tests

**AT1** [Manual] [Pre-release gate] — This AC's findings section states, for each of the three (or six, counting scoped-build variants) cargo invocations, whether its recompilation relative to the prior step is inherent to Cargo's profile/feature isolation or incidental, with supporting evidence (`Compiling`-line counts from `./build.sh`'s own phase output, `--timings` output, a nightly unit-graph diff if available, or equivalent). Audit's baseline run against govna's own repo (23/37/48 compiles across Clippy/Test/Build, 0 on a clippy repeat) is available as a reference point, not a substitute for Implement's own run.

**AT2** [Automated] [Pre-release gate] — If a fix is applied to `templates/overlays/code/stacks/rust/build.sh.tmpl` and/or govna's root `build.sh`, `./build.sh` still runs `cargo fmt --check`, `cargo clippy --all-targets --all-features -- -D warnings`, `cargo test --all-targets --all-features`, and `cargo build --release` to completion with unchanged pass/fail semantics in govna's own repo, and total wall-clock time for a clean run is recorded before and after the change.

**AT3** [Manual] [Pre-release gate] — If no safe fix is found, the findings section states this explicitly with the reason (e.g. "inherent to Cargo profile isolation; no action taken") and no template/`build.sh` change is made.

## Status

`DEFERRED` — frozen at Director's request pending a more exhaustive internet search for a viable mitigation (e.g. `sccache` or alternatives) before Ratify. Investigation Findings section stands as evidence gathered so far, not a final conclusion; do not delete or resume without explicit Director direction.
