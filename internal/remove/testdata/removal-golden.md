# AC7 Govna Removal from v0.35.0

## Summary

This removal AC was emitted by govna executable v9.8.7 with embedded canon v0.35.0. It removes Govna canon from this consumer repository without deleting consumer-owned content. Director-resolved routing protects every review path.

### Removal Instructions

- Render the selected canon into `<scratch>` with `govna render --flavor code --stack Go <scratch>`.
- Preserve every routing-pending path until its route is resolved.
- Resolve every routing decision in chat.
- Apply each in-scope route and each Director-resolved review route.

### Routing Decisions

1. `README.md` is mixed canon-shape and consumer content.
   - Compare `README.md` with `diff -ru <scratch>/README.md README.md`.
   - Choose one route for `README.md`: canon-only deletion, full preservation, or full deletion.
2. `govna/metadata.txt` is consumer-edited canon file.
   - Compare `govna/metadata.txt` with `diff -ru <scratch>/govna/metadata.txt govna/metadata.txt`.
   - Choose one route for `govna/metadata.txt`: canon-only deletion, full preservation, or full deletion.

## In Scope

- `CLAUDE.md` — delete symlink; govna compatibility link.
- `govna/roles.md` — delete file; byte-equal govna canon.
- `govna/preserve.txt` — delete control state last; preserve decisions applied before registry removal.

## Out Of Scope

- `custom.md` — keep; target-only repo-owned file.
- `plan.md` — keep; repo-owned govna-adjacent content.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every resolved removal target under `## In Scope` is absent.

**AT2** [Manual] [Pre-release gate] — Verify every routing-pending path matches its Director-resolved route.

**AT3** [Automated] [Pre-release gate] — Verify every preserve-registry decision is applied before the final removal of `govna/preserve.txt`.

## Status

`PENDING` — removal emission; awaiting explicit Director Audit.
