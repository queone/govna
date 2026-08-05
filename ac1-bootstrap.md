# AC1 Bootstrap govna project scaffolding

## Summary

Infrastructure-only AC. Stands up the `govna` Cargo project, CI pipeline, and a no-op CLI skeleton (`--version`/`--help` only) so later ACs have a working build/test/lint loop to land against. No governa port logic (canon rendering, drift-scan, template merge, AC tooling) happens in this AC — that starts once the piping is in place.

## In Scope

### Files to create

- `Cargo.toml` — package manifest, name `govna`, edition 2021, initial version `0.1.0`, `clap` as the only dependency
- `src/main.rs` — clap-based CLI entry point; supports `--version` and `--help` only, no subcommands yet
- `.github/workflows/ci.yml` — GitHub Actions workflow running `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test`, and `cargo build --release` on push and PR
- `README.md` — one-paragraph description, build/test instructions, explicit note that this is a Rust learning port of `governa` (Go) and not a drop-in replacement or dependency of it
- `LICENSE` — MIT license
- `.gitignore` — cargo's default (`/target`) plus common editor cruft

## Out Of Scope

- Any actual porting of governa's Go logic (canon rendering, drift-scan, template hunk-merge, AC tooling)
- Publishing to crates.io
- Cross-compilation or release binary packaging
- Dependency choices beyond `clap` (e.g. `serde`, a template engine) — deferred to the AC that first needs them
- Adopting governa's own AC-workflow tooling/AGENTS.md in this repo — this AC file borrows governa's *format* only, as a personal convention, not governa-the-tool

## Acceptance Tests

**AT1** [Automated] — `cargo build` exits 0 from a clean clone.
**AT2** [Automated] — `cargo test` exits 0 (zero tests is fine; harness must run clean).
**AT3** [Automated] — `cargo fmt --check` exits 0.
**AT4** [Automated] — `cargo clippy -- -D warnings` exits 0.
**AT5** [Automated] — `./target/debug/govna --version` exits 0, prints `govna 0.1.0` (plus newline) to stdout, and writes nothing to stderr.
**AT6** [Automated] — The GitHub Actions CI workflow runs AT1–AT4 on push and reports green.
**AT7** [Manual] — `gh repo view govna` confirms the repo exists on GitHub with the initial commit pushed.

## Status

PENDING — awaiting authorization to begin implementation.
