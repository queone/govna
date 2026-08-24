# AC7 Audit v0.31.0

## Summary

This adoption covers 2 sync paths, 0 migration paths, 1 review path, and 1 out-of-scope path.

This audit adoption synchronizes deterministic canon changes for `widget`. Audit surfaced 0 migration paths and 1 review path. Per-file inspection uses rendered canon, durable baseline evidence, preserve decisions, and bounded target evidence.

## In Scope

### Direct sync

- `README.md` — `clear-sync`.
- `govna/canon-baseline.txt` — `clear-sync`.

### Migration

- None.

### Review

- `local.md` — `target-has-no-canon`.

### Adoption Instructions

- Resolve every routing decision in chat.
- Leave this emitted stub unchanged.
- Render canon into a scratch directory.
- Verify every direct-sync and canon-backed migration path exists in the selected CODE stack scratch render before applying changes.
- Apply every resolved outcome without changing unrelated content.
- Install `govna/canon-baseline.txt` last.

### Routing Decisions

1. **`local.md`**: Which outcome applies: sync, preserve, migrate, or delete?

## Out Of Scope

- `plan.md` — `expected-divergence`.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every resolved sync target except `govna/canon-baseline.txt` against rendered canon and every resolved preserve target against `govna/preserve.txt`.

**AT2** [Automated] [Pre-release gate] — Satisfy validation disposition `./build.sh` inferred from exact AGENTS.md declarations after selected work and before baseline installation.

**AT3** [Automated] [Pre-release gate] — Install and verify `govna/canon-baseline.txt` from the same scratch render as the final adoption step.

## Status

`PENDING` — audit emission; awaiting explicit Director Audit.
