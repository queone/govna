# AC2 Review Files Added by Govna

Govna executable v9.8.7 added its embedded governance files (canon v0.36.0) for the CODE repository widget.

## Summary

Govna executable v9.8.7 added its embedded governance files (canon v0.36.0). The list below records whether each file was written, merged, or preserved.

## In Scope

Files Govna processed:

- `.gitignore` (written)
- `AGENTS.md` (updated Govna-managed section; kept repository-owned section)
- `CHANGELOG.md` (kept existing file)
- `README.md` (kept existing file)
- `arch.md` (kept existing file)
- `build.sh` (written)
- `govna/README.md` (written)
- `govna/ac-template.md` (written)
- `govna/audit.md` (written)
- `govna/build-release.md` (updated Govna-managed section; kept repository-owned section)
- `govna/canon-baseline.txt` (written)
- `govna/canon-cycle.md` (written)
- `govna/code-stacks.md` (written)
- `govna/development-cycle.md` (written)
- `govna/development-guidelines.md` (updated Govna-managed section; kept repository-owned section)
- `govna/metadata.txt` (written)
- `govna/operator-contract-rationale.md` (written)
- `govna/roles.md` (written)
- `plan.md` (kept existing file)
- `tests/build_cli.sh` (written)
- `CLAUDE.md` (existing regular file preserved — not a symlink, see warning)

## Out Of Scope

- Files not listed above.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Manual] [Pre-release gate] — Verify AGENTS.md reflects the repository's actual practices.

**AT2** [Manual] [Pre-release gate] — Verify govna/roles.md reflects the repository's delivery model (Operator + Director).

**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md remains the existing regular file instead of a symlink to AGENTS.md.

## Status

`PENDING` — apply emission; awaiting explicit Director Audit.
