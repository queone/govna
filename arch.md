# govna Architecture

## Purpose

Add, compare, inspect, and remove Govna governance files predictably in CODE and DOC repositories. The versioned governance files built into Govna are its canon.

Govna automates deterministic mechanics and surfaces decision-bearing choices to the Director. This separation makes recurring programming and publishing ceremonies more effective and efficient without weakening authorization, review, verification, or release gates.

## System Summary

One dependency-free Go module handles the complete workflow. It selects the CODE or DOC file set, writes embedded files, compares adopted repositories, prepares removal plans, and validates Govna itself.

## Current Platform

- Go

## Major Components

- `cmd/govna`: accepts commands and prints help, versions, colored terminal text, and command results.
- `internal/canon`: stores the embedded governance files, fills repository values, combines CODE or DOC layers, and creates deterministic baselines.
- `internal/render`: writes a selected file set to a directory after resolving and validating command options.
- `internal/repository`: determines repository type, stack, module path, name, adoption state, and source-checkout identity.
- `internal/apply`: adds Govna files to new or existing repositories while protecting local sections and optional Git state.
- `internal/audit`: rejects malformed saved Govna state before comparing files, explains each exact classification label, and reviews extra repository files only when specific Govna evidence identifies them.
- `internal/remove`: determines which files can be deleted, must be kept, or need a Director choice without following symlinks or deleting content.
- `internal/buildtest`: checks product tooling in isolated repositories with stable expected output.
- `internal/emission`: chooses the next AC number, reuses an unedited AC for the same canon version, and rejects edited generated bodies.
- Governance and release scaffolding.

## Core Files

- `AGENTS.md`: base governance contract
- `cmd/govna/main.go`: executable entry point and command runner
- `plan.md`: prioritized roadmap and approved direction
- `build.sh`: self-contained build / release-prep / release script (Bash 3.2+, no external tools)
- `govna/development-cycle.md`: workflow from roadmap through release
- `govna/ac-template.md`: acceptance-criteria template for new work
- `govna/build-release.md`: build, test, and release rules

## Data And Control Flow

The executable first detects whether stderr supports terminal color. Its command runner then sends each request to the matching package with explicit output writers and environment access.

Render selects a CODE or DOC file set (the flavor), asks `internal/canon` for path-sorted content, and writes it without first emptying the target. It applies deterministic file modes, writes the baseline—the saved hashes of installed Govna-managed regions—and recreates the `CLAUDE.md` symlink.

Apply determines repository identity through `internal/repository`, renders the selected embedded files, and either writes them into a new repository or merges registered Govna sections into an existing one. Adding those files is adoption. `internal/emission` writes one adoption AC that names the executable version and canon version separately. Apply never reads or changes legacy `governa/` content. Optional Git initialization runs last.

Audit validates a repository that has adopted Govna, reads metadata, the baseline, and the optional preserve registry—the files a Director chose to keep local—and compares Govna-managed regions in byte order. Each exact classification label explains whether a file needs no update, can be updated safely, stays local, or needs a Director choice. Clean audits write nothing. Actionable audits write or reuse one unedited AC keyed by canon version. Its marker records the executable and canon versions separately, and JSON uses the same report data. When an agent is explicitly asked to run the command, the Operator immediately reviews that AC, resumes no-edit Refine after blockers are resolved, runs the final readiness check, and stops before Implement. Active phase state remains in the session rather than the immutable AC.

Removal reads the same repository identity and preserve information. It compares current Govna files and examines repository-only entries without following symlinks. It sorts files into remove, keep, and Director-choice groups, removes preserve control state last, and writes or safely reuses one canon-version-keyed removal AC. The removal marker records executable and canon versions separately and upgrades unedited legacy markers. The command carries out no removal choice.

The canonical Go build discovers regular command entry points, checks their literal versions, compiles into invocation-owned external storage, validates the compiled programs, and only then replaces safe install destinations. Go release prep validates before and after its edits and prints the release command without running it. Validation-token and baseline-refresh behavior remains specific to Rust tooling.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

Integrated audit adoption is the only command-mediated phase exception. It can advance one emitted adoption AC through immediate Audit and no-edit Refine, but it cannot enter Implement. Every unpackaged AC with implementation in the unreleased state enters the pending release batch, including work awaiting Ratify. A private pre-Implement calculation prevents that complete batch from growing beyond one 80-byte prefix-plus-summary message. Package requires every member to be Ratified, rejects excluded implemented work, and rechecks the complete batch before prep. A named request such as `Package AC70+AC71` establishes a fitting multi-AC batch; a standalone Package alias reuses the complete batch already established in the active session.

## Architecture Notes

- Record approved intentional differences in the owning AC, documentation, and tests.

## Conventions

- Update this document when architecture or major workflow changes materially.
- Keep implementation detail in code and stable architecture here.
