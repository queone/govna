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
- `src/apply.rs`: `apply` implementation — mode detection, repo-shape assessment, config resolution, canon write via `governance::render_canonical_files` (hunk-merging `mixed_content_boundary`-registered hybrid files in existing mode instead of blind overwrite, skipping `README.md`/`CHANGELOG.md` when they already exist), adoption-AC emission, optional `git init`, and governa-managed-repo migration tracking (legacy-metadata carry-over, plus an emitted migration AC — precise via a live `governa render-canon` comparison when the `governa` binary is available, crude enumeration otherwise)
- `src/rm.rs`: `rm` implementation — canon rendering, classification (In Scope/Out Of Scope/Review) against the target's actual files, emits a single removal AC; deletes nothing itself. Review items carry an on-demand `govna render-canon`/`diff -ru` recipe rather than a pre-computed diff — no companion diffs file
- `build.sh`: self-contained Bash script for local validation (`./build.sh`), release staging (`./build.sh prep …`), and release orchestration (`./build.sh vX.Y.Z "…"`); isolates Cargo compilation in an invocation-owned external target dir under `$TMPDIR`, deleted after each run
- `tests/govna_cli.rs` + `tests/build_cli.sh`: Rust integration tests (declared-binary CLI contract: `--version` exactness, usage/exit-code behavior) and the build-tooling's own smoke-test harness
- `govna/`: root governance canon docs — see System Summary for current-vs-aspirational content status

## Data And Control Flow

A user runs `govna <subcommand>`. `version`/`ver`/`--version`, `render-canon`, `drift-scan`, `apply`, and `rm` produce real output; `deps` prints "not yet implemented" to stderr and exits 1. `apply` bootstraps the current directory: writes the full canon set (hunk-merging hybrid files rather than overwriting them when they already exist), a `CLAUDE.md` symlink, and a `govna/ac<N>-govna-apply.md` adoption record; `--init-git` optionally initializes git. When a `governa/` directory is detected, `apply` also emits `govna/ac<N>-govna-migrate-from-governa-<version>.md` tracking its review and removal. `rm` plans (but does not perform) canon removal: it emits `govna/ac<N>-govna-rm-<version>.md`, with on-demand comparison commands in place of pre-computed diffs, for Director review.

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
- `templates/overlays/doc/files/AGENTS.md.tmpl` deliberately carries a narrower `## Base Rules` than root/CODE's — it omits `### Build Verification`, `### AC Mechanics`, `### Errors`, `### Versioning and Dependencies`, and `### Code Style and Conventions` entirely. This is intentional, not staleness: DOC-flavor consumers are typically free-form documentation repos with no build pipeline, no installable utilities, and no code-style surface for those sections to govern. Considered and rejected — do not re-flag this gap without a concrete DOC consumer scenario the current contract actually fails.
- govna does not port governa's dependency-freshness subcommand. governa's version is a thin wrapper around `go list -m -u -json all` — Go's own built-in query, just recolored. Rust's ecosystem already has a more capable, actively-maintained equivalent (`cargo outdated`), so a govna-built wrapper would be a strictly worse reimplementation of a tool every Rust developer already knows to reach for. Considered and rejected — consumers should run `cargo outdated` directly; do not re-flag this gap without a concrete need it fails to cover.
- govna does not port governa's binary-install-freshness notice (a background check-for-a-newer-release-on-every-invocation, cached and best-effort). Rejected on product-philosophy grounds, not just because govna/governa are invoked infrequently: govna's whole model is a one-time `apply` after which the consumer repo owns its governance files independently, and canon-update adoption already has an explicit, deliberate mechanism (`drift-scan`'s emitted AC) — staying on an older govna binary isn't a problem state to nag about the way it would be for a continuously-run tool where currency matters (security patches, CI linters). Considered and rejected — do not re-flag this gap without a concrete scenario where binary staleness itself (not canon staleness, which `drift-scan` already covers) causes a real problem.

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
