# govna Architecture

## Purpose

Provide a self-contained template repo for governed `CODE` and `DOC` repositories, plus a deterministic bootstrap tool (`govna apply`) that renders the template into target repos. A Rust port of governa (`github.com/queone/governa`) — the end state is govna replacing governa as the canonical implementation, not remaining a permanent dependent of it.

## System Summary

This repo is early in its port from governa's mature Go implementation — most of the surface area described below is not built yet. Currently:

- a CLI skeleton dispatches five governed subcommands (`apply`, `drift-scan`, `rm`, `deps`, `render-canon`); `render-canon`, `drift-scan`, `apply`, and `rm` are implemented, `deps` remains the sole stub printing "not yet implemented", plus a real `version`/`ver`/`--version` surface
- `build.sh` + `tests/build_cli.sh` provide the canonical Rust-stack build/release tooling, adopted from governa's own canonical tooling and renamed to govna's identifiers
- `govna/` carries the root governance canon docs (`ac-template.md`, `development-guidelines.md`, `drift-scan.md`, `roles.md`, `canon-cycle.md`, `code-stacks.md`, `development-cycle.md`, `operator-contract-rationale.md`, `build-release.md`, `README.md`) — largely still governa's real mature-implementation content (embedded FS, `cmd/governa/main.go`, specific Go function names), not yet rewritten to describe govna's own implementation; each gets rewritten when its corresponding feature actually ports, per `plan.md`

The embedded-template machinery backs `render-canon`, `drift-scan`, and `apply` alike (`apply` reuses `render-canon`'s canon-rendering primitive rather than reimplementing it).

## Current Platform

- Rust CLI (`clap` declared in `Cargo.toml`, reserved for future per-subcommand flag parsing; current dispatch is hand-rolled against `std::env::args()` for exact stream/exit-code parity with governa's Go CLI)
- Bash build/release tooling (`build.sh`)

## Major Components

- `src/main.rs`: CLI entry point — subcommand dispatch, usage/help text, the real `version`/`ver`/`--version` surface, `render-canon` implementation, stub for `deps`
- `src/apply.rs`: `apply` implementation — mode detection, repo-shape assessment, config resolution, canon write via `governance::render_canonical_files`, adoption-AC emission, optional `git init`
- `src/rm.rs`: `rm` implementation — canon rendering, classification (In Scope/Out Of Scope/Review) against the target's actual files, emits a removal AC and a companion diffs file; deletes nothing itself
- `build.sh`: self-contained Bash script for local validation (`./build.sh`), release staging (`./build.sh prep …`), and release orchestration (`./build.sh vX.Y.Z "…"`); isolates Cargo compilation in an invocation-owned external target dir under `$TMPDIR`, deleted after each run
- `tests/govna_cli.rs` + `tests/build_cli.sh`: Rust integration tests (declared-binary CLI contract: `--version` exactness, usage/exit-code behavior) and the build-tooling's own smoke-test harness
- `govna/`: root governance canon docs — see System Summary for current-vs-aspirational content status

## Data And Control Flow

A user runs `govna <subcommand>`. `version`/`ver`/`--version`, `render-canon`, `drift-scan`, `apply`, and `rm` produce real output; `deps` prints "not yet implemented" to stderr and exits 1. `apply` bootstraps the current directory: writes the full canon set, a `CLAUDE.md` symlink, and a `govna/ac<N>-govna-apply.md` adoption record; `--init-git` optionally initializes git. `rm` plans (but does not perform) canon removal: it emits `govna/ac<N>-govna-rm-<version>.md` and a companion `-diffs.md` for Director review.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

Acceptance Criteria are non-runtime control artifacts for non-trivial changes. An AC carries Director intent through bounded Operator implementation and verification, then is deleted during release prep after durable decisions land elsewhere. `AGENTS.md` is authoritative for the AC threshold and gates.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo, but does not re-bootstrap itself through `apply` (governa's own convention for its source repo; `apply` also refuses to run against govna's own source checkout, per `emission::refuse_govna_source`)
- `build.sh` is the canonical build/release tool; implementation lives in shell, not Rust
- ACs control non-trivial change flow but are not runtime architecture
- `govna/roles.md` defines the two-role model (Operator, Director) that supplements the shared governance contract
- justify every added crate in the governing AC (per `govna/code-stacks.md`); `clap` is currently the only external dependency, and is itself unused pending real flag-parsing work
- do not blind-rename `governa` → `govna` in the `govna/` canon docs' prose describing mature Go-implementation specifics — rewrite each doc's content when its corresponding feature actually ports (see `plan.md`)

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
