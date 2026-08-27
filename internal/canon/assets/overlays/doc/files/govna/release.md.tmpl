# Release

Use this release, pre-release, and acceptance-test reference under the authoritative `AGENTS.md` gates.

## Release Command

Run `./build.sh vX.Y.Z "release message"` to show status and planned Git steps.
Require its confirmation prompt before `git add → commit → annotated tag → push tag → push branch`.

`build.sh` is self-contained Bash 3.2+ tooling. DOC repositories do not need a compiler toolchain for release preparation or release orchestration and define no automated content-validation command.

Keep DOC release prep repository-wide.
Reject CODE target selection.
Limit release messages to 80 bytes.

DOC repositories do not provide installable utilities, so the independent
utility-version declaration and `--version` contract does not apply to this
release path. CODE consumers use the stack-specific adapter and validation
rules in `govna/build-release.md`.

### Build Presentation

- Reuse the canonical build color policy and palette.
- Color release status output by semantic role.
- Emit plain output when stdout is not a terminal.
- Emit plain output when `NO_COLOR` is set.
- Emit plain output when `TERM=dumb`.
- Require a 256-color-capable terminal before emitting ANSI sequences.
- Preserve plain-text content and output streams when color is disabled.
- Keep the self-contained build script compatible with Bash 3.2.

## Acceptance Tests

Every AT in an AC document must be labeled `[Automated]` or `[Manual]`.

- **Automated** — The result can be verified from CLI output, file inspection, or manual review of rendered content. Automated ATs are run during implementation and re-run as part of the pre-release checklist.
- **Manual** — Requires a live end-to-end action and must be confirmed by the user. The agent cannot self-verify these.

Default to Automated whenever the result is verifiable without a live external service. Manual ATs add friction to the release flow, so reserve them for behaviors that genuinely cannot be checked any other way.

Source axis (`[Automated]` / `[Manual]`) names who verifies. Timing axis (`[Pre-release gate]` / `[Post-release verification]`) names when verification happens. `[Pre-release gate]` is the default and may be omitted; `[Post-release verification]` is explicit. Use `[Post-release verification]` only when automated regression coverage already gates pre-release on the underlying class. The label communicates that the AT is a confidence check, not a gate, so future Operators do not promote it back into a gate.

## Pre-Release Checklist

- Start this checklist only when the Director explicitly requests a valid Package instruction for the established Ratified release batch.
- Map every unpackaged AC with implementation in the unreleased repository state to the complete pending release batch.
- Require every pending release-batch member to complete Ratify before prep.
- Reject prep while excluded implemented work remains in the unreleased repository state.
- Require the unique release-message AC-reference set to equal the established release batch before prep.
- Require the established release batch to equal the complete pending release batch before prep.
- Reject a release message longer than 80 bytes before prep.
- Prohibit a smaller release batch while excluded implemented work remains.
- Prohibit automatic release-batch splitting.
- Do not treat `./build.sh prep ...` or ordinary build-preparation language as a workflow request.

1. **Verify completion.**
   - Verify all in-scope AC work is complete.
   - Verify every AC acceptance test passes.
2. **Determine the version.**
   - Classify the change set using semver.
   - Use PATCH for formatting, fixes, or user-invisible refactors.
   - Use MINOR for user-visible structure, navigation, or schema changes.
   - Bump from the latest tag.
3. **Derive the release message.**
   - Summarize the delivered user-visible result in no more than 80 bytes.
   - Include every established release-batch AC reference.
   - Exclude every AC reference outside the established release batch.
   - Lead with the plus-joined release-batch AC references.
4. **Run release prep.**
   - Run `./build.sh prep vX.Y.Z "derived message"`.
   - Use `--dry-run` or `-n` to inspect without writes.
5. **Complete the Package report.**
   - End the structured Package completion report with `Run below to release:`.
   - Place the exact release command immediately after that line.
   - Add nothing after the release command.

Example release command:

```
./build.sh vX.Y.Z "derived message"
```

The agent never runs the release command. Only the director does.

## CHANGELOG Conventions

- Use a `# Changelog` heading.
- Follow it with the two-column `| Version | Summary |` table.
- Use `|---------|---------|` as the separator.
- Keep `| Unreleased | |` as the first data row.
- Add one row per release.
- Keep summaries single-line and no longer than 500 characters.
- Lead summaries with the AC reference when one exists.
- Versions are unprefixed (`0.29.0`, not `v0.29.0`).
- Do not backfill historical tags or invent alternative shapes (Keep-a-Changelog, sectioned `## vX.Y.Z`, etc.).
