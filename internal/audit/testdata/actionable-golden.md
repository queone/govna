# AC7 Audit v0.29.0

## Summary

Adopt the actionable govna audit findings for `widget`.

## In Scope

### Direct sync

- `README.md` — `clear-sync`.

### Migration

- `govna/canon-baseline.txt` — `migration-required`.

### Review

- `local.md` — `target-has-no-canon`.

### Adoption Instructions

- Resolve every routing decision in chat.
- Leave this emitted stub unchanged.
- Render canon into a scratch directory.
- Apply every resolved outcome without changing unrelated content.
- Install `govna/canon-baseline.txt` last.

### Routing Decisions

1. **`local.md`**: Which outcome applies: sync, preserve, migrate, or delete?

## Out Of Scope

- `plan.md` — `expected-divergence`.

## Migration findings

- Apply only the migration outcomes resolved above.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every resolved sync target against rendered canon and every resolved preserve target against `govna/preserve.txt`.

**AT2** [Automated] [Pre-release gate] — Satisfy validation disposition `./build.sh` inferred from exact AGENTS.md declarations after selected work and before baseline installation.

**AT3** [Automated] [Pre-release gate] — Install and verify `govna/canon-baseline.txt` from the same scratch render as the final adoption step.

## Status

`PENDING` — audit emission; awaiting explicit Director Audit.
