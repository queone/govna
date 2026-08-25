# AC2 Govna Apply

govna executable v9.8.7 applied embedded canon v0.34.0 (CODE overlay) to widget.

## Summary

govna executable v9.8.7 applied embedded canon v0.34.0 (CODE overlay). Every file listed below is consumer-owned.

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

**AT1** [Manual] [Pre-release gate] — Verify AGENTS.md reflects the repository's actual practices.

**AT2** [Manual] [Pre-release gate] — Verify govna/roles.md reflects the repository's delivery model (Operator + Director).

**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md remains the existing regular file instead of a symlink to AGENTS.md.

## Status

`PENDING` — apply emission; awaiting explicit Director Audit.
