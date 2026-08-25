# AC7 Audit v0.35.0

## Summary

This adoption covers 2 sync paths, 0 migration paths, 1 review path, and 1 out-of-scope path.

This audit adoption synchronizes deterministic canon changes for `widget`. Audit surfaced 0 migration paths and 1 review path. Per-file inspection uses rendered canon, durable baseline evidence, preserve decisions, and bounded target evidence.

## In Scope

### Direct sync

- `AGENTS.md` — `clear-sync`.
- `govna/canon-baseline.txt` — `clear-sync`.

### Migration

- None.

### Review

- `local.md` — `ambiguity`.

### Adoption Instructions

- Resolve every routing decision in chat.
- Leave this emitted stub unchanged.
- Render canon into a scratch directory.
- Verify every direct-sync and canon-backed migration path exists in the selected CODE stack scratch render as a precondition.
- Apply every resolved outcome within the authorized content boundaries.
- Install `govna/canon-baseline.txt` last.

### Routing Decisions

1. **`local.md`**: Which outcome applies: sync, preserve, migrate, or delete?
2. **Validation disposition**: Which outcome applies after selected work: run a repository validation command, or record `Not applicable` with repository evidence?

## Out Of Scope

- `plan.md` — `preserve`.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every resolved sync target except `govna/canon-baseline.txt` against rendered canon and every resolved preserve target against `govna/preserve.txt`.

**AT2** [Automated] [Pre-release gate] — Preserve the protected region in `AGENTS.md` from `## Project Rules` through EOF with SHA-256 `abc123` for any sync outcome.

**AT3** [Manual] [Pre-release gate] — Resolve the validation disposition in chat.

**AT4** [Automated] [Pre-release gate] — Satisfy the resolved validation disposition after selected work and before baseline installation.

**AT5** [Automated] [Pre-release gate] — Verify the final adoption step installed `govna/canon-baseline.txt` from the same scratch render.

## Status

`PENDING` — audit emission; awaiting explicit Director Audit.
