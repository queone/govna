# govna Architecture

## Purpose

Add, compare, inspect, and remove Govna governance files predictably in CODE and DOC repositories. The versioned governance files built into Govna are its canon.

Govna automates deterministic mechanics and surfaces decision-bearing choices to the Director. This separation makes recurring programming and publishing ceremonies more effective and efficient without weakening authorization, review, verification, or release gates.

## System Summary

One dependency-free Go module handles the complete workflow. It selects the CODE or DOC file set, writes embedded files, compares adopted repositories, prepares removal plans, and validates Govna itself.

## Current Platform

- Go
- Interaction surface: terminal coding-agent CLIs (Claude Code, Codex CLI); IDE, desktop, mobile, and web agent interfaces are untested.

## Major Components

- `cmd/govna`: accepts commands and prints help, versions, colored terminal text, and command results.
- `internal/help`: renders every help page in the shared layout: three header lines, capitalized headings, rows aligned per section, the appended version and help rows, and single-sequence color only when the written stream supports it.
- `internal/canon`: stores the embedded governance files, fills repository values, combines CODE or DOC layers, and creates deterministic baselines.
- `internal/render`: writes a selected file set to a directory after resolving and validating command options.
- `internal/repository`: determines repository type, stack, module path, name, adoption state, and source-checkout identity, and provides the shared contained file access: path validation, link rejection, and reads and writes rooted at the resolved repository.
- `internal/apply`: adds Govna files to new or existing repositories, keeps repository-owned documents and local sections whether or not an agent instruction file exists, validates every destination before writing, and initializes optional Git state.
- `internal/audit`: rejects malformed saved Govna state before comparing files, treats an escaping baseline path as malformed, fails instead of guessing when a governed path is a link, a directory, a special file, or unreadable, explains each exact classification label, and reviews extra repository files only when specific Govna evidence identifies them.
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

Render selects a CODE or DOC file set (the flavor), asks `internal/canon` for path-sorted content, validates every destination through the shared contained access, and writes it without first emptying the target. It applies deterministic file modes and writes the baseline—the saved hashes of installed Govna-managed regions.

Apply determines repository identity through `internal/repository`, renders the selected embedded files, validates every destination, and either writes them into a new repository or merges registered Govna sections into an existing one while keeping repository-owned documents. Adding those files is adoption. `internal/emission` writes one adoption AC that names the executable version and canon version separately. Apply never reads or changes legacy `governa/` content. Optional Git initialization runs last.

Audit validates a repository that has adopted Govna, reads metadata, the baseline, the optional preserve registry—the files a Director chose to keep local—and the optional repository-check registry `govna/repo-check.txt`—the Director's standing answer to the emitted repository check—and compares Govna-managed regions in byte order through the shared contained access. A link, directory, special file, or unreadable governed path fails the audit before any emission, and preserve phrases come from the canonical Unreleased table row or a legacy Unreleased section. Each exact classification label explains whether a file needs no update, can be updated safely, stays local, or needs a Director choice. Clean audits write nothing. Actionable audits write or reuse one unedited AC keyed by canon version. A valid configured repository check emits pre-resolved to the configured command; a malformed registry fails the audit before any emission. Its marker records the executable and canon versions separately, and JSON uses the same report data. When an agent is explicitly asked to run the command, the Operator immediately reviews that AC, resumes no-edit Refine after blockers are resolved, runs the final readiness check, and stops before Implement. Active phase state remains in the session rather than the immutable AC.

Removal reads the same repository identity and preserve information. It compares current Govna files and examines repository-only entries without following symlinks. It sorts files into remove, keep, and Director-choice groups, removes preserve control state last, and writes or safely reuses one canon-version-keyed removal AC. The removal marker records executable and canon versions separately and upgrades unedited legacy markers. The command carries out no removal choice.

The canonical Go build discovers regular command entry points, checks their literal versions, compiles into invocation-owned external storage, validates the compiled programs, and only then replaces safe install destinations. Go release prep performs bookkeeping only, rejects any result outside its planned transformations, and prints the release command without running it. Validation-token and baseline-refresh behavior remains specific to Rust tooling.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

Integrated audit adoption is the only command-mediated phase exception. It can advance one emitted adoption AC through immediate Audit and no-edit Refine, but it cannot enter Implement. Every unpackaged AC with implementation in the unreleased state enters the pending release batch, including work awaiting Ratify. A private pre-Implement calculation prevents that complete batch from growing beyond one 80-byte prefix-plus-summary message. Package requires every member to be Ratified, rejects excluded implemented work, and rechecks the complete batch before prep. A named request such as `Package AC70+AC71` establishes a fitting multi-AC batch; a standalone Package alias reuses the complete batch already established in the active session. A direct batch, one with no implemented AC awaiting release, packages direct-handled changes with a release message that names no AC.

## Audit Emission Contract

Audit emits each adoption AC in the shape below. These rules bind `internal/audit` and its tests; a consumer repository receives the emitted AC, not this contract.

### Emitted AC instruction and phase shape

- Name each emitted adoption AC `# AC<N> Adopt Govna Governance Files v<CANON_VERSION>`.
- Place the repository paragraph first under `## Summary`.
- Start the repository paragraph with `This AC updates`.
- Follow it with `The result label (classification)`.
- Place the count paragraph after the repository paragraph.
- Start the count paragraph with `Govna found`.
- Keep the count and Summary paragraphs descriptive.
- Place one `### Audit Review` section before `### Adoption Instructions`.
- Bind Audit Review to the resolved executable and emitted marker versions.
- Require one unique scratch render outside the consumer repository.
- Emit one executable review command for every actionable path.
- Apply `### Mixed-content sync verification` to every existing mixed-content review target.
- Require exact rule, overlap, placement, reference, contract-growth, and acceptance-evidence review.
- Require exact scratch cleanup before the Audit report.
- Omit rendered diff bodies from the emitted AC.
- Omit companion review artifacts.
- Confirm each file selected for update exists in the selected CODE render.
- Place that CODE-render check and all routing procedure under `### Adoption Instructions`.
- Omit the CODE-render check from DOC audit emissions.
- Emit each adoption instruction as one imperative bullet.
- Format every numbered routing entry as one Director decision question.
- End every numbered routing entry with `?`.
- Keep shared implementation procedure out of routing questions.
- End every emitted adoption AC with exact status `` `PENDING` — immutable audit emission; workflow state is tracked in the active session.``

### Mixed-content sync verification emission

- Capture the SHA-256 digest of each existing mixed-content target from the first byte of its exact registered boundary-heading line through end of file.
- Include the boundary line, its line ending, the complete repository-owned tail, and the final-newline state in the protected region.
- Emit the expected digest and boundary in the file-specific automated acceptance test for every direct sync.
- Emit the same conditional verification for every review item whose Director resolution is sync.
- Keep the protected-region digest out of classification, baseline scope, and JSON output.

### Conditional routing verification

- Emit a conditional rendered-region check for each offered sync outcome.
- Emit a conditional preserve-registry exclusion check for each offered sync outcome.
- Emit a conditional target-presence check for each offered preserve outcome.
- Emit a conditional preserve-registry inclusion check for each offered preserve outcome.
- Emit a conditional target-absence check for each offered delete outcome.
- Emit a conditional preserve-registry exclusion check for each offered delete outcome.
- Emit a conditional named-destination check for each offered migration outcome.
- Emit a conditional source check for each offered migration outcome.
- Emit a conditional canon-backed destination check for each offered migration outcome.
- Emit a conditional repository-owned destination check for each offered migration outcome.
- Emit a conditional preserve-registry check for each canon-backed migration outcome.
- Emit a replacement-before-retired-source check for each replacement-missing route.
- Emit a referenced-target state check for each marker-only route.
- Emit a conversion registry check for each marker-only route.
- Emit a removal registry check for each marker-only route.
- Emit an exact-phrase absence check for each legacy-phrase route.
- Emit a target-before-phrase check for each independently actionable legacy-phrase route.
- Emit an unrelated-Summary preservation check for each legacy-phrase route.
- Emit an outside-Summary preservation check for each legacy-phrase route.
- Keep every emitted routing check atomic.
- Keep emitted AT numbering stable across identical reports.

- Apply repository-check inference when baseline installation or replacement is present.
- Infer the repository check only from bounded target governance evidence.
- Accept positive declarations only from exactly one AGENTS.md rule shaped ``Run `<command>` as the first validation command ...`` and exactly one rule shaped ``Use `<command>` for repository-wide ... validation ...``.
- Require both positive declarations to name `./build.sh` for CODE inference.
- Require root `build.sh` to resolve to a regular file for CODE inference.
- Require the selected CODE stack's recognized root manifest before inferring `./build.sh`.
- Recognize `go.mod`, `Cargo.toml`, `Package.swift`, and `.terraform.lock.hcl` or a root `*.tf` for Go, Rust, Swift, and Terraform respectively.
- Require each recognized manifest path used as evidence to resolve to a regular file.
- Treat selected-stack manifest evidence only as proof that the declared repository command can run.
- Keep exact AGENTS.md declarations as the repository-command authority.
- Infer `Not applicable` for DOC only when `govna/release.md` contains the exact canon no-automated-content-validation declaration and AGENTS.md contains no recognized positive declaration.
- Leave missing, duplicate, incomplete, mismatched, positive-plus-negative, non-`./build.sh`, or non-regular-file evidence unresolved for a Director decision.
- Leave absent, non-regular, or other-stack-only selected-manifest evidence unresolved for a Director decision.
- Ignore unrelated manifests, other prose, governance documents, executables, CI files, and flavor defaults.

- Record inferred repository-check evidence without requesting Director confirmation.
- Emit the repository-check outcome pre-resolved to the configured command.
- Omit the repository-check question and its manual resolution AT when the check is inferred.
- Emit an unresolved repository check as the final numbered routing decision.
- Use the exact unresolved repository-check question recorded in the note below.
- Emit one manual resolution AT for an unresolved repository check.
- Place the manual repository-check AT after every protected-region AT.
- Emit one automated verification AT for an unresolved repository check.
- Place the automated repository-check AT immediately after its manual AT.
- Use singular nouns in emitted count summaries only for a count of one.
- Use plural nouns in emitted count summaries for zero or multiple counts.

Note: exact unresolved repository-check question: ``<N>. **Repository check**: Which command should run after the selected file updates, or what repository evidence shows that no command applies?``

## Architecture Notes

- Record approved intentional differences in the owning AC, documentation, and tests.

## Conventions

- Update this document when architecture or major workflow changes materially.
- Keep implementation detail in code and stable architecture here.
