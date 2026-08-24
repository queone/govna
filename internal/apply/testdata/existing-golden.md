# AC2 Govna Apply

Applied govna v0.32.0 governance template (CODE overlay) to widget.

## Summary

Applied govna v0.32.0 governance template (CODE overlay). All files below are now consumer-owned — modify freely to fit the repo's needs.

## In Scope

Files written by govna apply:

- `.gitignore` (written)
- `AGENTS.md` (canon zone merged, existing tail preserved)
- `CHANGELOG.md` (existing content preserved)
- `README.md` (existing content preserved)
- `arch.md` (existing content preserved)
- `build.sh` (written)
- `govna/README.md` (written)
- `govna/ac-template.md` (written)
- `govna/audit.md` (written)
- `govna/build-release.md` (canon zone merged, existing tail preserved)
- `govna/canon-baseline.txt` (written)
- `govna/canon-cycle.md` (written)
- `govna/code-stacks.md` (written)
- `govna/development-cycle.md` (written)
- `govna/development-guidelines.md` (canon zone merged, existing tail preserved)
- `govna/metadata.txt` (written)
- `govna/operator-contract-rationale.md` (written)
- `govna/roles.md` (written)
- `plan.md` (existing content preserved)
- `tests/build_cli.sh` (written)
- `CLAUDE.md` (existing regular file preserved — not a symlink, see warning)

## Out Of Scope

- All applied files are consumer-owned and can be freely modified

## Migration findings

- None.

## Acceptance Tests

**AT1** [Manual] [Pre-release gate] — Director reads AGENTS.md and confirms it reflects this repo's actual practices; adjust any section that doesn't.

**AT2** [Manual] [Pre-release gate] — Verify govna/roles.md reflects the repo's delivery model (Operator + Director).

**AT3** [Manual] [Pre-release gate] — CLAUDE.md exists as a regular file, not a symlink to AGENTS.md; this apply left it untouched.

## Status

`PENDING` — review applied governance and adapt to repo needs.
