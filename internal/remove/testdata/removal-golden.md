# AC7 Review Removal of Govna Files

## Summary

Govna executable v9.8.7 created this removal plan from its embedded governance files (canon v0.41.0). This AC removes Govna-managed content without deleting repository-owned content. Files needing a choice stay unchanged until the Director decides what to do.

### Removal Instructions

- Create a temporary copy of the selected Govna files with `govna render --flavor code --stack Go <scratch>`.
- Preserve every file under Routing Decisions until the Director resolves it.
- Resolve every Director choice in chat.
- Apply each in-scope removal and Director choice.

### Routing Decisions

1. `README.md`: contains both Govna-managed and repository-owned content.
   - Compare `README.md` with `diff -ru <scratch>/README.md README.md`.
   - Choose what to remove from `README.md`: only its Govna-managed section, nothing, or the whole file.
2. `govna/metadata.txt`: Govna-managed file has local edits.
   - Compare `govna/metadata.txt` with `diff -ru <scratch>/govna/metadata.txt govna/metadata.txt`.
   - Choose what to remove from `govna/metadata.txt`: only its Govna-managed section, nothing, or the whole file.

## In Scope

- `CLAUDE.md` — delete symlink; govna compatibility link.
- `govna/roles.md` — delete file; matches the current Govna file exactly.
- `govna/preserve.txt` — delete control state last; preserve decisions applied before registry removal.

## Out Of Scope

- `custom.md` — keep; repository-owned file with no matching entry in Govna's current canon.
- `plan.md` — keep; a repository-owned planning file that Govna never manages.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every resolved removal target under `## In Scope` is absent.

**AT2** [Manual] [Pre-release gate] — Verify every file under Routing Decisions matches its Director-resolved action.

**AT3** [Automated] [Pre-release gate] — Verify every keep-local choice is applied before the final removal of `govna/preserve.txt`.

## Status

`PENDING` — removal emission; awaiting explicit Director Audit.
