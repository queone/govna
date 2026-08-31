# AC7 Adopt Govna Governance Files v0.50.0

## Summary

This AC updates `widget` to Govna's embedded governance files (canon v0.50.0). The result label (classification) beside each path explains why Govna can update it, must leave it unchanged, or needs a Director choice. Installing the selected updates is the adoption step.

Govna found 2 files ready to update, 0 required control files to add, 1 file needing a Director decision, and 1 file that will stay unchanged.

## In Scope

### Files ready to update

- `README.md` — `clear-sync`: the file still matches the previously installed Govna version and is safe to update.
- `govna/canon-baseline.txt` — `clear-sync`: the file still matches the previously installed Govna version and is safe to update.

### Required control files

- None.

### Files needing a Director choice

- `local.md` — `target-has-no-canon`: the file is absent from the selected current canon but specific repository evidence connects it to Govna.

### Audit Review

- Use the same resolved Govna executable that emitted this AC.
- Verify its detailed version output matches this AC's guarded marker.
- Create one unique system-temporary scratch directory outside this repository.
- Render the selected canon into that directory once with the resolved executable.
- Verify the rendered `govna/canon-baseline.txt` canon version matches this AC's guarded marker.
- Compare `README.md` with `diff -ru <scratch>/README.md README.md`.
- Compare `govna/canon-baseline.txt` with `diff -ru <scratch>/govna/canon-baseline.txt govna/canon-baseline.txt`.
- Verify `local.md` is absent from the selected scratch render with `test ! -e <scratch>/local.md`.
- Review the exact proposed rules.
- Check rule overlap and placement.
- Resolve every candidate reference.
- Measure prospective contract growth.
- Verify target-side acceptance evidence.
- Keep this emitted AC and every consumer file unchanged.
- Remove the exact scratch directory before reporting Audit completion or a blocker.
- Use no JSON diff field as required review evidence.

### Adoption Instructions

- Resolve every Director choice in chat.
- Leave this generated AC unchanged.
- Create a temporary copy of the embedded Govna files with `govna render`.
- Confirm each file selected for update exists in the selected CODE render.
- Apply each Director choice only to its authorized file region.
- Write `govna/canon-baseline.txt` as the final file update.

### Routing Decisions

1. **`local.md`**: Which action should Govna record: keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?

## Out Of Scope

- `plan.md` — `expected-divergence`: the repository is expected to keep its own version of this file.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every file selected for update except `govna/canon-baseline.txt` against the rendered Govna files and every preserved file against `govna/preserve.txt`.

**AT2** [Automated] [Pre-release gate] — Verify `local.md` remains present when its resolved action is preserve.

**AT3** [Automated] [Pre-release gate] — Verify `local.md` occurs exactly once in `govna/preserve.txt` when its resolved action is preserve.

**AT4** [Automated] [Pre-release gate] — Verify `local.md` is absent when its resolved action is delete.

**AT5** [Automated] [Pre-release gate] — Verify `local.md` is absent from `govna/preserve.txt` when its resolved action is delete.

**AT6** [Automated] [Pre-release gate] — Verify the Director response names a migration destination for `local.md` when its resolved action is migrate.

**AT7** [Automated] [Pre-release gate] — Verify `local.md` is absent unless the Director explicitly preserves it when its resolved action is migrate.

**AT8** [Automated] [Pre-release gate] — Verify any canon-backed migration destination for `local.md` matches its applicable rendered canon region.

**AT9** [Automated] [Pre-release gate] — Verify any repository-owned migration destination for `local.md` matches the Director-stated result.

**AT10** [Automated] [Pre-release gate] — Verify `local.md` is absent from `govna/preserve.txt` when its resolved action is a canon-backed migration.

**AT11** [Automated] [Pre-release gate] — Run `./build.sh` after the selected file updates and before `govna/canon-baseline.txt` installation (selected from exact AGENTS.md declarations).

**AT12** [Automated] [Pre-release gate] — Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.

## Status

`PENDING` — immutable audit emission; workflow state is tracked in the active session.
