# govna Architecture

## Purpose

Provide a self-contained template repo for governed `CODE` and `DOC` repositories, plus a deterministic bootstrap tool (`govna apply`) that renders the template into target repos and can migrate a repo previously managed by governa (`apply` auto-detects a `governa/`-managed target and tracks the migration).

## System Summary

- a CLI skeleton dispatches four governed subcommands (`apply`, `drift-scan`, `rm`, `render-canon`), all implemented, plus a real `version`/`ver`/`--version` surface
- `build.sh` + `tests/build_cli.sh` provide the canonical Rust-stack build/release tooling
- `govna/` carries the root governance canon docs (`ac-template.md`, `development-guidelines.md`, `drift-scan.md`, `roles.md`, `canon-cycle.md`, `code-stacks.md`, `development-cycle.md`, `operator-contract-rationale.md`, `build-release.md`, `README.md`), describing govna's own implementation

The embedded-template machinery backs `render-canon`, `drift-scan`, and `apply` alike (`apply` reuses `render-canon`'s canon-rendering primitive rather than reimplementing it).

## Current Platform

- Rust CLI (`clap` declared in `Cargo.toml`, reserved for future per-subcommand flag parsing; current dispatch is hand-rolled against `std::env::args()` for exact stream/exit-code control)
- Bash build/release tooling (`build.sh`)

## Major Components

- `src/main.rs`: CLI entry point — subcommand dispatch, usage/help text, the real `version`/`ver`/`--version` surface, `render-canon` implementation
- `src/apply.rs`: `apply` implementation — mode detection, repo-shape assessment, config resolution, canon write via `governance::render_canonical_files` (hunk-merging `mixed_content_boundary`-registered hybrid files in existing mode instead of blind overwrite, skipping `README.md`/`CHANGELOG.md` when they already exist), adoption-AC emission with a real per-file outcome label, optional `git init`, and governa-managed-repo migration tracking folded into that same adoption AC as a `## Migration findings` section (legacy-metadata carry-over, plus classification — precise via a live `governa render-canon` comparison when the `governa` binary is available, crude enumeration otherwise)
- `src/rm.rs`: `rm` implementation — canon rendering, classification (In Scope/Out Of Scope/Review) against the target's actual files, emits a single removal AC; deletes nothing itself. Review items carry an on-demand `govna render-canon`/`diff -ru` recipe rather than a pre-computed diff — no companion diffs file
- `build.sh`: self-contained Bash script for local validation (`./build.sh`), release staging (`./build.sh prep …`), and release orchestration (`./build.sh vX.Y.Z "…"`); isolates Cargo compilation in an invocation-owned external target dir under `$TMPDIR`, deleted after each run
- `tests/govna_cli.rs` + `tests/build_cli.sh`: Rust integration tests (declared-binary CLI contract: `--version` exactness, usage/exit-code behavior) and the build-tooling's own smoke-test harness
- `govna/`: root governance canon docs, describing govna's own implementation

## Data And Control Flow

A user runs `govna <subcommand>`. `version`/`ver`/`--version`, `render-canon`, `drift-scan`, `apply`, and `rm` produce real output; an unrecognized subcommand prints usage to stderr and exits 2. `apply` bootstraps the current directory: writes the full canon set (hunk-merging hybrid files rather than overwriting them when they already exist, labeling each file's actual outcome in the emitted AC), a `CLAUDE.md` symlink, and a `govna/ac<N>-govna-apply.md` adoption record; `--init-git` optionally initializes git. When a `governa/` directory is detected, that same adoption record gets a version-suffixed name (`govna/ac<N>-govna-apply-<version>.md`) and a `## Migration findings` section tracking the legacy tree's review and removal — one file, not a separate migration AC. `rm` plans (but does not perform) canon removal: it emits `govna/ac<N>-govna-rm-<version>.md`, with on-demand comparison commands in place of pre-computed diffs, for Director review.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

Acceptance Criteria are non-runtime control artifacts for non-trivial changes. An AC carries Director intent through bounded Operator implementation and verification, then is deleted during release prep after durable decisions land elsewhere. `AGENTS.md` is authoritative for the AC threshold and gates.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo but does not re-bootstrap itself through `apply`; `apply` refuses to run against govna's own source checkout, per `emission::refuse_govna_source`
- `build.sh` is the canonical build/release tool; implementation lives in shell, not Rust
- ACs control non-trivial change flow but are not runtime architecture
- `govna/roles.md` defines the two-role model (Operator, Director) that supplements the shared governance contract
- justify every added crate in the governing AC (per `govna/code-stacks.md`); `clap` is currently the only external dependency, and is itself unused pending real flag-parsing work
- `templates/overlays/doc/files/AGENTS.md.tmpl` deliberately carries a narrower `## Base Rules` than root/CODE's — it omits `### Build Verification`, `### AC Mechanics`, `### Errors`, `### Versioning and Dependencies`, and `### Code Style and Conventions` entirely. This is intentional, not staleness: DOC-flavor consumers are typically free-form documentation repos with no build pipeline, no installable utilities, and no code-style surface for those sections to govern. Considered and rejected — do not re-flag this gap without a concrete DOC consumer scenario the current contract actually fails.
- govna has no `deps` subcommand for any CODE stack: a dependency-freshness wrapper is a thin shim around each stack's own tool (`cargo outdated` for Rust, `go list -m -u -json all` for Go, etc.) that consumers already know to reach for directly. Considered and rejected — do not re-flag this gap without a concrete need current native tooling fails to cover.
- govna has no background update-check / newer-release notice. Rejected on product-philosophy grounds: govna's whole model is a one-time `apply` after which the consumer repo owns its governance files independently, and canon-update adoption already has an explicit, deliberate mechanism (`drift-scan`'s emitted AC) — staying on an older govna binary isn't a problem state to nag about the way it would be for a continuously-run tool where currency matters (security patches, CI linters). Considered and rejected — do not re-flag this gap without a concrete scenario where binary staleness itself (not canon staleness, which `drift-scan` already covers) causes a real problem.

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
