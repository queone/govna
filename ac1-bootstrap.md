# AC1 Bootstrap govna project scaffolding

## Summary

Infrastructure-only AC. Stands up the `govna` Cargo project and a no-op CLI skeleton (`--version`/`--help` only) so later ACs have a working local build/test/lint loop to land against. No CI wiring and no governa port logic (canon rendering, drift-scan, template merge, AC tooling) happen in this AC — CI is deferred to a later AC, and porting starts once the piping is in place.

## In Scope

### Files to create

- `Cargo.toml` — package manifest, name `govna`, edition 2021, initial version `0.1.0`, `clap` as the only dependency
- `src/main.rs` — clap-based CLI entry point; supports `--version` and `--help` only, no subcommands yet
- `README.md` — one-paragraph description, build/test instructions, explicit note that this is a gradual port of `governa` from Go to Rust and not a drop-in replacement or dependency of it while the port is in progress
- `LICENSE` — MIT license
- `.gitignore` — cargo's default (`/target`) plus common editor cruft

## Out Of Scope

- Any actual porting of governa's Go logic (canon rendering, drift-scan, template hunk-merge, AC tooling)
- Publishing to crates.io
- Cross-compilation or release binary packaging
- Dependency choices beyond `clap` (e.g. `serde`, a template engine) — deferred to the AC that first needs them
- GitHub Actions CI wiring (`.github/workflows/`) — deferred to a later AC; ATs in this AC are local-only
- Adopting governa's own AC-workflow tooling/AGENTS.md in this repo — this AC file borrows governa's *format* only, as a personal convention, not governa-the-tool

## Acceptance Tests

**AT1** [Automated] — `cargo build` exits 0 from a clean clone.
**AT2** [Automated] — `cargo test` exits 0 (zero tests is fine; harness must run clean).
**AT3** [Automated] — `cargo fmt --check` exits 0.
**AT4** [Automated] — `cargo clippy -- -D warnings` exits 0.
**AT5** [Automated] — `./target/debug/govna --version` exits 0, prints `govna 0.1.0` (plus newline) to stdout, and writes nothing to stderr.
**AT6** [Manual] — `gh repo view govna` confirms the repo exists on GitHub with the initial commit pushed.

## Status

IN PROGRESS — AT1–AT5 pass locally. AT6 (`gh repo view`) is pending the commit/push below.
