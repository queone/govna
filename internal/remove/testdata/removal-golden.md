# AC7 Govna Removal from v0.30.0

## Summary

Extricate govna canon from this consumer repo without deleting consumer-owned content. Emitted by `govna rm` against canon v0.30.0. Implement only after the Director resolves the routing decisions below.

Compare each routing-pending file yourself before choosing how to route it. Do not auto-delete routing-pending files until the Director chooses their routing.

### Routing Decisions

1. `README.md` is mixed canon-shape and consumer content. Compare with: `govna render --flavor code --stack Go <scratch> && diff -ru <scratch>/README.md README.md`. Choose: delete canon-shape only, keep entirely, or delete entirely.
2. `govna/metadata.txt` is consumer-edited canon file. Compare with: `govna render --flavor code --stack Go <scratch> && diff -ru <scratch>/govna/metadata.txt govna/metadata.txt`. Choose: delete canon-shape only, keep entirely, or delete entirely.

## In Scope

- `CLAUDE.md` — delete symlink; govna compatibility link.
- `govna/roles.md` — delete file; byte-equal govna canon.
- `govna/preserve.txt` — delete control state last; preserve decisions applied before registry removal.

## Out Of Scope

- `custom.md` — keep; target-only repo-owned file.
- `plan.md` — keep; repo-owned govna-adjacent content.

## Migration findings

- None. Apply only the Director-resolved removal routes.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Removed files listed under `## In Scope` no longer exist.

**AT2** [Manual] [Pre-release gate] — Director confirms every routing-pending file under `### Routing Decisions` was routed exactly as decided.

**AT3** [Automated] [Pre-release gate] — Every preserve-registry decision is applied and verified before `govna/preserve.txt` is deleted as the final control-state removal.

## Status

`PENDING` — Emitted by `govna rm`; awaiting Director review.
