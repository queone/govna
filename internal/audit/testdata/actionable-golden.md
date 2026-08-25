# AC7 Review Govna File Updates

## Summary

Govna found 2 files ready to update, 0 required control files to add, 1 file needing a Director decision, and 1 file that will stay unchanged.

This AC updates `widget` to Govna's embedded governance files (canon v0.36.0). The result label (classification) beside each path explains why Govna can update it, must leave it unchanged, or needs a Director choice. Installing the selected updates is the adoption step.

## In Scope

### Files ready to update

- `README.md` — `clear-sync`: the file still matches the previously installed Govna version and is safe to update.
- `govna/canon-baseline.txt` — `clear-sync`: the file still matches the previously installed Govna version and is safe to update.

### Required control files

- None.

### Files needing a Director choice

- `local.md` — `target-has-no-canon`: the file is absent from the selected current canon but specific repository evidence connects it to Govna.

### Adoption Instructions

- Resolve every Director choice in chat.
- Leave this generated AC unchanged.
- Create a temporary copy of the embedded Govna files with `govna render`.
- Confirm each file selected for update exists in the selected CODE render.
- Apply each Director choice only to its authorized file region.
- Write `govna/canon-baseline.txt` as the final file update.

### Routing Decisions

1. **`local.md`**: Which action should Govna record: update (sync), keep local (preserve), move content (migrate), or remove (delete)?

## Out Of Scope

- `plan.md` — `expected-divergence`: the repository is expected to keep its own version of this file.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every file selected for update except `govna/canon-baseline.txt` against the rendered Govna files and every preserved file against `govna/preserve.txt`.

**AT2** [Automated] [Pre-release gate] — Run `./build.sh` after the selected file updates and before `govna/canon-baseline.txt` installation (selected from exact AGENTS.md declarations).

**AT3** [Automated] [Pre-release gate] — Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.

## Status

`PENDING` — audit emission; awaiting explicit Director Audit.
