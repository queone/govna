# Canon Specification

This repository-owned document records what the Govna tools and build scripts do. No consumer receives it, and it carries no rule for a consumer.
Use `govna/canon-maintenance.md` for the procedures of a canon change, and `AGENTS.md` `### Canon Maintenance` for its rules.

## Metadata and retired routing marker

- Require `schema_version`, `canon_version`, and `repo_type`.
- Require `code_stack` only for CODE consumers.
- Skip legacy marker compatibility because Govna never shipped a marker file.
- Write metadata during `render`/`apply`.
- Write `govna/canon-baseline.txt` during `render` and `apply` as the saved hashes of Govna-managed file regions.
- Route an existing target path as retired when it remains in the prior baseline but disappears from current canon.
- Use the retired-path tombstone registry, the saved list of old Govna paths, for removals that predate baseline adoption.
- Preserve unrelated repository-owned governance documents unless specific baseline, retired-path, other-flavor, or governed-file evidence connects them to Govna.

These sections specify `govna audit` beyond what `govna/audit.md` tells a consumer.

## Classification order and file handling

The classification is the exact result label beside each file. Audit applies these checks in order:

| Order | Condition | Classification | Plain meaning |
|---|---|---|---|
| 1 | The file is missing and the preserve list records it. | `match` | The file needs no Govna update because the Director chose the omission. |
| 2 | The file is missing and has no preserve entry. | `missing-in-target` | A file from the embedded Govna files is missing from the repository. |
| 3 | The complete file matches the embedded Govna file. | `match` | The file needs no Govna update. |
| 4 | The Govna-managed section of a mixed file matches. | `match` | The repository-owned section is ignored and the Govna section needs no update. |
| 5 | The expected-difference list names the file. | `expected-divergence` | The repository is expected to keep its own version. |
| 6 | The file differs and the preserve list names it. | `preserve` | The preserve list says to keep the repository's version. |
| 7 | The file differs but still matches its saved baseline region. | `clear-sync` | The file is safe to update because its Govna-managed region has no local edits. |
| 8 | The file differs and does not match a saved baseline region. | `ambiguity` | Govna cannot safely choose between updating and keeping the file. |

`govna/metadata.txt` gets metadata-specific handling layered on top. An absent file is forced to `migration-required` regardless of the byte-comparison result (see Migration-required items). A present `canon_version` must use strict `vMAJOR.MINOR.PATCH` form. When the target version is lower than embedded canon and replacing only that field makes the whole file byte-equal to rendered canon, the file is forced to `clear-sync` regardless of git history or a preserve-registry entry. Other metadata differences remain whole-file review items. A malformed version fails before AC emission; a target version newer than embedded canon also fails and directs the operator to upgrade govna rather than downgrade consumer metadata.

Files absent from the selected embedded Govna files use `target-has-no-canon` only when specific baseline, retired-path, other-flavor, or governed-file evidence connects them to Govna. See Target-only detection.

Audit reads every governed path through a contained handle on the resolved repository root and never follows a symbolic link.

- Treat a path as missing only when nothing exists at it.
- Fail the audit before emission when a governed path is a symbolic link, a directory, a special file, or unreadable.
- Name the failed path and its recovery action in that failure.
- Print no tally and no partial JSON report after that failure.
- Leave every existing emitted AC unchanged after that failure.
- Apply the same failure to a preserved path that exists but cannot be read.

## Format-defining files

`govna/ac-template.md` and `AGENTS.md` are format-defining: any non-`match`, non-`expected-divergence` classification for these two files forces a sync entry in the emitted stub regardless of what the ordered check above produced (an `ambiguity` or `preserve` result still surfaces as a forced-sync note, since these two files define the shape every other AC and canon doc depends on).

## Expected-divergence registry

`plan.md` and `arch.md` are registered as expected per-repo divergence — canon ships them as content stubs, and every adopting repo is expected to carry repo-specific content in their place. Divergence here never routes to review. `## IE spacing` names the one exception for `plan.md`.

## Target-only detection

Audit classifies an existing target as `target-has-no-canon` when the path is absent from current flavor canon, the preserve registry does not name it, and at least one bounded evidence source identifies it: the valid prior baseline, the pre-baseline retired-path tombstone registry, other-flavor canon, or a path reference from an already-divergent governed file. A reference to a consumer-owned `govna/ac<N>-<slug>.md` document is not evidence, because `plan.md` AC-pointers name drafted ACs by design. Evidence is merged by target path with tombstone replacement metadata retained, then emitted in deterministic path order.

The tombstone registry bridges removals that predate baseline adoption. It currently records `govna/drift-scan.md` as replaced by `govna/audit.md`. A missing current-canon replacement already appears as a direct update. The emitted AC names and installs that replacement before routing the retired source to preserve, explicitly named migration, or delete. It never offers restore as a separate routing outcome.

Audit does not flag arbitrary consumer-owned governance documents that have none of these evidence sources. Audit never deletes or migrates a target file itself.

- Require every evidence path to be a normalized repository-relative path.
- Drop an escaping reference or legacy phrase path without inspecting it.
- Inspect each candidate path without following links.
- Treat an absent candidate or a directory as no evidence.
- Fail the audit with a replace-the-link recovery action when a candidate path is a symbolic link.

## Migration-required items

`govna/metadata.txt` or `govna/canon-baseline.txt` absent from an otherwise govna-adopted target classifies as `migration-required`. Every emitted AC includes `## Migration findings` after `## Out Of Scope`: it lists each migration path and completion action, or `None` when no migration exists. Migration paths also remain under `## In Scope`.

## Markdown identifiers

- Apply these rules only when audit writes an adoption AC for another reason.
- Check the Markdown text that the repository keeps after the adoption AC with the rules of the Markdown identifier scan.
- Treat the text from the boundary heading to the end of the file as the kept text of a file that the AC updates from canon.
- Skip a file that the AC replaces whole with canon or deletes.
- Attach each finding to its file without changing the file's classification.
- List each file with findings under `### Bare identifiers to fix`, with each line number and identifier.
- Add one acceptance test per listed file.
- Treat the findings of a file that needs a Director choice as actionable only when its resolved action keeps the current text in place.
- Write no adoption AC for findings alone, for an up-to-date clean result or a version-only update.

Note: a repository that keeps its own `build.sh` runs no Markdown identifier scan, so its clean and version-only audit results stay silent about bare identifiers.

## IE spacing

- Apply these rules to the root `plan.md` when audit classifies it as an expected local difference.
- Treat an IE item as a `- IE<N>: ` line plus the indented lines that continue it.
- Treat two IE items as adjacent when only blank lines separate them.
- Flag `plan.md` when any adjacent pair lacks exactly one blank line between them.
- Keep the flagged file's classification unchanged.
- Treat the flag as work that writes an adoption AC on its own.
- Exit `3` from check mode for the flag on its own.
- Set `ie_spacing` to `true` on the flagged file in the JSON report.
- Name the flag in the plain result tally.
- List the flagged file under `### IE items to space` instead of `## Out Of Scope`.
- Add one acceptance test that checks the file's line order and IE spacing.

## Retired link

Govna used to create `CLAUDE.md` as a symbolic link to `AGENTS.md`. Claude Code v2.1.277 or later reads `AGENTS.md` directly, so audit retires that link.

- Classify a root `CLAUDE.md` symbolic link whose target is exactly `AGENTS.md` as `retired-link`.
- List the link under `### Retired Govna link` in the emitted AC as a deletion with no Director choice.
- Add the acceptance test `Verify CLAUDE.md no longer exists.` for the link.
- Report nothing for any other `CLAUDE.md`.

Audit prints two hints to stderr on every run. The hints leave the exit code, stdout, the JSON report, and the emitted AC unchanged. The deletable hint links Anthropic's `AGENTS.md` documentation.

- Print an upgrade hint when `claude --version` reports a version lower than 2.1.277.
- Skip the upgrade hint when `claude` is absent, fails, exceeds two seconds, or prints an unreadable version.
- Print a deletable hint when the repository root holds its own regular file or symbolic link named `CLAUDE.md`.

## Boundary-less build guide

Treat an existing CODE `govna/build-release.md` without its registered boundary as a one-time reviewed migration. Route it to `ambiguity` for full-file review even when a legacy whole-file preserve phrase exists, and retain that phrase as migration evidence. During reapply, leave the boundary-less file unchanged and emit a manual migration item. Place reviewed repository-specific release mechanics below the new boundary, sync rendered canon above it, and remove any obsolete registry entry and exact legacy phrase only through the consumer's authorized adoption cycle. Keep DOC `govna/release.md` outside this mixed-content model.

## Preserve registry handling

`govna/audit.md` `## Preserve registry` gives a consumer the registry format and its add and remove rules.

- Classify an existing target-only path named in the preserve registry as `preserve`.
- Keep that preserved target-only path visible in audit and JSON results.
- Omit another routing question for that preserved target-only path.

Exclude `govna/preserve.txt` from rendered canon, canon baselines, ordinary audit drift, name-referenced target-only evidence, and ordinary rm target-only content. Include it in rm only as the final control-state deletion after applying all registered preserve decisions.

Treat only exact legacy preserve phrases in the Unreleased CHANGELOG Summary as migration evidence: `preserve <path>`, `do not sync <path>`, `intentional divergence: <path>`, and `<path>: keep local`. Route each phrase under `## Legacy phrase routing`. Remove it only after verifying its required target and registry state. Preserve unrelated Summary text and historical rows. Ignore matching prose in historical CHANGELOG rows, emitted ACs, and every other governance document.

- Read the Summary from the canonical `| Unreleased | <summary> |` table row, where each `\|` pair is one escaped pipe.
- Read the Summary from a legacy `## Unreleased` heading section as well.
- Combine both sources in table-then-heading order.
- Drop a duplicate phrase-and-path pair.
- Ignore a phrase whose path is not a normalized repository-relative path.
- Accept LF or CRLF line endings in the changelog.

A registry entry on a missing current-canon file suppresses `missing-in-target` to a suppressed `match`; an entry on a divergent current-canon file or an existing target-only file routes it to `preserve` instead of a review classification. Exceptions are an eligible stale-version-only `govna/metadata.txt`, whose canon-owned `canon_version` cannot be pinned, and a boundary-less CODE `govna/build-release.md`, which remains a reviewed migration.

## Legacy phrase routing

- Define marker-only evidence as an exact Unreleased CHANGELOG preserve phrase whose referenced path has no independent file action.
- Offer conversion to `govna/preserve.txt` or exact-phrase removal for marker-only evidence.
- Add the referenced path to `govna/preserve.txt` for a conversion choice.
- Leave the referenced target unchanged during marker-only conversion.
- Remove the converted phrase after registry verification.
- Remove only the exact phrase for a marker-only removal choice.
- Preserve the referenced target during marker-only phrase removal.
- Preserve unrelated registry state during marker-only phrase removal.
- Apply each independently actionable file's capability-specific route before legacy-phrase cleanup.
- Treat a preserve choice on that file as conversion of its legacy phrase.
- Verify the result of every resolved sync, migration, or deletion before legacy-phrase cleanup.
- Remove the exact legacy phrase after that verification.

## Repository-check handling

`govna/audit.md` `## Repository-check registry` gives a consumer the file format and its authorization rules.

- Name `govna/repo-check.txt` as the standing-resolution source in the emitted acceptance test.
- Keep the no-baseline-migration `Not applicable` outcome ahead of the configured command.
- Prefer the configured command over inferred validation evidence.
- Require the configured check to succeed before `govna/canon-baseline.txt` installation.
- Exclude `govna/repo-check.txt` from rendered canon, canon baselines, ordinary audit drift, and name-referenced target-only evidence.

## Child registry in audit

A CODE parent lists each child repository in optional `govna/children.txt`:

```text
govna-children-v1
<parent-relative-path>
```

- Require the header, a final newline, and nonempty, unique, byte-sorted entries.
- Require each entry to be a slash-normalized path relative to the parent root, with `..` components only at its start.
- Reject absolute paths, backslashes, control characters, `.` components, and leading or trailing slashes.
- Require each entry to resolve to a real directory outside the parent worktree that does not contain the parent.
- Stop audit before emission when the registry is malformed or a child cannot be read.
- Stop audit and rm before emission when a DOC repository has the registry.
- Exclude the registry from rendered canon, canon baselines, ordinary audit drift, and target-only evidence.
- Compare each child's files with `govna render --child` for the parent's stack.
- Name each child file by its parent-relative path in the Markdown and JSON file lists.
- Force-sync a differing or missing child pointer or build tooling file.
- Give a differing child `.gitignore` the ordinary Director choices.
- Check only the canonical heading and two-column table shape of a child `CHANGELOG.md`.
- Classify a child `CHANGELOG.md` with a broken shape as `manual-fix`.
- Write the orphan note from `govna render --orphan` when the parent drops a child.

## Retired files

A retired file is a path that the recorded baseline holds and the selected current canon no longer does.

- Classify an existing retired file as `retired-file` when its recorded scope is `full` and its content matches its recorded hash.
- Keep `target-has-no-canon` and its routing question for a retired file that the repository edited, that has a legacy preserve phrase, or that has a bounded scope.
- Classify a retired file named in the preserve registry as `preserve`.
- List each `retired-file` under `### Retired Govna files` in the emitted AC as a deletion with no Director choice.
- Add the acceptance test ``Verify `<path>` no longer exists.`` for each `retired-file`.
- Treat a `retired-file` as work that writes an adoption AC and rules out a version-only result.

## References to retired files

- Apply these rules only when audit writes an adoption AC for another reason.
- Scan the Markdown text that the repository keeps after the adoption AC for the path of each retired file, present or absent.
- Treat the text from the boundary heading to the end of the file as the kept text of a file that the AC updates from canon.
- Skip a file that the AC replaces whole with canon or deletes.
- Skip `CHANGELOG.md`, `history.md`, AC documents, and test fixtures, because they record past work.
- Count the repository-relative path anywhere on a line, unless a longer path contains it.
- Count a Markdown link that uses the bare file name from a file in the same directory.
- Attach each match to its file without changing the file's classification.
- List each file with matches under `### References to retired files`, with each line number and retired path.
- Add one acceptance test per listed file.
- Mark a file's matches as conditional when one names a retired file that waits on a Director choice.
- Set `retired_references` in the JSON report only when audit writes an adoption AC.
- Write no adoption AC for matches alone, for an up-to-date clean result or a version-only update.

## Canon baseline manifest

`govna/canon-baseline.txt` is the baseline: the saved hashes of the Govna-managed file regions previously installed in the repository. Its first line is `govna-canon-baseline-v1`, its second line is `canon_version = vMAJOR.MINOR.PATCH`, and each sorted remaining line is `<path><TAB><scope><TAB><sha256>`. Scope is `full` or `before:<boundary-heading>`. The manifest excludes itself and `govna/preserve.txt`; neither is classified as an ordinary governed file.

Audit fails before emission for malformed fields, duplicate or unsorted paths, invalid hashes, unknown or mismatched scopes, an entry that is not a normalized repository-relative path, or a baseline canon version newer than embedded canon. A valid manifest missing one file entry routes that divergent file to `ambiguity`. Audit leaves the baseline unchanged; the emitted AC installs or replaces it last after all other work succeeds.

- Reject an entry that is absolute, contains a backslash or control character, or has an empty, `.`, or `..` component.
- Direct the consumer to correct the invalid entry before retrying.
- Retain a valid entry for a file absent from current canon as target-only evidence.

- Accept legacy `full` scope only for `govna/build-release.md` in a CODE target whose baseline canon version predates v0.11.0.
- Retain the legacy hash only as migration evidence.
- Apply normal boundary migration and comparison behavior after parsing.
- Reject the exception for DOC targets, other paths, v0.11.0-or-newer baselines, and every other mismatched scope.
- Leave the accepted baseline unchanged during audit.
- Replace it with the rendered bounded baseline only as the emitted adoption AC's final step.

## Canon-coherence precondition

Before comparing anything against the target, audit checks that Govna's rendered files agree with each other. This deterministic precondition checks only embedded canon. It does not replace the consumer-equivalent candidate-canon review in `govna/canon-maintenance.md`.

- Require `govna/roles.md` to reference the release document present in the selected flavor.
- Reject a `govna/roles.md` reference to the absent opposite-flavor release document.
- Require `AGENTS.md` to define the structured Package completion report.
- Require the selected release guide to end that structured report with the exact release command.
- Reject retired command-only Package wording.
- Require `AGENTS.md` to carry the integrated-audit Refine-entry exemption and pre-Implement stop.

Audit skips target comparison when any coherence rule fails. The error names the conflicting rendered file and directs the Operator to report it to the Govna maintainer.

## Emitted AC stub

Audit writes `govna/ac<N>-audit-<canon-version>.md` only when the repository has files ready to update, required control files to add, files needing a Director choice, a retired Govna link or file to delete, or `plan.md` IE items to space. The canon version identifies the embedded governance-file version; `N` follows the monotonic AC-numbering rule. Clear-sync, missing-target, migration-required, ambiguity, target-has-no-canon, retired-link, retired-file, and format-defining forced-sync results require work. Match, expected-divergence, and ordinary preserve results do not. The generated AC follows `govna/ac-template.md` and groups every non-`match` file as follows:

- **Files ready to update** — `clear-sync`, `missing-in-target`, and any format-defining file forced to sync.
- **Required control files** — `migration-required` items under `## Migration findings`.
- **Out of scope** — files that stay unchanged: `preserve` and `expected-divergence`.
- **Files needing a Director choice** — `ambiguity` and `target-has-no-canon`.
- **Retired Govna link** — `retired-link`: the `CLAUDE.md` link that the AC deletes with no Director choice.
- **Retired Govna files** — `retired-file`: each unedited retired file that the AC deletes with no Director choice.
- **References to retired files** — each kept Markdown file that names a retired file.
- **IE items to space** — a `plan.md` flagged under `## IE spacing`.

The stub carries an edit-detection marker (SHA-256 body hash). Re-running audit against an unedited stub for the same canon version reuses the same AC number. When audit writes an AC, it replaces each unedited stub for an older canon version: the new stub keeps that stub's AC number when no stub for the current canon exists, and audit deletes the older stub after writing and reports the replacement. An edited older stub, or more than one older stub without a current one, stops audit before it writes anything. Re-running it against an edited stub fails and directs the Director to delete or rename that generated file before retrying.

An audit with no updates or Director choices exits successfully and prints `No Govna updates or Director choices found`, followed by a plain result tally and `No AC was written.` It performs no AC-number allocation, stub inspection, directory creation, or file write. It never deletes, overwrites, or validates an existing audit stub. With `--json`, the complete report remains available and `emitted` is `null`; no additional prose is written.

A check-mode audit (`-c` or `--check`) runs the same comparison and writes nothing. It performs no AC-number allocation, stub inspection, directory creation, or file write. A clean result prints the clean-result line above and exits `0`. An actionable result prints `Govna updates or Director choices found`, followed by a plain result tally and `Run govna audit without --check to write the review AC.`, and exits `3`. With `--json`, the complete report is printed with `emitted` set to `null` and no additional prose. Because check mode emits no AC, it starts no agent-mediated adoption review.

A version-only update writes nothing either. When the only actionable results are `govna/metadata.txt` and `govna/canon-baseline.txt`, `govna/metadata.txt` differs only in `canon_version`, and the recorded baseline differs from the rendered one only in its version line and the `govna/metadata.txt` hash, both audit and check-mode audit print `Only the canon version changed: canon v<new> changes no Govna file in this <profile> repository (recorded v<old>). No AC was written.` and exit `0`. The profile is the CODE stack or `DOC`. Preserved files still count; child drift, a retired link, a retired file, or a forced update rules a version-only result out. The two stamps keep the last canon version that changed the repository until a real update moves them. With `--json`, `version_only` is `true` and `emitted` is `null`.

Emitted acceptance tests verify updates, required control files, every offered routing outcome, replacement ordering, legacy-phrase cleanup, and preservation according to the Director's choices. The pre-install rendered-file check covers declared update items except `govna/canon-baseline.txt`, review targets selected for update, and migration destinations backed by embedded Govna files. After all selected work, the chosen repository command must succeed, or the `Not applicable` evidence must hold. Only after every other applicable automated AT and routing outcome passes does the baseline get installed and verified separately from the same temporary render as the final step.

Every audit-emitted AT carries exactly one source axis and one explicit timing axis. Current audit ATs use `[Automated] [Pre-release gate]` or `[Manual] [Pre-release gate]`; none defer verification until after release.

Pass `--json` to print a machine-readable report (`header`: invocation, canon SHA, target, flavor and its source, repo name, govna/code-stack versions from metadata; `files`: one entry per scanned file with its classification, effective classification when force-synced, diff, prior commits, matched preserve-registry entries, legacy preserve-phrase evidence, canon reference, and mixed-content boundary where applicable; `emitted`: the stub's path for actionable reports or `null` for clean reports).

## First-Class CODE Stacks

- Use only Go, Rust, Swift, or Terraform as selectable CODE stacks.
- Use this reference for their selection, validation, installation, release prep, and scoped builds.
- Keep each stack's child override and pin commands in `internal/canon/assets/profile/children/pins/`.

## Multi-Utility Versioning

- Apply `govna/build-release.md` `## Independent Utility Versions` to every installable utility.
- Let each stack adapter choose declaration syntax and source layout while reporting the normalized utility contract.

## Release Command

- Print one shell-safe two-argument release command after prep in every stack.
- Emit the validated release tag bare in the printed release command.
- Emit the release message single-quoted with POSIX quote escaping in the printed release command.
- Confine the tag to `v`, digits, and dots through each adapter's existing validation before emission.
- Route each adapter's release-command prints through one emit helper.
- Reject a Rust, Terraform, or Swift release whose tag, message, or CHANGELOG row prep did not prepare in the current repository before the approval prompt.
- Accept `--yes` or `-y` after the two release arguments in every stack.
- Print the same candidate files and release sequence under `--yes` as the interactive release.
- Skip only the approval prompt under `--yes`.
- Keep `--yes` out of the printed release command.
- Refuse a release from a detached `HEAD` before the approval prompt in every stack.
- Push the branch with `git push -u origin HEAD` when it has no upstream.
- Keep each stack's branch push when the branch has an upstream.

## Child Repositories

- Read `govna/children.txt` at the start of every build in every stack.
- Reject a registry without the `govna-children-v1` header, a final newline, or unique, byte-sorted, slash-normalized relative paths.
- Run each declared child's `build.sh` before the parent's own build steps during a full build.
- Run a child only when every build tooling file matches the parent's copy byte for byte.
- Treat `build.sh` as the build tooling in Go, Swift, and Terraform, and `build.sh` plus `tests/build_cli.sh` in Rust.
- Stop the parent build when a child is missing, sits inside the parent, contains the parent, has different build tooling, or fails its build.
- Name the child and its recovery action in every child-build failure.
- Skip child builds during a scoped build and print one line saying so.
- Keep each child build's own side effects, such as installing a Go child's utilities.
- Stop prep and release when a local override resolves to a declared child.
- Name the child, the override, and the pin update that removes it in that message.
- Accept a local override to a declared child during a full build.
- Accept every local override that resolves outside every declared child.
- Clone each declared child at its registry path before a full build, including in CI.
- Run a scoped build in a clone that lacks its children.

## Markdown Identifier Check

- Scan only root `AGENTS.md`, `README.md`, `arch.md`, `plan.md`, `history.md`, `CHANGELOG.md`, and every Markdown file under `govna/` for bare AC, AT, and IE identifiers.
- Count a symbolic link by its own path.
- Run the scan before the first formatting step of every full and scoped build.
- Run the scan at the start of every prep, before any write or dry-run report.
- Fail on each identifier that `AGENTS.md` `### AC Mechanics` and `### Child Repositories` do not allow.
- Treat an uppercase AC, AT, or IE followed by digits, with no adjacent letter, digit, or underscore, as bare.
- Treat a one-line `<!-- Historical: ... -->` comment as a Markdown `Historical:` comment.
- Treat a repository as a child when its `AGENTS.md` has a line that starts with `This repository is a Govna child repository.`.
- Skip `tests/fixtures/` and every `testdata/` directory.
- Name the file, line, identifier, and a placeholder fix such as `AC<N>` in each finding.
- Keep the scan in one helper block that is identical in every CODE build script.

## Go

- Discover utilities from regular `cmd/<target>/main.go` files in byte order.
- Validate scoped target names against discovered utilities before running mutable build steps.
- Reject unknown, duplicate, and path-containing scoped targets.
- Compile each selected utility once in an invocation-owned external temporary directory.
- Validate every selected compiled `--version` output before installing any selected utility.
- Validate every selected compiled utility's help output against `## CLI Usage Formatting` before installing any selected utility.
- Require `-h`, `-?`, and `--help` to print identical help on stdout with exit 0 and no escape sequence.
- Require help line three to be the `go.mod` module path, optionally followed by `/` plus a sub-path.
- Require a utility README at `cmd/<name>/README.md`, when present, to carry a `### Usage` text block byte-equal to the help.
- Replace each installed utility atomically from an adjacent staging file.
- Preserve each installed utility until its replacement succeeds.
- Require every prep-changed version declaration to equal the unprefixed release tag.
- Accept each `\|` pair in an existing summary cell as one escaped pipe during changelog shape validation.
- Reject every release-message pipe, escaped or raw.
- Emit no validation token from Go builds.
- Remove invocation-owned build, coverage, version-probe, and installation-staging outputs on every handled exit.
- Terminate after handling HUP, INT, or TERM.
- Infer Go from `go.mod`.
- Select Go explicitly with `--stack Go`.
- Require the Go toolchain and the pinned staticcheck version installed by `build.sh`.
- Pin `GOTOOLCHAIN` in `build.sh` to the `go.mod` `toolchain` directive, or else to its `go` directive, before the first Go command.
- Name the pinned staticcheck, the active `GOTOOLCHAIN`, and both remedies when staticcheck cannot decode the toolchain's export data.
- Run dependency tidying, formatting, fixes, vetting, tests with coverage, staticcheck, and compilation.
- Install command binaries into `$(go env GOPATH)/bin`.
- Bump the single detected `programVersion` during release prep.
- Derive the primary utility name from the `go.mod` module path without its major-version suffix.
- Reject a release tag at v2 or later when the `go.mod` module path lacks the matching `/v<major>` suffix.
- Accept a repository with no commits during release prep.
- Capture the candidate Git tree during a release-prep dry run.
- Accept command names for scoped builds while retaining package-wide shared validation.
- Capture the complete candidate Git tree without changing the repository index.
- Create the first commit during release in a repository with no commits.
- Display the candidate files and exact release sequence before approval.
- Reject a changed candidate tree after approval.
- Verify the staged and committed trees against the approved candidate.
- Require the release commit to use the exact approved message.
- Require clean committed `HEAD` before release compilation.
- Reuse an approved retry commit only when the tag is absent, Git state is clean, the full commit message matches, and prepared metadata matches.
- Compile each discovered utility once in byte order with `go build -mod=readonly -buildvcs=true -o <temporary-output> -ldflags '-s -w' ./cmd/<target>`.
- Run no canonical validation phase during release compilation.
- Validate every compiled utility version and committed-HEAD provenance before installation.
- Validate every compiled utility's help output before installation during release compilation.
- Preserve independent secondary utility versions during release compilation.
- Install every validated utility by atomic adjacent replacement.
- Recheck every installed utility version and committed-HEAD provenance before tagging.
- Recheck every installed utility's help output before tagging.
- Create the release tag only after compilation, validation, installation, and rechecks pass.
- Push nothing after an earlier release failure.
- Remove invocation-owned release outputs on every handled exit.
- Release a repository with no `cmd/<target>/main.go` from its tag alone, without compiling or installing a utility.
- Skip `go mod tidy` in a full build only while that workspace names a declared child.
- Print one line that names the child when the build skips `go mod tidy`.

## Rust

- Infer Rust from `Cargo.toml`.
- Select Rust explicitly with `--stack Rust`.
- Require Cargo, rustfmt, and Clippy.
- Run formatting, Clippy, tests, and release compilation.
- Keep compilation in an invocation-owned external Cargo target.
- Install binaries into `$CARGO_HOME/bin`, or `$HOME/.cargo/bin` when `CARGO_HOME` is unset.
- Bump the root package version during release prep.
- Refresh `Cargo.lock` during release prep.
- Accept declared binary names for scoped builds.
- Preserve package-wide shared validation during scoped builds.
- Require one literal `PROGRAM_VERSION: &str` strict stable SemVer declaration in each declared binary path.

## Terraform

- Infer Terraform from `.terraform.lock.hcl` or root Terraform files.
- Select Terraform explicitly with `--stack Terraform`.
- Require the Terraform CLI.
- Run recursive formatting checks and module validation.
- Keep Terraform working data in repository-local ignored artifact directories.
- Derive release versions from Git tags without a source version bump.
- Reject scoped builds because Terraform validation is repository-wide.

## Swift

- Infer Swift from a root `Package.swift`.
- Select Swift explicitly with `--stack Swift`.
- Prefer Go, Terraform, and Rust manifests over Swift.
- Require Swift 6.0 or newer, Git, and one root SwiftPM package on macOS or Linux.
- Run strict toolchain formatting, debug compilation, tests, and release compilation with compiler warnings as errors.
- Keep SwiftPM artifacts in one invocation-owned external scratch directory and clean it on success, failure, and handled signals.
- Keep project-level `.swiftpm/` configuration trackable.
- Keep `Package.resolved` tracked for leaf packages with dependencies.
- Treat `Package.resolved` as optional for dependency libraries.
- Derive release versions from Git tags.
- Leave `Package.swift` unchanged during release prep.
- Install executable products into `${SWIFT_BIN_HOME:-$HOME/.local/bin}` by atomically replacing regular destination files and refusing unsafe entries.
- Let library-only packages complete without installation.
- Use the canonical color and plain-text presentation policy for build, prep, and release output.
- Accept executable-product names for scoped builds while retaining package-wide formatting and tests.
- Build only selected executable products during scoped builds.
- Install only selected executable products during scoped builds.
- Treat native Xcode projects and Apple application bundles as a possible future backend.

## CODE Build Presentation

- Reuse the canonical build color policy and palette across supported CODE stacks.
- Color phase headings, command previews, status values, failures, prep output, and release output by semantic role.
- Emit plain output when stdout is not a terminal.
- Emit plain output when `NO_COLOR` is set.
- Emit plain output when `TERM=dumb`.
- Require a 256-color-capable terminal before emitting ANSI sequences.
- Preserve plain-text content and output streams when color is disabled.
- Keep self-contained build scripts compatible with Bash 3.2.

## What prep does

`./build.sh prep` performs release bookkeeping in every CODE stack. Go and Terraform prep perform bookkeeping only. Rust prep adds a fallback pre-change full build when validation evidence is missing or stale and one post-change full build after its writes. Swift prep runs the canonical build before its writes and again with installation after them unless `-B` skips both. Every stack follows these bookkeeping phases:

1. **Validate inputs.** Require strict stable SemVer and one non-empty, single-line, table-safe message of no more than 80 bytes.
2. **Validate git state.** Inside a git work tree, target tag does not exist yet, HEAD is not at the latest tag with a clean working tree. Go accepts a repository with no commits.
3. **Scan Markdown identifiers.** Run the scan in `## Markdown Identifier Check` and stop on any finding before any write, including under `--dry-run` or `-n`.
4. **Process version targets.**
   - Detect every version target.
   - Validate every version target.
   - Follow this repository's Project Practices.
   - Follow the stack build implementation.
   - Reject missing, malformed, duplicate, or unsafe targets before any write.
   - Reject a bumped utility's README whose `### Usage` block cannot match the help probe.
5. **Guard CHANGELOG shape and idempotency.**
   - Detect the root `CHANGELOG.md` target.
   - Require the canonical heading and two-column table.
   - Reject an existing row for the target version before any write.
6. **Parse AC refs.** Scan the release message for `AC[0-9]+`; require one matching AC file for every unique reference after the Operator verifies exact release-batch equality. Accept a message without AC references.
7. **Apply writes.**
   - Apply idempotent version bumps.
   - Rewrite each bumped utility's README `### Usage` version line.
   - Insert the CHANGELOG row under `| Unreleased | |`.
   - Delete each released AC file whole.
   - Sweep matching AC-pointer IE lines from `plan.md`.
   - Skip writes under `--dry-run` or `-n`.
   - Leave already-swept lines unchanged on rerun.
8. **Verify and report.** Reject results outside the planned paths, verify every planned transformation, and print one shell-safe two-argument release command.

## DOC Release

### Build Presentation

- Reuse the canonical build color policy and palette.
- Color release status output by semantic role.
- Emit plain output when stdout is not a terminal.
- Emit plain output when `NO_COLOR` is set.
- Emit plain output when `TERM=dumb`.
- Require a 256-color-capable terminal before emitting ANSI sequences.
- Preserve plain-text content and output streams when color is disabled.
- Keep the self-contained build script compatible with Bash 3.2.

### Markdown Identifier Check

- Scan only root `AGENTS.md`, `README.md`, `plan.md`, `CHANGELOG.md`, and every Markdown file under `govna/` for bare AC, AT, and IE identifiers.
- Count a symbolic link by its own path.
- Run the scan at the start of every prep, before any write or dry-run report.
- Stop prep on each identifier that the `AGENTS.md` bare-identifier rules do not allow.
- Treat an uppercase AC, AT, or IE followed by digits, with no adjacent letter, digit, or underscore, as bare.
- Treat a one-line `<!-- Historical: ... -->` comment as a Markdown `Historical:` comment.
- Skip `tests/fixtures/` and every `testdata/` directory.
- Name the file, line, identifier, and a placeholder fix such as `AC<N>` in each finding.
- Keep the scan in one helper block that differs from the CODE block only in its file list.
