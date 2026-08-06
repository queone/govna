# AC11 Bare-Cargo Target Guard

## Summary

Doc-only AC (governance wording), single part. `govna/development-guidelines.md`'s Rust Practices already says "Keep Cargo compilation artifacts in the build-managed temporary target" — describing what `build.sh`'s `_create_cargo_target`/`_cleanup_cargo_target` mechanism actually does (build under a self-cleaning `mktemp -d` outside the repo) — but nothing prohibits running `cargo build`/`cargo check`/`cargo test` directly against the repo's default `./target`, outside a `./build.sh` validation cycle. A bare invocation silently defaults to `<cwd>/target`, which `.gitignore` hides but nothing cleans up. A consumer repo (`rkit`) accumulated a 1.1G orphaned `./target/` this way. Tighten `AGENTS.md`'s `### Build Verification` and `govna/development-guidelines.md`'s Rust Practices to close the gap, and mirror the change to every site those two files already propagate to.

Open question for Audit: whether a doc-level prohibition is sufficient, or whether it should be paired with a technical guardrail — e.g., a committed `.cargo/config.toml` `target-dir` override — so a bare `cargo build` can't default into the repo root even when the rule is missed. A static committed config path doesn't match `build.sh`'s existing per-invocation mktemp+cleanup model (no isolation across concurrent builds, nothing prunes it), so it isn't a drop-in win. Flagging rather than pre-deciding.

## In Scope

### Files to modify

- `AGENTS.md` — `### Build Verification`: add a bullet explicitly prohibiting bare `cargo` compilation commands (`build`/`check`/`test`/`clippy`, etc.) outside a `./build.sh` validation cycle; extend the existing "diagnose or correct a reported failure" carve-out to require an explicit `--target-dir` outside the repo when a direct command is authorized.
- `govna/development-guidelines.md` — Rust Practices: reword "Keep Cargo compilation artifacts in the build-managed temporary target" (or add an adjacent bullet) to name the failure mode directly (bare invocation defaulting to `./target`) so the rule reads as a prohibition, not just a description of `build.sh`'s own behavior.
- `templates/base/AGENTS.md` — mirror the `AGENTS.md` wording change (existing Project Rules mirror requirement).
- `templates/overlays/doc/files/AGENTS.md.tmpl` — mirror the same change (DOC flavor inherits `### Build Verification` unchanged from base; confirm no divergence introduced).
- `templates/overlays/code/files/govna/development-guidelines.md.tmpl` — mirror the `development-guidelines.md` change (CODE-only file; no DOC counterpart exists).

## Out Of Scope

- Any `.cargo/config.toml` guardrail or other technical enforcement — open question above, left for Audit/Refine to settle before Implement.
- Changes to `build.sh`'s own `_create_cargo_target`/`_cleanup_cargo_target` mechanism — already correct; not touched.
- Retroactive cleanup of any consumer repo's stray `./target/` — a per-repo operator action, not a govna change.
- `editing-guidelines.md` / DOC-flavor Rust-specific wording — DOC flavor has no Rust Practices section to begin with.

## Acceptance Tests

**AT1** [Automated] — `AGENTS.md`'s `### Build Verification` section contains a bullet prohibiting bare `cargo` compilation commands outside `./build.sh`.

**AT2** [Automated] — `govna/development-guidelines.md`'s Rust Practices section reflects the same prohibition (grep for updated wording).

**AT3** [Automated] — `diff` confirms `templates/base/AGENTS.md` and `templates/overlays/doc/files/AGENTS.md.tmpl` carry the same `### Build Verification` wording as govna source `AGENTS.md`.

**AT4** [Automated] — `diff` confirms `templates/overlays/code/files/govna/development-guidelines.md.tmpl` carries the same Rust Practices wording as govna source `govna/development-guidelines.md`.

**AT5** [Manual] — Director confirms the open `.cargo/config.toml` question is resolved (adopted or explicitly declined) before Implement.

## Status

`PENDING` — awaiting user authorization to begin implementation.
