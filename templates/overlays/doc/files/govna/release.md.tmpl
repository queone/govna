# Release

Use this release, pre-release, and acceptance-test reference under the authoritative `AGENTS.md` gates.

## Release Command

Run `./build.sh vX.Y.Z "release message"` to show status and planned Git steps.
Require its confirmation prompt before `git add → commit → annotated tag → push tag → push branch`.

`build.sh` is self-contained Bash 3.2+ tooling. DOC repositories do not need a compiler toolchain for release preparation or release orchestration and define no automated content-validation command.

Keep DOC release prep repository-wide.
Reject CODE target selection.
Limit release messages to 80 characters.

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

## Pre-Release Checklist (`Package`, `package`, `pack`, or `prep`)

Do not start this checklist unless the director explicitly requests standalone
`Package`, `package`, `pack`, or `prep` in the active Ratified AC context.
Do not treat `./build.sh prep ...` or ordinary build-preparation language as a
workflow request.

1. **Verify completion.**
   - Verify all in-scope AC work is complete.
   - Verify every AC acceptance test passes.
2. **Determine the version.**
   - Classify the change set using semver.
   - Use PATCH for formatting, fixes, or user-invisible refactors.
   - Use MINOR for user-visible structure, navigation, or schema changes.
   - Bump from the latest tag.
3. **Derive the release message.**
   - Summarize the change set in no more than 80 characters.
   - Lead with the AC reference when one exists.
4. **Run release prep.**
   - Run `./build.sh prep vX.Y.Z "derived message"`.
   - Use `--dry-run` or `-n` to inspect without writes.
5. **Present the release command.** Print the exact command emitted by prep for the director to run:

```
./build.sh vX.Y.Z "derived message"
```

The agent never runs the release command — only the director does. Do not add trailing commentary after presenting the command. The director already knows.

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
