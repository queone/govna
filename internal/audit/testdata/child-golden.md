# AC7 Adopt Govna Governance Files v0.89.0

## Summary

This AC updates `widget` to Govna's embedded governance files (canon v0.89.0). The result label (classification) beside each path explains why Govna can update it, must leave it unchanged, or needs a Director choice. Installing the selected updates is the adoption step.

Govna found 2 files ready to update, 0 required control files to add, 1 file needing a Director decision, and 0 files that will stay unchanged.

Govna also found 1 child file needing a manual fix.

## In Scope

### Files ready to update

- `../engine/AGENTS.md` — `missing-in-target`: a file from current Govna rules is missing from the repository.
- `../engine/build.sh` — `force-sync`: this file's governed structure always syncs to canon, regardless of local edits or the preserve list.

### Required control files

- None.

### Files needing a Director choice

- `../engine/.gitignore` — `ambiguity`: Govna cannot safely choose between updating and keeping the file.

### Files needing a manual fix

- `../engine/CHANGELOG.md` — `manual-fix`: the child's CHANGELOG.md breaks the canonical table shape, and only a manual edit that keeps every row can restore it.

### Audit Review

- Use the same resolved Govna executable that emitted this AC.
- Verify its detailed version output matches this AC's guarded marker.
- Create one unique system-temporary scratch directory outside this repository.
- Render the selected canon into that directory once with the resolved executable.
- Verify the rendered `govna/canon-baseline.txt` canon version matches this AC's guarded marker.
- Render the child files once into a second scratch directory with `govna render --child --stack Go <child-scratch>`.
- Compare `../engine/AGENTS.md` with `diff -u /dev/null <child-scratch>/AGENTS.md`.
- Compare `../engine/build.sh` with `diff -u <child-scratch>/build.sh ../engine/build.sh`.
- Compare `../engine/.gitignore` with `diff -u <child-scratch>/.gitignore ../engine/.gitignore`.
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
- Create a temporary copy of the child files with `govna render --child`.
- Confirm each parent file selected for update exists in the selected CODE render.
- Confirm each child file selected for update exists in the child render.
- Apply each Director choice only to its authorized file region.
- Write `govna/canon-baseline.txt` as the final file update.

### Routing Decisions

1. **`../engine/.gitignore`**: Which action should Govna record: update (sync), keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?

## Out Of Scope

- No preserved or expected-divergence entries.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every file selected for update except `govna/canon-baseline.txt` against the rendered Govna files and every preserved file against `govna/preserve.txt`.

**AT2** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` matches its applicable rendered canon region when its resolved action is sync.

**AT3** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` is absent from `govna/preserve.txt` when its resolved action is sync.

**AT4** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` remains present when its resolved action is preserve.

**AT5** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` occurs exactly once in `govna/preserve.txt` when its resolved action is preserve.

**AT6** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` is absent when its resolved action is delete.

**AT7** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` is absent from `govna/preserve.txt` when its resolved action is delete.

**AT8** [Automated] [Pre-release gate] — Verify the Director response names a migration destination for `../engine/.gitignore` when its resolved action is migrate.

**AT9** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` is absent unless the Director explicitly preserves it when its resolved action is migrate.

**AT10** [Automated] [Pre-release gate] — Verify any canon-backed migration destination for `../engine/.gitignore` matches its applicable rendered canon region.

**AT11** [Automated] [Pre-release gate] — Verify any repository-owned migration destination for `../engine/.gitignore` matches the Director-stated result.

**AT12** [Automated] [Pre-release gate] — Verify `../engine/.gitignore` is absent from `govna/preserve.txt` when its resolved action is a canon-backed migration.

**AT13** [Automated] [Pre-release gate] — Verify `../engine/CHANGELOG.md` keeps every row and passes the canonical CHANGELOG heading and two-column table shape.

**AT14** [Automated] [Pre-release gate] — Run `./build.sh` after the selected file updates and before `govna/canon-baseline.txt` installation (selected from exact AGENTS.md declarations).

**AT15** [Automated] [Pre-release gate] — Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.

## Status

`PENDING` — immutable audit emission; workflow state is tracked in the active session.
